import assert from 'node:assert/strict';
import { afterEach, beforeEach, mock, test } from 'node:test';
import type { Cookies } from '@sveltejs/kit';
import { proxyIdentity } from '../src/server.server.ts';

const admin = 'http://admin.localhost:5175';
const student = 'http://praktikum.localhost:5178';
const elearning = 'http://elearning.localhost:5177';
const siakad = 'http://siakad.localhost:5176';
const auth = 'http://auth.localhost:5174';
const settings = {
  ALGOHUB_ENV: 'local',
  ALGOHUB_ENABLE_FIXTURES: 'true',
  ALGOHUB_DATABASE_URL: '',
  ALGOHUB_API_BASE_URL: 'http://127.0.0.1:8080',
  ALGOHUB_AUTH_ORIGIN: auth,
  ALGOHUB_ALLOWED_ORIGINS: [admin, student, elearning, siakad, auth].join(','),
  ALGOHUB_SSO_ALLOWED_ORIGINS: [admin, student, elearning, siakad, auth].join(
    ','
  )
};
let original: Record<string, string | undefined>;
beforeEach(() => {
  original = Object.fromEntries(
    Object.keys(settings).map((key) => [key, process.env[key]])
  );
  Object.assign(process.env, settings);
  mock.method(globalThis, 'fetch', async () => {
    throw new Error('Unexpected upstream request');
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
  path: string,
  options: {
    method?: string;
    body?: unknown;
    key?: string;
    origin?: string;
    csrf?: boolean;
  } = {}
) {
  const url = new URL(`/api/v1${path}`, options.origin ?? student);
  const headers: Record<string, string> = {
    origin: url.origin,
    'content-type': 'application/json'
  };
  if (options.csrf !== false) headers['x-csrf-token'] = 'fixture-csrf';
  if (options.key !== undefined) headers['idempotency-key'] = options.key;
  return {
    url,
    request: new Request(url, {
      method: options.method ?? 'GET',
      headers,
      body:
        options.body === undefined ? undefined : JSON.stringify(options.body)
    }),
    cookies: {
      get: (name: string) =>
        name === 'algohub_session' ? 'fixture-session' : 'fixture-csrf',
      set: () => {}
    } as unknown as Cookies,
    setHeaders: () => {}
  };
}
function reply(data: unknown, status = 200) {
  return Response.json(
    {
      data,
      meta: { request_id: 'proxy-test', operation_id: 'practicum.test' }
    },
    { status, headers: { 'x-request-id': 'proxy-test' } }
  );
}

test('Practicum proxy forwards CSRF and stable idempotency key with a domain-sized body', async () => {
  let calls = 0;
  mock.method(globalThis, 'fetch', async (url: string, init: RequestInit) => {
    calls++;
    assert.equal(
      url,
      'http://127.0.0.1:8080/v1/practicum/attempts/attempt-1/answers'
    );
    const headers = new Headers(init.headers);
    assert.equal(headers.get('idempotency-key'), 'save:revision-1');
    assert.equal(headers.get('x-csrf-token'), 'fixture-csrf');
    assert.match(
      headers.get('cookie') ?? '',
      /algohub_session=fixture-session/
    );
    assert.ok(Buffer.byteLength(init.body as string) > 65_536);
    return reply({ revision: 1 });
  });
  const request = () =>
    event('/practicum/attempts/attempt-1/answers', {
      method: 'PATCH',
      key: 'save:revision-1',
      body: {
        revision: 0,
        answers: [
          { question_id: 'one', value: 'a'.repeat(40_000) },
          { question_id: 'two', value: 'b'.repeat(40_000) }
        ]
      }
    });
  for (let i = 0; i < 2; i++) {
    const response = await proxyIdentity(request(), 'praktikum');
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('cache-control'), 'no-store');
    assert.equal(response.headers.get('x-request-id'), 'proxy-test');
  }
  assert.equal(calls, 2);
});

test('proxy rejects invalid replay keys, missing CSRF, and oversized bodies before upstream', async () => {
  for (const key of [undefined, 'bad key', 'x'.repeat(129)]) {
    const response = await proxyIdentity(
      event('/practicum/attempts', { method: 'POST', key, body: {} }),
      'praktikum'
    );
    assert.equal(response.status, 422);
  }
  assert.equal(
    (
      await proxyIdentity(
        event('/practicum/attempts', {
          method: 'POST',
          key: 'start',
          csrf: false,
          body: {}
        }),
        'praktikum'
      )
    ).status,
    403
  );
  assert.equal(
    (
      await proxyIdentity(
        event('/practicum/attempts', {
          method: 'POST',
          key: 'start',
          body: { value: 'a'.repeat(262_145) }
        }),
        'praktikum'
      )
    ).status,
    413
  );
  // The larger Practicum limit must not enlarge identity's existing body limit.
  assert.equal(
    (
      await proxyIdentity(
        event('/me', { method: 'PATCH', body: { value: 'a'.repeat(65_537) } }),
        'praktikum'
      )
    ).status,
    413
  );
});

test('browser proxy denies cron, arbitrary provider URLs, admin on student host and duplicate queries', async () => {
  for (const path of [
    '/internal/practicum/reconcile',
    '/practicum/provider',
    '/admin/praktikum/sesi'
  ]) {
    assert.equal((await proxyIdentity(event(path), 'praktikum')).status, 404);
  }
  assert.equal(
    (
      await proxyIdentity(
        event('/practicum/sessions?limit=1&limit=2'),
        'praktikum'
      )
    ).status,
    422
  );
  assert.equal(
    (await proxyIdentity(event('/practicum/sessions?user_id=2'), 'praktikum'))
      .status,
    422
  );
});

test('only activation creation exposes a classroom token; identity secrets stay filtered', async () => {
  const data = {
    id: 'activation-new',
    access_token: 'classroom-token',
    refresh_token: 'secret',
    nested: { access_token: 'secret' }
  };
  mock.method(globalThis, 'fetch', async () => reply(data, 201));
  const response = await proxyIdentity(
    event('/admin/praktikum/sesi/session-1/activate', {
      origin: admin,
      method: 'POST',
      key: 'activate',
      body: {}
    }),
    'admin'
  );
  assert.equal(response.status, 201);
  assert.deepEqual((await response.json()).data, {
    id: 'activation-new',
    access_token: 'classroom-token',
    nested: {}
  });
  mock.method(globalThis, 'fetch', async () => reply(data));
  const other = await proxyIdentity(
    event('/practicum/sessions/session-1'),
    'praktikum'
  );
  assert.deepEqual((await other.json()).data, {
    id: 'activation-new',
    nested: {}
  });
});

test('proxy preserves runner quota retry headers and error correlation', async () => {
  mock.method(globalThis, 'fetch', async () =>
    Response.json(
      { error: { code: 'RUNNER_QUOTA_EXCEEDED', message: 'Coba lagi.' } },
      {
        status: 429,
        headers: { 'retry-after': '30', 'x-request-id': 'quota-test' }
      }
    )
  );
  const response = await proxyIdentity(
    event('/practicum/attempts/attempt-1/run', {
      method: 'POST',
      key: 'run',
      body: {}
    }),
    'praktikum'
  );
  assert.equal(response.status, 429);
  assert.equal(response.headers.get('retry-after'), '30');
  assert.equal((await response.json()).meta.request_id, 'quota-test');
});

test('E-learning proxy forwards mutations with idempotency key and blocks admin routes on student host', async () => {
  mock.method(globalThis, 'fetch', async (url: string, init: RequestInit) => {
    assert.equal(
      url,
      'http://127.0.0.1:8080/v1/learning/lessons/lesson-1/progress'
    );
    const headers = new Headers(init.headers);
    assert.equal(headers.get('idempotency-key'), 'progress-1');
    assert.equal(headers.get('x-csrf-token'), 'fixture-csrf');
    return reply({ progress: { revision: 1 } });
  });
  const ok = await proxyIdentity(
    event('/learning/lessons/lesson-1/progress', {
      origin: elearning,
      method: 'PUT',
      key: 'progress-1',
      body: {
        revision: 0,
        draft_code: 'int main(void) { return 0; }',
        language: 'c',
        complete: false
      }
    }),
    'elearning'
  );
  assert.equal(ok.status, 200);
  assert.equal(
    (
      await proxyIdentity(
        event('/admin/elearning/kurikulum', { origin: elearning }),
        'elearning'
      )
    ).status,
    404
  );
});

test('SIAKAD proxy exposes one-time qr_token only on attendance session creation and blocks admin routes on siakad host', async () => {
  const data = {
    id: 'att-session-1',
    qr_token: 'one-time-qr',
    refresh_token: 'secret'
  };
  mock.method(globalThis, 'fetch', async () => reply(data, 201));
  const created = await proxyIdentity(
    event('/admin/siakad/absensi/sesi', {
      origin: admin,
      method: 'POST',
      key: 'open-att',
      body: {}
    }),
    'admin'
  );
  assert.equal(created.status, 201);
  assert.deepEqual((await created.json()).data, {
    id: 'att-session-1',
    qr_token: 'one-time-qr'
  });
  mock.method(globalThis, 'fetch', async () => reply(data, 200));
  const listed = await proxyIdentity(
    event('/academic/schedules', { origin: siakad }),
    'siakad'
  );
  assert.deepEqual((await listed.json()).data, {
    id: 'att-session-1'
  });
  assert.equal(
    (
      await proxyIdentity(
        event('/admin/siakad/laporan', { origin: siakad }),
        'siakad'
      )
    ).status,
    404
  );
});

test('preview fallback proxy forwards only POST with the domain body budget', async () => {
  let upstreamBody = '';
  mock.method(globalThis, 'fetch', async (url: string, init: RequestInit) => {
    assert.equal(url, 'http://127.0.0.1:8080/v1/playground/run');
    upstreamBody = init.body as string;
    return reply({ status: 'provider_error', authoritative: false });
  });
  const response = await proxyIdentity(
    event('/playground/run', {
      method: 'POST',
      key: 'preview:one',
      origin: elearning,
      body: {
        language: 'c',
        source: 'a'.repeat(70_000),
        stdin: ''
      }
    }),
    'elearning'
  );
  assert.equal(response.status, 200);
  assert.ok(Buffer.byteLength(upstreamBody) > 65_536);
  assert.equal(
    (
      await proxyIdentity(
        event('/playground/run', { origin: elearning }),
        'elearning'
      )
    ).status,
    405
  );
  assert.equal(
    (
      await proxyIdentity(
        event('/playground/run/other', {
          method: 'POST',
          key: 'preview:two',
          origin: elearning
        }),
        'elearning'
      )
    ).status,
    404
  );
});
