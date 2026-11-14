-- Idempotent canonical seed. No production users or passwords are included.
insert into public.roles (code, name) values
  ('koordinator', 'Koordinator'),
  ('asisten', 'Asisten'),
  ('mahasiswa', 'Mahasiswa'),
  ('peminjam', 'Peminjam alat')
on conflict (code) do update set name = excluded.name;

insert into public.divisions (code, name) values
  ('sekretaris', 'Sekretaris'),
  ('bendahara', 'Bendahara'),
  ('k3', 'K3'),
  ('labops', 'Labops'),
  ('pendidikan', 'Pendidikan'),
  ('pdd', 'PDD')
on conflict (code) do update set name = excluded.name, is_active = true;

insert into public.permissions (code, app, description) values
  ('admin.overview.read', 'admin', 'Melihat ringkasan console admin'),
  ('user.read', 'all', 'Membaca user dalam scope'),
  ('user.write', 'all', 'Mengubah user dalam scope'),
  ('schedule.write', 'siakad', 'Mengelola jadwal'),
  ('schedule.approve', 'praktikum', 'Menyetujui jadwal praktikum'),
  ('class.manage', 'siakad', 'Mengelola kelas'),
  ('attendance.read', 'siakad', 'Membaca absensi'),
  ('inventory.manage', 'siakad', 'Mengelola inventaris'),
  ('rental.approve', 'siakad', 'Menyetujui penyewaan'),
  ('finance.manage', 'siakad', 'Mengelola data keuangan'),
  ('learning.content.read', 'elearning', 'Membaca konten pembelajaran'),
  ('learning.content.manage', 'elearning', 'Mengelola konten pembelajaran'),
  ('learning.monitor', 'elearning', 'Memantau pembelajaran'),
  ('practicum.session.manage', 'praktikum', 'Mengelola sesi praktikum'),
  ('question.manage', 'praktikum', 'Mengelola bank soal'),
  ('practicum.grade.write', 'praktikum', 'Mengubah penilaian'),
  ('audit.read', 'all', 'Membaca audit log sesuai scope'),
  ('settings.manage', 'all', 'Mengelola pengaturan')
on conflict (code) do update set app = excluded.app, description = excluded.description;

insert into public.role_permissions (role_code, permission_code, default_scope)
select 'koordinator', code, '{"domain":"*"}'::jsonb
from public.permissions
where code in ('admin.overview.read', 'user.read', 'user.write', 'class.manage', 'learning.monitor', 'practicum.session.manage', 'audit.read', 'settings.manage')
on conflict (role_code, permission_code) do update set default_scope = excluded.default_scope;

insert into public.role_permissions (role_code, permission_code, default_scope)
values ('mahasiswa', 'learning.content.read', '{"domain":"elearning"}'::jsonb)
on conflict (role_code, permission_code) do update set default_scope = excluded.default_scope;

insert into public.role_permissions (role_code, permission_code, default_scope)
values ('asisten', 'learning.content.read', '{"domain":"elearning"}'::jsonb)
on conflict (role_code, permission_code) do update set default_scope = excluded.default_scope;

insert into public.division_permissions (division_code, permission_code, default_scope)
select division_code, permission_code, scope
from (values
  ('sekretaris', 'admin.overview.read', '{"domain":"admin"}'::jsonb),
  ('sekretaris', 'schedule.write', '{"domain":"siakad","division":"sekretaris"}'::jsonb),
  ('sekretaris', 'schedule.approve', '{"domain":"praktikum","division":"sekretaris"}'::jsonb),
  ('bendahara', 'admin.overview.read', '{"domain":"admin"}'::jsonb),
  ('bendahara', 'finance.manage', '{"domain":"siakad","division":"bendahara"}'::jsonb),
  ('k3', 'admin.overview.read', '{"domain":"admin"}'::jsonb),
  ('k3', 'inventory.manage', '{"domain":"siakad","division":"k3"}'::jsonb),
  ('k3', 'rental.approve', '{"domain":"siakad","division":"k3"}'::jsonb),
  ('labops', 'admin.overview.read', '{"domain":"admin"}'::jsonb),
  ('labops', 'inventory.manage', '{"domain":"siakad","division":"labops"}'::jsonb),
  ('labops', 'rental.approve', '{"domain":"siakad","division":"labops"}'::jsonb),
  ('pendidikan', 'admin.overview.read', '{"domain":"admin"}'::jsonb),
  ('pendidikan', 'learning.content.manage', '{"domain":"elearning","division":"pendidikan"}'::jsonb),
  ('pendidikan', 'learning.monitor', '{"domain":"elearning","division":"pendidikan"}'::jsonb),
  ('pdd', 'admin.overview.read', '{"domain":"admin"}'::jsonb)
) as grants(division_code, permission_code, scope)
on conflict (division_code, permission_code) do update set default_scope = excluded.default_scope;
