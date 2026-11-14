alter table public.users
  add column access_version bigint not null default 1,
  add constraint users_access_version_positive check (access_version > 0),
  add constraint users_email_normalized check (email = btrim(email) and length(email) > 0);

alter table public.user_role_assignments
  add constraint assignments_id_user_unique unique (id, user_id),
  add constraint assignments_scope_object check (jsonb_typeof(scope) = 'object');

alter table public.user_permission_grants
  add column assignment_id bigint,
  add constraint grants_scope_object check (jsonb_typeof(scope) = 'object');

update public.user_permission_grants g
set assignment_id = (
  select min(a.id)
  from public.user_role_assignments a
  left join public.divisions d on d.code = a.division_code
  where a.user_id = g.user_id and a.is_active
    and (a.division_code is null or d.is_active)
    and (
      select coalesce(jsonb_object_agg(key, value), '{}'::jsonb)
      from jsonb_each(a.scope)
      where value not in ('""'::jsonb, '"*"'::jsonb, '0'::jsonb, 'null'::jsonb)
    ) <@ g.scope
  having count(*) = 1
);

alter table public.user_permission_grants
  alter column assignment_id set not null,
  add constraint grants_assignment_user_fk foreign key (assignment_id, user_id)
    references public.user_role_assignments(id, user_id) on delete cascade;

create unique index user_permission_grants_assignment_unique
  on public.user_permission_grants(assignment_id, permission_code, scope) where is_active;

alter table public.auth_sessions add column family_id text;
update public.auth_sessions set family_id = id;

alter table public.auth_sessions
  alter column family_id set not null,
  add constraint auth_sessions_id_user_unique unique (id, user_id),
  add constraint auth_sessions_id_not_empty check (length(id) > 0),
  add constraint auth_sessions_family_user_fk foreign key (family_id, user_id)
    references public.auth_sessions(id, user_id) on delete cascade,
  add constraint auth_sessions_token_hash_format check (token_hash ~ '^[0-9a-f]{64}$'),
  add constraint auth_sessions_csrf_hash_format check (csrf_token_hash ~ '^[0-9a-f]{64}$'),
  add constraint auth_sessions_lifetime check (expires_at > created_at);

create index auth_sessions_family_idx on public.auth_sessions(family_id);

create function public.enforce_auth_session_family() returns trigger
language plpgsql set search_path = pg_catalog, public as $$
begin
  if tg_op = 'UPDATE' then
    if (new.id, new.user_id, new.family_id, new.host, new.created_at)
      is distinct from (old.id, old.user_id, old.family_id, old.host, old.created_at) then
      raise exception 'session identity and family are immutable' using errcode = '23514';
    end if;
    if old.revoked_at is not null and new.revoked_at is distinct from old.revoked_at then
      raise exception 'revoked sessions cannot be restored' using errcode = '23514';
    end if;
    if new.expires_at > old.expires_at then
      raise exception 'session lifetime cannot be extended' using errcode = '23514';
    end if;
  end if;
  if new.family_id <> new.id then
    perform 1 from public.auth_sessions
      where id = new.family_id and family_id = id and user_id = new.user_id
      for key share;
    if not found then
      raise exception 'session family must reference its own user root' using errcode = '23514';
    end if;
  end if;
  return new;
end;
$$;

create trigger auth_sessions_enforce_family
  before insert or update on public.auth_sessions
  for each row execute function public.enforce_auth_session_family();

create table public.sso_handshakes (
  code_hash text primary key check (code_hash ~ '^[0-9a-f]{64}$'),
  session_id text not null references public.auth_sessions(id) on delete cascade,
  code_challenge text not null check (code_challenge ~ '^[A-Za-z0-9_-]{43}$'),
  return_to text not null check (length(btrim(return_to)) > 0),
  expires_at timestamptz not null,
  consumed_at timestamptz,
  created_at timestamptz not null default now()
);
create index sso_handshakes_session_idx on public.sso_handshakes(session_id);
create index sso_handshakes_expiry_idx on public.sso_handshakes(expires_at);

create table public.auth_rate_limits (
  key_hash text not null check (key_hash ~ '^[0-9a-f]{64}$'),
  window_start timestamptz not null,
  hits integer not null check (hits > 0),
  primary key (key_hash, window_start)
);
create index auth_rate_limits_window_idx on public.auth_rate_limits(window_start);

insert into public.permissions (code, app, description)
values ('identity.access.manage', 'admin', 'Mengelola role, divisi, dan permission user')
on conflict (code) do update set app = excluded.app, description = excluded.description;

insert into public.role_permissions (role_code, permission_code, default_scope)
values ('koordinator', 'identity.access.manage', '{"domain":"*"}'::jsonb)
on conflict (role_code, permission_code) do update set default_scope = excluded.default_scope;

update public.role_permissions set default_scope = '{}'::jsonb
where permission_code = 'admin.overview.read';
update public.division_permissions set default_scope = '{}'::jsonb
where permission_code = 'admin.overview.read';

alter table public.sso_handshakes enable row level security;
alter table public.auth_rate_limits enable row level security;

drop policy if exists users_self_update on public.users;
drop policy if exists sessions_self_select on public.auth_sessions;

revoke all on public.users, public.roles, public.divisions, public.permissions,
  public.role_permissions, public.division_permissions, public.user_role_assignments,
  public.user_permission_grants, public.auth_sessions, public.sso_handshakes,
  public.auth_rate_limits, public.audit_logs from public, anon, authenticated;

revoke all on sequence public.users_id_seq, public.user_role_assignments_id_seq,
  public.user_permission_grants_id_seq, public.audit_logs_id_seq
  from public, anon, authenticated;

revoke all on function public.enforce_auth_session_family() from public, anon, authenticated;

grant select on public.users, public.user_role_assignments, public.user_permission_grants to authenticated;
