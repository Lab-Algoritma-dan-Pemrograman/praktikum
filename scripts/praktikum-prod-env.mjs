import { readFileSync, appendFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const productionProject = 'zngmetvdvsfowerwngma';
export function loadPrivateEnv(path = '.env.production.local') {
  const values = {};
  for (const line of readFileSync(resolve(path), 'utf8').split(/\r?\n/)) {
    const match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/);
    if (!match) continue;
    let value = match[2];
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) value = value.slice(1, -1);
    values[match[1]] = value;
  }
  return values;
}
export function productionEnv(path) {
  const env = loadPrivateEnv(path);
  const supabase = new URL(env.SUPABASE_URL);
  if (supabase.protocol !== 'https:' || supabase.hostname !== `${productionProject}.supabase.co`) throw new Error('Production Supabase target does not match the recorded project; stopped.');
  const raw = env.ALGOHUB_MIGRATION_DATABASE_URL || env.ALGOHUB_DATABASE_URL;
  const db = new URL(raw);
  if (!['postgres:', 'postgresql:'].includes(db.protocol)) throw new Error('Invalid database scheme.');
  if (db.hostname !== `db.${productionProject}.supabase.co` && !decodeURIComponent(db.username).includes(productionProject)) throw new Error('Database target cannot be bound to the expected production project; stopped.');
  if (db.hostname.endsWith('.pooler.supabase.com') && db.port === '6543') throw new Error('Migration/backup requires a direct or session-pooler connection, not transaction pooling.');
  return env;
}
export function databaseEnvironment(raw, readOnly = true) {
  const db = new URL(raw);
  return {
    ...process.env,
    PGHOST: db.hostname,
    PGPORT: db.port || '5432',
    PGUSER: decodeURIComponent(db.username),
    PGPASSWORD: decodeURIComponent(db.password),
    PGDATABASE: decodeURIComponent(db.pathname.slice(1)),
    PGSSLMODE: db.searchParams.get('sslmode') || 'require',
    PGCONNECT_TIMEOUT: '10',
    PGAPPNAME: 'praktikum-production-preflight',
    PGOPTIONS: `-c statement_timeout=30000 -c lock_timeout=3000${readOnly ? ' -c default_transaction_read_only=on' : ''}`
  };
}
export function psqlJson(env, sql, { readOnly = true, label = 'SQL preflight' } = {}) {
  const database = env.ALGOHUB_MIGRATION_DATABASE_URL || env.ALGOHUB_DATABASE_URL;
  const result = spawnSync('psql', ['-X', '--no-password', '-qAt', '-v', 'ON_ERROR_STOP=1', '-c', `begin${readOnly ? ' read only' : ''}; ${sql}; commit;`], {
    env: databaseEnvironment(database, readOnly), encoding: 'utf8', timeout: 45000, maxBuffer: 8 * 1024 * 1024
  });
  if (result.error || result.status !== 0) {
    if (process.env.PRAKTIKUM_PG_STDERR_LOG) appendFileSync(process.env.PRAKTIKUM_PG_STDERR_LOG, `\n--- ${label} (exit ${result.status}) ---\n${result.stderr || result.error?.message || ''}`, { mode: 0o600 });
    throw new Error(`${label} failed; connection credentials and raw database diagnostics withheld.`);
  }
  return JSON.parse(result.stdout.trim().split(/\r?\n/).filter((line) => line.trim()).at(-1));
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const env = productionEnv(process.argv[2]);
    const db = new URL(env.ALGOHUB_MIGRATION_DATABASE_URL || env.ALGOHUB_DATABASE_URL);
    console.log(JSON.stringify({ project: productionProject, databaseHost: db.hostname, databasePort: db.port || '5432', configuredKeys: Object.keys(env).sort(), practicumFlag: env.ALGOHUB_PRACTICUM_ENABLED === 'true', serviceRoleConfigured: !!env.SUPABASE_SERVICE_ROLE_KEY }, null, 2));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
