// Command rehearsal is a local-only synthetic migration rehearsal tool.
// It deliberately has no database URL, remote mode, secret file, or legacy input adapter.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	r "github.com/algohub/algohub-rewrite/backend/internal/migrationrehearsal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("use fixture, transform, init, preflight, apply, reconcile, or rollback")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	targetPath := flags.String("target", "", "harness-owned target JSON (not credentials)")
	manifestPath := flags.String("manifest", "", "synthetic manifest")
	planPath := flags.String("plan", "", "verified transform plan")
	receiptPath := flags.String("receipt", "", "first successful apply receipt")
	out := flags.String("out", "", "new output path (never overwritten)")
	confirm := flags.String("confirm-batch", "", "exact batch ID for apply/rollback")
	baselinePath := flags.String("baseline", "", "preflight JSON with pinned structural fingerprint")
	authShim := flags.String("auth", "minimal", "init auth prerequisite: minimal or full")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *out == "" {
		return errors.New("out is required")
	}
	if _, err := os.Lstat(*out); err == nil {
		return errors.New("output exists; use a new path")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if command == "fixture" {
		return fixture(*out)
	}
	if command == "transform" {
		m, rows, err := r.LoadManifest(*manifestPath)
		if err != nil {
			return err
		}
		plan, err := r.BuildPlan(m, rows)
		if err != nil {
			return err
		}
		return r.WriteJSONAtomic(*out, plan, false)
	}
	switch command {
	case "init", "preflight", "apply", "reconcile", "rollback":
	default:
		return errors.New("unsupported command")
	}
	var target r.LocalTarget
	if err := readJSON(*targetPath, &target); err != nil {
		return errors.New("invalid owned target file")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_]*_test$`).MatchString(target.Database) || target.Port == 0 {
		return errors.New("invalid disposable database identity")
	}
	// Do not inherit service files, fallback hosts, passfiles, or credentials.
	for _, name := range []string{"PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGPASSFILE", "PGSERVICE", "PGSERVICEFILE", "PGSYSCONFDIR", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY", "PGOPTIONS", "PGTARGETSESSIONATTRS", "PGCONNECT_TIMEOUT"} {
		_ = os.Unsetenv(name)
	}
	config, err := pgxpool.ParseConfig("host=127.0.0.1 port=" + strconv.Itoa(int(target.Port)) + " dbname=" + target.Database + " user=phase6 password='' passfile='" + filepath.ToSlash(os.DevNull) + "' sslmode=disable connect_timeout=5")
	if err != nil {
		return errors.New("cannot build local connection")
	}
	config.MaxConns = 2
	config.MinConns = 0
	config.ConnConfig.Fallbacks = nil
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.RuntimeParams["statement_timeout"] = "30000"
	config.ConnConfig.RuntimeParams["lock_timeout"] = "5000"
	config.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "45000"
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	config.ConnConfig.RuntimeParams["application_name"] = "algohub-phase6-local"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ctx = r.WithLocalTarget(ctx, target)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("cannot open owned local database")
	}
	defer pool.Close()
	if command != "init" && *baselinePath == "" {
		return errors.New("existing database commands require the initialized baseline fingerprint")
	}
	if command != "init" {
		var baseline r.Preflight
		if err = readJSON(*baselinePath, &baseline); err != nil {
			return errors.New("invalid baseline preflight")
		}
		current, err := r.Inspect(ctx, pool)
		if err != nil {
			return err
		}
		if current.Snapshot.SchemaDigest != baseline.Snapshot.SchemaDigest {
			changed := []int{}
			for i, section := range current.Snapshot.SchemaSections {
				if i >= len(baseline.Snapshot.SchemaSections) || section != baseline.Snapshot.SchemaSections[i] {
					changed = append(changed, i)
				}
			}
			return fmt.Errorf("target schema differs from initialized baseline (sections %v)", changed)
		}
	}
	var output any
	switch command {
	case "init":
		if *authShim == "only" {
			err = r.InitializeAuthPrerequisites(ctx, pool)
			output = map[string]any{"status": "passed", "scope": "local-auth-prerequisites-only", "target": target}
		} else if *authShim == "full" {
			output, err = r.InitializeFullAuthShim(ctx, pool)
		} else {
			output, err = r.InitializeSynthetic(ctx, pool)
		}
	case "preflight":
		output, err = r.Inspect(ctx, pool)
	case "apply":
		var plan *r.Plan
		plan, err = r.LoadPlan(*planPath)
		if err != nil {
			break
		}
		if *confirm != plan.BatchID {
			return errors.New("apply requires exact confirm-batch")
		}
		output, err = r.Apply(ctx, pool, plan)
	case "reconcile":
		var plan *r.Plan
		plan, err = r.LoadPlan(*planPath)
		if err != nil {
			break
		}
		var receipt *r.Result
		receipt, err = r.LoadResult(*receiptPath)
		if err != nil {
			break
		}
		output, err = r.Reconcile(ctx, pool, plan, receipt)
	case "rollback":
		var receipt *r.Result
		receipt, err = r.LoadResult(*receiptPath)
		if err != nil {
			break
		}
		output, err = r.Rollback(ctx, pool, receipt, *confirm)
	}
	if err != nil {
		return err
	}
	if err = r.WriteJSONAtomic(*out, output, false); err != nil {
		return errors.New("result write failed; a transaction may already be committed: preserve baseline and recover using isolated restore, never infer rollback ownership from a retry")
	}
	return nil
}
func readJSON(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil {
		return err
	}
	if len(b) > 8<<20 {
		return errors.New("input exceeds limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		return err
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
func fixture(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	m, rows := r.SyntheticFixture()
	var body bytes.Buffer
	for _, row := range rows {
		b, err := r.CanonicalJSON(row)
		if err != nil {
			return err
		}
		body.Write(b)
		body.WriteByte('\n')
	}
	path := filepath.Join(dir, "rows.ndjson")
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		return err
	}
	sum := sha256.Sum256(body.Bytes())
	m.Files = []r.SourceFile{{Path: "rows.ndjson", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(body.Len()), Rows: len(rows)}}
	return r.WriteJSONAtomic(filepath.Join(dir, "manifest.json"), m, false)
}
