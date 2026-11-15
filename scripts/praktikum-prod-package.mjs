import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, copyFileSync, writeFileSync, existsSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { createHash } from 'node:crypto';

const root=resolve(process.argv[2]);
const release=join(root,'release');
if(existsSync(release))throw new Error('Release directory already exists; refusing to overwrite it.');
mkdirSync(release);
const targets=new Set(['auth','admin','praktikum']);
const files=execFileSync('git',['ls-files','-z'],{encoding:'utf8'}).split('\0').filter(Boolean).filter(path=>{
 if(['package.json','package-lock.json','go.work','go.work.sum','.vercelignore'].includes(path))return true;
 if(path.startsWith('backend/'))return !path.endsWith('_test.go')&&!path.includes('/tests/');
 if(path.startsWith('packages/'))return !path.includes('/tests/')&&!path.endsWith('.test.ts');
 const app=path.match(/^apps\/([^/]+)\/(.+)$/);
 return !!app&&(app[2]==='package.json'||targets.has(app[1]));
});
const additions=['backend/internal/usecase/practicum_persistence.go','backend/migrations/0009_activation_scope_shift.sql','backend/migrations/0010_activation_roster_backfill.sql','backend/migrations/0011_practicum_attempt_metadata_repair.sql','backend/migrations/0012_practicum_legacy_scope_context.sql','packages/ui/src/ServicePrismCards.svelte','packages/ui/src/prism-cards.ts'];
const selected=[...new Set([...files,...additions])].sort();
const manifest=[];
for(const path of selected){
 if(path.split('/').some(part=>part.startsWith('.env'))||path.includes('.tmp')||path.includes('__preview'))throw new Error('Private/preview material selected for release.');
 const body=readFileSync(path);
 const destination=join(release,path);mkdirSync(dirname(destination),{recursive:true});copyFileSync(path,destination);
 manifest.push({path,bytes:body.length,sha256:createHash('sha256').update(body).digest('hex')});
}
writeFileSync(join(release,'.vercelignore'),['.env*','**/.env*','.claude/','.tmp*/','docs/','memory/','tests/','**/tests/','**/*_test.go','**/node_modules/','**/.svelte-kit/','**/.vercel/','**/*.log','**/*.exe'].join('\n')+'\n');
const releaseHash=createHash('sha256').update(JSON.stringify(manifest)).digest('hex');
writeFileSync(join(root,'evidence','release-manifest.json'),JSON.stringify({baseCommit:execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim(),release,sha256:releaseHash,files:manifest},null,2));
console.log(JSON.stringify({release,sha256:releaseHash,files:manifest.length,privateEnvCopied:false,unrelatedPreviewCopied:false},null,2));
