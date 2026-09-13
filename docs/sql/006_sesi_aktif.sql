-- 006: sesi_aktif -- heartbeat "siapa sedang belajar sekarang".
-- Dipakai dashboard monitoring. Data sesaat, bukan riwayat: satu baris per user,
-- ditimpa tiap heartbeat. Riwayat permanen tetap di audit_logs.
--
-- Idempoten. Rollback: docs/sql/006_rollback.sql

create table if not exists public.sesi_aktif (
  user_id           integer primary key references public.users(id) on delete cascade,
  aktivitas         varchar(50) not null default 'lesson',
  detak_terakhir    timestamptz not null default now()
);

create index if not exists idx_sesi_aktif_detak on public.sesi_aktif (detak_terakhir desc);

-- Kolom yang dulu dikirim client (nama, kelas) sengaja TIDAK disimpan:
-- backend sudah tahu keduanya dari users + kelas lewat token, jadi client
-- tidak bisa mengaku jadi orang lain.
