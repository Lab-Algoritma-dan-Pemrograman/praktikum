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

type PostgresAcademicRepository struct{ pool *pgxpool.Pool }

func NewPostgresAcademicRepository(pool *pgxpool.Pool) *PostgresAcademicRepository {
	return &PostgresAcademicRepository{pool: pool}
}

type postgresAcademicTx struct {
	ctx context.Context
	tx  pgx.Tx
}

func (r *PostgresAcademicRepository) Transaction(ctx context.Context, fn func(AcademicTx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	// Serialize SIAKAD attendance check-in and inventory stock allocation per transaction.
	if _, err = tx.Exec(ctx, "select pg_advisory_xact_lock(731905007)"); err != nil {
		return err
	}
	if err = fn(&postgresAcademicTx{ctx: ctx, tx: tx}); err != nil {
		return databaseError(err)
	}
	return databaseError(tx.Commit(ctx))
}

func academicTable(k AcademicKind) (string, error) {
	if !validAcademicKind(k) {
		return "", ErrConflict
	}
	return "public.academic_" + string(k), nil
}

func (tx *postgresAcademicTx) get(k AcademicKind, id string, lock bool) (AcademicRecord, error) {
	table, err := academicTable(k)
	if err != nil {
		return AcademicRecord{}, err
	}
	suffix := ""
	if lock {
		suffix = " for update"
	}
	var r AcademicRecord
	var scope []byte
	err = tx.tx.QueryRow(tx.ctx, "select id,parent_id,owner_id,scope,status,unique_key,expires_at,data from "+table+" where id=$1"+suffix, id).Scan(&r.ID, &r.ParentID, &r.OwnerID, &scope, &r.Status, &r.UniqueKey, &r.ExpiresAt, &r.Data)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrAcademicNotFound
	}
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(scope, &r.Scope)
	return r, err
}

func (tx *postgresAcademicTx) Get(k AcademicKind, id string) (AcademicRecord, error) {
	return tx.get(k, id, false)
}

func (tx *postgresAcademicTx) Lock(k AcademicKind, id string) (AcademicRecord, error) {
	return tx.get(k, id, true)
}

func (tx *postgresAcademicTx) List(k AcademicKind, f AcademicFilter) ([]AcademicRecord, error) {
	table, err := academicTable(k)
	if err != nil {
		return nil, err
	}
	scopes, err := json.Marshal(f.Scopes)
	if err != nil {
		return nil, err
	}
	membership := `data->>'active'='true' and (coalesce(data->'student_ids','[]'::jsonb) @> jsonb_build_array($9::bigint) or coalesce(data->'assistant_ids','[]'::jsonb) @> jsonb_build_array($9::bigint))`
	if k == AcademicSchedules {
		membership = `exists(select 1 from public.academic_classes c where c.id=academic_schedules.parent_id and c.data->>'active'='true' and (coalesce(c.data->'student_ids','[]'::jsonb) @> jsonb_build_array($9::bigint) or coalesce(c.data->'assistant_ids','[]'::jsonb) @> jsonb_build_array($9::bigint)))`
	}
	rows, err := tx.tx.Query(tx.ctx, `select id,parent_id,owner_id,scope,status,unique_key,expires_at,data from `+table+`
 where ($1::text='' or parent_id=$1) and ($2::bigint=0 or owner_id=$2)
 and ($3::text='' or status=$3) and ($4::text='' or unique_key=$4) and id>$5
 and ($6::timestamptz is null or expires_at<=$6)
 and ($7::jsonb='null'::jsonb or exists(select 1 from jsonb_array_elements(case when $7::jsonb='null'::jsonb then '[]'::jsonb else $7::jsonb end) g where public.practicum_scope_matches(g,scope,$8)))
 and ($9::bigint=0 or (`+membership+`))
 order by id limit $10 offset $11`, f.ParentID, f.OwnerID, f.Status, f.UniqueKey, f.AfterID, f.DueBefore, string(scopes), f.Overlap, f.MemberID, learningLimit(f.Limit), max(0, f.Offset))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AcademicRecord{}
	for rows.Next() {
		var r AcademicRecord
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

func (tx *postgresAcademicTx) Put(k AcademicKind, r AcademicRecord) error {
	table, err := academicTable(k)
	if err != nil {
		return err
	}
	scope, err := json.Marshal(r.Scope)
	if err != nil {
		return err
	}
	if !json.Valid(r.Data) {
		return fmt.Errorf("invalid academic JSON")
	}
	_, err = tx.tx.Exec(tx.ctx, `insert into `+table+` (id,parent_id,owner_id,scope,status,unique_key,expires_at,data) values($1,$2,$3,$4::jsonb,$5,$6,$7,$8::jsonb)
 on conflict(id) do update set parent_id=excluded.parent_id,owner_id=excluded.owner_id,scope=excluded.scope,status=excluded.status,unique_key=excluded.unique_key,expires_at=excluded.expires_at,data=excluded.data`, r.ID, r.ParentID, r.OwnerID, string(scope), r.Status, r.UniqueKey, r.ExpiresAt, string(r.Data))
	return databaseError(err)
}

func (tx *postgresAcademicTx) Now() (time.Time, error) {
	var now time.Time
	err := tx.tx.QueryRow(tx.ctx, "select clock_timestamp()").Scan(&now)
	return now.UTC(), err
}

func (tx *postgresAcademicTx) Audit(e entity.AuditEvent) error {
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
