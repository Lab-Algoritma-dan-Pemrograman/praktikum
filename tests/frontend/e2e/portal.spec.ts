import { test, expect, hosts, login, noBrowserCredentials } from './helpers';

for (const width of [320, 768, 1280]) {
  test(`portal landing is complete and responsive at ${width}px`, async ({
    page
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: 'reduce' });

    const response = await page.goto(hosts.portal);
    expect(response?.status()).toBe(200);
    await expect(page).toHaveTitle(
      'Algohub — Belajar, Praktikum, dan Layanan Laboratorium'
    );
    await expect(
      page.getByRole('heading', {
        level: 1,
        name: 'Belajar, praktik, dan kelola lab dalam satu identitas.'
      })
    ).toBeVisible();
    await expect(
      page.getByRole('heading', {
        level: 2,
        name: 'Berpindah layanan tanpa kehilangan konteks.'
      })
    ).toBeVisible();
    await expect(page.getByRole('article')).toHaveCount(4);

    await expect(
      page.getByRole('link', { name: /Buka SIAKAD & layanan lab/ })
    ).toHaveAttribute('href', `${hosts.siakad}/login`);
    await expect(
      page.getByRole('link', { name: /Buka E-learning interaktif/ })
    ).toHaveAttribute('href', `${hosts.elearning}/login`);
    await expect(
      page.getByRole('link', { name: /Buka Praktikum terarah/ })
    ).toHaveAttribute('href', `${hosts.praktikum}/login`);

    if (width <= 720) {
      await page.getByText('Menu', { exact: true }).click();
      await expect(
        page.getByRole('navigation', { name: 'Navigasi mobile' })
      ).toBeVisible();
    } else {
      await expect(
        page.getByRole('navigation', { name: 'Navigasi landing page' })
      ).toBeVisible();
    }

    const dimensions = await page.evaluate(() => ({
      viewport: innerWidth,
      document: document.documentElement.scrollWidth
    }));
    expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1);
    await noBrowserCredentials(page);
  });
}

test('portal landing shows scoped workspace for an authenticated user', async ({
  page
}) => {
  await login(page, 'portal', 'mahasiswa@example.test');

  await expect(
    page.getByRole('heading', { name: /Selamat datang kembali/ })
  ).toBeVisible();
  await expect(
    page.getByText('mahasiswa', { exact: false }).first()
  ).toBeVisible();
  const workspace = page.locator('#workspace');
  await expect(
    workspace.getByRole('link', { name: 'SIAKAD', exact: true })
  ).toHaveAttribute('href', `${hosts.siakad}/login`);
  await expect(
    workspace.getByRole('link', { name: 'E-learning', exact: true })
  ).toHaveAttribute('href', `${hosts.elearning}/login`);
  await expect(
    workspace.getByRole('link', { name: 'Praktikum', exact: true })
  ).toHaveAttribute('href', `${hosts.praktikum}/login`);
  await noBrowserCredentials(page);
});
