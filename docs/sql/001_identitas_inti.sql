-- ============================================================
-- AlgoHub — 001_identitas_inti.sql
-- Inti identitas & izin: users, izin, divisi_izin, kelas,
-- keanggotaan, audit_log. Sesuai docs/ERD-gabungan.md.
-- Status: rancangan, diuji di Postgres lokal (container uji).
-- RLS menyusul saat diterapkan di Supabase (ERD §8).
-- ============================================================

begin;

-- ---------- ENUM ----------
create type peran_pengguna as enum ('koordinator', 'asisten', 'mahasiswa', 'peminjam');
create type divisi_asisten as enum ('k3', 'sekretaris', 'bendahara', 'labops', 'pendidikan', 'pdd');
create type app_asal as enum ('praktikum', 'siakad', 'elearning');
create type peran_keanggotaan as enum ('peserta', 'pengampu', 'pj_absen');

-- ---------- USERS (1 akun, 3 subdomain) ----------
-- ponytail: updated_at dikelola app (GORM auto-update); tambahkan
-- trigger modtime() jika RPC Supabase mulai menulis tabel ini.
create table users (
  id              bigserial primary key,
  nim             varchar(32) unique,
  email           varchar(150) unique,
  nama            varchar(150) not null,
  password_hash   varchar(255),
  role            peran_pengguna not null default 'mahasiswa',
  tersembunyi     boolean not null default false,  -- koordinator ninja: tak tampil di daftar user
  divisi          divisi_asisten,                 -- fitur asisten terbuka via divisi (ketua lab, Sep 2026)
  kode_asisten    varchar(10) unique,
  is_aktif        boolean not null default true,
  is_terdaftar    boolean not null default false,
  supabase_user_id uuid unique,
  foto_url        varchar(500),
  nomor_hp        varchar(30),
  medsos_link     varchar(500),
  last_login_at   timestamptz,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  constraint users_wajib_nim_atau_email check (nim is not null or email is not null),
  constraint users_divisi_hanya_staf check (divisi is null or role in ('asisten', 'koordinator'))
);
create index users_role_idx on users (role);
create index users_divisi_idx on users (divisi);

-- ---------- IZIN ----------
-- Kunci memakai kosakata menu siakad (division_access.menu_key di prod)
-- supaya pemetaan data produksi nanti 1:1 (ERD §9.1).
create table izin (
  key       varchar(64) primary key,
  deskripsi varchar(255) not null,
  app       app_asal not null
);

create table divisi_izin (
  divisi   divisi_asisten not null,
  izin_key varchar(64) not null references izin (key) on delete cascade,
  primary key (divisi, izin_key)
);

-- ---------- KELAS & KEANGGOTAAN ----------
-- Polimorfik peran kontekstual: satu orang bisa peserta kelas A
-- sekaligus pengampu kelas B (kolom users.kelas_id lama tak bisa).
create table kelas (
  id         bigserial primary key,
  kode       varchar(32) unique not null,
  jurusan    varchar(100),
  created_at timestamptz not null default now()
);

create table keanggotaan (
  id         bigserial primary key,
  user_id    bigint not null references users (id) on delete cascade,
  kelas_id   bigint not null references kelas (id) on delete cascade,
  peran      peran_keanggotaan not null default 'peserta',
  shift      int,
  gelombang  int,
  kelompok   varchar(50),
  created_at timestamptz not null default now(),
  unique (user_id, kelas_id, peran)
);
create index keanggotaan_kelas_idx on keanggotaan (kelas_id);

-- ---------- AUDIT LOG (polimorfik, 3 app) ----------
create table audit_log (
  id          bigserial primary key,
  aktor_id    bigint references users (id) on delete set null,  -- NULL utk login gagal
  aksi        varchar(32) not null,
  subjek_tipe varchar(64) not null,
  subjek_id   bigint,
  app         app_asal not null,
  detail      jsonb,
  ip          inet,
  created_at  timestamptz not null default now()
);
create index audit_log_subjek_idx on audit_log (subjek_tipe, subjek_id);
create index audit_log_aktor_idx on audit_log (aktor_id, created_at desc);

-- ---------- SEED: katalog izin ----------
-- Sumber: RESTRICTED_MENUS siakad (AppSidebar.tsx:24) + cek hardcoded Absensi.tsx.
insert into izin (key, deskripsi, app) values
  ('manajemen-user',       'Kelola user (CRUD)',                      'siakad'),
  ('manajemen-jadwal',     'Edit jadwal praktikum',                   'siakad'),
  ('manajemen-kelas',      'Plotting kelas & asisten pengampu',       'siakad'),
  ('validasi-absensi',     'Validasi absensi',                        'siakad'),
  ('inventaris',           'Kelola inventaris lab',                   'siakad'),
  ('laporan-keuangan',     'Lihat/tulis laporan keuangan',            'siakad'),
  ('penunjang-praktikum',  'Kelola materi penunjang praktikum',       'siakad'),
  ('e-learning',           'Sinkron & pantau progres e-learning',     'siakad'),
  ('ketersediaan',         'Kelola jadwal ketersediaan asisten',      'siakad'),
  ('manajemen-sewa',       'Approval penyewaan alat',                 'siakad'),
  ('audit-log',            'Lihat audit log',                         'siakad'),
  ('absensi.export',       'Export rekap absensi (Absensi.tsx:306)',  'siakad'),
  ('absensi.penuh',        'Akses penuh halaman absensi (Absensi.tsx:335)', 'siakad');

-- ---------- SEED: divisi_izin (HANYA yang terbukti dari kode) ----------
-- Bukti: siakad/supabase/migrations/20260417010000_production_sync_fix.sql:39-41
--        + siakad/src/pages/Absensi.tsx:306,335
insert into divisi_izin (divisi, izin_key) values
  ('sekretaris', 'manajemen-user'),
  ('sekretaris', 'laporan-keuangan'),
  ('sekretaris', 'absensi.export'),
  ('sekretaris', 'absensi.penuh'),
  ('k3',         'inventaris'),
  ('k3',         'absensi.penuh');

-- TEBAK — JANGAN dipakai sebelum dicek ke tabel division_access prod (ERD §9.1):
--   bendahara  -> laporan-keuangan
--   labops     -> inventaris, manajemen-sewa
--   pendidikan -> penunjang-praktikum, e-learning
--   pdd        -> ? (arti/ruang lingkup PDD belum dikonfirmasi)

commit;
