import { chromium } from 'playwright-core';
import { mkdirSync, writeFileSync } from 'node:fs';
const exe = process.env.UX_CHROMIUM || undefined;
const b = await chromium.launch({
  executablePath: exe,
  args: ['--no-proxy-server']
});
const c = await b.newContext();
const p = await c.newPage();
// SSO
await p.goto('http://admin.localhost:5175/login', {
  waitUntil: 'domcontentloaded'
});
await p.waitForTimeout(600);
if (await p.locator('#email').count()) {
  await p.fill('#email', 'koordinator@example.test');
  await p.fill('#password', 'fixture-password');
  await p.locator('button[type=submit]').first().click();
  await p.waitForTimeout(1600);
}
const audit = () => {
  const vw = innerWidth,
    vh = innerHeight,
    de = document.documentElement;
  const sm = [],
    off = [],
    seen = new Set();
  for (const el of document.querySelectorAll(
    'button,a[href],input,select,textarea,summary,[role=button]'
  )) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 && r.height === 0) continue;
    const t = (el.textContent || el.getAttribute('aria-label') || '')
      .trim()
      .slice(0, 36);
    const k = el.tagName + t + Math.round(r.width) + 'x' + Math.round(r.height);
    if (seen.has(k)) continue;
    seen.add(k);
    if (r.height < 24 || r.width < 24)
      sm.push({
        text: t,
        size: `${Math.round(r.width)}x${Math.round(r.height)}`
      });
    if (r.right > vw + 1 || r.left < -1)
      off.push({
        text: t,
        left: Math.round(r.left),
        right: Math.round(r.right)
      });
  }
  const hs = [...document.querySelectorAll('h1,h2,h3,h4,h5,h6')].map((h) => ({
    l: +h.tagName[1],
    t: h.textContent.trim().slice(0, 44)
  }));
  let skip = false;
  for (let i = 1; i < hs.length; i++)
    if (hs[i].l - hs[i - 1].l > 1) skip = true;
  let tiny = 0;
  for (const el of document.querySelectorAll('p,li,td,span')) {
    if (!el.textContent.trim()) continue;
    const fs = parseFloat(getComputedStyle(el).fontSize);
    if (fs && fs < 12) tiny++;
  }
  return {
    title: document.title,
    url: location.href,
    h1: hs.filter((h) => h.l === 1).length,
    skip,
    headings: hs.slice(0, 10),
    hscroll: de.scrollWidth > de.clientWidth + 1,
    sw: de.scrollWidth,
    cw: de.clientWidth,
    scrollPx: Math.max(0, de.scrollHeight - vh),
    small: sm,
    offscreen: off,
    tiny
  };
};
const targets = [
  ['admin', 'http://admin.localhost:5175/'],
  ['admin/praktikum', 'http://admin.localhost:5175/praktikum'],
  ['admin/elearning', 'http://admin.localhost:5175/elearning'],
  ['admin/siakad', 'http://admin.localhost:5175/siakad'],
  ['admin/auditlog', 'http://admin.localhost:5175/auditlog'],
  ['admin/pengaturan', 'http://admin.localhost:5175/pengaturan'],
  ['admin/profil', 'http://admin.localhost:5175/profil'],
  ['siakad', 'http://siakad.localhost:5176/'],
  ['elearning', 'http://elearning.localhost:5177/'],
  ['praktikum', 'http://praktikum.localhost:5178/']
];
const vps = [
  { n: 'mobile', w: 390, h: 844 },
  { n: 'tablet', w: 768, h: 1024 },
  { n: 'desktop', w: 1280, h: 800 }
];
const res = {};
for (const [name, url] of targets) {
  const o = {};
  for (const vp of vps) {
    await p.setViewportSize({ width: vp.w, height: vp.h });
    try {
      await p.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await p.waitForTimeout(700);
      o[vp.n] = await p.evaluate(audit);
    } catch (e) {
      o[vp.n] = { error: String(e).slice(0, 150) };
    }
  }
  res[name] = { url, ...o };
}
await b.close();
mkdirSync('docs/ux-visual', { recursive: true });
writeFileSync('docs/ux-visual/after-admin.json', JSON.stringify(res, null, 2));
console.log('done');
for (const [name, r] of Object.entries(res)) {
  console.log('\n=== ' + name + ' ===');
  for (const vp of ['mobile', 'tablet', 'desktop']) {
    const m = r[vp];
    if (!m || m.error) {
      console.log('  ' + vp + ': ERR');
      continue;
    }
    const f = [];
    if (m.hscroll) f.push('HSCROLL ' + m.sw + '>' + m.cw);
    if (m.scrollPx > 200 && vp !== 'mobile') f.push('scroll=' + m.scrollPx);
    if (m.h1 !== 1) f.push('h1=' + m.h1);
    if (m.skip) f.push('SKIP');
    if (m.small.length)
      f.push(
        'small=' +
          m.small.length +
          ' [' +
          m.small
            .slice(0, 3)
            .map((t) => t.text + '(' + t.size + ')')
            .join(',') +
          ']'
      );
    if (m.offscreen.length) f.push('off=' + m.offscreen.length);
    if (m.tiny) f.push('tiny=' + m.tiny);
    console.log(
      '  ' +
        vp +
        ' [' +
        (m.title || '').slice(0, 28) +
        ']: ' +
        (f.join(' | ') || 'ok')
    );
  }
}
