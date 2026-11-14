import fs from 'node:fs';
import assert from 'node:assert/strict';
import {chromium} from '@playwright/test';
const argv=process.argv.slice(2);const value=k=>argv[argv.indexOf(k)+1];
const plan=JSON.parse(fs.readFileSync(value('--plan'),'utf8'));
const credential=JSON.parse(fs.readFileSync(value('--private'),'utf8'));
const state=JSON.parse(fs.readFileSync(value('--sessions'),'utf8'));
const acceptance=plan.auth_users.find(u=>u.acceptance_login);
assert.equal(credential.email,acceptance.email);
assert.equal(state.scope,'phase6_single_acceptance_identity');
const roots=state.sessions.filter(s=>s.active && s.id===s.family_id).map(s=>s.id);
const auth='https://auth.staging.algohub.web.id';
const app='https://elearning.staging.algohub.web.id';
const report={scope:'owned-phase6-acceptance-session-cleanup',status:'failed',old_roots:roots.length,deleted:0,cleanup:[]};
let browser,context,page,loggedIn=false;
async function api(path,method='GET'){
 return page.evaluate(async({path,method})=>{
  const csrf=document.cookie.split(';').map(s=>s.trim()).find(s=>s.startsWith('algohub_csrf='))?.slice('algohub_csrf='.length)||'';
  const r=await fetch('/api/v1'+path,{method,cache:'no-store',headers:{Accept:'application/json',...(method!=='GET'?{'Content-Type':'application/json','X-CSRF-Token':decodeURIComponent(csrf)}:{})},...(method==='POST'?{body:'{}'}:{})});
  let body={};try{body=await r.json();}catch{}return {status:r.status,data:body.data};
 },{path,method});
}
try{
 browser=await chromium.launch({headless:true});context=await browser.newContext();page=await context.newPage();page.setDefaultTimeout(45000);
 await context.route('**/*',route=>[auth,app].includes(new URL(route.request().url()).origin)?route.continue():route.abort());
 await page.goto(app+'/login');
 await page.getByLabel('Email',{exact:true}).fill(credential.email);
 await page.getByLabel('Password',{exact:true}).fill(credential.password);
 loggedIn=true;await page.getByRole('button',{name:'Masuk',exact:true}).click();await page.waitForURL(u=>u.origin===app&&u.pathname==='/');
 const me=await api('/auth/me');assert.equal(me.status,200);assert.equal(me.data.user.email,acceptance.email);
 const listing=await api('/auth/sessions');assert.equal(listing.status,200);
 for(const id of roots){
  assert.ok(listing.data.some(s=>s.id===id),'owned root not in self-session listing');
  const r=await api('/auth/sessions/'+encodeURIComponent(id),'DELETE');assert.equal(r.status,200);report.deleted++;
 }
 report.status='passed';
}catch{report.failure='Owned acceptance-session cleanup failed; credential contents withheld.';}
finally{
 if(page&&loggedIn){try{const r=await api('/auth/logout','POST');report.cleanup.push({action:'current-family-logout',http:r.status});if(r.status!==200)report.status='failed';else{const me=await api('/auth/me');report.cleanup.push({action:'current-family-revoked',http:me.status});if(me.status!==401)report.status='failed';}}catch{report.status='failed';report.cleanup.push({action:'current-family-logout',status:'failed'});}}
 await context?.close();await browser?.close();fs.writeFileSync(value('--out'),JSON.stringify(report,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify(report));
}
process.exitCode=report.status==='passed'?0:1;
