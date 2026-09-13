-- AlgoHub 004 ROLLBACK: buang tabel domain baru.
-- PERINGATAN: menghapus data di 4 tabel ini.
begin;
drop table if exists public.peminjaman;
drop table if exists public.inventaris;
drop table if exists public.absensi;
drop table if exists public.materi;
commit;

select count(*) as sisa_tabel_004
from information_schema.tables
where table_schema='public'
  and table_name in ('materi','absensi','inventaris','peminjaman');
