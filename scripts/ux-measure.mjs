// DOM-only UX measurement harness for Algohub surfaces.
// Usage: node scripts/ux-measure.mjs <label> [url...]
// Outputs JSON to docs/ux-visual/<label>.json
import { chromium } from 'playwright-core';
import { mkdirSync, writeFileSync } from 'node:fs';

const label = process.argv[2] ?? 'baseline';
const urls = process.argv.slice(3);
if (urls.length === 0) {
  console.error('provide at least one url');
  process.exit(1);
}

const viewports = [
  { name: 'mobile', width: 390, height: 844 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 800 },
  { name: 'wide', width: 1440, height: 900 }
];

const auditFn = () => {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const de = document.documentElement;

  const smallTargets = [];
  const lowContrast = [];
  const offscreen = [];
  const seen = new Set();
  for (const el of document.querySelectorAll(
    'button, a[href], input, select, textarea, [role="button"], summary'
  )) {
    const r = el.getBoundingClientRect();
    if (r.width === 0 && r.height === 0) continue;
    const key =
      el.tagName +
      ':' +
      (el.textContent || '').trim().slice(0, 24) +
      ':' +
      Math.round(r.width) +
      'x' +
      Math.round(r.height);
    if (seen.has(key)) continue;
    seen.add(key);
    if (r.width < 24 || r.height < 24) {
      smallTargets.push({
        text: (el.textContent || el.getAttribute('aria-label') || '')
          .trim()
          .slice(0, 40),
        size: `${Math.round(r.width)}x${Math.round(r.height)}`
      });
    }
    if (r.right > vw + 1 || r.left < -1) {
      offscreen.push({
        text: (el.textContent || '').trim().slice(0, 40),
        left: Math.round(r.left),
        right: Math.round(r.right)
      });
    }
  }

  const isInteractive = (el) =>
    el.matches('button, a[href], input, select, textarea, [role="button"]');
  let imgsNoAlt = 0;
  for (const img of document.querySelectorAll('img:not([alt])')) imgsNoAlt++;

  // heading hierarchy
  const headings = [...document.querySelectorAll('h1,h2,h3,h4,h5,h6')].map(
    (h) => ({
      level: Number(h.tagName[1]),
      text: h.textContent.trim().slice(0, 60)
    })
  );
  let headingSkip = false;
  for (let i = 1; i < headings.length; i++) {
    if (headings[i].level - headings[i - 1].level > 1) headingSkip = true;
  }
  const h1Count = headings.filter((h) => h.level === 1).length;

  // horizontal overflow
  const docScroll = de.scrollWidth > de.clientWidth + 1;

  // large empty regions (dead space) — scan for big empty blocks
  // measured roughly by counting containers with >200px empty vertical space

  // text sizes
  const typeSizes = {};
  for (const el of document.querySelectorAll('p, li, td, span, div')) {
    if (!el.textContent || !el.textContent.trim()) continue;
    const cs = getComputedStyle(el);
    const fs = parseFloat(cs.fontSize);
    if (fs < 12) {
      const t = el.textContent.trim().slice(0, 30);
      typeSizes[t] = fs;
    }
  }
  const tinyTextCount = Object.keys(typeSizes).length;

  // focusable count & labeled inputs
  const inputsNoLabel = [];
  for (const input of document.querySelectorAll('input, select, textarea')) {
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
      inputsNoLabel.push(
        input.getAttribute('name') ||
          input.getAttribute('placeholder') ||
          input.tagName
      );
  }

  return {
    viewport: { w: vw, h: vh },
    page: {
      scrollWidth: de.scrollWidth,
      clientWidth: de.clientWidth,
      scrollHeight: de.scrollHeight,
      horizontalOverflow: docScroll,
      pageScrollPx: Math.max(0, de.scrollHeight - vh)
    },
    smallTargets,
    offscreen,
    imgsNoAlt,
    h1Count,
    headingSkip,
    headings: headings.slice(0, 12),
    tinyTextCount,
    inputsNoLabel
  };
};

const browser = await chromium.launch({ args: ['--no-proxy-server'] });
const results = {};
try {
  for (const url of urls) {
    const page = await browser.newPage();
    const perViewport = {};
    for (const vp of viewports) {
      await page.setViewportSize({ width: vp.width, height: vp.height });
      try {
        await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
        await page.waitForTimeout(600);
        const data = await page.evaluate(auditFn);
        perViewport[vp.name] = data;
      } catch (e) {
        perViewport[vp.name] = { error: String(e).slice(0, 200) };
      }
    }
    // also capture reduced-motion & keyboard focus check on desktop
    await page.setViewportSize({ width: 1280, height: 800 });
    try {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await page.waitForTimeout(300);
      const focusInfo = await page.evaluate(() => {
        const els = [
          ...document.querySelectorAll(
            'button, a[href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'
          )
        ].filter((e) => e.offsetParent !== null || e.getClientRects().length);
        return { focusableCount: els.length };
      });
      perViewport.keyboard = focusInfo;
    } catch {
      /* ignore */
    }
    results[url] = perViewport;
    await page.close();
  }
} finally {
  await browser.close();
}

mkdirSync('docs/ux-visual', { recursive: true });
const out = `docs/ux-visual/${label}.json`;
writeFileSync(out, JSON.stringify(results, null, 2));
console.log(`written ${out}`);
