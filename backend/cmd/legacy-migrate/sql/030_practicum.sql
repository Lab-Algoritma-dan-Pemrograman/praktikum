-- Practicum domain: sessions -> courses -> questions, course activations, and
-- attempts with their answers. Legacy retakes are alphabetical per
-- (activation_session_id, course_id, student_id), which matches the v2
-- activation/attempt keys exactly, so every legacy attempt keeps its own row.
drop view if exists legacy_session, legacy_course, legacy_question, legacy_activation, legacy_selection, legacy_attempt, legacy_answer;

create temp view legacy_session as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.sesi_praktikum';
create temp view legacy_course as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.course';
create temp view legacy_question as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.soal';
create temp view legacy_activation as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.aktivasi_course';
create temp view legacy_activation_session as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.aktivasi_sesi';
create temp view legacy_selection as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.soal_terpilih';
create temp view legacy_attempt as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.pengerjaan_course';
create temp view legacy_answer as
select row_data, (row_data->>'id')::bigint as legacy_id from legacy_preserve.rows where source_table = 'public.jawaban_mahasiswa';

insert into public.practicum_sessions (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-session-' || s.legacy_id, '', 0,
  jsonb_build_object('domain','praktikum','session_id','legacy-session-' || s.legacy_id),
  'active', 'legacy:sesi_praktikum:' || s.legacy_id,
  jsonb_build_object(
    'id', 'legacy-session-' || s.legacy_id,
    'title', coalesce(nullif(btrim(s.row_data->>'judul_sesi'), ''), 'Sesi ' || s.legacy_id),
    'sequence', coalesce((s.row_data->>'urutan')::int, s.legacy_id),
    'scope', jsonb_build_object('domain','praktikum','session_id','legacy-session-' || s.legacy_id),
    'active', true,
    'version', 1,
    'created_at', coalesce((s.row_data->>'created_at')::timestamptz, now()),
    'updated_at', coalesce((s.row_data->>'updated_at')::timestamptz, now()),
    'description', coalesce(s.row_data->>'deskripsi', ''),
    'is_ujian_praktik', coalesce((s.row_data->>'is_ujian_praktik')::boolean, false),
    'legacy', s.row_data
  )
from legacy_session s
on conflict (id) do update set scope = excluded.scope, data = excluded.data, status = excluded.status;

insert into public.practicum_courses (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-course-' || c.legacy_id, parent.id, 0, parent.scope, 'active',
  'legacy:course:' || c.legacy_id,
  jsonb_build_object(
    'id', 'legacy-course-' || c.legacy_id,
    'session_id', 'legacy-session-' || (c.row_data->>'sesi_praktikum_id'),
    'title', coalesce(nullif(btrim(c.row_data->>'judul'), ''), 'Course ' || c.legacy_id),
    'kind', case when c.row_data->>'jenis' in ('pretest', 'posttest', 'keterampilan', 'ujian_praktik')
                 then c.row_data->>'jenis' else 'keterampilan' end,
    'duration_minutes', least(greatest(coalesce((c.row_data->>'durasi_menit')::int, 60), 1), 180),
    'token_required', coalesce(c.row_data->>'jenis', '') in ('pretest', 'posttest', 'ujian_praktik'),
    'version', 1,
    'legacy', c.row_data
  )
from legacy_course c
join public.practicum_sessions parent on parent.id = 'legacy-session-' || (c.row_data->>'sesi_praktikum_id')
on conflict (id) do update set parent_id = excluded.parent_id, scope = excluded.scope, data = excluded.data, status = excluded.status;

insert into public.practicum_questions (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-question-' || q.legacy_id, parent.id, 0, parent.scope,
  case when coalesce((q.row_data->>'is_arsip')::boolean, false) then 'archived' else 'active' end,
  'legacy:soal:' || q.legacy_id,
  jsonb_build_object(
    'id', 'legacy-question-' || q.legacy_id,
    'course_id', 'legacy-course-' || (q.row_data->>'course_id'),
    'type', case when q.row_data->>'jenis_soal' = 'coding' then 'coding' else 'essay' end,
    'difficulty', coalesce(q.row_data->>'difficulty', 'medium'),
    'category', coalesce(q.row_data->>'kategori_ujian', ''),
    'prompt', coalesce(q.row_data->>'teks_soal', ''),
    'points', least(greatest(coalesce((q.row_data->>'poin')::int, 1), 1), 100),
    'starter_code', '',
    'language', 'c',
    'reference_answer', coalesce(q.row_data->>'kunci_jawaban', ''),
    'rubric', '[]'::jsonb,
    'version', 1,
    'legacy', q.row_data
  )
from legacy_question q
join public.practicum_courses parent on parent.id = 'legacy-course-' || (q.row_data->>'course_id')
on conflict (id) do update set parent_id = excluded.parent_id, scope = excluded.scope, data = excluded.data, status = excluded.status;

insert into public.practicum_activations (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-activation-' || ac.legacy_id,
  'legacy-session-' || (s.row_data->>'sesi_praktikum_id'),
  0,
  jsonb_strip_nulls(jsonb_build_object(
    'domain', 'praktikum',
    'session_id', 'legacy-session-' || (s.row_data->>'sesi_praktikum_id'),
    'class_id', case when s.row_data->>'kelas_id' is not null then 'legacy-class-' || (s.row_data->>'kelas_id') end,
    'shift', case when s.row_data->>'shift' ~ '^[0-9]+$' then (s.row_data->>'shift')::int end)),
  case when coalesce((ac.row_data->>'is_open')::boolean, false) then 'open' else 'closed' end,
  'legacy:aktivasi_course:' || ac.legacy_id,
  jsonb_build_object(
    'activation', jsonb_build_object(
      'id', 'legacy-activation-' || ac.legacy_id,
      'session_id', 'legacy-session-' || (s.row_data->>'sesi_praktikum_id'),
      'course_id', 'legacy-course-' || (ac.row_data->>'course_id'),
      'scope', jsonb_strip_nulls(jsonb_build_object(
        'domain', 'praktikum',
        'session_id', 'legacy-session-' || (s.row_data->>'sesi_praktikum_id'),
        'class_id', case when s.row_data->>'kelas_id' is not null then 'legacy-class-' || (s.row_data->>'kelas_id') end,
        'shift', case when s.row_data->>'shift' ~ '^[0-9]+$' then (s.row_data->>'shift')::int end)),
      'timer_mode', 'shared',
      'is_open', coalesce((ac.row_data->>'is_open')::boolean, false),
      'started_at', coalesce((ac.row_data->>'started_at')::timestamptz, (ac.row_data->>'opened_at')::timestamptz),
      'deadline_at', (ac.row_data->>'closed_at')::timestamptz,
      'closes_at', (ac.row_data->>'closed_at')::timestamptz,
      'question_count', (select count(*) from legacy_selection st
                         where st.row_data->>'aktivasi_sesi_id' = ac.row_data->>'aktivasi_sesi_id'
                           and st.row_data->>'course_id' = ac.row_data->>'course_id'),
      'version', 1,
      'student_ids', coalesce((
        select jsonb_agg(u.id order by u.id)
        from legacy_preserve.rows k
        join public.users u on u.id = (k.row_data->>'user_id')::bigint
        where k.source_table = 'public.keanggotaan'
          and (k.row_data->>'kelas_id')::bigint = (s.row_data->>'kelas_id')::bigint
          and (s.row_data->>'shift' is null or k.row_data->>'shift' = s.row_data->>'shift')), '[]'::jsonb),
      'access_token', coalesce(s.row_data->>'token', ''),
      'legacy_aktivasi_sesi_id', s.legacy_id,
      'legacy_kelas_id', s.row_data->>'kelas_id',
      'legacy_gelombang', s.row_data->>'gelombang',
      'legacy_urutan', ac.row_data->'urutan'
    ),
    'course', (select c.data from public.practicum_courses c where c.id = 'legacy-course-' || (ac.row_data->>'course_id')),
    'questions', coalesce((
      select jsonb_agg(qq.data order by st.row_data->>'urutan')
      from legacy_selection st
      join public.practicum_questions qq on qq.id = 'legacy-question-' || (st.row_data->>'soal_id')
      where st.row_data->>'aktivasi_sesi_id' = ac.row_data->>'aktivasi_sesi_id'
        and st.row_data->>'course_id' = ac.row_data->>'course_id'), '[]'::jsonb),
    'token_hash', case when coalesce(s.row_data->>'token', '') = '' then ''
                       else encode(sha256(convert_to(s.row_data->>'token', 'UTF8')), 'hex') end,
    'legacy', jsonb_build_object('aktivasi_course', ac.row_data, 'aktivasi_sesi', s.row_data)
  )
from legacy_activation ac
join legacy_activation_session s on s.legacy_id = (ac.row_data->>'aktivasi_sesi_id')::bigint
where exists (select 1 from public.practicum_courses c where c.id = 'legacy-course-' || (ac.row_data->>'course_id'))
on conflict (id) do nothing;

insert into public.practicum_attempts (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-attempt-' || p.legacy_id,
  'legacy-activation-' || ac.legacy_id,
  (p.row_data->>'mahasiswa_id')::bigint,
  act.scope,
  case when p.row_data->>'status' = 'selesai' then 'submitted' else 'in_progress' end,
  'legacy:pengerjaan_course:' || p.legacy_id,
  jsonb_build_object(
    'attempt', jsonb_build_object(
      'attempt_id', 'legacy-attempt-' || p.legacy_id,
      'activation_id', 'legacy-activation-' || ac.legacy_id,
      'session_id', act.parent_id,
      'course_id', 'legacy-course-' || (p.row_data->>'course_id'),
      'user_id', (p.row_data->>'mahasiswa_id')::bigint,
      'status', case when p.row_data->>'status' = 'selesai' then 'submitted' else 'in_progress' end,
      'submission_reason', case when p.row_data->>'status' = 'selesai' then 'legacy_submit' else '' end,
      'revision', 1,
      'grade_revision', 1,
      'grading_status', case when p.row_data->>'total_nilai' is not null then 'graded' else 'pending' end,
      'started_at', coalesce((p.row_data->>'waktu_mulai')::timestamptz, now()),
      'deadline_at', coalesce((p.row_data->>'waktu_selesai')::timestamptz, (p.row_data->>'waktu_mulai')::timestamptz, now()),
      'submitted_at', (p.row_data->>'waktu_selesai')::timestamptz,
      'server_now', now(),
      'timezone', 'Asia/Jakarta',
      'timezone_offset', '+0700',
      'questions', coalesce(act.data->'questions', '[]'::jsonb),
      'answers', coalesce(j.answers, '[]'::jsonb),
      'score', (p.row_data->>'total_nilai')::int,
      'max_score', coalesce((select sum((el->>'points')::int)
                             from jsonb_array_elements(coalesce(act.data->'questions', '[]'::jsonb)) as q(el)), 0),
      'percentage', (p.row_data->>'total_nilai')::numeric
    ),
    'scope', act.scope,
    'user_name', coalesce(u.name, ''),
    'submitted_answers', coalesce(j.answers, '[]'::jsonb),
    'manual_grades', coalesce(j.manual_grades, '[]'::jsonb),
    'grade_reason', 'legacy_import',
    'legacy', jsonb_build_object('pengerjaan_course', p.row_data, 'answer_ids', coalesce(j.answer_ids, '[]'::jsonb))
  )
from legacy_attempt p
join legacy_activation ac on ac.row_data->>'aktivasi_sesi_id' = p.row_data->>'aktivasi_sesi_id'
                         and ac.row_data->>'course_id' = p.row_data->>'course_id'
join public.users u on u.id = (p.row_data->>'mahasiswa_id')::bigint
join lateral (
  select
    jsonb_agg(jsonb_build_object(
      'question_id', 'legacy-question-' || (st.row_data->>'soal_id'),
      'value', coalesce(a.row_data->>'jawaban_teks', ''),
      'language', 'c',
      'updated_at', coalesce((a.row_data->>'updated_at')::timestamptz, (a.row_data->>'waktu_submit')::timestamptz, now()),
      'score', (a.row_data->>'nilai')::int,
      'feedback', coalesce(a.row_data->>'feedback', ''),
      'legacy_id', a.legacy_id,
      'legacy_submitted', coalesce((a.row_data->>'is_submitted')::boolean, false),
      'legacy_submitted_at', a.row_data->'waktu_submit'
    ) order by st.row_data->>'urutan', a.legacy_id) as answers,
    jsonb_agg(jsonb_build_object(
      'question_id', 'legacy-question-' || (st.row_data->>'soal_id'),
      'score', (a.row_data->>'nilai')::int,
      'feedback', coalesce(a.row_data->>'feedback', '')
    ) order by st.row_data->>'urutan', a.legacy_id) filter (where a.row_data->>'nilai' is not null) as manual_grades,
    jsonb_agg(a.legacy_id order by a.legacy_id) as answer_ids
  from legacy_answer a
  join legacy_selection st on st.legacy_id = (a.row_data->>'soal_terpilih_id')::bigint
  where (st.row_data->>'aktivasi_sesi_id')::bigint = (p.row_data->>'aktivasi_sesi_id')::bigint
    and (st.row_data->>'course_id')::bigint = (p.row_data->>'course_id')::bigint
    and (a.row_data->>'mahasiswa_id')::bigint = (p.row_data->>'mahasiswa_id')::bigint
) j on true
join lateral (
  select act2.data, act2.scope, act2.parent_id from public.practicum_activations act2
  where act2.id = 'legacy-activation-' || ac.legacy_id
) act on true
on conflict (parent_id, owner_id) do nothing;
