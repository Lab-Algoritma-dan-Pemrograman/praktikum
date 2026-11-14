package migrationrehearsal

import (
	"context"
	"errors"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InitializeFullAuthShim builds a Supabase-like auth schema on a NEW empty
// database so the masked dataset rehearsal exercises the same Auth write path
// as staging. It never connects to Supabase and seeds no application users.
func InitializeFullAuthShim(ctx context.Context, pool *pgxpool.Pool) (*Preflight, error) {
	if err := InitializeAuthPrerequisites(ctx, pool); err != nil {
		return nil, err
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		return nil, errors.New("new database schema initialization failed")
	}
	return Inspect(ctx, pool)
}

// InitializeAuthPrerequisites only creates the synthetic Auth shim on an empty
// owned database. Public schema is then restored from a verified pg_dump, not
// reconstructed by migrations. It never operates against a remote target.
func InitializeAuthPrerequisites(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if _, err := verifyTarget(ctx, pool); err != nil {
		return err
	}
	var tables int
	if err := pool.QueryRow(ctx, `select count(*) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname in ('public','auth') and c.relkind in ('r','p','S','v','m')`).Scan(&tables); err != nil {
		return err
	}
	if tables != 0 {
		return errors.New("initialization requires an empty owned database")
	}
	_, err := pool.Exec(ctx, `
 do $roles$ begin
  if not exists(select from pg_roles where rolname='anon') then create role anon nologin; end if;
  if not exists(select from pg_roles where rolname='authenticated') then create role authenticated nologin; end if;
 end $roles$;
 create schema auth;
 create table auth.users(
  instance_id uuid, id uuid primary key, aud text, role text, email text,
  encrypted_password text, email_confirmed_at timestamptz, invited_at timestamptz,
  confirmation_token text, confirmation_sent_at timestamptz, recovery_token text,
  recovery_sent_at timestamptz, email_change_token_new text, email_change text,
  email_change_sent_at timestamptz, last_sign_in_at timestamptz,
  raw_app_meta_data jsonb, raw_user_meta_data jsonb, is_super_admin boolean,
  created_at timestamptz, updated_at timestamptz,
  phone text default null, phone_confirmed_at timestamptz, phone_change text default '',
  phone_change_token text default '', phone_change_sent_at timestamptz,
  confirmed_at timestamptz, email_change_token_current text default '',
  email_change_confirm_status smallint default 0, banned_until timestamptz,
  reauthentication_token text default '', reauthentication_sent_at timestamptz,
  is_sso_user boolean not null default false, deleted_at timestamptz,
  is_anonymous boolean not null default false);
 create unique index users_email_partial_key on auth.users(lower(email)) where email is not null;
 create table auth.identities(
  id uuid primary key default gen_random_uuid(), user_id uuid not null references auth.users(id) on delete cascade,
  provider_id text not null, identity_data jsonb not null, provider text not null,
  last_sign_in_at timestamptz, created_at timestamptz, updated_at timestamptz,
  email text generated always as (lower(identity_data->>'email')) stored,
  unique(provider_id,provider));
 create function auth.uid() returns uuid language sql stable as $uid$
 select nullif(current_setting('request.jwt.claim.sub',true),'')::uuid
 $uid$;
 grant usage on schema auth,public to anon,authenticated;
 grant execute on function auth.uid() to anon,authenticated;
 `)
	if err != nil {
		return errors.New("auth shim prerequisite setup failed")
	}
	return nil
}
