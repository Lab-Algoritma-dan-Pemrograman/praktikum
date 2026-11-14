// Command phase6-inspect reads only PostgreSQL metadata and aggregate counts.
// Credentials are consumed from an existing private environment file, never logged.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type table struct {
	Name    string             `json:"name"`
	Rows    int64              `json:"rows"`
	Digest  string             `json:"content_sha256"`
	RLS     bool               `json:"rls"`
	Columns []column           `json:"columns"`
	Latest  map[string]*string `json:"latest_timestamps,omitempty"`
}
type report struct {
	Label               string                      `json:"label"`
	Mode                string                      `json:"mode"`
	Status              string                      `json:"status"`
	EndpointFingerprint string                      `json:"endpoint_fingerprint"`
	ProjectFingerprint  string                      `json:"project_fingerprint"`
	ServerVersion       string                      `json:"server_version"`
	Tables              []table                     `json:"tables"`
	Migrations          []map[string]string         `json:"migrations,omitempty"`
	MaskedRows          map[string][]map[string]any `json:"masked_rows,omitempty"`
	MaskingPolicy       string                      `json:"masking_policy,omitempty"`
	Quality             map[string]int64            `json:"quality_counts,omitempty"`
	AuthUserColumns     []map[string]any            `json:"auth_user_columns,omitempty"`
	CapturedAt          time.Time                   `json:"captured_at"`
	Failure             string                      `json:"failure,omitempty"`
}

func main() {
	file := flag.String("env-file", "", "existing private config; values are never printed")
	label := flag.String("label", "", "source label")
	out := flag.String("out", "", "new metadata output file")
	masked := flag.Bool("masked-snapshot", false, "explicitly authorized source snapshot, masked in memory before persistence")
	expected := flag.String("expected-project", "", "pinned source project fingerprint required for masked export")
	flag.Parse()
	r := report{Label: *label, Mode: "read-only-metadata-and-counts", Status: "failed", CapturedAt: time.Now().UTC()}
	err := inspect(*file, &r, *masked, *expected)
	if err != nil {
		r.MaskedRows = nil
		r.Failure = err.Error()
	} else {
		r.Status = "passed"
	}
	f, e := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		fmt.Fprintln(os.Stderr, "cannot create report")
		os.Exit(1)
	}
	e = json.NewEncoder(f).Encode(r)
	closeErr := f.Close()
	if err != nil || e != nil || closeErr != nil {
		fmt.Fprintln(os.Stderr, "read-only inventory failed; see sanitized report")
		os.Exit(1)
	}
	fmt.Printf("%s: metadata/counts captured for %d public tables\n", r.Label, len(r.Tables))
}
func readConfig(path string) (map[string]string, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("config unavailable")
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 65536), 1<<20)
	v := map[string]string{}
	allowed := map[string]bool{"DB_HOST": true, "DB_PORT": true, "DB_USER": true, "DB_PASSWORD": true, "DB_NAME": true, "SUPABASE_URL": true, "ALGOHUB_MIGRATION_DATABASE_URL": true, "ALGOHUB_DATABASE_URL": true}
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		line = strings.TrimPrefix(line, "export ")
		k, value, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || !allowed[k] {
			continue
		}
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "\"") {
			decoded, e := strconv.Unquote(value)
			if e != nil {
				return nil, errors.New("unsupported private config quoting")
			}
			value = decoded
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		v[k] = value
	}
	if s.Err() != nil {
		return nil, errors.New("config read failed")
	}
	return v, nil
}
func fingerprint(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func inspect(path string, r *report, masked bool, expected string) error {
	v, e := readConfig(path)
	if e != nil {
		return e
	}
	dsn := v["ALGOHUB_MIGRATION_DATABASE_URL"]
	if dsn == "" {
		dsn = v["ALGOHUB_DATABASE_URL"]
	}
	if dsn == "" {
		if v["DB_HOST"] == "" || v["DB_NAME"] == "" || v["DB_USER"] == "" {
			return errors.New("direct PostgreSQL configuration absent")
		}
		port := v["DB_PORT"]
		if port == "" {
			port = "5432"
		}
		u := url.URL{Scheme: "postgres", Host: v["DB_HOST"] + ":" + port, Path: "/" + v["DB_NAME"], User: url.UserPassword(v["DB_USER"], v["DB_PASSWORD"])}
		q := url.Values{}
		q.Set("sslmode", "require")
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	u, e := url.Parse(dsn)
	if e != nil || u.Hostname() == "" {
		return errors.New("invalid direct endpoint")
	}
	r.EndpointFingerprint = fingerprint(strings.ToLower(u.Hostname()) + "/" + u.Path + "/" + u.User.Username())
	p, e := url.Parse(v["SUPABASE_URL"])
	if e == nil {
		r.ProjectFingerprint = fingerprint(strings.ToLower(p.Hostname()))
	}
	if masked && (len(expected) != 64 || expected != r.ProjectFingerprint) {
		return errors.New("masked export requires the pinned source project")
	}
	for _, key := range []string{"PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGPASSFILE", "PGSERVICE", "PGSERVICEFILE", "PGSYSCONFDIR", "PGOPTIONS", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY"} {
		os.Unsetenv(key)
	}
	c, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return errors.New("cannot parse direct connection")
	}
	if c.ConnConfig.TLSConfig == nil {
		return errors.New("TLS required for remote inspection")
	}
	c.MaxConns = 1
	c.MinConns = 0
	c.ConnConfig.ConnectTimeout = 8 * time.Second
	c.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	c.ConnConfig.RuntimeParams["application_name"] = "phase6-readonly-inspection"
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool, e := pgxpool.NewWithConfig(ctx, c)
	if e != nil {
		return errors.New("connection setup failed")
	}
	defer pool.Close()
	tx, e := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return errors.New("cannot begin read-only source transaction")
	}
	defer func() {
		cl, cc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cc()
		tx.Rollback(cl)
	}()
	if _, e = tx.Exec(ctx, `set local statement_timeout='5s';set local lock_timeout='500ms';set local idle_in_transaction_session_timeout='15s';set local search_path=public,extensions`); e != nil {
		return errors.New("cannot set read-only inspection deadlines")
	}
	if e = tx.QueryRow(ctx, `show server_version`).Scan(&r.ServerVersion); e != nil {
		return errors.New("version metadata query failed")
	}
	rows, e := tx.Query(ctx, `select c.relname,c.relrowsecurity from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relkind in ('r','p') order by c.relname`)
	if e != nil {
		return errors.New("table metadata query failed")
	}
	for rows.Next() {
		t := table{}
		if e = rows.Scan(&t.Name, &t.RLS); e != nil {
			rows.Close()
			return errors.New("table metadata decode failed")
		}
		r.Tables = append(r.Tables, t)
	}
	rows.Close()
	if rows.Err() != nil {
		return errors.New("table metadata read failed")
	}
	if len(r.Tables) > 250 {
		return errors.New("public table count exceeds bounded inspection scope")
	}
	for i := range r.Tables {
		t := &r.Tables[i]
		rows, e = tx.Query(ctx, `select column_name,data_type from information_schema.columns where table_schema='public' and table_name=$1 order by ordinal_position`, t.Name)
		if e != nil {
			return errors.New("column metadata query failed")
		}
		for rows.Next() {
			col := column{}
			if e = rows.Scan(&col.Name, &col.Type); e != nil {
				rows.Close()
				return errors.New("column metadata decode failed")
			}
			t.Columns = append(t.Columns, col)
		}
		rows.Close()
		if rows.Err() != nil {
			return errors.New("column metadata read failed")
		}
		fingerprintQuery := "select count(*),encode(digest(coalesce(string_agg(to_jsonb(t)::text,E'\\n' order by to_jsonb(t)::text),''),'sha256'),'hex') from " + pgx.Identifier{"public", t.Name}.Sanitize() + " t"
		if e = tx.QueryRow(ctx, fingerprintQuery).Scan(&t.Rows, &t.Digest); e != nil {
			return errors.New("bounded count/fingerprint query failed for " + t.Name)
		}
	}
	rows, e = tx.Query(ctx, `select column_name,data_type,is_nullable,column_default from information_schema.columns where table_schema='auth' and table_name='users' order by ordinal_position`)
	if e != nil {
		return errors.New("auth metadata query failed")
	}
	for rows.Next() {
		var name, typ, nullable string
		var def *string
		if e = rows.Scan(&name, &typ, &nullable, &def); e != nil {
			rows.Close()
			return errors.New("auth metadata decode failed")
		}
		r.AuthUserColumns = append(r.AuthUserColumns, map[string]any{"name": name, "type": typ, "nullable": nullable, "has_default": def != nil})
	}
	rows.Close()
	if rows.Err() != nil {
		return errors.New("auth metadata read failed")
	}
	for _, t := range r.Tables {
		if t.Name != "schema_migrations" {
			continue
		}
		hasName, hasChecksum := false, false
		for _, c := range t.Columns {
			hasName = hasName || c.Name == "name"
			hasChecksum = hasChecksum || c.Name == "checksum"
		}
		if !hasName || !hasChecksum {
			break
		}
		rows, e = tx.Query(ctx, `select name,checksum from public.schema_migrations order by name`)
		if e != nil {
			return errors.New("migration history query failed")
		}
		for rows.Next() {
			var n, h string
			if e = rows.Scan(&n, &h); e != nil {
				rows.Close()
				return errors.New("migration history decode failed")
			}
			r.Migrations = append(r.Migrations, map[string]string{"name": n, "sha256": h})
		}
		rows.Close()
		if rows.Err() != nil {
			return errors.New("migration history read failed")
		}
	}
	if e = inspectQuality(ctx, tx, r); e != nil {
		return e
	}
	var captured map[string][]map[string]any
	if masked {
		captured, e = collectSnapshot(ctx, tx, r)
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return errors.New("read-only inspection commit failed")
	}
	// Close the read-only snapshot before CPU-bound masking. Keeping it open
	// needlessly holds a production snapshot and can hit the idle-tx deadline.
	if masked {
		if e = maskRows(captured); e != nil {
			return e
		}
		r.MaskedRows = captured
		r.MaskingPolicy = "local-pseudonymized-v1-no-credentials-no-learner-free-text"
		r.Mode = "read-only-consistent-pseudonymized-working-snapshot"
	}
	return nil
}
