# UX/UI Audit — Inventory (Scope C)

**Date:** 2026-11-15
**Method:** DOM measurement only (no image calls). Playwright headless Chromium at
390×844, 768×1024, 1280×800, 1440×900. Authenticated as `koordinator@example.test`
via the real SSO flow.
**Harness:** `scripts/ux-measure.mjs`, `scripts/ux-measure-auth.mjs`
**Raw data:** `docs/ux-visual/baseline.json`, `docs/ux-visual/baseline-auth.json`

## Design system source of truth

- Tokens: `packages/ui/src/tokens.css` (maroon "calm technical lab" DNA).
- Console primitives: `packages/ui/src/console.css`.
- Shells: `packages/ui/src/AppShell.svelte`, `AdminShell.svelte`.
- Landing: `apps/portal/src/routes/+page.svelte` (self-contained tokens).
- No Tailwind. No CSS framework. Plain CSS + custom properties.

## Surfaces in scope

| App | Surface | Route |
|---|---|---|
| portal | Landing | `/` |
| portal | Public info pages | `/jadwal`, `/asisten`, `/laporan`, `/modul`, `/bantuan`, `/privacy`, `/terms` |
| siakad | Identity header + nav + dashboard | `/`, `/absensi`, `/penyewaan` |
| elearning | Identity header + nav + kurikulum | `/`, `/playground`, `/bug-hunt`, `/leaderboard`, `/lesson/[id]` |
| praktikum | Identity header + nav + sessions | `/`, `/sesi/[id]`, `/attempts/[id]`, `/nilai` |
| admin | Admin console shell | `/`, `/praktikum`, `/elearning`, `/siakad`, `/auditlog`, `/pengaturan`, `/profil` |

## Baseline measurements (summary)

| Surface | Finding |
|---|---|
| portal `/` | 7 interactive targets < 24px (footer links 128×22). 5 footer links overflow 32px past 390px viewport. Page 5571px tall at 1440×900. |
| siakad `/` | 3 nav links 21px tall (below 24px WCAG 2.2). Identity card repeated above every page. |
| elearning `/` | 4 nav links 21px tall. Identity card repeated above every page. |
| praktikum `/` | 2 nav links 21px tall. Identity card repeated above every page. |
| admin (all) | Breadcrumb "Admin" link 40×18px. Audit log has 6 text items < 12px. Admin `/praktikum` scrolls 1390px, `/auditlog` 1085px at desktop. |

## Excellent as-is

- Landing has a distinct brand mark, arch-shaped CTA, coherent editorial rhythm, skip link, `prefers-reduced-motion` block, and keyboard-reachable nav.
- AdminShell already has a mobile drawer with focus trap, Escape handling, `aria-current`, and focus restore.
- `AdminShell` sidebar links and app-switcher links are already 44px.
- `PortalPage.svelte` / `PublicInfoPage.svelte` nav links are already 44px with active state.
