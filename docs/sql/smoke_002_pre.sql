-- ============================================================
-- AlgoHub — smoke_002_pre.sql
-- Data UJI format LAMA: jalankan di replika schema live
-- PRAKTIKUM, SEBELUM 002_evolusi_identitas.sql.
-- id eksplisit karena users.id live tak ber-default.
-- ============================================================

begin;
insert into public.users (id, role, nim, nama, email, is_registered)
values (1, 'superadmin', '2024111006', 'Kordas Lama', 'kordas@x.id', true),
       (2, 'admin',      '2023111022', 'Asisten Lama', null, true),
       (3, 'user',       '2025111033', 'Mhs Lama', null, false),
       (4, 'user',       '2025111044', 'Mhs Kedua', null, false);

insert into public.kelas (id, nama_kelas) values (1, 'TTL A');
commit;
