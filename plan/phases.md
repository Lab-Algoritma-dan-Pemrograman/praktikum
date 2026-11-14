# Execution Phases dan Gate

**Status:** Planning roadmap

## Phase 0 — Audit dan freeze keputusan

Output: feature matrix, role matrix, schema inventory, code-runner decision,
OpenAPI principles, risk register. Test: source inventory check dan review parity.
Gate: tidak ada P0 tanpa owner, source evidence, dan acceptance test.

## Phase 1 — Contract dan foundation

Output: monorepo, SvelteKit apps, Go composition root/serverless adapter, OpenAPI,
CI, environment template, logging/request ID, design tokens. Test: build, lint,
unit, OpenAPI contract, preview smoke.

## Phase 2 — Identity, SSO, authorization

Output: Supabase Auth, profile, role/division/permission/scope, centralized admin
shell, audit event. Test: login silang lima host, cookie/CSRF, full 401/403/IDOR
matrix, mobile navigation.

## Phase 3 — Practicum core

Output: session, activation, question, attempt, autosave, timer, submit, grading,
admin Praktikum. Test: transaction/race, GMT+7, refresh, close-course auto-submit,
runner integration, E2E student/asisten.

## Phase 4 — E-learning

Output: level/module/lesson, progress, XP, game, achievement, leaderboard,
content admin. Test: duplicate completion/XP, RLS/API scope, offline/reconnect,
responsive editor and code lab.

## Phase 5 — SIAKAD dan inventory

Output: class, schedule, attendance, inventory, rental, reports, admin SIAKAD.
Test: division permission, QR/attendance state, inventory concurrency, export,
audit, notification.

## Phase 6 — Migration rehearsal

Output: staging schema, transform scripts, file copy, reconciliation report,
rollback script, dry-run metrics. Test: repeatable dry-run, counts/FK/checksum,
backup restore, representative user flows.

## Phase 7 — UAT dan pilot

Output: UAT sign-off, accessibility/performance/security report, pilot cohort,
runbook and support playbook. Test: browser matrix, load, 429, alerting, cron,
incident drill.

## Phase 8 — Cutover dan stabilization

Output: final delta import, DNS/TLS switch, monitoring, rollback window, legacy
read-only plan. Test: smoke immediately after cutover, data reconciliation,
cross-host login, critical business flows.

Tidak boleh melompati gate hanya karena UI sudah selesai. Setiap phase menyimpan
test artifacts dan keputusan unresolved di changelog/risk register.
