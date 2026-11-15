import { mkdirSync, writeFileSync, readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { productionEnv, productionProject, psqlJson } from './praktikum-prod-env.mjs';

const sql = `select jsonb_build_object(
 'project','${productionProject}', 'database',current_database(), 'read_only',current_setting('transaction_read_only'),
 'migrations',(select jsonb_agg(jsonb_build_object('name',name,'checksum',checksum) order by name) from public.schema_migrations),
 'counts',jsonb_build_object(
 'users',(select count(*) from public.users),
 'sessions',(select count(*) from public.practicum_sessions),
 'courses',(select count(*) from public.practicum_courses),
 'questions',(select count(*) from public.practicum_questions),
 'activations',(select count(*) from public.practicum_activations),
 'attempts',(select count(*) from public.practicum_attempts),
 'answers',(select coalesce(sum(jsonb_array_length(data->'attempt'->'answers')),0) from public.practicum_attempts),
 'grading_jobs',(select count(*) from public.practicum_grading_jobs),
 'legacy_rows',(select count(*) from legacy_preserve.rows),
 'storage_objects',(select count(*) from storage.objects)),
 'checks',jsonb_build_object(
 'active_future_attempts',(select count(*) from public.practicum_attempts where status='in_progress' and coalesce(expires_at,(data->'attempt'->>'deadline_at')::timestamptz)>clock_timestamp()),
 'attempt_session_mismatch',(select count(*) from public.practicum_attempts t join public.practicum_activations a on a.id=t.parent_id where t.data->'attempt'->>'session_id' is distinct from a.parent_id),
 'attempt_scope_mismatch',(select count(*) from public.practicum_attempts t join public.practicum_activations a on a.id=t.parent_id where t.scope is distinct from a.scope or t.data->'scope' is distinct from a.scope),
 'attempt_owner_mismatch',(select count(*) from public.practicum_attempts where data->'attempt'->>'user_id' is distinct from owner_id::text),
 'unknown_legacy_attempt_marker',(select count(*) from public.practicum_attempts where id like 'legacy-attempt-%' and not coalesce(data->'legacy' ? 'pengerjaan_course',false)),
 'activation_scope_copies_mismatch',(select count(*) from public.practicum_activations where scope is distinct from data->'activation'->'scope'),
 'activation_parent_mismatch',(select count(*) from public.practicum_activations where data->'activation'->>'session_id' is distinct from parent_id or data->'course'->>'session_id' is distinct from parent_id),
 'activation_missing_domain',(select count(*) from public.practicum_activations where coalesce(scope->>'domain','')!='praktikum'),
 'activation_missing_session_scope',(select count(*) from public.practicum_activations where scope->>'session_id' is distinct from parent_id),
 'session_missing_domain',(select count(*) from public.practicum_sessions where coalesce(scope->>'domain','')!='praktikum'),
 'graded_attempts',(select count(*) from public.practicum_attempts where status='graded')),
 'freeze_triggers',(select jsonb_agg(jsonb_build_object('name',tgname,'enabled',tgenabled::text)) from pg_trigger where tgname in ('practicum_attempt_frozen','practicum_activation_frozen','practicum_attempt_metadata'))
)`;

try {
  const env = productionEnv();
  const result = psqlJson(env, sql);
  const local = readdirSync(resolve('backend/migrations')).filter((name) => name.endsWith('.sql')).sort().map((name) => ({ name, checksum: createHash('sha256').update(readFileSync(join('backend/migrations',name))).digest('hex') }));
  const expected = new Map(local.map((item) => [item.name,item.checksum]));
  result.historyMatches = (result.migrations || []).every((item) => expected.get(item.name) === item.checksum);
  result.pendingMigrations = local.filter((item) => !(result.migrations || []).some((applied) => applied.name === item.name)).map((item) => item.name);
  if (process.argv[2]) {
    const out = resolve(process.argv[2]);
    mkdirSync(out,{recursive:true});
    writeFileSync(join(out,'preflight.json'),JSON.stringify(result,null,2));
  }
  console.log(JSON.stringify(result,null,2));
  if (!result.historyMatches) process.exitCode = 2;
} catch (error) { console.error(error.message); process.exitCode = 1; }
