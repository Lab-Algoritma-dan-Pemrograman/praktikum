import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { productionEnv, psqlJson } from './praktikum-prod-env.mjs';

const root = process.argv[2] ? join(process.argv[2], 'evidence') : '.';

const sql = `select jsonb_build_object(
 'score_key_present_null',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score' and e->'score' = 'null'::jsonb),
 'score_key_nonnull',(select count(*) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score' and e->'score' <> 'null'::jsonb),
 'score_nonnull_min',(select min((e->>'score')::numeric) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score' and e->'score' <> 'null'::jsonb),
 'score_nonnull_max',(select max((e->>'score')::numeric) from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score' and e->'score' <> 'null'::jsonb),
 'score_nonnull_distinct',(select count(distinct e->>'score') from public.practicum_attempts a, lateral jsonb_array_elements(a.data->'attempt'->'answers') e where e ? 'score' and e->'score' <> 'null'::jsonb),
 'legacy_jawaban_nilai_keys',(select count(*) from legacy_preserve.rows where source_table='public.jawaban_mahasiswa' and row_data ? 'nilai'),
 'legacy_jawaban_nilai_null',(select count(*) from legacy_preserve.rows where source_table='public.jawaban_mahasiswa' and (row_data ? 'nilai') and (row_data->'nilai' = 'null'::jsonb)),
 'legacy_pengerjaan_total_nilai_keys',(select count(*) from legacy_preserve.rows where source_table='public.pengerjaan_course' and row_data ? 'total_nilai'),
 'legacy_pengerjaan_total_nilai_null',(select count(*) from legacy_preserve.rows where source_table='public.pengerjaan_course' and (row_data ? 'total_nilai') and (row_data->'total_nilai' = 'null'::jsonb)),
 'attempts_status_mix',(select jsonb_object_agg(status,c) from (select status,count(*) c from public.practicum_attempts group by status) s),
 'grading_status_mix',(select jsonb_object_agg(gs,c) from (select data->'attempt'->>'grading_status' gs,count(*) c from public.practicum_attempts group by 1) s),
 'submitted_reason_mix',(select jsonb_object_agg(r,c) from (select coalesce(data->'attempt'->>'submission_reason','') r,count(*) c from public.practicum_attempts group by 1) s),
 'active_with_future_deadline',(select count(*) from public.practicum_attempts where status='in_progress' and (data->'attempt'->>'deadline_at')::timestamptz > clock_timestamp()),
 'active_past_deadline',(select count(*) from public.practicum_attempts where status='in_progress' and (data->'attempt'->>'deadline_at')::timestamptz <= clock_timestamp()),
 'attempts_missing_answers',(select count(*) from public.practicum_attempts where coalesce(jsonb_array_length(data->'attempt'->'answers'),0)=0),
 'attempts_missing_questions',(select count(*) from public.practicum_attempts where coalesce(jsonb_array_length(data->'attempt'->'questions'),0)=0),
 'legacy_pengerjaan_rows',(select count(*) from legacy_preserve.rows where source_table='public.pengerjaan_course'),
 'legacy_soal_rows',(select count(*) from legacy_preserve.rows where source_table='public.soal'))`;

try {
  const result = psqlJson(productionEnv(), sql, { readOnly: true, label: 'Production grade completeness' });
  if (root !== '.') writeFileSync(join(root, 'production-grades.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify(result, null, 2));
} catch (error) { console.error(error.message); process.exitCode = 1; }
