import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, type Principal } from '@algohub/auth-client';
import {
  PracticumClient,
  command,
  reusableCommand,
  permitted,
  jakarta,
  errorMessage
} from '../src/index.ts';

test('client uses only same-origin BFF with no cache and stable mutation bytes/key', async () => {
  const requests: { url: unknown; init: RequestInit }[] = [];
  const client = new PracticumClient(async (url, init) => {
    requests.push({ url, init: init! });
    return Response.json({
      data: { revision: 1 },
      meta: { request_id: 'request-fixture' }
    });
  });
  const mutation = command('/practicum/attempts/fixture/answers', 'PATCH', {
    revision: 0,
    answers: []
  });
  await client.execute(mutation);
  await client.execute(mutation);
  assert.equal(requests[0].url, '/api/v1/practicum/attempts/fixture/answers');
  assert.equal(requests[0].init.credentials, 'same-origin');
  assert.equal(requests[0].init.cache, 'no-store');
  assert.equal(requests[0].init.redirect, 'error');
  assert.equal(
    new Headers(requests[0].init.headers).get('Idempotency-Key'),
    mutation.key
  );
  assert.equal(new Headers(requests[0].init.headers).has('X-CSRF-Token'), true);
  assert.equal(requests[0].init.body, requests[1].init.body);
  assert.equal(
    new Headers(requests[1].init.headers).get('Idempotency-Key'),
    mutation.key
  );
});

test('only identical operations reuse an idempotency key', () => {
  const previous = command('/path', 'POST', { n: 1 });
  assert.equal(reusableCommand(previous, '/path', 'POST', { n: 1 }), previous);
  assert.notEqual(
    reusableCommand(previous, '/path', 'POST', { n: 2 }).key,
    previous.key
  );
  assert.notEqual(
    reusableCommand(previous, '/other', 'POST', { n: 1 }).key,
    previous.key
  );
  assert.notEqual(
    reusableCommand(previous, '/path', 'PUT', { n: 1 }).key,
    previous.key
  );
});

test('read identifiers are encoded and list pagination is explicit', async () => {
  const urls: string[] = [];
  const client = new PracticumClient(async (url) => {
    urls.push(String(url));
    return Response.json({ data: [] });
  });
  await client.attempt('part/other?x=1');
  await client.detail('session/special');
  await client.sessions(25);
  await client.grades(50);
  assert.deepEqual(urls, [
    '/api/v1/practicum/attempts/part%2Fother%3Fx%3D1',
    '/api/v1/practicum/sessions/session%2Fspecial',
    '/api/v1/practicum/sessions?limit=25&offset=25',
    '/api/v1/practicum/grades?limit=25&offset=50'
  ]);
});

test('malformed success bodies and network failures become bounded API errors', async () => {
  for (const body of [null, 'not an envelope', 1, {}, []]) {
    const client = new PracticumClient(async () => Response.json(body));
    await assert.rejects(
      client.sessions(),
      (error: unknown) => error instanceof ApiError && error.status === 502
    );
  }
  const client = new PracticumClient(async () => {
    throw new Error('private upstream details');
  });
  await assert.rejects(
    client.sessions(),
    (error: unknown) =>
      error instanceof ApiError && !error.message.includes('private upstream')
  );
});

test('conflict/forbidden errors retain request IDs for a safe retry explanation', async () => {
  const client = new PracticumClient(async () =>
    Response.json(
      {
        error: { code: 'CONFLICT', message: 'revision changed' },
        meta: { request_id: 'fixture-request' }
      },
      { status: 409 }
    )
  );
  await assert.rejects(client.sessions(), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.match(errorMessage(error), /Data berubah di server/);
    assert.match(errorMessage(error), /fixture-request/);
    return true;
  });
  assert.match(errorMessage(new ApiError('forbidden', 403)), /Akses ditolak/);
  assert.match(
    errorMessage(new ApiError('expired', 401)),
    /Masuk kembali di tab lain/
  );
});

test('UI permissions require both exact permission and contained scope, never the admin host', () => {
  const principal = {
    grants: [
      {
        permission: 'practicum.session.manage',
        scope: { domain: 'praktikum', class_id: 'class-101', shift: 1 }
      }
    ]
  } as Principal;
  assert.equal(
    permitted(principal, 'practicum.session.manage', {
      domain: 'praktikum',
      class_id: 'class-101',
      shift: 1,
      session_id: 's1'
    }),
    true
  );
  assert.equal(
    permitted(principal, 'practicum.session.manage', {
      domain: 'praktikum',
      class_id: 'other',
      shift: 1
    }),
    false
  );
  assert.equal(
    permitted(principal, 'practicum.session.manage', {
      domain: 'praktikum',
      class_id: 'class-101',
      shift: 2
    }),
    false
  );
  assert.equal(
    permitted(principal, 'practicum.session.manage', { domain: 'praktikum' }),
    false
  );
  assert.equal(
    permitted(principal, 'practicum.grade.write', {
      domain: 'praktikum',
      class_id: 'class-101',
      shift: 1
    }),
    false
  );
  assert.equal(permitted(null, 'practicum.session.manage', {}), false);
});

test('display always converts UTC to Jakarta, independent of device timezone', () => {
  assert.match(jakarta('2026-01-01T00:00:00Z'), /07[.:]00[.:]00 WIB$/);
  assert.equal(jakarta(null), 'Belum ditetapkan');
});
