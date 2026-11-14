import { test, expect, hosts, login, api, restrictNetwork } from './helpers';

const paths = [
  '/',
  '/praktikum',
  '/elearning',
  '/siakad',
  '/auditlog',
  '/profil',
  '/pengaturan'
];
const matrix: Array<[string, string[]]> = [
  ['mahasiswa', []],
  ['asisten', []],
  ['peminjam', []],
  ['sekretaris', ['/', '/profil', '/praktikum', '/siakad']],
  ['bendahara', ['/', '/profil', '/siakad']],
  ['k3', ['/', '/profil', '/siakad']],
  ['labops', ['/', '/profil', '/siakad']],
  ['pendidikan', ['/', '/profil', '/elearning']],
  ['pdd', ['/', '/profil']],
  ['scoped', ['/', '/profil', '/praktikum']],
  ['koordinator', paths]
];

test('anonymous direct admin routes return401 without client JavaScript', async ({
  browser
}) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  await restrictNetwork(context);
  const page = await context.newPage();
  try {
    for (const path of paths) {
      expect((await page.goto(`${hosts.admin}${path}`))?.status()).toBe(401);
      await expect(
        page.getByRole('heading', { name: 'Sesi berakhir atau belum masuk' })
      ).toBeVisible();
    }
  } finally {
    await context.close();
  }
});

for (const [fixture, allowed] of matrix) {
  test(`${fixture} receives actual SSR route permissions without JavaScript`, async ({
    browser
  }) => {
    const context = await browser.newContext({ javaScriptEnabled: false });
    await restrictNetwork(context);
    const page = await context.newPage();
    try {
      await login(page, 'admin', `${fixture}@example.test`);
      for (const path of paths) {
        const expected = allowed.includes(path) ? 200 : 403;
        expect(
          (await page.goto(`${hosts.admin}${path}`))?.status(),
          `${fixture} ${path}`
        ).toBe(expected);
        if (expected === 403)
          await expect(
            page.getByRole('heading', { name: 'Akses tidak diizinkan' })
          ).toBeVisible();
      }
    } finally {
      await context.close();
    }
  });
}

test('menus follow actual scoped grants and client role headers cannot elevate access', async ({
  page
}) => {
  await login(page, 'admin', 'scoped@example.test');
  const nav = page.getByRole('navigation', { name: 'Menu utama' });
  await expect(
    nav.getByRole('link', { name: 'Praktikum', exact: true })
  ).toBeVisible();
  await expect(
    nav.getByRole('link', { name: 'E-learning', exact: true })
  ).toHaveCount(0);
  await expect(
    nav.getByRole('link', { name: 'Pengaturan', exact: true })
  ).toHaveCount(0);
  await expect(
    nav.getByRole('link', { name: 'SIAKAD', exact: true })
  ).toHaveCount(0);
  const scope = page.getByLabel('Scope navigasi');
  const option = scope.locator('option').nth(1);
  const selectedLabel = await option.textContent();
  await scope.selectOption((await option.getAttribute('value'))!);
  await expect(page.locator('.scope-context')).toHaveText(selectedLabel!);
  const result = await page.evaluate(
    async () =>
      (
        await fetch('/api/v1/admin/users', {
          headers: {
            'x-user-role': 'koordinator',
            'x-user-division': 'sekretaris'
          }
        })
      ).status
  );
  expect(result).toBe(403);
  expect((await api(page, '/v1/admin/users/9/access')).status).toBe(403);
});

for (const width of [320, 768, 1280]) {
  test(`admin shell is keyboard usable at${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await login(page, 'admin');
    await expect(
      page.getByRole('heading', { name: 'Ruang kendali laboratorium' })
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBe(true);
    const opener = page.getByRole('button', { name: 'Buka navigasi' });
    if (width === 320) {
      await opener.click();
      const dialog = page.getByRole('dialog', { name: 'Navigasi admin' });
      await expect(dialog).toBeVisible();
      const close = dialog.getByRole('button', {
        name: 'Tutup navigasi',
        exact: true
      });
      await expect(close).toBeFocused();
      await page.keyboard.press('Shift+Tab');
      await expect(
        dialog.getByRole('button', { name: 'Keluar', exact: true })
      ).toBeFocused();
      await page.keyboard.press('Tab');
      await expect(close).toBeFocused();
      await page.keyboard.press('Escape');
      await expect(dialog).not.toBeVisible();
      await expect(opener).toBeFocused();
      expect(
        await opener.evaluate(
          (element) => getComputedStyle(element).outlineStyle
        )
      ).not.toBe('none');
      await opener.click();
    } else await expect(opener).not.toBeVisible();
    await page.getByText('Pindah aplikasi', { exact: true }).click();
    await page.locator(`a[href="${hosts.siakad}/login"]`).click();
    await expect(page).toHaveURL(`${hosts.siakad}/`);
    await expect(
      page.getByRole('heading', { name: 'SIAKAD', exact: true })
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBe(true);
  });
}
