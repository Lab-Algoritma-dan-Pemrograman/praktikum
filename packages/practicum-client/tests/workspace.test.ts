import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError } from '@algohub/auth-client';
import type { Answer, Attempt, Command } from '../src/index.ts';
import {
  Workspace,
  ServerClock,
  acceptsAttempt,
  answersById,
  reconcileDrafts,
  timerLabel
} from '../src/workspace.ts';

function attempt(overrides: Partial<Attempt> = {}): Attempt {
  return {
    attempt_id: 'attempt-test',
    activation_id: 'activation-test',
    session_id: 'session-test',
    course_id: 'course-test',
    user_id: 1,
    status: 'in_progress',
    submission_reason: null,
    revision: 0,
    grade_revision: 0,
    grading_status: 'pending',
    started_at: '2026-01-01T00:00:00Z',
    deadline_at: '2026-01-01T00:30:00Z',
    server_now: '2026-01-01T00:00:00Z',
    submitted_at: null,
    timezone: 'Asia/Jakarta',
    timezone_offset: '+07:00',
    score: null,
    percentage: null,
    max_score: 10,
    questions: [
      {
        id: 'q1',
        type: 'essay',
        prompt: 'Fixture question',
        difficulty: 'easy',
        category: '',
        points: 10,
        starter_code: '',
        language: ''
      }
    ],
    answers: [],
    ...overrides
  };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
function harness(initial = attempt()) {
  let server = initial;
  let now = 0;
  const sent: Command[] = [];
  let handler = async (cmd: Command): Promise<Attempt> => {
    if (cmd.method === 'PATCH') {
      const body = JSON.parse(cmd.body);
      server = {
        ...server,
        revision: server.revision + 1,
        answers: body.answers
      };
    } else
      server = { ...server, status: 'submitted', submission_reason: 'manual' };
    return structuredClone(server);
  };
  const workspace = new Workspace(
    {
      attempt: async () => structuredClone(server),
      execute: async <T>(cmd: Command) => {
        sent.push(cmd);
        return (await handler(cmd)) as T;
      }
    },
    initial.attempt_id,
    () => {},
    () => now
  );
  return {
    workspace,
    sent,
    setServer(value: Attempt) {
      server = value;
    },
    setHandler(value: typeof handler) {
      handler = value;
    },
    advance(ms: number) {
      now += ms;
    }
  };
}

test('server clock uses a monotonic anchor, clamps delayed snapshots and fails closed on invalid time', () => {
  let monotonic = 0;
  const clock = new ServerClock(() => monotonic);
  clock.sync(attempt());
  monotonic = 10_000;
  assert.equal(clock.milliseconds(), 1_790_000);
  clock.sync(attempt()); // stale response cannot add the elapsed ten seconds back
  assert.equal(clock.milliseconds(), 1_790_000);
  monotonic += 1_790_001;
  assert.equal(clock.milliseconds(), 0);
  assert.equal(timerLabel(60_001), '01:01');
  assert.equal(timerLabel(0), '00:00');
  clock.sync({ deadline_at: 'invalid', server_now: 'invalid' });
  assert.equal(clock.milliseconds(), 0);
});

test('out-of-order responses cannot regress revision, grade revision, final status, or identity', () => {
  const current = attempt({ revision: 2, grade_revision: 1, status: 'graded' });
  for (const old of [
    { revision: 1 },
    { grade_revision: 0 },
    { status: 'submitted' as const },
    { attempt_id: 'another-user' }
  ])
    assert.equal(acceptsAttempt(current, { ...current, ...old }), false);
  assert.equal(
    acceptsAttempt(current, { ...current, grade_revision: 2 }),
    true
  );
});

test('three-way reconciliation preserves local edits and identifies only diverging answers', () => {
  const a = (value: string): Answer => ({ question_id: 'q1', value });
  assert.deepEqual(
    reconcileDrafts({ q1: a('base') }, { q1: a('local') }, { q1: a('remote') })
      .conflicts,
    ['q1']
  );
  assert.equal(
    reconcileDrafts({ q1: a('base') }, { q1: a('local') }, { q1: a('base') })
      .drafts.q1.value,
    'local'
  );
  assert.deepEqual(
    reconcileDrafts({ q1: a('base') }, { q1: a('local') }, { q1: a('local') })
      .conflicts,
    []
  );
  assert.equal(answersById(attempt()).q1.value, '');
});

test('background polling stops after terminal state, authorization failure, or disposal', async () => {
  for (const status of ['submitted', 'graded'] as const) {
    let reads = 0;
    const workspace = new Workspace(
      {
        attempt: async () => {
          reads += 1;
          return attempt({ status });
        },
        execute: async () => {
          throw new Error('Unexpected mutation');
        }
      },
      'attempt-test',
      () => {}
    );
    await workspace.refresh();
    assert.equal(await workspace.poll(), false);
    assert.equal(reads, 1);
    workspace.dispose();
    assert.equal(await workspace.refresh(), false);
    assert.equal(reads, 1);
  }
  for (const status of [401, 403]) {
    let reads = 0;
    const workspace = new Workspace(
      {
        attempt: async () => {
          reads += 1;
          throw new ApiError('Access denied', status);
        },
        execute: async () => {
          throw new Error('Unexpected mutation');
        }
      },
      'attempt-test',
      () => {}
    );
    await workspace.refresh();
    for (let tick = 0; tick < 10; tick++) await workspace.poll();
    assert.equal(reads, 1);
    workspace.dispose();
  }
});

test('background polling pauses after three read failures and manual recovery resumes it', async () => {
  let reads = 0;
  let unavailable = true;
  const workspace = new Workspace(
    {
      attempt: async () => {
        reads += 1;
        if (unavailable) throw new ApiError('Unavailable', 503);
        return attempt();
      },
      execute: async () => {
        throw new Error('Unexpected mutation');
      }
    },
    'attempt-test',
    () => {}
  );
  await workspace.refresh();
  for (let tick = 0; tick < 10; tick++) await workspace.poll();
  assert.equal(reads, 3);
  unavailable = false;
  assert.equal(await workspace.refresh(), true);
  assert.equal(await workspace.poll(), true);
  assert.equal(reads, 5);
  workspace.dispose();
});

test('background polling does not compete with offline, conflicted, or pending writes', async () => {
  let reads = 0;
  const workspace = new Workspace(
    {
      attempt: async () => {
        reads += 1;
        return attempt();
      },
      execute: async () => {
        throw new Error('Unexpected mutation');
      }
    },
    'attempt-test',
    () => {}
  );
  await workspace.refresh();
  for (const key of [
    'offline',
    'conflict',
    'saving',
    'submitting',
    'syncing'
  ] as const) {
    workspace.state[key] = true;
    assert.equal(await workspace.poll(), false);
    workspace.state[key] = false;
  }
  assert.equal(reads, 1);
  workspace.dispose();
});

test('autosave debounces, sends revision and idempotency key, and adopts server timestamp', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  const response = deferred<Attempt>();
  h.setHandler(() => response.promise);
  h.workspace.edit('q1', 'a');
  t.mock.timers.tick(500);
  h.workspace.edit('q1', 'ab');
  t.mock.timers.tick(699);
  assert.equal(h.sent.length, 0);
  t.mock.timers.tick(1);
  assert.equal(h.sent.length, 1);
  assert.ok(h.sent[0].key);
  assert.deepEqual(JSON.parse(h.sent[0].body), {
    revision: 0,
    answers: [{ question_id: 'q1', value: 'ab' }]
  });
  response.resolve(
    attempt({
      revision: 1,
      answers: [
        { question_id: 'q1', value: 'ab', updated_at: '2026-01-01T00:00:01Z' }
      ]
    })
  );
  await h.workspace.flush();
  assert.equal(h.workspace.state.dirty, false);
  assert.equal(h.workspace.state.attempt?.revision, 1);
  assert.equal(
    h.workspace.state.attempt?.answers[0].updated_at,
    '2026-01-01T00:00:01Z'
  );
});

test('edits during an in-flight save are serialized into the next revision, never overwritten', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'first');
  const response = deferred<Attempt>();
  h.setHandler(async () =>
    h.sent.length === 1
      ? response.promise
      : attempt({
          revision: 2,
          answers: [{ question_id: 'q1', value: 'second' }]
        })
  );
  const saving = h.workspace.flush();
  h.workspace.edit('q1', 'second');
  response.resolve(
    attempt({ revision: 1, answers: [{ question_id: 'q1', value: 'first' }] })
  );
  assert.equal(await saving, true);
  assert.equal(h.sent.length, 2);
  assert.equal(JSON.parse(h.sent[1].body).revision, 1);
  assert.notEqual(h.sent[0].key, h.sent[1].key);
  assert.equal(h.workspace.state.drafts.q1.value, 'second');
  assert.equal(h.workspace.state.conflict, false);
  assert.equal(h.workspace.state.dirty, false);
});

test('lost save response retries the identical command even when a newer draft exists', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'first');
  h.setHandler(async () => {
    if (h.sent.length === 1) throw new ApiError('lost response');
    return attempt({
      revision: h.sent.length - 1,
      answers: [
        { question_id: 'q1', value: h.sent.length === 2 ? 'first' : 'second' }
      ]
    });
  });
  assert.equal(await h.workspace.flush(), false);
  h.workspace.edit('q1', 'second');
  assert.equal(await h.workspace.flush(), true);
  assert.deepEqual(h.sent[0], h.sent[1]);
  assert.notEqual(h.sent[1].key, h.sent[2].key);
  assert.equal(JSON.parse(h.sent[2].body).revision, 1);
  assert.equal(h.workspace.state.dirty, false);
});

test('reconnect acknowledges an applied-but-unacknowledged save without a false conflict', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'first');
  h.setHandler(async () => {
    h.setServer(
      attempt({ revision: 1, answers: [{ question_id: 'q1', value: 'first' }] })
    );
    throw new ApiError('lost response');
  });
  await h.workspace.flush();
  h.workspace.edit('q1', 'second');
  h.setHandler(async (cmd) =>
    attempt({
      revision: JSON.parse(cmd.body).revision + 1,
      answers: JSON.parse(cmd.body).answers
    })
  );
  await h.workspace.reconnect();
  assert.equal(h.workspace.state.conflict, false);
  assert.equal(h.workspace.state.drafts.q1.value, 'second');
  assert.equal(h.workspace.state.dirty, false);
});

test('offline edits are retained in memory and saved only after server reconciliation', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.setOffline(true);
  h.workspace.edit('q1', 'offline fixture');
  assert.equal(await h.workspace.flush(), false);
  assert.equal(h.sent.length, 0);
  await h.workspace.reconnect();
  assert.equal(h.sent.length, 1);
  assert.equal(h.workspace.state.dirty, false);
});

test('409 fetches authority and pauses automatic retry until explicit local resolution', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'local');
  h.setServer(
    attempt({ revision: 1, answers: [{ question_id: 'q1', value: 'remote' }] })
  );
  h.setHandler(async () => {
    throw new ApiError('revision conflict', 409);
  });
  assert.equal(await h.workspace.flush(), false);
  assert.equal(h.workspace.state.conflict, true);
  assert.equal(h.workspace.state.drafts.q1.value, 'local');
  assert.equal(await h.workspace.flush(), false);
  assert.equal(h.sent.length, 1);
  h.setHandler(async (cmd) =>
    attempt({ revision: 2, answers: JSON.parse(cmd.body).answers })
  );
  h.workspace.resolve(true);
  await h.workspace.flush();
  assert.equal(JSON.parse(h.sent[1].body).revision, 1);
  assert.notEqual(h.sent[1].key, h.sent[0].key);
  assert.equal(h.workspace.state.dirty, false);
});

test('choosing authoritative answers discards local edits without a write', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'local');
  h.setServer(
    attempt({ revision: 1, answers: [{ question_id: 'q1', value: 'remote' }] })
  );
  await h.workspace.refresh();
  assert.equal(h.workspace.state.conflict, true);
  h.workspace.resolve(false);
  assert.equal(h.workspace.state.drafts.q1.value, 'remote');
  assert.equal(h.workspace.state.dirty, false);
  assert.equal(h.sent.length, 0);
});

test('known validation rejection can be corrected rather than replaying invalid bytes forever', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'invalid');
  h.setHandler(async () => {
    throw new ApiError('invalid', 422);
  });
  await h.workspace.flush();
  h.workspace.edit('q1', 'corrected');
  h.setHandler(async (cmd) =>
    attempt({ revision: 1, answers: JSON.parse(cmd.body).answers })
  );
  assert.equal(await h.workspace.flush(), true);
  assert.notEqual(h.sent[0].key, h.sent[1].key);
  assert.equal(JSON.parse(h.sent[1].body).answers[0].value, 'corrected');
});

test('manual submit waits for save, requires server confirmation and retries the same submit key', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'answer');
  h.setHandler(async (cmd) => {
    if (cmd.method === 'PATCH')
      return attempt({ revision: 1, answers: JSON.parse(cmd.body).answers });
    if (h.sent.length === 2) throw new ApiError('submit response lost');
    return attempt({
      revision: 1,
      status: 'submitted',
      submission_reason: 'manual',
      answers: [{ question_id: 'q1', value: 'answer' }]
    });
  });
  assert.equal(await h.workspace.submit(), false);
  assert.equal(h.workspace.state.attempt?.status, 'in_progress');
  assert.equal(await h.workspace.submit(), true);
  assert.deepEqual(
    h.sent.map((cmd) => cmd.method),
    ['PATCH', 'POST', 'POST']
  );
  assert.equal(h.sent[1].key, h.sent[2].key);
  assert.equal(h.workspace.editable(), false);
});

test('manual submit does not discard an unsaved draft on save failure', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'answer');
  h.setHandler(async () => {
    throw new ApiError('unavailable');
  });
  assert.equal(await h.workspace.submit(), false);
  assert.equal(h.sent.length, 1);
  assert.equal(h.sent[0].method, 'PATCH');
  assert.equal(h.workspace.state.dirty, true);
});

test('deadline locks input and submits server answers even with an unresolved draft conflict', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'local');
  h.setServer(
    attempt({ revision: 1, answers: [{ question_id: 'q1', value: 'remote' }] })
  );
  await h.workspace.refresh();
  assert.equal(h.workspace.state.conflict, true);
  h.advance(30 * 60 * 1000);
  assert.equal(h.workspace.editable(), false);
  h.setHandler(async () =>
    attempt({
      revision: 1,
      status: 'submitted',
      submission_reason: 'deadline',
      answers: [{ question_id: 'q1', value: 'remote' }]
    })
  );
  assert.equal(await h.workspace.submit(), true);
  assert.equal(h.sent.length, 1);
  assert.equal(h.sent[0].method, 'POST');
  assert.equal(h.workspace.state.drafts.q1.value, 'local');
  assert.match(
    h.workspace.state.issue,
    /belum tersimpan tidak masuk pengumpulan/
  );
});

test('admin close discovered on reconnect never sends unsaved answers', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.setOffline(true);
  h.workspace.edit('q1', 'unsaved');
  h.setServer(attempt({ status: 'submitted', submission_reason: 'closed' }));
  await h.workspace.reconnect();
  assert.equal(h.sent.length, 0);
  assert.equal(h.workspace.editable(), false);
  assert.equal(h.workspace.state.drafts.q1.value, 'unsaved');
});

test('permission denial blocks writes until an authoritative refresh succeeds', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('q1', 'answer');
  h.setHandler(async () => {
    throw new ApiError('forbidden', 403);
  });
  await h.workspace.flush();
  assert.equal(h.workspace.state.blocked, true);
  h.workspace.edit('q1', 'not accepted');
  assert.equal(h.workspace.state.drafts.q1.value, 'answer');
  assert.equal(await h.workspace.submit(), false);
  assert.equal(h.sent.length, 1);
});

test('answer size is measured in UTF-8 bytes and unknown question IDs cannot enter a batch', async (t) => {
  const h = harness();
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.workspace.edit('other-scope', 'answer');
  assert.equal(h.workspace.state.dirty, false);
  h.workspace.edit('q1', 'é'.repeat(32_769));
  assert.match(h.workspace.state.issue, /64 KiB/);
  assert.equal(h.workspace.state.dirty, false);
  h.workspace.edit('q1', 'é'.repeat(32_768));
  assert.equal(h.workspace.state.dirty, true);
});

test('batch envelope over 256 KiB is rejected locally without sending source', async (t) => {
  const initial = attempt();
  initial.questions = Array.from({ length: 4 }, (_, i) => ({
    ...initial.questions[0],
    id: `q${i}`
  }));
  const h = harness(initial);
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  for (const q of initial.questions) h.workspace.edit(q.id, 'x'.repeat(65_536));
  assert.equal(await h.workspace.flush(), false);
  assert.equal(h.sent.length, 0);
  assert.match(h.workspace.state.issue, /256 KiB/);
});

test('a stale read cannot reopen a submitted attempt or erase its saved revision', async (t) => {
  const h = harness(attempt({ revision: 2, status: 'submitted' }));
  t.after(() => h.workspace.dispose());
  await h.workspace.refresh();
  h.setServer(attempt());
  await h.workspace.refresh();
  assert.equal(h.workspace.state.attempt?.status, 'submitted');
  assert.equal(h.workspace.state.attempt?.revision, 2);
});
