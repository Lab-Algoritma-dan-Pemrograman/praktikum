import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { hosts } from '../local';

// WCAG 2.2 A + AA ruleset. 'color-contrast' dinonaktifkan: false-positive di
// headless Chrome akibat fallback font rendering; dicatat di laporan statis.
const disabledRules = ['color-contrast'];

type PageTarget = {
  name: string;
  url: string;
  finalOrigin: string;
  finalPath: string;
  title: string;
  heading: string;
  returnTo?: string;
};

const loginTarget = (name: string, origin: string): PageTarget => ({
  name,
  url: `${origin}/login`,
  finalOrigin: hosts.auth,
  finalPath: '/login',
  title: 'Login — Algohub',
  heading: 'Masuk dengan identitas Algohub.',
  returnTo: `${origin}/`
});

const pages: PageTarget[] = [
  {
    name: 'portal-root',
    url: `${hosts.portal}/`,
    finalOrigin: hosts.portal,
    finalPath: '/',
    title: 'Algohub — Belajar, Praktikum, dan Layanan Laboratorium',
    heading: 'Belajar, praktik, dan kelola lab dalam satu identitas.'
  },
  {
    name: 'portal-jadwal',
    url: `${hosts.portal}/jadwal`,
    finalOrigin: hosts.portal,
    finalPath: '/jadwal',
    title: 'Jadwal yang aman untuk dibagikan. — Algohub',
    heading: 'Jadwal yang aman untuk dibagikan.'
  },
  {
    name: 'portal-asisten',
    url: `${hosts.portal}/asisten`,
    finalOrigin: hosts.portal,
    finalPath: '/asisten',
    title: 'Temui tim yang mengelola sesi. — Algohub',
    heading: 'Temui tim yang mengelola sesi.'
  },
  {
    name: 'portal-laporan',
    url: `${hosts.portal}/laporan`,
    finalOrigin: hosts.portal,
    finalPath: '/laporan',
    title: 'Laporan praktikum, dengan akses yang tepat. — Algohub',
    heading: 'Laporan praktikum, dengan akses yang tepat.'
  },
  {
    name: 'portal-modul',
    url: `${hosts.portal}/modul`,
    finalOrigin: hosts.portal,
    finalPath: '/modul',
    title: 'Materi yang siap dipelajari. — Algohub',
    heading: 'Materi yang siap dipelajari.'
  },
  {
    name: 'portal-bantuan',
    url: `${hosts.portal}/bantuan`,
    finalOrigin: hosts.portal,
    finalPath: '/bantuan',
    title: 'Mulai dari ruang yang sesuai. — Algohub',
    heading: 'Mulai dari ruang yang sesuai.'
  },
  {
    name: 'portal-privacy',
    url: `${hosts.portal}/privacy`,
    finalOrigin: hosts.portal,
    finalPath: '/privacy',
    title: 'Privasi perlu disetujui pemilik. — Algohub',
    heading: 'Privasi perlu disetujui pemilik.'
  },
  {
    name: 'portal-terms',
    url: `${hosts.portal}/terms`,
    finalOrigin: hosts.portal,
    finalPath: '/terms',
    title: 'Ketentuan penggunaan perlu persetujuan pemilik. — Algohub',
    heading: 'Ketentuan penggunaan perlu persetujuan pemilik.'
  },
  {
    name: 'auth-login',
    url: `${hosts.auth}/login`,
    finalOrigin: hosts.auth,
    finalPath: '/login',
    title: 'Login — Algohub',
    heading: 'Masuk dengan identitas Algohub.'
  },
  loginTarget('admin-login', hosts.admin),
  loginTarget('siakad-login', hosts.siakad),
  loginTarget('elearning-login', hosts.elearning),
  loginTarget('praktikum-login', hosts.praktikum)
];

const viewports = [
  { name: '320px', width: 320, height: 640 },
  { name: '375px', width: 375, height: 720 },
  { name: '768px', width: 768, height: 720 },
  { name: '1024px', width: 1024, height: 720 },
  { name: '1440px', width: 1440, height: 900 }
];

for (const viewport of viewports) {
  test.describe(`a11y @ ${viewport.name}`, () => {
    test.use({ viewport: { width: viewport.width, height: viewport.height } });

    for (const page of pages) {
      test(`axe WCAG 2.2 A+AA: ${page.name}`, async ({ page: pw }) => {
        const response = await pw.goto(page.url, { waitUntil: 'load' });
        expect(response?.ok()).toBe(true);

        const finalUrl = new URL(pw.url());
        expect(finalUrl.origin).toBe(page.finalOrigin);
        expect(finalUrl.pathname).toBe(page.finalPath);
        if (page.returnTo)
          expect(finalUrl.searchParams.get('return_to')).toBe(page.returnTo);
        await expect(pw).toHaveTitle(page.title);
        await expect(
          pw.getByRole('heading', { level: 1, name: page.heading })
        ).toBeVisible();

        const results = await new AxeBuilder({ page: pw })
          .withTags(['wcag2a', 'wcag2aa', 'wcag22a', 'wcag22aa'])
          .disableRules(disabledRules)
          .analyze();

        const summary = results.violations.map((violation) => ({
          id: violation.id,
          impact: violation.impact,
          nodes: violation.nodes.length,
          sample: violation.nodes[0]?.target.join(' ')
        }));
        expect(
          summary,
          `${page.name} @ ${viewport.name}: ${JSON.stringify(summary)}`
        ).toEqual([]);
      });
    }
  });
}
