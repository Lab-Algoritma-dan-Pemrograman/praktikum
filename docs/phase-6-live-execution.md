# Phase 6 — live rehearsal execution (active tracker)

Status: **selesai untuk scope rehearsal masked, 13 November 2026**. Tracker historis berikut dipertahankan sebagai jejak investigasi; status/gate/batas izin terbaru adalah [closeout final](phase-6-closeout.md) dan [evidence](evidence/phase-6-final-2026-11-13.json). Import19.026 receipt rows,157 final checks,31 browser checks,7 recovery gates dan regresi170 top-level tests terbukti; nol sesi acceptance aktif/owned pgdata tersisa. Tidak ada production cutover atau klaim migrasi lossless rejected history.

## Explicit owner authorization — 2026-11-13

The owner asked this agent to execute the remaining Phase 6 gates. In the permission questions they answered:

- Source: **check the live DB directly and check the old project**, rather than selecting the old local backups.
- Data handling: **allow local isolated PII processing, then masked import to the existing dedicated rewrite staging**. Only PII-free files may be copied. Migration-specific Auth identities may be created if needed. No legacy passwords/tokens copied, no production writes, no new projects, no scheduler changes. Remove raw working copies after verification; retain original source backups.

This does not permit inventing missing mappings, silently dropping ambiguous records, or claiming 100% before the gates pass.

## Current evidence

Evidence directory: `E:/tmp/algohub-phase6-live-20261113-f9aelu0c/`.

- New bounded read-only CLI `backend/cmd/phase6-inspect` consumes existing private config internally and writes only table/column metadata and aggregate counts. It uses a read-only transaction, TLS, query/connection deadlines, no migration/write commands and sanitized errors.
- Live legacy PostgreSQL from `project_ap/praktikum/backend/.env`: **PostgreSQL17.6, 107 public tables, 396 users, 16,576 student_progress, 1,634 pengerjaan_course, 3,746 jawaban_mahasiswa, 678 attendance_logs**. Source contains both standalone English EL/SIAKAD tables and alternative consolidated Indonesian tables. Many alternatives are empty or replica copies; never blindly union them.
- Existing rewrite staging: **PostgreSQL17.11, 38 public tables, 30 retained users, six migrations0002–0007**; source/target endpoint and project fingerprints differ. No migrations rerun or remote writes performed.
- Legacy SIAKAD configuration REST inventory: **26 relations, 323 users, 3 attendance_logs, 3 inventory_items, 3 inventory_rentals**. This is a different project from the107-table DB; its current authority still needs verification rather than automatic merging.
- Legacy E-learning `.env` REST target was unreachable (`URLError`). Its active browser client hardcoded project fallback fingerprints to the107-table DB, not that old `.env` URL. The mounted SupabaseProvider queries English `users/student_progress/levels/modules/lessons` directly; Go `/belajar` route existence does not establish frontend use.
- The `.env.vercel.production` / production export files are not usable source config for this inspection (invalid endpoint / absent parsed host); do not print values or treat those as deployment truth.
- Public probes of `elearning.algohub.web.id` and `siakad.algohub.web.id` returned HTTP errors. They did not establish deployed source identity. No login or app writes were attempted.

Relevant artifacts:

- `legacy-praktikum-local-metadata.json`
- `rewrite-staging-metadata.json`
- `source-config-fingerprints.json`
- `legacy-siakad-api-metadata.json`
- `legacy-elearning-api-metadata.json` (failure preserved)
- `deployed-source-fingerprints.json` (failure preserved)

Only schema/aggregate metadata has been captured so far; no legacy user records or uploads have been exported yet.

## Source routing reviewed

A bounded Explore agent examined12 active code files:

- EL `src/App.tsx` mounts `SupabaseProvider`; it directly calls Supabase. `src/lib/supabase.ts` uses VITE variables or the107-table project's hardcoded fallback. Database-empty/error curriculum can fall back to static content.
- SIAKAD `src/integrations/supabase/client.ts` intercepts RPC to same-origin `/api/rpc`; server handler uses Supabase service credentials, preferring VITE_SUPABASE_URL. Other Supabase methods pass through.
- Praktikum `frontend/src/lib/api.ts` uses PUBLIC_API_BASE_URL; Go composition root shares one PostgreSQL connection among practicum/learning/game repositories. Supabase storage is separate wiring.

## Next execution steps

1. Resolve active source lineage using non-secret deployment/config evidence and metadata freshness; never merge divergent source generations by assumption.
2. Capture a bounded read-only consistent snapshot excluding every credential/token column. Mask identifiers/free-text before any staging upload; keep raw working data private and outside reports.
3. Quantify actual mapping issues (duplicate identity, fractional grades, curriculum/game variant, attendance linkage, stock/financial/file semantics) before choosing adapters and exact reconciliation expectations.
4. Implement/test actual masked-source adapters; retain explicit dispositions and evidence for unsupported/ambiguous records rather than claiming lossless migration.
5. Snapshot/recovery of existing staging must exclude credential leakage and protect retained acceptance/scheduler data. Current local importer/rollback guards must not be weakened into a blanket remote mode.
6. Perform the approved staged import, repeatability/reconciliation, file-copy verification, real Supabase/browser imported-data acceptance, dataset-scoped cleanup, and final evidence/handover.

Earlier completed local evidence remains valid at `docs/phase-6-closeout.md` and `docs/evidence/phase-6-local-2026-11-13.json`; no need to repeat it merely for status. Any new relevant implementation must receive appropriate fresh verification.

## Defects found and corrected during masked rehearsal

Every failure below was observed on a disposable loopback database; staging was not written before these were fixed.

1. Insert-verification compared against zero-valued locals after a fresh insert (users and assignments), reporting false conflicts. Fixed by re-reading the inserted row before comparison.
2. Assignment scope was passed as raw bytes, so PostgreSQL could not infer the `jsonb` parameter type. Fixed with explicit `json.RawMessage` binding.
3. Auth shim lacked the columns Supabase provides; the rehearsal now initializes a Supabase-like `auth.users` + `auth.identities` so the same write path as staging is exercised.
4. `jsonb_build_object` parameters could not be typed by PostgreSQL (SQLSTATE 42P18). Fixed with explicit `::text`/`::uuid`/`::timestamptz` casts.
5. Legacy schedule days are uppercase (`KAMIS`, `JUM'AT`) but the target constraint accepts `Senin`–`Sabtu`. Added an explicit alias map; unknown days are now rejected with `unsupported_schedule_day`.
6. Legacy curriculum `sort_order` starts at 0 while the target requires 1–1000. Sort orders are deterministically renumbered within each parent, preserving legacy order.
7. Rollback check compared raw `psql` output including the `SET` command tag. Harness parsing fixed; no data assertion weakened.
8. The imported-data flow test expected explicit per-user grants to be non-empty. Investigation showed the application authorizes via explicit `user_permission_grants`, so a migration that imports only role assignments would produce users who cannot open any lesson. **This was a real migration defect**: the importer now materializes the canonical role/division default grants once per newly inserted assignment, and rollback removes them with the assignment.
9. Imported inventory items are legacy zero-fee, non-rentable equipment. The lifecycle test now selects any available imported item and accepts fee 0 instead of requiring a payable item; the stock/payment invariants are unchanged.
10. Re-running preflight after an import flagged this batch's own rows as conflicts. Both the identity (email/NIM) and business-key (class/item code) checks now exclude rows owned by the receipt's batch.
11. The database rejected manual grading with `submitted answers are immutable`: the imported attempt JSON contained empty optional fields (`"score": null`, `"feedback": ""`) that the Go entity drops on read/write, so the freeze trigger saw a phantom change. The importer now re-encodes every payload through its entity type before insert, making stored JSON identical to a normal round-trip; this was verified against the PostgreSQL server log, not by loosening the trigger.

## Open verification at this point

- Masked plan: 19.816 imported / 60.533 rejected of 80.349 source rows. Rejections are dominated by archive/operational tables outside the target model, achievement and whole-game history whose semantics cannot be preserved, and attendance rows without provable schedule linkage. Details: [dataset manifest](phase-6-dataset-manifest.md).
- Local rehearsal of the final plan (v3) **passed every gate** in `E:/tmp/algohub-phase6-masked-local-mfe3ooas/summary.json`: 19.026 rows imported and fingerprinted, repeat import fully unchanged, rollback removed the dataset, populated backup restored exactly, and the imported-data flow suite produced 10 passing events (identity/access, fake-provider login+SSO, zero-XP learning replay, manual grading publish/replay, attendance duplicate/close, zero-fee rental lifecycle, scoped report, untouched baseline identities, plus the parent test).
- The staging run (preflight → backup → atomic import → repeat → reconcile → restore drill → imported flows on the restored copy → real Supabase login and staging API reads) is recorded in its own evidence directory and only counts when its `summary.json` says `passed`.
- Staging status so far: read-only preflight passed; sensitive-excluding backup and data-only backup produced; **atomic import committed 19,026 rows** (`inserted=19026`, 396 auth identities + 396 profiles + 396 assignments + 17,838 domain rows). The first reconciliation attempt exceeded its 300 s budget because it scanned every JSON payload union; the check was narrowed to the three tables that can carry unresolved bind markers and deadlines were raised. Verification then resumed from reconciliation without re-importing.

