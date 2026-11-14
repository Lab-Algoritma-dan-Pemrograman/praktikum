// Authenticated UX measurement: logs in as coordinator via the auth app, then
// measures authenticated surfaces on each app host.
import { chromium } from 'playwright-core';
import { mkdirSync, writeFileSync } from 'node:fs';

const exe = process.env.UX_CHROMIUM || undefined;
const label = process.argv[2] ?? 'baseline-auth';
const email = process.env.UX_EMAIL ?? 'koordinator@example.test';
const password = process.env.UX_PASSWORD ?? 'fixture-password';

const targets = [
  ['portal', 'http://portal.localhost:5173/'],
  ['siakad', 'http://siakad.localhost:5176/'],
  ['elearning', 'http://elearning.localhost:5177/'],
  ['praktikum', 'http://praktikum.localhost:5178/'],
  ['admin', 'http://admin.localhost:5175/'],
  ['admin/praktikum', 'http://admin.localhost:5175/praktikum'],
  ['admin/auditlog', 'http://admin.localhost:5175/auditlog'],
  ['admin/pengaturan', 'http://admin.localhost:5175/pengaturan']
];

const viewports = [
  { name: 'mobile', width: 390, height: 844 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 800 }
];

const auditFn = () => {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const de = document.documentElement;
  const smallTargets = [];
  const offscreen = [];
  const seen = new Set();
  for (const el of document.querySelectorAll(
    'button, a[href], input, select, textarea, summary, [role="button"]'
  )) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 && r.height === 0) continue;
    const t = (el.textContent || el.getAttribute('aria-label') || '')
      .trim()
      .slice(0, 36);
    const key =
      el.tagName +
      ':' +
      t +
      ':' +
      Math.round(r.width) +
      'x' +
      Math.round(r.height);
    if (seen.has(key)) continue;
    seen.add(key);
    // Only flag real interactive controls smaller than 24px in either dimension
    if (r.height < 24 || r.width < 24)
      smallTargets.push({
        text: t,
        size: `${Math.round(r.width)}x${Math.round(r.height)}`,
        top: Math.round(r.top)
      });
    if (r.right > vw + 1 || r.left < -1)
      offscreen.push({
        text: t,
        left: Math.round(r.left),
        right: Math.round(r.right)
      });
  }
  const headings = [...document.querySelectorAll('h1,h2,h3,h4,h5,h6')].map(
    (h) => ({
      level: Number(h.tagName[1]),
      text: h.textContent.trim().slice(0, 50)
    })
  );
  let headingSkip = false;
  for (let i = 1; i < headings.length; i++)
    if (headings[i].level - headings[i - 1].level > 1) headingSkip = true;
  const inputsNoLabel = [];
  for (const input of document.querySelectorAll('input, select, textarea')) {
    if (input.type === 'hidden') continue;
    const id = input.getAttribute('id');
    let labeled = false;
    if (id && document.querySelector(`label[for="${id}"]`)) labeled = true;
    if (
      !labeled &&
      (input.getAttribute('aria-label') ||
        input.getAttribute('aria-labelledby'))
    )
      labeled = true;
    if (!labeled && input.closest('label')) labeled = true;
    if (!labeled)
      inputsNoLabel.push(input.getAttribute('name') || input.tagName);
  }
  // tiny text
  let tiny = 0;
  for (const el of document.querySelectorAll('p, li, td, span')) {
    if (!el.textContent || !el.textContent.trim()) continue;
    const fs = parseFloat(getComputedStyle(el).fontSize);
    if (fs > 0 && fs < 12) tiny++;
  }
  return {
    title: document.title,
    url: location.href,
    h1Count: headings.filter((h) => h.level === 1).length,
    headingSkip,
    headings: headings.slice(0, 10),
    horizontalOverflow: de.scrollWidth > de.clientWidth + 1,
    scrollWidth: de.scrollWidth,
    clientWidth: de.clientWidth,
    pageScrollPx: Math.max(0, de.scrollHeight - vh),
    smallTargets,
    offscreen,
    inputsNoLabel,
    tiny
  };
};

const browser = await chromium.launch({
  executablePath: exe,
  args: ['--no-proxy-server']
});
const context = await browser.newContext();
const page = await context.newPage();

// Log in via the auth app.
await page.goto('http://auth.localhost:5174/login', {
  waitUntil: 'domcontentloaded'
});
await page.waitForTimeout(500);
const hasForm = await page.locator('#email').count();
let loginOk = false;
if (hasForm) {
  await page.fill('#email', email);
  await page.fill('#password', password);
  await Promise.all([
    page.waitForLoadState('networkidle').catch(() => {}),
    page.locator('button[type="submit"]').first().click()
  ]);
  await page.waitForTimeout(1200);
  loginOk =
    !page.url().includes('/login') || !(await page.locator('#email').count());
  console.log('login result url:', page.url(), 'ok:', loginOk);
} else {
  console.log('no login form at', page.url());
}

const results = {};
for (const [name, url] of targets) {
  const perViewport = {};
  for (const vp of viewports) {
    await page.setViewportSize({ width: vp.width, height: vp.height });
    try {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await page.waitForTimeout(700);
      perViewport[vp.name] = await page.evaluate(auditFn);
    } catch (e) {
      perViewport[vp.name] = { error: String(e).slice(0, 180) };
    }
  }
  results[name] = { url, ...perViewport };
}
await browser.close();

mkdirSync('docs/ux-visual', { recursive: true });
const out = `docs/ux-visual/${label}.json`;
writeFileSync(out, JSON.stringify({ loginOk, email, results }, null, 2));
console.log('written', out);
