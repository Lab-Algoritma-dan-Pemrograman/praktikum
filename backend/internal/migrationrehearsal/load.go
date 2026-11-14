package migrationrehearsal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const rehearsalAdvisoryLock int64 = 0x5245484541525345

type Result struct {
	FormatVersion string         `json:"format_version"`
	BatchID       string         `json:"batch_id"`
	PlanDigest    string         `json:"plan_digest"`
	Target        LocalTarget    `json:"target"`
	Before        Snapshot       `json:"before"`
	After         Snapshot       `json:"after"`
	Rows          []ResultRow    `json:"rows"`
	Counts        map[string]int `json:"counts"`
	Checks        []Check        `json:"checks"`
	Timing        Timing         `json:"timing"`
	Digest        string         `json:"digest"`
}
type ResultRow struct {
	RowToken    string      `json:"row_token"`
	Table       string      `json:"table"`
	Entity      string      `json:"entity"`
	Disposition Disposition `json:"disposition"`
	Reason      string      `json:"reason"`
	Digest      string      `json:"digest,omitempty"`
	TargetKey   string      `json:"target_key,omitempty"`
}
type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Observed int    `json:"observed,omitempty"`
	Expected int    `json:"expected,omitempty"`
}
type Timing struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMS int64     `json:"duration_ms"`
}
type loadState struct {
	ids  map[string]int64
	keys map[string]string
}

func Apply(ctx context.Context, pool *pgxpool.Pool, plan *Plan) (*Result, error) {
	if err := VerifyPlan(plan); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	target, err := verifyTarget(ctx, pool)
	if err != nil {
		return nil, err
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
	checks, err := schemaChecks(ctx, tx)
	if err != nil {
		return nil, err
	}
	before, err := captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	result := &Result{FormatVersion: FormatVersion, BatchID: plan.BatchID, PlanDigest: plan.Digest, Target: target, Before: before, Counts: map[string]int{}, Checks: checks}
	state := &loadState{ids: map[string]int64{}, keys: map[string]string{}}
	for _, row := range plan.Rows {
		rr := ResultRow{RowToken: row.RowToken, Table: row.Table, Entity: row.Entity, Disposition: row.Disposition, Reason: row.Reason}
		if row.Disposition == Imported {
			action, key, loadErr := loadOne(ctx, tx, row, state)
			// Never include PostgreSQL detail: it can contain a rejected row's contents.
			if loadErr != nil {
				return nil, fmt.Errorf("load failed for %s row %s (transaction rolled back)", row.Entity, row.RowToken)
			}
			rr.Disposition, rr.TargetKey = action, key
			if action == Imported {
				rr.Reason = ReasonInserted
			} else {
				rr.Reason = ReasonEqual
			}
			rr.Digest, err = currentDigest(ctx, tx, row.Table, key)
			if err != nil {
				return nil, errors.New("cannot fingerprint loaded row")
			}
		}
		result.Rows = append(result.Rows, rr)
		result.Counts[string(rr.Disposition)]++
	}
	result.After, err = captureSnapshot(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err = verifyDeltas(result); err != nil {
		return nil, err
	}
	additional, err := reconcileTx(ctx, tx, plan, result)
	if err != nil {
		return nil, err
	}
	result.Checks = append(result.Checks, additional...)
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	finished := time.Now().UTC()
	result.Timing = Timing{StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds()}
	result.Digest = resultDigest(result)
	return result, nil
}

func loadOne(ctx context.Context, tx pgx.Tx, row PlanRow, state *loadState) (Disposition, string, error) {
	d := row.Desired
	if d == nil {
		return Rejected, "", errors.New("missing desired record")
	}
	switch row.Entity {
	case "user":
		var id, access int64
		var nim *string
		var email, name string
		var active, hidden bool
		var created, updated time.Time
		err := tx.QueryRow(ctx, `select id,nim,email,name,is_active,hidden,created_at,updated_at,access_version from public.users where auth_user_id=$1`, d.AuthUserID).Scan(&id, &nim, &email, &name, &active, &hidden, &created, &updated, &access)
		inserted := false
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `insert into public.users(auth_user_id,nim,email,name,is_active,hidden,created_at,updated_at,access_version) values($1,$2,$3,$4,$5,$6,$7,$7,1) returning id`, d.AuthUserID, d.NIM, d.Email, d.Name, *d.IsActive, *d.Hidden, *d.CreatedAt).Scan(&id)
			inserted = true
		} else if err == nil && (!equalStringPtr(nim, d.NIM) || email != d.Email || name != d.Name || active != *d.IsActive || hidden != *d.Hidden || !created.Equal(*d.CreatedAt) || !updated.Equal(*d.CreatedAt) || access != 1) {
			return Rejected, "", conflictError()
		}
		if err != nil {
			return Rejected, "", err
		}
		state.ids[row.RowToken] = id
		return disposition(inserted), strconv.FormatInt(id, 10), nil
	case "assignment":
		userID, ok := state.ids[d.UserToken]
		if !ok {
			return Rejected, "", errors.New("unresolved user")
		}
		var id int64
		var active bool
		var created time.Time
		err := tx.QueryRow(ctx, `select id,is_active,created_at from public.user_role_assignments where user_id=$1 and role_code=$2 and division_code is not distinct from $3 and scope=$4`, userID, d.RoleCode, d.DivisionCode, d.Scope).Scan(&id, &active, &created)
		inserted := false
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `insert into public.user_role_assignments(user_id,role_code,division_code,scope,is_active,created_at) values($1,$2,$3,$4,$5,$6) returning id`, userID, d.RoleCode, d.DivisionCode, d.Scope, *d.IsActive, *d.CreatedAt).Scan(&id)
			inserted = true
		} else if err == nil && (active != *d.IsActive || !created.Equal(*d.CreatedAt)) {
			return Rejected, "", conflictError()
		}
		if err != nil {
			return Rejected, "", err
		}
		state.ids[row.RowToken] = id
		return disposition(inserted), strconv.FormatInt(id, 10), nil
	case "grant":
		userID, uok := state.ids[d.UserToken]
		assignmentID, aok := state.ids[d.AssignmentToken]
		if !uok || !aok {
			return Rejected, "", errors.New("unresolved assignment")
		}
		var id int64
		var active bool
		var created time.Time
		err := tx.QueryRow(ctx, `select id,is_active,created_at from public.user_permission_grants where assignment_id=$1 and user_id=$2 and permission_code=$3 and scope=$4`, assignmentID, userID, d.PermissionCode, d.Scope).Scan(&id, &active, &created)
		inserted := false
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `insert into public.user_permission_grants(user_id,assignment_id,permission_code,scope,is_active,created_at) values($1,$2,$3,$4,$5,$6) returning id`, userID, assignmentID, d.PermissionCode, d.Scope, *d.IsActive, *d.CreatedAt).Scan(&id)
			inserted = true
		} else if err == nil && (active != *d.IsActive || !created.Equal(*d.CreatedAt)) {
			return Rejected, "", conflictError()
		}
		if err != nil {
			return Rejected, "", err
		}
		state.ids[row.RowToken] = id
		return disposition(inserted), strconv.FormatInt(id, 10), nil
	default:
		return loadGeneric(ctx, tx, row, state)
	}
}
func loadGeneric(ctx context.Context, tx pgx.Tx, row PlanRow, state *loadState) (Disposition, string, error) {
	d, err := bindDomain(row, state.ids, state.keys)
	if err != nil {
		return Rejected, "", err
	}
	ownerID := int64(0)
	if d.OwnerToken != "" {
		var ok bool
		ownerID, ok = state.ids[d.OwnerToken]
		if !ok {
			return Rejected, "", errors.New("unresolved owner")
		}
	}
	query, insert, ok := genericSQL(row.Table)
	if !ok {
		return Rejected, "", errors.New("table not allowlisted")
	}
	var parent, status, unique string
	var owner int64
	var scope, data []byte
	var expires *time.Time
	err = tx.QueryRow(ctx, query, d.ID).Scan(&parent, &owner, &scope, &status, &unique, &expires, &data)
	inserted := false
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, insert, d.ID, d.ParentID, ownerID, d.Scope, d.Status, d.UniqueKey, d.ExpiresAt, d.Data)
		inserted = true
	} else if err == nil && (parent != d.ParentID || owner != ownerID || status != d.Status || unique != d.UniqueKey || !equalTimePtr(expires, d.ExpiresAt) || !jsonEqual(scope, d.Scope) || !jsonEqual(data, d.Data)) {
		return Rejected, "", conflictError()
	}
	if err != nil {
		return Rejected, "", err
	}
	state.keys[row.RowToken] = d.ID
	return disposition(inserted), d.ID, nil
}
func genericSQL(table string) (string, string, bool) {
	allowed := map[string]bool{"practicum_sessions": true, "practicum_courses": true, "practicum_questions": true, "practicum_activations": true, "practicum_attempts": true, "practicum_grading_jobs": true, "learning_levels": true, "learning_modules": true, "learning_lessons": true, "learning_progress": true, "learning_profiles": true, "learning_bug_hunt": true, "learning_game_history": true, "academic_classes": true, "academic_schedules": true, "academic_attendance_sessions": true, "academic_attendance_records": true, "academic_inventory_items": true, "academic_inventory_rentals": true}
	if !allowed[table] {
		return "", "", false
	}
	ident := pgx.Identifier{"public", table}.Sanitize()
	return "select parent_id,owner_id,scope,status,unique_key,expires_at,data from " + ident + " where id=$1", "insert into " + ident + "(id,parent_id,owner_id,scope,status,unique_key,expires_at,data) values($1,$2,$3,$4,$5,$6,$7,$8)", true
}
func verifyMigrationHistory(ctx context.Context, tx pgx.Tx, expected []migrations.Definition) error {
	rows, err := tx.Query(ctx, `select name,checksum from public.schema_migrations order by name`)
	if err != nil {
		return errors.New("migration history unavailable")
	}
	defer rows.Close()
	actual := []migrations.Definition{}
	for rows.Next() {
		var d migrations.Definition
		if err = rows.Scan(&d.Name, &d.Checksum); err != nil {
			return err
		}
		actual = append(actual, d)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return errors.New("migration history mismatch")
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return errors.New("migration history mismatch")
		}
	}
	return nil
}
func conflictError() error { return errors.New(ReasonConflict) }
func disposition(inserted bool) Disposition {
	if inserted {
		return Imported
	}
	return Unchanged
}
func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
func equalTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}
func jsonEqual(a, b []byte) bool {
	ca, ea := ParseCanonicalJSON(a)
	cb, eb := ParseCanonicalJSON(b)
	return ea == nil && eb == nil && bytes.Equal(ca, cb)
}
func digestValue(v any) string {
	body, err := CanonicalJSON(v)
	if err != nil {
		panic("non-JSON value in internal fingerprint")
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func resultDigest(result *Result) string {
	clone := *result
	clone.Digest = ""
	clone.Timing = Timing{}
	return digestValue(clone)
}
func VerifyResult(result *Result) error {
	if result == nil || result.FormatVersion != FormatVersion || result.BatchID == "" || result.Digest != resultDigest(result) {
		return errors.New("result digest mismatch")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, row := range result.Rows {
		if !validDisposition(row.Disposition) || seen[row.RowToken] || row.RowToken == "" {
			return errors.New("invalid result row")
		}
		seen[row.RowToken] = true
		counts[string(row.Disposition)]++
		if row.Disposition == Imported || row.Disposition == Unchanged {
			if !sha256Pattern.MatchString(row.Digest) || row.TargetKey == "" {
				return errors.New("missing imported row fingerprint")
			}
		}
	}
	if digestValue(counts) != digestValue(result.Counts) {
		return errors.New("result accounting mismatch")
	}
	return verifyDeltas(result)
}
func LoadResult(path string) (*Result, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result Result
	if err = decodeStrict(body, &result); err != nil {
		return nil, err
	}
	if err = VerifyResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
