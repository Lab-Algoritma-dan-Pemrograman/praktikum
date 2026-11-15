import { readFileSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join, resolve } from 'node:path';
const root=resolve(process.argv[2]);
const connection=JSON.parse(readFileSync(join(root,'local-pg','connection.json'),'utf8'));
const safe=Object.fromEntries(Object.entries(process.env).filter(([key])=>! /^(PG|ALGOHUB_|SUPABASE_)/i.test(key)));
const env={...safe,ALGOHUB_TEST_DATABASE_URL:connection.unitUrl};
try{
 const run=spawnSync('go',['-C','backend','test','-count=1','-timeout=180s','./...'],{env,encoding:'utf8',timeout:240000,maxBuffer:8*1024*1024});
 const output=`${run.stdout||''}\n${run.stderr||''}`;
 writeFileSync(join(root,'evidence','local-go-tests.log'),output,{mode:0o600});
 if(run.error||run.status!==0)throw new Error('Go/SQL integration failed; private log retained.');
 writeFileSync(join(root,'evidence','local-go-result.json'),JSON.stringify({passed:true,database:'private-loopback-synthetic-unit',productionWritten:false},null,2));
 console.log(JSON.stringify({passed:true,productionWritten:false,postgresIntegrationEnabled:true},null,2));
}catch(error){console.error(error.message);process.exitCode=1;}
