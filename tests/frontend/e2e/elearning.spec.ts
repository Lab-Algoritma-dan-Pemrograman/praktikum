import { randomUUID } from 'node:crypto';
import type { Browser, Page } from '@playwright/test';
import type {
  LearningBugHuntQuestion,
  LearningLesson,
  LearningLessonView,
  LearningLevel,
  LearningModule
} from '@algohub/elearning-client';
import {
  test,
  expect,
  hosts,
  login,
  restrictNetwork,
  fetchLocalRoute,
  noBrowserCredentials
} from './helpers';

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
  await login(page, 'elearning', 'mahasiswa@example.test');
  return { context, page };
}

async function assertFits(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1);
  for (const control of await page
    .locator(
      '.elearning button:visible, .elearning textarea:visible, .elearning select:visible'
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
  test(`elearning student and admin full flow at ${width}px`, async ({
    page: admin,
    browser
  }) => {
    test.setTimeout(120_000);
    await admin.setViewportSize({ width, height: 900 });
    await admin.emulateMedia({ reducedMotion: 'reduce' });
    await login(admin, 'admin');

    const level = await mutation<LearningLevel>(
      admin,
      '/admin/elearning/levels',
      {
        title: `Level E2E ${width}px`,
        description: 'Level E2E responsif',
        sort_order: 10 + Math.floor(width / 100),
        access_mode: 'unlocked',
        required_xp: 0,
        scope: {
          domain: 'elearning',
          institution: 'lab',
          division: 'pendidikan'
        }
      }
    );
    const mod = await mutation<LearningModule>(
      admin,
      '/admin/elearning/modules',
      {
        level_id: level.id,
        title: `Modul E2E ${width}px`,
        description: 'Modul E2E responsif',
        sort_order: 1
      }
    );
    const lesson = await mutation<LearningLesson>(
      admin,
      '/admin/elearning/lessons',
      {
        module_id: mod.id,
        title: `Pelajaran E2E ${width}px`,
        sort_order: 1,
        language: 'c',
        explanation: 'Materi E2E tentang fungsi utama C.',
        code_example: 'int main(void) { return 0; }',
        starter_code: '// starter e2e\n',
        solution: 'PRIVATE_E2E_SOLUTION',
        hint: 'Hint E2E: kembalikan nilai 0 dari fungsi main.',
        xp_reward: 60,
        quiz: {
          question: 'Fungsi pembuka program C?',
          options: ['main', 'start'],
          correct_option: 0
        },
        test_cases: [
          {
            description: 'Uji output browser',
            input: '',
            expected_output: 'Halo Browser'
          }
        ]
      }
    );

    await admin.goto(`${hosts.admin}/elearning`);
    await expect(
      admin.getByRole('heading', { name: 'Konfigurasi e-learning' })
    ).toBeVisible();
    await assertFits(admin);

    const student = await newStudent(browser, width);
    const page = student.page;
    const errors: string[] = [];
    let authoritativeRunRequests = 0;
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('request', (request) => {
      if (
        new URL(request.url()).pathname.endsWith(
          '/api/v1/learning/playground/run'
        )
      )
        authoritativeRunRequests++;
    });
    try {
      await expect(
        page.getByRole('heading', { name: 'Kurikulum dan progres belajar' })
      ).toBeVisible();
      await assertFits(page);

      const lessonCard = page.locator(`[data-lesson-id="${lesson.id}"]`);
      await lessonCard.getByRole('link', { name: 'Buka pelajaran' }).click();
      await expect(page).toHaveURL(new RegExp(`/lesson/${lesson.id}$`));
      await expect(
        page.getByRole('heading', { name: lesson.title, exact: true })
      ).toBeVisible();

      await page
        .getByLabel('Kode latihan')
        .fill(
          '#include <stdio.h>\nint main(void) { printf("Halo Browser\\n"); return 0; }'
        );
      await page.getByRole('radio', { name: 'main' }).check();
      await expect(page.getByTestId('lesson-save-status')).toContainText(
        'Draf tersimpan'
      );

      await page.getByRole('button', { name: 'Minta hint' }).click();
      await expect(
        page.getByRole('region', { name: 'Hint pelajaran' })
      ).toContainText('Hint E2E');

      const beforeRun = (
        await domain<LearningLessonView>(page, `/learning/lessons/${lesson.id}`)
      ).data;
      const unverified = await domain(
        page,
        `/learning/lessons/${lesson.id}/progress`,
        'PUT',
        {
          revision: beforeRun.progress.revision,
          draft_code: beforeRun.progress.draft_code,
          language: 'c',
          quiz_option: 0,
          complete: true
        }
      );
      expect(unverified.status).toBe(409);
      expect(unverified.code).toBe('CODE_NOT_VERIFIED');
      expect(
        (
          await domain<LearningLessonView>(
            page,
            `/learning/lessons/${lesson.id}`
          )
        ).data.profile.xp
      ).toBe(beforeRun.profile.xp);

      await page
        .getByRole('button', { name: 'Jalankan & uji di browser' })
        .click();
      await expect(
        page.getByRole('region', { name: 'Hasil testcase browser' })
      ).toContainText('Lulus');
      expect(authoritativeRunRequests).toBe(0);

      if (width === 320) {
        await page
          .getByLabel('Kode latihan')
          .fill(
            '#include <stdio.h>\nint main(void) { printf("Halo Browser\\n"); return 0; }\n'
          );
        await expect(
          page.getByRole('button', { name: /Selesaikan pelajaran/ })
        ).toBeDisabled();
        expect(
          (
            await domain<LearningLessonView>(
              page,
              `/learning/lessons/${lesson.id}`
            )
          ).data.profile.xp
        ).toBe(beforeRun.profile.xp);
        await page
          .getByRole('button', { name: 'Jalankan & uji di browser' })
          .click();
        await expect(
          page.getByRole('region', { name: 'Hasil testcase browser' })
        ).toContainText('Lulus');
        await expect(
          page.getByRole('button', { name: /Selesaikan pelajaran/ })
        ).toBeEnabled();
      }

      await page.getByRole('button', { name: /Selesaikan pelajaran/ }).click();
      await expect(
        page.getByText('Pelajaran selesai dikonfirmasi server', {
          exact: false
        })
      ).toBeVisible();
      const afterComplete = (
        await domain<LearningLessonView>(page, `/learning/lessons/${lesson.id}`)
      ).data;
      expect(afterComplete.progress.status).toBe('completed');
      expect(afterComplete.progress.xp_awarded).toBe(60);

      // Duplicate completion click does not add duplicate XP.
      const xpBeforeRetry = afterComplete.profile.xp;
      await page
        .getByRole('button', {
          name: 'Perbarui penyelesaian (XP sudah diklaim)'
        })
        .click();
      const afterDuplicate = (
        await domain<LearningLessonView>(page, `/learning/lessons/${lesson.id}`)
      ).data;
      expect(afterDuplicate.profile.xp).toBe(xpBeforeRetry);

      await page.getByRole('link', { name: 'Playground' }).click();
      await expect(
        page.getByRole('heading', { name: 'Playground C & Python' })
      ).toBeVisible();
      await page.getByRole('button', { name: 'Jalankan di browser' }).click();
      await expect(page.getByTestId('terminal-status')).toContainText(
        'Preview browser: success'
      );
      expect(authoritativeRunRequests).toBe(0);
      await assertFits(page);

      await page
        .getByRole('link', { name: 'Leaderboard & Achievement' })
        .click();
      await expect(
        page.getByRole('heading', { name: 'Leaderboard & Achievement' })
      ).toBeVisible();
      await expect(
        page.locator('[data-achievement-id="first-lesson"]')
      ).toContainText('Terbuka');
      await expect(page.locator('[data-leaderboard-user="1"]')).toContainText(
        'Mahasiswa Fixture'
      );
      await assertFits(page);

      await admin
        .getByRole('button', { name: 'Monitoring peserta', exact: true })
        .click();
      await expect(admin.locator('[data-monitoring-user="1"]')).toContainText(
        'Mahasiswa Fixture'
      );
      await assertFits(admin);
      await noBrowserCredentials(page);
      expect(errors).toEqual([]);
    } finally {
      await student.context.close();
    }
  });
}

test('browser-trusted code completion is independent of server runner and text-only quiz still completes', async ({
  page: admin,
  browser
}) => {
  await login(admin, 'admin');
  const level = await mutation<LearningLevel>(
    admin,
    '/admin/elearning/levels',
    {
      title: `Text-only ${randomUUID().slice(0, 8)}`,
      description: 'Completion without a code requirement',
      sort_order: 50,
      access_mode: 'unlocked',
      required_xp: 0,
      scope: { domain: 'elearning' }
    }
  );
  const mod = await mutation<LearningModule>(
    admin,
    '/admin/elearning/modules',
    {
      level_id: level.id,
      title: 'Text and quiz',
      description: 'No runner needed',
      sort_order: 1
    }
  );
  const lesson = await mutation<LearningLesson>(
    admin,
    '/admin/elearning/lessons',
    {
      module_id: mod.id,
      title: 'Read and answer',
      sort_order: 1,
      language: 'c',
      explanation:
        'A text-only lesson can be completed without executing code.',
      code_example: '',
      starter_code: '',
      solution: '',
      hint: '',
      xp_reward: 60,
      quiz: {
        question: 'Apakah eksekusi kode diperlukan?',
        options: ['Tidak', 'Ya'],
        correct_option: 0
      },
      test_cases: []
    }
  );
  const student = await newStudent(browser);
  try {
    await student.page.goto(`${hosts.elearning}/lesson/lesson-1`);
    await expect(
      student.page.getByTestId('lesson-run-requirement')
    ).toContainText('Testcase dijalankan di browser');
    await expect(
      student.page.getByRole('button', {
        name: /Selesaikan pelajaran|Perbarui penyelesaian/
      })
    ).toBeDisabled();

    await student.page.goto(`${hosts.elearning}/lesson/${lesson.id}`);
    await expect(student.page.getByLabel('Kode latihan')).toHaveValue('');
    await student.page
      .getByRole('radio', { name: 'Tidak', exact: true })
      .check();
    await student.page
      .getByRole('button', { name: /Selesaikan pelajaran/ })
      .click();
    await expect(
      student.page.getByText('Pelajaran selesai dikonfirmasi server', {
        exact: false
      })
    ).toBeVisible();
    const completed = (
      await domain<LearningLessonView>(
        student.page,
        `/learning/lessons/${lesson.id}`
      )
    ).data;
    expect(completed.progress.draft_code).toBe('');
    expect(completed.progress.status).toBe('completed');
    expect(completed.progress.xp_awarded).toBe(60);
  } finally {
    await student.context.close();
  }
});

test('elearning offline draft reconciliation, 409 conflict resolution, Bug Hunt and scope guards', async ({
  page: admin,
  browser
}) => {
  test.setTimeout(120_000);
  await login(admin, 'admin');
  const bh = await mutation<LearningBugHuntQuestion>(
    admin,
    '/admin/elearning/bug-hunt',
    {
      language: 'c',
      difficulty: 'easy',
      title: `Bug Hunt E2E ${randomUUID().slice(0, 6)}`,
      buggy_code:
        'int main(void) {\n  int a = 1;\n  if (a = 2) return 1;\n  return 0;\n}',
      bug_line: 3,
      explanation: 'Operator assignment = pada if',
      xp_reward: 50,
      active: true,
      scope: { domain: 'elearning', institution: 'lab', division: 'pendidikan' }
    }
  );

  const student = await newStudent(browser);
  const other = await newStudent(browser);
  try {
    const page = student.page;
    await page.goto(`${hosts.elearning}/lesson/lesson-1`);
    await page.getByLabel('Kode latihan').fill('// baseline draft');
    await expect(page.getByTestId('lesson-save-status')).toContainText(
      'Draf tersimpan'
    );
    const base = (
      await domain<LearningLessonView>(page, '/learning/lessons/lesson-1')
    ).data;

    await student.context.setOffline(true);
    await page.getByLabel('Kode latihan').fill('// local offline edit');
    await expect(
      page.getByText('Offline. Perubahan draf tetap di memori', {
        exact: false
      })
    ).toBeVisible();

    await mutation(
      other.page,
      '/learning/lessons/lesson-1/progress',
      {
        revision: base.progress.revision,
        draft_code: '// remote concurrent edit',
        language: 'c',
        quiz_option: 0,
        complete: false
      },
      'PUT'
    );

    await student.context.setOffline(false);
    await expect(
      page.getByRole('heading', { name: 'Draf berubah di server' })
    ).toBeVisible();
    await page
      .getByRole('button', { name: 'Simpan draf lokal di atas versi terbaru' })
      .click();
    await expect(page.getByTestId('lesson-save-status')).toContainText(
      'Draf tersimpan'
    );

    await page.goto(`${hosts.elearning}/bug-hunt`);
    await page.getByLabel('Pilih tantangan Bug Hunt').selectOption(bh.id);
    await page.getByLabel('Baris bug', { exact: true }).fill('3');
    await page
      .getByLabel('Penjelasan bug', { exact: true })
      .fill('Operator = seharusnya == pada kondisi if');
    await page.getByRole('button', { name: 'Kirim analisis Bug Hunt' }).click();
    await expect(page.getByRole('status').first()).toContainText('Tepat!');

    //Locked level-3 lesson cannot be opened or modified by student.
    expect((await domain(page, '/learning/lessons/lesson-3')).status).toBe(403);
    expect((await domain(page, '/admin/elearning/kurikulum')).status).toBe(404);
  } finally {
    await student.context.close();
    await other.context.close();
  }
});
