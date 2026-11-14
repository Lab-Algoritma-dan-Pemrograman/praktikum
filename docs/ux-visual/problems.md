# Ranked Problems — Scope C

Ranked by severity × reach × centrality ÷ effort. Root-cause first.

## P1 — Global app nav links are 21px tall (below 24px WCAG 2.2 minimum)

- **Root cause:** `packages/ui/src/console.css` styles `nav` as a flex row but never gives `nav a` a minimum height or padding. App layouts (siakad, elearning, praktikum) render bare `<a>` elements.
- **Affected:** siakad `/`, `/absensi`, `/penyewaan`; elearning `/`, `/playground`, `/bug-hunt`, `/leaderboard`, `/lesson/[id]`; praktikum `/`, `/sesi/[id]`, `/nilai`.
- **Evidence:** `baseline-auth.json` → `small=3 [Kelas & Jadwal(105x21)…]`, `small=4 […]`, `small=2 […]`.
- **Acceptance:** every nav link ≥ 44px tall at all viewports; active location highlighted with `aria-current`.

## P1 — Identity card repeated above every app page

- **Root cause:** `AppShell` renders a full identity page (big h1, description, "Masuk sebagai…", email, roles, Keluar) and is used as a persistent header by siakad/elearning/praktikum. It is only appropriate on `auth /`.
- **Affected:** 3 apps × every route.
- **Evidence:** DOM: `.identity` block present on `/`, `/absensi`, `/penyewaan`, etc. It pushes real content below the fold.
- **Acceptance:** app pages show one slim header (brand + nav + compact account menu); identity card only on the auth identity page; logout still reachable and keyboard-accessible.

## P1 — Landing footer links overflow the mobile viewport

- **Root cause:** `footer { grid-template-columns: minmax(0,1fr) auto }` plus `footer nav { grid-template-columns: repeat(2, minmax(8rem,1fr)) }` and no `min-width:0` on children. At 390px the nav column cannot shrink.
- **Affected:** portal `/` at ≤ 390px.
- **Evidence:** `baseline.json` → 5 offscreen links at `right=422 > 390`; `offscreen=5`.
- **Acceptance:** zero horizontal page overflow and zero off-screen controls at 320–390px.

## P1 — Landing footer links are 22px tall

- **Root cause:** `footer nav a { font-size: 0.9rem }` with no min-height/padding.
- **Evidence:** `smallTargets=7` at every viewport, largest group = footer links `128×22`.
- **Acceptance:** all footer links ≥ 44px tall; still visually a tidy list.

## P2 — Admin breadcrumb link is 40×18px

- **Root cause:** `AdminShell.svelte` `.breadcrumb a` has color only, no height.
- **Evidence:** `small=1 [Admin(40x18)]` on every admin route.
- **Acceptance:** breadcrumb links ≥ 24px tall with visible focus.

## P2 — Admin eyebrow text is 11.52px

- **Root cause:** `apps/admin/src/app.css` `.identity-page .eyebrow { font-size: 0.72rem }`.
- **Evidence:** `tiny=6` on `/auditlog`.
- **Acceptance:** no body/meta text under 12px.

## P2 — Landing page length

- **Observation:** 5571px tall at 1440×900 (hero 815, services 1310, info 1191). Not a conformance failure but heavy. Tighten section padding and service card density without changing content.
- **Acceptance:** reduce total height ≥ 10% at 1440×900 while keeping all sections and readable spacing.

## Deferred (not in this pass)

- Full dark mode.
- Landing hero motion polish.
- Admin table virtualization (auditlog > 1000 rows).
- Human screen-reader / physical-device review (deferred-human).
