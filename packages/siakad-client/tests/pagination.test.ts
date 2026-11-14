import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  SiakadClient,
  emptyPage,
  loadResourcePage,
  mergeResources,
  versionedCommand
} from '../src/index.ts';

const identify = (row: { id: string }) => row.id;
const rows = (start: number, count: number) =>
  Array.from({ length: count }, (_, i) => ({ id: `row-${start + i}` }));

test('one user-driven page at a time reaches records past 25 and stops on a short page', async () => {
  const requests: string[] = [];
  const client = new SiakadClient(async (url) => {
    requests.push(String(url));
    const offset = Number(
      new URL(String(url), 'https://siakad.test').searchParams.get('offset')
    );
    return Response.json({ data: rows(offset, offset === 0 ? 25 : 2) });
  });
  const first = await loadResourcePage(
    client,
    '/inventory/items',
    emptyPage<{ id: string }>(),
    identify
  );
  assert.equal(first.rows.length, 25);
  assert.equal(first.offset, 25);
  assert.equal(first.hasMore, true);
  assert.equal(requests.length, 1);
  const next = await loadResourcePage(
    client,
    '/inventory/items',
    first,
    identify,
    true
  );
  assert.equal(next.rows.length, 27);
  assert.equal(next.offset, 27);
  assert.equal(next.hasMore, false);
  assert.deepEqual(requests, [
    '/api/v1/inventory/items?limit=25&offset=0',
    '/api/v1/inventory/items?limit=25&offset=25'
  ]);
  assert.equal(
    await loadResourcePage(client, '/inventory/items', next, identify, true),
    next
  );
  assert.equal(requests.length, 2);
});

test('retained creations and overlapping rows do not move the server offset or replace fresh scope', async () => {
  let count = 0;
  const fresh = {
    id: 'row-0',
    scope: { domain: 'siakad', institution: 'new' },
    version: 2
  };
  const client = new SiakadClient(async () =>
    Response.json({
      data: count++ === 0 ? [fresh, ...rows(1, 24)] : [fresh, { id: 'row-25' }]
    })
  );
  const retained = [
    {
      id: 'created-off-page',
      scope: { domain: 'siakad', institution: 'campus' },
      version: 1
    },
    { ...fresh, scope: { domain: 'siakad', institution: 'old' }, version: 1 }
  ];
  const page = await loadResourcePage(
    client,
    '/admin/siakad/kelas',
    emptyPage<{ id: string }>(),
    identify,
    false,
    retained
  );
  assert.equal(page.rows.length, 26);
  assert.equal(page.offset, 25);
  assert.deepEqual(
    page.rows.find((row) => row.id === fresh.id),
    fresh
  );
  assert.equal(
    page.rows.find((row) => row.id === retained[0].id),
    retained[0]
  );
  const next = await loadResourcePage(
    client,
    '/admin/siakad/kelas',
    page,
    identify,
    true
  );
  assert.equal(next.rows.length, 27);
  assert.equal(next.offset, 27);
  assert.equal(next.rows.filter((row) => row.id === fresh.id).length, 1);
});

test('failed load-more leaves the original cursor, rows and retry commands untouched', async () => {
  const original = { rows: rows(0, 25), offset: 25, hasMore: true };
  const client = new SiakadClient(async () => {
    throw new Error('offline');
  });
  await assert.rejects(
    loadResourcePage(client, '/academic/attendance', original, identify, true)
  );
  assert.equal(original.offset, 25);
  assert.equal(original.rows.length, 25);
  assert.equal(original.hasMore, true);
});

test('nested schedule identity can merge newly created schedules without altering pagination', () => {
  const old = {
    schedule: { id: 'schedule-1', version: 1 },
    open_session: null
  };
  const fresh = {
    schedule: { id: 'schedule-1', version: 2 },
    open_session: null
  };
  assert.deepEqual(
    mergeResources([old], [fresh], (row) => row.schedule.id),
    [fresh]
  );
});

test('bounded refresh retains an unresolved off-page rental without advancing the cursor', async () => {
  const pendingRow = { id: 'rental-off-page', version: 1, status: 'requested' };
  const requests: string[] = [];
  const client = new SiakadClient(async (url) => {
    requests.push(String(url));
    return Response.json({ data: rows(0, 25) });
  });
  const previous = {
    rows: [...rows(0, 25), pendingRow],
    offset: 50,
    hasMore: true
  };
  const refreshed = await loadResourcePage(
    client,
    '/admin/siakad/penyewaan',
    previous,
    identify,
    false,
    [pendingRow]
  );
  assert.equal(refreshed.rows.length, 26);
  assert.equal(
    refreshed.rows.find((row) => row.id === pendingRow.id),
    pendingRow
  );
  assert.equal(refreshed.offset, 25);
  assert.equal(refreshed.hasMore, true);
  assert.deepEqual(requests, [
    '/api/v1/admin/siakad/penyewaan?limit=25&offset=0'
  ]);
});

test('unresolved versioned validation replays its original bytes/key after a server refresh', () => {
  const path = '/admin/siakad/absensi/records/record-1';
  const original = versionedCommand(null, path, {
    version: 1,
    status: 'hadir',
    validated: true,
    notes: 'confirmed'
  });
  const retry = versionedCommand(original, path, {
    version: 2,
    status: 'hadir',
    validated: true,
    notes: 'confirmed'
  });
  assert.equal(retry, original);
  assert.equal(JSON.parse(retry.body).version, 1);
  const edited = versionedCommand(original, path, {
    version: 2,
    status: 'izin',
    validated: true,
    notes: 'confirmed'
  });
  assert.notEqual(edited.key, original.key);
  assert.equal(JSON.parse(edited.body).version, 2);
});
