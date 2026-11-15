package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresPracticumRepository struct{ pool *pgxpool.Pool }

func NewPostgresPracticumRepository(pool *pgxpool.Pool) *PostgresPracticumRepository {
	return &PostgresPracticumRepository{pool: pool}
}

type postgresPracticumTx struct {
	ctx context.Context
	tx  pgx.Tx
}

func (r *PostgresPracticumRepository) Transaction(ctx context.Context, fn func(PracticumTx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	// A transaction-level domain mutex bounds concurrency and serializes shared
	// idempotency/lease admission. It never spans a runner/grader network call.
	// Row locks below always acquire activation before attempt; clock_timestamp
	// is obtained by the usecase only after these locks have been acquired.
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(731903005)"); err != nil {
		return err
	}
	if err = fn(&postgresPracticumTx{ctx: ctx, tx: tx}); err != nil {
		return databaseError(err)
	}
	return databaseError(tx.Commit(ctx))
}
func practicumTable(k PracticumKind) (string, error) {
	if !validPracticumKind(k) {
		return "", ErrConflict
	}
	return "public.practicum_" + string(k), nil
}
func (tx *postgresPracticumTx) get(k PracticumKind, id string, lock bool) (PracticumRecord, error) {
	table, err := practicumTable(k)
	if err != nil {
		return PracticumRecord{}, err
	}
	suffix := ""
	if lock {
		suffix = " for update"
	}
	var r PracticumRecord
	var scope []byte
	err = tx.tx.QueryRow(tx.ctx, "select id,parent_id,owner_id,scope,status,unique_key,expires_at,data from "+table+" where id=$1"+suffix, id).Scan(&r.ID, &r.ParentID, &r.OwnerID, &scope, &r.Status, &r.UniqueKey, &r.ExpiresAt, &r.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrPracticumNotFound
	}
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(scope, &r.Scope)
	return r, err
}
func (tx *postgresPracticumTx) Get(k PracticumKind, id string) (PracticumRecord, error) {
	return tx.get(k, id, false)
}
func (tx *postgresPracticumTx) Lock(k PracticumKind, id string) (PracticumRecord, error) {
	return tx.get(k, id, true)
}
func (tx *postgresPracticumTx) List(k PracticumKind, f PracticumFilter) ([]PracticumRecord, error) {
	table, err := practicumTable(k)
	if err != nil {
		return nil, err
	}
	scopes, err := json.Marshal(f.Scopes)
	if err != nil {
		return nil, err
	}
	if f.ActivationScope && k != PracticumAttempts {
		return nil, ErrConflict
	}
	rows, err := tx.tx.Query(tx.ctx, `select r.id,r.parent_id,r.owner_id,r.scope,r.status,r.unique_key,r.expires_at,r.data from `+table+` r
 where ($1::text='' or r.parent_id=$1) and ($2::bigint=0 or r.owner_id=$2)
 and ($3::text='' or r.status=$3) and ($4::text='' or r.unique_key=$4) and r.id>$5
 and ($6::timestamptz is null or r.expires_at<=$6)
 and ($7::jsonb='null'::jsonb or exists(select 1 from jsonb_array_elements(case when $7::jsonb='null'::jsonb then '[]'::jsonb else $7::jsonb end) g where public.practicum_scope_matches(g,case when $12 then (select a.scope from public.practicum_activations a where a.id=r.parent_id) else r.scope end,$8)))
 and ($9::bigint=0 or (r.data->'activation'->'student_ids') @> jsonb_build_array($9::bigint))
 and ($13::timestamptz is null or r.expires_at>$13)
 order by r.id limit $10 offset $11`, f.ParentID, f.OwnerID, f.Status, f.UniqueKey, f.AfterID, f.DueBefore, string(scopes), f.Overlap, f.MemberID, practicumLimit(f.Limit), max(0, f.Offset), f.ActivationScope, f.LiveAfter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PracticumRecord{}
	for rows.Next() {
		var r PracticumRecord
		var scope []byte
		if err = rows.Scan(&r.ID, &r.ParentID, &r.OwnerID, &scope, &r.Status, &r.UniqueKey, &r.ExpiresAt, &r.Data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(scope, &r.Scope); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (tx *postgresPracticumTx) EligibleStudents(ids []int64, scope entity.Scope) ([]int64, error) {
	raw, err := json.Marshal(scope)
	if err != nil {
		return nil, err
	}
	rows, err := tx.tx.Query(tx.ctx, `select distinct u.id
 from public.users u join public.user_role_assignments a on a.user_id=u.id
 left join public.divisions d on d.code=a.division_code
 where u.id=any($1::bigint[]) and u.is_active and a.is_active
 and a.role_code='mahasiswa' and (a.division_code is null or d.is_active)
 and public.practicum_scope_matches(a.scope,$2::jsonb,false)
 order by u.id`, ids, string(raw))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (tx *postgresPracticumTx) Put(k PracticumKind, r PracticumRecord) error {
	table, err := practicumTable(k)
	if err != nil {
		return err
	}
	scope, err := json.Marshal(r.Scope)
	if err != nil {
		return err
	}
	if !json.Valid(r.Data) {
		return fmt.Errorf("invalid practicum JSON")
	}
	_, err = tx.tx.Exec(tx.ctx, `insert into `+table+` (id,parent_id,owner_id,scope,status,unique_key,expires_at,data) values($1,$2,$3,$4::jsonb,$5,$6,$7,$8::jsonb)
 on conflict(id) do update set parent_id=excluded.parent_id,owner_id=excluded.owner_id,scope=excluded.scope,status=excluded.status,unique_key=excluded.unique_key,expires_at=excluded.expires_at,data=excluded.data`, r.ID, r.ParentID, r.OwnerID, string(scope), r.Status, r.UniqueKey, r.ExpiresAt, string(r.Data))
	return databaseError(err)
}
func (tx *postgresPracticumTx) Now() (time.Time, error) {
	var now time.Time
	err := tx.tx.QueryRow(tx.ctx, "select clock_timestamp()").Scan(&now)
	return now.UTC(), err
}
func (tx *postgresPracticumTx) Audit(e entity.AuditEvent) error {
	scope, err := json.Marshal(e.Scope)
	if err != nil {
		return err
	}
	if e.Detail == nil {
		e.Detail = map[string]any{}
	}
	detail, err := json.Marshal(e.Detail)
	if err != nil {
		return err
	}
	_, err = tx.tx.Exec(tx.ctx, `insert into public.audit_logs(actor_id,action,app,subject_type,subject_id,scope,request_id,operation_id,detail,created_at) values($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9::jsonb,$10)`, e.ActorID, e.Action, e.App, e.SubjectType, e.SubjectID, string(scope), e.RequestID, e.OperationID, string(detail), e.CreatedAt)
	return err
}
