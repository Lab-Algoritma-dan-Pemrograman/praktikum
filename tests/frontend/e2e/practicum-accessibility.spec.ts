import { randomUUID } from 'node:crypto';
import AxeBuilder from '@axe-core/playwright';
import type { Page, Dialog } from '@playwright/test';
import {
  test,
  expect,
  hosts,
  login,
  api,
  restrictNetwork,
  noBrowserCredentials
} from './helpers';

test('Praktikum account logout still revokes its SSO family', async ({
  page
}) => {
  await login(page, 'portal', 'mahasiswa@example.test');
  await login(page, 'praktikum', 'mahasiswa@example.test');
  await page.locator('.user-menu > summary').click();
  await page.getByRole('button', { name: 'Keluar', exact: true }).click();
  await expect(page).toHaveURL(`${hosts.auth}/logout`);
  for (const origin of [hosts.portal, hosts.praktikum]) {
    await page.goto(origin);
    expect((await api(page, '/v1/auth/me')).status).toBe(401);
  }
});

async function create(page: Page, path: string, body: unknown) {
  const result = await api(
    page,
    `/v1${path}`,
    'POST',
    body,
    true,
    randomUUID()
  );
  expect(result.status).toBeGreaterThanOrEqual(200);
  expect(result.status).toBeLessThan(300);
  return result.body.data;
}
async function accessible(page: Page) {
  await expect(page.getByRole('main')).toHaveCount(1);
  await expect(page.locator('h1')).toHaveCount(1);
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag22a', 'wcag22aa'])
    .analyze();
  expect(
    results.violations.map((v) => ({
      id: v.id,
      impact: v.impact,
      targets: v.nodes.map((n) => n.target)
    }))
  ).toEqual([]);
  const fits = await page.evaluate(
    () => document.documentElement.scrollWidth <= innerWidth + 1
  );
  expect(fits).toBe(true);
}

for (const width of [320, 375, 768, 1024, 1440]) {
  test(`Praktikum authenticated accessibility and keyboard at ${width}px`, async ({
    page: admin,
    browser
  }, info) => {
    test.setTimeout(120_000);
    await login(admin, 'admin');
    const session = await create(admin, '/admin/praktikum/sesi', {
      title: `Praktikum UI synthetic ${randomUUID().slice(0, 6)}`,
      sequence: 22,
      scope: { domain: 'praktikum', institution: 'lab', division: 'pendidikan' }
    });
    const course = await create(admin, '/admin/praktikum/courses', {
      session_id: session.id,
      title: 'Pretest · algoritma dasar',
      kind: 'pretest',
      duration_minutes: 30,
      token_required: false
    });
    const question = await create(admin, '/admin/praktikum/questions', {
      course_id: course.id,
      type: 'essay',
      difficulty: 'easy',
      category: 'modul',
      prompt: 'Jelaskan langkah algoritma untuk menjumlahkan dua bilangan.',
      points: 10,
      starter_code: '',
      language: '',
      reference_answer: 'SYNTHETIC_PRIVATE_REFERENCE',
      rubric: [{ id: 'logic', description: 'Urutan langkah', max_score: 10 }]
    });
    const coding = await create(admin, '/admin/praktikum/questions', {
      course_id: course.id,
      type: 'coding',
      difficulty: 'easy',
      category: 'modul',
      prompt: 'Tulis program penjumlahan sederhana.',
      points: 10,
      starter_code: '/* SYNTHETIC STARTER */',
      language: 'c',
      reference_answer: 'SYNTHETIC_PRIVATE_REFERENCE',
      rubric: [{ id: 'logic', description: 'Penjumlahan', max_score: 10 }]
    });
    const activation = await create(
      admin,
      `/admin/praktikum/sesi/${session.id}/activate`,
      {
        course_id: course.id,
        scope: { ...session.scope, class_id: 'class-101', shift: 1 },
        timer_mode: 'individual',
        student_ids: [1],
        entry_approved_ids: [],
        question_ids: [question.id, coding.id]
      }
    );
    const context = await browser.newContext({
      viewport: { width, height: 900 },
      reducedMotion: 'reduce'
    });
    await restrictNetwork(context);
    const page = await context.newPage();
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    try {
      await login(page, 'praktikum', 'mahasiswa@example.test');
      await expect(
        page.getByRole('heading', { name: session.title, exact: true })
      ).toBeVisible();
      await accessible(page);
      await page.keyboard.press('Tab');
      await expect(
        page.getByRole('link', { name: 'Lewati ke konten' })
      ).toBeFocused();
      await page.keyboard.press('Enter');
      await expect(page.getByRole('main')).toBeFocused();
      await page.getByRole('heading', { level: 1 }).click();
      await page.screenshot({
        path: info.outputPath('sessions.png'),
        fullPage: true
      });
      await page.goto(`${hosts.praktikum}/sesi/${session.id}`);
      const start = page
        .locator(`[data-activation-id="${activation.id}"]`)
        .getByRole('button', { name: 'Mulai / lanjutkan pengerjaan' });
      await expect(start).toBeEnabled();
      await accessible(page);
      await page.evaluate(() => {
        document.documentElement.style.fontSize = '200%';
      });
      await accessible(page);
      await page.evaluate(() => {
        document.documentElement.style.removeProperty('font-size');
      });
      await page.setViewportSize({ width: 900, height: 375 });
      await accessible(page);
      await page.setViewportSize({ width, height: 900 });
      await page.getByRole('heading', { level: 1 }).click();
      await page.screenshot({
        path: info.outputPath('session-detail.png'),
        fullPage: true
      });
      await start.click();
      const answer = page.getByLabel('Jawaban soal 1', { exact: true });
      await expect(answer).toBeEditable();
      let releaseSave!: () => void;
      const saveGate = new Promise<void>((resolve) => {
        releaseSave = resolve;
      });
      const savePattern = '**/api/v1/practicum/attempts/*/answers';
      await page.route(savePattern, async (route) => {
        await saveGate;
        await route.continue();
      });
      let navigationPrompt = false;
      const handlePrompt = async (dialog: Dialog) => {
        navigationPrompt = true;
        await dialog.dismiss();
      };
      page.on('dialog', handlePrompt);
      try {
        await answer.fill(
          'SYNTHETIC_UI_ANSWER: baca dua bilangan, jumlahkan, tampilkan hasil.'
        );
        await page.getByRole('link', { name: 'Soal 2', exact: true }).click();
        await expect(page).toHaveURL((url) =>
          url.hash.startsWith('#question-')
        );
        expect(navigationPrompt).toBe(false);
        await expect(answer).toHaveValue(
          'SYNTHETIC_UI_ANSWER: baca dua bilangan, jumlahkan, tampilkan hasil.'
        );
      } finally {
        releaseSave();
        await page.unroute(savePattern);
        page.off('dialog', handlePrompt);
      }
      await expect(page.getByTestId('save-status')).toContainText(
        'Semua jawaban tersimpan'
      );
      await accessible(page);
      await page.getByRole('heading', { level: 1 }).click();
      await page.screenshot({
        path: info.outputPath('workspace.png'),
        fullPage: true
      });
      const submit = page.getByRole('button', { name: 'Kumpulkan jawaban' });
      await submit.click();
      const dialog = page.getByRole('dialog', { name: 'Kumpulkan sekarang?' });
      await expect(dialog).toBeVisible();
      await expect(
        dialog.getByRole('button', { name: 'Batal', exact: true })
      ).toBeFocused();
      const dialogAxe = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag22a', 'wcag22aa'])
        .analyze();
      expect(dialogAxe.violations.map((v) => v.id)).toEqual([]);
      await page.screenshot({
        path: info.outputPath('submit-dialog.png'),
        fullPage: false
      });
      await page.keyboard.press('Escape');
      await expect(dialog).not.toBeVisible();
      await expect(submit).toBeFocused();
      await page.keyboard.press('Enter');
      await expect(dialog).toBeVisible();
      await expect(
        dialog.getByRole('button', { name: 'Batal', exact: true })
      ).toBeFocused();
      await page.keyboard.press('Tab');
      await expect(
        dialog.getByRole('button', { name: 'Ya, kumpulkan' })
      ).toBeFocused();
      await page.keyboard.press('Enter');
      await expect(
        page.getByText('Pengumpulan dikonfirmasi server', { exact: false })
      ).toBeVisible();
      await expect(page.getByRole('timer')).toHaveCount(0);
      await accessible(page);
      await page.getByRole('link', { name: 'Lihat status nilai' }).click();
      await expect(
        page.getByRole('heading', { name: 'Nilai & riwayat', exact: true })
      ).toBeVisible();
      await expect(
        page.getByText('Belum dipublikasikan').first()
      ).toBeVisible();
      await accessible(page);
      await page.getByRole('heading', { level: 1 }).click();
      await page.screenshot({
        path: info.outputPath('history.png'),
        fullPage: true
      });
      await noBrowserCredentials(page);
      expect(errors).toEqual([]);
    } finally {
      await context.close();
    }
  });
}

test('Praktikum anonymous session deep link survives real login', async ({
  page: admin,
  browser
}) => {
  await login(admin, 'admin');
  const session = await create(admin, '/admin/praktikum/sesi', {
    title: `SSO synthetic ${randomUUID().slice(0, 6)}`,
    sequence: 23,
    scope: { domain: 'praktikum', institution: 'lab', division: 'pendidikan' }
  });
  const course = await create(admin, '/admin/praktikum/courses', {
    session_id: session.id,
    title: 'SSO course',
    kind: 'pretest',
    duration_minutes: 10,
    token_required: false
  });
  const question = await create(admin, '/admin/praktikum/questions', {
    course_id: course.id,
    type: 'essay',
    difficulty: 'easy',
    category: '',
    prompt: 'Synthetic SSO question',
    points: 1,
    starter_code: '',
    language: '',
    reference_answer: '',
    rubric: [{ id: 'logic', description: 'Synthetic', max_score: 1 }]
  });
  await create(admin, `/admin/praktikum/sesi/${session.id}/activate`, {
    course_id: course.id,
    scope: { ...session.scope, class_id: 'class-101', shift: 1 },
    timer_mode: 'individual',
    student_ids: [1],
    entry_approved_ids: [],
    question_ids: [question.id]
  });
  const context = await browser.newContext();
  await restrictNetwork(context);
  const page = await context.newPage();
  try {
    await page.goto(`${hosts.praktikum}/sesi/${session.id}`);
    await page.getByRole('link', { name: 'Masuk', exact: true }).click();
    await page
      .getByLabel('Email atau NIM', { exact: true })
      .fill('mahasiswa@example.test');
    await page.getByLabel('Password', { exact: true }).fill('fixture-password');
    await page.getByRole('button', { name: 'Masuk', exact: true }).click();
    await expect(page).toHaveURL(`${hosts.praktikum}/sesi/${session.id}`);
    await expect(
      page.getByRole('heading', { name: session.title, exact: true })
    ).toBeVisible();
  } finally {
    await context.close();
  }
});
