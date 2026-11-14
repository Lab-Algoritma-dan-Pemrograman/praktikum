import { randomUUID } from 'node:crypto';
import type { Browser, Page } from '@playwright/test';
import type {
  Activation,
  Attempt,
  Course,
  Question,
  Session,
  SessionDetail
} from '@algohub/practicum-client';
import {
  test,
  expect,
  hosts,
  login,
  restrictNetwork,
  noBrowserCredentials
} from './helpers';

// Real local identity, BFF and Go domain API. Only synthetic fixture answers are
// used; tracing/screenshots stay disabled so no answer/token artifacts are made.
async function domain<T>(
  page: Page,
  path: string,
  method = 'GET',
  data?: unknown,
  key = randomUUID()
) {
  return page.evaluate(
    async ({ path, method, data, key }) => {
      const csrf =
        document.cookie
          .split(';')
          .map((x) => x.trim())
          .find((x) => x.startsWith('algohub_csrf='))
          ?.slice(13) ?? '';
      const response = await fetch(`/api/v1${path}`, {
        method,
        cache: 'no-store',
        headers: {
          Accept: 'application/json',
          ...(method !== 'GET'
            ? {
                'Content-Type': 'application/json',
                'X-CSRF-Token': decodeURIComponent(csrf),
                'Idempotency-Key': key
              }
            : {})
        },
        ...(data !== undefined ? { body: JSON.stringify(data) } : {})
      });
      const body = await response.json();
      return {
        status: response.status,
        data: body.data as T,
        code: body.error?.code
      };
    },
    { path, method, data, key }
  );
}
async function mutation<T>(
  page: Page,
  path: string,
  data: unknown,
  method = 'POST'
): Promise<T> {
  const response = await domain<T>(page, path, method, data);
  expect(
    response.status,
    `${method} ${path}: ${response.code ?? ''}`
  ).toBeGreaterThanOrEqual(200);
  expect(
    response.status,
    `${method} ${path}: ${response.code ?? ''}`
  ).toBeLessThan(300);
  return response.data;
}
async function newStudent(browser: Browser, width = 1280) {
  const context = await browser.newContext({
    viewport: { width, height: 900 },
    reducedMotion: 'reduce'
  });
  await restrictNetwork(context);
  const page = await context.newPage();
  await login(page, 'praktikum', 'mahasiswa@example.test');
  return { context, page };
}
async function fixture(admin: Page) {
  const session = await mutation<Session>(admin, '/admin/praktikum/sesi', {
    title: `E2E Praktikum ${randomUUID().slice(0, 8)}`,
    sequence: 10,
    scope: { domain: 'praktikum', institution: 'lab', division: 'pendidikan' }
  });
  const course = await mutation<Course>(admin, '/admin/praktikum/courses', {
    session_id: session.id,
    title: 'Pretest E2E',
    kind: 'pretest',
    duration_minutes: 30,
    token_required: false
  });
  const questions: Question[] = [];
  for (const type of ['essay', 'coding'])
    questions.push(
      await mutation<Question>(admin, '/admin/praktikum/questions', {
        course_id: course.id,
        type,
        difficulty: 'easy',
        category: 'module',
        prompt:
          type === 'essay'
            ? 'Pertanyaan essay fixture'
            : 'Pertanyaan coding fixture',
        points: 10,
        starter_code: type === 'coding' ? '/* simulated fixture only */' : '',
        language: type === 'coding' ? 'c' : '',
        reference_answer: 'Private fixture reference',
        rubric: [
          { id: 'logic', description: 'Ketepatan fixture', max_score: 10 }
        ]
      })
    );
  return { session, course, questions };
}
async function activate(
  admin: Page,
  f: Awaited<ReturnType<typeof fixture>>,
  closesAt?: string
) {
  return mutation<Activation>(
    admin,
    `/admin/praktikum/sesi/${f.session.id}/activate`,
    {
      course_id: f.course.id,
      scope: {
        ...f.session.scope,
        session_id: f.session.id,
        class_id: 'class-101',
        shift: 1
      },
      timer_mode: 'individual',
      student_ids: [1],
      entry_approved_ids: [],
      question_ids: f.questions.map((q) => q.id),
      ...(closesAt ? { closes_at: closesAt } : {})
    }
  );
}
async function openAdmin(page: Page, sessionId: string) {
  await page.goto(`${hosts.admin}/praktikum`);
  await page
    .getByRole('combobox', { name: 'Sesi', exact: true })
    .selectOption(sessionId);
  await expect(
    page.getByRole('heading', { name: 'Activation sesi', exact: true })
  ).toBeVisible();
}
async function start(page: Page, sessionId: string, activationId: string) {
  await page.goto(`${hosts.praktikum}/sesi/${sessionId}`);
  await page
    .locator(`[data-activation-id="${activationId}"]`)
    .getByRole('button', { name: 'Mulai / lanjutkan attempt' })
    .click();
  await expect(page).toHaveURL(/\/attempts\/[^/]+$/);
  await expect(
    page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
  ).toBeEditable();
  return new URL(page.url()).pathname.split('/').at(-1)!;
}
async function assertFits(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1);
  for (const control of await page
    .locator(
      '.practicum button:visible, .practicum textarea:visible, .practicum select:visible'
    )
    .all()) {
    const box = await control.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(-1);
    expect(box!.x + box!.width).toBeLessThanOrEqual(dimensions.viewport + 1);
    expect(box!.height).toBeGreaterThanOrEqual(43);
  }
}

for (const width of [320, 768, 1280]) {
  test(`practicum student and admin full flow at ${width}px`, async ({
    page: admin,
    browser
  }) => {
    test.setTimeout(120_000);
    await admin.setViewportSize({ width, height: 900 });
    await admin.emulateMedia({ reducedMotion: 'reduce' });
    await login(admin, 'admin');
    const f = await fixture(admin);
    await openAdmin(admin, f.session.id);
    await admin.getByLabel('Kelas', { exact: true }).fill('class-101');
    await admin.getByLabel('ID mahasiswa roster').fill('1');
    await admin
      .getByRole('checkbox', { name: /essay.*Pertanyaan essay fixture/ })
      .check();
    await admin
      .getByRole('checkbox', { name: /coding.*Pertanyaan coding fixture/ })
      .check();
    await admin.getByRole('button', { name: 'Aktifkan snapshot' }).click();
    await expect(admin.getByRole('status')).toContainText('Activation dibuat');
    const detail = (
      await domain<SessionDetail>(
        admin,
        `/admin/praktikum/sesi/${f.session.id}`
      )
    ).data;
    const activation = detail.activations[0];
    await assertFits(admin);
    const student = await newStudent(browser, width);
    const page = student.page;
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      const sessionCard = page.getByRole('article').filter({
        has: page.getByRole('heading', { name: f.session.title, exact: true })
      });
      await sessionCard.getByRole('link', { name: 'Buka sesi' }).click();
      await page
        .locator(`[data-activation-id="${activation.id}"]`)
        .getByRole('button', { name: 'Mulai / lanjutkan attempt' })
        .click();
      await expect(page).toHaveURL(/\/attempts\/[^/]+$/);
      const attemptId = new URL(page.url()).pathname.split('/').at(-1)!;
      await expect(
        page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
      ).toBeEditable();
      await page
        .getByLabel(/^Jawaban soal \d+$/, { exact: true })
        .fill('Jawaban fixture tersimpan');
      await page
        .getByLabel(/^Kode soal \d+$/, { exact: true })
        .fill('/* fixture; never executed */');
      await expect(page.getByTestId('save-status')).toContainText(
        'Semua jawaban tersimpan'
      );
      const before = (
        await domain<Attempt>(page, `/practicum/attempts/${attemptId}`)
      ).data;
      expect(before.revision).toBeGreaterThan(0);
      expect(
        before.questions.every(
          (q) => !('reference_answer' in q) && !('rubric' in q)
        )
      ).toBe(true);
      await assertFits(page);
      await page.clock.setFixedTime(new Date('2040-01-01T00:00:00Z'));
      await page.reload();
      await expect(
        page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
      ).toHaveValue('Jawaban fixture tersimpan');
      await expect(
        page.getByLabel(/^Kode soal \d+$/, { exact: true })
      ).toHaveValue('/* fixture; never executed */');
      const resumed = (
        await domain<Attempt>(page, `/practicum/attempts/${attemptId}`)
      ).data;
      expect(resumed.started_at).toBe(before.started_at);
      expect(resumed.deadline_at).toBe(before.deadline_at);
      expect(resumed.revision).toBe(before.revision);
      await expect(page.getByRole('timer')).not.toHaveText('00:00');
      await expect(
        page.getByRole('button', { name: 'Jalankan kode' })
      ).toBeEnabled();
      await page.getByLabel(/^Input standar soal \d+$/).fill('2 3');
      await page.getByRole('button', { name: 'Jalankan kode' }).click();
      await expect(
        page.getByRole('heading', { name: 'Hasil: success' })
      ).toBeVisible();
      await expect(
        page.getByRole('region', { name: /^Hasil runner soal \d+$/ })
      ).toContainText('SIMULASI');
      await page.getByRole('button', { name: 'Kumpulkan jawaban' }).click();
      await page.getByRole('button', { name: 'Ya, kumpulkan' }).focus();
      await page.keyboard.press('Enter');
      await expect(
        page.getByText('Pengumpulan dikonfirmasi server', { exact: false })
      ).toBeVisible();
      await expect(
        page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
      ).not.toBeEditable();
      const submitted = (
        await domain<Attempt>(page, `/practicum/attempts/${attemptId}`)
      ).data;
      expect(submitted.status).toBe('submitted');
      expect(submitted.submission_reason).toBe('manual');
      expect(submitted.score).toBeNull();
      await admin
        .getByRole('button', { name: 'Monitoring dan penilaian', exact: true })
        .click();
      const row = admin.locator(`[data-attempt-id="${attemptId}"]`);
      await expect(row).toContainText('submitted');
      await expect(row).toContainText('2 / 2');
      await assertFits(admin);
      await row
        .getByRole('button', { name: 'Review Mahasiswa Fixture' })
        .click();
      await admin
        .getByRole('button', { name: 'Minta / ulang saran penilaian' })
        .click();
      await expect(
        admin.getByText('Saran simulasi dari fixture.', { exact: false })
      ).toBeVisible();
      await admin.getByLabel('Nilai soal 1', { exact: true }).fill('8');
      await admin.getByLabel('Nilai soal 2', { exact: true }).fill('9');
      await admin
        .getByLabel('Alasan penilaian / perubahan (audit)')
        .fill('Penilaian manual fixture E2E');
      await admin
        .getByRole('checkbox', { name: /Saya telah meninjau/ })
        .check();
      await admin
        .getByRole('button', { name: 'Publikasikan nilai manual' })
        .click();
      await expect(admin.getByRole('status')).toContainText(
        'Nilai dipublikasikan oleh server'
      );
      await assertFits(admin);
      await page.getByRole('link', { name: 'Lihat status nilai' }).click();
      const grade = page.locator(`[data-attempt-id="${attemptId}"]`);
      await expect(grade).toContainText('17 / 20');
      await expect(grade).toContainText('85.0%');
      await grade
        .getByRole('link', { name: 'Lihat jawaban dan feedback' })
        .click();
      await expect(
        page.getByText('Nilai dipublikasikan', { exact: true })
      ).toBeVisible();
      await expect(
        page.getByLabel(/^Kode soal \d+$/, { exact: true })
      ).not.toBeEditable();
      await assertFits(page);
      await noBrowserCredentials(page);
      expect(errors).toEqual([]);
    } finally {
      await student.context.close();
    }
  });
}

test('practicum scoped assistant activation, monitoring, permanent close and scope denial', async ({
  page: admin,
  browser
}) => {
  test.setTimeout(120_000);
  await login(admin, 'admin');
  const outside = await fixture(admin);
  const outsideActivation = await activate(admin, outside);
  const staffContext = await browser.newContext();
  await restrictNetwork(staffContext);
  const staff = await staffContext.newPage();
  const student = await newStudent(browser);
  try {
    await login(staff, 'admin', 'scoped@example.test');
    await openAdmin(staff, 'session-1');
    await expect(
      staff.getByRole('button', { name: 'Definisi dan soal' })
    ).toHaveCount(0);
    await staff.getByLabel('Kelas', { exact: true }).fill('class-outside');
    await expect(
      staff.getByRole('button', { name: 'Aktifkan snapshot' })
    ).toBeDisabled();
    expect(
      (await domain(staff, `/admin/praktikum/sesi/${outside.session.id}`))
        .status
    ).toBe(404);
    expect(
      (
        await domain(
          staff,
          `/admin/praktikum/sesi/${outside.session.id}/courses/${outside.course.id}/close`,
          'POST',
          { activation_id: outsideActivation.id }
        )
      ).status
    ).toBe(404);
    expect(
      (
        await domain<unknown[]>(
          staff,
          `/admin/praktikum/monitoring?session_id=${outside.session.id}`
        )
      ).data
    ).toEqual([]);
    await staff.getByLabel('Kelas', { exact: true }).fill('class-101');
    await staff.getByLabel('ID mahasiswa roster').fill('1');
    await staff.getByLabel('Course activation').selectOption('course-pretest');
    await staff
      .getByLabel('ID soal yang disetujui pengelola bank soal')
      .fill('question-essay, question-coding');
    await staff.getByRole('button', { name: 'Aktifkan snapshot' }).click();
    await expect(staff.getByRole('status')).toContainText('Activation dibuat');
    const activations = (
      await domain<SessionDetail>(staff, '/admin/praktikum/sesi/session-1')
    ).data.activations;
    const activation = activations.find((a) => a.id !== 'activation-1')!;
    expect(activation).toBeDefined();
    const id = await start(student.page, 'session-1', activation.id);
    await student.page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Jawaban sebelum admin close');
    await expect(student.page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    await staff
      .getByRole('button', { name: 'Monitoring dan penilaian', exact: true })
      .click();
    await expect(staff.locator(`[data-attempt-id="${id}"]`)).toContainText(
      'in_progress'
    );
    await student.context.setOffline(true);
    await student.page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Perubahan offline belum diterima');
    await expect(
      student.page.getByText('Offline. Perubahan tetap di memori', {
        exact: false
      })
    ).toBeVisible();
    await staff
      .getByRole('button', { name: 'Activation', exact: true })
      .click();
    const card = staff.locator(`[data-activation-id="${activation.id}"]`);
    await card
      .getByRole('button', { name: 'Tutup activation', exact: true })
      .click();
    await card.getByRole('button', { name: 'Ya, tutup activation' }).click();
    await expect(card.getByRole('heading').first()).toContainText('Ditutup');
    const closePath = `/admin/praktikum/sesi/session-1/courses/course-pretest/close`;
    const key = randomUUID();
    expect(
      (
        await domain(
          staff,
          closePath,
          'POST',
          { activation_id: activation.id },
          key
        )
      ).status
    ).toBe(200);
    expect(
      (
        await domain(
          staff,
          closePath,
          'POST',
          { activation_id: activation.id },
          key
        )
      ).status
    ).toBe(200);
    await student.context.setOffline(false);
    await expect(
      student.page.getByText('Pengumpulan dikonfirmasi server (closed)', {
        exact: false
      })
    ).toBeVisible();
    await expect(
      student.page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).not.toBeEditable();
    await expect(
      student.page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).toHaveValue('Perubahan offline belum diterima');
    const closed = (
      await domain<Attempt>(student.page, `/practicum/attempts/${id}`)
    ).data;
    expect(closed.answers[0].value === 'Jawaban sebelum admin close').toBe(
      true
    );
    expect(closed.submission_reason).toBe('closed');
    expect(
      (await domain(staff, `/admin/praktikum/penilaian/${id}`)).status
    ).toBe(404);
    const studentAdmin = await student.context.newPage();
    await login(studentAdmin, 'admin', 'mahasiswa@example.test');
    const forbidden = await studentAdmin.goto(`${hosts.admin}/praktikum`);
    expect(forbidden?.status()).toBe(403);
  } finally {
    await student.context.close();
    await staffContext.close();
  }
});

test('practicum reconnect resolves real revision conflict, retry keeps one save, and runner failure is explicit', async ({
  page: admin,
  browser
}) => {
  test.setTimeout(120_000);
  await login(admin, 'admin');
  const f = await fixture(admin);
  const activation = await activate(admin, f);
  const student = await newStudent(browser);
  const other = await newStudent(browser);
  try {
    const page = student.page;
    const id = await start(page, f.session.id, activation.id);
    await page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Baseline fixture');
    await expect(page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    const base = (await domain<Attempt>(page, `/practicum/attempts/${id}`))
      .data;
    await student.context.setOffline(true);
    await page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Local fixture correction');
    await mutation(
      other.page,
      `/practicum/attempts/${id}/answers`,
      {
        revision: base.revision,
        answers: [
          { question_id: f.questions[0].id, value: 'Remote fixture correction' }
        ]
      },
      'PATCH'
    );
    await student.context.setOffline(false);
    await expect(
      page.getByRole('heading', { name: 'Jawaban berubah di server' })
    ).toBeVisible();
    await expect(
      page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).toHaveValue('Local fixture correction');
    await page
      .getByRole('button', {
        name: 'Simpan pilihan lokal di atas versi terbaru'
      })
      .click();
    await expect(page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    const requests: string[] = [];
    let lost = true;
    await page.route(
      `**/api/v1/practicum/attempts/${id}/answers`,
      async (route) => {
        requests.push(route.request().headers()['idempotency-key']);
        if (lost) {
          lost = false;
          // Commit through a second browser session for the same user, then
          // lose only the original response. Avoid Node route.fetch: it neither
          // uses Chromium's localhost DNS map nor safely redacts fetch errors.
          const committed = await domain(
            other.page,
            `/practicum/attempts/${id}/answers`,
            'PATCH',
            route.request().postDataJSON(),
            requests.at(-1)!
          );
          expect(committed.status).toBe(200);
          await route.abort('failed');
        } else await route.continue();
      }
    );
    await page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Retry fixture correction');
    await expect(page.getByRole('alert')).toContainText('Koneksi');
    await page
      .getByRole('button', { name: 'Hubungkan ulang / coba lagi' })
      .click();
    await expect(page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    expect(requests.length).toBe(2);
    expect(requests[0]).toBe(requests[1]);
    const runKeys: string[] = [];
    page.on('request', (request) => {
      if (request.url().endsWith(`/attempts/${id}/run`))
        runKeys.push(request.headers()['idempotency-key']);
    });
    await page
      .getByLabel(/^Kode soal \d+$/, { exact: true })
      .fill('/* simulated only */');
    await page
      .getByLabel(/^Input standar soal \d+$/)
      .fill('[fixture:compile_error]');
    await page.getByRole('button', { name: 'Jalankan kode' }).click();
    await expect(
      page.getByRole('heading', { name: 'Hasil: compile_error' })
    ).toBeVisible();
    await page.getByLabel(/^Input standar soal \d+$/).fill('[fixture:outage]');
    await page.getByRole('button', { name: 'Jalankan kode' }).click();
    await expect(
      page.getByRole('heading', { name: 'Hasil: provider_error' })
    ).toBeVisible();
    await expect(
      page.getByRole('region', { name: /^Hasil runner soal \d+$/ })
    ).toContainText('SIMULASI');
    const repeatedRun = page.waitForResponse((response) =>
      response.url().endsWith(`/attempts/${id}/run`)
    );
    await page.getByRole('button', { name: 'Jalankan kode' }).click();
    await repeatedRun;
    await expect(
      page.getByRole('heading', { name: 'Hasil: provider_error' })
    ).toBeVisible();
    expect(runKeys).toHaveLength(3);
    expect(runKeys[2]).not.toBe(runKeys[1]);
    let loseRun = true;
    await page.route(
      `**/api/v1/practicum/attempts/${id}/run`,
      async (route) => {
        if (loseRun) {
          loseRun = false;
          await route.abort('failed');
        } else await route.continue();
      }
    );
    await page.getByRole('button', { name: 'Jalankan kode' }).click();
    await expect(page.getByRole('alert')).toContainText('Koneksi');
    await page.getByRole('button', { name: 'Jalankan kode' }).click();
    await expect(
      page.getByRole('heading', { name: 'Hasil: provider_error' })
    ).toBeVisible();
    expect(runKeys).toHaveLength(5);
    expect(runKeys[3]).toBe(runKeys[4]);
    await expect(page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    await page.reload();
    await expect(
      page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).toHaveValue('Retry fixture correction');
  } finally {
    await student.context.close();
    await other.context.close();
  }
});

test('practicum deadline uses real server closure, not browser time, and refuses late answers', async ({
  page: admin,
  browser
}) => {
  test.setTimeout(90_000);
  await login(admin, 'admin');
  const f = await fixture(admin);
  const student = await newStudent(browser);
  try {
    // Short local-fixture closes_at bound; no server or response mocking.
    const activation = await activate(
      admin,
      f,
      new Date(Date.now() + 15_000).toISOString()
    );
    const id = await start(student.page, f.session.id, activation.id);
    await student.page
      .getByLabel(/^Jawaban soal \d+$/, { exact: true })
      .fill('Saved before real deadline');
    await expect(student.page.getByTestId('save-status')).toContainText(
      'Semua jawaban tersimpan'
    );
    await student.page.clock.setFixedTime(new Date('2001-01-01T00:00:00Z'));
    await expect(
      student.page.getByText('Pengumpulan dikonfirmasi server (deadline)', {
        exact: false
      })
    ).toBeVisible({ timeout: 30_000 });
    await expect(
      student.page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).not.toBeEditable();
    const final = (
      await domain<Attempt>(student.page, `/practicum/attempts/${id}`)
    ).data;
    expect(final.status).toBe('submitted');
    expect(final.submission_reason).toBe('deadline');
    expect(
      (
        await domain(
          student.page,
          `/practicum/attempts/${id}/answers`,
          'PATCH',
          {
            revision: final.revision,
            answers: [{ question_id: f.questions[0].id, value: 'late fixture' }]
          }
        )
      ).status
    ).toBe(409);
    await student.page.reload();
    await expect(
      student.page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).toHaveValue('Saved before real deadline');
    await expect(
      student.page.getByLabel(/^Jawaban soal \d+$/, { exact: true })
    ).not.toBeEditable();
  } finally {
    await student.context.close();
  }
});

test('practicum empty, non-roster, ownership and direct-route authorization remain enforced', async ({
  page: admin,
  browser
}) => {
  await login(admin, 'admin');
  const f = await fixture(admin);
  const activation = await activate(admin, f);
  const student = await newStudent(browser);
  const outsiderContext = await browser.newContext();
  await restrictNetwork(outsiderContext);
  const outsider = await outsiderContext.newPage();
  try {
    const id = await start(student.page, f.session.id, activation.id);
    await login(outsider, 'praktikum', 'asisten@example.test');
    await expect(
      outsider.getByText('Akun ini tidak memiliki penugasan mahasiswa aktif.', {
        exact: false
      })
    ).toBeVisible();
    expect(
      (await domain<unknown[]>(outsider, '/practicum/sessions')).data
    ).toEqual([]);
    await outsider.goto(`${hosts.praktikum}/sesi/${f.session.id}`);
    await expect(outsider.getByRole('alert')).toContainText(
      'Resource tidak ditemukan'
    );
    await outsider.goto(`${hosts.praktikum}/attempts/${id}`);
    await expect(outsider.getByRole('alert')).toContainText(
      'Resource tidak ditemukan'
    );
    expect(
      (await domain(outsider, `/practicum/attempts/${id}/submit`, 'POST', {}))
        .status
    ).toBe(404);
    expect(
      (await domain<unknown[]>(outsider, '/practicum/grades')).data
    ).toEqual([]);
    // A session without an eligible activation is neither listed nor readable.
    const hidden = await fixture(admin);
    expect(
      (await domain(student.page, `/practicum/sessions/${hidden.session.id}`))
        .status
    ).toBe(404);
    await outsider.goto(`${hosts.praktikum}/nilai`);
    await expect(outsider.getByText('Belum ada hasil attempt.')).toBeVisible();
  } finally {
    await student.context.close();
    await outsiderContext.close();
  }
});
