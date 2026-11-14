-- Identity: legacy public.users -> auth.users + public.users + user_role_assignments.
-- Credentials move verbatim: the legacy bcrypt hash becomes encrypted_password so
-- every user keeps the password they already know.
drop view if exists legacy_user;
create temp view legacy_user as
select
  (row_data->>'id')::bigint as legacy_id,
  coalesce(nullif(btrim(row_data->>'nim'), ''), 'legacy-' || (row_data->>'id')) as nim,
  nullif(btrim(lower(row_data->>'email')), '') as real_email,
  coalesce(nullif(btrim(row_data->>'nama'), ''), nullif(btrim(row_data->>'full_name'), ''), 'User ' || (row_data->>'id')) as name,
  nullif(btrim(row_data->>'password_hash'), '') as password_hash,
  nullif(btrim(row_data->>'supabase_user_id'), '')::uuid as linked_auth_id,
  coalesce((row_data->>'is_aktif')::boolean, (row_data->>'is_active')::boolean, true) as is_active,
  coalesce((row_data->>'tersembunyi')::boolean, false) as hidden,
  coalesce((row_data->>'created_at')::timestamptz, now()) as created_at,
  coalesce((row_data->>'updated_at')::timestamptz, now()) as updated_at,
  lower(nullif(btrim(row_data->>'role'), '')) as role,
  lower(nullif(btrim(row_data->>'divisi'), '')) as division,
  nullif(btrim(row_data->>'shift'), '') as shift_text,
  nullif(btrim(row_data->>'kelas_id'), '')::bigint as kelas_id,
  row_data
from legacy_preserve.rows
where source_table = 'public.users';

drop view if exists legacy_auth_user;
create temp view legacy_auth_user as
select
  (row_data->>'id')::uuid as auth_id,
  nullif(btrim(row_data->'raw_user_meta_data'->>'nim'), '') as meta_nim,
  (row_data->>'email_confirmed_at')::timestamptz as confirmed_at
from legacy_preserve.rows
where source_table = 'auth.users';

-- Auth identity resolution: keep the legacy Supabase UUID when the old project
-- already linked one, otherwise adopt the SSO identity that carries the same NIM,
-- otherwise derive a stable UUID so reruns are byte-identical.
drop view if exists legacy_user_auth;
create temp view legacy_user_auth as
select a.*, coalesce(src.confirmed_at, now()) as confirmed_at
from (
  select u.*,
    coalesce(
      u.linked_auth_id,
      (select l.auth_id from legacy_auth_user l
        where l.meta_nim = u.nim and u.linked_auth_id is null
        order by l.auth_id limit 1),
      md5('algohub-legacy-user-' || u.legacy_id)::uuid
    ) as auth_user_id,
    coalesce(u.real_email, lower(u.nim) || '@users.algohub.web.id') as email
  from legacy_user u
) a
left join legacy_auth_user src on src.auth_id = a.auth_user_id;

insert into auth.users (
  instance_id, id, aud, role, email, encrypted_password, email_confirmed_at,
  created_at, updated_at, raw_app_meta_data, raw_user_meta_data,
  is_sso_user, is_anonymous, confirmation_token, recovery_token,
  email_change_token_new, email_change
)
select
  '00000000-0000-0000-0000-000000000000', a.auth_user_id, 'authenticated', 'authenticated',
  a.email, a.password_hash, a.confirmed_at,
  a.created_at, a.updated_at,
  jsonb_build_object('provider', 'email', 'providers', jsonb_build_array('email')),
  jsonb_build_object('name', a.name, 'nim', a.nim, 'legacy_user_id', a.legacy_id, 'legacy_login', 'nim'),
  false, false, '', '', '', ''
from legacy_user_auth a
on conflict (id) do update
set email = excluded.email,
    encrypted_password = excluded.encrypted_password,
    email_confirmed_at = excluded.email_confirmed_at,
    updated_at = excluded.updated_at,
    raw_user_meta_data = excluded.raw_user_meta_data;

insert into auth.identities (id, user_id, provider_id, provider, identity_data, created_at, updated_at)
select a.auth_user_id, a.auth_user_id, a.email, 'email',
       jsonb_build_object('sub', a.auth_user_id::text, 'email', a.email, 'email_verified', true, 'nim', a.nim),
       a.created_at, a.updated_at
from legacy_user_auth a
on conflict (provider_id, provider) do update
set identity_data = excluded.identity_data,
    updated_at = excluded.updated_at;

insert into public.users (id, auth_user_id, nim, email, name, is_active, hidden, created_at, updated_at)
select a.legacy_id, a.auth_user_id, a.nim, a.email, a.name, a.is_active, a.hidden, a.created_at, a.updated_at
from legacy_user_auth a
on conflict (id) do update
set auth_user_id = excluded.auth_user_id,
    nim = excluded.nim,
    email = excluded.email,
    name = excluded.name,
    is_active = excluded.is_active,
    hidden = excluded.hidden,
    updated_at = excluded.updated_at;

insert into public.user_role_assignments (id, user_id, role_code, division_code, scope, is_active, created_at)
select
  -a.legacy_id,
  a.legacy_id,
  role.code,
  case when role.code = 'asisten' and a.division in (select code from public.divisions) then a.division end,
  jsonb_strip_nulls(jsonb_build_object(
    'class_id', case when a.kelas_id is not null then 'legacy-class-' || a.kelas_id::text end,
    'shift', case when a.shift_text ~ '^[0-9]+$' then a.shift_text::int end,
    'legacy_division', case when role.code <> 'asisten' then a.division end
  )),
  a.is_active,
  a.created_at
from legacy_user_auth a
cross join lateral (
  select case a.role
    when 'koordinator' then 'koordinator'
    when 'asisten' then 'asisten'
    when 'peminjam' then 'peminjam'
    else 'mahasiswa'
  end as code
) role
on conflict (id) do update
set role_code = excluded.role_code,
    division_code = excluded.division_code,
    scope = excluded.scope,
    is_active = excluded.is_active;
