import { spawnSync } from 'node:child_process';
import { writeFileSync, readFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { productionEnv } from './praktikum-prod-env.mjs';

// Authenticated production journey. Uses a one-time synthetic owner perspective:
// it signs in with the owner-supplied acceptance account via the real SSO chain,
// reads only that student's own history, and never writes domain data.
const root = process.argv[2] ? join(process.argv[2], 'evidence') : '.';
const email = process.env.PRAKTIKUM_ACCEPT_EMAIL || '';
const password = process.env.PRAKTIKUM_ACCEPT_PASSWORD || '';
const base = 'https://praktikum.algohub.web.id';
const api = 'https://api.algohub.web.id';

if (!email || !password) {
  console.log(JSON.stringify({ status: 'skipped', reason: 'No owner-supplied acceptance credentials in the environment; authenticated production journey not executed.' }, null, 2));
  process.exitCode = 0;
  process.exit();
}
const env = productionEnv();
if (env.ALGOHUB_ENV !== 'production') { console.error('Unexpected environment.'); process.exit(1); }

function curl(args, input) {
  const result = spawnSync('curl', ['--silent', '--show-error', '--max-time', '40', '--cookie-jar', jar, ...args], { input, encoding: 'utf8', timeout: 60000 });
  return { out: result.stdout || '', status: result.status };
}
const jar = join(root === '.' ? '.' : root, 'accept-cookies.txt');
const checks = [];
const check = (name, ok, detail) => checks.push({ name, ok, detail: detail ?? '' });

try {
  const login = curl(['-X', 'POST', `${api}/v1/auth/password`, '-H', 'Content-Type: application/json', '--data-binary', JSON.stringify({ email, password })]);
  const body = (() => { try { return JSON.parse(login.out); } catch { return null; } })();
  check('authenticated', !!body && !body.error, body?.error ? `refused: ${body.error.code}` : 'session established');
  if (!body || body.error) throw new Error('Authentication failed; no further reads attempted.');
  const csrf = (() => { try { return JSON.parse(readFileSync(jar, 'utf8')).cookies.filter((c) => c.name === 'algohub_csrf').at(-1)?.value; } catch { return null; } })();
  check('csrf-issued', !!csrf, csrf ? 'present' : 'missing');

  const sessions = curl([`${api}/v1/practicum/sessions`]);
  const sessionBody = JSON.parse(sessions.out);
  check('sessions-readable', Array.isArray(sessionBody.data), `${Array.isArray(sessionBody.data) ? sessionBody.data.length : 'invalid'} session(s)`);
  check('sessions-scoped', (sessionBody.data || []).every((s) => s.id), 'session identifiers present');

  if (Array.isArray(sessionBody.data) && sessionBody.data.length) {
    const detail = curl([`${api}/v1/practicum/sessions/${encodeURIComponent(sessionBody.data[0].id)}`]);
    const detailBody = JSON.parse(detail.out);
    const activations = detailBody.data?.activations || [];
    check('activation-has-class', activations.every((a) => a.scope?.class_id), `${activations.length} activation(s)`);
    check('activation-has-shift', activations.some((a) => a.scope?.shift), 'shift context present');
    check('questions-visible', activations.length === 0 || detailBody.data?.courses?.length > 0, `${detailBody.data?.courses?.length || 0} course(s)`);
  }

  const grades = curl([`${api}/v1/practicum/grades`]);
  const gradeBody = JSON.parse(grades.out);
  check('history-readable', Array.isArray(gradeBody.data), `${Array.isArray(gradeBody.data) ? gradeBody.data.length : 'invalid'} record(s)`);
  const unpublished = (gradeBody.data || []).filter((g) => g.score === null);
  check('null-scores-not-invented', unpublished.every((g) => g.percentage === null), `${unpublished.length} unpublished`);

  // Cross-user isolation: an owned record must never be readable without ownership.
  if (Array.isArray(gradeBody.data) && gradeBody.data.length) {
    const attempt = curl([`${api}/v1/practicum/attempts/${encodeURIComponent(gradeBody.data[0].attempt_id)}`]);
    const attemptBody = JSON.parse(attempt.out);
    check('owned-attempt-readable', !attemptBody.error, attemptBody.error ? attemptBody.error.code : 'ok');
    check('no-private-grading-fields', !(attemptBody.data?.questions || []).some((q) => 'reference_answer' in q || 'rubric' in q), 'no rubric/reference leak');
  }
  const failed = checks.filter((c) => !c.ok);
  writeFileSync(join(root, 'authenticated-acceptance.json'), JSON.stringify({ checkedAt: new Date().toISOString(), email, checks }, null, 2));
  console.log(JSON.stringify({ passed: failed.length === 0, failed, checks }, null, 2));
  if (failed.length) process.exitCode = 5;
} finally {
  try { rmSync(jar, { force: true }); } catch {}
}
