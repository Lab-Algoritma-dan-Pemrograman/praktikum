package migrationrehearsal

import (
	"context"
	"errors"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InitializeSynthetic is only for an EMPTY database created by the native
// harness. Existing targets must use Inspect, which never calls migrations.Apply.
func InitializeSynthetic(ctx context.Context, pool *pgxpool.Pool) (*Preflight, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := verifyTarget(ctx, pool); err != nil {
		return nil, err
	}
	var tables int
	if err := pool.QueryRow(ctx, `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth') and c.relkind in ('r','p','S','v','m')`).Scan(&tables); err != nil {
		return nil, err
	}
	if tables != 0 {
		return nil, errors.New("initialization requires an empty owned database")
	}
	_, err := pool.Exec(ctx, `
 do $roles$ begin
  if not exists(select from pg_roles where rolname='anon') then create role anon nologin; end if;
  if not exists(select from pg_roles where rolname='authenticated') then create role authenticated nologin; end if;
 end $roles$;
 create schema auth;
 create table auth.users(id uuid primary key);
 create function auth.uid() returns uuid language sql stable as $uid$
 select nullif(current_setting('request.jwt.claim.sub',true),'')::uuid
 $uid$;
 grant usage on schema auth,public to anon,authenticated;
 grant execute on function auth.uid() to anon,authenticated;
 `)
	if err != nil {
		return nil, errors.New("synthetic auth prerequisite setup failed")
	}
	if err = migrations.Apply(ctx, pool); err != nil {
		return nil, errors.New("new database schema initialization failed")
	}
	// Auth stubs and an unrelated sentinel are baseline, never imported source rows.
	// They have no password and cannot log in to an external provider.
	_, err = pool.Exec(ctx, `insert into auth.users(id) values($1),($2),('00000000-0000-4000-8000-000000000099');
 `, SyntheticStudentAuthID, SyntheticTeacherAuthID)
	if err != nil {
		return nil, errors.New("synthetic auth stub setup failed")
	}
	_, err = pool.Exec(ctx, `insert into public.users(auth_user_id,email,name,is_active,hidden,created_at,updated_at)
 values('00000000-0000-4000-8000-000000000099','phase6-baseline@example.test','Unrelated baseline sentinel',true,false,'2026-11-01T00:00:00Z','2026-11-01T00:00:00Z')`)
	if err != nil {
		return nil, errors.New("baseline sentinel setup failed")
	}
	return Inspect(ctx, pool)
}
