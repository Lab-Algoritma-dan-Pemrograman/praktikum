import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError } from '@algohub/auth-client';
import {
  PendingCommands,
  refreshAttendanceDrafts,
  reusableCommand,
  versionedCommand,
  type AcademicAttendanceRecord
} from '../src/index.ts';

const original = {
  id: 'record-1',
  status: 'hadir',
  notes: '',
  version: 1
} as AcademicAttendanceRecord;
const fresh = {
  ...original,
  status: 'izin',
  notes: 'Excused by another staff member',
  version: 2
} as AcademicAttendanceRecord;
const path = '/admin/siakad/absensi/records/record-1';

function validation(version: number, status = 'hadir', notes = '') {
  return { version, status, validated: true, notes };
}

test('refresh rebases pristine validation fields instead of overwriting another staff update', () => {
  const statuses = { [original.id]: original.status };
  const notes = { [original.id]: original.notes };
  const refreshed = refreshAttendanceDrafts(
    [original],
    [fresh],
    statuses,
    notes,
    new PendingCommands()
  );
  assert.equal(refreshed.statuses[original.id], 'izin');
  assert.equal(refreshed.notes[original.id], fresh.notes);
  assert.deepEqual(refreshed.conflicts, {});
  assert.equal(statuses[original.id], 'hadir');
  assert.equal(notes[original.id], '');
});

test('a note-only draft adopts a remote status change without discarding the note', () => {
  const remote = { ...fresh, notes: original.notes };
  const refreshed = refreshAttendanceDrafts(
    [original],
    [remote],
    { [original.id]: 'hadir' },
    { [original.id]: 'My unsaved note' },
    new PendingCommands()
  );
  assert.equal(refreshed.statuses[original.id], 'izin');
  assert.equal(refreshed.notes[original.id], 'My unsaved note');
  assert.deepEqual(refreshed.conflicts, {});
});

test('diverging local fields remain conflicts across repeated refreshes until resolved', () => {
  const registry = new PendingCommands();
  const refreshed = refreshAttendanceDrafts(
    [original],
    [fresh],
    { [original.id]: 'alpha' },
    { [original.id]: 'My unsaved note' },
    registry
  );
  assert.equal(refreshed.statuses[original.id], 'alpha');
  assert.equal(refreshed.notes[original.id], 'My unsaved note');
  assert.deepEqual(refreshed.conflicts[original.id], {
    status: true,
    notes: true
  });
  const again = refreshAttendanceDrafts(
    [fresh],
    [fresh],
    refreshed.statuses,
    refreshed.notes,
    registry,
    refreshed.conflicts
  );
  assert.deepEqual(again.conflicts, refreshed.conflicts);
  const resolved = refreshAttendanceDrafts(
    [fresh],
    [fresh],
    { [original.id]: fresh.status },
    { [original.id]: fresh.notes },
    registry,
    again.conflicts
  );
  assert.deepEqual(resolved.conflicts, {});
});

test('refresh initializes newly fetched rows and ignores pending commands for other resources', () => {
  const registry = new PendingCommands();
  registry.put(versionedCommand(null, `${path}-other`, validation(1)));
  const another = { ...fresh, id: 'record-2' };
  const refreshed = refreshAttendanceDrafts(
    [original],
    [fresh, another],
    { [original.id]: 'hadir' },
    { [original.id]: '' },
    registry
  );
  assert.equal(refreshed.statuses[original.id], 'izin');
  assert.equal(refreshed.notes[original.id], fresh.notes);
  assert.equal(refreshed.statuses[another.id], 'izin');
  assert.equal(refreshed.notes[another.id], another.notes);
});

test('an ambiguous validation response keeps even a pristine draft and its original retry bytes', () => {
  const registry = new PendingCommands();
  const pending = registry.put(versionedCommand(null, path, validation(1)));
  registry.failed(pending, new ApiError('Disconnected'));
  const refreshed = refreshAttendanceDrafts(
    [original],
    [fresh],
    { [original.id]: 'hadir' },
    { [original.id]: '' },
    registry
  );
  const retry = versionedCommand(
    registry.get(path, 'PATCH'),
    path,
    validation(2, refreshed.statuses[original.id], refreshed.notes[original.id])
  );
  assert.equal(retry, pending);
  assert.equal(JSON.parse(retry.body).version, 1);
  assert.deepEqual(refreshed.conflicts, {});
  registry.failed(pending, new ApiError('Unavailable', 503));
  assert.equal(registry.get(path, 'PATCH'), pending);
});

test('definitive 409 clears only the rejected entry and allows the refreshed server version', () => {
  for (const [target, body] of [
    [path, validation(1)],
    [
      '/admin/siakad/penyewaan/rental-1',
      { version: 1, action: 'verify_payment' }
    ]
  ] as const) {
    const registry = new PendingCommands();
    const unrelated = registry.put(
      reusableCommand(null, '/admin/siakad/inventaris', 'POST', {
        name: 'New item'
      })
    );
    const pending = registry.put(versionedCommand(null, target, body));
    registry.failed(pending, new ApiError('Conflict', 409, 'CONFLICT'));
    assert.equal(registry.get(target, 'PATCH'), null);
    assert.equal(registry.get(unrelated.path, unrelated.method), unrelated);
    const retry = versionedCommand(registry.get(target, 'PATCH'), target, {
      ...body,
      version: 2
    });
    assert.notEqual(retry.key, pending.key);
    assert.equal(JSON.parse(retry.body).version, 2);
  }
});

test('lost committed creates retain their bytes and key across unrelated successful operations', () => {
  for (const createPath of [
    '/admin/siakad/inventaris',
    '/admin/siakad/jadwal'
  ]) {
    const registry = new PendingCommands();
    const draft = { name: 'Unchanged create draft' };
    const created = registry.put(
      reusableCommand(null, createPath, 'POST', draft)
    );
    registry.failed(created, new ApiError('Disconnected'));
    const unrelated = registry.put(versionedCommand(null, path, validation(1)));
    registry.complete(unrelated);
    const retry = registry.put(
      reusableCommand(
        registry.get(createPath, 'POST'),
        createPath,
        'POST',
        draft
      )
    );
    assert.equal(retry, created);
    assert.equal(retry.body, created.body);
    assert.equal(retry.key, created.key);
  }
});

test('registry isolates methods and old completions cannot clear an edited replacement', () => {
  const registry = new PendingCommands();
  const create = registry.put(
    reusableCommand(null, path, 'POST', { title: 'Original' })
  );
  const patch = registry.put(versionedCommand(null, path, validation(1)));
  const edited = registry.put(
    reusableCommand(create, path, 'POST', { title: 'Edited' })
  );
  registry.complete(create);
  assert.equal(registry.get(path, 'POST'), edited);
  assert.equal(registry.get(path, 'PATCH'), patch);
  registry.complete(patch);
  assert.equal(registry.get(path, 'POST'), edited);
  assert.equal(registry.get(path, 'PATCH'), null);
});
