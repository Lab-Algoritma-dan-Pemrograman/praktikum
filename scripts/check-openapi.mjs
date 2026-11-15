import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer as createHTTPServer } from 'node:http';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import Ajv from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import { checkPracticumContract } from './check-practicum-contract.mjs';
import { checkLearningContract } from './check-learning-contract.mjs';
import { checkSiakadContract } from './check-siakad-contract.mjs';

const document = JSON.parse(
  await readFile('docs/openapi/openapi.json', 'utf8')
);
assert.equal(document.openapi, '3.1.0');
const methods = new Set(['get', 'post', 'put', 'patch', 'delete']);
const operations = new Map();
const ids = new Set();
const pointer = (reference) => {
  assert.ok(
    reference.startsWith('#/'),
    `Only local refs are allowed: ${reference}`
  );
  return reference
    .slice(2)
    .split('/')
    .reduce((value, key) => {
      const result = value?.[key.replace(/~1/g, '/').replace(/~0/g, '~')];
      assert.notEqual(result, undefined, `Unresolved reference ${reference}`);
      return result;
    }, document);
};
const resolve = (value) => (value.$ref ? resolve(pointer(value.$ref)) : value);
function checkReferences(value) {
  if (!value || typeof value !== 'object') return;
  if (value.$ref) pointer(value.$ref);
  for (const item of Object.values(value)) checkReferences(item);
}
checkReferences(document);
const ajv = new Ajv({ strict: false, allErrors: true });
addFormats(ajv);
// JSON Schema maxLength counts characters; Go validates UTF-8 byte budgets.
ajv.addKeyword({
  keyword: 'x-max-bytes',
  type: 'string',
  schemaType: 'number',
  validate: (limit, value) => Buffer.byteLength(value, 'utf8') <= limit
});
ajv.addSchema(document, 'urn:algohub:contract');
const schemaValidators = new WeakMap();
function validator(schema) {
  if (!schemaValidators.has(schema)) {
    const absoluteRefs = (value) => {
      if (Array.isArray(value)) return value.map(absoluteRefs);
      if (!value || typeof value !== 'object') return value;
      return Object.fromEntries(
        Object.entries(value).map(([key, item]) => [
          key,
          key === '$ref' ? `urn:algohub:contract${item}` : absoluteRefs(item)
        ])
      );
    };
    schemaValidators.set(schema, ajv.compile(absoluteRefs(schema)));
  }
  return schemaValidators.get(schema);
}
for (const name of Object.keys(document.components.schemas)) {
  ajv.getSchema(`urn:algohub:contract#/components/schemas/${name}`);
}
for (const [path, item] of Object.entries(document.paths)) {
  for (const [method, operation] of Object.entries(item)) {
    if (!methods.has(method)) continue;
    assert.ok(
      operation.operationId && !ids.has(operation.operationId),
      'Missing or duplicate operationId'
    );
    ids.add(operation.operationId);
    assert.ok(
      Object.keys(operation.responses).some((status) => /^2\d\d$/.test(status)),
      `Missing success contract: ${path}`
    );
    for (const [status, response] of Object.entries(operation.responses)) {
      const schema = resolve(response).content?.['application/json']?.schema;
      assert.ok(schema?.$ref, `Untyped ${status} response: ${method} ${path}`);
      validator(schema);
    }
    if (operation.requestBody) {
      const schema = resolve(operation.requestBody).content['application/json']
        .schema;
      validator(schema);
    }
    const security = operation.security ?? document.security;
    assert.ok(Array.isArray(security), `Missing security declaration: ${path}`);
    const parameters = [
      ...(item.parameters ?? []),
      ...(operation.parameters ?? [])
    ].map(resolve);
    for (const parameter of parameters) validator(parameter.schema);
    for (const [, name] of path.matchAll(/\{([^}]+)\}/g)) {
      assert.ok(
        parameters.some(
          (p) => p.in === 'path' && p.name === name && p.required
        ),
        `Missing path parameter: ${path}`
      );
    }
    const mutation = ['post', 'put', 'patch', 'delete'].includes(method);
    if (
      security.some((requirement) => 'SessionCookie' in requirement) &&
      mutation
    ) {
      assert.ok(
        parameters.some((p) => p.name === 'X-CSRF-Token' && p.required),
        `Missing CSRF contract: ${path}`
      );
      if (
        operation.operationId.startsWith('practicum.') ||
        operation.operationId.startsWith('learning.') ||
        operation.operationId.startsWith('academic.') ||
        operation.operationId.startsWith('inventory.')
      ) {
        assert.ok(
          parameters.some((p) => p.name === 'Idempotency-Key' && p.required),
          `Missing idempotency contract: ${path}`
        );
        assert.ok(
          operation.requestBody?.required,
          `Missing request contract: ${path}`
        );
      }
    }
    if (operation.operationId === 'practicum.internal.reconcile') {
      assert.deepEqual(security, [{ PracticumCronBearer: [] }]);
      assert.ok(
        !parameters.some((p) =>
          ['Origin', 'X-CSRF-Token', 'Idempotency-Key'].includes(p.name)
        )
      );
    }
    operations.set(`${method} ${path}`, operation);
  }
}
// A removed/renamed operation must not silently shrink live coverage.
const phase2IDs = [
  'getHealth',
  'getReadiness',
  'auth.password.login',
  'auth.callback',
  'auth.sso.prepare',
  'auth.sso.exchange',
  'auth.me',
  'auth.refresh',
  'auth.logout',
  'auth.sessions.list',
  'auth.sessions.revoke',
  'identity.profile',
  'identity.profile.update',
  'admin.access',
  'admin.overview',
  'admin.permissions',
  'admin.auditlog',
  'admin.permission-catalog',
  'admin.users.list',
  'admin.users.access',
  'admin.users.access.update'
];
for (const id of phase2IDs)
  assert.ok(ids.has(id), `Phase 2 operationId changed: ${id}`);
for (const file of [
  'backend/internal/delivery/http/practicum_handlers.go',
  'backend/internal/delivery/http/learning_handlers.go',
  'backend/internal/delivery/http/academic_handlers.go'
]) {
  const handlers = await readFile(file, 'utf8');
  for (const [, method, path, id, status] of handlers.matchAll(
    /route\("(GET|POST|PATCH|PUT)", "([^"]+)", "([^"]+)", (\d+)/g
  )) {
    const operation = operations.get(`${method.toLowerCase()} ${path}`);
    assert.equal(
      operation?.operationId,
      id,
      `Missing/drifted domain route: ${method} ${path}`
    );
    assert.ok(operation.responses[status], `Wrong success status for ${id}`);
  }
}
assert.equal([...ids].filter((id) => id.startsWith('practicum.')).length, 28);
assert.equal([...ids].filter((id) => id.startsWith('learning.')).length, 21);
assert.equal(
  [...ids].filter(
    (id) => id.startsWith('academic.') || id.startsWith('inventory.')
  ).length,
  22
);

const temp = await mkdtemp(join(tmpdir(), 'algohub-contract-'));
let child;
let previewProvider;
let previewRequest = null;
try {
  const binary = join(temp, process.platform === 'win32' ? 'api.exe' : 'api');
  const build = spawnSync('go', ['build', '-o', binary, './backend/cmd/api'], {
    encoding: 'utf8'
  });
  assert.equal(
    build.status,
    0,
    build.stderr || 'Backend contract server failed to build'
  );
  const reservation = createServer();
  reservation.listen(0, '127.0.0.1');
  await once(reservation, 'listening');
  const port = reservation.address().port;
  await new Promise((resolve) => reservation.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  previewProvider = createHTTPServer((request, response) => {
    let body = '';
    request.on('data', (chunk) => (body += chunk));
    request.on('end', () => {
      previewRequest = {
        url: request.url,
        authorization: request.headers.authorization,
        body
      };
      response
        .writeHead(200, { 'Content-Type': 'application/json' })
        .end(
          JSON.stringify({
            duration: 1,
            stdout: 'ok\n',
            stderr: '',
            error: ''
          })
        );
    });
  });
  previewProvider.listen(0, '127.0.0.1');
  await once(previewProvider, 'listening');
  const previewPort = previewProvider.address().port;
  const authOrigin = 'https://auth.algohub.web.id';
  const adminOrigin = 'https://admin.algohub.web.id';
  const cronSecret = 'contract-fixture-practicum-cron-secret-32';
  child = spawn(binary, [], {
    env: {
      ...process.env,
      VERCEL: '',
      ALGOHUB_ENV: 'local',
      ALGOHUB_ENABLE_FIXTURES: 'true',
      ALGOHUB_PRACTICUM_ENABLED: 'true',
      ALGOHUB_ELEARNING_ENABLED: 'true',
      ALGOHUB_SIAKAD_ENABLED: 'true',
      ALGOHUB_PRACTICUM_CRON_SECRET: cronSecret,
      ALGOHUB_ADDR: `127.0.0.1:${port}`,
      ALGOHUB_COOKIE_SECURE: 'false',
      ALGOHUB_DATABASE_URL: '',
      SUPABASE_URL: '',
      SUPABASE_ANON_KEY: '',
      ALGOHUB_SESSION_TTL: '12h',
      ALGOHUB_REQUEST_TIMEOUT: '10s',
      ALGOHUB_DB_MAX_CONNS: '4',
      ALGOHUB_AUTH_RATE_LIMIT: '2000',
      ALGOHUB_USER_RATE_LIMIT: '4000',
      ALGOHUB_ALLOWED_ORIGINS: `${authOrigin},${adminOrigin}`,
      ALGOHUB_SSO_ALLOWED_ORIGINS: `${authOrigin},${adminOrigin}`,
      ALGOHUB_PLAYGROUND_GLOT_URL: `http://127.0.0.1:${previewPort}/api/mux`,
      ALGOHUB_PLAYGROUND_GLOT_TOKEN: 'contract-preview-token'
    },
    stdio: ['ignore', 'ignore', 'pipe']
  });
  let stderr = '';
  child.stderr.on('data', (chunk) => {
    stderr = (stderr + chunk).slice(-4000);
  });
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(`${base}/healthz`)).ok) break;
    } catch {}
    assert.ok(
      attempt < 100 && child.exitCode === null,
      stderr || 'Contract API did not become ready'
    );
    await delay(100);
  }

  const covered = new Set();
  let assertions = 0;
  async function call(method, template, options = {}) {
    const operation = operations.get(`${method} ${template}`);
    assert.ok(operation, `Undocumented operation: ${method} ${template}`);
    const cookies = options.cookies ?? [];
    const headers = {
      Origin: options.origin ?? authOrigin,
      'Content-Type': 'application/json'
    };
    if (cookies.length) {
      headers.Cookie = cookies
        .map((cookie) => cookie.split(';', 1)[0])
        .join('; ');
      const csrf = cookies.find((cookie) => cookie.startsWith('algohub_csrf='));
      if (csrf)
        headers['X-CSRF-Token'] = csrf
          .split(';', 1)[0]
          .slice('algohub_csrf='.length);
    }
    for (const [name, value] of Object.entries(options.headers ?? {})) {
      if (value === null) delete headers[name];
      else headers[name] = value;
    }
    if (options.origin === null) delete headers.Origin;
    const target = new URL(`${base}${options.path ?? template}`);
    const names = [...template.matchAll(/\{([^}]+)\}/g)].map(
      (match) => match[1]
    );
    const pattern = template
      .split(/\{[^}]+\}/)
      .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
      .join('([^/]+)');
    const pathValues = target.pathname.match(new RegExp(`^${pattern}$`));
    assert.ok(pathValues, `Path does not match ${template}`);
    const pathParams = Object.fromEntries(
      names.map((name, i) => [name, decodeURIComponent(pathValues[i + 1])])
    );
    const parameters = [
      ...(document.paths[template].parameters ?? []),
      ...(operation.parameters ?? [])
    ].map(resolve);
    const requestHeaders = new Headers(headers);
    let parametersValid = true;
    const parameterErrors = [];
    for (const p of parameters) {
      let value =
        p.in === 'path'
          ? pathParams[p.name]
          : p.in === 'query'
            ? target.searchParams.get(p.name)
            : requestHeaders.get(p.name);
      if (value === null || value === undefined) {
        if (p.required) {
          parametersValid = false;
          parameterErrors.push(`${p.name} is required`);
        }
        continue;
      }
      if (p.schema.type === 'integer' && /^-?\d+$/.test(value))
        value = Number(value);
      const validate = validator(p.schema);
      if (!validate(value)) {
        parametersValid = false;
        parameterErrors.push(`${p.name}: ${ajv.errorsText(validate.errors)}`);
      }
    }
    assert.equal(
      parametersValid,
      options.parametersValid ?? true,
      `${operation.operationId} request parameters: ${parameterErrors.join('; ')}`
    );
    const rawBody =
      options.rawBody ??
      (options.body === undefined ? undefined : JSON.stringify(options.body));
    if (operation.requestBody) {
      const request = resolve(operation.requestBody);
      const validate = validator(request.content['application/json'].schema);
      let data;
      try {
        data = JSON.parse(rawBody);
      } catch {}
      const valid =
        validate(data) &&
        (!request['x-max-bytes'] ||
          Buffer.byteLength(rawBody) <= request['x-max-bytes']);
      assert.equal(
        Boolean(valid),
        options.requestValid ?? true,
        `${operation.operationId} request body: ${ajv.errorsText(validate.errors)}`
      );
    } else {
      assert.equal(
        rawBody,
        undefined,
        `Undocumented request body: ${operation.operationId}`
      );
    }
    const response = await fetch(target, {
      method: (options.wireMethod ?? method).toUpperCase(),
      headers,
      body: rawBody
    });
    const body = await response.json();
    const expectedStatuses = options.statuses ?? [options.status ?? 200];
    assert.ok(
      expectedStatuses.includes(response.status),
      `${operation.operationId}: unexpected status ${response.status} (expected ${expectedStatuses.join('/')}); ${JSON.stringify(body)}`
    );
    assert.ok(
      operation.responses[String(response.status)],
      `Undocumented status: ${operation.operationId} ${response.status}`
    );
    const responseSchema = resolve(operation.responses[String(response.status)])
      .content['application/json'].schema;
    const validate = validator(responseSchema);
    assert.ok(
      validate(body),
      `${operation.operationId}: ${ajv.errorsText(validate.errors)}`
    );
    assert.match(
      response.headers.get('content-type') ?? '',
      /application\/json/
    );
    const requestID = response.headers.get('x-request-id');
    assert.ok(requestID);
    if (response.ok) {
      assert.equal(body.meta.operation_id, operation.operationId);
      assert.equal(body.meta.request_id, requestID);
      covered.add(`${method} ${template}`);
    } else {
      assert.equal(body.error.request_id, requestID);
      if (options.errorCode) assert.equal(body.error.code, options.errorCode);
    }
    if (template.startsWith('/v1/'))
      assert.equal(response.headers.get('cache-control'), 'private, no-store');
    assertions++;
    return {
      body,
      cookies: response.headers.getSetCookie(),
      headers: response.headers
    };
  }

  await call('get', '/healthz');
  await call('get', '/readyz');
  await call('get', '/v1/auth/me', { status: 401 });
  let admin = (
    await call('post', '/v1/auth/password', {
      body: { email: 'koordinator@example.test', password: 'fixture-password' }
    })
  ).cookies;
  const verifier = '1234567890abcdefghijklmnopqrstuvwxyzABCDEFG';
  const student = (
    await call('post', '/v1/auth/callback', {
      body: { code: 'fixture-student', code_verifier: verifier }
    })
  ).cookies;
  await call('get', '/v1/admin/overview', { cookies: student, status: 403 });
  const playgroundPreview = await call('post', '/v1/playground/run', {
    cookies: student,
    body: { language: 'c', source: 'int main(void){return 0;}', stdin: '' }
  });
  assert.equal(playgroundPreview.body.data.provider, 'glot');
  assert.equal(playgroundPreview.body.data.authoritative, false);
  assert.equal(previewRequest?.url, '/api/mux');
  assert.equal(previewRequest?.authorization, 'Token contract-preview-token');
  assert.deepEqual(JSON.parse(previewRequest?.body ?? '{}'), {
    action: 'run',
    data: {
      image: 'glot/clang:latest',
      payload: {
        runInstructions: {
          buildCommands: ['clang -o a.out -lm main.c'],
          runCommand: './a.out'
        },
        files: [{ name: 'main.c', content: 'int main(void){return 0;}' }],
        stdin: ''
      }
    }
  });
  await call('get', '/v1/auth/me', { cookies: admin });
  await call('get', '/v1/me', { cookies: admin });
  await call('patch', '/v1/me', {
    cookies: admin,
    body: { name: 'Coordinator Fixture' }
  });
  await call('patch', '/v1/me', {
    cookies: admin,
    body: { name: 'Name', role: 'admin' },
    requestValid: false,
    status: 422
  });
  await call('get', '/v1/auth/sessions', { cookies: admin });
  admin = (await call('post', '/v1/auth/refresh', { cookies: admin })).cookies;
  const challenge = createHash('sha256').update(verifier).digest('base64url');
  const handshake = await call('post', '/v1/auth/sso/prepare', {
    cookies: admin,
    body: { return_to: `${adminOrigin}/`, code_challenge: challenge }
  });
  const exchange = {
    code: handshake.body.data.code,
    return_to: `${adminOrigin}/`,
    code_verifier: verifier
  };
  const app = (
    await call('post', '/v1/auth/sso/exchange', {
      origin: adminOrigin,
      body: exchange
    })
  ).cookies;
  await call('post', '/v1/auth/sso/exchange', {
    origin: adminOrigin,
    body: exchange,
    status: 401
  });
  for (const path of [
    '/v1/admin/access',
    '/v1/admin/overview',
    '/v1/admin/permissions',
    '/v1/admin/permission-catalog',
    '/v1/admin/users'
  ]) {
    await call('get', path, { cookies: app, origin: adminOrigin });
  }
  const access = await call('get', '/v1/admin/users/{id}/access', {
    path: '/v1/admin/users/1/access',
    cookies: app,
    origin: adminOrigin
  });
  const update = {
    version: access.body.data.version,
    assignments: [{ role: 'mahasiswa', scope: {} }],
    grants: []
  };
  await call('put', '/v1/admin/users/{id}/access', {
    path: '/v1/admin/users/1/access',
    cookies: app,
    origin: adminOrigin,
    body: update
  });
  await call('put', '/v1/admin/users/{id}/access', {
    path: '/v1/admin/users/1/access',
    cookies: app,
    origin: adminOrigin,
    body: update,
    status: 409
  });
  await call('get', '/v1/admin/users/{id}/access', {
    path: '/v1/admin/users/99999/access',
    cookies: app,
    origin: adminOrigin,
    status: 404
  });
  await call('get', '/v1/admin/auditlog', {
    cookies: app,
    origin: adminOrigin
  });
  await checkPracticumContract({
    call,
    student,
    admin: app,
    authOrigin,
    adminOrigin,
    cronSecret
  });
  await checkLearningContract({
    call,
    student,
    admin: app,
    authOrigin,
    adminOrigin
  });
  await checkSiakadContract({
    call,
    student,
    admin: app,
    authOrigin,
    adminOrigin
  });
  const sessions = await call('get', '/v1/auth/sessions', { cookies: student });
  await call('delete', '/v1/auth/sessions/{id}', {
    path: `/v1/auth/sessions/${sessions.body.data[0].id}`,
    cookies: student
  });
  await call('post', '/v1/auth/logout', { cookies: app, origin: adminOrigin });
  assert.equal(
    covered.size,
    operations.size,
    `Untested operations: ${[...operations.keys()].filter((key) => !covered.has(key)).join(', ')}`
  );
  console.log(
    `OpenAPI checks passed: ${operations.size} operations, ${assertions} live response contracts.`
  );
} finally {
  if (child && child.exitCode === null) {
    const stopped = once(child, 'exit');
    child.kill();
    await stopped;
  }
  previewProvider?.close();
  await rm(temp, {
    recursive: true,
    force: true,
    maxRetries: 5,
    retryDelay: 100
  });
}
