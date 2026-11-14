# Phase 6 — Migration rehearsal

Status: **Phase 6 100% selesai pada scope rehearsal masked yang dipilih dengan izin pemilik, 13 November 2026**. Lihat [closeout final](phase-6-closeout.md) dan [evidence](evidence/phase-6-final-2026-11-13.json). Bagian local synthetic di bawah adalah runbook/bukti awal; real masked staging import, recovery, browser acceptance dan cleanup juga sudah terbukti. Bukan migrasi legacy lossless atau production cutover; Phase1–5 tetap closed.

## Safety boundary

- Work in the existing rewrite workspace; never write to the legacy application.
- No legacy rows, PII, credentials, or uploaded files are copied without separate approval of source, scope, masking, retention, and destination.
- No staging project/account/DNS/scheduler provisioning. Retained acceptance data and cron are out of scope.
- No runner/provider changes. E-learning XP remains browser-trusted; Praktikum grades remain manual.
- Inspect migration history and checksums first. Never reapply migrations 0002–0007 on a matching target.
- Local databases must be newly created, loopback-only, disposable, and named with `_test`. Every command and transaction needs a deadline. Do not connect the integration harness to staging.
- Reports contain counts, classifications, checksums, and timing—not raw rejected rows or secrets.
- Database backup/restore must be tested before any approved remote import. Never restore an entire backup over shared staging as an automatic rollback.

## Baseline and evidence

The workspace is not a Git repository. Before implementation, 354 rewrite source/document files were archived (excluding environment/credential files, dependencies, generated outputs, static assets, and legacy sources/data):

- Evidence directory: `E:/tmp/algohub-phase6-20261113-zkaqtokq/`
- Archive: `source-before.zip`
- SHA-256: `bdc586643489d9ea3ece1f4af53b469a3e6967975d8c650a35dab712ab45d74a`
- Per-file checksums: `source-before.json`

Anonymous read-only staging smoke returned health/readiness 200 and auth 401. See `staging-public-preflight.json`. This verifies the public endpoint only; it does **not** establish current database identity, migration history, or permission to write.

Native PostgreSQL 18.3 and Go 1.26.11 are available. Docker daemon is unavailable; no Docker startup is required for the planned native loopback rehearsal.

## Verification contract (declared before the run)

| Layer | Required evidence |
| --- | --- |
| Transform unit | Stable IDs/output under reorder/retry, strict input validation, no implicit identity merge, enum/null/money/time handling, rejection reason for every excluded row |
| Import integration | Transactional load in FK order; constraints remain enabled; second identical import changes nothing; conflicting existing data fails rather than overwrites |
| Reconciliation | Input rows = imported + rejected; exact accepted counts and content checksums; zero FK/orphan/duplicate violations; unexpected target rows detected |
| Recovery | Pre-import backup restored into a second disposable DB; restored baseline matches; rollback rehearsal restores baseline; import after recovery has the same checksums |
| User flows | Test application reads/mutations against imported synthetic data, separate from existing fixture-only tests; explicitly distinguish local/fake-provider identity tests from real Supabase/browser SSO |
| Cleanup | Stop only the owned cluster; remove owned temporary database data; retain manifests, metrics, test output, and failures |

No arbitrary rejected-row tolerance is accepted for a real snapshot. Local fixture expectations must be declared before import; real dataset tolerances require owner approval.

## Local operator runbook

Prerequisites: Go 1.26.11+, Python 3 (standard library only), PostgreSQL 18 native `initdb`, `pg_ctl`, `psql`, `createdb`, `pg_dump`, and `pg_restore` on PATH. Docker, cloud accounts, API keys and database credentials are not required. Run from the rewrite workspace:

```sh
python -I scripts/phase6-rehearsal.py --evidence-parent E:/tmp
```

The script creates a **new** evidence directory and loopback-only cluster, with a unique port and `_test` database names. It does not attach to an existing service. Generated target files contain only run/database/cluster identity and port, not connection credentials. All CLI database actions check the literal-loopback endpoint, database comment, data directory and PostgreSQL system identifier. PostgreSQL ambient service/passfile/credential configuration is excluded. Do not bypass these guards or edit a target file to use shared staging.

### Executed order

1. Build the local CLI and run unit tests; pin source checksums.
2. Generate the versioned fixture: **23 input rows, 21 accepted, 2 explicitly rejected**. Accepted rows: 2 identities, 2 assignments, and one row in each of 17 domain tables. Rejections: one excluded auth-session placeholder and one unsupported ledger placeholder, neither containing credentials or real data.
3. Transform twice and require identical plans. References are qualified `source-system/entity/source-ref`; only declared reference fields accept the special synthetic placeholder.
4. Initialize only the empty owned database. Auth UUID stubs, canonical access catalogs, and an unrelated profile sentinel belong to baseline. Read-only preflight must not change that baseline.
5. Dump baseline; restore into a different new DB and require identical schema/table/sequence fingerprints.
6. Import atomically; reconcile every accepted/rejected input, all SQL FKs/unique indexes, exact per-table deltas and full table content checksums. Import again; require zero inserts, all accepted rows unchanged, and no sequence changes.
7. Dump populated state and restore into a separate acceptance DB. Exercise imported identities and domain records through real PostgreSQL repositories/usecases and local fake-provider HTTP login/SSO. This is **not real Supabase or browser acceptance**.
8. Show that a no-op receipt deletes nothing, then rollback using the **first** apply receipt. Restore baseline rows and schema, including the unrelated sentinel; leave identity sequences monotonic.
9. Restore baseline into another new DB and reimport. Require the same populated fingerprint, including sequences, as the original import.
10. On separate disposable copies, prove wrong-target receipt rejection, late-conflict row atomicity, changed-user and new-dependent rollback refusal, and schema-drift preflight refusal.
11. Stop only the owned cluster; remove its data directory/databases. Retain dumps, reports, source checksums, commands/timing, and failures. A run is not successful if cleanup cannot be confirmed.

### CLI and evidence

Source: [local CLI](../backend/cmd/rehearsal/main.go), [transform/loader](../backend/internal/migrationrehearsal/), [native harness](../scripts/phase6-rehearsal.py), [imported-data flow tests](../backend/internal/repository/migration_rehearsal_postgres_test.go).

CLI commands are `fixture`, `transform`, `init`, `preflight`, `apply`, `reconcile`, and `rollback`; invoke a command with `-h` for its flags. `init` refuses any populated application schema. `preflight` does no DDL. `apply` and `rollback` require the exact batch confirmation. Output paths must be new; reports are never silently overwritten. `--baseline` is required for existing-database commands and pins the structural fingerprint from initialization.

Read `summary.json` first, then `baseline.json`, `apply-first.json`, `apply-repeat.json`, `reconcile-first.json`, `rollback-first.json`, and the numbered logs. Each receipt contains full before/after **hashes and counts**, not raw database rows. Synthetic fixture and transform artifacts do contain fictional payloads. Backups contain only synthetic local data.

### Recovery boundaries

- Do not use blanket UPSERT: submitted/completed histories have immutable triggers.
- Do not use the second/no-op receipt to undo the first import; it owns no new rows.
- Rollback refuses if **any** table/schema/sequence changed since that receipt. This deliberately protects new child rows from cascade deletion or audit `SET NULL`. It is suitable for a quiescent owned local rehearsal, not an online shared-staging rollback algorithm.
- Row rollback does not rewind identity sequences. Failed transactions may consume sequence values too. Exact sequence restoration is proved through baseline dump/restore into a fresh database, never an unconditional `setval` on a shared service.
- If the process dies after commit but before receipt persistence, ownership cannot be reconstructed from an equal-row retry. Preserve evidence, treat the receipt as missing, and recover through a tested baseline restore into a **new owned DB**. The harness validates that recovery route, not arbitrary process-crash durability.
- If cleanup fails, keep the ownership manifest and data directory, inspect its own `pg_ctl status`, then stop that exact cluster. Never kill all PostgreSQL processes or delete a parent temp directory.

## Decisions still needed before a real-data/remote run

1. Exact snapshot location, format, source version, capture time, and approved table/file scope.
2. Permission to process/copy any real PII; masking policy, retention period, and deletion owner.
3. Approved identity crosswalk and treatment of ambiguous/missing legacy relationships.
4. Target database identity verified without printing credentials; write permission limited to the rehearsal dataset.
5. Expected counts, accepted rejection reasons/tolerances, rollback boundary, and maintenance window if necessary.

Until these are resolved, real-data migration and remote imported-data acceptance remain blocked—not passed or silently skipped.
