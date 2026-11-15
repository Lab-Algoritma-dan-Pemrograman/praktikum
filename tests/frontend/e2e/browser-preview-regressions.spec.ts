import type { BrowserContext, Page } from '@playwright/test';
import { test, expect, hosts, login } from './helpers';

async function pythonPage(page: Page, context: BrowserContext) {
  await context.route(
    'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/**',
    (route) => route.continue()
  );
  await login(page, 'elearning', 'mahasiswa@example.test');
  await page.goto(`${hosts.elearning}/playground`);
  await expect(
    page.getByRole('button', { name: 'Jalankan di browser', exact: true })
  ).toBeEnabled();
  await page
    .getByRole('combobox', { name: 'Bahasa playground', exact: true })
    .selectOption('python');
  await expect(
    page.getByRole('combobox', { name: 'Bahasa playground', exact: true })
  ).toHaveValue('python');
}

async function run(page: Page, source: string) {
  await page.getByLabel('Kode playground', { exact: true }).fill(source);
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
}

const stdout = (page: Page) => page.getByTestId('terminal-scrollback');

test.beforeEach(() => {
  test.skip(
    process.env.ALGOHUB_TEST_PYODIDE !== 'true',
    'Opt-in real pinned Python runtime.'
  );
  test.setTimeout(180000);
});

test('real Python interactive syntax, wait, cancel and isolated rerun', async ({
  page,
  context
}) => {
  await pythonPage(page, context);
  await page.clock.install();
  await page.getByRole('checkbox', { name: /Input interaktif/ }).check();
  await run(
    page,
    'from __future__ import annotations\nprint("input(")\nanswer = input ("Name: ").strip().upper()\nprint(answer)'
  );
  const input = page.getByLabel('Name:', { exact: true });
  await expect(input).toBeVisible({ timeout: 60000 });
  await expect(page.getByTestId('terminal-scrollback')).toContainText('input(');
  await page.clock.fastForward(46000);
  await expect(input).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await input.fill('  Ana  ');
  await page.getByRole('button', { name: 'Kirim input', exact: true }).click();
  await expect(stdout(page)).toContainText('ANA');

  await run(page, 'print("answer" in globals())');
  await expect(stdout(page)).toHaveText('False');
  await run(page, 'print(input("Cancel: "))');
  await expect(page.getByLabel('Cancel:', { exact: true })).toBeVisible();
  await page
    .getByRole('button', { name: 'Hentikan preview', exact: true })
    .click();
  await expect(page.getByLabel('Cancel:', { exact: true })).toHaveCount(0);
  await run(page, 'print(input("Again: ").strip())');
  await page.getByLabel('Again:', { exact: true }).fill('fresh');
  await page.getByRole('button', { name: 'Kirim input', exact: true }).click();
  await expect(stdout(page)).toContainText('fresh');

  await page.getByRole('checkbox', { name: /Input interaktif/ }).uncheck();
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('helper\n');
  await run(page, 'def ask():\n    return input().strip()\nprint(ask())');
  await expect(stdout(page)).toHaveText('helper');
});

test('real Python cold initialization does not spend the execution deadline', async ({
  page,
  context
}) => {
  await pythonPage(page, context);
  await page.clock.install();
  let release!: () => void;
  let requested!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  const seen = new Promise<void>((resolve) => {
    requested = resolve;
  });
  await context.route(
    'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/pyodide.js',
    async (route) => {
      requested();
      await held;
      await route.continue();
    }
  );
  try {
    await run(page, 'print("cold-ready")');
    await seen;
    await page.clock.fastForward(11000);
    await expect(
      page.getByRole('button', { name: 'Hentikan preview', exact: true })
    ).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
  } finally {
    release();
  }
  await expect(stdout(page)).toHaveText('cold-ready', { timeout: 60000 });
});

test('real Python matplotlib renders PNG and clears figures after failed runs', async ({
  page,
  context
}, testInfo) => {
  await pythonPage(page, context);
  await run(
    page,
    'import matplotlib.pyplot as plt\nplt.plot([1, 2, 3], [1, 4, 9], color="#334155", linewidth=2, marker="o", markersize=8)\nplt.xlabel("x")\nplt.ylabel("y")\nplt.title("Synthetic squares")\nplt.show()\nprint("plot-ready")'
  );
  await expect(stdout(page)).toHaveText('plot-ready', { timeout: 150000 });
  const image = page.getByRole('img', {
    name: 'Grafik hasil preview 1',
    exact: true
  });
  await expect(image).toBeVisible();
  await expect
    .poll(() => image.evaluate((el) => (el as HTMLImageElement).naturalWidth))
    .toBeGreaterThan(100);
  await page
    .getByRole('region', { name: 'Preview lokal di browser' })
    .screenshot({ path: testInfo.outputPath('matplotlib.png') });
  await run(page, 'print("clean-success")');
  await expect(stdout(page)).toHaveText('clean-success');
  await expect(
    page.getByRole('img', { name: /Grafik hasil preview/ })
  ).toHaveCount(0);
  await run(
    page,
    'import matplotlib.pyplot as plt\nplt.plot([1, 2], [3, 4])\nraise ValueError("synthetic-failure")'
  );
  await expect(page.getByTestId('terminal-status')).toContainText(
    'Preview browser: runtime_error'
  );
  await expect(
    page.getByRole('img', { name: /Grafik hasil preview/ })
  ).toHaveCount(0);
  await run(page, 'print("clean-error")');
  await expect(stdout(page)).toHaveText('clean-error', { timeout: 60000 });
  await expect(
    page.getByRole('img', { name: /Grafik hasil preview/ })
  ).toHaveCount(0);
});

test('real Python terminal renders stderr, timeout status and input echo distinctly', async ({
  page,
  context
}) => {
  await pythonPage(page, context);
  await page.clock.install();

  // stderr lines carry the danger class contract used by the theme.
  await run(page, 'import sys\nsys.stderr.write("stream-err")');
  const errLine = page
    .getByTestId('terminal-scrollback')
    .locator('.term-line[data-kind="stderr"]')
    .filter({ hasText: 'stream-err' });
  await expect(errLine).toHaveCount(1, { timeout: 60000 });
  // The theme paints stderr with --ah-console-danger; stdout stays neutral ink.
  const stdoutLine = page
    .getByTestId('terminal-scrollback')
    .locator('.term-line[data-kind="stdout"]')
    .first();
  const [stderrColor, stdoutColor] = await Promise.all([
    errLine.evaluate((el) => getComputedStyle(el).color),
    stdoutLine.evaluate((el) => getComputedStyle(el).color).catch(() => null)
  ]);
  expect(stderrColor).toMatch(/^rgb\(/);
  if (stdoutColor) {
    expect(stderrColor).not.toBe(stdoutColor);
  }

  // Input echo keeps its own class, distinct from stdout.
  await page.getByRole('checkbox', { name: /Input interaktif/ }).check();
  await run(page, 'name = input("Echo: ")\nprint(name)');
  const echoInput = page.getByLabel('Echo:', { exact: true });
  await expect(echoInput).toBeVisible({ timeout: 60000 });
  await echoInput.fill('echo-check');
  await page.getByRole('button', { name: 'Kirim input', exact: true }).click();
  await expect(stdout(page)).toContainText('echo-check');
  await expect(
    page
      .getByTestId('terminal-scrollback')
      .locator('.term-line[data-kind="input"]')
      .filter({ hasText: 'Echo: echo-check' })
  ).toHaveCount(1);

  // An infinite loop hits the watchdog and maps to the timeout status.
  await page.getByRole('checkbox', { name: /Input interaktif/ }).uncheck();
  await run(page, 'while True: pass');
  await expect(page.getByTestId('terminal-status')).toContainText(
    'Preview browser: timeout',
    { timeout: 90000 }
  );
});
