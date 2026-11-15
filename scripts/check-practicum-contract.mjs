import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { setTimeout as delay } from 'node:timers/promises';

// Called only by check-openapi's isolated, loopback, in-memory fixture server.
// No provider credentials, remote URL, real records or source execution are used.
export async function checkPracticumContract({
  call,
  student,
  admin,
  authOrigin,
  adminOrigin,
  cronSecret
}) {
  let keyNumber = 0;
  const key = () => `contract-practicum-${++keyNumber}`;
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
  const closed = { status: 409, errorCode: 'ATTEMPT_CLOSED' };
  const conflict = { status: 409, errorCode: 'CONFLICT' };
  const invalid = { status: 422, errorCode: 'VALIDATION_ERROR' };
  const hidden = { status: 404, errorCode: 'NOT_FOUND' };
  const noPrivateQuestions = (attempt) => {
    for (const question of attempt.questions) {
      for (const field of [
        'reference_answer',
        'rubric',
        'course_id',
        'version',
        'provider_prompt'
      ])
        assert.ok(!(field in question), `Leaked public question ${field}`);
    }
    assert.equal(attempt.timezone, 'Asia/Jakarta');
    assert.equal(attempt.timezone_offset, '+07:00');
    assert.ok(Date.parse(attempt.server_now) >= Date.parse(attempt.started_at));
  };
  const unpublished = (attempt) => {
    assert.equal(attempt.score, null);
    assert.equal(attempt.percentage, null);
    for (const answer of attempt.answers) {
      assert.ok(!('score' in answer));
      assert.ok(!('feedback' in answer));
    }
    noPrivateQuestions(attempt);
  };
  const cron = (options) =>
    call('post', '/v1/internal/practicum/reconcile', {
      origin: null,
      headers: { Authorization: `Bearer ${cronSecret}` },
      ...options
    });

  await request('anonymous', 'get', '/v1/practicum/sessions', {
    status: 401,
    errorCode: 'UNAUTHENTICATED'
  });
  const sessions = data(await s('get', '/v1/practicum/sessions'));
  assert.deepEqual(
    sessions.map((session) => session.id),
    ['session-1']
  );
  assert.deepEqual(data(await a('get', '/v1/practicum/sessions')), []);
  assert.deepEqual(
    data(
      await s('get', '/v1/practicum/sessions', {
        path: '/v1/practicum/sessions?limit=1&offset=1'
      })
    ),
    []
  );
  for (const query of [
    'limit=0',
    'limit=101',
    'offset=-1',
    'offset=1000001',
    'limit=nope'
  ]) {
    await s('get', '/v1/practicum/sessions', {
      path: `/v1/practicum/sessions?${query}`,
      ...invalid,
      parametersValid: false
    });
  }
  const detail = data(
    await s('get', '/v1/practicum/sessions/{id}', {
      path: '/v1/practicum/sessions/session-1'
    })
  );
  assert.equal(detail.activations[0].id, 'activation-1');
  assert.equal(detail.courses[0].id, 'course-pretest');
  for (const activation of detail.activations) {
    for (const field of [
      'student_ids',
      'entry_approved_ids',
      'question_ids',
      'access_token',
      'token_hash'
    ])
      assert.ok(!(field in activation), `Leaked student activation ${field}`);
  }
  await s('get', '/v1/practicum/sessions/{id}', {
    path: '/v1/practicum/sessions/missing',
    ...hidden
  });
  await a('get', '/v1/practicum/sessions/{id}', {
    path: '/v1/practicum/sessions/session-1',
    ...hidden
  });
  assert.deepEqual(data(await s('get', '/v1/practicum/grades')), []);
  assert.deepEqual(data(await s('get', '/v1/admin/praktikum/sesi')), []);
  await s('get', '/v1/admin/praktikum/sesi/{id}', {
    path: '/v1/admin/praktikum/sesi/session-1',
    ...hidden
  });
  await s('get', '/v1/admin/praktikum/questions', {
    path: '/v1/admin/praktikum/questions?course_id=course-pretest',
    ...hidden
  });

  const startInput = { activation_id: 'activation-1' };
  await s('post', '/v1/practicum/attempts', {
    body: startInput,
    headers: { 'X-CSRF-Token': 'wrong-fixture-csrf' },
    status: 403,
    errorCode: 'CSRF_INVALID'
  });
  await s('post', '/v1/practicum/attempts', {
    body: startInput,
    headers: { 'X-CSRF-Token': null },
    parametersValid: false,
    status: 403,
    errorCode: 'CSRF_INVALID'
  });
  for (const value of [null, 'unsafe key', 'x'.repeat(129)]) {
    await s('post', '/v1/practicum/attempts', {
      body: startInput,
      headers: { 'Idempotency-Key': value },
      parametersValid: false,
      ...invalid
    });
  }
  await s('post', '/v1/practicum/attempts', {
    body: { ...startInput, user_id: 9 },
    requestValid: false,
    ...invalid
  });
  await s('post', '/v1/practicum/attempts', {
    rawBody: '{"activation_id":"activation-1"} {}',
    requestValid: false,
    ...invalid
  });
  await s('post', '/v1/practicum/attempts', {
    body: startInput,
    headers: { 'Content-Type': 'text/plain' },
    ...invalid
  });
  await a('post', '/v1/practicum/attempts', { body: startInput, ...hidden });
  await s('post', '/v1/practicum/attempts', {
    body: { activation_id: 'missing' },
    ...hidden
  });
  await s('patch', '/v1/practicum/attempts/{id}/answers', {
    path: '/v1/practicum/attempts/missing/answers',
    body: {
      revision: 0,
      answers: [{ question_id: 'question-essay', value: 'synthetic' }]
    },
    ...hidden
  });
  const startKey = key();
  const attempt = data(
    await s('post', '/v1/practicum/attempts', {
      body: startInput,
      key: startKey
    })
  );
  assert.equal(attempt.user_id, 1);
  assert.equal(attempt.status, 'in_progress');
  assert.equal(attempt.revision, 0);
  unpublished(attempt);
  const activeHistory = data(await s('get', '/v1/practicum/grades'));
  assert.equal(activeHistory[0].status, 'in_progress');
  assert.equal(activeHistory[0].can_open, true);
  assert.equal(activeHistory[0].score, null);
  const attemptPath = `/v1/practicum/attempts/${attempt.attempt_id}`;
  for (const startRetryKey of [startKey, key()]) {
    const retry = data(
      await s('post', '/v1/practicum/attempts', {
        body: startInput,
        key: startRetryKey
      })
    );
    assert.equal(retry.attempt_id, attempt.attempt_id);
    assert.equal(retry.deadline_at, attempt.deadline_at);
  }
  await a('get', '/v1/practicum/attempts/{id}', {
    path: attemptPath,
    ...hidden
  });
  await s('get', '/v1/practicum/attempts/{id}', {
    path: '/v1/practicum/attempts/missing',
    ...hidden
  });
  await s('get', '/v1/admin/praktikum/penilaian/{attemptId}', {
    path: `/v1/admin/praktikum/penilaian/${attempt.attempt_id}`,
    ...hidden
  });

  // Valid >64 KiB body proves the Practicum decoder does not inherit identity's limit.
  const answerInput = {
    revision: attempt.revision,
    answers: [
      {
        question_id: 'question-essay',
        value: `SYNTHETIC_CONTRACT_ANSWER_${'a'.repeat(40000)}`
      },
      {
        question_id: 'question-coding',
        value: `// SYNTHETIC_CONTRACT_SOURCE_${' '.repeat(40000)}`,
        language: 'c'
      }
    ]
  };
  assert.ok(Buffer.byteLength(JSON.stringify(answerInput)) > 65536);
  const answersKey = key();
  const save = (options) =>
    s('patch', '/v1/practicum/attempts/{id}/answers', {
      path: `${attemptPath}/answers`,
      body: answerInput,
      ...options
    });
  const saved = data(await save({ key: answersKey }));
  assert.equal(saved.revision, 1);
  assert.equal(saved.answers.length, 2);
  assert.equal(data(await save({ key: answersKey })).revision, 1);
  await save(conflict);
  await save({
    key: answersKey,
    body: {
      ...answerInput,
      answers: [{ question_id: 'question-essay', value: 'different' }]
    },
    ...conflict
  });
  await save({
    body: {
      revision: 1,
      answers: [{ question_id: 'question-essay', value: 'é'.repeat(32769) }]
    },
    requestValid: false,
    ...invalid
  });
  await save({
    body: {
      revision: 1,
      answers: [{ question_id: 'question-essay', value: 'x'.repeat(262144) }]
    },
    requestValid: false,
    status: 413,
    errorCode: 'BODY_TOO_LARGE'
  });
  await save({
    body: {
      revision: 1,
      answers: [{ question_id: 'outside-snapshot', value: 'synthetic' }]
    },
    ...invalid
  });
  const reloaded = data(
    await s('get', '/v1/practicum/attempts/{id}', { path: attemptPath })
  );
  assert.deepEqual(reloaded.answers, saved.answers);
  assert.equal(reloaded.revision, 1);

  const runner = data(await s('get', '/v1/practicum/runner'));
  assert.equal(runner.available, true);
  assert.equal(runner.simulated, true);
  const cLanguage = runner.languages.find(
    (language) => language.language === 'c'
  );
  assert.ok(cLanguage);
  const runInput = {
    question_id: 'question-coding',
    source: '// SYNTHETIC_RUN_SOURCE; never executed',
    language: 'c',
    version: cLanguage.version,
    stdin: ''
  };
  const run = (options) =>
    s('post', '/v1/practicum/attempts/{id}/run', {
      path: `${attemptPath}/run`,
      body: runInput,
      ...options
    });
  const runKey = key();
  const result = data(await run({ key: runKey }));
  assert.equal(result.status, 'success');
  assert.equal(result.simulated, true);
  assert.equal(
    result.source_hash,
    createHash('sha256').update(runInput.source).digest('hex')
  );
  assert.deepEqual(data(await run({ key: runKey })), result);
  await run({
    key: runKey,
    body: { ...runInput, stdin: 'different' },
    ...conflict
  });
  await run({ body: { ...runInput, version: 'not-allowed' }, ...invalid });
  await run({
    body: { ...runInput, source: 'x'.repeat(65537) },
    requestValid: false,
    ...invalid
  });
  for (const [scenario, expected] of [
    ['compile_error', 'compile_error'],
    ['runtime_error', 'runtime_error'],
    ['timeout', 'timeout'],
    ['outage', 'provider_error']
  ]) {
    const outcome = data(
      await run({ body: { ...runInput, stdin: `[fixture:${scenario}]` } })
    );
    assert.equal(outcome.status, expected);
    assert.equal(outcome.simulated, true);
  }

  // Admit at most 30 runs per fixed minute. Allow one minute-boundary rollover,
  // but require an actual typed 429 and Retry-After rather than assuming one.
  let quotaRejected = false;
  for (let admission = 0; admission < 61; admission++) {
    const admissionResult = await run({ statuses: [200, 429] });
    if (admissionResult.body.error) {
      assert.equal(admissionResult.body.error.code, 'RUNNER_QUOTA_EXCEEDED');
      assert.equal(admissionResult.headers.get('retry-after'), '30');
      quotaRejected = true;
      break;
    }
    assert.equal(admissionResult.body.data.simulated, true);
  }
  assert.ok(quotaRejected, 'Runner admission quota was never enforced');
  // A persisted replay consumes neither a new admission nor another execution.
  assert.deepEqual(data(await run({ key: runKey })), result);

  const monitor = data(
    await a('get', '/v1/admin/praktikum/monitoring', {
      path: '/v1/admin/praktikum/monitoring?session_id=session-1&activation_id=activation-1&limit=1&offset=0'
    })
  );
  assert.equal(monitor.length, 1);
  assert.equal(monitor[0].answer_count, 2);
  assert.equal(monitor[0].attempt_id, attempt.attempt_id);
  assert.ok(!('answers' in monitor[0]));
  assert.deepEqual(data(await s('get', '/v1/admin/praktikum/monitoring')), []);
  const reviewPath = `/v1/admin/praktikum/penilaian/${attempt.attempt_id}`;
  const earlyReview = data(
    await a('get', '/v1/admin/praktikum/penilaian/{attemptId}', {
      path: reviewPath
    })
  );
  assert.equal(earlyReview.grading_job, null);
  assert.ok(
    earlyReview.questions.every(
      (question) => question.reference_answer && question.rubric.length
    )
  );
  noPrivateQuestions(earlyReview.attempt);
  await a('post', '/v1/admin/praktikum/penilaian/bulk', {
    body: { attempt_ids: [attempt.attempt_id] },
    ...conflict
  });

  const submitKey = key();
  const submit = (options) =>
    s('post', '/v1/practicum/attempts/{id}/submit', {
      path: `${attemptPath}/submit`,
      body: {},
      ...options
    });
  const submitted = data(await submit({ key: submitKey }));
  assert.equal(submitted.status, 'submitted');
  assert.equal(submitted.submission_reason, 'manual');
  assert.deepEqual(submitted.answers, saved.answers);
  for (const retryKey of [submitKey, key()])
    assert.equal(
      data(await submit({ key: retryKey })).submitted_at,
      submitted.submitted_at
    );
  await save({ body: { ...answerInput, revision: saved.revision }, ...closed });
  await run(closed);
  assert.equal(data(await save({ key: answersKey })).status, 'submitted');
  assert.equal(
    data(
      await s('post', '/v1/practicum/attempts', {
        body: startInput,
        key: startKey
      })
    ).status,
    'submitted'
  );
  assert.deepEqual(data(await run({ key: runKey })), result);

  const bulkInput = { attempt_ids: [attempt.attempt_id] };
  const bulkKey = key();
  await a('post', '/v1/admin/praktikum/penilaian/bulk', {
    body: { attempt_ids: [attempt.attempt_id, 'missing'] },
    ...hidden
  });
  await a('post', '/v1/admin/praktikum/penilaian/bulk', {
    body: { attempt_ids: Array(26).fill(attempt.attempt_id) },
    requestValid: false,
    ...invalid
  });
  const jobs = data(
    await a('post', '/v1/admin/praktikum/penilaian/bulk', {
      body: bulkInput,
      key: bulkKey
    })
  );
  assert.equal(jobs.length, 1);
  assert.equal(jobs[0].status, 'pending');
  assert.deepEqual(
    data(
      await a('post', '/v1/admin/praktikum/penilaian/bulk', {
        body: bulkInput,
        key: bulkKey
      })
    ),
    jobs
  );
  const processKey = key();
  const processed = data(
    await a('post', '/v1/admin/praktikum/penilaian/{attemptId}/grade', {
      path: `${reviewPath}/grade`,
      body: {},
      key: processKey
    })
  );
  assert.equal(processed.attempt.status, 'submitted');
  assert.equal(processed.attempt.grading_status, 'needs_manual');
  assert.equal(processed.grading_job.simulated, true);
  assert.ok(processed.suggestions.length > 0);
  unpublished(
    data(await s('get', '/v1/practicum/attempts/{id}', { path: attemptPath }))
  );

  const manualInput = {
    grade_revision: processed.attempt.grade_revision,
    grades: processed.questions.map((question) => ({
      question_id: question.id,
      score: question.points - 1,
      feedback: 'SYNTHETIC_PRIVATE_DRAFT_FEEDBACK'
    })),
    publish: false,
    reason: 'Synthetic contract review'
  };
  const manual = (options) =>
    a('put', '/v1/admin/praktikum/penilaian/{attemptId}/manual', {
      path: `${reviewPath}/manual`,
      body: manualInput,
      ...options
    });
  await manual({
    body: { ...manualInput, grades: manualInput.grades.slice(0, 1) },
    ...invalid
  });
  const manualKey = key();
  const draft = data(await manual({ key: manualKey }));
  assert.equal(draft.attempt.grade_revision, 1);
  assert.ok(
    draft.attempt.answers.some(
      (answer) => answer.feedback === 'SYNTHETIC_PRIVATE_DRAFT_FEEDBACK'
    )
  );
  unpublished(
    data(await s('get', '/v1/practicum/attempts/{id}', { path: attemptPath }))
  );
  await manual(conflict);
  await manual({
    key: manualKey,
    body: { ...manualInput, reason: 'Changed payload' },
    ...conflict
  });
  assert.equal(
    data(await manual({ key: manualKey })).attempt.grade_revision,
    1
  );
  const publication = {
    ...manualInput,
    grade_revision: 1,
    publish: true,
    reason: 'Synthetic publish'
  };
  const published = data(
    await manual({ body: publication, key: 'contract-publish' })
  );
  assert.equal(published.attempt.status, 'graded');
  assert.equal(published.attempt.score, 18);
  assert.equal(published.attempt.max_score, 20);
  assert.equal(published.attempt.percentage, 90);
  const publicGrade = data(
    await s('get', '/v1/practicum/attempts/{id}', { path: attemptPath })
  );
  noPrivateQuestions(publicGrade);
  assert.equal(publicGrade.score, 18);
  assert.equal(publicGrade.answers[0].score, 9);
  const afterProcessReplay = data(
    await a('post', '/v1/admin/praktikum/penilaian/{attemptId}/grade', {
      path: `${reviewPath}/grade`,
      body: {},
      key: processKey
    })
  );
  assert.equal(
    afterProcessReplay.attempt.grade_revision,
    published.attempt.grade_revision
  );
  assert.equal(afterProcessReplay.attempt.score, 18);
  const gradeRows = data(
    await s('get', '/v1/practicum/grades', {
      path: '/v1/practicum/grades?limit=1&offset=0'
    })
  );
  assert.equal(gradeRows[0].attempt_id, attempt.attempt_id);
  assert.equal(gradeRows[0].score, 18);

  // Exercise authoring, optimistic versions and one-time activation token handling.
  await a('get', '/v1/admin/praktikum/sesi');
  await a('get', '/v1/admin/praktikum/sesi/{id}', {
    path: '/v1/admin/praktikum/sesi/session-1'
  });
  const sessionInput = {
    title: 'Synthetic contract session',
    sequence: 2,
    scope: { domain: 'praktikum', institution: 'lab', division: 'pendidikan' }
  };
  await s('post', '/v1/admin/praktikum/sesi', {
    body: sessionInput,
    ...hidden
  });
  const sessionKey = key();
  const createdSession = data(
    await a('post', '/v1/admin/praktikum/sesi', {
      body: sessionInput,
      key: sessionKey,
      status: 201
    })
  );
  assert.equal(
    data(
      await a('post', '/v1/admin/praktikum/sesi', {
        body: sessionInput,
        key: sessionKey,
        status: 201
      })
    ).id,
    createdSession.id
  );
  await a('post', '/v1/admin/praktikum/sesi', {
    body: { ...sessionInput, title: 'Changed title' },
    key: sessionKey,
    ...conflict
  });
  const sessionUpdate = {
    version: createdSession.version,
    title: 'Synthetic updated session',
    sequence: 3,
    active: true
  };
  const updatedSession = data(
    await a('patch', '/v1/admin/praktikum/sesi/{id}', {
      path: `/v1/admin/praktikum/sesi/${createdSession.id}`,
      body: sessionUpdate
    })
  );
  assert.equal(updatedSession.version, createdSession.version + 1);
  await a('patch', '/v1/admin/praktikum/sesi/{id}', {
    path: `/v1/admin/praktikum/sesi/${createdSession.id}`,
    body: sessionUpdate,
    ...conflict
  });
  const courseInput = {
    session_id: createdSession.id,
    title: 'Synthetic practical exam',
    kind: 'ujian_praktik',
    duration_minutes: 30,
    token_required: true
  };
  const courseKey = key();
  const course = data(
    await a('post', '/v1/admin/praktikum/courses', {
      body: courseInput,
      key: courseKey,
      status: 201
    })
  );
  assert.equal(
    data(
      await a('post', '/v1/admin/praktikum/courses', {
        body: courseInput,
        key: courseKey,
        status: 201
      })
    ).id,
    course.id
  );
  await a('post', '/v1/admin/praktikum/courses', {
    body: courseInput,
    ...conflict
  });
  const courseUpdate = {
    version: course.version,
    title: 'Synthetic updated exam',
    duration_minutes: 20,
    token_required: true
  };
  assert.equal(
    data(
      await a('patch', '/v1/admin/praktikum/courses/{id}', {
        path: `/v1/admin/praktikum/courses/${course.id}`,
        body: courseUpdate
      })
    ).version,
    2
  );
  await a('patch', '/v1/admin/praktikum/courses/{id}', {
    path: `/v1/admin/praktikum/courses/${course.id}`,
    body: courseUpdate,
    ...conflict
  });
  for (const kind of ['pretest', 'posttest', 'keterampilan']) {
    await a('post', '/v1/admin/praktikum/courses', {
      body: { ...courseInput, kind, title: `Synthetic ${kind}` },
      status: 201
    });
  }
  const questionInput = {
    course_id: course.id,
    type: 'essay',
    difficulty: 'medium',
    category: 'module',
    prompt: 'Synthetic frozen prompt',
    points: 10,
    starter_code: '',
    language: '',
    reference_answer: 'SYNTHETIC_PRIVATE_REFERENCE',
    rubric: [{ id: 'logic', description: 'Synthetic rubric', max_score: 10 }]
  };
  const questionKey = key();
  const question = data(
    await a('post', '/v1/admin/praktikum/questions', {
      body: questionInput,
      key: questionKey,
      status: 201
    })
  );
  assert.equal(
    data(
      await a('post', '/v1/admin/praktikum/questions', {
        body: questionInput,
        key: questionKey,
        status: 201
      })
    ).id,
    question.id
  );
  const questionRows = data(
    await a('get', '/v1/admin/praktikum/questions', {
      path: `/v1/admin/praktikum/questions?course_id=${course.id}&limit=1&offset=0`
    })
  );
  assert.equal(
    questionRows[0].reference_answer,
    questionInput.reference_answer
  );
  assert.deepEqual(
    data(
      await a('get', '/v1/admin/praktikum/questions', {
        path: `/v1/admin/praktikum/questions?course_id=${course.id}&offset=1`
      })
    ),
    []
  );
  await a('post', '/v1/admin/praktikum/questions', {
    body: { ...questionInput, points: 11 },
    ...invalid
  });
  await a('post', '/v1/admin/praktikum/questions', {
    body: { ...questionInput, points: -1 },
    requestValid: false,
    ...invalid
  });
  const activationInput = {
    course_id: course.id,
    scope: { ...createdSession.scope, class_id: 'class-101', shift: 1 },
    timer_mode: 'individual',
    student_ids: [1],
    entry_approved_ids: [],
    question_ids: [question.id]
  };
  const activatePath = `/v1/admin/praktikum/sesi/${createdSession.id}/activate`;
  const activate = (body, options = {}) =>
    a('post', '/v1/admin/praktikum/sesi/{id}/activate', {
      path: activatePath,
      body,
      status: 201,
      ...options
    });
  await activate({ ...activationInput, student_ids: [99999] }, invalid);
  await activate(
    { ...activationInput, question_ids: [question.id, question.id] },
    { ...invalid, requestValid: false }
  );
  const activationKey = key();
  const unapproved = data(
    await activate(activationInput, { key: activationKey })
  );
  assert.ok(unapproved.access_token);
  const replayActivation = data(
    await activate(activationInput, { key: activationKey })
  );
  assert.equal(replayActivation.id, unapproved.id);
  assert.ok(!('access_token' in replayActivation));
  await s('post', '/v1/practicum/attempts', {
    body: { activation_id: unapproved.id, access_token: 'wrong-token' },
    status: 403,
    errorCode: 'FORBIDDEN'
  });
  await s('post', '/v1/practicum/attempts', {
    body: {
      activation_id: unapproved.id,
      access_token: unapproved.access_token
    },
    status: 403,
    errorCode: 'FORBIDDEN'
  });
  const approvedInput = { ...activationInput, entry_approved_ids: [1] };
  const approved = data(await activate(approvedInput));
  const questionUpdate = {
    ...questionInput,
    prompt: 'Synthetic changed authoring prompt',
    version: question.version
  };
  const updatedQuestion = data(
    await a('patch', '/v1/admin/praktikum/questions/{id}', {
      path: `/v1/admin/praktikum/questions/${question.id}`,
      body: questionUpdate
    })
  );
  assert.equal(updatedQuestion.version, 2);
  await a('patch', '/v1/admin/praktikum/questions/{id}', {
    path: `/v1/admin/praktikum/questions/${question.id}`,
    body: questionUpdate,
    ...conflict
  });
  const exam = data(
    await s('post', '/v1/practicum/attempts', {
      body: { activation_id: approved.id, access_token: approved.access_token }
    })
  );
  assert.equal(exam.questions[0].prompt, questionInput.prompt);
  assert.equal(exam.max_score, 10);
  noPrivateQuestions(exam);

  // One class-scoped assistant can review that activation, but not shared definitions
  // or another session/class. Authorization is checked independently of hostname.
  const assistant = (
    await call('post', '/v1/auth/password', {
      origin: adminOrigin,
      body: { email: 'asisten@example.test', password: 'fixture-password' }
    })
  ).cookies;
  const assistantAccess = data(
    await call('get', '/v1/admin/users/{id}/access', {
      path: '/v1/admin/users/2/access',
      cookies: admin,
      origin: adminOrigin
    })
  );
  const assistantScope = {
    domain: 'praktikum',
    institution: 'lab',
    division: 'pendidikan',
    session_id: 'session-1',
    class_id: 'class-101',
    shift: 1
  };
  await call('put', '/v1/admin/users/{id}/access', {
    path: '/v1/admin/users/2/access',
    cookies: admin,
    origin: adminOrigin,
    body: {
      version: assistantAccess.version,
      assignments: [
        { role: 'asisten', division: 'pendidikan', scope: assistantScope }
      ],
      grants: [
        'practicum.session.manage',
        'question.manage',
        'practicum.grade.write'
      ].map((permission) => ({
        permission,
        role: 'asisten',
        division: 'pendidikan',
        scope: assistantScope
      }))
    }
  });
  const scoped = (method, template, options) =>
    a(method, template, { ...options, cookies: assistant });
  await scoped('get', '/v1/admin/praktikum/penilaian/{attemptId}', {
    path: reviewPath
  });
  await scoped('get', '/v1/admin/praktikum/penilaian/{attemptId}', {
    path: `/v1/admin/praktikum/penilaian/${exam.attempt_id}`,
    ...hidden
  });
  await scoped('patch', '/v1/admin/praktikum/sesi/{id}', {
    path: '/v1/admin/praktikum/sesi/session-1',
    body: {
      version: 1,
      title: 'Forbidden scoped edit',
      sequence: 1,
      active: true
    },
    ...hidden
  });
  await scoped('get', '/v1/admin/praktikum/questions', {
    path: '/v1/admin/praktikum/questions?course_id=course-pretest',
    ...hidden
  });
  assert.deepEqual(
    data(
      await scoped('get', '/v1/admin/praktikum/monitoring', {
        path: `/v1/admin/praktikum/monitoring?session_id=${createdSession.id}`
      })
    ),
    []
  );

  const closePath = `/v1/admin/praktikum/sesi/${createdSession.id}/courses/${course.id}/close`;
  const close = (activationID, options) =>
    a('post', '/v1/admin/praktikum/sesi/{id}/courses/{courseId}/close', {
      path: closePath,
      body: { activation_id: activationID },
      ...options
    });
  await close(approved.id, { cookies: assistant, ...hidden });
  const closeKey = key();
  const closedActivation = data(await close(approved.id, { key: closeKey }));
  assert.equal(closedActivation.is_open, false);
  assert.equal(
    data(await close(approved.id, { key: closeKey })).version,
    closedActivation.version
  );
  const closedExam = data(
    await s('get', '/v1/practicum/attempts/{id}', {
      path: `/v1/practicum/attempts/${exam.attempt_id}`
    })
  );
  assert.equal(closedExam.status, 'submitted');
  assert.equal(closedExam.submission_reason, 'closed');
  await close(unapproved.id);
  await s('post', '/v1/practicum/attempts', {
    body: {
      activation_id: unapproved.id,
      access_token: unapproved.access_token
    },
    ...closed
  });

  // Cron rejects cookie sessions and wrong/missing Bearer credentials, without CSRF.
  await cron({ headers: {}, status: 401, errorCode: 'UNAUTHENTICATED' });
  await cron({
    cookies: admin,
    headers: { Authorization: 'Bearer incorrect-synthetic-secret' },
    status: 401,
    errorCode: 'UNAUTHENTICATED'
  });
  await cron({
    wireMethod: 'get',
    status: 405,
    errorCode: 'METHOD_NOT_ALLOWED'
  });
  const expiryActivation = data(
    await activate({
      ...approvedInput,
      closes_at: new Date(Date.now() + 2500).toISOString()
    })
  );
  const expiring = data(
    await s('post', '/v1/practicum/attempts', {
      body: {
        activation_id: expiryActivation.id,
        access_token: expiryActivation.access_token
      }
    })
  );
  assert.equal(expiring.deadline_at, expiryActivation.closes_at);
  await delay(Math.max(0, Date.parse(expiring.deadline_at) - Date.now() + 100));
  const reconciled = data(await cron());
  assert.equal(reconciled.expired, 1);
  assert.equal(reconciled.jobs_processed, 2);
  assert.equal(data(await cron()).expired, 0);
  const expired = data(
    await s('get', '/v1/practicum/attempts/{id}', {
      path: `/v1/practicum/attempts/${expiring.attempt_id}`
    })
  );
  assert.equal(expired.submission_reason, 'deadline');
  assert.equal(expired.status, 'submitted');
  await s('patch', '/v1/practicum/attempts/{id}/answers', {
    path: `/v1/practicum/attempts/${expiring.attempt_id}/answers`,
    body: {
      revision: expired.revision,
      answers: [{ question_id: question.id, value: 'late' }]
    },
    ...closed
  });

  const audit = data(
    await call('get', '/v1/admin/auditlog', {
      cookies: admin,
      origin: adminOrigin,
      path: '/v1/admin/auditlog?limit=100'
    })
  );
  assert.ok(audit.some((event) => event.action.startsWith('practicum.')));
  const serializedAudit = JSON.stringify(audit);
  for (const secret of [
    'SYNTHETIC_CONTRACT_ANSWER',
    'SYNTHETIC_CONTRACT_SOURCE',
    'SYNTHETIC_RUN_SOURCE',
    'SYNTHETIC_PRIVATE_REFERENCE',
    'SYNTHETIC_PRIVATE_DRAFT_FEEDBACK',
    approved.access_token,
    cronSecret
  ]) {
    assert.ok(
      !serializedAudit.includes(secret),
      'Sensitive Practicum content leaked into shared audit log'
    );
  }
}
