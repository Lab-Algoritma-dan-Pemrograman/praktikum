import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join, resolve } from 'node:path';
import { randomBytes, createHash } from 'node:crypto';
import { databaseEnvironment, productionEnv, psqlJson } from './praktikum-prod-env.mjs';
import { buildRepairSQL, workFingerprintSQL } from './praktikum-prod-repair-sql.mjs';

function command(tool,args,env,timeout=120000){
 const r=spawnSync(tool,args,{env,encoding:'utf8',timeout,maxBuffer:8*1024*1024,...(tool==='pg_ctl'?{stdio:'ignore'}:{})});
 if(r.error||r.status!==0)throw new Error(`${tool} rehearsal failed; raw diagnostics withheld to protect restored data.`);
 return r.stdout;
}
function sql(env,text){return psqlJson(env,`${text}; select jsonb_build_object('ok',true)`,{readOnly:false,label:'Local clone SQL'});}
const root=resolve(process.argv[2]);
const before=JSON.parse(readFileSync(join(root,'evidence','before.json'),'utf8'));
try{
 const pwfile=join(root,'local-pg','password.txt');
 const resume=process.argv.includes('--resume');
 const password=resume?readFileSync(pwfile,'utf8').trim():randomBytes(24).toString('hex');
 const dataDir=join(root,'local-pg','data');
 if(!resume){
  writeFileSync(pwfile,password,{mode:0o600});
  command('initdb',['-D',dataDir,'-U','postgres','--auth=scram-sha-256','--pwfile',pwfile,'--no-locale'],process.env);
  command('pg_ctl',['-D',dataDir,'-l',join(root,'local-pg','server.log'),'-o','-h 127.0.0.1 -p 55449','start'],process.env);
 }
 const raw=`postgres://postgres:${password}@127.0.0.1:55449/praktikum_restore_test?sslmode=disable`;
 writeFileSync(join(root,'local-pg','connection.json'),JSON.stringify({url:raw,unitUrl:raw.replace('praktikum_restore_test','praktikum_unit_test'),dataDir}),{mode:0o600});
 const local={ALGOHUB_MIGRATION_DATABASE_URL:raw};
 const localEnv=databaseEnvironment(raw,false);
 command('createdb',['praktikum_restore_test'],{...localEnv,PGDATABASE:'postgres'});
 command('createdb',['praktikum_unit_test'],{...localEnv,PGDATABASE:'postgres'});
 sql(local,`create role anon nologin;create role authenticated nologin;create schema auth;create table auth.users(id uuid primary key);create function auth.uid() returns uuid language sql stable as $$select nullif(current_setting('request.jwt.claim.sub',true),'')::uuid$$;grant usage on schema auth,public to anon,authenticated;grant execute on function auth.uid() to anon,authenticated;create table public.schema_migrations(name text primary key,checksum text not null,applied_at timestamptz not null default now());`);
 for(const item of before.migrations){
  const body=readFileSync(join('backend/migrations',item.name),'utf8');
  if(createHash('sha256').update(body).digest('hex')!==item.checksum)throw new Error('Rehearsal history checksum drift.');
  sql(local,`${body};insert into public.schema_migrations(name,checksum) values('${item.name}','${item.checksum}')`);
 }
 const ids=JSON.parse(readFileSync(join(root,'clone-data','auth-ids.json'),'utf8'));
 if(ids.some(id=>!/^[-0-9a-f]{36}$/i.test(id)))throw new Error('Invalid restored auth identifier.');
 for(let n=0;n<ids.length;n+=100)sql(local,`insert into auth.users(id) values ${ids.slice(n,n+100).map(id=>`('${id}'::uuid)`).join(',')}`);
 sql(local,`truncate public.user_permission_grants,public.user_role_assignments,public.division_permissions,public.role_permissions,public.permissions,public.divisions,public.roles cascade`);
 command('pg_restore',['--data-only','--disable-triggers','--no-owner','--no-privileges','--exit-on-error','--dbname','praktikum_restore_test',join(root,'clone-data','practicum-identity-data.dump')],localEnv,180000);
 sql(local,`select setval('public.users_id_seq',greatest((select coalesce(max(id),1) from public.users),1),true)`);
 const fingerprint=`set local time zone 'UTC';${workFingerprintSQL()}`;
 const restored=psqlJson(local,fingerprint,{readOnly:true});
 const live=psqlJson(productionEnv(),fingerprint,{readOnly:true});
 if(JSON.stringify(restored)!==JSON.stringify(live))throw new Error('Restored work fingerprint differs from production; stopped.');
 writeFileSync(join(root,'evidence','restore-fingerprints.json'),JSON.stringify(restored,null,2));
 const repair=buildRepairSQL(before.migrations);
 const repaired=psqlJson(local,repair,{readOnly:false,label:'Local complete metadata repair'});
 const checks=psqlJson(local,`select jsonb_build_object('attempt_session_mismatch',(select count(*) from public.practicum_attempts t join public.practicum_activations a on a.id=t.parent_id where t.data#>>'{attempt,session_id}' is distinct from a.parent_id),'attempt_scope_mismatch',(select count(*) from public.practicum_attempts t join public.practicum_activations a on a.id=t.parent_id where t.scope is distinct from a.scope or t.data->'scope' is distinct from a.scope),'missing_domain',(select count(*) from public.practicum_activations where scope->>'domain' is distinct from 'praktikum'),'missing_session_scope',(select count(*) from public.practicum_activations where scope->>'session_id' is distinct from parent_id),'attempts',(select count(*) from public.practicum_attempts),'answers',(select sum(jsonb_array_length(data->'attempt'->'answers')) from public.practicum_attempts))`);
 if(checks.attempt_session_mismatch||checks.attempt_scope_mismatch||checks.missing_domain||checks.missing_session_scope||checks.attempts!==before.attempts||checks.answers!==before.answers)throw new Error('Repaired clone failed parity checks.');
 const history=psqlJson(local,`select jsonb_agg(jsonb_build_object('name',name,'checksum',checksum) order by name) from public.schema_migrations`);
 const second=psqlJson(local,buildRepairSQL(history),{readOnly:false,label:'Local idempotency replay'});
 if(JSON.stringify(repaired)!==JSON.stringify(second))throw new Error('Repair replay changed preserved work.');
 const result={status:'passed',checks,originalFingerprint:restored,repairedFingerprint:repaired,idempotent:true,productionWritten:false};
 writeFileSync(join(root,'evidence','rehearsal.json'),JSON.stringify(result,null,2));
 writeFileSync(join(root,'local-pg','connection.json'),JSON.stringify({url:raw,unitUrl:raw.replace('praktikum_restore_test','praktikum_unit_test'),dataDir}),{mode:0o600});
 console.log(JSON.stringify({status:'passed',root,checks,idempotent:true,productionWritten:false},null,2));
}catch(error){console.error(error.message);process.exitCode=1;}
