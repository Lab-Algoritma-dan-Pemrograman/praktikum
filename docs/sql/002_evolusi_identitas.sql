-- ============================================================
-- AlgoHub — 002_evolusi_identitas.sql
-- EVOLUSI di atas public.users praktikum yang SUDAH ADA (live).
-- Bukan create-from-scratch. Uji di replika dulu (Docker chain).
--
-- PRASYARAT: schema public praktikum (pg_dump --schema public).
-- Backend Go SUDAH pakai kosakata baru (enums.go: mahasiswa /
-- asisten / koordinator — build + test lolos). Frontend menyusul.
--
-- DIVISI = TABEL, bukan enum Postgres: koordinator menambah
-- divisi baru cukup INSERT baris (tanpa DDL), dan hak akses tiap
-- divisi diatur mutlak lewat divisi_izin. Enum Postgres tak bisa
-- ditambah dari UI tanpa ALTER TYPE — tabel bisa.
-- ============================================================

begin;

-- ---------- 1. DIVISI (enum yang bisa tumbuh) ----------
create table if not exists divisi (
  kode        varchar(32) primary key,
  nama        varchar(64) not null,
  dibuat_pada timestamptz not null default now()
);

-- 5 divisi terkonfirmasi ketua lab (Sep 2026).
insert into divisi (kode, nama) values
  ('k3',         'K3'),
  ('sekretaris', 'Sekretaris'),
  ('labops',     'Labops'),
  ('pendidikan', 'Pendidikan'),
  ('pdd',        'PDD')
on conflict (kode) do nothing;
-- 'bendahara' tidak masuk daftar ketua; jika disepakati cukup:
--   insert into divisi (kode, nama) values ('bendahara', 'Bendahara');

-- ---------- 2. ROLE: pelebaran + konversi kosakata ----------
-- varchar(10) tak muat 'koordinator' (11 char) -> 20.
alter table public.users alter column role type varchar(20);

update public.users set role = 'mahasiswa'   where role = 'user';
update public.users set role = 'asisten'     where role = 'admin';
update public.users set role = 'koordinator' where role = 'superadmin';

alter table public.users
  alter column role set not null,
  add constraint users_role_check check (role in ('koordinator', 'asisten', 'mahasiswa', 'peminjam'));

-- ---------- 3. KOLOM BARU users ----------
-- nim dilonggarkan: role peminjam (orang luar) tanpa NIM.
-- Index unique uq_users_nim sudah partial-aman utk NULL (banyak NULL boleh).
alter table public.users alter column nim drop not null;

alter table public.users
  add column if not exists tersembunyi   boolean      not null default false,  -- koordinator ninja
  add column if not exists divisi        varchar(32) references divisi (kode) on delete set null,  -- pembuka fitur asisten
  add column if not exists kode_asisten  varchar(10),
  add column if not exists is_aktif      boolean      not null default true;

create unique index if not exists uq_users_kode_asisten
  on public.users (kode_asisten) where (kode_asisten is not null);
create index if not exists idx_users_divisi on public.users (divisi);

alter table public.users
  add constraint users_wajib_nim_atau_email check (nim is not null or email is not null),
  add constraint users_divisi_hanya_staf
    check (divisi is null or role in ('asisten', 'koordinator'));

-- ---------- 4. IZIN & DIVISI_IZIN ----------
create type app_asal as enum ('praktikum', 'siakad', 'elearning');
create type peran_keanggotaan as enum ('peserta', 'pengampu', 'pj_absen');

create table if not exists izin (
  key       varchar(64) primary key,
  deskripsi varchar(255) not null,
  app       app_asal not null
);

create table if not exists divisi_izin (
  divisi   varchar(32) not null references divisi (kode) on delete cascade,
  izin_key varchar(64) not null references izin (key) on delete cascade,
  primary key (divisi, izin_key)
);

-- ---------- 5. KEANGGOTAAN (polimorfik peran kontekstual) ----------
-- kelas tabel LIVE dipakai apa adanya (id, nama_kelas).
-- users.kelas_id lama tetap (kompatibilitas Go); keanggotaan
-- jadi sumber kebenaran bertahap.
create table if not exists keanggotaan (
  id         bigserial primary key,
  user_id    integer not null references public.users (id) on delete cascade,
  kelas_id   integer not null references public.kelas (id) on delete cascade,
  peran      peran_keanggotaan not null default 'peserta',
  shift      integer,
  gelombang  integer,
  kelompok   varchar(50),
  created_at timestamptz not null default now(),
  unique (user_id, kelas_id, peran)
);
create index if not exists idx_keanggotaan_kelas on keanggotaan (kelas_id);

-- ---------- 6. SEED IZIN ----------
insert into izin (key, deskripsi, app) values
  ('manajemen-user',      'Kelola user (CRUD)',                       'siakad'),
  ('manajemen-jadwal',    'Edit jadwal praktikum',                    'siakad'),
  ('manajemen-kelas',     'Plotting kelas & asisten pengampu',        'siakad'),
  ('validasi-absensi',    'Validasi absensi',                         'siakad'),
  ('inventaris',          'Kelola inventaris lab',                    'siakad'),
  ('laporan-keuangan',    'Lihat/tulis laporan keuangan',             'siakad'),
  ('penunjang-praktikum', 'Kelola materi penunjang praktikum',        'siakad'),
  ('e-learning',          'Sinkron & pantau progres e-learning',      'siakad'),
  ('ketersediaan',        'Kelola jadwal ketersediaan asisten',       'siakad'),
  ('manajemen-sewa',      'Approval penyewaan alat',                  'siakad'),
  ('audit-log',           'Lihat audit log',                          'siakad'),
  ('absensi.export',      'Export rekap absensi (Absensi.tsx:306)',   'siakad'),
  ('absensi.penuh',       'Akses penuh halaman absensi (Absensi.tsx:335)', 'siakad')
on conflict (key) do nothing;

-- Seed HANYA yang terbukti dari kode (migrasi siakad 20260417010000:39-41,
-- Absensi.tsx:306,335). Divisi lain menunggu division_access prod (Luxey9).
insert into divisi_izin (divisi, izin_key) values
  ('sekretaris', 'manajemen-user'),
  ('sekretaris', 'laporan-keuangan'),
  ('sekretaris', 'absensi.export'),
  ('sekretaris', 'absensi.penuh'),
  ('k3',         'inventaris'),
  ('k3',         'absensi.penuh')
on conflict (divisi, izin_key) do nothing;

commit;
