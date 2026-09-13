-- AlgoHub 005: serap elearning ke praktikum DB (satu DB untuk 3 subdomain).
-- Sumber: Supabase elearning tvsawtkevzfqobsfkiag, 14 tabel / 572 baris.
--
-- Keputusan bentuk:
--   levels   -> level             (id text 'c-level-1', dipertahankan; dipakai FK modul)
--   modules  -> modul             (id text 'c-level-1-m1')
--   lessons  -> materi DIBENTUK ULANG (versi 004 cuma 4 kolom, tak muat 13 kolom lessons)
--   student_lessons -> TIDAK dipindah: itu salinan lessons minus kolom `solution`
--                      (penyembunyi kunci jawaban). Jadi filter kolom di Go, bukan tabel.
--   users.xp/level/streak -> profil_belajar (1:1 users, jangan kotori tabel identitas)
--   student_progress -> progres_belajar
--   achievements/unlocked -> pencapaian / pencapaian_terbuka
--   game_* -> game_soal / game_riwayat / game_konfigurasi
--   playground_examples -> playground_contoh
--   activity_logs -> TIDAK dipindah (log lama elearning, 104 baris; bukan data operasional).
--
-- Semua yang menunjuk orang pakai users.id (integer), BUKAN nim text.
-- Idempoten: aman diulang. Rollback: 005_rollback.sql
--
-- PRASYARAT: materi harus kosong (sudah diverifikasi: 0 baris, nol endpoint).

begin;

-- ---------------------------------------------------------------
-- 0) MATERI dibentuk ulang agar muat lessons
-- ---------------------------------------------------------------
-- Buang materi HANYA jika masih bentuk 004 (punya kolom `konten`).
-- Kalau sudah bentuk 005, jangan disentuh -- menjaga data saat dijalankan ulang.
do $$
begin
  if exists (
    select 1 from information_schema.columns
    where table_schema='public' and table_name='materi' and column_name='konten'
  ) then
    drop table public.materi cascade;
  end if;
end $$;

create table if not exists public.level (
  id          varchar(64) primary key,
  judul       varchar(200) not null,
  deskripsi   text,
  mode_akses  varchar(20) not null default 'auto',
  terkunci    boolean not null default false,
  urutan      integer not null default 0,
  constraint level_mode_akses_check check (mode_akses in ('auto','manual'))
);

create table if not exists public.modul (
  id        varchar(64) primary key,
  level_id  varchar(64) not null references public.level(id) on delete cascade,
  judul     varchar(200) not null,
  urutan    integer not null default 0
);
create index if not exists idx_modul_level on public.modul (level_id, urutan);

create table if not exists public.materi (
  id               varchar(64) primary key,
  modul_id         varchar(64) not null references public.modul(id) on delete cascade,
  judul            varchar(200) not null,
  penjelasan       text,
  contoh_kode      text,
  kode_awal        text,
  solusi           text,
  petunjuk         text,
  kuis             jsonb,
  kasus_uji        jsonb,
  aturan_validasi  jsonb,
  urutan           integer not null default 0,
  xp_hadiah        integer not null default 0 check (xp_hadiah >= 0),
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now()
);
create index if not exists idx_materi_modul on public.materi (modul_id, urutan);

-- View tanpa `solusi` -> pengganti student_lessons (96 baris duplikat batal dipindah).
create or replace view public.materi_mahasiswa as
  select id, modul_id, judul, penjelasan, contoh_kode, kode_awal,
         petunjuk, kuis, kasus_uji, aturan_validasi, urutan, xp_hadiah
  from public.materi;

-- ---------------------------------------------------------------
-- 1) PROFIL BELAJAR (xp/level/streak) — 1:1 users
-- ---------------------------------------------------------------
create table if not exists public.profil_belajar (
  user_id        integer primary key references public.users(id) on delete cascade,
  xp             integer not null default 0 check (xp >= 0),
  level_angka    integer not null default 1 check (level_angka >= 1),
  streak         integer not null default 0 check (streak >= 0),
  waktu_belajar  integer not null default 0 check (waktu_belajar >= 0),
  terakhir_aktif timestamptz,
  akses_level    jsonb not null default '{}'::jsonb,
  akses_asesmen  jsonb not null default '{}'::jsonb
);

-- ---------------------------------------------------------------
-- 2) PROGRES BELAJAR
-- ---------------------------------------------------------------
create table if not exists public.progres_belajar (
  id           serial primary key,
  user_id      integer not null references public.users(id) on delete cascade,
  materi_id    varchar(64) not null references public.materi(id) on delete cascade,
  selesai      boolean not null default false,
  selesai_pada timestamptz,
  constraint uq_progres_user_materi unique (user_id, materi_id)
);
create index if not exists idx_progres_user on public.progres_belajar (user_id, selesai);

-- ---------------------------------------------------------------
-- 3) PENCAPAIAN
-- ---------------------------------------------------------------
create table if not exists public.pencapaian (
  id             varchar(64) primary key,
  judul          varchar(150) not null,
  deskripsi      text,
  ikon           varchar(100),
  syarat_tipe    varchar(50),
  syarat_nilai   varchar(64)          -- angka ATAU id level (mis. 'c-level-1')
);

create table if not exists public.pencapaian_terbuka (
  id            serial primary key,
  user_id       integer not null references public.users(id) on delete cascade,
  pencapaian_id varchar(64) not null references public.pencapaian(id) on delete cascade,
  dibuka_pada   timestamptz not null default now(),
  constraint uq_pencapaian_user unique (user_id, pencapaian_id)
);

-- ---------------------------------------------------------------
-- 4) GAME (bug hunt)
-- ---------------------------------------------------------------
create table if not exists public.game_soal (
  id          serial primary key,
  bahasa      varchar(20) not null,
  kesulitan   varchar(20) not null,
  judul       varchar(200) not null,
  kode        text not null,
  baris_bug   integer,
  penjelasan  text
);
create index if not exists idx_game_soal_bahasa on public.game_soal (bahasa, kesulitan);

create table if not exists public.game_riwayat (
  id           serial primary key,
  user_id      integer not null references public.users(id) on delete cascade,
  jenis_game   varchar(50) not null,
  xp_didapat   integer not null default 0 check (xp_didapat >= 0),
  dimainkan    timestamptz not null default now()
);
create index if not exists idx_game_riwayat_user on public.game_riwayat (user_id, dimainkan);

create table if not exists public.game_konfigurasi (
  id                 varchar(32) primary key default 'default',
  bug_hunt_aktif     boolean not null default true,
  bug_hunt_c_aktif   boolean not null default true,
  bug_hunt_py_aktif  boolean not null default false,
  batas_mingguan     integer not null default 1 check (batas_mingguan >= 0)
);

-- ---------------------------------------------------------------
-- 5) PLAYGROUND
-- ---------------------------------------------------------------
create table if not exists public.playground_contoh (
  id         serial primary key,
  judul      varchar(200) not null,
  bahasa     varchar(20) not null,
  kode       text not null,
  deskripsi  text,
  urutan     integer not null default 0,
  created_at timestamptz not null default now()
);

commit;

-- verifikasi
select 'level' t, count(*) n from public.level
union all select 'modul', count(*) from public.modul
union all select 'materi', count(*) from public.materi
union all select 'profil_belajar', count(*) from public.profil_belajar
union all select 'progres_belajar', count(*) from public.progres_belajar
union all select 'pencapaian', count(*) from public.pencapaian
union all select 'pencapaian_terbuka', count(*) from public.pencapaian_terbuka
union all select 'game_soal', count(*) from public.game_soal
union all select 'game_riwayat', count(*) from public.game_riwayat
union all select 'game_konfigurasi', count(*) from public.game_konfigurasi
union all select 'playground_contoh', count(*) from public.playground_contoh
order by 1;
