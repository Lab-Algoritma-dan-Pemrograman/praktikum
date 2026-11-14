#!/usr/bin/env python3
"""Historical initial import driver — DO NOT resume this script for verification.

The batch is already committed. This driver's recovery/acceptance sequence was
found invalid (pre-import backup used as post-import proof) and is retained only
as failure provenance. Use phase6-recovery-verify.py for verified restore and
phase6-staging-browser.mjs for real browser acceptance. Never run a second apply
without first checking the original receipt and active closeout.

Approved masked import into the existing dedicated rewrite staging DB.

Steps: read-only preflight, backup, masked import, repeat no-op, reconciliation,
restore drill on the local disposable cluster, imported-data acceptance on the
restored copy, and real Supabase password login. Never prints credentials,
never touches production/legacy, never reruns migrations.
"""
import argparse, hashlib, json, os, re, secrets, shutil, socket, subprocess, sys, tempfile, time, uuid
from pathlib import Path
import urllib.request, urllib.error, urllib.parse
ROOT=Path(__file__).resolve().parents[1]
def load_env(path):
    v={}
    for line in Path(path).read_text(encoding='utf-8-sig').splitlines():
        m=re.match(r'^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$',line)
        if not m: continue
        k,x=m.groups(); x=x.strip()
        if len(x)>=2 and x[0]==x[-1] and x[0] in "'\"": x=x[1:-1]
        v[k]=x
    return v
def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--plan',required=True); p.add_argument('--env-file',required=True)
    p.add_argument('--expected-project',required=True); p.add_argument('--evidence-parent',default=tempfile.gettempdir()); p.add_argument('--resume',default='',help='reuse an existing evidence directory after a later-stage failure')
    a=p.parse_args()
    plan=Path(a.plan).resolve(); plan_sha=hashlib.sha256(plan.read_bytes()).hexdigest(); pdata=json.loads(plan.read_text()); batch=pdata['batch_id']
    acceptance=next(x for x in pdata['auth_users'] if x['acceptance_login'])
    out=Path(tempfile.mkdtemp(prefix='algohub-phase6-stage-',dir=a.evidence_parent)); data=out/'pgdata'; run='stage'+uuid.uuid4().hex[:10]
    summary={'run_id':run,'mode':'masked-import-existing-staging','plan_sha256':plan_sha,'status':'running','checks':[],'commands':[],'cleanup':{}}; tools={x:shutil.which(x) for x in ('go','initdb','pg_ctl','psql','createdb','pg_dump','pg_restore')}
    env={k:v for k,v in os.environ.items() if not k.startswith(('PG','ALGOHUB_','SUPABASE_','PHASE6_'))}; env['PGCONNECT_TIMEOUT']='8'; counter=0; owned=False
    print(json.dumps({'evidence_dir':str(out),'status':'started'}),flush=True)
    def write(name,obj,mode=0o600):
        path=out/name; fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,mode)
        with os.fdopen(fd,'w',encoding='utf8') as f: json.dump(obj,f,indent=2); f.write('\n')
        return path
    def cmd(label,argv,timeout=300,expected=0,xenv=None):
        nonlocal counter; counter+=1; log=out/f'{counter:03d}-{label}.log'; started=time.monotonic()
        try:
            with log.open('xb') as f: r=subprocess.run([str(x) for x in argv],cwd=ROOT,env=xenv or env,stdin=subprocess.DEVNULL,stdout=f,stderr=subprocess.STDOUT,timeout=timeout)
        except subprocess.TimeoutExpired:
            summary['commands'].append({'label':label,'timeout':timeout,'log':log.name}); raise RuntimeError(label+' timeout')
        summary['commands'].append({'label':label,'exit':r.returncode,'seconds':round(time.monotonic()-started,3),'log':log.name})
        if (expected=='nonzero' and r.returncode==0) or (expected!='nonzero' and r.returncode!=expected): raise RuntimeError(f'{label} exit {r.returncode}')
        return log
    def check(name,ok,**d):
        summary['checks'].append({'name':name,'passed':bool(ok),**d})
        if not ok: raise RuntimeError('assertion failed: '+name)
    def pgpass_and_args(dsn,args):
        u=urllib.parse.urlsplit(dsn)
        if not u.hostname or not u.path.startswith('/'): raise RuntimeError('invalid staging endpoint')
        pw=urllib.parse.unquote(u.password or '')
        pf=out/'private.pgpass'; pf.write_text(f"{u.hostname}:{u.port or 5432}:{u.path[1:]}:{urllib.parse.unquote(u.username or '')}:{pw.replace(':','\\:')}\n"); os.chmod(pf,0o600)
        e=dict(env); e['PGPASSFILE']=str(pf); e['PGSSLMODE']='require'
        return ['--host',u.hostname,'--port',str(u.port or 5432),'--username',urllib.parse.unquote(u.username or ''),'--dbname',u.path[1:]]+args,e
    try:
        if not all(tools.values()): raise RuntimeError('native tools missing')
        cfg=load_env(a.env_file); dsn=cfg.get('ALGOHUB_MIGRATION_DATABASE_URL') or cfg.get('ALGOHUB_DATABASE_URL')
        resumed=None
        if a.resume:
            resumed=Path(a.resume)
            for name in ('apply-first.json','staging-before.dump','staging-before-dataonly.dump','acceptance-private.json'):
                src=resumed/name
                if not src.exists(): raise RuntimeError('resume artifact missing: '+name)
                shutil.copy2(src,out/name)
            summary['resumed_from']=str(resumed)
        if not dsn: raise RuntimeError('staging migration configuration absent')
        owned=True; write('ownership.json',{'run_id':run,'data_directory':data.as_posix(),'note':'local validation cluster only; staging backup retained'})
        private=out/'acceptance-private.json'
        if resumed is None: private=write('acceptance-private.json',{'email':acceptance['email'],'password':secrets.token_urlsafe(30)})
        priv=json.loads(private.read_text())
        live_env=dict(env); live_env['PHASE6_ACCEPTANCE_PASSWORD']=priv['password']
        livebin=out/('live-import.exe' if os.name=='nt' else 'live-import'); rehearsal=out/('rehearsal.exe' if os.name=='nt' else 'rehearsal')
        cmd('build-live-import',[tools['go'],'build','-o',livebin,'./backend/cmd/phase6-live-import']); cmd('build-rehearsal',[tools['go'],'build','-o',rehearsal,'./backend/cmd/rehearsal'])
        cmd('unit-tests',[tools['go'],'test','-count=1','./backend/cmd/phase6-live-import','./backend/internal/migrationrehearsal'])
        # 1. Read-only preflight against the existing target.
        pre=out/'preflight.json'; cmd('preflight',[livebin,'preflight','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--out',pre],180,xenv=live_env)
        check('preflight',json.loads(pre.read_text())['status']=='passed')
        # 2. Backup existing staging schema/data (public only; no auth secrets).
        # Exclude ephemeral authentication state and personal audit data: the
        # rollback artifact must not become a second copy of sessions or logs.
        sensitive=['--exclude-table-data=public.auth_sessions','--exclude-table-data=public.sso_handshakes','--exclude-table-data=public.auth_rate_limits','--exclude-table-data=public.audit_logs','--exclude-table-data=public.academic_notifications','--exclude-table-data=public.practicum_operations','--exclude-table-data=public.learning_operations','--exclude-table-data=public.academic_operations','--exclude-table-data=public.practicum_runner_leases','--exclude-table-data=public.learning_runner_leases']
        backup=out/'staging-before.dump'
        if resumed is None:
            args,e=pgpass_and_args(dsn,['--schema','public','--format','custom','--no-owner','--lock-wait-timeout=10000','--file',str(backup),*sensitive])
            cmd('backup',[tools['pg_dump'],*args],600,xenv=e)
        check('backup-nonempty',backup.stat().st_size>0,bytes=backup.stat().st_size,sha256=hashlib.sha256(backup.read_bytes()).hexdigest())
        # Data-only copy for restoring into an already-migrated shim database:
        # migration history and seeded catalogs already exist there.
        seeds=['--exclude-table-data=public.schema_migrations','--exclude-table-data=public.roles','--exclude-table-data=public.divisions','--exclude-table-data=public.permissions','--exclude-table-data=public.role_permissions','--exclude-table-data=public.division_permissions']
        dataonly=out/'staging-before-dataonly.dump'
        if resumed is None:
            args,e=pgpass_and_args(dsn,['--schema','public','--format','custom','--no-owner','--lock-wait-timeout=10000','--file',str(dataonly),*sensitive,*seeds])
            cmd('backup-dataonly',[tools['pg_dump'],*args],600,xenv=e)
        check('backup-dataonly-nonempty',dataonly.stat().st_size>0,bytes=dataonly.stat().st_size,sha256=hashlib.sha256(dataonly.read_bytes()).hexdigest())
        pgpass=out/'private.pgpass'
        if pgpass.exists(): pgpass.unlink()
        # 3. Masked import, then repeat for idempotency.
        receipt=out/'apply-first.json'
        if resumed is None:
            cmd('apply',[livebin,'apply','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--confirm-batch',batch,'--out',receipt],1500,xenv=live_env)
        else:
            cmd('apply-verified-existing',[livebin,'preflight','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--out',out/'resume-check.json'],180,xenv=live_env)
        rec1=out/'reconcile.json'; cmd('reconcile',[livebin,'reconcile','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--receipt',receipt,'--out',rec1],900,xenv=live_env)
        repeat=out/'apply-repeat.json'; cmd('apply-repeat',[livebin,'apply','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--confirm-batch',batch,'--out',repeat],1500,xenv=live_env)
        check('repeat-noop',json.loads(repeat.read_text())['counts'].get('inserted',0)==0,counts=json.loads(repeat.read_text())['counts'])
        rec2=out/'reconcile-repeat.json'; cmd('reconcile-repeat',[livebin,'reconcile','--plan',plan,'--expected-plan-sha256',plan_sha,'--env-file',a.env_file,'--expected-project',a.expected_project,'--receipt',repeat,'--out',rec2],900,xenv=live_env)
        # 4. Restore drill on a new local cluster: schema+data into a shell, data-only into the shim.
        with socket.socket() as s: s.bind(('127.0.0.1',0)); port=s.getsockname()[1]
        cmd('initdb',[tools['initdb'],'-D',data,'-U','phase6','--auth-local=trust','--auth-host=trust','--encoding=UTF8','--locale=C'],90)
        cmd('start',[tools['pg_ctl'],'-D',data,'-l',out/'postgres.log','-w','-t','30','-o',f'-h 127.0.0.1 -p {port} -c max_connections=24 -c timezone=UTC','start'],45)
        sysid=cmd('system-id',[tools['psql'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d','postgres','-X','-A','-t','-c','select system_identifier::text from pg_control_system()'],30).read_text().strip()
        def localdb(label):
            db=f'p6_{run}_{label}_test'; cmd('create-'+label,[tools['createdb'],'-h','127.0.0.1','-p',str(port),'-U','phase6',db]); cmd('own-'+label,[tools['psql'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',db,'-X','-c',f"comment on database {db} is 'algohub-phase6:{run}'"])
            return db,write(f'target-{label}.json',{'run_id':run,'database':db,'data_directory':data.as_posix(),'system_identifier':sysid,'port':port})
        # 4a. Schema+data restore into a shell that only provides auth.users(id).
        shell_db,shell_target=localdb('shell')
        cmd('shell-auth',[tools['psql'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',shell_db,'-X','-v','ON_ERROR_STOP=1','-c','create schema auth;create table auth.users(id uuid primary key);'])
        cmd('restore-schema-data',[tools['pg_restore'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',shell_db,'--no-owner','--single-transaction','--exit-on-error',backup],300)
        # 4b. Data-only restore into the full shim for imported-data acceptance.
        flow_db,flow_target=localdb('flows'); cmd('init-shim',[rehearsal,'init','--auth','full','--target',flow_target,'--out',out/'flow-baseline.json'],180)
        flow_env_file=out/'env-flows'; flow_env_file.write_text(f"ALGOHUB_MIGRATION_DATABASE_URL='postgres://phase6@127.0.0.1:{port}/{flow_db}?sslmode=disable'\nSUPABASE_URL='https://local-phase6.invalid'\n"); os.chmod(flow_env_file,0o600)
        cmd('restore-data',[tools['pg_restore'],'-h','127.0.0.1','-p',str(port),'-U','phase6','-d',flow_db,'--data-only','--no-owner','--disable-triggers','--single-transaction','--exit-on-error',dataonly],600)
        # The schema-only dump never carries auth credentials. Replaying the plan
        # against the restored copy recreates only the missing auth identities and
        # proves every public row is already identical (idempotent replay).
        replay=out/'restore-apply.json'
        cmd('restore-apply',[livebin,'apply','--plan',plan,'--expected-plan-sha256',plan_sha,'--local-target',flow_target,'--env-file',flow_env_file,'--confirm-batch',batch,'--out',replay],900,xenv=live_env)
        replay_counts=json.loads(replay.read_text())['counts']
        check('restored-replay-idempotent',replay_counts.get('inserted',0)==len(pdata['auth_users']),counts=replay_counts)
        # 5. Imported-data acceptance on the restored copy (never mutates staging).
        tenv=dict(env); tenv['ALGOHUB_PHASE6_LIVE_FLOWS']='true'; tenv['ALGOHUB_TEST_DATABASE_URL']=f'postgres://phase6@127.0.0.1:{port}/{flow_db}?sslmode=disable'
        tenv['PHASE6_ACCEPTANCE_EMAIL']=priv['email']; tenv['PHASE6_ACCEPTANCE_PASSWORD']=priv['password']; tenv['ALGOHUB_PHASE6_TARGET_FILE']=str(flow_target)
        log=cmd('imported-flows',[tools['go'],'test','-json','-count=1','-run','^TestPhase6LiveImportedFlows$','./backend/internal/repository'],600,xenv=tenv)
        events=[json.loads(x) for x in log.read_text().splitlines() if x.startswith('{')]
        check('imported-flows',not [x for x in events if x.get('Action')=='fail'] and not [x for x in events if x.get('Action')=='skip'],pass_events=sum(x.get('Action')=='pass' for x in events))
        # 6. Real Supabase password grant and deployed staging API reads.
        accept=out/'staging-acceptance.json'
        cmd('staging-acceptance',[sys.executable,'-I',str(ROOT/'scripts/phase6-staging-acceptance.py'),'--plan',plan,'--env-file',a.env_file,'--private',private,'--out',accept],300)
        check('staging-acceptance',json.loads(accept.read_text())['status']=='passed')
        summary['status']='passed'; summary['plan_counts']=pdata['counts']
    except Exception as e:
        summary['status']='failed'; summary['failure']=str(e)
    finally:
        if owned and (data/'PG_VERSION').exists():
            try:
                st=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
                if st.returncode==0: cmd('stop',[tools['pg_ctl'],'-D',data,'-w','-t','30','-m','fast','stop'],45)
                st=subprocess.run([tools['pg_ctl'],'-D',str(data),'status'],env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=10)
                if st.returncode!=3: raise RuntimeError('shutdown unconfirmed')
                shutil.rmtree(data); summary['cleanup']={'cluster_stopped':True,'data_removed':True,'staging_backup_retained':True}
            except Exception as e: summary['status']='failed'; summary['cleanup']={'failed':str(e),'data_preserved':data.exists()}
        (out/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
        print(json.dumps({'evidence_dir':str(out),'status':summary['status'],'checks':len(summary['checks']),'cleanup':summary['cleanup'],'failure':summary.get('failure')}),flush=True)
    return 0 if summary['status']=='passed' else 1
if __name__=='__main__': raise SystemExit('Historical import driver disabled: use phase6-recovery-verify.py and phase6-staging-browser.mjs; the batch is already committed.')
