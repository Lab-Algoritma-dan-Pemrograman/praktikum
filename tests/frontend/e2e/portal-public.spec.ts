import { test, expect } from '@playwright/test';
import { hosts } from '../local';

const publicRoutes = [
  {
    path: '/jadwal',
    heading: 'Kalender pekan praktikum.'
  },
  { path: '/asisten', heading: 'Divisi yang menjalankan laboratorium.' },
  {
    path: '/laporan',
    heading: 'Laporan yang bisa dinilai, bukan sekadar dikumpulkan.'
  },
  {
    path: '/modul',
    heading: 'Modul praktikum: apa yang dibagikan dan di mana mengaksesnya.'
  },
  { path: '/bantuan', heading: 'Mulai dari ruang yang sesuai.' },
  { path: '/privacy', heading: 'Privasi perlu disetujui pemilik.' },
  { path: '/terms', heading: 'Ketentuan penggunaan perlu persetujuan pemilik.' }
];

for (const width of [320, 375, 768, 1024, 1440]) {
  test(`public portal routes are readable without overflow at ${width}px`, async ({
    page
  }) => {
    await page.setViewportSize({ width, height: 900 });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    for (const route of publicRoutes) {
      const response = await page.goto(`${hosts.portal}${route.path}`);
      expect(response?.status(), route.path).toBe(200);
      await expect(
        page.getByRole('heading', { level: 1, name: route.heading })
      ).toBeVisible();
      const dimensions = await page.evaluate(() => ({
        viewport: innerWidth,
        document: document.documentElement.scrollWidth
      }));
      expect(dimensions.document, route.path).toBeLessThanOrEqual(
        dimensions.viewport + 1
      );
    }
  });
}

test('legacy info route redirects to the sole landing root', async ({
  page
}) => {
  const response = await page.goto(`${hosts.portal}/info`);
  expect(response?.status()).toBe(200);
  await expect(page).toHaveURL(`${hosts.portal}/`);
  await expect(
    page.getByRole('heading', {
      level: 1,
      name: 'Belajar, praktik, dan kelola lab dalam satu identitas.'
    })
  ).toBeVisible();
});

test('practicum compatibility route points to the dedicated host', async ({
  page
}) => {
  const target = new URL(`${hosts.portal}/praktikum`);
  target.hostname = '127.0.0.1';
  const response = await page.request.get(target.toString(), {
    maxRedirects: 0,
    headers: { host: new URL(hosts.portal).host }
  });
  expect(response.status()).toBe(307);
  expect(response.headers().location).toBe('https://praktikum.algohub.web.id/');
});
