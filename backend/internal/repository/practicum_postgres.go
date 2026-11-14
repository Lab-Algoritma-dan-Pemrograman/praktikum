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
	defer tx.Rollback(context.WithoutCancel(ctx))
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
	rows, err := tx.tx.Query(tx.ctx, `select id,parent_id,owner_id,scope,status,unique_key,expires_at,data from `+table+`
 where ($1::text='' or parent_id=$1) and ($2::bigint=0 or owner_id=$2)
 and ($3::text='' or status=$3) and ($4::text='' or unique_key=$4) and id>$5
 and ($6::timestamptz is null or expires_at<=$6)
 and ($7::jsonb='null'::jsonb or exists(select 1 from jsonb_array_elements(case when $7::jsonb='null'::jsonb then '[]'::jsonb else $7::jsonb end) g where public.practicum_scope_matches(g,scope,$8)))
 and ($9::bigint=0 or (data->'activation'->'student_ids') @> jsonb_build_array($9::bigint))
 order by id limit $10 offset $11`, f.ParentID, f.OwnerID, f.Status, f.UniqueKey, f.AfterID, f.DueBefore, string(scopes), f.Overlap, f.MemberID, practicumLimit(f.Limit), max(0, f.Offset))
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
