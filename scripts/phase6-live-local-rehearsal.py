#!/usr/bin/env python3
"""Full local rehearsal for the real masked Phase 6 plan using a new PG cluster."""
import argparse,hashlib,json,os,secrets,shutil,socket,subprocess,tempfile,time,uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--plan',required=True);p.add_argument('--evidence-parent',default=tempfile.gettempdir());a=p.parse_args();plan=Path(a.plan).resolve();plan_sha=hashlib.sha256(plan.read_bytes()).hexdigest();plan_data=json.loads(plan.read_text());batch=plan_data['batch_id'];acceptance=next(x for x in plan_data['auth_users'] if x['acceptance_login']);out=Path(tempfile.mkdtemp(prefix='algohub-phase6-masked-local-',dir=a.evidence_parent));data=out/'pgdata';run='phase6live-'+uuid.uuid4().hex[:12];summary={'run_id':run,'status':'running','plan_sha256':plan_sha,'checks':[],'commands':[],'cleanup':{}};tools={x:shutil.which(x) for x in ('go','initdb','pg_ctl','psql','createdb','pg_dump','pg_restore')};env={k:v for k,v in os.environ.items() if not k.startswith(('PG','ALGOHUB_','SUPABASE_','PHASE6_'))};env['PGPASSFILE']=os.devnull;env['PGCONNECT_TIMEOUT']='5';counter=0;owned=False
 print(json.dumps({'evidence_dir':str(out),'status':'started'}),flush=True)
 def source_checksums():
  files=list((ROOT/'backend').rglob('*.go'))+list((ROOT/'backend'/'migrations').glob('*.sql'))
  files += [ROOT/'backend'/'go.mod',ROOT/'backend'/'go.sum',ROOT/'go.work',Path(__file__).resolve()]
  return {p.relative_to(ROOT).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(files)}
 def write(name,obj,mode=0o600):
  path=out/name;fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,mode)
  with os.fdopen(fd,'w',encoding='utf8') as f:json.dump(obj,f,indent=2);f.write('\n')
  return path
 def cmd(label,argv,timeout=300,expected=0,xenv=None):
  nonlocal counter;counter+=1;log=out/f'{counter:03d}-{label}.log';start=time.monotonic()
  try:
   with log.open('xb') as f:r=subprocess.run([str(x) for x in argv],cwd=ROOT,env=xenv or env,stdin=subprocess.DEVNULL,stdout=f,stderr=subprocess.STDOUT,timeout=timeout)
  except subprocess.TimeoutExpired:summary['commands'].append({'label':label,'timeout':timeout,'log':log.name});raise RuntimeError(label+' timeout')
  summary['commands'].append({'label':label,'exit':r.returncode,'seconds':round(time.monotonic()-start,3),'log':log.name})
  if (expected=='nonzero' and r.returncode==0) or (expected!='nonzero' and r.returncode!=expected):raise RuntimeError(f'{label} exit {r.returncode}')
  return log
 def check(name,ok,**detail):
  summary['checks'].append({'name':name,'passed':bool(ok),**detail})
  if not ok:raise RuntimeError('assertion failed: '+name)
 def sql(label,db,q):return cmd(label,[tools['psql'],'-X','--no-psqlrc','-A','-t','-v','ON_ERROR_STOP=1','-h','127.0.0.1','-p',str(port),'-U','phase6','-d',db,'-c',q],60)
 def create(label):
  db=f'p6_{run.split("-")[1]}_{label}_test';cmd('create-'+label,[tools['createdb'],'-h','127.0.0.1','-p',str(port),'-U','phase6',db]);sql('own-'+label,db,f"comment on database {db} is 'algohub-phase6:{run}'");target=write('target-'+label+'.json',{'run_id':run,'database':db,'data_directory':data.as_posix(),'system_identifier':system,'port':port});envfile=out/('env-'+label);envfile.write_text(f"ALGOHUB_MIGRATION_DATABASE_URL='postgres://phase6@127.0.0.1:{port}/{db}?sslmode=disable'\nSUPABASE_URL='https://local-phase6.invalid'\n");os.chmod(envfile,0o600);return db,target,envfile
 def live(label,action,target,envfile,receipt=None,confirm=False,expected=0):
  outp=out/(label+'.json');argv=[livebin,action,'--plan',plan,'--expected-plan-sha256',plan_sha,'--local-target',target,'--env-file',envfile,'--out',outp]
  if receipt:argv+=['--receipt',receipt]
  if confirm:argv+=['--confirm-batch',batch]
  cmd(label,argv,900,expected,xenv=live_env);return outp
 def dataset_fingerprint(label,db):
  query="""with imported_users as(select id from public.users where email like 'migration.%@example.test'), rows as(
 select 'auth.users' n,to_jsonb(a) j from auth.users a where email like 'migration.%@example.test'
 union all select 'users',to_jsonb(u) from public.users u where email like 'migration.%@example.test'
 union all select 'assignments',to_jsonb(a) from user_role_assignments a where a.user_id in(select id from imported_users)
 union all select 'learning_progress',to_jsonb(t) from learning_progress t where owner_id in(select id from imported_users)
 union all select 'learning_profiles',to_jsonb(t) from learning_profiles t where owner_id in(select id from imported_users)
 union all select 'practicum_attempts',to_jsonb(t) from practicum_attempts t where owner_id in(select id from imported_users)
 union all select 'domains',to_jsonb(t) from learning_levels t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from learning_modules t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from learning_lessons t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from practicum_sessions t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from practicum_courses t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from practicum_questions t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from practicum_activations t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from practicum_grading_jobs t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from academic_classes t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from academic_schedules t where id like 'mig-%'
 union all select 'domains',to_jsonb(t) from academic_inventory_items t where id like 'mig-%')
 select n,count(*),encode(digest(coalesce(string_agg(j::text,E'\\n' order by j::text),''),'sha256'),'hex') from rows group by n order by n"""
  log=sql(label,db,"set search_path=public,extensions;"+query);return '\n'.join(x for x in log.read_text().splitlines() if x.strip() not in ('','SET'))
 try:
  if not all(tools.values()):raise RuntimeError('native PostgreSQL/Go tools missing')
  inputs=source_checksums();write('source-checksums.json',inputs)
  owned=True;write('ownership.json',{'run_id':run,'data_directory':data.as_posix()});private=write('acceptance-private.json',{'email':acceptance['email'],'password':secrets.token_urlsafe(30)});live_env=dict(env);live_env['PHASE6_ACCEPTANCE_PASSWORD']=json.loads(private.read_text())['password']
  rehearsal=out/('rehearsal.exe' if os.name=='nt' else 'rehearsal');livebin=out/('live-import.exe' if os.name=='nt' else 'live-import');cmd('build-rehearsal',[tools['go'],'build','-o',rehearsal,'./backend/cmd/rehearsal']);cmd('build-live-import',[tools['go'],'build','-o',livebin,'./backend/cmd/phase6-live-import']);cmd('unit-live-import',[tools['go'],'test','-count=1','./backend/cmd/phase6-live-import','./backend/internal/migrationrehearsal'])
  with socket.socket() as s:s.bind(('127.0.0.1',0));port=s.getsockname()[1]
  cmd('initdb',[tools['initdb'],'-D',data,'-U','phase6','--auth-local=trust','--auth-host=trust','--encoding=UTF8','--locale=C'],90);cmd('start',[tools['pg_ctl'],'-D',data,'-l',out/'postgres.log','-w','-t','30','-o',f'-h 127.0.0.1 -p {port} -c max_connections=24 -c timezone=UTC','start'],45)
  system=sql('system-id','postgres','select system_identifier::text from pg_control_system()').read_text().strip();main_db,target,envfile=create('main');baseline=out/'baseline.json';cmd('initialize-schema',[rehearsal,'init','--auth','full','--target',target,'--out',baseline],180)
  pre=live('preflight','preflight',target,envfile);receipt1=live('apply-first','apply',target,envfile,confirm=True);reconcile1=live('reconcile-first','reconcile',target,envfile,receipt1);receipt_repeat=live('apply-repeat','apply',target,envfile,confirm=True);repeat=json.loads(receipt_repeat.read_text());check('repeat-noop',repeat['counts'].get('inserted',0)==0,counts=repeat['counts'])
  rollback=live('rollback-first','rollback',target,envfile,receipt1,True);fp_empty=dataset_fingerprint('fingerprint-after-rollback',main_db);check('rollback-removed-dataset',fp_empty=='')
  receipt2=live('apply-second','apply',target,envfile,confirm=True);live('reconcile-second','reconcile',target,envfile,receipt2)
  populated=out/'masked-populated.dump';cmd('dump-populated',[tools['pg_dump'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',main_db,'-Fc','--no-owner','--lock-wait-timeout=5000','-f',populated],180);check('backup-nonempty',populated.stat().st_size>0,bytes=populated.stat().st_size,sha256=hashlib.sha256(populated.read_bytes()).hexdigest())
  acc_db,acc_target,acc_env=create('acceptance');cmd('restore-acceptance',[tools['pg_restore'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',acc_db,'--no-owner','--single-transaction','--exit-on-error',populated],240);fp_main=dataset_fingerprint('fingerprint-main',main_db);fp_acc=dataset_fingerprint('fingerprint-acceptance',acc_db);check('populated-restore-exact',fp_main==fp_acc)
  # Imported flow test may be added while this harness is developed; require it when present.
  testfile=ROOT/'backend/internal/repository/phase6_live_import_postgres_test.go'
  if not testfile.exists():raise RuntimeError('required imported-flow test is missing')
  if testfile.exists():
   private_data=json.loads(private.read_text());tenv=dict(env);tenv['ALGOHUB_PHASE6_LIVE_FLOWS']='true';tenv['ALGOHUB_TEST_DATABASE_URL']=f'postgres://phase6@127.0.0.1:{port}/{acc_db}?sslmode=disable';tenv['PHASE6_ACCEPTANCE_EMAIL']=private_data['email'];tenv['PHASE6_ACCEPTANCE_PASSWORD']=private_data['password'];tenv['ALGOHUB_PHASE6_TARGET_FILE']=str(acc_target);log=cmd('imported-live-flows',[tools['go'],'test','-json','-count=1','-run','^TestPhase6LiveImportedFlows$','./backend/internal/repository'],300,xenv=tenv);events=[json.loads(x) for x in log.read_text().splitlines() if x.startswith('{')];fails=[x for x in events if x.get('Action')=='fail'];skips=[x for x in events if x.get('Action')=='skip'];check('imported-live-flows',not fails and not skips and any(e.get('Action')=='pass' and e.get('Test')=='TestPhase6LiveImportedFlows' for e in events),pass_events=sum(x.get('Action')=='pass' for x in events))
  # Verify first database remains pristine after mutation testing on restored copy.
  live('reconcile-pristine-after-acceptance','reconcile',target,envfile,receipt2)
  # The ordinary integration suite gets its own EMPTY database; never run its
  # fixture/migrator setup against the populated acceptance/staging copy.
  reg_db,_,_=create('regression');regenv=dict(env);regenv['ALGOHUB_TEST_DATABASE_URL']=f'postgres://phase6@127.0.0.1:{port}/{reg_db}?sslmode=disable'
  log=cmd('backend-regression',[tools['go'],'test','-json','-count=1','./backend/...'],timeout=300,xenv=regenv)
  events=[json.loads(line) for line in log.read_text().splitlines() if line.startswith('{')]
  allowed_skips={'TestPhase6LiveImportedFlows','TestMigrationRehearsalImportedFlows'}
  skipped=[e.get('Test') for e in events if e.get('Action')=='skip' and e.get('Test')];failed=[e for e in events if e.get('Action')=='fail']
  passed=[e for e in events if e.get('Action')=='pass' and e.get('Test')]
  check('backend-postgres-regression',bool(passed) and not failed and all(s in allowed_skips for s in skipped),top_level_pass=sum('/' not in e['Test'] for e in passed),pass_events=len(passed),skipped=skipped)
  cmd('backend-vet',[tools['go'],'vet','./backend/...'],timeout=180);cmd('backend-build',[tools['go'],'build','./backend/...'],timeout=180)
  check('source-stable-through-tests',inputs==source_checksums(),files=len(inputs))
  summary['status']='passed';summary['plan_counts']=plan_data['counts'];summary['target_counts']={'auth_users':len(plan_data['auth_users']),'users':len(plan_data['users']),'assignments':len(plan_data['assignments']),'domain_rows':len(plan_data['domain_rows'])}
 except Exception as e:summary['status']='failed';summary['failure']=str(e)
 finally:
  keep=os.environ.get('PHASE6_KEEP_CLUSTER')=='1'
  if keep and (data/'PG_VERSION').exists():
   summary['cleanup']={'kept_for_debugging':True,'data_directory':data.as_posix(),'port':locals().get('port')}
   (out/'debug-cluster.json').write_text(json.dumps({'data_directory':data.as_posix(),'port':locals().get('port')})+'\n')
  elif owned and (data/'PG_VERSION').exists():
   try:
    st=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
    if st.returncode==0:cmd('stop',[tools['pg_ctl'],'-D',data,'-w','-t','30','-m','fast','stop'],45)
    st=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
    if st.returncode!=3:raise RuntimeError('shutdown unconfirmed')
    shutil.rmtree(data);summary['cleanup']={'cluster_stopped':True,'data_removed':True,'backups_retained':True}
   except Exception as e:summary['status']='failed';summary['cleanup']={'failed':str(e),'data_preserved':data.exists()}
  (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps({'evidence_dir':str(out),'status':summary['status'],'checks':len(summary['checks']),'cleanup':summary['cleanup'],'failure':summary.get('failure')}),flush=True)
 return 0 if summary['status']=='passed' else 1
if __name__=='__main__':raise SystemExit(main())
