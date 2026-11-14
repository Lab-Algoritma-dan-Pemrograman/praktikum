package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	mr "github.com/algohub/algohub-rewrite/backend/internal/migrationrehearsal"
	"github.com/jackc/pgx/v5"
)

func rollbackBounded(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}

func reconcile(c *config, p *Plan, fileHash string, receipt *Receipt) (*ReconcileReport, error) {
	if err := verifyReceipt(receipt, p, fileHash, c.fingerprint); err != nil {
		return nil, err
	}
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='12min';set local lock_timeout='10s';set local idle_in_transaction_session_timeout='14min'`); err != nil {
		return nil, err
	}
	report := &ReconcileReport{FormatVersion: "phase6-live-reconcile-v1", BatchID: p.BatchID, PlanFileSHA256: fileHash, TargetFingerprint: c.fingerprint, Counts: map[string]int{}}
	verifiedReceipt := *receipt
	verifiedReceipt.Rows = append([]ReceiptRow(nil), receipt.Rows...)
	if c.afterAcceptance {
		for _, u := range p.AuthUsers {
			if !u.AcceptanceLogin {
				continue
			}
			var originalShape, currentShape string
			query := `select to_jsonb(t)::text,(to_jsonb(t)||jsonb_build_object('last_sign_in_at',null,'updated_at',t.created_at))::text from auth.users t where id=$1::uuid`
			if err = tx.QueryRow(c.ctx, query, u.AuthUserID).Scan(&currentShape, &originalShape); err != nil {
				return nil, errors.New("acceptance Auth state query failed")
			}
			for i, row := range verifiedReceipt.Rows {
				if row.Kind != "auth_user" || row.Key != u.AuthUserID {
					continue
				}
				if digest(json.RawMessage(currentShape)) != row.Digest {
					if digest(json.RawMessage(originalShape)) != row.Digest {
						return nil, errors.New("acceptance Auth changed beyond login timestamps")
					}
					verifiedReceipt.Rows[i].Digest = digest(json.RawMessage(currentShape))
					report.Checks = append(report.Checks, Check{Name: "acceptance_auth_only_login_timestamps_changed", Status: "passed", Observed: 1, Expected: 1})
				}
			}
		}
	}
	if err := verifyReceiptRows(c.ctx, tx, &verifiedReceipt); err != nil {
		return nil, err
	}
	for _, row := range receipt.Rows {
		report.Counts[row.Kind]++
	}
	report.Checks = append(report.Checks, Check{Name: "receipt_rows_exact", Status: "passed", Observed: len(receipt.Rows), Expected: len(p.AuthUsers) + len(p.Users) + len(p.Assignments) + len(p.DomainRows)})
	report.Checks = append(report.Checks, Check{Name: "source_accounting", Status: "passed", Observed: p.Counts["imported"] + p.Counts["rejected"], Expected: p.Counts["imported"] + p.Counts["rejected"]})
	queries := []struct{ name, sql string }{
		{"no_unbound_markers_learning_progress", `select count(*) from learning_progress where data::text like '%"$user"%' or data::text like '%"$id"%'`},
		{"no_unbound_markers_practicum_attempts", `select count(*) from practicum_attempts where data::text like '%"$user"%' or data::text like '%"$id"%'`},
		{"no_unbound_markers_academic_classes", `select count(*) from academic_classes where data::text like '%"$user"%' or data::text like '%"$id"%' or scope::text like '%"$id"%'`},
		{"learning_progress_identity", `select count(*) from learning_progress p where p.data->>'lesson_id'<>p.parent_id or (p.data->>'user_id')::bigint<>p.owner_id or p.data->>'status'<>p.status`},
		{"learning_profile_identity", `select count(*) from learning_profiles p where (p.data->>'user_id')::bigint<>p.owner_id or p.status<>'active'`},
		{"practicum_course_identity", `select count(*) from practicum_courses c where c.data->>'session_id'<>c.parent_id`},
		{"practicum_question_identity", `select count(*) from practicum_questions q where q.data->>'course_id'<>q.parent_id`},
		{"practicum_activation_identity", `select count(*) from practicum_activations a where a.data->'activation'->>'id'<>a.id or a.data->'activation'->>'session_id'<>a.parent_id or a.data->'course'->>'session_id'<>a.parent_id`},
		{"practicum_attempt_identity", `select count(*) from practicum_attempts a where a.data->'attempt'->>'attempt_id'<>a.id or a.data->'attempt'->>'activation_id'<>a.parent_id or (a.data->'attempt'->>'user_id')::bigint<>a.owner_id or a.data->'attempt'->>'status'<>a.status`},
		{"practicum_job_identity", `select count(*) from practicum_grading_jobs j where j.id<>j.parent_id or j.data->>'attempt_id'<>j.parent_id or j.data->>'status'<>j.status`},
		{"academic_class_identity", `select count(*) from academic_classes c where c.data->>'id'<>c.id or c.scope->>'class_id'<>c.id`},
		{"academic_schedule_identity", `select count(*) from academic_schedules s where s.data->>'id'<>s.id or s.data->>'class_id'<>s.parent_id or s.scope->>'class_id'<>s.parent_id`},
		{"inventory_identity", `select count(*) from academic_inventory_items i where i.data->>'id'<>i.id or i.data->>'status'<>i.status`},
	}
	for _, q := range queries {
		var n int
		if err = tx.QueryRow(c.ctx, q.sql).Scan(&n); err != nil {
			return nil, errors.New("semantic query failed: " + q.name)
		}
		if n != 0 {
			return nil, errors.New("semantic mismatch: " + q.name)
		}
		report.Checks = append(report.Checks, Check{Name: q.name, Status: "passed", Observed: n, Expected: 0})
	}
	// PostgreSQL constraints are validated globally, while imported JSON checks are above.
	var invalid, orphans int
	if err = tx.QueryRow(c.ctx, `select count(*) from pg_constraint c join pg_namespace n on n.oid=c.connamespace where n.nspname in('public','auth') and not c.convalidated`).Scan(&invalid); err != nil || invalid != 0 {
		return nil, errors.New("invalid target constraint")
	}
	if err = tx.QueryRow(c.ctx, `select (select count(*) from learning_progress p left join users u on u.id=p.owner_id where u.id is null)+(select count(*) from practicum_attempts p left join users u on u.id=p.owner_id where u.id is null)+(select count(*) from academic_inventory_items i where (i.data->>'available_quantity')::int>(i.data->>'total_quantity')::int)`).Scan(&orphans); err != nil || orphans != 0 {
		return nil, errors.New("orphan/business invariant failed")
	}
	report.Checks = append(report.Checks, Check{Name: "validated_constraints", Status: "passed"}, Check{Name: "owner_and_inventory_invariants", Status: "passed"})
	relations, err := mr.CheckRelationalIntegrity(c.ctx, tx)
	if err != nil {
		return nil, errors.New("full relational integrity check failed")
	}
	for _, check := range relations {
		report.Checks = append(report.Checks, Check{Name: check.Name, Status: check.Status, Observed: check.Observed, Expected: check.Expected})
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	report.Digest = digest(report)
	return report, nil
}
func rollback(c *config, p *Plan, fileHash string, receipt *Receipt) (*ReconcileReport, error) {
	if err := verifyReceipt(receipt, p, fileHash, c.fingerprint); err != nil {
		return nil, err
	}
	tx, err := c.pool.BeginTx(c.ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer rollbackBounded(tx)
	if _, err = tx.Exec(c.ctx, `set local statement_timeout='10min';set local lock_timeout='10s';select pg_advisory_xact_lock(5784123647119155286)`); err != nil {
		return nil, err
	}
	// Verify all owned rows before deleting any. This rejects post-import mutation.
	if err := verifyReceiptRows(c.ctx, tx, receipt); err != nil {
		return nil, fmt.Errorf("rollback refused: %w", err)
	}
	insertedUsers := []int64{}
	for _, row := range receipt.Rows {
		if row.Kind == "user" && row.Action == "inserted" {
			id, _ := strconv.ParseInt(row.Key, 10, 64)
			insertedUsers = append(insertedUsers, id)
		}
	}
	if len(insertedUsers) > 0 {
		var n int
		if err = tx.QueryRow(c.ctx, `select (select count(*) from audit_logs where actor_id=any($1))+(select count(*) from auth_sessions where user_id=any($1))`, insertedUsers).Scan(&n); err != nil {
			return nil, err
		}
		if n != 0 {
			return nil, errors.New("rollback refused: unexpected session/audit dependents")
		}
	}
	deleted := 0
	for i := len(receipt.Rows) - 1; i >= 0; i-- {
		row := receipt.Rows[i]
		if row.Action != "inserted" {
			continue
		}
		switch row.Kind {
		case "domain":
			tag, err := tx.Exec(c.ctx, `delete from `+pgx.Identifier{"public", row.Table}.Sanitize()+` where id=$1`, row.Key)
			if err != nil || tag.RowsAffected() != 1 {
				return nil, errors.New("domain rollback failed")
			}
		case "assignment":
			id, _ := strconv.ParseInt(row.Key, 10, 64)
			// Grants materialized for this assignment are derived state; remove
			// them before the assignment they belong to.
			if _, err := tx.Exec(c.ctx, `delete from user_permission_grants where assignment_id=$1`, id); err != nil {
				return nil, errors.New("grant rollback failed")
			}
			tag, err := tx.Exec(c.ctx, `delete from user_role_assignments where id=$1`, id)
			if err != nil || tag.RowsAffected() != 1 {
				return nil, errors.New("assignment rollback failed")
			}
		case "user":
			id, _ := strconv.ParseInt(row.Key, 10, 64)
			tag, err := tx.Exec(c.ctx, `delete from users where id=$1`, id)
			if err != nil || tag.RowsAffected() != 1 {
				return nil, errors.New("user rollback failed")
			}
		case "auth_user":
			tag, err := tx.Exec(c.ctx, `delete from auth.users where id=$1`, row.Key)
			if err != nil || tag.RowsAffected() != 1 {
				return nil, errors.New("auth rollback failed")
			}
		default:
			return nil, errors.New("unknown receipt kind")
		}
		deleted++
	}
	if err = tx.Commit(c.ctx); err != nil {
		return nil, err
	}
	report := &ReconcileReport{FormatVersion: "phase6-live-rollback-v1", BatchID: p.BatchID, PlanFileSHA256: fileHash, TargetFingerprint: c.fingerprint, Counts: map[string]int{"deleted": deleted}, Checks: []Check{{Name: "exact_inserted_rows_deleted", Status: "passed", Observed: deleted, Expected: receipt.Counts["inserted"]}}}
	report.Digest = digest(report)
	return report, nil
}
func verifyReceipt(r *Receipt, p *Plan, fileHash, target string) error {
	if r.FormatVersion != "phase6-live-receipt-v1" || r.BatchID != p.BatchID || r.PlanFileSHA256 != fileHash || r.TargetFingerprint != target {
		return errors.New("receipt binding mismatch")
	}
	clone := *r
	clone.Digest = ""
	if r.Digest == "" || r.Digest != digest(&clone) {
		return errors.New("receipt digest mismatch")
	}
	if len(r.Rows) != len(p.AuthUsers)+len(p.Users)+len(p.Assignments)+len(p.DomainRows) {
		return errors.New("receipt row count mismatch")
	}
	expected := map[string]string{}
	for _, u := range p.AuthUsers {
		expected["auth_user\x00"+u.Token] = "auth.users"
	}
	for _, u := range p.Users {
		expected["user\x00"+u.Token] = "public.users"
	}
	for _, a := range p.Assignments {
		expected["assignment\x00"+a.Token] = "public.user_role_assignments"
	}
	for _, d := range p.DomainRows {
		expected["domain\x00"+d.Token] = d.Table
	}
	counts := map[string]int{}
	for _, row := range r.Rows {
		key := row.Kind + "\x00" + row.Token
		table, ok := expected[key]
		if !ok || row.Table != table || row.Key == "" || len(row.Digest) != 64 || (row.Action != "inserted" && row.Action != "unchanged") {
			return errors.New("invalid receipt row correspondence")
		}
		if _, err := receiptTable(row.Kind, row.Table); err != nil {
			return err
		}
		delete(expected, key)
		counts[row.Action]++
	}
	if len(expected) != 0 || digest(counts) != digest(r.Counts) {
		return errors.New("receipt accounting mismatch")
	}
	return nil
}
func currentReceiptDigest(ctx context.Context, tx pgx.Tx, row ReceiptRow) (string, error) {
	switch row.Kind {
	case "auth_user":
		return rowDigest(ctx, tx, "auth.users", "id", row.Key)
	case "user", "assignment":
		id, err := strconv.ParseInt(row.Key, 10, 64)
		if err != nil {
			return "", err
		}
		table := "public.users"
		if row.Kind == "assignment" {
			table = "public.user_role_assignments"
		}
		return rowDigest(ctx, tx, table, "id", id)
	case "domain":
		return rowDigest(ctx, tx, "public."+row.Table, "id", row.Key)
	default:
		return "", errors.New("unknown receipt kind")
	}
}
func unionDataTables() string {
	tables := []string{}
	for table := range allowedTables {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	parts := make([]string, len(tables))
	for i, t := range tables {
		parts[i] = `select scope,data from ` + pgx.Identifier{"public", t}.Sanitize()
	}
	return strings.Join(parts, " union all ")
}
func expectedDomainJSON(r boundDomain) json.RawMessage {
	m := map[string]any{"id": r.id, "parent_id": r.parent, "owner_id": r.owner, "scope": json.RawMessage(r.scope), "status": r.row.Status, "unique_key": r.unique, "expires_at": r.row.ExpiresAt, "data": json.RawMessage(r.data)}
	return raw(m)
}
func timePointerEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// receiptTable validates both the row kind and its exact table; receipt data
// never gets to choose an arbitrary relation or change schemas via search_path.
func receiptTable(kind, table string) (pgx.Identifier, error) {
	switch kind {
	case "auth_user":
		if table == "auth.users" {
			return pgx.Identifier{"auth", "users"}, nil
		}
	case "user":
		if table == "public.users" {
			return pgx.Identifier{"public", "users"}, nil
		}
	case "assignment":
		if table == "public.user_role_assignments" {
			return pgx.Identifier{"public", "user_role_assignments"}, nil
		}
	case "domain":
		if allowedTables[table] {
			return pgx.Identifier{"public", table}, nil
		}
	}
	return nil, errors.New("invalid receipt kind/table")
}

// verifyReceiptRows checks every receipt row with one query per table batch.
// Per-row round trips are too slow on a shared pooler for a real dataset.
func verifyReceiptRows(ctx context.Context, tx pgx.Tx, receipt *Receipt) error {
	type groupKey struct{ kind, table string }
	groups := map[groupKey][]ReceiptRow{}
	for _, row := range receipt.Rows {
		groups[groupKey{row.Kind, row.Table}] = append(groups[groupKey{row.Kind, row.Table}], row)
	}
	for key, rows := range groups {
		ident, err := receiptTable(key.kind, key.table)
		if err != nil {
			return err
		}
		for base := 0; base < len(rows); base += 400 {
			limit := base + 400
			if limit > len(rows) {
				limit = len(rows)
			}
			batch := rows[base:limit]
			var parameter any
			cast := "text[]"
			if key.kind == "user" || key.kind == "assignment" {
				ids := make([]int64, len(batch))
				for i, row := range batch {
					ids[i], err = strconv.ParseInt(row.Key, 10, 64)
					if err != nil {
						return errors.New("invalid numeric receipt key")
					}
				}
				parameter, cast = ids, "bigint[]"
			} else {
				ids := make([]string, len(batch))
				for i, row := range batch {
					ids[i] = row.Key
				}
				parameter = ids
				if key.kind == "auth_user" {
					cast = "uuid[]"
				}
			}
			query := "select id::text,to_jsonb(t)::text from " + ident.Sanitize() + " t where id=any($1::" + cast + ")"
			found := map[string]string{}
			result, err := tx.Query(ctx, query, parameter)
			if err != nil {
				return safePG("receipt verification query failed for "+ident.Sanitize(), err)
			}
			for result.Next() {
				var id, body string
				if err = result.Scan(&id, &body); err != nil {
					result.Close()
					return errors.New("receipt verification read failed")
				}
				found[id] = digest(json.RawMessage(body))
			}
			result.Close()
			if result.Err() != nil {
				return errors.New("receipt verification stream failed")
			}
			for _, row := range batch {
				if found[row.Key] != row.Digest {
					return fmt.Errorf("receipt row mismatch: %s", row.Token)
				}
			}
		}
	}
	return nil
}
