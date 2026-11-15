import { test, expect, hosts, login } from './helpers';

test('browser C preview is lazy, isolated, reusable, bounded and never calls the runner API', async ({
  page
}) => {
  test.setTimeout(120000);
  const runtimeRequests: string[] = [];
  const serverRuns: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/runtimes/'))
      runtimeRequests.push(request.url());
    if (
      request.method() === 'POST' &&
      /\/api\/v1\/.*\/run$/.test(new URL(request.url()).pathname)
    )
      serverRuns.push(request.url());
  });
  await login(page, 'elearning', 'mahasiswa@example.test');
  await page.goto(`${hosts.elearning}/playground`);
  await expect(
    page.getByRole('button', { name: 'Jalankan di browser', exact: true })
  ).toBeEnabled();
  expect(runtimeRequests).toEqual([]);
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill(
      '#include <stdio.h>\nint main(void) { int n; scanf("%d", &n); printf("hasil:%d\\n", n + 2); return 0; }'
    );
  await expect
    .poll(() =>
      runtimeRequests.some((url) => url.includes('/runtimes/v3/frame.html'))
    )
    .toBe(true);
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('40\n');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('hasil:42', {
    timeout: 60000
  });
  console.log(
    'preview.metric.first_c_ms',
    Number(
      (await page.getByTestId('preview-duration').textContent())?.match(
        /^(\d+)/
      )?.[1]
    )
  );
  console.log(
    'preview.metric.initial_runtime_requests',
    runtimeRequests.length
  );
  expect(serverRuns).toEqual([]);
  const frame = page
    .frames()
    .find((candidate) => candidate.url().endsWith('/runtimes/v3/frame.html'));
  expect(frame).toBeDefined();
  expect(
    await frame!.evaluate(() => {
      try {
        return document.cookie;
      } catch (error) {
        return (error as Error).name;
      }
    })
  ).toBe('SecurityError');
  await expect(
    page.locator('iframe[title="Isolated browser code preview"]')
  ).toHaveAttribute('sandbox', 'allow-scripts');
  const assetCount = runtimeRequests.length;
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('5\n');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('hasil:7');
  expect(runtimeRequests.length).toBe(assetCount);
  const secondRunMs = Number(
    (await page.getByTestId('preview-duration').textContent())?.match(
      /^(\d+)/
    )?.[1]
  );
  console.log('preview.metric.second_c_ms', secondRunMs);
  console.log(
    'preview.metric.second_run_asset_requests',
    runtimeRequests.length - assetCount
  );
  expect(secondRunMs).toBeGreaterThan(0);
  expect(secondRunMs).toBeLessThanOrEqual(150);

  await page
    .getByLabel('Kode playground', { exact: true })
    .fill('int main( { syntax error');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-status')).toContainText(
    'Preview browser: compile_error'
  );
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill(
      '#include <stdio.h>\nint main(void) { for(int i=0;i<30000;i++) putchar(65); return 0; }'
    );
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(
    page.getByText('Output dipotong sesuai batas ukuran.', { exact: true })
  ).toBeVisible();
  const outputSize = await page
    .getByTestId('terminal-scrollback')
    .locator('.term-line')
    .evaluateAll((lines) =>
      lines.reduce((total, line) => total + (line.textContent ?? '').length, 0)
    );
  expect(outputSize).toBeLessThanOrEqual(16384);
  expect(serverRuns).toEqual([]);
});

test('browser preview keeps heap growth bounded across 20 runs', async ({
  page,
  context
}) => {
  test.setTimeout(180000);
  const cdp = await context.newCDPSession(page);
  await cdp.send('Performance.enable');
  const heap = async () => {
    const { metrics } = await cdp.send('Performance.getMetrics');
    return (
      metrics.find((metric) => metric.name === 'JSHeapUsedSize')?.value ?? 0
    );
  };
  await login(page, 'elearning', 'mahasiswa@example.test');
  await page.goto(`${hosts.elearning}/playground`);
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill(
      '#include <stdio.h>\nint main(void) { int n; scanf("%d", &n); printf("hasil:%d\\n", n + 2); return 0; }'
    );
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('0\n');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('hasil:2', {
    timeout: 60000
  });
  const before = await heap();
  for (let index = 1; index <= 20; index++) {
    await page
      .getByLabel('Input standar playground', { exact: true })
      .fill(`${index}\n`);
    await page
      .getByRole('button', { name: 'Jalankan di browser', exact: true })
      .click();
    await expect(page.getByTestId('terminal-scrollback')).toHaveText(
      `hasil:${index + 2}`
    );
  }
  const after = await heap();
  const growthMb = (after - before) / (1024 * 1024);
  console.log('preview.metric.heap_growth_mb_20_runs', growthMb.toFixed(2));
  expect(growthMb).toBeLessThan(50);
});

test('browser preview terminates a nonresponsive worker and does not retry on the server', async ({
  page
}) => {
  test.setTimeout(120000);
  let serverRuns = 0;
  page.on('request', (request) => {
    if (
      request.method() === 'POST' &&
      new URL(request.url()).pathname.endsWith('/run')
    )
      serverRuns += 1;
  });
  await login(page, 'elearning', 'mahasiswa@example.test');
  await page.goto(`${hosts.elearning}/playground`);
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill('int main(void) { volatile unsigned int n = 0; for (;;) n++; }');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByRole('alert')).toContainText('20 detik', {
    timeout: 70000
  });
  await expect(
    page.getByRole('button', { name: 'Jalankan di browser', exact: true })
  ).toBeEnabled();
  expect(serverRuns).toBe(0);
});

test('failed preview bootstrap can be retried without reloading the application', async ({
  page,
  context
}) => {
  test.setTimeout(120000);
  let frameRequests = 0;
  await context.route(`${hosts.portal}/runtimes/v3/frame.html`, (route) => {
    frameRequests += 1;
    return frameRequests === 1
      ? route.fulfill({
          status: 503,
          contentType: 'text/plain',
          body: 'Temporarily unavailable'
        })
      : route.continue();
  });
  await login(page, 'elearning', 'mahasiswa@example.test');
  await page.goto(`${hosts.elearning}/playground`);
  await page
    .getByLabel('Kode playground', { exact: true })
    .evaluate((element, value) => {
      const textarea = element as HTMLTextAreaElement;
      textarea.value = value;
      textarea.dispatchEvent(new Event('input', { bubbles: true }));
    }, '#include <stdio.h>\nint main(void) { puts("recovered"); return 0; }');
  await page.clock.install();
  const failedFrame = page.waitForResponse(
    (response) =>
      response.url().endsWith('/runtimes/v3/frame.html') &&
      response.status() === 503
  );
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await failedFrame;
  await page.clock.fastForward(60001);
  await expect(page.getByRole('alert')).toContainText('60 detik');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText(
    'recovered',
    { timeout: 60000 }
  );
  expect(frameRequests).toBe(2);
});

test('real Python preview uses only the pinned public runtime and no application API', async ({
  page,
  context
}) => {
  test.skip(
    process.env.ALGOHUB_TEST_PYODIDE !== 'true',
    'Opt-in test downloads the pinned public Python runtime.'
  );
  test.setTimeout(120000);
  await context.route(
    'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/**',
    (route) => route.continue()
  );
  await page.addInitScript(() => {
    const scope = globalThis as typeof globalThis & {
      previewTerminations: number;
    };
    scope.previewTerminations = 0;
    const terminate = Worker.prototype.terminate;
    Worker.prototype.terminate = function () {
      scope.previewTerminations += 1;
      terminate.call(this);
    };
  });
  let serverRuns = 0;
  page.on('request', (request) => {
    if (
      request.method() === 'POST' &&
      new URL(request.url()).pathname.endsWith('/run')
    )
      serverRuns += 1;
  });
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
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill('print("hasil:" + str(int(input()) + 2))');
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('40\n');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-status')).toContainText(
    'Preview browser:',
    { timeout: 60000 }
  );
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('hasil:42', {
    timeout: 60000
  });
  console.log(
    'preview.metric.python_first_run_ms',
    Number(
      (await page.getByTestId('preview-duration').textContent())?.match(
        /^(\d+)/
      )?.[1]
    )
  );
  await page
    .getByLabel('Input standar playground', { exact: true })
    .fill('2\n');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('hasil:4', {
    timeout: 60000
  });
  const pythonRunMs = Number(
    (await page.getByTestId('preview-duration').textContent())?.match(
      /^(\d+)/
    )?.[1]
  );
  console.log('preview.metric.python_second_run_ms', pythonRunMs);
  console.log('preview.metric.python_server_runs', serverRuns);
  expect(pythonRunMs).toBeGreaterThan(0);
  expect(pythonRunMs).toBeLessThanOrEqual(300);
  expect(serverRuns).toBe(0);

  await page
    .getByLabel('Kode playground', { exact: true })
    .fill('print("interaktif:" + input())');
  await page.getByLabel('Input standar playground', { exact: true }).fill('');
  await page.getByRole('checkbox', { name: /Input interaktif/ }).check();
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await page.getByLabel('Program meminta input', { exact: true }).fill('42');
  await page.getByRole('button', { name: 'Kirim input', exact: true }).click();
  await expect(page.getByTestId('terminal-scrollback')).toContainText(
    'interaktif:42',
    { timeout: 60000 }
  );
  expect(serverRuns).toBe(0);

  const frame = page
    .frames()
    .find((candidate) => candidate.url().endsWith('/runtimes/v3/frame.html'));
  // The Python worker is reused across runs; it is recycled only after 50 runs,
  // 10 minutes idle, cancellation, timeout, or a fatal error.
  expect(
    await frame!.evaluate(
      () =>
        (globalThis as typeof globalThis & { previewTerminations: number })
          .previewTerminations
    )
  ).toBe(0);
});

test('real Python preview loads packages from user imports', async ({
  page,
  context
}) => {
  test.skip(
    process.env.ALGOHUB_TEST_PYODIDE !== 'true',
    'Opt-in test downloads the pinned public Python runtime.'
  );
  test.setTimeout(180000);
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
  await page
    .getByLabel('Kode playground', { exact: true })
    .fill('import numpy as np\nprint(int(np.sum([1, 2, 3])))');
  await page
    .getByRole('button', { name: 'Jalankan di browser', exact: true })
    .click();
  await expect(page.getByTestId('terminal-scrollback')).toHaveText('6', {
    timeout: 150000
  });
});
