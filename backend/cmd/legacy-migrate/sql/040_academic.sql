-- Academic domain: classes, schedules, attendance sessions/records, and the
-- operational documents that keep the remaining academic rows intact.
-- Attendance sessions are keyed per (qr session, meeting) because a single qr
-- session covers several meetings; rows that would collide inside one meeting
-- are kept in academic_operations instead of being dropped.
drop view if exists legacy_kelas, legacy_keanggotaan, legacy_jadwal, legacy_qr, legacy_att, legacy_schedules;

create temp view legacy_kelas as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.kelas';
create temp view legacy_keanggotaan as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.keanggotaan';
create temp view legacy_jadwal as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.jadwal';
create temp view legacy_qr as
select row_data, (row_data->>'id') as legacy_id from legacy_preserve.rows where source_table = 'public.qr_sessions';
create temp view legacy_att as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.attendance_logs';
create temp view legacy_schedules as
select row_data from legacy_preserve.rows where source_table = 'public.schedules';

insert into public.academic_classes (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-class-' || k.legacy_id, '', 0,
  jsonb_strip_nulls(jsonb_build_object(
    'class_id', 'legacy-class-' || k.legacy_id,
    'shift', (select max(kk.row_data->>'shift')::int from legacy_keanggotaan kk
              where (kk.row_data->>'kelas_id')::bigint = k.legacy_id and kk.row_data->>'shift' ~ '^[0-9]+$'))),
  'active',
  'legacy:kelas:' || k.legacy_id,
  jsonb_build_object(
    'id', 'legacy-class-' || k.legacy_id,
    'code', coalesce(nullif(btrim(k.row_data->>'nama_kelas'), ''), 'Kelas ' || k.legacy_id),
    'course_code', '',
    'course_name', coalesce(k.row_data->>'nama_kelas', ''),
    'period_name', '',
    'shift', least(greatest(coalesce((select max(kk.row_data->>'shift')::int from legacy_keanggotaan kk
                                     where (kk.row_data->>'kelas_id')::bigint = k.legacy_id
                                       and kk.row_data->>'shift' ~ '^[0-9]+$'), 1), 1), 2),
    'student_ids', coalesce((
      select jsonb_agg(distinct (kk.row_data->>'user_id')::bigint)
      from legacy_keanggotaan kk
      join public.users u on u.id = (kk.row_data->>'user_id')::bigint
      where (kk.row_data->>'kelas_id')::bigint = k.legacy_id), '[]'::jsonb),
    'assistant_ids', coalesce((
      select jsonb_agg(distinct (gm.row_data->>'assistant_id')::bigint)
      from legacy_preserve.rows gm
      join public.users u on u.id = (gm.row_data->>'assistant_id')::bigint
      where gm.source_table = 'public.group_members'
        and (gm.row_data->>'schedule_id')::bigint in (
          select (j.row_data->>'id')::bigint from legacy_jadwal j
          where (j.row_data->>'kelas_id')::bigint = k.legacy_id)), '[]'::jsonb),
    'active', true,
    'scope', '{}'::jsonb,
    'version', 1,
    'created_at', now(),
    'updated_at', now(),
    'legacy', k.row_data
  )
from legacy_kelas k
on conflict (id) do update set scope = excluded.scope, data = excluded.data, status = excluded.status;

insert into public.academic_schedules (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-schedule-' || j.legacy_id, 'legacy-class-' || (j.row_data->>'kelas_id'), 0,
  jsonb_strip_nulls(jsonb_build_object('class_id', 'legacy-class-' || (j.row_data->>'kelas_id'))),
  'active',
  'legacy:jadwal:' || j.legacy_id,
  jsonb_build_object(
    'id', 'legacy-schedule-' || j.legacy_id,
    'class_id', 'legacy-class-' || (j.row_data->>'kelas_id'),
    'class_code', coalesce(k.row_data->>'nama_kelas', ''),
    'course_name', coalesce(k.row_data->>'nama_kelas', ''),
    'day_of_week', case upper(btrim(coalesce(j.row_data->>'hari', '')))
                     when 'SENIN' then 'Senin' when 'SELASA' then 'Selasa' when 'RABU' then 'Rabu'
                     when 'KAMIS' then 'Kamis' when 'JUMAT' then 'Jumat' when 'JUM''AT' then 'Jumat'
                     when 'SABTU' then 'Sabtu' else 'Senin' end,
    'shift', least(greatest(coalesce((j.row_data->>'shift')::int, 1), 1), 2),
    'room', coalesce(j.row_data->>'keterangan', ''),
    'starts_at', coalesce(j.row_data->>'jam_mulai', ''),
    'ends_at', coalesce(j.row_data->>'jam_selesai', ''),
    'scope', '{}'::jsonb,
    'version', 1,
    'legacy', j.row_data
  )
from legacy_jadwal j
left join legacy_kelas k on k.legacy_id = (j.row_data->>'kelas_id')::bigint
where exists (select 1 from public.academic_classes c where c.id = 'legacy-class-' || (j.row_data->>'kelas_id'))
on conflict (id) do update set parent_id = excluded.parent_id, scope = excluded.scope, data = excluded.data, status = excluded.status;

insert into public.academic_classes (id, parent_id, owner_id, scope, status, unique_key, data)
values ('legacy-class-container', '', 0, '{"shift":1}'::jsonb, 'container', 'legacy:class-container',
  jsonb_build_object('id', 'legacy-class-container', 'code', 'legacy', 'course_code', '', 'course_name',
    'Legacy attendance container', 'period_name', '', 'shift', 1, 'active', false, 'scope', '{}'::jsonb, 'version', 1))
on conflict (id) do nothing;

insert into public.academic_schedules (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-schedule-container', 'legacy-class-container', 0, '{}'::jsonb, 'container', 'legacy:schedule-container',
  jsonb_build_object('id', 'legacy-schedule-container', 'class_id', '', 'class_code', 'legacy',
    'course_name', 'Legacy QR attendance container', 'day_of_week', 'Senin', 'shift', 1, 'room', '',
    'starts_at', '', 'ends_at', '', 'scope', '{}'::jsonb, 'version', 1)
on conflict (id) do nothing;

with keys as (
  select distinct q.legacy_id as qr_id, coalesce(a.row_data->>'meeting', '') as meeting_key
  from legacy_att a
  left join legacy_qr q on q.legacy_id = (a.row_data->>'session_id')
  union
  select q.legacy_id, '' from legacy_qr q
)
insert into public.academic_attendance_sessions (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-att-session-' || coalesce(k.qr_id::text, 'null') || '-' || md5(k.meeting_key),
  'legacy-schedule-container',
  0,
  '{}'::jsonb,
  case when coalesce((q.row_data->>'is_active')::boolean, false) then 'open' else 'closed' end,
  'legacy:attendance_session:' || coalesce(k.qr_id::text, 'null') || ':' || md5(k.meeting_key),
  jsonb_build_object(
    'session', jsonb_build_object(
      'id', 'legacy-att-session-' || coalesce(k.qr_id::text, 'null') || '-' || md5(k.meeting_key),
      'schedule_id', 'legacy-schedule-container',
      'class_id', '',
      'title', coalesce(nullif(q.row_data->>'title', ''), 'Presensi ' || coalesce(k.meeting_key, '')),
      'is_open', coalesce((q.row_data->>'is_active')::boolean, false),
      'opened_at', coalesce((q.row_data->>'created_at')::timestamptz, now()),
      'expires_at', coalesce((q.row_data->>'created_at')::timestamptz, now()) + interval '3650 days',
      'closed_at', case when coalesce((q.row_data->>'is_active')::boolean, false) then null else now() end,
      'access_token', coalesce(q.row_data->>'token', ''),
      'scope', '{}'::jsonb,
      'version', 1,
      'meeting', k.meeting_key,
      'legacy_qr_session_id', k.qr_id,
      'legacy_previous_token', q.row_data->'previous_token'
    ),
    'token_hash', case when coalesce(q.row_data->>'token', '') = ''
      then encode(sha256(convert_to('legacy-' || coalesce(k.qr_id::text, 'null') || '-' || k.meeting_key, 'UTF8')), 'hex')
      else encode(sha256(convert_to(q.row_data->>'token', 'UTF8')), 'hex') end,
    'legacy_qr', q.row_data
  )
from keys k
left join legacy_qr q on q.legacy_id = k.qr_id
on conflict (id) do nothing;

insert into public.academic_attendance_records (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-att-' || t.legacy_id,
  t.session_key,
  t.user_id,
  '{}'::jsonb,
  t.status,
  'legacy:attendance_logs:' || t.legacy_id,
  jsonb_build_object(
    'id', 'legacy-att-' || t.legacy_id,
    'session_id', t.session_key,
    'schedule_id', 'legacy-schedule-container',
    'class_id', '',
    'user_id', t.user_id,
    'user_name', coalesce(t.user_name, ''),
    'status', t.status,
    'verified_by_qr', coalesce((t.row_data->>'is_verified')::boolean, false),
    'validated', coalesce((t.row_data->>'verification_status') = 'verified', false),
    'checked_in_at', coalesce((t.row_data->>'check_in_time')::timestamptz, now()),
    'validated_by', t.recorded_by,
    'validated_at', coalesce((t.row_data->>'check_in_time')::timestamptz, now()),
    'notes', coalesce(t.row_data->>'notes', ''),
    'scope', '{}'::jsonb,
    'version', 1,
    'legacy', t.row_data
  )
from (
  select
    a.legacy_id,
    a.row_data,
    (a.row_data->>'custom_user_id')::bigint as user_id,
    'legacy-att-session-' || coalesce((a.row_data->>'session_id'), 'null') || '-' || md5(coalesce(a.row_data->>'meeting', '')) as session_key,
    case upper(btrim(coalesce(a.row_data->>'status', '')))
      when 'HADIR' then 'hadir'
      when 'IZIN' then 'izin'
      when 'SAKIT' then 'izin'
      when 'ALPHA' then 'alpha'
      when 'TERLAMBAT' then 'terlambat'
      else 'alpha' end as status,
    u.name as user_name,
    nullif(a.row_data->>'recorded_by', '')::bigint as recorded_by,
    row_number() over (
      partition by coalesce(a.row_data->>'session_id', 'null'), coalesce(a.row_data->>'meeting', ''), a.row_data->>'custom_user_id'
      order by coalesce((a.row_data->>'check_in_time')::timestamptz, 'epoch'::timestamptz) desc, a.legacy_id desc
    ) as rank_in_session
  from legacy_att a
  left join public.users u on u.id = coalesce((a.row_data->>'custom_user_id')::bigint, 0)
  where exists (select 1 from public.academic_attendance_sessions s
                where s.id = 'legacy-att-session-' || coalesce((a.row_data->>'session_id'), 'null') || '-' || md5(coalesce(a.row_data->>'meeting', '')))
    and exists (select 1 from public.users u2 where u2.id = coalesce((a.row_data->>'custom_user_id')::bigint, 0))
) t
where t.rank_in_session = 1
on conflict (parent_id, owner_id) do nothing;

-- Attendance rows that collide inside one meeting stay visible to admins here.
insert into public.academic_operations (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-att-overflow-' || a.legacy_id, '', coalesce((a.row_data->>'custom_user_id')::bigint, 0), '{}'::jsonb,
  'preserved', 'legacy:attendance_overflow:' || a.legacy_id,
  jsonb_build_object('kind', 'attendance_overflow', 'legacy_table', 'attendance_logs',
    'reason', 'duplicate check-in inside the same attendance session/meeting', 'row', a.row_data)
from legacy_att a
where not exists (select 1 from public.academic_attendance_records r where r.id = 'legacy-att-' || a.legacy_id)
on conflict (id) do nothing;

insert into public.academic_operations (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-op-' || o.source_table || '-' || md5(o.row_data::text), '', 0, '{}'::jsonb, 'preserved',
  'legacy:' || o.source_table || ':' || md5(o.row_data::text),
  jsonb_build_object('kind', o.source_table, 'legacy_table', o.source_table, 'row', o.row_data)
from legacy_preserve.rows o
where o.source_table in (
  'public.assistant_availability','public.schedule_assignments','public.group_members','public.group_assistants',
  'public.schedules','public.konfigurasi','public.system_settings','public.laporan_besar','public.laporan_besar_target',
  'public.pengumpulan_laporan_besar','public.pedoman_laporan','public.peserta_susulan','public.divisi','public.divisi_izin',
  'public.izin','public.division_access','public.achievements','public.pencapaian','public.game_settings','public.game_konfigurasi',
  'public.weekly_standings','public.feedback','public.absensi',
  'public.assistant_schedules','public.class_rosters','public.practical_groups','public.peminjaman','public.inventaris',
  'public.inventory_items','public.inventory_rentals','public.equipment','public.external_links','public.financial_records',
  'public.attendance_deletion_history','public.prestige_history','public.xp_bonus','public.active_sessions','public.revoked_tokens',
  'public.token_blocklist','public.push_subscriptions','public.siakad_audit_logs','public.materi','public.modul','public.level',
  'public.game_soal','public.playground_contoh','public.playground_examples','public.student_lessons','public.materi_mahasiswa',
  'public.progress_archive','public.profil_belajar','public.progres_belajar','public.elearning_progress'
)
on conflict (id) do nothing;