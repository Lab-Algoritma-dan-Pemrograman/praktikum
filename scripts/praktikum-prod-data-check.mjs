import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { productionEnv, psqlJson } from './praktikum-prod-env.mjs';

const root = process.argv[2] ? join(process.argv[2], 'evidence') : '.';

const sql = `select jsonb_build_object(
 'answers_total',(select coalesce(sum(jsonb_array_length(data->'attempt'->'answers')),0) from public.practicum_attempts),
 'answers_with_value',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where coalesce(e->>'value','') <> ''),
 'answers_with_code',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where coalesce(e->>'language','') <> ''),
 'answers_with_score',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score'),
 'answers_with_feedback',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where coalesce(e->>'feedback','') <> ''),
 'attempts_with_score',(select count(*) from public.practicum_attempts where data->'attempt'->>'score' is not null and data->'attempt'->>'score' <> 'null'),
 'attempts_graded',(select count(*) from public.practicum_attempts where status='graded'),
 'attempts_submitted',(select count(*) from public.practicum_attempts where status='submitted'),
 'attempts_in_progress',(select count(*) from public.practicum_attempts where status='in_progress'),
 'questions_total',(select count(*) from public.practicum_questions),
 'questions_with_prompt',(select count(*) from public.practicum_questions where coalesce(data->>'prompt','') <> ''),
 'questions_essay',(select count(*) from public.practicum_questions where data->>'type'='essay'),
 'questions_coding',(select count(*) from public.practicum_questions where data->>'type'='coding'),
 'activations_with_class',(select count(*) from public.practicum_activations where scope ? 'class_id'),
 'activations_with_shift',(select count(*) from public.practicum_activations where scope ? 'shift'),
 'distinct_classes',(select count(distinct scope->>'class_id') from public.practicum_activations),
 'distinct_shifts',(select coalesce(jsonb_agg(distinct scope->>'shift'),'[]'::jsonb) from public.practicum_activations where scope ? 'shift'),
 'sessions_total',(select count(*) from public.practicum_sessions),
 'courses_total',(select count(*) from public.practicum_courses),
 'activations_total',(select count(*) from public.practicum_activations),
 'attempts_total',(select count(*) from public.practicum_attempts),
 'grading_jobs',(select count(*) from public.practicum_grading_jobs),
 'manual_grades',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(coalesce(a.data->'manual_grades','[]'::jsonb)) e),
 'legacy_rows',(select count(*) from legacy_preserve.rows),
 'legacy_answer_rows',(select count(*) from legacy_preserve.rows where source_table='public.jawaban_mahasiswa'),
 'legacy_total_nilai_nonnull',(select count(*) from legacy_preserve.rows where source_table='public.pengerjaan_course' and coalesce(row_data->>'total_nilai','') not in ('','null')),
 'legacy_answer_nilai_nonnull',(select count(*) from legacy_preserve.rows where source_table='public.jawaban_mahasiswa' and coalesce(row_data->>'nilai','') not in ('','null')),
 'storage_objects',(select count(*) from storage.objects),
 'students_with_attempts',(select count(distinct owner_id) from public.practicum_attempts))`;

try {
  const result = psqlJson(productionEnv(), sql, { readOnly: true, label: 'Production data completeness' });
  if (root !== '.') writeFileSync(join(root, 'production-data.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
} catch (error) { console.error(error.message); process.exitCode = 1; }
