import { readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { productionEnv, psqlJson } from './praktikum-prod-env.mjs';
import { buildRepairSQL, workFingerprintSQL } from './praktikum-prod-repair-sql.mjs';

const root=resolve(process.argv[2]);
try{
 if(!process.argv.includes('--apply-production'))throw new Error('Explicit production apply mode is required.');
 const backup=JSON.parse(readFileSync(join(root,'evidence','backup-manifest.json'),'utf8'));
 const rehearsal=JSON.parse(readFileSync(join(root,'evidence','rehearsal.json'),'utf8'));
 const tests=JSON.parse(readFileSync(join(root,'evidence','local-go-result.json'),'utf8'));
 const instructor=JSON.parse(readFileSync(join(root,'evidence','instructor-authorization.json'),'utf8'));
 const students=JSON.parse(readFileSync(join(root,'evidence','authorization-simulation.json'),'utf8'));
 const release=JSON.parse(readFileSync(join(root,'evidence','release-manifest.json'),'utf8'));
 if(rehearsal.status!=='passed'||!rehearsal.idempotent||!tests.passed)throw new Error('Required rehearsal/test gate is not green.');
 const iv=instructor.verdict;
 if(iv.grade.after<0||iv.grade.after<iv.grade.before||iv.questions.after<iv.questions.before||iv.manage.after<iv.manage.before)throw new Error('Instructor authorization gate is not green.');
 if(students.repairedClone.studentEligiblePairsAfter<students.repairedClone.studentEligiblePairsBefore)throw new Error('Student eligibility gate is not green.');
 if(createHash('sha256').update(readFileSync(backup.full.path)).digest('hex')!==backup.full.sha256)throw new Error('Private backup checksum changed.');
 for(const name of ['0011_practicum_attempt_metadata_repair.sql','0012_practicum_legacy_scope_context.sql']){
  const file=release.files.find(item=>item.path===`backend/migrations/${name}`);
  if(!file||file.sha256!==createHash('sha256').update(readFileSync(`backend/migrations/${name}`)).digest('hex'))throw new Error('Migration differs from tested release artifact.');
 }
 const env=productionEnv();
 const history=psqlJson(env,`select jsonb_agg(jsonb_build_object('name',name,'checksum',checksum) order by name) from public.schema_migrations`);
 const before=psqlJson(env,`set local time zone 'UTC';${workFingerprintSQL()}`);
 if(JSON.stringify(before)!==JSON.stringify(rehearsal.originalFingerprint))throw new Error('Production work changed since rehearsal; held for a fresh backup/rehearsal.');
 const after=psqlJson(env,buildRepairSQL(history),{readOnly:false,label:'Atomic production metadata repair'});
 if(JSON.stringify(before)!==JSON.stringify(after))throw new Error('Unexpected preservation mismatch after commit; investigate immediately.');
 const result={status:'applied',appliedAt:new Date().toISOString(),backupSha256:backup.full.sha256,releaseSha:release.sha256,workUnchanged:true,fingerprints:after};
 writeFileSync(join(root,'evidence','production-repair.json'),JSON.stringify(result,null,2));
 console.log(JSON.stringify({status:'applied',workUnchanged:true,releaseSha:release.sha256,backupSha256:backup.full.sha256},null,2));
}catch(error){console.error(error.message);process.exitCode=1;}
