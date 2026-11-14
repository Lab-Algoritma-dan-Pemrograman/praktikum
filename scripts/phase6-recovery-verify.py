#!/usr/bin/env python3
"""Finish recovery verification of a committed Phase6 masked batch.
No import/reimport or deletion on staging. Read-only remote snapshot/dump; restore
only to a new owned loopback cluster and mutate only its acceptance copy.
"""
import argparse,hashlib,json,os,re,secrets,shutil,socket,subprocess,tempfile,time,uuid
from pathlib import Path
from urllib.parse import urlsplit,unquote
ROOT=Path(__file__).resolve().parents[1]
def read_env(path):
 values={}
 for line in Path(path).read_text(encoding='utf-8-sig').splitlines():
  m=re.match(r'^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$',line)
  if m:
   k,v=m.groups();v=v.strip()
   if len(v)>=2 and v[0]==v[-1] and v[0] in "'\"":v=v[1:-1]
   if k in {'ALGOHUB_MIGRATION_DATABASE_URL','ALGOHUB_DATABASE_URL','SUPABASE_URL'}:values[k]=v
 return values

def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--plan',required=True);p.add_argument('--original-run',required=True);p.add_argument('--env-file',required=True);p.add_argument('--expected-project',required=True);p.add_argument('--auth-metadata',required=True);p.add_argument('--evidence-parent',default=tempfile.gettempdir())
 a=p.parse_args();plan=Path(a.plan).resolve();source=Path(a.original_run).resolve();body=plan.read_bytes();plan_sha=hashlib.sha256(body).hexdigest();pdata=json.loads(body);receipt=json.loads((source/'apply-first.json').read_text());metadata=Path(a.auth_metadata).resolve()
 if receipt['plan_file_sha256']!=plan_sha or receipt['batch_id']!=pdata['batch_id'] or receipt['target_fingerprint']!=a.expected_project:raise SystemExit('original receipt/plan/target mismatch')
 cfg=read_env(a.env_file);uri=cfg.get('ALGOHUB_MIGRATION_DATABASE_URL') or cfg.get('ALGOHUB_DATABASE_URL');u=urlsplit(uri);project=urlsplit(cfg.get('SUPABASE_URL','')).hostname
 if not project or hashlib.sha256(project.encode()).hexdigest()!=a.expected_project or not u.hostname:raise SystemExit('staging configuration identity mismatch')
 out=Path(tempfile.mkdtemp(prefix='algohub-phase6-recovery-',dir=a.evidence_parent));data=out/'pgdata';run='p6recovery-'+uuid.uuid4().hex[:10]
 summary={'run_id':run,'status':'running','plan_sha256':plan_sha,'original_receipt_sha256':hashlib.sha256((source/'apply-first.json').read_bytes()).hexdigest(),'checks':[],'commands':[],'cleanup':{}}
 tools={n:shutil.which(n) for n in ('go','initdb','pg_ctl','psql','createdb','pg_dump','pg_restore')};env={k:v for k,v in os.environ.items() if not k.startswith(('PG','ALGOHUB_','SUPABASE_','PHASE6_'))};env['PGPASSFILE']=os.devnull;env['PGCONNECT_TIMEOUT']='5';index=0
 print(json.dumps({'status':'started','evidence_dir':str(out)}),flush=True)
 def write(name,value):
  path=out/name
  with path.open('x',encoding='utf8') as f:json.dump(value,f,indent=2);f.write('\n')
  return path
 def command(label,args,timeout=120,xenv=None):
  nonlocal index;index+=1;log=out/f'{index:03d}-{label}.log';start=time.monotonic()
  with log.open('xb') as f:
   try:r=subprocess.run([str(x) for x in args],cwd=ROOT,env=xenv or env,stdin=subprocess.DEVNULL,stdout=f,stderr=subprocess.STDOUT,timeout=timeout,check=False)
   except subprocess.TimeoutExpired:summary['commands'].append({'label':label,'timeout':timeout,'log':log.name});raise RuntimeError(label+' timed out')
  summary['commands'].append({'label':label,'exit':r.returncode,'log':log.name,'seconds':round(time.monotonic()-start,3)})
  if r.returncode:raise RuntimeError(label+' failed: '+log.name)
  return log
 def check(name,ok,**details):
  summary['checks'].append({'name':name,'passed':bool(ok),**details})
  if not ok:raise RuntimeError('check failed: '+name)
 def cli(label,action,target=None,localenv=None,extra=(),xenv=None):
  path=out/(label+'.json');args=[binary,action,'--plan',plan,'--expected-plan-sha256',plan_sha,'--out',path]
  if target:args+=['--local-target',target,'--env-file',localenv]
  else:args+=['--env-file',a.env_file,'--expected-project',a.expected_project]
  command(label,args+list(extra),180,xenv);return json.loads(path.read_text())
 def sql(label,db,q):return command(label,[tools['psql'],'-X','-A','-t','-v','ON_ERROR_STOP=1','-h','127.0.0.1','-p',port,'-U','phase6','-d',db,'-c',q],120)
 def create(label):
  db='p6_'+run.split('-')[1]+'_'+label+'_test';command('create-'+label,[tools['createdb'],'-h','127.0.0.1','-p',port,'-U','phase6',db]);sql('ownership-'+label,db,"comment on database "+db+" is 'algohub-phase6:"+run+"'")
  target=write('target-'+label+'.json',{'run_id':run,'database':db,'data_directory':data.as_posix(),'system_identifier':system,'port':int(port)})
  lf=out/('local-'+label+'.env');lf.write_text("ALGOHUB_MIGRATION_DATABASE_URL='postgres://phase6@127.0.0.1:"+str(port)+"/"+db+"?sslmode=disable'\nSUPABASE_URL='https://local-recovery.invalid'\n");os.chmod(lf,0o600)
  command('auth-prerequisites-'+label,[rehearsal,'init','--auth','only','--target',target,'--out',out/('init-'+label+'.json')])
  cli('auth-metadata-'+label,'seed-auth-metadata',target,lf,['--auth-metadata',metadata])
  return db,target,lf
 def restore(label,archive):
  db,t,lf=create(label)
  # template1 already has an empty public schema. Remove only that schema
  # before restore, on this newly created owned database; no app rows exist.
  sql('empty-public-schema-'+label,db,'drop schema public;')
  command('restore-'+label,[tools['pg_restore'],'-h','127.0.0.1','-p',port,'-U','phase6','-d',db,'--no-owner','--exit-on-error','--single-transaction',archive],240)
  # pgcrypto is infrastructure, not application schema. Supabase stores it in
  # extensions; the native recovery cluster installs it into the same schema.
  sql('pgcrypto-'+label,db,"create schema if not exists extensions;create extension if not exists pgcrypto with schema extensions;")
  return db,t,lf
 try:
  if not all(tools.values()):raise RuntimeError('native tools unavailable')
  write('ownership.json',{'run_id':run,'data_directory':data.as_posix()})
  manifest={name:hashlib.sha256((source/name).read_bytes()).hexdigest() for name in ('apply-first.json','staging-before.dump','staging-before-dataonly.dump')};manifest.update({'plan_sha256':plan_sha,'batch_id':pdata['batch_id'],'project_fingerprint':a.expected_project});write('original-artifact-binding.json',manifest)
  binary=out/('import.exe' if os.name=='nt' else 'import');rehearsal=out/('rehearsal.exe' if os.name=='nt' else 'rehearsal')
  command('build-verifier',[tools['go'],'build','-o',binary,'./backend/cmd/phase6-live-import']);command('build-recovery-init',[tools['go'],'build','-o',rehearsal,'./backend/cmd/rehearsal'])
  before=cli('staging-postimport-before','snapshot')
  # Supply authentication only in the child environment, never argv or a
  # materialized passfile. It is neither printed nor persisted in artifacts.
  remoteenv=dict(env);remoteenv['PGPASSWORD']=unquote(u.password or '');remoteenv['PGSSLMODE']='require'
  sensitive=['auth_sessions','sso_handshakes','auth_rate_limits','audit_logs','academic_notifications','practicum_operations','learning_operations','academic_operations','practicum_runner_leases','learning_runner_leases']
  archive=out/'staging-postimport-sanitized.dump'
  try:command('backup-postimport',[tools['pg_dump'],'--host',u.hostname,'--port',u.port or 5432,'--username',unquote(u.username or ''),'--dbname',u.path[1:],'--no-password','--schema','public','--format','custom','--no-owner','--lock-wait-timeout=5000','--file',archive,*['--exclude-table-data=public.'+x for x in sensitive]],300,remoteenv)
  finally:remoteenv.pop('PGPASSWORD',None)
  after=cli('staging-postimport-after','snapshot');check('staging-stable-during-backup',before==after)
  check('backup-postimport-hash',archive.stat().st_size>0,bytes=archive.stat().st_size,sha256=hashlib.sha256(archive.read_bytes()).hexdigest())
  with socket.socket() as sock:sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
  command('initdb',[tools['initdb'],'-D',data,'-U','phase6','--auth-host=trust','--auth-local=trust','--encoding=UTF8','--locale=C'],90)
  command('start-owned-cluster',[tools['pg_ctl'],'-D',data,'-l',out/'postgres.log','-w','-t','30','-o',f'-h 127.0.0.1 -p {port} -c max_connections=24 -c timezone=UTC','start'],45)
  system=sql('cluster-system-id','postgres','select system_identifier::text from pg_control_system()').read_text().strip()
  # Provider ACL role names need to exist for the public-only dump; they have
  # no login and no secrets. Preserve ACLs rather than suppressing validation.
  sql('provider-role-prerequisites','postgres',"create role postgres nologin;create role service_role nologin;create role supabase_admin nologin;create role supabase_auth_admin nologin;")
  baseline_db,baseline_target,baseline_env=restore('baseline',source/'staging-before.dump')
  baseline=cli('baseline-restored-state','snapshot',baseline_target,baseline_env)
  check('baseline-profile-count',next(t['count'] for t in baseline['tables'] if t['table']=='users')==30)
  db,target,lf=restore('populated',archive)
  restored=cli('postimport-restored-state','snapshot',target,lf)
  check('public-restore-exact',restored['tables']==after['tables'] and restored['public_digest']==after['public_digest'] and restored['schema_sha256']==after['schema_sha256'])
  check('sequence-restore-exact',restored['sequences']==after['sequences'])
  check('auth-metadata-restored',restored['auth_users_metadata']==after['auth_users_metadata'])
  # State mutations only on the locally restored copy; private staging password
  # is not reused or overwritten and its hash is not copied into the backup.
  tenv=dict(env);tenv['PHASE6_ACCEPTANCE_PASSWORD']=secrets.token_urlsafe(30)
  cli('local-test-password','local-acceptance-password',target,lf,xenv=tenv)
  acceptance=next(u for u in pdata['auth_users'] if u['acceptance_login'])
  tenv.update({'ALGOHUB_PHASE6_LIVE_FLOWS':'true','ALGOHUB_TEST_DATABASE_URL':f'postgres://phase6@127.0.0.1:{port}/{db}?sslmode=disable','ALGOHUB_PHASE6_TARGET_FILE':str(target),'PHASE6_ACCEPTANCE_EMAIL':acceptance['email']})
  # This copy contains retained data from phases1-5. The test must assert per-flow
  # deltas and unchanged baseline, never require a globally empty target.
  log=command('imported-staging-restore-flows',[tools['go'],'test','-json','-count=1','-run','^TestPhase6LiveImportedFlows$','./backend/internal/repository'],240,tenv)
  events=[json.loads(l) for l in log.read_text().splitlines() if l.startswith('{')]
  ran=any(e.get('Action')=='run' and e.get('Test')=='TestPhase6LiveImportedFlows' for e in events);passed=any(e.get('Action')=='pass' and e.get('Test')=='TestPhase6LiveImportedFlows' for e in events)
  check('imported-test-executed',ran and passed and not any(e.get('Action') in ('skip','fail') for e in events),pass_events=sum(e.get('Action')=='pass' and bool(e.get('Test')) for e in events))
  summary['status']='passed'
 except Exception as exc:summary['status']='failed';summary['failure']=str(exc)
 finally:
  (out/'private.pgpass').unlink(missing_ok=True)
  if (data/'PG_VERSION').exists():
   try:
    r=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
    if r.returncode==0:command('stop-owned-cluster',[tools['pg_ctl'],'-D',data,'-m','fast','-w','-t','30','stop'],45)
    r=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
    if r.returncode!=3:raise RuntimeError('owned cluster shutdown unconfirmed')
    shutil.rmtree(data);summary['cleanup']={'cluster_stopped':True,'pgdata_removed':True,'password_file_removed':not (out/'private.pgpass').exists(),'archives_retained':True}
   except Exception as exc:summary['status']='failed';summary['cleanup']={'failure':str(exc),'pgdata_preserved':data.exists()}
  (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps({'evidence_dir':str(out),'status':summary['status'],'checks':len(summary['checks']),'failure':summary.get('failure'),'cleanup':summary['cleanup']}),flush=True)
 return 0 if summary['status']=='passed' else 1
if __name__=='__main__':raise SystemExit(main())
