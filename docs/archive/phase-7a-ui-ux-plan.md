# Phase 7A — UI/UX Audit & Consolidation Plan

**Tanggal:** 2026-11-14  
**Status:** execution plan and inventory baseline; local-only  
**Scope:** source, automated review, screenshot evidence, documentation, deployment preparation. No deploy, DNS mutation, remote allowlist mutation, human UAT, pilot, load/stress, alert delivery, incident drill, or Phase 8.

## Executive summary

The main structural gap is that the root landing prototype exists, but the public Portal surface is incomplete and current staging documentation still describes `portal.staging.algohub.web.id`. The target is one public landing root: `algohub.web.id/` in production and `staging.algohub.web.id/` in staging. `portal.localhost` remains an isolated local harness alias only. The first local slices are therefore: (1) inventory and host contract, (2) public route parity with safe blocked states where data/privacy contracts are absent, (3) shared tokens and representative responsive/a11y evidence, and (4) cross-app UI regression and closeout.

**Blocking owner decisions:** verified owner contact/legal copy and approval of public data/attachment publication. These remain `blocked-owner-data`; they are not filled with placeholders. Human UAT and physical-device/screen-reader review remain `deferred-human`.

## Status vocabulary

Only these statuses are used: `verified`, `needs-change`, `blocked-owner-data`, `deferred-human`, `not-applicable`. A percentage is not used because all denominators are explicit inventories below.

## 1. Route × app inventory

### Portal/public (`apps/portal`)

| Route | Target | Current baseline | Status | Next evidence |
|---|---|---|---|---|
| `/` | `algohub.web.id/`, `staging.algohub.web.id/` | Editorial landing prototype exists with auth-aware workspace | needs-change | responsive screenshots, axe, E2E |
| `/jadwal` | Portal public | Missing | needs-change | public-safe empty/blocked state + route E2E |
| `/asisten` | Portal public | Missing | needs-change | privacy-safe blocked state + route E2E |
| `/laporan` | Portal public | Missing | needs-change | attachment-contract blocked state + route E2E |
| `/modul` | Portal public | Missing | needs-change | attachment-contract blocked state + route E2E |
| `/bantuan` | Portal public | Missing | needs-change | static success/help state + route E2E |
| `/privacy` | Portal public | Missing | blocked-owner-data | owner/legal review required |
| `/terms` | Portal public | Missing | blocked-owner-data | owner/legal review required |
| `/info` | compatibility | Missing | needs-change | 3xx redirect to `/` |
| `/praktikum` | compatibility | Missing | needs-change | explicit host redirect, no portal admin route |
| `/login` | Auth handoff | redirect server exists | verified | existing identity tests |
| `/callback` | SSO callback | existing | verified | existing identity tests |

### Auth (`apps/auth`)

| Route group | Current | Status |
|---|---|---|
| `/`, `/login`, `/callback`, `/logout` | implemented identity shell and SSO flow | verified for existing automated scope |
| recovery/register/sessions target routes | not present in current source | needs-change / separate parity slice |

### Admin (`apps/admin`)

| Route group | Current | Status |
|---|---|---|
| shell, profile, auditlog, settings | implemented with permission-aware shell | verified for existing automated scope |
| `/praktikum/*`, `/elearning/*`, `/siakad/*` | representative admin modules exist | needs-change for full sitemap denominator |
| mobile drawer/focus/error/loading | implemented in shared shell/modules | deferred-human for physical device/manual SR |

### SIAKAD (`apps/siakad`)

| Route group | Current | Status |
|---|---|---|
| `/`, `/absensi`, `/penyewaan` | implemented data flows and states | verified for existing automated scope |
| `/dashboard`, `/profil`, `/jadwal`, `/kelas`, `/mata-kuliah`, `/nilai`, `/inventaris`, `/laporan` | target sitemap but not all current routes | needs-change / parity audit slice |

### E-learning (`apps/elearning`)

| Route group | Current | Status |
|---|---|---|
| `/`, `/leaderboard`, `/bug-hunt`, `/lesson/[id]`, `/playground` | implemented representative flows | verified for existing automated scope |
| `/courses/*`, `/profile`, `/achievements`, `/assessments`, `/monitoring`, `/auditlog`, `/terminal-demo` | target sitemap denominator not fully implemented | needs-change / parity audit slice |

### Praktikum (`apps/praktikum`)

| Route group | Current | Status |
|---|---|---|
| `/`, `/sesi/[id]`, `/attempts/[id]`, `/nilai` | implemented timer/attempt/grades representative flows | verified for existing automated scope |
| `/dashboard`, `/profil`, `/sesi`, auth compatibility routes | target sitemap denominator not fully implemented | needs-change / parity audit slice |

**Route denominator:** 11 Portal routes/groups, 4 Auth groups, 3 Admin groups, 2 SIAKAD groups, 2 E-learning groups, 2 Praktikum groups = 24 inventory rows. A route is not `verified` merely because a shell renders; API, permission, data, state, responsive behavior, and test evidence are required.

## 2. Current component/token inventory

| Area | Existing source | Finding | Status |
|---|---|---|---|
| Landing tokens | `apps/portal/src/routes/+page.svelte` local CSS variables | strong editorial tokens, but isolated from app shells | needs-change |
| Global app reset/focus | `apps/*/src/app.css` | repeated Inter/system reset and focus rule | needs-change |
| Identity shell | `packages/ui/src/AppShell.svelte` | shared but legacy visual language and inline CSS | needs-change |
| Admin shell | `packages/ui/src/AdminShell.svelte` | shared navigation/permission shell | verified for behavior; needs-change for consolidation |
| Error | `packages/ui/src/IdentityError.svelte`, `RequestError.svelte` | reusable error presentation | verified |
| Loading/empty/error | app route components | repeated local states, no unified primitive | needs-change |
| Fields/buttons/cards/tables | app route components | repeated local CSS; domain behavior must remain local | needs-change |
| Design tokens | no standalone package file | add non-secret shared tokens without new dependency | needs-change |

## 3. Plan requirement × current implementation gap

| Requirement | Evidence/current source | Gap | Status |
|---|---|---|---|
| One root landing only | `plan/ui-ux.md`, portal prototype | staging docs and env still use public portal host | needs-change |
| `/info` compatibility redirect | `plan/sitemap.md` | route missing | needs-change |
| Public Portal parity | `plan/feature-matrix.md`, legacy info routes | five public routes missing; privacy/data contracts need owner decisions | needs-change / blocked-owner-data |
| Calm technical lab DNA | portal prototype + local Burrow DESIGN | draft is directionally aligned; screenshot inspection still missing | needs-change |
| Shared tokens/primitives | `packages/ui`, repeated app CSS | tokens not centralized; primitives only partial | needs-change |
| 320/375/768/1024/1440 responsive contract | existing E2E only 320/768/1280 | five-viewport evidence missing | needs-change |
| axe/keyboard/reduced motion | existing a11y and portal tests | automated scope exists; full new public routes need coverage; manual gate deferred | needs-change / deferred-human |
| No PII/cache leak | auth BFF and public route design | public routes must fail closed when source/permission/attachment contract is absent | verified for security boundary; needs-change for public routes |
| 21st workflow | `plan/21st-mcp.md`, prior design review | no component source/dependency added; API availability must not block manual primitives | blocked-owner-data only if owner wants further exploration; otherwise not-applicable |

## 4. Host/origin mismatch inventory

| Location | Current mismatch | Correction | Status |
|---|---|---|---|
| `.env.staging.example` | `portal.staging.algohub.web.id` in allowed origins | root `staging.algohub.web.id` | needs-change |
| `docs/staging-setup.md` | Portal staging host listed as public host | landing root row; app hosts unchanged | needs-change |
| `docs/staging-dns.md` | historical portal CNAME | migration checklist uses root host; historical evidence retained and labeled | needs-change |
| `docs/staging-accounts.md` | portal staging link | root staging link | needs-change |
| `packages/auth-client/src/server.server.ts` | portal matcher accepts a public portal prefix | production/staging root explicit; local alias only | needs-change |
| `tests/frontend/local.ts` | `portal.localhost` | retain as isolated harness alias | verified / documented |
| Playwright host config | local aliases only | no remote mutation; production/staging contract tests are pure unit tests | verified |
| Vercel/DNS/remote allowlists | remote state | read-only audit only; migration checklist, no mutation | deferred-human |

## 5. Legacy parity and data-contract inventory

| Public route | Legacy source audit target | Safe target contract | Status |
|---|---|---|---|
| `/jadwal` | legacy `info/jadwal` | public schedule only after explicit publication contract; no student-specific schedule | blocked-owner-data |
| `/asisten` | legacy `info/asisten` | public names/contact only with privacy decision; otherwise safe unavailable state | blocked-owner-data |
| `/laporan` | legacy `info/laporan` | public guidance/metadata only; attachments require target model and signed/public policy | blocked-owner-data |
| `/modul` | legacy `info/modul` | public module metadata only when source/attachment model is confirmed | blocked-owner-data |
| `/bantuan` | legacy help behavior + current identity rules | static help that does not expose PII or credentials | needs-change |
| `/privacy`, `/terms` | target planning docs | owner-approved legal text, not invented | blocked-owner-data |

The implementation must not hardcode legacy file URLs, contact details, NIMs, student names, or private API responses.

## 6. Test and evidence mapping

| Evidence | Scope | Required result | Status |
|---|---|---|---|
| `npm run check` | all workspaces | zero errors/warnings | needs-change after edits |
| `npm run test:frontend` | shared/client units | all pass | needs-change after edits |
| Portal E2E | root + public routes | hard title/heading/redirect/overflow assertions | needs-change |
| A11y E2E | root/public representative | zero active axe violations, no false target | needs-change |
| Responsive screenshots | portal + representative app pages | unique artifacts at 320/375/768/1024/1440 | needs-change |
| Reduced-motion | portal + representative pages | readable finished layout | needs-change |
| Build | all frontend apps | successful builds | needs-change |
| Host contract unit | production/staging/local/illegal origin | explicit pass/fail | needs-change |
| Checksum manifest | source/evidence docs, no `.env` | sanitized reproducible hashes | needs-change |
| Human visual/device/SR | all apps | not run in 7A | deferred-human |

## 7. Dependency/license inventory

| Item | Current use | License/source | Decision |
|---|---|---|---|
| SvelteKit/Svelte/Vite | existing app foundation | existing package manifests | retain; no new dependency |
| Playwright + axe | local automated evidence | existing dev dependencies | retain |
| `@algohub/ui` | shared Svelte primitives | workspace package, private | consolidate incrementally |
| Burrow reference | design direction only | local MIT reference; attribution retained | adapt manually, no source/copy/assets |
| 21st MCP | exploration/config only | project MCP config; key stays in environment | no component source installed |
| Unsplash/Google Sans | not used by Algohub landing prototype | Burrow credits only | do not import |

## 8. Execution slices and stop conditions

1. **Inventory/docs:** create this plan and preserve historical Phase 7 evidence. Stop if a secret or credential appears in evidence.
2. **Host contract:** update local source/templates/docs and add tests. Stop before any DNS/Vercel/remote allowlist mutation.
3. **Public Portal:** add explicit routes, redirect behavior, and safe blocked/empty states. Stop if implementation would expose PII or invent owner facts.
4. **Design foundation:** add shared non-secret tokens and align representative shell styling without a rewrite. Stop if behavior/API/authorization changes.
5. **Automated review:** run check/unit/build/E2E/axe/screenshots. Preserve failures under unique artifact names; no expected skip is counted as pass.
6. **Closeout:** document residual gaps as `blocked-owner-data` or `deferred-human`; conclude `ready for human UAT`, never UAT passed or Phase 8 ready.

## Explicitly out of scope

No deployment, DNS mutation, Vercel domain binding, remote SSO allowlist change, production/legacy write, new account/invitation, human UAT/pilot, manual screen-reader/device testing, load/stress/429 staging run, alert delivery, incident injection, secret rotation, or Phase 8.
