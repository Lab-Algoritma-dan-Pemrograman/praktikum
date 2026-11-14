import { mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { test, expect, hosts, login, noBrowserCredentials } from './helpers';

const artifactDir = process.env.PHASE_7A_SCREENSHOT_DIR ?? 'tests/frontend/results/phase-7a-screenshots';
mkdirSync(artifactDir, { recursive: true });

function artifact(name: string): string {
  return join(artifactDir, `${name}.png`);
}

for (const viewport of [
  { name: '320', width: 320, height: 900 },
  { name: '375', width: 375, height: 900 },
  { name: '768', width: 768, height: 900 },
  { name: '1024', width: 1024, height: 900 },
  { name: '1440', width: 1440, height: 900 }
]) {
  test(`capture Portal landing at ${viewport.name}px`, async ({ page }) => {
    await page.setViewportSize({ width: viewport.width, height: viewport.height });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    const response = await page.goto(hosts.portal, { waitUntil: 'domcontentloaded' });
    expect(response?.status()).toBe(200);
    await expect(page.getByRole('heading', { level: 1, name: 'Belajar, praktik, dan kelola lab dalam satu identitas.' })).toBeVisible();
    await page.screenshot({ path: artifact(`portal-root-${viewport.name}`), fullPage: true });
    await noBrowserCredentials(page);
  });
}

const representatives: Array<{
  name: keyof typeof hosts;
  email: string;
  heading: RegExp;
}> = [
  { name: 'auth', email: 'koordinator@example.test', heading: /Algohub|Auth Center/i },
  { name: 'admin', email: 'koordinator@example.test', heading: /Ruang kendali laboratorium/i },
  { name: 'siakad', email: 'mahasiswa@example.test', heading: /SIAKAD|akademik/i },
  { name: 'elearning', email: 'mahasiswa@example.test', heading: /E-learning|kurikulum/i },
  { name: 'praktikum', email: 'mahasiswa@example.test', heading: /Praktikum|sesi/i }
];

for (const representative of representatives) {
  test(`capture representative ${representative.name} page`, async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await login(page, representative.name, representative.email);
    await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
    await expect(page.getByRole('heading', { level: 1 }).first()).toHaveText(representative.heading);
    await page.screenshot({ path: artifact(`${representative.name}-root-1024`), fullPage: true });
    await noBrowserCredentials(page);
  });
}
