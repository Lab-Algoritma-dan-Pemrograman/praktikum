package main

import (
	"encoding/json"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
)

// stateSnapshot emits only aggregate counts/content hashes, never rows. The
// exclusion set mirrors the sanitized public backup; it is not full Auth or
// ephemeral-session recovery. No migration/DDL is executed.
func stateSnapshot(c *config) (map[string]any, error) {
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='60s';set local lock_timeout='5s';set local timezone='UTC'`); err != nil {
		return nil, err
	}
	excluded := map[string]bool{"auth_sessions": true, "sso_handshakes": true, "auth_rate_limits": true, "audit_logs": true, "academic_notifications": true, "practicum_operations": true, "learning_operations": true, "academic_operations": true, "practicum_runner_leases": true, "learning_runner_leases": true}
	rows, err := tx.Query(c.ctx, `select c.relname from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relkind='r' order by c.relname`)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		if !excluded[name] {
			names = append(names, name)
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	tables := []map[string]any{}
	for _, name := range names {
		var count int64
		var hash string
		q := `select count(*),encode(digest(coalesce(string_agg(to_jsonb(t)::text,E'\n' order by to_jsonb(t)::text),''),'sha256'),'hex') from ` + pgx.Identifier{"public", name}.Sanitize() + ` t`
		if err = tx.QueryRow(c.ctx, q).Scan(&count, &hash); err != nil {
			return nil, errors.New("snapshot fingerprint failed: " + name)
		}
		tables = append(tables, map[string]any{"table": name, "count": count, "sha256": hash})
	}
	// Auth's stable identifying fields are hashed without password/token reads.
	var authCount int64
	var authHash string
	err = tx.QueryRow(c.ctx, `select count(*),encode(digest(coalesce(string_agg(jsonb_build_array(id,email,raw_user_meta_data->>'phase6_batch')::text,E'\n' order by id),''),'sha256'),'hex') from auth.users`).Scan(&authCount, &authHash)
	if err != nil {
		return nil, errors.New("auth metadata fingerprint failed")
	}
	var grants int
	if err = tx.QueryRow(c.ctx, `select count(*) from user_permission_grants`).Scan(&grants); err != nil {
		return nil, err
	}
	sequences := []map[string]any{}
	seqRows, err := tx.Query(c.ctx, `select sequencename from pg_sequences where schemaname='public' order by sequencename`)
	if err != nil {
		return nil, err
	}
	seqNames := []string{}
	for seqRows.Next() {
		var name string
		if err = seqRows.Scan(&name); err != nil {
			seqRows.Close()
			return nil, err
		}
		seqNames = append(seqNames, name)
	}
	seqRows.Close()
	if seqRows.Err() != nil {
		return nil, seqRows.Err()
	}
	for _, name := range seqNames {
		var last int64
		var called bool
		if err = tx.QueryRow(c.ctx, `select last_value,is_called from `+pgx.Identifier{"public", name}.Sanitize()).Scan(&last, &called); err != nil {
			return nil, err
		}
		sequences = append(sequences, map[string]any{"name": name, "last_value": last, "is_called": called})
	}
	// Schema evidence is object-based (no OIDs/owner role), stable across a
	// pg_dump restore and different database identities.
	// PostgreSQL18 represents NOT NULL in pg_constraint (contype=n), while
	// PostgreSQL17 only exposes attnotnull. It is already included in column
	// fingerprints, so omit the redundant version-specific constraint object.
	schemaParts := []string{}
	schemaSections := []string{}
	schemaQueries := []string{
		`select jsonb_build_array(c.relname,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,pg_get_expr(d.adbin,d.adrelid)) from pg_attribute a join pg_class c on c.oid=a.attrelid join pg_namespace n on n.oid=c.relnamespace left join pg_attrdef d on d.adrelid=a.attrelid and d.adnum=a.attnum where n.nspname='public' and c.relkind='r' and a.attnum>0 and not a.attisdropped`,
		`select jsonb_build_array(c.relname,k.conname,k.convalidated,pg_get_constraintdef(k.oid,true)) from pg_constraint k join pg_class c on c.oid=k.conrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and k.contype<>'n'`,
		`select jsonb_build_array(c.relname,t.tgname,t.tgenabled,pg_get_triggerdef(t.oid)) from pg_trigger t join pg_class c on c.oid=t.tgrelid join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and not t.tgisinternal`,
		`select jsonb_build_array(c.relname,c.relrowsecurity,c.relforcerowsecurity) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relkind='r'`,
		`select to_jsonb(p) from pg_policies p where schemaname='public'`,
		`select jsonb_build_array(tablename,indexname,indexdef) from pg_indexes where schemaname='public'`,
	}
	for _, q := range schemaQueries {
		section := []string{}
		r, err := tx.Query(c.ctx, q)
		if err != nil {
			return nil, errors.New("schema snapshot failed")
		}
		for r.Next() {
			var raw []byte
			if err = r.Scan(&raw); err != nil {
				r.Close()
				return nil, err
			}
			section = append(section, digest(json.RawMessage(raw)))
		}
		r.Close()
		if r.Err() != nil {
			return nil, r.Err()
		}
		sort.Strings(section)
		schemaSections = append(schemaSections, digest(section))
		schemaParts = append(schemaParts, section...)
	}
	sort.Strings(schemaParts)
	sort.Strings(names)
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	result := map[string]any{"scope": "sanitized_public_state", "target_fingerprint": c.fingerprint, "tables": tables, "sequences": sequences, "schema_sha256": digest(schemaParts), "schema_sections": schemaSections, "auth_users_metadata": map[string]any{"count": authCount, "sha256": authHash}, "explicit_grants": grants}
	b, _ := json.Marshal(tables)
	result["public_digest"] = digest(json.RawMessage(b))
	return result, nil
}
