-- AlgoHub 003 ROLLBACK: batalkan email sintetis + auth.users
-- Hanya untuk user yang dibuat 003 (yang punya email @itpln.ac.id sintetis
-- dan email_lama NULL = memang tidak punya email sebelumnya).

begin;

-- 1) putuskan tautan + hapus auth rows milik user sintetis
delete from auth.identities i
using public.users u
where i.user_id = u.supabase_user_id
  and u.email_lama is null;

delete from auth.users a
using public.users u
where a.id = u.supabase_user_id
  and u.email_lama is null;

update public.users
set supabase_user_id = null
where email_lama is null;

-- 2) kembalikan email
update public.users set email = email_lama;

drop index if exists users_email_lower_uidx;
alter table public.users drop column if exists email_lama;

commit;

select
  (select count(*) from public.users where email is not null) as email_terisi,
  (select count(*) from public.users where supabase_user_id is not null) as tertaut,
  (select count(*) from auth.users) as auth_users;
