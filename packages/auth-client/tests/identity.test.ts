import assert from 'node:assert/strict';
import { afterEach, beforeEach, mock, test } from 'node:test';
import { createHash } from 'node:crypto';
import { isHttpError, isRedirect, type Cookies } from '@sveltejs/kit';
import {
  AuthClient,
  ApiError,
  adminNavigation,
  canAccessAdminRoute,
  csrfToken,
  safeRelativePath,
  safeReturnTo,
  scopeContains,
  type Principal
} from '../src/index.ts';
import {
  applySessionCookies,
  BODY_LIMIT,
  applicationLinks,
  centralLoginLoad,
  completeAppSSO,
  completeCentralCallback,
  identityConfig,
  identityRequest,
  passwordLogin,
  pkce,
  proxyIdentity,
  readLimitedBody,
  startAppSSO,
  startProviderLogin
} from '../src/server.server.ts';

const origins = [
  'portal',
  'auth',
  'admin',
  'siakad',
  'elearning',
  'praktikum'
].map((app, index) => `http://${app}.localhost:${5173 + index}`);
const localEnv = {
  ALGOHUB_ENV: 'local',
  ALGOHUB_ENABLE_FIXTURES: 'true',
  ALGOHUB_DATABASE_URL: '',
  ALGOHUB_API_BASE_URL: 'http://127.0.0.1:8080',
  ALGOHUB_AUTH_ORIGIN: origins[1],
  ALGOHUB_SSO_ALLOWED_ORIGINS: origins.join(','),
  ALGOHUB_ALLOWED_ORIGINS: origins.join(','),
  SUPABASE_URL: '',
  SUPABASE_SSO_PROVIDER: ''
};
let original: Record<string, string | undefined>;
beforeEach(() => {
  original = Object.fromEntries(
    Object.keys(localEnv).map((key) => [key, process.env[key]])
  );
  Object.assign(process.env, localEnv);
  mock.method(globalThis, 'fetch', async () => {
    throw new Error('Unexpected upstream call');
  });
});
afterEach(() => {
  mock.restoreAll();
  for (const [key, value] of Object.entries(original)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

function event(
  path = '/api/v1/auth/me',
  options: {
    origin?: string;
    method?: string;
    headers?: Record<string, string>;
    body?: string;
    jar?: Map<string, string>;
  } = {}
) {
  const url = new URL(path, options.origin ?? origins[2]);
  const jar = options.jar ?? new Map<string, string>();
  const writes: Array<{
    name: string;
    value: string;
    options: Record<string, unknown>;
  }> = [];
  const responseHeaders: Record<string, string> = {};
  const cookies = {
    get: (name: string) => jar.get(name),
    set: (name: string, value: string, options: Record<string, unknown>) => {
      jar.set(name, value);
      writes.push({ name, value, options });
    },
    delete: (name: string, options: Record<string, unknown>) => {
      jar.delete(name);
      writes.push({ name, value: '', options: { ...options, maxAge: 0 } });
    }
  } as unknown as Cookies;
  return {
    url,
    request: new Request(url, {
      method: options.method ?? 'GET',
      headers: options.headers,
      body: options.body
    }),
    cookies,
    setHeaders: (headers: Record<string, string>) =>
      Object.assign(responseHeaders, headers),
    jar,
    writes,
    responseHeaders
  };
}
function envelope(
  data: unknown,
  options: { status?: number; headers?: Headers; next?: number } = {}
) {
  return Response.json(
    {
      data,
      meta: {
        request_id: 'test-request',
        operation_id: 'test.operation',
        ...(options.next !== undefined ? { next_before: options.next } : {})
      }
    },
    { status: options.status, headers: options.headers }
  );
}
function unauthorized() {
  return Response.json(
    {
      error: { code: 'UNAUTHENTICATED', message: 'Sesi tidak valid.' },
      meta: { request_id: 'test-request', operation_id: 'auth.me' }
    },
    { status: 401 }
  );
}
function status(code: number) {
  return (cause: unknown) => isHttpError(cause, code);
}
async function location(action: () => unknown): Promise<string> {
  try {
    await action();
    assert.fail('Expected redirect');
  } catch (cause) {
    if (!isRedirect(cause)) throw cause;
    return cause.location;
  }
}

const principal: Principal = {
  user: {
    id: 9,
    name: 'Koordinator',
    email: 'koordinator@example.test',
    is_active: true,
    access_version: 1
  },
  assignments: [
    { id: 1, user_id: 9, role: 'koordinator', active: true, scope: {} }
  ],
  grants: [
    'admin.overview.read',
    'identity.access.manage',
    'audit.read',
    'practicum.session.manage',
    'learning.monitor',
    'class.manage'
  ].map((permission) => ({
    permission,
    app: 'admin',
    role: 'koordinator',
    scope: {}
  }))
};

test('return paths and origins are explicit allowlists', () => {
  assert.equal(
    safeReturnTo(`${origins[2]}/profil`, origins),
    `${origins[2]}/profil`
  );
  for (const path of [
    '//evil.test',
    '/callback',
    '/api/auth/start',
    '/%2e%2e/',
    '/profil?x=1',
    '/profil#x',
    '/unknown'
  ])
    assert.equal(safeRelativePath(path), null);
  for (const target of [
    'https://evil.test/',
    `${origins[2]}/../`,
    `${origins[2]}/%2F`,
    `${origins[2]}/?token=x`,
    `${origins[2]}/#x`,
    `${origins[2]}/callback`,
    `http://user@admin.localhost:5175/`,
    `${origins[2]}\\evil`,
    `${origins[2]}//evil`
  ])
    assert.equal(safeReturnTo(target, origins), null);
});

test('local HTTP and fixture activation fail closed outside isolated local origins', () => {
  assert.equal(identityConfig(new URL(origins[2]), localEnv).secure, false);
  assert.throws(
    () =>
      identityConfig(new URL(origins[2]), {
        ...localEnv,
        ALGOHUB_ENV: 'production'
      }),
    status(503)
  );
  assert.throws(
    () => identityConfig(new URL('https://admin.algohub.web.id'), localEnv),
    status(503)
  );
  assert.throws(
    () =>
      identityConfig(new URL(origins[2]), {
        ...localEnv,
        ALGOHUB_DATABASE_URL: 'configured'
      }),
    status(503)
  );
  assert.throws(
    () =>
      identityConfig(new URL(origins[2]), {
        ...localEnv,
        ALGOHUB_API_BASE_URL: 'https://external.test'
      }),
    status(503)
  );
  assert.throws(
    () =>
      identityConfig(new URL(origins[2]), {
        ...localEnv,
        ALGOHUB_ENABLE_FIXTURES: 'false',
        ALGOHUB_API_BASE_URL: 'https://external.test'
      }),
    status(503)
  );
  assert.throws(
    () =>
      identityConfig(new URL(origins[2]), {
        ...localEnv,
        ALGOHUB_ENABLE_FIXTURES: 'false',
        SUPABASE_URL: 'https://external.test',
        SUPABASE_SSO_PROVIDER: 'google'
      }),
    status(503)
  );
  assert.throws(
    () => identityConfig(new URL('http://evil.localhost:5175'), localEnv),
    status(503)
  );
  const production = identityConfig(new URL('https://admin.algohub.web.id'), {
    ALGOHUB_API_BASE_URL: 'https://api.algohub.web.id'
  });
  assert.equal(production.secure, true);
  assert.equal(production.fixtures, false);
  const staging = identityConfig(new URL('https://staging.algohub.web.id'), {
    ALGOHUB_ENV: 'staging',
    ALGOHUB_API_BASE_URL: 'https://api.staging.algohub.web.id',
    ALGOHUB_AUTH_ORIGIN: 'https://auth.staging.algohub.web.id',
    ALGOHUB_SSO_ALLOWED_ORIGINS:
      'https://staging.algohub.web.id,https://auth.staging.algohub.web.id,https://admin.staging.algohub.web.id',
    ALGOHUB_ALLOWED_ORIGINS:
      'https://staging.algohub.web.id,https://auth.staging.algohub.web.id,https://admin.staging.algohub.web.id'
  });
  assert.equal(staging.appOrigin, 'https://staging.algohub.web.id');
  assert.deepEqual(applicationLinks(staging), [
    {
      name: 'portal',
      label: 'Portal',
      origin: 'https://staging.algohub.web.id'
    },
    {
      name: 'auth',
      label: 'Auth Center',
      origin: 'https://auth.staging.algohub.web.id'
    },
    {
      name: 'admin',
      label: 'Admin',
      origin: 'https://admin.staging.algohub.web.id'
    }
  ]);
  assert.throws(
    () =>
      identityConfig(new URL('https://portal.staging.algohub.web.id'), {
        ALGOHUB_ENV: 'staging',
        ALGOHUB_API_BASE_URL: 'https://api.staging.algohub.web.id',
        ALGOHUB_AUTH_ORIGIN: 'https://auth.staging.algohub.web.id',
        ALGOHUB_SSO_ALLOWED_ORIGINS: staging.allowedOrigins.join(','),
        ALGOHUB_ALLOWED_ORIGINS: staging.allowedOrigins.join(',')
      }),
    status(503)
  );
});

test('PKCE challenge uses SHA-256 with a fresh random verifier', () => {
  const first = pkce();
  assert.match(first.verifier, /^[A-Za-z0-9_-]{43}$/);
  assert.equal(
    first.challenge,
    createHash('sha256').update(first.verifier).digest('base64url')
  );
  assert.ok(first.verifier !== pkce().verifier);
});

test('app start stores target and verifier only in host-only HttpOnly pending cookie', async () => {
  const app = event('/api/auth/start?return_to=/profil');
  const target = new URL(await location(() => startAppSSO(app)));
  assert.equal(target.origin, origins[1]);
  assert.equal(target.searchParams.get('return_to'), `${origins[2]}/profil`);
  const pending = JSON.parse(
    Buffer.from(app.jar.get('algohub_sso_pending')!, 'base64url').toString()
  );
  assert.ok(!target.toString().includes(pending.verifier));
  assert.equal(
    target.searchParams.get('code_challenge'),
    createHash('sha256').update(pending.verifier).digest('base64url')
  );
  assert.deepEqual(app.writes[0].options, {
    path: '/callback',
    httpOnly: true,
    sameSite: 'lax',
    secure: false,
    maxAge: 600
  });
  assert.equal(app.responseHeaders['cache-control'], 'no-store');
  assert.throws(
    () => startAppSSO(event('/api/auth/start?return_to=//evil.test')),
    status(422)
  );
});

test('callback requires pending verifier, exact return target and one-use cookie', async () => {
  const app = event('/api/auth/start?return_to=/profil');
  await location(() => startAppSSO(app));
  const saved = JSON.parse(
    Buffer.from(app.jar.get('algohub_sso_pending')!, 'base64url').toString()
  );
  const responseCookies = new Headers();
  responseCookies.append(
    'set-cookie',
    'algohub_session=fresh_session; Path=/; HttpOnly; SameSite=Lax; Max-Age=120'
  );
  responseCookies.append(
    'set-cookie',
    'algohub_csrf=fresh_csrf; Path=/; SameSite=Lax; Max-Age=120'
  );
  const upstream = mock.method(
    globalThis,
    'fetch',
    async (input: string | URL | Request, init?: RequestInit) => {
      assert.equal(String(input), 'http://127.0.0.1:8080/v1/auth/sso/exchange');
      assert.equal(new Headers(init?.headers).get('origin'), origins[2]);
      assert.deepEqual(JSON.parse(init?.body as string), {
        code: 'one_use_code',
        return_to: `${origins[2]}/profil`,
        code_verifier: saved.verifier
      });
      return envelope({ user: principal.user }, { headers: responseCookies });
    }
  );
  const callbackPath = `/callback?${new URLSearchParams({ code: 'one_use_code', return_to: `${origins[2]}/profil` })}`;
  const callback = event(callbackPath, { jar: app.jar });
  assert.equal(
    await location(() => completeAppSSO(callback)),
    `${origins[2]}/profil`
  );
  assert.equal(callback.jar.get('algohub_session'), 'fresh_session');
  assert.equal(callback.jar.has('algohub_sso_pending'), false);
  await assert.rejects(
    () => completeAppSSO(event(callbackPath, { jar: app.jar })),
    status(400)
  );
  assert.equal(upstream.mock.callCount(), 1);
});

test('callback rejects swapped target, duplicate parameters, absent and expired cookies before upstream', async () => {
  for (const target of [`${origins[0]}/`, `${origins[2]}/pengaturan`]) {
    const start = event('/api/auth/start?return_to=/profil');
    await location(() => startAppSSO(start));
    await assert.rejects(
      () =>
        completeAppSSO(
          event(
            `/callback?${new URLSearchParams({ code: 'code', return_to: target })}`,
            { jar: start.jar }
          )
        ),
      status(400)
    );
  }
  const start = event('/api/auth/start');
  await location(() => startAppSSO(start));
  await assert.rejects(
    () =>
      completeAppSSO(
        event(
          `/callback?code=a&code=b&return_to=${encodeURIComponent(`${origins[2]}/`)}`,
          { jar: start.jar }
        )
      ),
    status(400)
  );
  const jar = new Map([
    [
      'algohub_sso_pending',
      Buffer.from(
        JSON.stringify({
          returnTo: `${origins[2]}/`,
          nonce: 'a'.repeat(43),
          verifier: 'b'.repeat(43),
          expiresAt: Date.now() - 1
        })
      ).toString('base64url')
    ]
  ]);
  await assert.rejects(
    () => completeAppSSO(event('/callback?code=expired', { jar })),
    status(400)
  );
  await assert.rejects(
    () => completeAppSSO(event('/callback?code=missing')),
    status(400)
  );
});

test('server request replaces spoofed origin and only forwards approved cookies and metadata', async () => {
  const app = event('/profil', {
    headers: {
      origin: 'https://evil.test',
      authorization: 'Bearer not-forwarded',
      'x-user-role': 'koordinator',
      'x-request-id': 'request-123'
    },
    jar: new Map([
      ['algohub_session', 'opaque_session'],
      ['algohub_csrf', 'csrf_value'],
      ['provider_token', 'not-forwarded']
    ])
  });
  mock.method(
    globalThis,
    'fetch',
    async (_input: unknown, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      assert.equal(headers.get('origin'), origins[2]);
      assert.equal(
        headers.get('cookie'),
        'algohub_session=opaque_session; algohub_csrf=csrf_value'
      );
      assert.equal(headers.get('x-csrf-token'), 'csrf_value');
      assert.equal(headers.get('x-request-id'), 'request-123');
      assert.equal(headers.get('authorization'), null);
      assert.equal(headers.get('x-user-role'), null);
      assert.equal(init?.redirect, 'error');
      assert.ok(init?.signal instanceof AbortSignal);
      return envelope({
        user: { ...principal.user, auth_user_id: 'private-id' },
        access_token: 'not-public',
        nested: { refresh_token: 'not-public', safe: true }
      });
    }
  );
  const body = await (await identityRequest(app, '/v1/auth/me')).json();
  assert.equal(JSON.stringify(body).includes('not-public'), false);
  assert.equal(JSON.stringify(body).includes('private-id'), false);
  assert.deepEqual(body.data.nested, { safe: true });
});

test('BFF preserves rate-limit headers and authoritative error correlation', async () => {
  const app = event('/api/v1/auth/me', {
    headers: { 'x-request-id': 'invalid:request'.repeat(10) }
  });
  mock.method(
    globalThis,
    'fetch',
    async (_input: unknown, init?: RequestInit) => {
      assert.match(
        new Headers(init?.headers).get('x-request-id')!,
        /^[A-Za-z0-9._-]{1,64}$/
      );
      return Response.json(
        {
          error: {
            code: 'RATE_LIMITED',
            message: 'Try later.',
            request_id: 'req_upstream'
          }
        },
        {
          status: 429,
          headers: {
            'retry-after': '60',
            'x-request-id': 'req_upstream',
            'x-operation-id': 'auth.me'
          }
        }
      );
    }
  );
  const response = await proxyIdentity(app, 'admin');
  assert.equal(response.status, 429);
  assert.equal(response.headers.get('retry-after'), '60');
  assert.equal(response.headers.get('x-request-id'), 'req_upstream');
  assert.equal(response.headers.get('x-operation-id'), 'auth.me');
  const body = await response.json();
  assert.equal(body.meta.request_id, 'req_upstream');
  assert.equal(body.meta.operation_id, 'auth.me');
});

test('cookies preserve upstream expiry and force host-only security attributes', () => {
  const app = event('/');
  const config = identityConfig(new URL('https://admin.algohub.web.id'), {
    ALGOHUB_API_BASE_URL: 'https://api.algohub.web.id'
  });
  const headers = new Headers();
  headers.append(
    'set-cookie',
    'algohub_session=opaque; Path=/; Max-Age=42; Expires=Tue, 01 Jan 2030 00:00:00 GMT'
  );
  headers.append('set-cookie', 'algohub_csrf=csrf; Path=/; Max-Age=42');
  headers.append('set-cookie', 'provider_token=secret; Path=/');
  applySessionCookies(app, headers, config);
  assert.equal(app.writes.length, 2);
  assert.equal(app.writes[0].options.httpOnly, true);
  assert.equal(app.writes[1].options.httpOnly, false);
  assert.equal(app.writes[0].options.secure, true);
  assert.equal(app.writes[0].options.sameSite, 'lax');
  assert.equal(app.writes[0].options.maxAge, 42);
  assert.equal(app.writes[0].options.domain, undefined);
  assert.ok(app.writes[0].options.expires instanceof Date);
  assert.throws(
    () =>
      applySessionCookies(
        app,
        new Headers({
          'set-cookie': 'algohub_session=bad; Domain=algohub.web.id; Path=/'
        }),
        config
      ),
    status(502)
  );
  applySessionCookies(
    app,
    new Headers({ 'set-cookie': 'algohub_session=; Path=/; Max-Age=0' }),
    config
  );
  assert.equal(app.writes.at(-1)?.options.maxAge, 0);
});

test('browser proxy is not an arbitrary upstream or provider authentication proxy', async () => {
  for (const path of [
    '/api/v1/healthz',
    '/api/v1/auth/password',
    '/api/v1/auth/callback',
    '/api/v1/auth/sso/exchange',
    '/api/v1/admin/users/1/profile',
    '/api/v1/admin/auditlog?url=https://evil.test'
  ]) {
    const response = await proxyIdentity(event(path), 'admin');
    assert.ok([404, 422].includes(response.status), path);
  }
  assert.equal(
    (await proxyIdentity(event('/api/v1/admin/users'), 'portal')).status,
    404
  );
  assert.equal(
    (await proxyIdentity(event('/api/v1/auth/logout'), 'admin')).status,
    405
  );
});

test('browser proxy rejects CSRF omissions, foreign origins and non-JSON mutations', async () => {
  const path = '/api/v1/me';
  const cases = [
    {
      headers: { 'content-type': 'application/json', 'x-csrf-token': 'csrf' },
      expected: 403
    },
    {
      headers: {
        origin: 'https://evil.test',
        'content-type': 'application/json',
        'x-csrf-token': 'csrf'
      },
      expected: 403
    },
    {
      headers: { origin: origins[2], 'content-type': 'application/json' },
      expected: 403
    },
    {
      headers: {
        origin: origins[2],
        'content-type': 'text/plain',
        'x-csrf-token': 'csrf'
      },
      expected: 415
    }
  ];
  for (const value of cases)
    assert.equal(
      (
        await proxyIdentity(
          event(path, { method: 'PATCH', headers: value.headers, body: '{}' }),
          'admin'
        )
      ).status,
      value.expected
    );
});

test('browser proxy forwards mutations and audit scan cursor without exposing upstream headers', async () => {
  mock.method(
    globalThis,
    'fetch',
    async (_input: unknown, init?: RequestInit) => {
      if (init?.method === 'PATCH')
        assert.deepEqual(JSON.parse(init.body as string), { name: 'Updated' });
      return envelope([], {
        next: 40,
        headers: new Headers({ 'x-provider-secret': 'private' })
      });
    }
  );
  const response = await proxyIdentity(
    event('/api/v1/me', {
      method: 'PATCH',
      headers: {
        origin: origins[2],
        'content-type': 'application/json',
        'x-csrf-token': 'csrf'
      },
      body: '{"name":"Updated"}'
    }),
    'admin'
  );
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('x-provider-secret'), null);
  assert.equal(response.headers.get('cache-control'), 'no-store');
  assert.equal((await response.json()).meta.next_before, 40);
});

test('body limits apply to declared and streamed payloads and slow streams', async () => {
  await assert.rejects(
    () =>
      readLimitedBody(
        new Request(origins[2], {
          method: 'POST',
          headers: { 'content-length': String(BODY_LIMIT + 1) },
          body: '{}'
        })
      ),
    status(413)
  );
  await assert.rejects(
    () => readLimitedBody(new Response('x'.repeat(BODY_LIMIT + 1))),
    status(413)
  );
  let cancelled = false;
  const slow = new Response(
    new ReadableStream({
      cancel() {
        cancelled = true;
      }
    })
  );
  await assert.rejects(() => readLimitedBody(slow, BODY_LIMIT, 5), status(504));
  assert.equal(cancelled, true);
});

test('unavailable or invalid upstream responses are sanitized', async () => {
  const response = await proxyIdentity(event(), 'admin');
  assert.equal(response.status, 502);
  assert.equal((await response.text()).includes('Unexpected upstream'), false);
  mock.method(
    globalThis,
    'fetch',
    async () => new Response('<html>provider secret</html>', { status: 500 })
  );
  assert.equal((await proxyIdentity(event(), 'admin')).status, 502);
});

test('upstream error envelopes expose only approved fields', async () => {
  mock.method(globalThis, 'fetch', async () =>
    Response.json(
      {
        error: {
          code: 'FORBIDDEN',
          message: 'Tidak diizinkan.',
          provider_token: 'must-not-leak'
        },
        meta: { request_id: 'test-request', operation_id: 'admin.access' }
      },
      { status: 403 }
    )
  );
  const response = await proxyIdentity(event(), 'admin');
  assert.equal(response.status, 403);
  assert.equal((await response.text()).includes('must-not-leak'), false);
});

test('central login resumes an existing opaque session instead of requiring password', async () => {
  const app = event(
    `/login?${new URLSearchParams({ return_to: `${origins[2]}/`, code_challenge: 'x'.repeat(43) })}`,
    {
      origin: origins[1],
      jar: new Map([
        ['algohub_session', 'auth_session'],
        ['algohub_csrf', 'auth_csrf']
      ])
    }
  );
  mock.method(
    globalThis,
    'fetch',
    async (input: unknown, init?: RequestInit) => {
      if (String(input).endsWith('/me')) return envelope(principal);
      assert.equal(String(input).endsWith('/sso/prepare'), true);
      assert.equal(
        new Headers(init?.headers).get('cookie'),
        'algohub_session=auth_session; algohub_csrf=auth_csrf'
      );
      assert.equal(new Headers(init?.headers).get('x-csrf-token'), 'auth_csrf');
      return envelope({ code: 'prepared_code', expires_in: 120 });
    }
  );
  const destination = new URL(await location(() => centralLoginLoad(app)));
  assert.equal(destination.origin, origins[2]);
  assert.equal(destination.pathname, '/callback');
  assert.equal(destination.searchParams.get('code'), 'prepared_code');
  assert.equal(destination.searchParams.has('access_token'), false);
});

test('password login rejects missing pending state and nonce mismatch before sending credentials', async () => {
  const missing = event('/login?/password', {
    origin: origins[1],
    method: 'POST',
    headers: {
      origin: origins[1],
      'content-type': 'application/x-www-form-urlencoded'
    },
    body: 'email=a&password=b'
  });
  await assert.rejects(() => passwordLogin(missing), status(400));
  mock.method(globalThis, 'fetch', async () => unauthorized());
  const login = event('/login', { origin: origins[1] });
  await centralLoginLoad(login);
  const mismatch = event('/login?/password', {
    origin: origins[1],
    method: 'POST',
    jar: login.jar,
    headers: {
      origin: origins[1],
      'content-type': 'application/x-www-form-urlencoded'
    },
    body: new URLSearchParams({
      email: 'a@example.test',
      password: 'fixture-password',
      login_token: 'z'.repeat(43)
    }).toString()
  });
  await assert.rejects(() => passwordLogin(mismatch), status(403));
});

test('OAuth callback uses only cookie-bound PKCE and target, rejecting return_to spoofing and replay', async () => {
  process.env.ALGOHUB_ENABLE_FIXTURES = 'false';
  process.env.SUPABASE_URL = 'http://supabase.localhost:9999';
  process.env.SUPABASE_SSO_PROVIDER = 'google';
  mock.method(globalThis, 'fetch', async () => unauthorized());
  const login = event('/login', { origin: origins[1] });
  const data = await centralLoginLoad(login);
  const oauth = event('/login?/oauth', {
    origin: origins[1],
    method: 'POST',
    jar: login.jar,
    headers: {
      origin: origins[1],
      'content-type': 'application/x-www-form-urlencoded'
    },
    body: new URLSearchParams({ login_token: data.loginToken }).toString()
  });
  const authorize = new URL(await location(() => startProviderLogin(oauth)));
  assert.equal(
    authorize.searchParams.get('redirect_to'),
    `${origins[1]}/callback`
  );
  const pending = JSON.parse(
    Buffer.from(login.jar.get('algohub_auth_pending')!, 'base64url').toString()
  );
  assert.equal(
    authorize.searchParams.get('code_challenge'),
    createHash('sha256').update(pending.verifier).digest('base64url')
  );
  assert.equal(authorize.searchParams.has('apikey'), false);
  await assert.rejects(
    () =>
      completeCentralCallback(
        event(
          `/callback?code=provider_code&return_to=${encodeURIComponent('https://evil.test/')}`,
          { origin: origins[1], jar: login.jar }
        )
      ),
    status(400)
  );
  mock.method(
    globalThis,
    'fetch',
    async (input: unknown, init?: RequestInit) => {
      assert.equal(String(input), 'http://127.0.0.1:8080/v1/auth/callback');
      assert.deepEqual(JSON.parse(init?.body as string), {
        code: 'provider_code',
        code_verifier: pending.verifier
      });
      return envelope({ user: principal.user });
    }
  );
  assert.equal(
    await location(() =>
      completeCentralCallback(
        event('/callback?code=provider_code', {
          origin: origins[1],
          jar: login.jar
        })
      )
    ),
    '/'
  );
  await assert.rejects(
    () =>
      completeCentralCallback(
        event('/callback?code=provider_code', {
          origin: origins[1],
          jar: login.jar
        })
      ),
    status(400)
  );
});

test('permission-aware navigation requires overview and route-specific grants', () => {
  for (const item of adminNavigation)
    assert.equal(canAccessAdminRoute(principal, item.href), true, item.href);
  assert.equal(
    canAccessAdminRoute(
      {
        ...principal,
        grants: principal.grants.filter(
          (grant) => grant.permission !== 'admin.overview.read'
        )
      },
      '/praktikum'
    ),
    false
  );
  assert.equal(
    canAccessAdminRoute(
      {
        ...principal,
        assignments: [
          { ...principal.assignments[0], scope: { institution: 'only-one' } }
        ]
      },
      '/pengaturan'
    ),
    false
  );
  assert.equal(
    canAccessAdminRoute({ ...principal, grants: [] }, '/profil'),
    false
  );
  assert.equal(canAccessAdminRoute(principal, '/unknown'), false);
  for (const permission of ['question.manage', 'schedule.approve'])
    assert.equal(
      canAccessAdminRoute(
        {
          ...principal,
          grants: [
            principal.grants[0],
            {
              permission,
              app: 'praktikum',
              role: 'asisten',
              scope: { domain: 'praktikum' }
            }
          ]
        },
        '/praktikum'
      ),
      true
    );
  assert.equal(
    scopeContains(
      { domain: 'elearning', class_id: 'a' },
      { domain: 'elearning', class_id: 'b' }
    ),
    false
  );
  assert.equal(scopeContains({}, { domain: 'elearning', class_id: 'a' }), true);
});

test('browser client stays same-origin, parses errors, and preserves empty audit pagination', async () => {
  assert.equal(csrfToken('other=x; algohub_csrf=csrf_value'), 'csrf_value');
  assert.equal(csrfToken('algohub_csrf=%invalid'), '');
  const calls: Array<{ path: string; init?: RequestInit }> = [];
  const client = new AuthClient(async (input, init) => {
    calls.push({ path: String(input), init });
    return envelope([], { next: 25 });
  });
  assert.deepEqual(await client.audit(), { events: [], nextBefore: 25 });
  assert.equal(calls[0].path, '/api/v1/admin/auditlog?limit=50');
  assert.equal(calls[0].init?.credentials, 'same-origin');
  const denied = new AuthClient(async () => unauthorized());
  await assert.rejects(
    () => denied.me(),
    (cause: unknown) =>
      cause instanceof ApiError &&
      cause.status === 401 &&
      cause.requestId === 'test-request'
  );
});
