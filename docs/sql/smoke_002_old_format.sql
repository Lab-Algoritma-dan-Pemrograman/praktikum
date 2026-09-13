-- ============================================================
-- AlgoHub — smoke_002_old_format.sql
-- Data UJI format LAMA (sebelum 002): disisipkan ke replika
-- schema live praktikum SEBELUM 002 dijalankan, untuk membuktikan
-- konversi role bekerja pada data bentuk produksi.
-- ============================================================

begin;
insert into public.users (role, nim, nama, email, is_registered)
values ('superadmin', '2024111006', 'Kordas Lama', 'kordas@x.id', true),
       ('admin',      '2023111022', 'Asisten Lama', null, true),
       ('user',       '2025111033', 'Mhs Lama', null, false),
       ('user',       '2025111044', 'Mhs Kedua', null, false);

insert into public.kelas (nama_kelas) values ('TTL A');

-- keanggotaan: asisten lama jadi pengampu TTL A, mhs jadi peserta
insert into keanggotaan (user_id, kelas_id, peran)
select u.id, k.id, 'pengampu'
from public.users u, public.kelas k
where u.nim = '2023111022' and k.nama_kelas = 'TTL A';

insert into keanggotaan (user_id, kelas_id, peran, shift)
select u.id, k.id, 'peserta', 1
from public.users u, public.kelas k
where u.nim = '2025111033' and k.nama_kelas = 'TTL A';
commit;

-- ---------- VERIFIKASI PASCA-002 ----------
select nim, nama, role, divisi, tersembunyi, is_aktif,
       coalesce(string_agg(di.izin_key, ', ' order by di.izin_key), '-') as izin
from public.users u
left join divisi_izin di on di.divisi = u.divisi
group by u.id, u.nim, u.nama, u.role, u.divisi, u.tersembunyi, u.is_aktif
order by u.id;

select k.nama_kelas, u.nama, m.peran
from keanggotaan m
join public.kelas k on k.id = m.kelas_id
join public.users u on u.id = m.user_id
order by k.nama_kelas, m.peran;
