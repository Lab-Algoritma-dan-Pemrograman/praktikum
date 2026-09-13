-- Rollback 005. Mengembalikan materi ke bentuk 004.
-- PERINGATAN: menghapus seluruh data elearning yang sudah dipindah.
-- Backup: backups/pre005_*.sql

begin;

drop view  if exists public.materi_mahasiswa;
drop table if exists public.playground_contoh  cascade;
drop table if exists public.game_konfigurasi   cascade;
drop table if exists public.game_riwayat       cascade;
drop table if exists public.game_soal          cascade;
drop table if exists public.pencapaian_terbuka cascade;
drop table if exists public.pencapaian         cascade;
drop table if exists public.progres_belajar    cascade;
drop table if exists public.profil_belajar     cascade;
drop table if exists public.materi             cascade;
drop table if exists public.modul              cascade;
drop table if exists public.level              cascade;

-- materi bentuk 004 dipulihkan
create table if not exists public.materi (
  id          serial primary key,
  sesi_id     integer references public.sesi_praktikum(id) on delete set null,
  judul       varchar(200) not null,
  konten      text,
  urutan      integer not null default 0,
  is_publik   boolean not null default false,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now()
);
create index if not exists idx_materi_sesi on public.materi (sesi_id, urutan);

commit;

select 'sisa_tabel_005' t, count(*) n
from information_schema.tables
where table_schema='public'
  and table_name in ('level','modul','profil_belajar','progres_belajar',
                     'pencapaian','pencapaian_terbuka','game_soal','game_riwayat',
                     'game_konfigurasi','playground_contoh');
