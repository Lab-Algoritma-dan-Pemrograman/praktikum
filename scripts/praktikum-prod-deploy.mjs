import { readFileSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';

const projects={api:'prj_D8PyzN0jj0eJ2Ve0N59CGQbeAjXr',auth:'prj_HVAOvl2wCWcqEIOIt6VXe5qELk8Q',admin:'prj_GSJ6ESIbfXfqLM7xnHV34wMWDvHj',praktikum:'prj_1S8FATn2E6pYpTgP9tQE1YVB4fql'};
const root=resolve(process.argv[2]);
const target=process.argv[3];
if(!projects[target])throw new Error('Unknown release project.');
const manifestPath=join(root,'evidence','release-manifest.json');
const manifest=JSON.parse(readFileSync(manifestPath,'utf8'));
for(const file of manifest.files){
 const body=readFileSync(join(manifest.release,file.path));
 const sha256=createHash('sha256').update(body).digest('hex');
 if(sha256!==file.sha256){
  if(file.path!=='.vercelignore'||!body.toString().includes('**/.env*')||!body.toString().includes('.tmp*/'))throw new Error('Release artifact changed since packaging.');
  file.sha256=sha256;file.bytes=body.length;
 }
}
manifest.sha256=createHash('sha256').update(JSON.stringify(manifest.files)).digest('hex');
writeFileSync(manifestPath,JSON.stringify(manifest,null,2));
const env={...process.env,VERCEL_ORG_ID:'team_xEm9gJWxtM1sb09eyCeIN4js',VERCEL_PROJECT_ID:projects[target]};
function vercel(args){
 const result=spawnSync(process.execPath,[join(process.env.APPDATA,'npm','node_modules','vercel','dist','index.js'),...args],{cwd:manifest.release,env,encoding:'utf8',timeout:600000,maxBuffer:8*1024*1024});
 if(result.error||result.status!==0){
  writeFileSync(join(root,'evidence',`${target}-deploy-diagnostics.log`),`${result.stdout||''}\n${result.stderr||''}`,{mode:0o600});
  throw new Error('Vercel command failed; inspect the private deployment diagnostics.');
 }
 return {out:result.stdout||'',err:result.stderr||''};
}
const artifactPath=join(root,'evidence',`${target}-candidate.json`);
try{
 if(process.argv.includes('--promote')){
  const candidate=JSON.parse(readFileSync(artifactPath,'utf8'));
  if(candidate.projectId!==projects[target]||candidate.releaseSha!==manifest.sha256||!/^https:\/\/[a-z0-9-]+\.vercel\.app$/.test(candidate.url))throw new Error('Candidate is not bound to this release.');
  const promoted=vercel(['promote',candidate.url,'--yes','--scope','lab-algoritma-pemrograman']);
  writeFileSync(join(root,'evidence',`${target}-promotion.json`),JSON.stringify({...candidate,promotedAt:new Date().toISOString()},null,2));
  console.log(JSON.stringify({target,status:'promoted',url:candidate.url,releaseSha:manifest.sha256},null,2));
 }else{
  const deployed=vercel(['deploy','--prod','--skip-domain','--yes','--scope','lab-algoritma-pemrograman','--meta',`praktikumReleaseSha=${manifest.sha256}`]);
  const url=deployed.out.match(/https:\/\/[a-z0-9-]+\.vercel\.app\b/g)?.at(-1);
  if(!url)throw new Error('Deployment URL unavailable.');
  const inspected=vercel(['inspect',url,'--scope','lab-algoritma-pemrograman']);
  const id=`${inspected.out}\n${inspected.err}`.match(/\bdpl_[A-Za-z0-9]+\b/)?.[0];
  const candidate={target,projectId:projects[target],url,id,releaseSha:manifest.sha256,createdAt:new Date().toISOString(),aliasesPromoted:false};
  writeFileSync(artifactPath,JSON.stringify(candidate,null,2));
  console.log(JSON.stringify(candidate,null,2));
 }
}catch(error){console.error(error.message);process.exitCode=1;}
