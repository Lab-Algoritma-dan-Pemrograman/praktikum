import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, statSync } from 'node:fs';
import { homedir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { productionEnv, databaseEnvironment, psqlJson, productionProject } from './praktikum-prod-env.mjs';

const tables = ['users','roles','permissions','divisions','role_permissions','division_permissions','user_role_assignments','user_permission_grants','practicum_sessions','practicum_courses','practicum_questions','practicum_activations','practicum_attempts','practicum_grading_jobs','practicum_operations','practicum_runner_leases'];
function command(tool, args, env, timeout = 600000) {
  const result = spawnSync(tool,args,{env,encoding:'utf8',timeout,maxBuffer:8*1024*1024});
  if(result.error||result.status!==0) throw new Error(`${tool} failed; credentials and raw diagnostics withheld. No production mutation was requested.`);
  return result.stdout;
}
function checksum(path){return createHash('sha256').update(readFileSync(path)).digest('hex');}
try {
  const env=productionEnv();
  const root=mkdtempSync(join(homedir(),'.claude','praktikum-prod-'));
  for(const sub of ['backup','clone-data','evidence','local-pg']) mkdirSync(join(root,sub));
  const pgEnv=databaseEnvironment(env.ALGOHUB_MIGRATION_DATABASE_URL||env.ALGOHUB_DATABASE_URL);
  pgEnv.PGOPTIONS='-c statement_timeout=0 -c lock_timeout=3000 -c default_transaction_read_only=on';
  const before=psqlJson(env,`select jsonb_build_object('database',current_database(),'captured_at',clock_timestamp(),'migrations',(select jsonb_agg(jsonb_build_object('name',name,'checksum',checksum) order by name) from public.schema_migrations),'users',(select count(*) from public.users),'attempts',(select count(*) from public.practicum_attempts),'answers',(select sum(jsonb_array_length(data->'attempt'->'answers')) from public.practicum_attempts),'legacy_rows',(select count(*) from legacy_preserve.rows))`);
  writeFileSync(join(root,'evidence','before.json'),JSON.stringify(before,null,2));
  const full=join(root,'backup','production-logical.dump');
  command('pg_dump',['-Fc','--no-owner','--no-privileges','--lock-wait-timeout=3000','--schema=public','--schema=auth','--schema=legacy_preserve','--schema=storage','--file',full],pgEnv);
  const data=join(root,'clone-data','practicum-identity-data.dump');
  command('pg_dump',['-Fc','--data-only','--no-owner','--no-privileges','--lock-wait-timeout=3000',...tables.map(name=>`--table=public.${name}`),'--file',data],pgEnv);
  const authIDs=psqlJson(env,`select coalesce(jsonb_agg(auth_user_id::text order by id),'[]'::jsonb) from public.users`);
  writeFileSync(join(root,'clone-data','auth-ids.json'),JSON.stringify(authIDs),{mode:0o600});
  const toc=command('pg_restore',['--list',full],process.env,120000);
  if(!toc.includes('practicum_attempts')||!toc.includes('legacy_preserve')||statSync(full).size===0) throw new Error('Backup manifest incomplete.');
  const manifest={project:productionProject,createdAt:new Date().toISOString(),root,schemas:['public','auth','legacy_preserve','storage'],storageBytesChanged:false,full:{path:full,bytes:statSync(full).size,sha256:checksum(full)},rehearsal:{path:data,bytes:statSync(data).size,sha256:checksum(data),tables},authIdCount:authIDs.length,readOnly:true};
  writeFileSync(join(root,'evidence','backup-manifest.json'),JSON.stringify(manifest,null,2));
  console.log(JSON.stringify({root,project:manifest.project,readOnly:true,backupBytes:manifest.full.bytes,backupSha256:manifest.full.sha256,rehearsalBytes:manifest.rehearsal.bytes,authIdCount:manifest.authIdCount},null,2));
}catch(error){console.error(error.message);process.exitCode=1;}
