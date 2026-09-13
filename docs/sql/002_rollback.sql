-- ============================================================
-- AlgoHub — 002_rollback.sql
-- Membalikkan 002_evolusi_identitas.sql.
--
-- PERINGATAN: ini MENGHAPUS tabel divisi/izin/divisi_izin/keanggotaan
-- beserta isinya, dan mengembalikan kosakata role ke bentuk lama
-- (mahasiswa->user, asisten->admin, koordinator->superadmin).
-- Kolom baru users (tersembunyi, divisi, kode_asisten, is_aktif)
-- ikut dibuang.
--
-- Backend Go versi BARU (mahasiswa/asisten/koordinator) tidak akan
-- bisa login setelah rollback ini — deploy balik versi lama dulu.
--
-- Data users itu sendiri tidak dihapus. Backup penuh users+kelas ada di
-- backups/pre002_users_kelas_<timestamp>.sql
-- ============================================================

begin;

-- ---------- 1. Buang constraint & kolom baru users ----------
alter table public.users
  drop constraint if exists users_divisi_hanya_staf,
  drop constraint if exists users_wajib_nim_atau_email,
  drop constraint if exists users_role_check;

drop index if exists uq_users_kode_asisten;
drop index if exists idx_users_divisi;

alter table public.users
  drop column if exists tersembunyi,
  drop column if exists divisi,
  drop column if exists kode_asisten,
  drop column if exists is_aktif;

-- ---------- 2. Kembalikan kosakata role lama ----------
update public.users set role = 'user'       where role = 'mahasiswa';
update public.users set role = 'admin'      where role = 'asisten';
update public.users set role = 'superadmin' where role = 'koordinator';
-- 'peminjam' tak punya padanan lama; jadikan 'user' agar tak melanggar apa pun.
update public.users set role = 'user'       where role = 'peminjam';

alter table public.users alter column role type varchar(10);

-- ---------- 3. nim wajib lagi ----------
-- Gagal bila ada baris nim NULL (mis. peminjam yang sudah dibuat).
-- Bersihkan dulu bila perlu:
--   delete from public.users where nim is null;
alter table public.users alter column nim set not null;

-- ---------- 4. Buang tabel & type baru ----------
drop table if exists keanggotaan;
drop table if exists divisi_izin;
drop table if exists izin;
drop table if exists divisi;

drop type if exists peran_keanggotaan;
drop type if exists app_asal;

commit;
