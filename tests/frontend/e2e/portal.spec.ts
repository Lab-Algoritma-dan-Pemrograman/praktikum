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
    // Kartu layanan kini berupa tautan ServicePrismCards (0b9bdcf), bukan <article>.
    const services = page.getByRole('region', {
      name: 'Berpindah layanan tanpa kehilangan konteks.'
    });
    await expect(services.getByRole('link')).toHaveCount(4);

    await expect(
      services.getByRole('link', { name: /SIAKAD & layanan lab.*Buka SIAKAD/ })
    ).toHaveAttribute('href', `${hosts.siakad}/login`);
    await expect(
      services.getByRole('link', {
        name: /E-learning interaktif.*Buka E-learning/
      })
    ).toHaveAttribute('href', `${hosts.elearning}/login`);
    await expect(
      services.getByRole('link', { name: /Praktikum terarah.*Buka Praktikum/ })
    ).toHaveAttribute('href', `${hosts.praktikum}/login`);

    // PublicNav bersama beralih ke menu mobile di bawah 60rem (960px).
    if (width <= 960) {
      await page.getByText('Menu', { exact: true }).click();
      await expect(
        page.getByRole('navigation', { name: 'Navigasi mobile' })
      ).toBeVisible();
    } else {
      await expect(
        page.getByRole('navigation', { name: 'Navigasi utama' })
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

test('portal workspace shows scoped applications for an authenticated user', async ({
  page
}) => {
  await login(page, 'portal', 'mahasiswa@example.test');

  // Landing tidak lagi memuat data sesi (0ae1db0); ringkasan akun dan pilihan
  // aplikasi pindah ke /workspace (4aa6607).
  await expect(
    page.getByRole('link', { name: 'Buka ruang kerja' }).first()
  ).toHaveAttribute('href', '/workspace');
  await page.goto(`${hosts.portal}/workspace`);
  await expect(
    page.getByRole('heading', {
      level: 1,
      name: 'Mau mengerjakan apa hari ini?'
    })
  ).toBeVisible();
  await expect(
    page.getByRole('list', { name: 'Peran akun' }).getByText('mahasiswa')
  ).toBeVisible();
  for (const app of ['siakad', 'elearning', 'praktikum'] as const) {
    await expect(page.locator(`a.tile[data-app="${app}"]`)).toHaveAttribute(
      'href',
      `${hosts[app]}/api/auth/start?return_to=%2F`
    );
  }
  await expect(page.locator('a.tile[data-app="admin"]')).toHaveCount(0);
  await noBrowserCredentials(page);
});
