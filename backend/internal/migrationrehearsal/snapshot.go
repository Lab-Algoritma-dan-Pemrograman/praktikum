package migrationrehearsal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LocalTarget is a harness-owned cluster, not a general database destination.
// The database comment is set by the harness immediately after CREATE DATABASE.
type LocalTarget struct {
	RunID            string `json:"run_id"`
	Database         string `json:"database"`
	DataDirectory    string `json:"data_directory"`
	SystemIdentifier string `json:"system_identifier"`
	Port             uint16 `json:"port"`
}
type targetContextKey struct{}

func WithLocalTarget(ctx context.Context, target LocalTarget) context.Context {
	return context.WithValue(ctx, targetContextKey{}, target)
}
func verifyTarget(ctx context.Context, pool *pgxpool.Pool) (LocalTarget, error) {
	target, ok := ctx.Value(targetContextKey{}).(LocalTarget)
	if !ok || !batchPattern.MatchString(target.RunID) || !strings.HasSuffix(target.Database, "_test") || target.DataDirectory == "" || target.SystemIdentifier == "" || target.Port == 0 {
		return target, errors.New("owned local target is required")
	}
	c := pool.Config().ConnConfig
	if c.Host != "127.0.0.1" || net.ParseIP(c.Host) == nil || c.Port != target.Port || c.Database != target.Database || c.TLSConfig != nil || len(c.Fallbacks) != 0 {
		return target, errors.New("connection is not the owned literal-loopback target")
	}
	var db, dir, system, comment string
	err := pool.QueryRow(ctx, `select current_database(),current_setting('data_directory'),(select system_identifier::text from pg_control_system()),coalesce(shobj_description(oid,'pg_database'),'') from pg_database where datname=current_database()`).Scan(&db, &dir, &system, &comment)
	if err != nil {
		return target, errors.New("cannot verify owned target identity")
	}
	if db != target.Database || system != target.SystemIdentifier || !strings.EqualFold(filepath.Clean(dir), filepath.Clean(target.DataDirectory)) || comment != "algohub-phase6:"+target.RunID {
		return target, errors.New("owned target identity mismatch")
	}
	return target, nil
}

func VerifyLocalTarget(ctx context.Context, pool *pgxpool.Pool, target LocalTarget) error {
	_, err := verifyTarget(WithLocalTarget(ctx, target), pool)
	return err
}

var publicTables = []string{
	"academic_attendance_records", "academic_attendance_sessions", "academic_classes", "academic_inventory_items", "academic_inventory_rentals", "academic_notifications", "academic_operations", "academic_schedules",
	"audit_logs", "auth_rate_limits", "auth_sessions", "division_permissions", "divisions",
	"learning_bug_hunt", "learning_game_history", "learning_lessons", "learning_levels", "learning_modules", "learning_operations", "learning_profiles", "learning_progress", "learning_runner_leases",
	"permissions", "practicum_activations", "practicum_attempts", "practicum_courses", "practicum_grading_jobs", "practicum_operations", "practicum_questions", "practicum_runner_leases", "practicum_sessions",
	"role_permissions", "roles", "schema_migrations", "sso_handshakes", "user_permission_grants", "user_role_assignments", "users",
}

type TableSnapshot struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Digest string `json:"sha256"`
}
type SequenceSnapshot struct {
	Name      string `json:"name"`
	LastValue int64  `json:"last_value"`
	IsCalled  bool   `json:"is_called"`
}
type Snapshot struct {
	Tables         []TableSnapshot    `json:"tables"`
	Sequences      []SequenceSnapshot `json:"sequences"`
	SchemaDigest   string             `json:"schema_sha256"`
	SchemaSections []string           `json:"schema_sections"`
	Digest         string             `json:"sha256"`
}
type Preflight struct {
	Target   LocalTarget `json:"target"`
	Snapshot Snapshot    `json:"snapshot"`
	Checks   []Check     `json:"checks"`
}

func boundedRollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
func setDeadlines(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `set local statement_timeout='30s'; set local lock_timeout='5s'; set local idle_in_transaction_session_timeout='45s'; set local timezone='UTC'`)
	return err
}
func lockTables(ctx context.Context, tx pgx.Tx) error {
	names := []string{`auth.users`}
	for _, name := range publicTables {
		names = append(names, pgx.Identifier{"public", name}.Sanitize())
	}
	_, err := tx.Exec(ctx, "lock table "+strings.Join(names, ",")+" in share row exclusive mode")
	return err
}

// Inspect only reads metadata and aggregate hashes. It never initializes schema.
func Inspect(ctx context.Context, pool *pgxpool.Pool) (*Preflight, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	target, err := verifyTarget(ctx, pool)
	if err != nil {
		return nil, err
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer boundedRollback(tx)
	if err = setDeadlines(ctx, tx); err != nil {
		return nil, err
	}
	checks, err := schemaChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	snapshot, err := captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Preflight{Target: target, Snapshot: snapshot, Checks: checks}, nil
}

func schemaChecks(ctx context.Context, tx pgx.Tx) ([]Check, error) {
	if err := verifyMigrationHistory(ctx, tx, migrations.Catalog()); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `select c.relname,c.relrowsecurity from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relkind in ('r','p') order by c.relname`)
	if err != nil {
		return nil, err
	}
	actual := []string{}
	for rows.Next() {
		var name string
		var rls bool
		if err = rows.Scan(&name, &rls); err != nil {
			rows.Close()
			return nil, err
		}
		if !rls {
			rows.Close()
			return nil, errors.New("public table without RLS")
		}
		actual = append(actual, name)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if strings.Join(actual, ",") != strings.Join(publicTables, ",") {
		return nil, errors.New("public table set differs from pinned schema")
	}
	var invalid int
	err = tx.QueryRow(ctx, `select count(*) from pg_constraint c join pg_namespace n on n.oid=c.connamespace where n.nspname in ('public','auth') and not c.convalidated`).Scan(&invalid)
	if err != nil || invalid != 0 {
		return nil, errors.New("unvalidated constraint")
	}
	err = tx.QueryRow(ctx, `select count(*) from pg_index i join pg_class c on c.oid=i.indrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth') and (not i.indisvalid or not i.indisready)`).Scan(&invalid)
	if err != nil || invalid != 0 {
		return nil, errors.New("invalid index")
	}
	err = tx.QueryRow(ctx, `select count(*) from pg_trigger t join pg_class c on c.oid=t.tgrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and t.tgenabled <> 'O'`).Scan(&invalid)
	if err != nil || invalid != 0 {
		return nil, errors.New("disabled or altered trigger")
	}
	err = tx.QueryRow(ctx, `select count(*) from pg_roles where rolname in ('anon','authenticated') and (rolsuper or rolbypassrls or rolcanlogin)`).Scan(&invalid)
	if err != nil || invalid != 0 {
		return nil, errors.New("unsafe browser roles")
	}
	checks := []Check{{Name: "migration_history", Status: "passed", Reason: ReasonCheckPassed, Observed: len(migrations.Catalog()), Expected: len(migrations.Catalog())}, {Name: "exact_public_table_set_and_rls", Status: "passed", Reason: ReasonCheckPassed, Observed: len(actual), Expected: 38}}
	fkChecks, err := foreignKeyChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	checks = append(checks, fkChecks...)
	uniqueChecks, err := uniqueKeyChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	checks = append(checks, uniqueChecks...)
	semantic, err := semanticChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	checks = append(checks, semantic...)
	return checks, nil
}

func captureSnapshot(ctx context.Context, tx pgx.Tx) (Snapshot, error) {
	snapshot := Snapshot{}
	// Object descriptions omit OIDs so restore into another database is comparable.
	// Normalize implicit/default ACLs and use PostgreSQL's pretty deparser:
	// pg_dump can re-associate equivalent AND nodes and omit owner-only ACLs.
	// Names, operators, values, privileges, validation and RLS remain fingerprinted.
	schemaSQL := []string{
		`select jsonb_build_array(n.nspname,c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,a.attgenerated,pg_get_expr(d.adbin,d.adrelid)) from pg_attribute a join pg_class c on c.oid=a.attrelid join pg_namespace n on n.oid=c.relnamespace left join pg_attrdef d on d.adrelid=a.attrelid and d.adnum=a.attnum where n.nspname in ('public','auth') and c.relkind in ('r','p') and a.attnum>0 and not a.attisdropped`,
		`select jsonb_build_array(n.nspname,c.relname,k.conname,k.contype,k.convalidated,pg_get_constraintdef(k.oid,true)) from pg_constraint k join pg_class c on c.oid=k.conrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth')`,
		`select jsonb_build_array(n.nspname,c.relname,t.tgname,t.tgenabled,pg_get_triggerdef(t.oid)) from pg_trigger t join pg_class c on c.oid=t.tgrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth') and not t.tgisinternal`,
		`select jsonb_build_array(n.nspname,c.relname,c.relrowsecurity,c.relforcerowsecurity,array(select x::text from unnest(coalesce(c.relacl,acldefault(case when c.relkind='S' then 's'::"char" else 'r'::"char" end,c.relowner))) x order by x::text)) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth') and c.relkind in ('r','p','S')`,
		`select to_jsonb(p) from pg_policies p where schemaname in ('public','auth')`,
		`select jsonb_build_array(schemaname,tablename,indexname,indexdef) from pg_indexes where schemaname in ('public','auth')`,
		`select jsonb_build_array(n.nspname,p.proname,pg_get_function_identity_arguments(p.oid),pg_get_functiondef(p.oid),array(select x::text from unnest(coalesce(p.proacl,acldefault('f'::"char",p.proowner))) x order by x::text)) from pg_proc p join pg_namespace n on n.oid=p.pronamespace where n.nspname in ('public','auth') and p.prokind='f'`,
	}
	schemaParts := []string{}
	for _, q := range schemaSQL {
		parts, err := queryDigests(ctx, tx, q)
		if err != nil {
			return snapshot, err
		}
		sort.Strings(parts)
		snapshot.SchemaSections = append(snapshot.SchemaSections, digestValue(parts))
		schemaParts = append(schemaParts, parts...)
	}
	sort.Strings(schemaParts)
	snapshot.SchemaDigest = digestValue(schemaParts)
	for _, name := range append(append([]string{}, publicTables...), "auth.users") {
		ident := pgx.Identifier{"public", name}
		if name == "auth.users" {
			ident = pgx.Identifier{"auth", "users"}
		}
		hashes, err := queryDigests(ctx, tx, "select to_jsonb(t) from "+ident.Sanitize()+" t")
		if err != nil {
			return snapshot, err
		}
		sort.Strings(hashes)
		snapshot.Tables = append(snapshot.Tables, TableSnapshot{Name: name, Count: len(hashes), Digest: digestValue(hashes)})
	}
	rows, err := tx.Query(ctx, `select sequencename from pg_sequences where schemaname='public' order by sequencename`)
	if err != nil {
		return snapshot, err
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return snapshot, err
		}
		names = append(names, name)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return snapshot, err
	}
	for _, name := range names {
		s := SequenceSnapshot{Name: name}
		if err = tx.QueryRow(ctx, "select last_value,is_called from "+pgx.Identifier{"public", name}.Sanitize()).Scan(&s.LastValue, &s.IsCalled); err != nil {
			return snapshot, err
		}
		snapshot.Sequences = append(snapshot.Sequences, s)
	}
	snapshot.Digest = digestValue(snapshot)
	return snapshot, nil
}
func queryDigests(ctx context.Context, tx pgx.Tx, query string) ([]string, error) {
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hashes := []string{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		canonical, err := ParseCanonicalJSON(raw)
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, digestValue(jsonRaw(canonical)))
	}
	return hashes, rows.Err()
}
func sameRows(a, b Snapshot) bool {
	return a.SchemaDigest == b.SchemaDigest && digestValue(a.Tables) == digestValue(b.Tables)
}

// CheckRelationalIntegrity runs read-only checks for every declared FK and
// unique index. It can be reused on the live target without calling the local
// initializer or the synthetic-only XP policy checks.
func CheckRelationalIntegrity(ctx context.Context, tx pgx.Tx) ([]Check, error) {
	fk, err := foreignKeyChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	unique, err := uniqueKeyChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	return append(fk, unique...), nil
}

func foreignKeyChecks(ctx context.Context, tx pgx.Tx) ([]Check, error) {
	rows, err := tx.Query(ctx, `select c.conname,ns.nspname,t.relname,ps.nspname,p.relname,
 array(select a.attname from unnest(c.conkey) with ordinality k(num,ord) join pg_attribute a on a.attrelid=c.conrelid and a.attnum=k.num order by k.ord),
 array(select a.attname from unnest(c.confkey) with ordinality k(num,ord) join pg_attribute a on a.attrelid=c.confrelid and a.attnum=k.num order by k.ord)
 from pg_constraint c join pg_class t on t.oid=c.conrelid join pg_namespace ns on ns.oid=t.relnamespace join pg_class p on p.oid=c.confrelid join pg_namespace ps on ps.oid=p.relnamespace where c.contype='f' and ns.nspname in ('public','auth') order by ns.nspname,t.relname,c.conname`)
	if err != nil {
		return nil, err
	}
	type fk struct {
		name, ns, table, pns, parent string
		cols, pcols                  []string
	}
	keys := []fk{}
	for rows.Next() {
		var k fk
		if err = rows.Scan(&k.name, &k.ns, &k.table, &k.pns, &k.parent, &k.cols, &k.pcols); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	checks := []Check{}
	for _, k := range keys {
		nonnull, joins := []string{}, []string{}
		for i, c := range k.cols {
			cq := "c." + pgx.Identifier{c}.Sanitize()
			pq := "p." + pgx.Identifier{k.pcols[i]}.Sanitize()
			nonnull = append(nonnull, cq+" is not null")
			joins = append(joins, cq+"="+pq)
		}
		q := "select count(*) from " + pgx.Identifier{k.ns, k.table}.Sanitize() + " c where " + strings.Join(nonnull, " and ") + " and not exists (select 1 from " + pgx.Identifier{k.pns, k.parent}.Sanitize() + " p where " + strings.Join(joins, " and ") + ")"
		var n int
		if err = tx.QueryRow(ctx, q).Scan(&n); err != nil {
			return nil, err
		}
		if n != 0 {
			return nil, errors.New("foreign key orphan detected")
		}
		checks = append(checks, Check{Name: "fk:" + k.table + ":" + k.name, Status: "passed", Reason: ReasonCheckPassed})
	}
	return checks, nil
}
func uniqueKeyChecks(ctx context.Context, tx pgx.Tx) ([]Check, error) {
	rows, err := tx.Query(ctx, `select t.relname,i.relname,ix.indnullsnotdistinct,coalesce(pg_get_expr(ix.indpred,ix.indrelid),'true'),array(select pg_get_indexdef(ix.indexrelid,k,true) from generate_series(1,ix.indnkeyatts) k) from pg_index ix join pg_class t on t.oid=ix.indrelid join pg_class i on i.oid=ix.indexrelid join pg_namespace n on n.oid=t.relnamespace where n.nspname='public' and ix.indisunique order by t.relname,i.relname`)
	if err != nil {
		return nil, err
	}
	type idx struct {
		table, name string
		nulls       bool
		predicate   string
		exprs       []string
	}
	indexes := []idx{}
	for rows.Next() {
		var i idx
		if err = rows.Scan(&i.table, &i.name, &i.nulls, &i.predicate, &i.exprs); err != nil {
			rows.Close()
			return nil, err
		}
		indexes = append(indexes, i)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	checks := []Check{}
	for _, i := range indexes {
		where := []string{"(" + i.predicate + ")"}
		if !i.nulls {
			for _, expr := range i.exprs {
				where = append(where, "("+expr+") is not null")
			}
		}
		q := "select count(*) from (select 1 from " + pgx.Identifier{"public", i.table}.Sanitize() + " where " + strings.Join(where, " and ") + " group by " + strings.Join(i.exprs, ",") + " having count(*)>1) duplicate_keys"
		var n int
		if err = tx.QueryRow(ctx, q).Scan(&n); err != nil {
			return nil, fmt.Errorf("unique check %s: %w", i.name, err)
		}
		if n != 0 {
			return nil, errors.New("duplicate business key detected")
		}
		checks = append(checks, Check{Name: "unique:" + i.name, Status: "passed", Reason: ReasonCheckPassed})
	}
	return checks, nil
}
