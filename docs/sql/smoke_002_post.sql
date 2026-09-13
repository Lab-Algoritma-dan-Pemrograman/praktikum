-- ============================================================
-- AlgoHub — smoke_002_post.sql
-- Data UJI + verifikasi: jalankan SETELAH 002_evolusi_identitas.sql
-- di replika schema live praktikum. DB uji dibuang setelah selesai.
-- Bagian TOLAKAN memang harus ERROR (constraint bekerja).
-- ============================================================

-- ---------- 1. KONVERSI ROLE + IZIN DIVISI ----------
select u.id, u.nim, u.nama, u.role, u.divisi, u.tersembunyi, u.is_aktif,
       coalesce(string_agg(di.izin_key, ', ' order by di.izin_key), '-') as izin
from public.users u
left join divisi_izin di on di.divisi = u.divisi
where u.id in (1, 2, 3, 4)
group by u.id
order by u.id;

-- ---------- 2. KEANGGOTAAN polimorfik (asisten=pengampu, mhs=peserta, kelas sama) ----------
insert into keanggotaan (user_id, kelas_id, peran) values
  (2, 1, 'pengampu'),
  (3, 1, 'peserta');
select k.nama_kelas, u.nama, ke.peran
from keanggotaan ke
join public.users u on u.id = ke.user_id
join public.kelas k on k.id = ke.kelas_id
order by k.nama_kelas, ke.peran desc;

-- ---------- 3. DIVISI EXTENSIBLE (koordinator tambah divisi baru + atur hak, tanpa DDL) ----------
insert into divisi (kode, nama) values ('humas', 'Humas');
insert into divisi_izin (divisi, izin_key) values ('humas', 'inventaris');
update public.users set divisi = 'humas' where id = 2;
select u.nama, u.divisi,
       coalesce(string_agg(di.izin_key, ', ' order by di.izin_key), '-') as izin
from public.users u
left join divisi_izin di on di.divisi = u.divisi
where u.id = 2
group by u.id, u.nama, u.divisi;

-- ---------- 4. TOLAKAN (constraint bekerja — harus ERROR) ----------
insert into public.users (id, role, nim, nama, divisi) values (99, 'mahasiswa', '999', 'SalahDivisi', 'k3');  -- users_divisi_hanya_staf
insert into public.users (id, role, nim, nama) values (98, 'admin', '998', 'RoleLama');                       -- users_role_check (kosakata lama)
insert into divisi_izin (divisi, izin_key) values ('ngaco', 'inventaris');                                    -- FK divisi
insert into divisi_izin (divisi, izin_key) values ('humas', 'izin-ngaco');                                    -- FK izin
