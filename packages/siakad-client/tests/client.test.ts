import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, type Principal } from '@algohub/auth-client';
import {
  SiakadClient,
  command,
  reusableCommand,
  rentalCommand,
  permitted,
  jakarta,
  rupiah,
  errorMessage
} from '../src/index.ts';

test('SiakadClient uses same-origin BFF with CSRF and Idempotency-Key', async () => {
  const requests: { url: unknown; init: RequestInit }[] = [];
  const client = new SiakadClient(async (url, init) => {
    requests.push({ url, init: init! });
    return Response.json({
      data: { id: 'rental-1' },
      meta: { request_id: 'req-siakad' }
    });
  });
  const cmd = command('/inventory/rentals', 'POST', {
    item_id: 'item-1',
    quantity: 1,
    purpose: 'Praktikum',
    due_at: '2026-12-01T00:00:00Z'
  });
  await client.execute(cmd);
  assert.equal(requests[0].url, '/api/v1/inventory/rentals');
  assert.equal(requests[0].init.credentials, 'same-origin');
  assert.equal(requests[0].init.cache, 'no-store');
  assert.equal(
    new Headers(requests[0].init.headers).get('Idempotency-Key'),
    cmd.key
  );
  assert.equal(
    reusableCommand(cmd, cmd.path, cmd.method, JSON.parse(cmd.body)),
    cmd
  );
});

test('rental retries preserve due_at, request bytes and key despite elapsed time', () => {
  const draft = {
    item_id: 'item-1',
    quantity: 1,
    purpose: 'Praktikum',
    due_days: 3
  };
  const now = Date.parse('2026-11-09T01:00:00Z');
  const original = rentalCommand(null, draft, now);
  const replay = rentalCommand(original, { ...draft }, now + 60_000);
  assert.equal(replay, original);
  assert.equal(JSON.parse(original.body).due_at, '2026-11-12T01:00:00.000Z');
  assert.equal('due_days' in JSON.parse(original.body), false);
  for (const change of [
    { item_id: 'item-2' },
    { quantity: 2 },
    { purpose: 'Penelitian' },
    { due_days: 4 }
  ]) {
    const next = rentalCommand(original, { ...draft, ...change }, now + 60_000);
    assert.notEqual(next.key, original.key);
    assert.notEqual(next.body, original.body);
  }
  assert.notEqual(rentalCommand(null, draft, now).key, original.key);
});

test('division-scoped permissions match shared SIAKAD resources within institution/class scope', () => {
  const k3Principal = {
    grants: [
      {
        permission: 'inventory.manage',
        scope: { domain: 'siakad', division: 'k3' }
      },
      {
        permission: 'rental.approve',
        scope: { domain: 'siakad', division: 'k3', institution: 'lab' }
      }
    ]
  } as Principal;

  assert.equal(
    permitted(k3Principal, 'inventory.manage', {
      domain: 'siakad',
      institution: 'lab'
    }),
    true
  );
  assert.equal(
    permitted(k3Principal, 'rental.approve', {
      domain: 'siakad',
      institution: 'other-campus'
    }),
    false
  );
  assert.equal(
    permitted(k3Principal, 'finance.manage', {
      domain: 'siakad',
      institution: 'lab'
    }),
    false
  );
});

test('formatting and domain error messages preserve specific error codes', () => {
  assert.match(jakarta('2026-01-01T00:00:00Z'), /07[.:]00[.:]00 WIB$/);
  assert.match(rupiah(25000), /25[.,]000/);
  assert.match(
    errorMessage(
      new ApiError('Stok tidak cukup.', 409, 'INSUFFICIENT_STOCK', 'req-stock')
    ),
    /Stok tidak cukup.*req-stock/
  );
  assert.match(
    errorMessage(
      new ApiError('Sesi ditutup.', 409, 'ATTENDANCE_CLOSED', 'req-att')
    ),
    /Sesi ditutup.*req-att/
  );
});
