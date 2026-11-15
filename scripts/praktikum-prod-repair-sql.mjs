import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

const tables = ['practicum_sessions','practicum_courses','practicum_questions','practicum_activations','practicum_attempts'];
export function workFingerprintSQL() {
  const parts = tables.map((table) => {
    const data = table === 'practicum_sessions' ? `(data-'scope')` : table === 'practicum_activations' ? `(data#-'{activation,scope}')` : table === 'practicum_attempts' ? `((data-'scope')#-'{attempt,session_id}')` : 'data';
    return `select '${table}' as name,count(*) as rows,encode(sha256(convert_to(coalesce(string_agg(jsonb_build_array(id,parent_id,owner_id,status,unique_key,expires_at,${data})::text,E'\\n' order by id),''),'UTF8')),'hex') as sha256 from public.${table}`;
  });
  return `select jsonb_object_agg(name,jsonb_build_object('rows',rows,'sha256',sha256)) from (${parts.join(' union all ')}) fingerprints`;
}
function literal(value) { return `'${String(value).replaceAll("'","''")}'`; }
export function buildRepairSQL(history) {
  const requested = ['0011_practicum_attempt_metadata_repair.sql','0012_practicum_legacy_scope_context.sql'];
  const loaded = requested.map((name) => {
    const body=readFileSync(`backend/migrations/${name}`,'utf8');
    return {name,body,checksum:createHash('sha256').update(body).digest('hex')};
  });
  for (const item of history) {
    if(!/^\d{4}_[a-z0-9_]+\.sql$/.test(item.name)) throw new Error('Unexpected migration history name.');
    const expected=createHash('sha256').update(readFileSync(`backend/migrations/${item.name}`)).digest('hex');
    if(expected!==item.checksum) throw new Error('Migration checksum drift; stopped.');
  }
  const pending=loaded.filter((item)=>!history.some((old)=>old.name===item.name));
  const historyGuard=history.map((item)=>`not exists(select 1 from public.schema_migrations where name=${literal(item.name)} and checksum=${literal(item.checksum)})`).join(' or ') || 'false';
  const fingerprint=workFingerprintSQL();
  return `set local time zone 'UTC';
 set local lock_timeout='3s'; set local statement_timeout='30s';
 select pg_advisory_xact_lock(18379742642722114);
 select pg_advisory_xact_lock(731903005);
 do $history$ begin
 if (select count(*) from public.schema_migrations)<>${history.length} or ${historyGuard} then raise exception 'migration history changed since preflight'; end if;
 if exists(select 1 from public.practicum_attempts where status='in_progress' and coalesce(expires_at,(data->'attempt'->>'deadline_at')::timestamptz)>clock_timestamp()) then raise exception 'active timed attempts; production repair held'; end if;
 end $history$;
 lock table public.practicum_sessions,public.practicum_courses,public.practicum_questions,public.practicum_activations,public.practicum_attempts in access exclusive mode;
 create temporary table repair_work_before on commit drop as ${fingerprint};
 ${pending.map(item=>`${item.body}\ninsert into public.schema_migrations(name,checksum) values(${literal(item.name)},${literal(item.checksum)});`).join('\n')}
 do $preservation$ declare before_hash jsonb;after_hash jsonb; begin
 select jsonb_object_agg into before_hash from repair_work_before;
 ${fingerprint.replace(/^select /,'select ').replace(' from (',' into after_hash from (')};
 if before_hash is distinct from after_hash then raise exception 'non-metadata work changed; rollback'; end if;
 end $preservation$;
 ${fingerprint}`;
}
