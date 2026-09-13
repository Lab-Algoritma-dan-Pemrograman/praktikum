-- ============================================================
-- AlgoHub — smoke_test_001.sql
-- Data UJI, bukan migrasi. Jalankan setelah 001_identitas_inti.sql
-- di DB uji (container dibuang setelahnya).
-- ============================================================

begin;

-- 4 role berbeda + peminjam tanpa NIM
insert into users (nim, email, nama, role, tersembunyi) values
  ('2024111006', 'kordas@itpln.ac.id', 'Koordinator Ninja', 'koordinator', true);

insert into users (nim, nama, role, divisi, kode_asisten) values
  ('2023111022', 'Naufal Sekretaris', 'asisten', 'sekretaris', 'NS');

insert into users (nim, nama) values
  ('2025111033', 'Mahasiswa Biasa');  -- default role mahasiswa

insert into users (email, nama, role) values
  ('luar@gmail.com', 'Peminjam Luar', 'peminjam');  -- nim NULL, wajib punya email

-- keanggotaan polimorfik: asisten jadi pengampu, mahasiswa jadi peserta
insert into kelas (kode, jurusan) values ('TTL A', 'Teknik Tenaga Listrik');

insert into keanggotaan (user_id, kelas_id, peran)
  select id, (select id from kelas where kode = 'TTL A'), 'pengampu'
  from users where kode_asisten = 'NS';

insert into keanggotaan (user_id, kelas_id, peran, shift)
  select id, (select id from kelas where kode = 'TTL A'), 'peserta', 1
  from users where nim = '2025111033';

-- audit log polimorfik
insert into audit_log (aktor_id, aksi, subjek_tipe, subjek_id, app, detail, ip)
  select id, 'LOGIN', 'user', id, 'siakad', '{"hasil":"ok"}', '10.0.0.5'
  from users where nim = '2023111022';

commit;

-- ---------- verifikasi ----------
select u.nim, u.nama, u.role, u.divisi,
       coalesce(string_agg(di.izin_key, ', ' order by di.izin_key), '-') as izin_divisi
from users u
left join divisi_izin di on di.divisi = u.divisi
group by u.id, u.nim, u.nama, u.role, u.divisi
order by u.id;

select k.kode, u.nama, m.peran
from keanggotaan m
join kelas k on k.id = m.kelas_id
join users u on u.id = m.user_id
order by k.kode, m.peran;
