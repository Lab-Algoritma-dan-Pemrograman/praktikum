-- Rollback 006.
drop index if exists public.idx_sesi_aktif_detak;
drop table if exists public.sesi_aktif;
