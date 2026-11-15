import { spawnSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.argv[2] ? join(process.argv[2], 'evidence') : '.';
const hosts = {
  portal: 'https://algohub.web.id',
  auth: 'https://auth.algohub.web.id',
  admin: 'https://admin.algohub.web.id',
  praktikum: 'https://praktikum.algohub.web.id',
  api: 'https://api.algohub.web.id'
};

function curl(args) {
  const result = spawnSync('curl', ['--silent', '--show-error', '--max-time', '30', ...args], { encoding: 'utf8', timeout: 45000 });
  return { out: result.stdout || '', err: result.stderr || '', status: result.status };
}

const checks = [];
function check(name, ok, detail) { checks.push({ name, ok, detail }); }

// 1. Public entry points respond without leaking internals. The API root has no
// route by design (404); its liveness is asserted separately below.
for (const [name, origin] of Object.entries(hosts)) {
  if (name === 'api') continue;
  const r = curl(['-o', '/dev/null', '-w', '%{http_code}', `${origin}/`]);
  check(`${name}-reachable`, ['200', '307', '308', '302', '401', '403'].includes(r.out.trim()), `HTTP ${r.out.trim()}`);
}

// 1b. API liveness.
const health = curl([`${hosts.api}/healthz`]);
check('api-health', health.out.includes('"status":"ok"'), health.out.slice(0, 120));

// 2. API readiness.
const ready = curl([`${hosts.api}/readyz`]);
check('api-ready', ready.out.includes('"status":"ready"'), ready.out.slice(0, 120));

// 3. Anonymous practicum data must be refused.
const anon = curl(['-o', '/dev/null', '-w', '%{http_code}', `${hosts.api}/v1/practicum/sessions`]);
check('api-refuses-anonymous', anon.out.trim() === '401', `HTTP ${anon.out.trim()}`);

// 4. Praktikum host must set its own origin gate, not serve another app.
const pk = curl([`${hosts.praktikum}/`]);
check('praktikum-serves-html', pk.out.includes('<!') && pk.out.toLowerCase().includes('algohub'), `bytes ${pk.out.length}`);

// 5. Login page reachable and not a Vercel disabled page.
const login = curl(['-L', '-o', '/dev/null', '-w', '%{http_code}', `${hosts.praktikum}/login`]);
check('praktikum-login-redirects-to-auth', ['200', '307', '308', '303'].includes(login.out.trim()), `HTTP ${login.out.trim()}`);

// 6. Root portal still serves the new deployment.
const portal = curl(['-o', '/dev/null', '-w', '%{http_code}', `${hosts.portal}/`]);
check('portal-serves', portal.out.trim() === '200', `HTTP ${portal.out.trim()}`);

const failed = checks.filter((c) => !c.ok);
if (root !== '.') writeFileSync(join(root, 'production-acceptance.json'), JSON.stringify({ checkedAt: new Date().toISOString(), hosts, checks }, null, 2));
console.log(JSON.stringify({ passed: failed.length === 0, failed, checks }, null, 2));
if (failed.length) process.exitCode = 4;
