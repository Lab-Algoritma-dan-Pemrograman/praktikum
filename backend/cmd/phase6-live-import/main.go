package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	mr "github.com/algohub/algohub-rewrite/backend/internal/migrationrehearsal"
	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const format = "phase6-live-plan-v1"

type config struct {
	pool            *pgxpool.Pool
	ctx             context.Context
	cancel          context.CancelFunc
	fingerprint     string
	local           *mr.LocalTarget
	afterAcceptance bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "phase6-live-import:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("use preflight, apply, reconcile, or rollback")
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	get := func(name string) string {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == name {
				return args[i+1]
			}
		}
		return ""
	}
	planPath, out := get("--plan"), get("--out")
	if planPath == "" || out == "" {
		return errors.New("plan and out are required")
	}
	expected := get("--expected-plan-sha256")
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(expected) {
		return errors.New("expected plan SHA-256 is required")
	}
	plan, fileHash, err := loadPlan(planPath)
	if err != nil {
		return err
	}
	if fileHash != expected {
		return errors.New("plan file checksum mismatch")
	}
	cfg, err := openConfig(get("--env-file"), get("--expected-project"), get("--local-target"))
	if err != nil {
		return err
	}
	defer cfg.cancel()
	defer cfg.pool.Close()
	if cmd != "seed-auth-metadata" {
		if err = preflight(cfg, &plan); err != nil {
			return err
		}
	} else if cfg.local == nil {
		return errors.New("Auth metadata recovery is local-only")
	}
	var output any
	switch cmd {
	case "acceptance-sessions":
		output, err = acceptanceSessions(cfg, &plan)
	case "auth-metadata":
		output, err = exportAuthMetadata(cfg)
	case "seed-auth-metadata":
		output, err = seedLocalAuthMetadata(cfg, get("--auth-metadata"))
	case "local-acceptance-password":
		output, err = setLocalAcceptancePassword(cfg, &plan)
	case "snapshot":
		output, err = stateSnapshot(cfg)
	case "preflight":
		output = map[string]any{"status": "passed", "batch_id": plan.BatchID, "plan_file_sha256": fileHash, "target_fingerprint": cfg.fingerprint, "source_counts": plan.Counts, "target_counts": map[string]int{"auth_users": len(plan.AuthUsers), "users": len(plan.Users), "assignments": len(plan.Assignments), "domain_rows": len(plan.DomainRows)}}
	case "apply":
		if get("--confirm-batch") != plan.BatchID {
			return errors.New("exact confirm-batch required")
		}
		if get("--receipt") != "" {
			var receipt Receipt
			if err = readStrict(get("--receipt"), 64<<20, &receipt); err == nil {
				output, err = replayVerified(cfg, &plan, fileHash, &receipt)
			}
		} else {
			output, err = apply(cfg, &plan, fileHash)
		}
	case "reconcile-after-acceptance":
		cfg.afterAcceptance = true
		fallthrough
	case "reconcile":
		var receipt Receipt
		if err = readStrict(get("--receipt"), 64<<20, &receipt); err == nil {
			output, err = reconcile(cfg, &plan, fileHash, &receipt)
		}
	case "rollback":
		if get("--confirm-batch") != plan.BatchID {
			return errors.New("exact confirm-batch required")
		}
		var receipt Receipt
		if err = readStrict(get("--receipt"), 64<<20, &receipt); err == nil {
			output, err = rollback(cfg, &plan, fileHash, &receipt)
		}
	default:
		return errors.New("unsupported command")
	}
	if err != nil {
		return err
	}
	return writeNew(out, output)
}
func loadPlan(path string) (Plan, string, error) {
	var p Plan
	b, err := readFile(path, 128<<20)
	if err != nil {
		return p, "", err
	}
	h := sha256.Sum256(b)
	if err = decodeStrict(b, &p); err != nil {
		return p, "", err
	}
	if err = validatePlan(&p); err != nil {
		return p, "", err
	}
	return p, hex.EncodeToString(h[:]), nil
}
func validatePlan(p *Plan) error {
	if p.FormatVersion != format || p.BatchID == "" || len(p.AuthUsers) != len(p.Users) || len(p.Users) != len(p.Assignments) || len(p.DomainRows) == 0 {
		return errors.New("invalid plan header/counts")
	}
	seen := map[string]bool{}
	auth := map[string]bool{}
	for _, u := range p.AuthUsers {
		if u.Token == "" || u.AuthUserID == "" || u.Email == "" || seen[u.Token] || auth[u.AuthUserID] {
			return errors.New("invalid or duplicate identity plan")
		}
		seen[u.Token] = true
		auth[u.AuthUserID] = true
	}
	for _, u := range p.Users {
		if !seen[u.Token] || u.AuthUserID == "" || u.Email != strings.ToLower(u.Email) {
			return errors.New("invalid user plan")
		}
	}
	for _, a := range p.Assignments {
		if a.Token == "" || !seen[a.UserToken] || seen[a.Token] {
			return errors.New("invalid assignment plan")
		}
		seen[a.Token] = true
	}
	for _, r := range p.DomainRows {
		if r.Token == "" || seen[r.Token] || !allowedTables[r.Table] || len(r.ID) > 128 {
			return errors.New("invalid domain plan")
		}
		seen[r.Token] = true
	}
	source := 0
	counts := map[string]int{}
	for _, d := range p.Dispositions {
		if d.Rows < 1 || d.Reason == "" || (d.Disposition != "imported" && d.Disposition != "rejected") {
			return errors.New("invalid source disposition")
		}
		source += d.Rows
		counts[d.Disposition] += d.Rows
	}
	if source != p.Counts["imported"]+p.Counts["rejected"] || counts["imported"] != p.Counts["imported"] || counts["rejected"] != p.Counts["rejected"] {
		return errors.New("source accounting mismatch")
	}
	return nil
}
func openConfig(envFile, expectedProject, targetFile string) (*config, error) {
	vals, err := readEnv(envFile)
	if err != nil {
		return nil, err
	}
	dsn := vals["ALGOHUB_MIGRATION_DATABASE_URL"]
	if dsn == "" {
		dsn = vals["ALGOHUB_DATABASE_URL"]
	}
	if dsn == "" {
		return nil, errors.New("database configuration absent")
	}
	for _, k := range []string{"PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGPASSFILE", "PGSERVICE", "PGSERVICEFILE", "PGSYSCONFDIR", "PGOPTIONS", "PGSSLMODE"} {
		os.Unsetenv(k)
	}
	pc, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	pc.MaxConns = 2
	pc.MinConns = 0
	pc.ConnConfig.ConnectTimeout = 8 * time.Second
	pc.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pc.ConnConfig.RuntimeParams["application_name"] = "phase6-masked-live-rehearsal"
	pc.ConnConfig.RuntimeParams["search_path"] = "public,extensions"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		cancel()
		return nil, errors.New("database client setup failed")
	}
	c := &config{pool: pool, ctx: ctx, cancel: cancel}
	if targetFile != "" {
		var t mr.LocalTarget
		if err = readStrict(targetFile, 16<<10, &t); err != nil {
			pool.Close()
			cancel()
			return nil, errors.New("invalid local target descriptor")
		}
		if pc.ConnConfig.Host != "127.0.0.1" || net.ParseIP(pc.ConnConfig.Host) == nil || !strings.HasSuffix(pc.ConnConfig.Database, "_test") {
			pool.Close()
			cancel()
			return nil, errors.New("local target must be literal-loopback _test")
		}
		c.local = &t
		c.ctx = mr.WithLocalTarget(ctx, t)
		if err = mr.VerifyLocalTarget(c.ctx, pool, t); err != nil {
			pool.Close()
			cancel()
			return nil, err
		}
		c.fingerprint = "local:" + t.SystemIdentifier + ":" + t.Database
		return c, nil
	}
	u, err := url.Parse(vals["SUPABASE_URL"])
	if err != nil || u.Hostname() == "" || len(expectedProject) != 64 {
		pool.Close()
		cancel()
		return nil, errors.New("remote target identity unavailable")
	}
	h := sha256.Sum256([]byte(strings.ToLower(u.Hostname())))
	c.fingerprint = hex.EncodeToString(h[:])
	if c.fingerprint != expectedProject || pc.ConnConfig.TLSConfig == nil {
		pool.Close()
		cancel()
		return nil, errors.New("remote target fingerprint/TLS mismatch")
	}
	return c, nil
}
func preflight(c *config, p *Plan) error {
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='30s';set local lock_timeout='5s';set local idle_in_transaction_session_timeout='60s'`); err != nil {
		return err
	}
	rows, err := tx.Query(c.ctx, `select name,checksum from public.schema_migrations order by name`)
	if err != nil {
		return errors.New("migration history unavailable")
	}
	actual := map[string]string{}
	for rows.Next() {
		var n, h string
		if err = rows.Scan(&n, &h); err != nil {
			rows.Close()
			return err
		}
		actual[n] = h
	}
	rows.Close()
	catalog := migrations.Catalog()
	if len(actual) != len(catalog) {
		return errors.New("target migration count mismatch")
	}
	for _, m := range catalog {
		if actual[m.Name] != m.Checksum {
			return errors.New("target migration checksum mismatch")
		}
	}
	// Imported identities must not collide with existing emails/NIMs.
	emails := make([]string, 0, len(p.Users))
	nims := make([]string, 0, len(p.Users))
	for _, u := range p.Users {
		emails = append(emails, u.Email)
		if u.NIM != nil {
			nims = append(nims, *u.NIM)
		}
	}
	authIDs := make([]string, 0, len(p.AuthUsers))
	for _, u := range p.AuthUsers {
		authIDs = append(authIDs, u.AuthUserID)
	}
	var identityConflicts int
	if err = tx.QueryRow(c.ctx, `select count(*) from public.users where (email=any($1) or (nim is not null and nim=any($2))) and not (auth_user_id=any($3))`, emails, nims, authIDs).Scan(&identityConflicts); err != nil {
		return err
	}
	if identityConflicts != 0 {
		return errors.New("imported identity conflicts with existing target emails/NIMs")
	}
	// Business keys with a table-wide unique index must not already exist on a
	// shared target. Templated keys derive from new identities/definitions and
	// cannot collide; literal class codes and item codes can.
	keys := map[string][]string{}
	ownIDs := map[string][]string{}
	for _, r := range p.DomainRows {
		switch r.Table {
		case "academic_classes", "academic_inventory_items":
			if r.UniqueKey != "" {
				keys[r.Table] = append(keys[r.Table], r.UniqueKey)
			}
			if r.ID != "" {
				ownIDs[r.Table] = append(ownIDs[r.Table], r.ID)
			}
		}
	}
	for table, values := range keys {
		var conflicts int
		if err = tx.QueryRow(c.ctx, `select count(*) from `+pgx.Identifier{"public", table}.Sanitize()+` where unique_key=any($1) and not (id=any($2))`, values, ownIDs[table]).Scan(&conflicts); err != nil {
			return err
		}
		if conflicts != 0 {
			return errors.New("business key conflict with existing target rows: " + table)
		}
	}
	for table := range allowedTables {
		var exists bool
		if err = tx.QueryRow(c.ctx, `select exists(select 1 from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relname=$1 and c.relkind='r' and c.relrowsecurity)`, table).Scan(&exists); err != nil || !exists {
			return errors.New("required RLS table missing")
		}
	}
	return tx.Commit(c.ctx)
}
func readEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("private config unavailable")
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 64<<10), 1<<20)
	v := map[string]string{}
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Text()), "export "))
		k, x, ok := strings.Cut(line, "=")
		if !ok || !(k == "ALGOHUB_MIGRATION_DATABASE_URL" || k == "ALGOHUB_DATABASE_URL" || k == "SUPABASE_URL") {
			continue
		}
		x = strings.TrimSpace(x)
		if len(x) >= 2 && x[0] == x[len(x)-1] && (x[0] == 39 || x[0] == 34) {
			x = x[1 : len(x)-1]
		}
		v[k] = x
	}
	return v, s.Err()
}
func readFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(b)) > max {
		return nil, errors.New("bounded input read failed")
	}
	return b, nil
}
func readStrict(path string, max int64, dst any) error {
	b, err := readFile(path, max)
	if err != nil {
		return err
	}
	return decodeStrict(b, dst)
}
func decodeStrict(b []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	d.UseNumber()
	if err := d.Decode(dst); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
func writeNew(path string, v any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	e := json.NewEncoder(f)
	e.SetEscapeHTML(false)
	err = e.Encode(v)
	if ce := f.Close(); err == nil {
		err = ce
	}
	return err
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func fileBase(path string) string { return filepath.Base(path) }
func atoi(s string) int           { n, _ := strconv.Atoi(s); return n }
