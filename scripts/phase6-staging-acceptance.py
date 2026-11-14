#!/usr/bin/env python3
"""Historical API-only acceptance driver — disabled, never executed successfully.

Use phase6-staging-browser.mjs: it makes one real login, checks exact imported
records and logs out the owned session family. This historical driver has wrong
response-shape assertions and duplicate untracked logins; retained for provenance.

Real Supabase + staging API acceptance for the masked import batch.

No browser automation here: this proves the real Auth provider accepts the
imported acceptance identity and that the deployed staging API returns imported
curriculum/practicum records to that identity. It never prints tokens.
"""
import argparse, hashlib, json, re, sys
from pathlib import Path
import urllib.error, urllib.parse, urllib.request

def load_env(path):
    v={}
    for line in Path(path).read_text(encoding='utf-8-sig').splitlines():
        m=re.match(r'^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$',line)
        if not m: continue
        k,x=m.groups(); x=x.strip()
        if len(x)>=2 and x[0]==x[-1] and x[0] in "'\"": x=x[1:-1]
        v[k]=x
    return v

def request(url, method='GET', body=None, headers=None, timeout=20):
    data=None if body is None else json.dumps(body).encode()
    req=urllib.request.Request(url, data=data, method=method, headers=headers or {})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, dict(r.headers), r.read(2<<20)
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read(2<<20)

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--plan',required=True); p.add_argument('--env-file',required=True)
    p.add_argument('--private',required=True,help='acceptance email/password file produced by the import run')
    p.add_argument('--api-base',default='https://api.staging.algohub.web.id')
    p.add_argument('--out',required=True)
    a=p.parse_args()
    cfg=load_env(a.env_file); priv=json.loads(Path(a.private).read_text())
    plan=json.loads(Path(a.plan).read_text()); acceptance=next(x for x in plan['auth_users'] if x['acceptance_login'])
    supa=cfg['SUPABASE_URL'].rstrip('/'); anon=cfg['SUPABASE_ANON_KEY']
    origins=[x for x in cfg.get('ALGOHUB_ALLOWED_ORIGINS','').split(',') if x.strip()]
    origin=origins[0].strip() if origins else 'https://elearning.staging.algohub.web.id'
    report={'checks':[],'status':'failed'}
    def check(name,ok,**d):
        report['checks'].append({'name':name,'passed':bool(ok),**d})
        if not ok: raise RuntimeError(name)
    try:
        # 1. Real Supabase Auth password grant.
        status,h,body=request(f'{supa}/auth/v1/token?grant_type=password','POST',
            {'email':priv['email'],'password':priv['password']},
            {'apikey':anon,'Content-Type':'application/json'})
        check('supabase_password_grant',status==200,http=status)
        token=json.loads(body); user=token.get('user') or {}
        check('supabase_identity_matches_plan',user.get('id')==acceptance['auth_user_id'],masked_email=bool(user.get('email')))
        check('supabase_token_present',bool(token.get('access_token')))
        # 2. Deployed staging API login (session cookie) through the real provider.
        status,h,body=request(f'{a.api_base}/v1/auth/password','POST',
            {'email':priv['email'],'password':priv['password']},
            {'Origin':origin,'Content-Type':'application/json','Accept':'application/json'})
        check('staging_api_password_login',status==200,http=status)
        cookies=[]
        for k,v in h.items():
            if k.lower()=='set-cookie':
                pass
        raw=h.get('Set-Cookie') or ''
        # urllib collapses duplicates; re-issue with a cookie jar to be safe.
        import http.cookiejar
        jar=http.cookiejar.CookieJar()
        opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
        req=urllib.request.Request(f'{a.api_base}/v1/auth/password',data=json.dumps({'email':priv['email'],'password':priv['password']}).encode(),
            method='POST',headers={'Origin':origin,'Content-Type':'application/json','Accept':'application/json'})
        with opener.open(req,timeout=20) as r:
            check('staging_api_login_repeat',r.status==200)
        cookie_header='; '.join(f'{c.name}={c.value}' for c in jar)
        csrf=next((c.value for c in jar if 'csrf' in c.name.lower()),'')
        check('staging_session_cookie_present',bool(cookie_header))
        def api(path):
            st,hh,bb=request(a.api_base+path,'GET',None,{'Origin':origin,'Accept':'application/json','Cookie':cookie_header})
            return st,(json.loads(bb) if bb else {})
        st,me=api('/v1/auth/me'); check('staging_auth_me',st==200,http=st)
        data=me.get('data') or {}
        check('staging_identity_is_imported',(data.get('email') or '').lower()==priv['email'].lower(),role=data.get('role'))
        st,levels=api('/v1/learning/levels'); check('learning_levels_read',st==200,http=st)
        lv=json.dumps(levels)
        check('imported_curriculum_visible','mig-lvl-' in lv)
        st,sessions=api('/v1/practicum/sessions'); check('practicum_sessions_read',st==200,http=st)
        check('imported_practicum_visible','mig-psn-' in json.dumps(sessions))
        st,grades=api('/v1/practicum/grades'); check('practicum_grades_read',st==200,http=st)
        st,classes=api('/v1/academic/classes'); check('academic_classes_read',st==200,http=st)
        # 3. Logout invalidates the session.
        logout_headers={'Origin':origin,'Accept':'application/json','Cookie':cookie_header}
        if csrf: logout_headers['X-CSRF-Token']=csrf
        st,_,_=request(a.api_base+'/v1/auth/logout','POST',{},logout_headers)
        check('staging_logout',st==200,http=st)
        st,_,_=request(a.api_base+'/v1/auth/me','GET',None,{'Cookie':cookie_header,'Accept':'application/json'})
        check('session_revoked_after_logout',st==401,http=st)
        report['status']='passed'
    except Exception as e:
        report['failure']=str(e)
    Path(a.out).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'status':report['status'],'checks':len(report['checks']),'failure':report.get('failure')}))
    return 0 if report['status']=='passed' else 1

if __name__=='__main__': sys.exit('Historical API acceptance driver disabled: use phase6-staging-browser.mjs.')
