-- Domain reconciliation. Every check compares the projected v2 rows against the
-- preserved legacy rows, not against each other, and answers are reconciled per
-- legacy answer id (value/score/feedback/timestamp) instead of by question count.
do $$
declare
  failures text[] := '{}';
  got bigint;
  expected bigint;
begin
  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.users');
  got := (select count(*) from public.users u
          join legacy_preserve.rows r on r.source_table = 'public.users' and (r.row_data->>'id')::bigint = u.id);
  if got <> expected then failures := failures || format('users %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.users' and row_data->>'password_hash' is not null);
  got := (select count(*) from public.users u join auth.users a on a.id = u.auth_user_id
           join legacy_preserve.rows r on r.source_table = 'public.users' and (r.row_data->>'id')::bigint = u.id
           where a.encrypted_password = r.row_data->>'password_hash');
  if got <> expected then failures := failures || format('credential hashes preserved %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.users' and lower(nullif(btrim(row_data->>'role'), '')) = 'asisten');
  got := (select count(*) from public.user_role_assignments where role_code = 'asisten');
  if got <> expected then failures := failures || format('asisten assignments %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.levels');
  got := (select count(*) from public.learning_levels);
  if got <> expected then failures := failures || format('learning_levels %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.modules');
  got := (select count(*) from public.learning_modules);
  if got <> expected then failures := failures || format('learning_modules %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.lessons');
  got := (select count(*) from public.learning_lessons);
  if got <> expected then failures := failures || format('learning_lessons %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.student_progress');
  got := (select count(*) from public.learning_progress where id like 'legacy-progress-%');
  if got <> expected then failures := failures || format('learning_progress %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.student_progress' and coalesce((row_data->>'completed')::boolean, false));
  got := (select count(*) from public.learning_progress where id like 'legacy-progress-%' and status = 'completed');
  if got <> expected then failures := failures || format('completed progress %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.users');
  got := (select count(*) from public.learning_profiles where id like 'legacy-profile-%');
  if got <> expected then failures := failures || format('learning_profiles %s <> %s', got, expected); end if;

  expected := (select sum(coalesce((row_data->>'xp')::int, 0)) from legacy_preserve.rows where source_table = 'public.users');
  got := (select sum((data->>'xp')::int) from public.learning_profiles where id like 'legacy-profile-%');
  if got <> expected then failures := failures || format('profile xp total %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.unlocked_achievements');
  got := (select coalesce(sum(jsonb_array_length(data->'unlocked_achievements')), 0)
          from public.learning_profiles where id like 'legacy-profile-%');
  if got <> expected then failures := failures || format('unlocked achievements %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.game_history');
  got := (select count(*) from public.learning_game_history where id like 'legacy-game-%');
  if got <> expected then failures := failures || format('learning_game_history %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.game_questions');
  got := (select count(*) from public.learning_bug_hunt where id like 'legacy-bughunt-%');
  if got <> expected then failures := failures || format('bug hunt questions %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.sesi_praktikum');
  got := (select count(*) from public.practicum_sessions where id like 'legacy-session-%');
  if got <> expected then failures := failures || format('practicum_sessions %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.course');
  got := (select count(*) from public.practicum_courses where id like 'legacy-course-%');
  if got <> expected then failures := failures || format('practicum_courses %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.soal');
  got := (select count(*) from public.practicum_questions where id like 'legacy-question-%');
  if got <> expected then failures := failures || format('practicum_questions %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.aktivasi_course');
  got := (select count(*) from public.practicum_activations where id like 'legacy-activation-%');
  if got <> expected then failures := failures || format('practicum_activations %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.pengerjaan_course');
  got := (select count(*) from public.practicum_attempts where id like 'legacy-attempt-%');
  if got <> expected then failures := failures || format('practicum_attempts %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.jawaban_mahasiswa');
  got := (select coalesce(sum(jsonb_array_length(data->'attempt'->'answers')), 0)
          from public.practicum_attempts where id like 'legacy-attempt-%');
  if got <> expected then failures := failures || format('answers total %s <> %s', got, expected); end if;

  -- Per-answer reconciliation: each legacy answer must exist with identical
  -- value, score and feedback inside the attempt that owns it.
  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.jawaban_mahasiswa');
  got := (select count(*)
    from legacy_preserve.rows a
    join legacy_preserve.rows st on st.source_table = 'public.soal_terpilih'
      and (st.row_data->>'id')::bigint = (a.row_data->>'soal_terpilih_id')::bigint
    join public.practicum_activations act on act.id = 'legacy-activation-'
      || (select ac.row_data->>'id' from legacy_preserve.rows ac
          where ac.source_table = 'public.aktivasi_course'
            and ac.row_data->>'aktivasi_sesi_id' = st.row_data->>'aktivasi_sesi_id'
            and ac.row_data->>'course_id' = st.row_data->>'course_id')
    join public.practicum_attempts at on at.parent_id = act.id
      and at.owner_id = (a.row_data->>'mahasiswa_id')::bigint
    cross join lateral jsonb_array_elements(at.data->'attempt'->'answers') ans
    where a.source_table = 'public.jawaban_mahasiswa'
      and (ans->>'legacy_id')::bigint = (a.row_data->>'id')::bigint
      and coalesce(ans->>'value', '') = coalesce(a.row_data->>'jawaban_teks', '')
      and coalesce(ans->>'feedback', '') = coalesce(a.row_data->>'feedback', '')
      and (ans->>'score')::int is not distinct from (a.row_data->>'nilai')::int);
  if got <> expected then failures := failures || format('answer reconciliation %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.kelas');
  got := (select count(*) from public.academic_classes where id like 'legacy-class-%' and id <> 'legacy-class-container');
  if got <> expected then failures := failures || format('academic_classes %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.jadwal');
  got := (select count(*) from public.academic_schedules where id like 'legacy-schedule-%' and id <> 'legacy-schedule-container');
  if got <> expected then failures := failures || format('academic_schedules %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.attendance_logs');
  got := (select count(*) from public.academic_attendance_records where id like 'legacy-att-%')
       + (select count(*) from public.academic_operations where data->>'kind' = 'attendance_overflow');
  if got <> expected then failures := failures || format('attendance rows accounted %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows where source_table = 'public.qr_sessions');
  got := (select count(distinct data->'session'->>'legacy_qr_session_id') from public.academic_attendance_sessions);
  if got <> expected then failures := failures || format('qr sessions represented %s <> %s', got, expected); end if;

  expected := (select count(*) from legacy_preserve.rows
               where source_table in ('public.audit_logs', 'public.activity_logs', 'public.siakad_audit_logs'));
  got := (select count(*) from public.audit_logs where operation_id like 'legacy:%');
  if got <> expected then failures := failures || format('audit_logs %s <> %s', got, expected); end if;

  expected := (select count(*) - count(distinct source_pk) from legacy_preserve.rows where source_table = 'public.attendance_logs');
  got := 0;
  if expected <> 0 then failures := failures || format('duplicate attendance ids %s', expected); end if;

  if array_length(failures, 1) is not null then
    raise exception 'domain reconciliation failed: %', array_to_string(failures, '; ');
  end if;
  raise notice 'domain reconciliation passed';
end $$;
