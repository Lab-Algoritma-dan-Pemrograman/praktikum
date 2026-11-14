import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, type Principal } from '@algohub/auth-client';
import {
  ElearningClient,
  LessonWorkspace,
  calculateLevel,
  levelProgressPercent,
  command,
  reusableCommand,
  permitted,
  errorMessage,
  type Command,
  type LearningLessonView
} from '../src/index.ts';

function fixtureView(
  overrides: Partial<LearningLessonView['progress']> = {}
): LearningLessonView {
  return {
    lesson: {
      id: 'lesson-1',
      module_id: 'module-1',
      level_id: 'level-1',
      title: 'Program Pertama C',
      sort_order: 1,
      language: 'c',
      explanation: 'Penjelasan dasar C',
      code_example: 'int main(void) { return 0; }',
      starter_code: '// starter',
      hint: 'Gunakan printf',
      xp_reward: 60,
      quiz: {
        question: 'Fungsi utama C?',
        options: ['main', 'start']
      },
      test_cases: [],
      scope: { domain: 'elearning' },
      version: 1
    },
    progress: {
      lesson_id: 'lesson-1',
      module_id: 'module-1',
      level_id: 'level-1',
      user_id: 1,
      status: 'in_progress',
      draft_code: '// starter',
      language: 'c',
      quiz_option: null,
      test_passed: false,
      revision: 0,
      xp_awarded: 0,
      completed_at: null,
      updated_at: '2026-01-01T00:00:00Z',
      ...overrides
    },
    profile: {
      user_id: 1,
      user_name: 'Mahasiswa Fixture',
      xp: 0,
      level: 1,
      next_level_xp: 50,
      streak: 0,
      last_active_date: '',
      completed_lessons: 0,
      games_played: 0,
      perfect_games: 0,
      unlocked_achievements: [],
      timezone: 'Asia/Jakarta',
      timezone_offset: '+07:00',
      updated_at: '2026-01-01T00:00:00Z'
    },
    newly_unlocked: []
  };
}

test('XP level formula matches legacy specification', () => {
  assert.equal(calculateLevel(0), 1);
  assert.equal(calculateLevel(49), 1);
  assert.equal(calculateLevel(50), 2);
  assert.equal(calculateLevel(199), 2);
  assert.equal(calculateLevel(200), 3);
  assert.equal(levelProgressPercent(0), 0);
  assert.equal(levelProgressPercent(25), 50);
});

test('ElearningClient sends same-origin no-store requests with CSRF and Idempotency-Key', async () => {
  const requests: { url: unknown; init: RequestInit }[] = [];
  const client = new ElearningClient(async (url, init) => {
    requests.push({ url, init: init! });
    return Response.json({
      data: fixtureView({ revision: 1 }),
      meta: { request_id: 'req-elearning' }
    });
  });
  const cmd = command('/learning/lessons/lesson-1/progress', 'PUT', {
    revision: 0,
    draft_code: 'int main(void){}',
    language: 'c',
    complete: false
  });
  await client.execute(cmd);
  await client.execute(cmd);
  assert.equal(requests[0].url, '/api/v1/learning/lessons/lesson-1/progress');
  assert.equal(requests[0].init.credentials, 'same-origin');
  assert.equal(requests[0].init.cache, 'no-store');
  assert.equal(
    new Headers(requests[0].init.headers).get('Idempotency-Key'),
    cmd.key
  );
  assert.equal(
    new Headers(requests[1].init.headers).get('Idempotency-Key'),
    cmd.key
  );
  assert.equal(
    reusableCommand(cmd, cmd.path, cmd.method, JSON.parse(cmd.body)),
    cmd
  );
});

test('LessonWorkspace debounces draft edits, reconciles offline drafts and resolves 409 conflicts', async () => {
  let server = fixtureView();
  const sent: Command[] = [];
  let failConflict = false;
  const workspace = new LessonWorkspace(
    {
      lesson: async () => structuredClone(server),
      execute: async <T>(cmd: Command) => {
        sent.push(cmd);
        if (failConflict) throw new ApiError('conflict', 409, 'CONFLICT');
        const body = JSON.parse(cmd.body);
        server = {
          ...server,
          progress: {
            ...server.progress,
            revision: server.progress.revision + 1,
            draft_code: body.draft_code,
            quiz_option: body.quiz_option,
            status: body.complete ? 'completed' : server.progress.status,
            xp_awarded: body.complete ? 60 : server.progress.xp_awarded
          },
          profile: {
            ...server.profile,
            xp: body.complete ? 60 : server.profile.xp
          }
        };
        return structuredClone(server) as T;
      }
    },
    'lesson-1',
    () => {}
  );
  try {
    await workspace.refresh();
    workspace.setOffline(true);
    workspace.editCode('int main(void) { return 1; }');
    workspace.selectQuiz(0);
    assert.equal(await workspace.flush(), false);
    assert.equal(sent.length, 0);

    await workspace.reconnect();
    assert.equal(sent.length, 1);
    assert.equal(workspace.state.dirty, false);
    assert.equal(workspace.state.view?.progress.revision, 1);

    // Simulate remote edit + 409 conflict
    server = fixtureView({
      revision: 2,
      draft_code: '// remote change',
      quiz_option: 1
    });
    failConflict = true;
    workspace.editCode('// local change');
    assert.equal(await workspace.flush(), false);
    assert.equal(workspace.state.conflict, true);
    assert.equal(workspace.state.draftCode, '// local change');

    failConflict = false;
    workspace.resolve(true);
    await workspace.flush();
    assert.equal(workspace.state.conflict, false);
    assert.equal(workspace.state.dirty, false);
    assert.equal(workspace.state.view?.progress.revision, 3);

    const completed = await workspace.complete({
      lessonId: 'lesson-1',
      lessonVersion: 1,
      language: 'c',
      source: '// local change'
    });
    assert.equal(completed?.progress.status, 'completed');
    assert.equal(completed?.profile.xp, 60);
  } finally {
    workspace.dispose();
  }
});

test('code completion requires a current browser attestation and serializes the accepted risk', async () => {
  const server = fixtureView({ draft_code: 'int main(void) { return 0; }' });
  const sent: Command[] = [];
  const workspace = new LessonWorkspace(
    {
      lesson: async () => structuredClone(server),
      execute: async <T>(cmd: Command) => {
        sent.push(cmd);
        return {
          ...server,
          progress: {
            ...server.progress,
            status: 'completed',
            test_passed: true,
            revision: 1,
            xp_awarded: 60
          },
          profile: { ...server.profile, xp: 60 }
        } as T;
      }
    },
    'lesson-1',
    () => {}
  );
  try {
    await workspace.refresh();
    assert.equal(await workspace.complete(), null);
    assert.equal(sent.length, 0);
    assert.match(workspace.state.issue, /Jalankan kode.*browser/);

    assert.equal(
      await workspace.complete({
        lessonId: 'lesson-1',
        lessonVersion: 1,
        language: 'c',
        source: 'stale source'
      }),
      null
    );
    assert.equal(sent.length, 0);

    const completed = await workspace.complete({
      lessonId: 'lesson-1',
      lessonVersion: 1,
      language: 'c',
      source: server.progress.draft_code
    });
    assert.equal(completed?.progress.test_passed, true);
    assert.equal(completed?.profile.xp, 60);
    assert.equal(sent.length, 1);
    const body = JSON.parse(sent[0].body);
    assert.equal(body.browser_test_passed, true);
    assert.equal(body.lesson_version, 1);
    assert.equal(body.draft_code, server.progress.draft_code);
  } finally {
    workspace.dispose();
  }
});

test('permission helper and domain error messages preserve specific error codes', () => {
  const principal = {
    grants: [
      {
        permission: 'learning.content.manage',
        scope: { domain: 'elearning', division: 'pendidikan' }
      }
    ]
  } as Principal;
  assert.equal(
    permitted(principal, 'learning.content.manage', {
      domain: 'elearning',
      division: 'pendidikan',
      institution: 'lab'
    }),
    true
  );
  assert.equal(
    permitted(principal, 'learning.content.manage', {
      domain: 'elearning',
      division: 'k3'
    }),
    false
  );
  assert.match(
    errorMessage(
      new ApiError('Jawaban kuis belum tepat.', 409, 'QUIZ_INCORRECT', 'req-1')
    ),
    /Jawaban kuis belum tepat.*req-1/
  );
  assert.match(
    errorMessage(
      new ApiError('Level masih terkunci.', 403, 'LEVEL_LOCKED', 'req-2')
    ),
    /Level masih terkunci.*req-2/
  );
});
