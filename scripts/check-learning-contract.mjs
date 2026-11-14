import assert from 'node:assert/strict';

export async function checkLearningContract({
  call,
  student,
  admin,
  authOrigin,
  adminOrigin
}) {
  let keyNumber = 0;
  const key = () => `contract-learning-${++keyNumber}`;
  const request = (actor, method, template, options = {}) =>
    call(method, template, {
      cookies: actor === 'admin' ? admin : actor === 'student' ? student : [],
      origin: actor === 'admin' ? adminOrigin : authOrigin,
      ...options,
      headers: {
        ...(method === 'get'
          ? {}
          : { 'Idempotency-Key': options.key ?? key() }),
        ...options.headers
      }
    });
  const s = (method, template, options) =>
    request('student', method, template, options);
  const a = (method, template, options) =>
    request('admin', method, template, options);
  const data = (result) => result.body.data;
  const conflict = { status: 409, errorCode: 'CONFLICT' };
  const invalid = { status: 422, errorCode: 'VALIDATION_ERROR' };
  const hidden = { status: 404, errorCode: 'NOT_FOUND' };
  const locked = { status: 403, errorCode: 'LEVEL_LOCKED' };

  await request('anonymous', 'get', '/v1/learning/levels', {
    status: 401,
    errorCode: 'UNAUTHENTICATED'
  });

  const overview = data(await s('get', '/v1/learning/levels'));
  assert.equal(overview.profile.xp, 0);
  assert.equal(overview.profile.level, 1);
  assert.equal(overview.profile.timezone, 'Asia/Jakarta');
  assert.equal(overview.profile.timezone_offset, '+07:00');
  assert.equal(overview.runner.available, true);
  assert.equal(overview.runner.simulated, true);
  assert.deepEqual(
    overview.levels.map((l) => [l.id, l.unlocked]),
    [
      ['level-1', true],
      ['level-2', false],
      ['level-3', false]
    ]
  );
  for (const lvl of overview.levels) {
    for (const mod of lvl.modules ?? []) {
      for (const les of mod.lessons ?? []) {
        assert.ok(!('solution' in les), 'Leaked lesson solution');
        assert.ok(
          !les.quiz || !('correct_option' in les.quiz),
          'Leaked quiz correct_option'
        );
      }
    }
  }

  const mod1 = data(
    await s('get', '/v1/learning/modules/{id}', {
      path: '/v1/learning/modules/module-1'
    })
  );
  assert.equal(mod1.id, 'module-1');
  assert.equal(mod1.lessons.length, 1);
  assert.equal(mod1.lessons[0].status, 'not_started');
  await s('get', '/v1/learning/modules/{id}', {
    path: '/v1/learning/modules/module-2',
    ...locked
  });
  await s('get', '/v1/learning/modules/{id}', {
    path: '/v1/learning/modules/module-3',
    ...locked
  });
  await s('get', '/v1/learning/modules/{id}', {
    path: '/v1/learning/modules/missing',
    ...hidden
  });

  const les1 = data(
    await s('get', '/v1/learning/lessons/{id}', {
      path: '/v1/learning/lessons/lesson-1'
    })
  );
  assert.equal(les1.lesson.id, 'lesson-1');
  assert.equal(les1.progress.status, 'in_progress');
  assert.equal(les1.progress.revision, 0);
  assert.ok(les1.lesson.version >= 1);
  assert.ok(!('solution' in les1.lesson));
  await s('get', '/v1/learning/lessons/{id}', {
    path: '/v1/learning/lessons/lesson-2',
    ...locked
  });
  await s('get', '/v1/learning/lessons/{id}', {
    path: '/v1/learning/lessons/lesson-3',
    ...locked
  });

  const draftInput = {
    revision: 0,
    draft_code: '#include <stdio.h>\nint main(void) { return 0; }\n',
    language: 'c',
    quiz_option: 1,
    complete: false
  };
  const draftKey = key();
  const savedDraft = data(
    await s('put', '/v1/learning/lessons/{id}/progress', {
      path: '/v1/learning/lessons/lesson-1/progress',
      body: draftInput,
      key: draftKey
    })
  );
  assert.equal(savedDraft.progress.revision, 1);
  assert.equal(savedDraft.progress.status, 'in_progress');
  assert.equal(savedDraft.profile.xp, 0);
  assert.equal(
    data(
      await s('put', '/v1/learning/lessons/{id}/progress', {
        path: '/v1/learning/lessons/lesson-1/progress',
        body: draftInput,
        key: draftKey
      })
    ).progress.revision,
    1
  );
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-1/progress',
    body: { ...draftInput, draft_code: '// changed' },
    key: draftKey,
    ...conflict
  });
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-1/progress',
    body: draftInput,
    ...conflict
  });
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-1/progress',
    body: {
      revision: 1,
      draft_code: draftInput.draft_code,
      language: 'c',
      quiz_option: 2,
      complete: true
    },
    status: 409,
    errorCode: 'QUIZ_INCORRECT'
  });

  const completeInput = {
    revision: 1,
    draft_code:
      '#include <stdio.h>\nint main(void) { printf("Halo Algohub\\n"); return 0; }\n',
    language: 'c',
    quiz_option: 0,
    complete: true,
    browser_test_passed: true,
    lesson_version: les1.lesson.version
  };
  const completeKey = key();
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-1/progress',
    body: {
      ...completeInput,
      browser_test_passed: false,
      lesson_version: 0
    },
    status: 409,
    errorCode: 'CODE_NOT_VERIFIED'
  });
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-1/progress',
    body: { ...completeInput, lesson_version: les1.lesson.version - 1 },
    ...conflict
  });
  const cVersion = overview.runner.languages.find(
    (l) => l.language === 'c'
  ).version;
  const completed = data(
    await s('put', '/v1/learning/lessons/{id}/progress', {
      path: '/v1/learning/lessons/lesson-1/progress',
      body: completeInput,
      key: completeKey
    })
  );
  assert.equal(completed.progress.status, 'completed');
  assert.equal(completed.progress.xp_awarded, 60);
  assert.equal(completed.profile.xp, 60);
  assert.equal(completed.profile.level, 2);
  assert.equal(completed.profile.streak, 1);
  assert.equal(completed.profile.completed_lessons, 1);
  assert.ok(completed.newly_unlocked.some((a) => a.id === 'first-lesson'));
  assert.ok(completed.newly_unlocked.some((a) => a.id === 'level-1-complete'));

  // Replaying with the same key or completing again with a new key never grants duplicate XP.
  const replayCompleted = data(
    await s('put', '/v1/learning/lessons/{id}/progress', {
      path: '/v1/learning/lessons/lesson-1/progress',
      body: completeInput,
      key: completeKey
    })
  );
  assert.equal(replayCompleted.profile.xp, 60);
  const secondComplete = data(
    await s('put', '/v1/learning/lessons/{id}/progress', {
      path: '/v1/learning/lessons/lesson-1/progress',
      body: { ...completeInput, revision: completed.progress.revision }
    })
  );
  assert.equal(secondComplete.profile.xp, 60);
  assert.equal(secondComplete.profile.completed_lessons, 1);
  assert.equal(secondComplete.newly_unlocked.length, 0);

  // Level 2 (auto, required_xp=60) is now unlocked; Level 3 (locked) remains locked.
  await s('get', '/v1/learning/modules/{id}', {
    path: '/v1/learning/modules/module-2'
  });
  await s('get', '/v1/learning/lessons/{id}', {
    path: '/v1/learning/lessons/lesson-2'
  });
  await s('put', '/v1/learning/lessons/{id}/progress', {
    path: '/v1/learning/lessons/lesson-3/progress',
    body: completeInput,
    ...locked
  });

  const runInput = {
    lesson_id: 'lesson-1',
    source: '// SYNTHETIC_LEARNING_RUN_SOURCE',
    language: 'c',
    version: cVersion,
    stdin: '1 2'
  };
  const runKey = key();
  const runResp = data(
    await s('post', '/v1/learning/playground/run', {
      body: runInput,
      key: runKey
    })
  );
  assert.equal(runResp.result.status, 'success');
  assert.equal(runResp.result.simulated, true);
  assert.equal(runResp.all_passed, true);
  assert.equal(runResp.test_results.length, 1);
  assert.deepEqual(
    data(
      await s('post', '/v1/learning/playground/run', {
        body: runInput,
        key: runKey
      })
    ),
    runResp
  );
  await s('post', '/v1/learning/playground/run', {
    body: { ...runInput, version: 'invalid-version' },
    ...invalid
  });

  let runQuotaRejected = false;
  for (let i = 0; i < 61; i++) {
    const res = await s('post', '/v1/learning/playground/run', {
      body: { ...runInput, lesson_id: '' },
      statuses: [200, 429]
    });
    if (res.body.error) {
      assert.equal(res.body.error.code, 'RUNNER_QUOTA_EXCEEDED');
      assert.equal(res.headers.get('retry-after'), '30');
      runQuotaRejected = true;
      break;
    }
  }
  assert.ok(runQuotaRejected, 'Learning runner quota was never enforced');

  const hintKey = key();
  const hintResp = data(
    await s('post', '/v1/learning/hints', {
      body: { lesson_id: 'lesson-1', draft_code: '// stuck' },
      key: hintKey
    })
  );
  assert.equal(hintResp.lesson_id, 'lesson-1');
  assert.ok(!hintResp.hint.includes('PRIVATE_SOLUTION_LESSON_1_PRINTF'));
  assert.deepEqual(
    data(
      await s('post', '/v1/learning/hints', {
        body: { lesson_id: 'lesson-1', draft_code: '// stuck' },
        key: hintKey
      })
    ),
    hintResp
  );
  await s('post', '/v1/learning/hints', {
    body: { lesson_id: 'lesson-3' },
    ...locked
  });
  let hintQuotaRejected = false;
  for (let i = 0; i < 22; i++) {
    const res = await s('post', '/v1/learning/hints', {
      body: { lesson_id: 'lesson-1' },
      statuses: [200, 429]
    });
    if (res.body.error) {
      assert.equal(res.body.error.code, 'HINT_QUOTA_EXCEEDED');
      assert.equal(res.headers.get('retry-after'), '60');
      hintQuotaRejected = true;
      break;
    }
  }
  assert.ok(hintQuotaRejected, 'Learning hint quota was never enforced');

  const bhView = data(await s('get', '/v1/learning/bug-hunt'));
  assert.equal(bhView.questions.length, 2);
  assert.equal(bhView.weekly_used, 0);
  assert.equal(bhView.weekly_limit, 3);
  for (const q of bhView.questions) {
    assert.ok(!('bug_line' in q), 'Leaked bug_line');
    assert.ok(!('explanation' in q), 'Leaked bug explanation');
  }

  const wrongAttempt = data(
    await s('post', '/v1/learning/bug-hunt/attempts', {
      body: {
        question_id: 'bughunt-c-1',
        selected_line: 1,
        explanation: 'Tebakan baris belum tepat'
      }
    })
  );
  assert.equal(wrongAttempt.attempt.correct, false);
  assert.equal(wrongAttempt.attempt.xp_earned, 0);
  assert.equal(wrongAttempt.profile.xp, 60);

  const rightKey = key();
  const rightInput = {
    question_id: 'bughunt-c-1',
    selected_line: 4,
    explanation: 'Operator = seharusnya == pada kondisi if'
  };
  const rightAttempt = data(
    await s('post', '/v1/learning/bug-hunt/attempts', {
      body: rightInput,
      key: rightKey
    })
  );
  assert.equal(rightAttempt.attempt.correct, true);
  assert.equal(rightAttempt.attempt.xp_earned, 50);
  assert.equal(rightAttempt.profile.xp, 110);
  assert.ok(rightAttempt.newly_unlocked.some((a) => a.id === 'bug-hunter'));
  assert.ok(rightAttempt.newly_unlocked.some((a) => a.id === 'xp-100'));
  assert.equal(
    data(
      await s('post', '/v1/learning/bug-hunt/attempts', {
        body: rightInput,
        key: rightKey
      })
    ).profile.xp,
    110
  );

  // Solving the same Bug Hunt question again awards 0 extra XP and consumes the 3rd weekly play.
  const repeatSolved = data(
    await s('post', '/v1/learning/bug-hunt/attempts', {
      body: rightInput
    })
  );
  assert.equal(repeatSolved.attempt.correct, true);
  assert.equal(repeatSolved.attempt.xp_earned, 0);
  assert.equal(repeatSolved.profile.xp, 110);

  await s('post', '/v1/learning/bug-hunt/attempts', {
    body: {
      question_id: 'bughunt-py-1',
      selected_line: 3,
      explanation: 'Melebihi batas mingguan'
    },
    status: 409,
    errorCode: 'WEEKLY_LIMIT_REACHED'
  });

  const achievements = data(await s('get', '/v1/learning/achievements'));
  assert.equal(achievements.profile.xp, 110);
  assert.equal(
    achievements.achievements.filter((item) => item.unlocked).length,
    4
  );

  const leaderboard = data(await s('get', '/v1/learning/leaderboard'));
  assert.equal(leaderboard.length, 1);
  assert.equal(leaderboard[0].rank, 1);
  assert.equal(leaderboard[0].xp, 110);
  assert.equal(leaderboard[0].completed_lessons, 1);

  assert.deepEqual(data(await s('get', '/v1/admin/elearning/kurikulum')), []);
  assert.deepEqual(data(await s('get', '/v1/admin/elearning/bug-hunt')), []);
  assert.deepEqual(data(await s('get', '/v1/admin/elearning/monitoring')), []);
  const scope = {
    domain: 'elearning',
    institution: 'lab',
    division: 'pendidikan'
  };
  await s('post', '/v1/admin/elearning/levels', {
    body: {
      title: 'Forbidden student level',
      sort_order: 4,
      access_mode: 'unlocked',
      required_xp: 0,
      scope
    },
    ...hidden
  });

  const adminTree = data(await a('get', '/v1/admin/elearning/kurikulum'));
  assert.equal(adminTree.length, 3);
  assert.equal(
    adminTree[0].modules[0].lessons[0].solution,
    'PRIVATE_SOLUTION_LESSON_1_PRINTF'
  );

  const levelCreate = {
    title: 'Level 4 — Struktur Data',
    description: 'Array dan struct dasar.',
    sort_order: 4,
    access_mode: 'auto',
    required_xp: 200,
    scope
  };
  const levelKey = key();
  const createdLevel = data(
    await a('post', '/v1/admin/elearning/levels', {
      body: levelCreate,
      key: levelKey,
      status: 201
    })
  );
  assert.equal(
    data(
      await a('post', '/v1/admin/elearning/levels', {
        body: levelCreate,
        key: levelKey,
        status: 201
      })
    ).id,
    createdLevel.id
  );
  const levelUpdate = {
    version: createdLevel.version,
    title: 'Level 4 — Struktur Data Terpadu',
    description: 'Diperbarui.',
    sort_order: 4,
    access_mode: 'unlocked',
    required_xp: 200
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/elearning/levels/{id}', {
        path: `/v1/admin/elearning/levels/${createdLevel.id}`,
        body: levelUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/elearning/levels/{id}', {
    path: `/v1/admin/elearning/levels/${createdLevel.id}`,
    body: levelUpdate,
    ...conflict
  });

  const modCreate = {
    level_id: createdLevel.id,
    title: 'Modul 4 — Array',
    description: 'Indeks dan iterasi array.',
    sort_order: 1
  };
  const createdMod = data(
    await a('post', '/v1/admin/elearning/modules', {
      body: modCreate,
      status: 201
    })
  );
  const modUpdate = {
    version: createdMod.version,
    title: 'Modul 4 — Array 1D',
    description: 'Diperbarui.',
    sort_order: 1
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/elearning/modules/{id}', {
        path: `/v1/admin/elearning/modules/${createdMod.id}`,
        body: modUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/elearning/modules/{id}', {
    path: `/v1/admin/elearning/modules/${createdMod.id}`,
    body: modUpdate,
    ...conflict
  });

  const lessonCreate = {
    module_id: createdMod.id,
    title: 'Deklarasi Array C',
    sort_order: 1,
    language: 'c',
    explanation: 'Array menyimpan elemen bertipe sama secara berurutan.',
    code_example: 'int angka[3] = {1, 2, 3};',
    starter_code: '#include <stdio.h>\nint main(void) { return 0; }\n',
    solution: 'SYNTHETIC_PRIVATE_LESSON_SOLUTION',
    hint: 'Gunakan kurung siku untuk ukuran array.',
    xp_reward: 60,
    quiz: {
      question: 'Indeks pertama array di C adalah?',
      options: ['0', '1'],
      correct_option: 0
    },
    test_cases: [
      { description: 'Output array', input: '', expected_output: 'SIMULATED' }
    ]
  };
  const createdLesson = data(
    await a('post', '/v1/admin/elearning/lessons', {
      body: lessonCreate,
      status: 201
    })
  );
  const lessonUpdate = {
    ...lessonCreate,
    version: createdLesson.version,
    title: 'Deklarasi dan Akses Array C'
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/elearning/lessons/{id}', {
        path: `/v1/admin/elearning/lessons/${createdLesson.id}`,
        body: lessonUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/elearning/lessons/{id}', {
    path: `/v1/admin/elearning/lessons/${createdLesson.id}`,
    body: lessonUpdate,
    ...conflict
  });

  const adminBugs = data(await a('get', '/v1/admin/elearning/bug-hunt'));
  assert.ok(adminBugs.length >= 2);
  assert.equal(adminBugs[0].bug_line, 4);

  const bhCreate = {
    language: 'c',
    difficulty: 'hard',
    title: 'Off-by-one Array',
    buggy_code: 'int a[2] = {1, 2};\nprintf("%d", a[2]);',
    bug_line: 2,
    explanation: 'SYNTHETIC_PRIVATE_BUGHUNT_EXPLANATION',
    xp_reward: 90,
    active: true,
    scope
  };
  const createdBug = data(
    await a('post', '/v1/admin/elearning/bug-hunt', {
      body: bhCreate,
      status: 201
    })
  );
  const bhUpdate = {
    ...bhCreate,
    version: createdBug.version,
    title: 'Off-by-one Indeks Array'
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/elearning/bug-hunt/{id}', {
        path: `/v1/admin/elearning/bug-hunt/${createdBug.id}`,
        body: bhUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/elearning/bug-hunt/{id}', {
    path: `/v1/admin/elearning/bug-hunt/${createdBug.id}`,
    body: bhUpdate,
    ...conflict
  });

  const monitoring = data(await a('get', '/v1/admin/elearning/monitoring'));
  assert.equal(monitoring.length, 1);
  assert.equal(monitoring[0].user_id, 1);
  assert.equal(monitoring[0].xp, 110);
  assert.equal(monitoring[0].completed_lessons, 1);
  assert.equal(monitoring[0].games_played, 3);
  assert.ok(!('draft_code' in monitoring[0]));

  const audit = data(
    await call('get', '/v1/admin/auditlog', {
      cookies: admin,
      origin: adminOrigin,
      path: '/v1/admin/auditlog?limit=100'
    })
  );
  assert.ok(audit.some((event) => event.action.startsWith('learning.')));
  const serializedAudit = JSON.stringify(audit);
  for (const secret of [
    'PRIVATE_SOLUTION_LESSON_1_PRINTF',
    'SYNTHETIC_PRIVATE_LESSON_SOLUTION',
    'SYNTHETIC_PRIVATE_BUGHUNT_EXPLANATION',
    'SYNTHETIC_LEARNING_RUN_SOURCE'
  ]) {
    assert.ok(
      !serializedAudit.includes(secret),
      'Sensitive E-learning content leaked into shared audit log'
    );
  }
}
