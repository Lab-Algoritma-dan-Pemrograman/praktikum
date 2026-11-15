package migrationrehearsal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Report struct {
	FormatVersion string         `json:"format_version"`
	BatchID       string         `json:"batch_id"`
	Rows          []ReportRow    `json:"rows"`
	Counts        map[string]int `json:"counts"`
	Checks        []Check        `json:"checks"`
	Snapshot      Snapshot       `json:"snapshot"`
	Timing        Timing         `json:"timing"`
	Digest        string         `json:"digest"`
}
type ReportRow struct {
	RowToken    string      `json:"row_token"`
	Table       string      `json:"table"`
	Entity      string      `json:"entity"`
	Disposition Disposition `json:"disposition"`
	Reason      string      `json:"reason"`
	Digest      string      `json:"digest,omitempty"`
}

func Reconcile(ctx context.Context, pool *pgxpool.Pool, plan *Plan, result *Result) (*Report, error) {
	if err := VerifyPlan(plan); err != nil {
		return nil, err
	}
	if err := VerifyResult(result); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	target, err := verifyTarget(ctx, pool)
	if err != nil {
		return nil, err
	}
	if target != result.Target {
		return nil, errors.New("receipt belongs to another target")
	}
	started := time.Now().UTC()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer boundedRollback(tx)
	if err = setDeadlines(ctx, tx); err != nil {
		return nil, err
	}
	checks, err := reconcileTx(ctx, tx, plan, result)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	report := reportFromResult(result)
	report.Checks = checks
	report.Snapshot = result.After
	finished := time.Now().UTC()
	report.Timing = Timing{StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds()}
	report.Digest = reportDigest(report)
	return report, nil
}
func reconcileTx(ctx context.Context, tx pgx.Tx, plan *Plan, result *Result) ([]Check, error) {
	if result.BatchID != plan.BatchID || result.PlanDigest != plan.Digest || len(result.Rows) != len(plan.Rows) {
		return nil, errors.New("plan/receipt correspondence mismatch")
	}
	checks, err := schemaChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	accepted := 0
	for i, row := range result.Rows {
		p := plan.Rows[i]
		if p.RowToken != row.RowToken || p.Table != row.Table || p.Entity != row.Entity {
			return nil, errors.New("plan/result row correspondence mismatch")
		}
		if p.Disposition == Imported {
			if row.Disposition != Imported && row.Disposition != Unchanged {
				return nil, errors.New("accepted row not loaded")
			}
			if row.Disposition == Imported && row.Reason != ReasonInserted || row.Disposition == Unchanged && row.Reason != ReasonEqual {
				return nil, errors.New("result action/reason mismatch")
			}
			digest, err := currentDigest(ctx, tx, row.Table, row.TargetKey)
			if err != nil || digest != row.Digest {
				return nil, fmt.Errorf("row fingerprint mismatch: %s", row.RowToken)
			}
			accepted++
		} else if p.Disposition != row.Disposition || p.Reason != row.Reason || row.TargetKey != "" || row.Digest != "" {
			return nil, errors.New("rejected row disposition changed")
		}
	}
	if accepted != plan.Counts[string(Imported)] {
		return nil, errors.New("accepted count mismatch")
	}
	snapshot, err := captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if snapshot.Digest != result.After.Digest {
		return nil, errors.New("database differs from complete post-import snapshot")
	}
	checks = append(checks, Check{Name: "complete_input_accounting", Status: "passed", Reason: ReasonCheckPassed, Observed: len(result.Rows), Expected: len(plan.Rows)}, Check{Name: "accepted_row_fingerprints", Status: "passed", Reason: ReasonCheckPassed, Observed: accepted, Expected: plan.Counts[string(Imported)]}, Check{Name: "all_table_schema_sequence_checksums", Status: "passed", Reason: ReasonCheckPassed, Observed: len(snapshot.Tables), Expected: len(publicTables) + 1})
	return checks, nil
}

func verifySnapshot(s Snapshot) error {
	copy := s
	copy.Digest = ""
	if s.Digest == "" || s.Digest != digestValue(copy) || len(s.Tables) != len(publicTables)+1 || !sha256Pattern.MatchString(s.SchemaDigest) {
		return errors.New("invalid database snapshot")
	}
	names := append(append([]string{}, publicTables...), "auth.users")
	for i, t := range s.Tables {
		if t.Name != names[i] || t.Count < 0 || !sha256Pattern.MatchString(t.Digest) {
			return errors.New("invalid table snapshot")
		}
	}
	return nil
}
func verifyDeltas(result *Result) error {
	if err := verifySnapshot(result.Before); err != nil {
		return err
	}
	if err := verifySnapshot(result.After); err != nil {
		return err
	}
	if result.Before.SchemaDigest != result.After.SchemaDigest {
		return errors.New("import altered schema")
	}
	deltas := map[string]int{}
	for _, r := range result.Rows {
		if r.Disposition == Imported {
			deltas[r.Table]++
		}
	}
	for i, before := range result.Before.Tables {
		after := result.After.Tables[i]
		if after.Count != before.Count+deltas[before.Name] {
			return errors.New("unexpected per-table import delta")
		}
		if deltas[before.Name] == 0 && before.Digest != after.Digest {
			return errors.New("unmanaged rows changed")
		}
		delete(deltas, before.Name)
	}
	if len(deltas) != 0 {
		return errors.New("unrecognized imported table")
	}
	return nil
}

func currentDigest(ctx context.Context, tx pgx.Tx, table, key string) (string, error) {
	if table != "users" && table != "user_role_assignments" && table != "user_permission_grants" {
		if _, _, ok := genericSQL(table); !ok {
			return "", errors.New("table is not allowlisted")
		}
	}
	var value any = key
	if table == "users" || table == "user_role_assignments" || table == "user_permission_grants" {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return "", err
		}
		value = id
	}
	var raw []byte
	if err := tx.QueryRow(ctx, "select to_jsonb(t) from "+pgx.Identifier{"public", table}.Sanitize()+" t where id=$1", value).Scan(&raw); err != nil {
		return "", err
	}
	canonical, err := ParseCanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	return digestValue(jsonRaw(canonical)), nil
}

// Rollback is deliberately stricter than a shared-staging cleanup: every table,
// schema object and sequence must still match this exact receipt. Thus an audit
// SET NULL or a cascading delete cannot silently modify unowned child rows.
func Rollback(ctx context.Context, pool *pgxpool.Pool, result *Result, confirmBatch string) (*Report, error) {
	if err := VerifyResult(result); err != nil {
		return nil, err
	}
	if confirmBatch == "" || confirmBatch != result.BatchID {
		return nil, errors.New("confirm-batch must exactly match receipt")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	target, err := verifyTarget(ctx, pool)
	if err != nil {
		return nil, err
	}
	if target != result.Target {
		return nil, errors.New("receipt belongs to another target")
	}
	started := time.Now().UTC()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, err
	}
	defer boundedRollback(tx)
	if err = setDeadlines(ctx, tx); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, rehearsalAdvisoryLock); err != nil {
		return nil, err
	}
	if err = lockTables(ctx, tx); err != nil {
		return nil, err
	}
	if _, err = schemaChecks(ctx, tx); err != nil {
		return nil, err
	}
	current, err := captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if current.Digest != result.After.Digest {
		return nil, errors.New("rollback refused: target changed after import; use tested isolated restore")
	}
	report := &Report{FormatVersion: FormatVersion, BatchID: result.BatchID, Counts: map[string]int{}}
	for i := len(result.Rows) - 1; i >= 0; i-- {
		row := result.Rows[i]
		rr := ReportRow{RowToken: row.RowToken, Table: row.Table, Entity: row.Entity, Disposition: row.Disposition, Reason: row.Reason, Digest: row.Digest}
		if row.Disposition == Imported {
			digest, err := currentDigest(ctx, tx, row.Table, row.TargetKey)
			if err != nil || digest != row.Digest {
				return nil, errors.New("rollback row changed")
			}
			if err = deleteExact(ctx, tx, row.Table, row.TargetKey); err != nil {
				return nil, errors.New("exact-row rollback failed")
			}
			rr.Disposition = IntentionallySkipped
			rr.Reason = ReasonRolledBack
		}
		report.Rows = append(report.Rows, rr)
		report.Counts[string(rr.Disposition)]++
	}
	after, err := captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !sameRows(after, result.Before) {
		return nil, errors.New("rollback would alter baseline rows")
	}
	report.Snapshot = after
	report.Checks = []Check{{Name: "baseline_rows_and_schema_restored", Status: "passed", Reason: ReasonCheckPassed, Observed: len(publicTables) + 1, Expected: len(publicTables) + 1}, {Name: "identity_sequences_not_rewound", Status: "informational", Reason: "monotonic_sequences_preserved_use_backup_for_exact_restore"}}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	finished := time.Now().UTC()
	report.Timing = Timing{StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds()}
	report.Digest = reportDigest(report)
	return report, nil
}
func deleteExact(ctx context.Context, tx pgx.Tx, table, key string) error {
	var value any = key
	if table == "users" || table == "user_role_assignments" || table == "user_permission_grants" {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return err
		}
		value = id
	} else if _, _, ok := genericSQL(table); !ok {
		return errors.New("table is not allowlisted")
	}
	tag, err := tx.Exec(ctx, "delete from "+pgx.Identifier{"public", table}.Sanitize()+" where id=$1", value)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("unexpected exact-delete count")
	}
	return nil
}
func reportFromResult(result *Result) *Report {
	report := &Report{FormatVersion: FormatVersion, BatchID: result.BatchID, Counts: map[string]int{}}
	for _, row := range result.Rows {
		report.Rows = append(report.Rows, ReportRow{RowToken: row.RowToken, Table: row.Table, Entity: row.Entity, Disposition: row.Disposition, Reason: row.Reason, Digest: row.Digest})
		report.Counts[string(row.Disposition)]++
	}
	return report
}
func reportDigest(report *Report) string {
	clone := *report
	clone.Digest = ""
	clone.Timing = Timing{}
	return digestValue(clone)
}

type jsonRaw []byte

func (r jsonRaw) MarshalJSON() ([]byte, error) { return r, nil }
