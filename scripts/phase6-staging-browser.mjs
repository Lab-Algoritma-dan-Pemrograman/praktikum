import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import { chromium } from '@playwright/test';

// Drives existing deployed staging applications, not local memory fixtures.
// Private credential file is read only in this process; traces/HARs are disabled.
const options = Object.fromEntries(process.argv.slice(2).reduce((a,v,i,all) => {
  if (v.startsWith('--') && all[i+1]) a.push([v.slice(2),all[i+1]]); return a;
},[]));
const plan = JSON.parse(fs.readFileSync(options.plan,'utf8'));
const receipt = JSON.parse(fs.readFileSync(options.receipt,'utf8'));
const credentials = JSON.parse(fs.readFileSync(options.private,'utf8'));
const acceptance = plan.auth_users.find(u=>u.acceptance_login);
assert.equal(credentials.email,acceptance.email,'acceptance credential identity mismatch');
assert.equal(receipt.batch_id,plan.batch_id,'receipt batch mismatch');
const userRow = receipt.rows.find(r=>r.kind==='user' && r.token===acceptance.token);
const users = Object.fromEntries(receipt.rows.filter(r=>r.kind==='user').map(r=>[r.token,Number(r.key)]));
const ids = Object.fromEntries(receipt.rows.filter(r=>r.kind==='domain').map(r=>[r.token,r.key]));
const lessonProgress = plan.domain_rows.find(r=>r.table==='learning_progress' && r.owner_token===acceptance.token);
assert.ok(lessonProgress,'acceptance user has no imported progress');
const lessonId=ids[lessonProgress.parent_id];
const classRow=plan.domain_rows.find(r=>r.table==='academic_classes' && r.data.student_ids?.some(x=>x.$user===acceptance.token));
const origins = Object.fromEntries(['portal','auth','admin','elearning','praktikum','siakad'].map(app=>[app,`https://${app}.staging.algohub.web.id`]));
const report={scope:'existing-staging-imported-data-browser',status:'failed',checks:[],cleanup:[],batch_id:plan.batch_id};
fs.mkdirSync(options.out,{recursive:false});
const check=(name,condition,detail={})=>{report.checks.push({name,passed:!!condition,...detail});assert.ok(condition,name);};
let browser,context,page;let loggedIn=false;let loginForms=0;let browserErrors=0;
async function bff(page,path){return page.evaluate(async path=>{const r=await fetch('/api/v1'+path,{headers:{Accept:'application/json'},cache:'no-store'});const body=await r.json();return {status:r.status,data:body.data,code:body.error?.code};},path);}
async function noCredentialLeak(page){
 const cookies=(await context.cookies()).filter(c=>c.name==='algohub_session');
 const exposed=await page.evaluate(()=>JSON.stringify({url:location.href,dom:document.documentElement.innerHTML,local:{...localStorage},session:{...sessionStorage}}));
 check('no-session-in-dom-url-storage:'+new URL(page.url()).hostname,cookies.every(c=>!exposed.includes(c.value)));
}
async function logout(){
 if(!page || !loggedIn)return;
 try{
  // Logout from the origin on which this run actually has a session; simply
  // visiting the portal root does not establish its host-only cookie.
  const origin=(await context.cookies()).find(c=>c.name==='algohub_session' && c.domain==='elearning.staging.algohub.web.id')?origins.elearning:new URL(page.url()).origin;
  await page.goto(origin,{waitUntil:'domcontentloaded'});
  const r=await page.evaluate(async()=>{const csrf=document.cookie.split(';').map(s=>s.trim()).find(s=>s.startsWith('algohub_csrf='))?.slice('algohub_csrf='.length)||'';const r=await fetch('/api/v1/auth/logout',{method:'POST',headers:{Accept:'application/json','Content-Type':'application/json','X-CSRF-Token':decodeURIComponent(csrf)},body:'{}'});return r.status;});
  report.cleanup.push({name:'owned-login-family-logout',status:r});
  if(r!==200)throw new Error('logout failed');loggedIn=false;
  for(const [app,origin] of Object.entries(origins)){
   await page.goto(origin,{waitUntil:'domcontentloaded'});const me=await bff(page,'/auth/me');check('logout-401:'+app,me.status===401,{http:me.status});
  }
 }catch{report.cleanup.push({name:'logout_failed',status:'failed'});report.status='failed';}
}
try{
 browser=await chromium.launch({headless:true});
 context=await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce'});
 await context.route('**/*',route=>Object.values(origins).includes(new URL(route.request().url()).origin)?route.continue():route.abort());
 page=await context.newPage();page.setDefaultTimeout(30000);page.setDefaultNavigationTimeout(45000);
 page.on('pageerror',()=>{browserErrors++;});
 page.on('request',req=>{if(req.method()==='POST' && new URL(req.url()).pathname==='/login')loginForms++;});
 await page.goto(origins.elearning+'/login',{waitUntil:'domcontentloaded'});
 check('auth-center-login-route',new URL(page.url()).origin===origins.auth);
 await page.getByLabel('Email',{exact:true}).fill(credentials.email);
 await page.getByLabel('Password',{exact:true}).fill(credentials.password);
 loggedIn=true;await page.getByRole('button',{name:'Masuk',exact:true}).click();
 await page.waitForURL(url=>url.origin===origins.elearning && url.pathname==='/');
 let me=await bff(page,'/auth/me');check('imported-user-login',me.status===200 && me.data?.user?.email===acceptance.email && String(me.data.user.id)===userRow.key,{http:me.status,error_code:me.code,auth_id_redacted:me.data?.user?.auth_user_id===undefined});
 const levels=await bff(page,'/learning/levels');check('imported-level-set',levels.status===200 && plan.domain_rows.filter(r=>r.table==='learning_levels').every(r=>levels.data.levels.some(l=>l.id===r.id)));
 await page.locator('[data-level-id^="mig-lvl-"]').first().waitFor();
 check('imported-curriculum-rendered',(await page.locator('[data-level-id^="mig-lvl-"]').count())===plan.domain_rows.filter(r=>r.table==='learning_levels').length);
 await page.screenshot({path:path.join(options.out,'imported-curriculum.png')});
 await page.goto(origins.elearning+'/lesson/'+encodeURIComponent(lessonId),{waitUntil:'domcontentloaded'});
 const lesson=await bff(page,'/learning/lessons/'+encodeURIComponent(lessonId));check('owned-imported-progress-visible',lesson.status===200 && lesson.data.lesson.id===lessonId && lesson.data.progress.user_id===users[acceptance.token] && lesson.data.progress.status==='completed' && lesson.data.progress.xp_awarded===0);
 await noCredentialLeak(page);
 for(const app of ['portal','praktikum','siakad','admin','auth']){
  await page.goto(origins[app]+'/login',{waitUntil:'domcontentloaded'});
  await page.waitForURL(url=>url.origin===origins[app] && url.pathname!=='/callback');
  me=await bff(page,'/auth/me');check('cross-host-imported-identity:'+app,me.status===200 && String(me.data.user.id)===userRow.key);
  if(app==='admin'){const r=await page.goto(origins.admin+'/');check('student-admin-forbidden',r?.status()===403,{http:r?.status()});}
  if(app==='praktikum'){
   const grades=await bff(page,'/practicum/grades');check('imported-grade-read',grades.status===200);
   const sessions=await bff(page,'/practicum/sessions');check('session-read',sessions.status===200);
   const own=plan.domain_rows.filter(r=>r.table==='practicum_attempts' && r.owner_token===acceptance.token).map(r=>r.id);
   check('exact-owned-attempt-set',own.every(id=>grades.data.some(g=>g.attempt_id===id)),{expected_imported_attempts:own.length});
  }
  if(app==='siakad'){
   const classes=await bff(page,'/academic/classes');check('owned-imported-class-visible',classes.status===200 && (!classRow || classes.data.some(c=>c.id===classRow.id)));
   const schedules=await bff(page,'/academic/schedules');check('imported-schedule-read',schedules.status===200);
  }
  await noCredentialLeak(page);
 }
 check('one-password-form-exchange',loginForms===1,{form_submissions:loginForms});
 const sessions=(await context.cookies()).filter(c=>c.name==='algohub_session');
 check('six-host-only-secure-session-cookies',sessions.length===6 && new Set(sessions.map(s=>s.value)).size===6 && sessions.every(c=>!c.domain.startsWith('.')&&c.httpOnly&&c.secure&&c.sameSite==='Lax'),{cookies:sessions.length});
 check('no-browser-page-errors',browserErrors===0,{errors:browserErrors});
 report.status='passed';
}catch(error){report.failure=error instanceof assert.AssertionError ? error.message.split('\n')[0] : 'Browser acceptance failed; credentials/request contents withheld.';}
finally{
 await logout();await context?.close();await browser?.close();
 fs.writeFileSync(path.join(options.out,'summary.json'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});
 console.log(JSON.stringify({status:report.status,checks:report.checks.length,cleanup:report.cleanup,failure:report.failure}));
}
process.exitCode=report.status==='passed'?0:1;
