# Verification — Scope C

**Date:** 2026-11-15
**Method:** same DOM harness as the baseline, re-run after each change. No image calls.

## Before → after (measured)

| Surface | Viewport | Small targets | Sub-12px text | H-overflow |
|---|---|---|---|---|
| portal `/` | 390 | 7 → **0** | 3 → **0** | false → false |
| portal `/` | 768 | 7 → **0** | 3 → **0** | false → false |
| portal `/` | 1280 | 7 → **0** | 3 → **0** | false → false |
| portal `/` | 1440 | 7 → **0** | 3 → **0** | false → false |
| siakad `/` | all | 3 → **0** | 0 | false |
| elearning `/` | all | 4 → **0** | 0 | false |
| praktikum `/` | all | 2 → **0** | 0 | false |
| admin (7 routes) | all | 1–2 → **0** | 6 → **0** | false |

Landing height at 1440×900: 5571px → **4722px** (−15%).
Landing height at 1280×800: 4801px → **4576px**.
Mobile height grew ~160px because footer links are now 44px tall (intentional).

> Note: the 5 "off-screen" elements on the landing mobile view are the **closed
> mobile menu panel** (translated off-canvas), and the 17 on admin are the
> **closed drawer**. Both are expected and were present in the baseline.

## Root-cause fixes

1. `packages/ui/src/console.css` — `nav a` now `inline-flex`, `min-height: 44px`,
   padding, hover and `[aria-current='page']` active state. Fixes siakad,
   elearning, praktikum nav at the root.
2. `packages/ui/src/AppShell.svelte` — new `variant: 'identity' | 'app'`.
   - `identity` (auth `/`): unchanged full identity page. Verified: h1 "Auth Center",
     identity card present.
   - `app`: slim header + account `<details>` menu (avatar, name, email, roles,
     Keluar) + visually-hidden `h1` keeping the accessible name. The repeated
     identity card no longer appears on app pages.
3. `apps/{siakad,elearning,praktikum}/src/routes/+layout.svelte` — use `variant="app"`,
   `aria-current` from `$app/state`, removed the `:global` override hacks.
4. `packages/ui/src/AdminShell.svelte` — breadcrumb link `min-height: 24px` + padding;
   brand link `min-height: 44px`.
5. `apps/admin/src/app.css` — eyebrow `0.72rem` → `0.75rem` (12px floor).
6. `apps/portal/src/routes/+page.svelte` — footer: `minmax(0,1fr)` columns,
   `min-width: 0` on children, responsive gap (fixes overflow at 430px), footer
   links `min-height: 44px`, copyright link `min-height: 24px`; tightened section
   and hero padding; `visual-card` label 10.56px → 12px.

## Functional checks

- Auth identity page unchanged (h1 "Auth Center", identity card present).
- Siakad: identity card `0`, h1 "SIAKAD", nav `aria-current=page` on
  "Kelas & Jadwal", account menu opens, Keluar visible, email correct,
  app-switcher works, all nav links 44px.
- All nav links across siakad measured at 44px.

## Test evidence

| Check | Result |
|---|---|
| `svelte-check` portal, auth, admin, siakad, elearning, praktikum | 0 errors, 0 warnings (6/6) |
| `npm run test:frontend` (5 packages) | 18 pass / 0 fail |
| `prettier --check` changed files | clean |
| Playwright: guards + phase7a-screenshots | 26 / 26 pass |
| Playwright: a11y + portal + siakad + elearning + practicum | 88 / 88 pass |
| axe WCAG 2.2 A+AA at 320/375/768/1024/1440 | no new violations |

## Not verified (deferred)

- Human screen-reader and physical-device review (`deferred-human`).
- Dark mode (not implemented in this pass).
- Visual judgement of "does it look better" was not made with the vision model,
  per your instruction to use DOM measurement only.
