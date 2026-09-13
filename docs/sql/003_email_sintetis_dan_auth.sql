-- AlgoHub 003: email sintetis + daftar ke auth.users
-- Pola: <namadepan><nim tanpa 2 digit awal>@itpln.ac.id
-- Contoh: NAUFAL RAIHAN SAPUTRA / 202411106 -> naufal2411106@itpln.ac.id
-- Uji dulu di replika; ada 003_rollback.sql.

begin;

-- 1) kolom email unik (case-insensitive)
update public.users set email = null where email = '';

alter table public.users
  add column if not exists email_lama varchar(255);
update public.users set email_lama = email where email is not null and email_lama is null;

-- 2) isi email sintetis utk yang kosong
-- nama depan: huruf saja, lowercase. nim: buang 2 digit pertama.
-- akun non-NIM (superadmin_ap) -> pakai nim apa adanya, tanpa potong.
update public.users u
set email = lower(regexp_replace(split_part(u.nama, ' ', 1), '[^a-zA-Z]', '', 'g'))
         || case when u.nim ~ '^[0-9]+$' then substring(u.nim from 3) else u.nim end
         || '@itpln.ac.id'
where u.email is null;

-- 3) jaga unik
create unique index if not exists users_email_lower_uidx
  on public.users (lower(email)) where email is not null;

-- 4) daftarkan ke auth.users, bawa bcrypt yang sudah ada
-- CATATAN PENTING: kolom token WAJIB '' bukan NULL.
-- GoTrue scan kolom ini ke Go string; NULL bikin login gagal
-- 500 "Database error querying schema".
insert into auth.users (
  id, instance_id, aud, role, email, encrypted_password,
  email_confirmed_at, created_at, updated_at,
  confirmation_token, recovery_token, email_change,
  email_change_token_new, email_change_token_current,
  phone_change, phone_change_token, reauthentication_token,
  raw_app_meta_data, raw_user_meta_data
)
select
  gen_random_uuid(), '00000000-0000-0000-0000-000000000000',
  'authenticated', 'authenticated', lower(u.email), u.password_hash,
  now(), now(), now(),
  '', '', '', '', '', '', '', '',
  '{"provider":"email","providers":["email"]}'::jsonb,
  jsonb_build_object('nim', u.nim, 'nama', u.nama, 'app_role', u.role)
from public.users u
where u.email is not null
  and u.password_hash is not null
  and u.supabase_user_id is null
  and not exists (select 1 from auth.users a where a.email = lower(u.email));

-- 5) identities: wajib, kalau tidak signInWithPassword gagal
insert into auth.identities (
  id, user_id, provider_id, provider, identity_data,
  last_sign_in_at, created_at, updated_at
)
select
  gen_random_uuid(), a.id, a.id::text, 'email',
  jsonb_build_object('sub', a.id::text, 'email', a.email, 'email_verified', true),
  null, now(), now()
from auth.users a
where not exists (
  select 1 from auth.identities i
  where i.user_id = a.id and i.provider = 'email'
);

-- 6) tautkan
update public.users u
set supabase_user_id = a.id
from auth.users a
where lower(u.email) = a.email and u.supabase_user_id is null;

commit;

-- verifikasi
select
  (select count(*) from public.users where email is not null) as email_terisi,
  (select count(*) from public.users where supabase_user_id is not null) as tertaut,
  (select count(*) from auth.users) as auth_users,
  (select count(*) from auth.identities) as identities;
