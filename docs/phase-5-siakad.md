# Phase 5 — SIAKAD and Inventory

## Status and boundaries

The local SIAKAD/inventory core is connected to the existing identity, SSO, BFF, permissions, application factory and transaction/audit architecture. The approved scope is recorded in the contract appendix at the end of this document. This is a live completion report outside frozen `plan/`.

**Local implementation and final verification completed on 2026-11-09.** All final checks in the verification table passed; the Windows race-runtime startup failure and test-only workaround are recorded below. This is not production or full release acceptance: real Supabase/deployed-host validation, migration rehearsal, UAT/cutover and real runner sandbox acceptance remain pending. No production database, DNS, deployment, legacy source or frozen planning files were changed.

## Implemented flows

- Student: eligible active classes and schedules → open attendance session/token check-in → own attendance history and staff validation notifications.
- Borrower (`peminjam`, `mahasiswa`, `asisten`): authenticated equipment catalog → owned rental request → requested/approved/rejected/returned history and decision notifications.
- Coordinator: class/roster creation and edits with optimistic versions, schedule creation and attendance sessions, inventory authoring, rental decisions, finance verification, scoped reports/CSV and shared audit logs.
- `sekretaris`: permitted schedules and attendance; `k3`/`labops`: permitted inventory and rental decisions; `bendahara`: payment verification and permitted financial totals. Neither the admin hostname nor a different division grants additional capabilities.

Student routes on the SIAKAD host (local port 5176): `/`, `/absensi`, `/penyewaan`. Administration is on `/siakad` on the centralized admin host (local port 5175). No student-host `/admin` route is introduced.

Phase 5 adds **22 OpenAPI operations**: eight student/borrower operations and fourteen administrative operations; the combined API has **90 operations**. Identity administration and formal grades remain owned by the existing shared identity and Practicum engines rather than duplicated into SIAKAD.

## Persistence and invariants

Local forward migration `0007_siakad_inventory_core.sql` adds eight aggregate tables: `academic_classes`, `academic_schedules`, `academic_attendance_sessions`, `academic_attendance_records`, `academic_inventory_items`, `academic_inventory_rentals`, `academic_notifications`, and `academic_operations`.

- PostgreSQL and fixture repositories share the same usecase rules. Mutations, idempotency records, owned notifications and audit events commit or roll back together.
- Repository transactions serialize admission with `pg_advisory_xact_lock(731905007)` and row locks. Concurrent approvals cannot oversell stock; return restores units once.
- Item stock always satisfies `0 <= available_quantity <= total_quantity`. Inventory edits cannot declare units available while those units are still in approved rentals.
- QR tokens are cryptographically random; only their SHA-256 hash is persisted. Plaintext is returned solely by initial attendance session creation, never by replay, schedules, student responses or audit logs.
- Check-in requires active class membership, matching token, and the database clock before expiry. Duplicate commands remain idempotent while open; after expiry/closure they return `409 ATTENDANCE_CLOSED`.
- Class and schedule authorization uses actual class/shift identifiers. Attendance records/sessions carry their generated session ID. Shared resources remain division-neutral; permission-specific division grants are enriched only for authorization.
- Payment verification requires `finance.manage` and an approved/returned rental. Requested/rejected rentals cannot be paid. A returned rental may receive one-way financial verification without reopening or changing its frozen rental details.
- Quantities up to 1000 and per-unit fees up to 100000000 yield an allowed total up to 100000000000, represented consistently in Go, PostgreSQL bigint checks and OpenAPI.
- Report aggregation traverses all permitted pages. Non-finance actors receive a zero revenue field/CSV row, never another division's financial totals.
- All eight academic tables enable RLS and revoke direct `public`/`anon`/`authenticated` privileges.

The migration remains local and unshipped. Existing deployed databases must not have an applied migration rewritten; a future release must use the migration history/checksum gates in the staging guide.

## Browser safety and local parity

The same-origin BFF allowlists `/v1/academic/*`, `/v1/inventory/*` and `/v1/admin/siakad/*`, checks CSRF/idempotency metadata on mutations, denies admin routes from the student host, and returns private/no-store responses. The typed SIAKAD client keeps provider/session credentials out of browser code.

Rental retry preserves the same request bytes, absolute due date and key while its draft is unchanged, including when a committed response was lost. Attendance creation replay cannot recover the one-time token; the admin closes that session and opens a new one. New class forms do not silently pre-enroll fixture IDs, and invalid roster input is rejected rather than discarding IDs.

- Shared list pagination fetches one bounded page of 25 rows per user action. Its server offset is independent of retained selections and newly created rows, so keeping an off-page resource visible cannot skip the next server page.
- Admin tabs wait for hydration before accepting navigation. Schedule and inventory panels mount lazily, then retain drafts and pending commands while refreshing on activation.
- Pending mutation commands are isolated by method and path. An unrelated successful write cannot discard another resource's retry key. Transport/5xx failures retain the original command; a definitive `409` releases only the rejected entry.
- Attendance refresh rebases untouched fields onto canonical server data. Diverging edits remain record-local conflicts until the operator explicitly chooses server data or confirms their draft against the latest version. Refresh alone never authorizes overwriting another operator's edits.
- Class/item version conflicts require reloading, reselecting and reviewing/re-entering changes rather than silently rebasing a dirty form.

The local attendance flow is token entry/check-in, not a camera/QR-image scanner. Notifications refresh through bounded HTTP reads; no persistent WebSocket daemon is introduced. These boundaries are explicit and are not claimed as deployed device/provider acceptance.

## Verification — 2026-11-09

| Check | Actual result |
| --- | --- |
| Six-app formatting/typecheck | Final rerun passed: Prettier and all six Svelte checks, 0 errors / 0 warnings |
| Frontend unit tests | Final rerun passed: 75 tests (27 identity/BFF + 26 Practicum + 4 E-learning + 18 SIAKAD) |
| Live OpenAPI responses | Final rerun passed: 90 operations / 342 live response contracts, including closed check-in replay, requested/rejected payment denial, scoped revenue and maximum fee total; all bundles in sync |
| SIAKAD Chromium journeys | 13/13 passed in the final full-suite run: seven local-backend workflows plus six focused pagination/draft/retry regressions |
| PostgreSQL/RLS integration | Final non-race rerun passed against a newly initialized disposable loopback database, including academic lifecycle, maximum fee, pagination/reporting, concurrency, rollback and RLS regressions |
| Go cross-package coverage | 66.9% of statements using `-coverpkg=./...`; this is the current measured total, not the earlier phase's coverage |
| Go race detector | Full rerun passed across all packages, including PostgreSQL integration, with the Windows test-binary-only linker workaround below; no race reported. The original default Windows run failed before tests with ThreadSanitizer allocation error 87 |
| Six frontend production builds | Final rebuild passed for all six applications |
| Go build/vet and vulnerability scan | Final rerun passed: `go build ./cmd/api`, `go vet ./...`, `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`; scanner reported `No vulnerabilities found.` |
| Complete Phase 2–5 Chromium regression suite | 50/50 passed in 4.3 minutes, one worker, zero retries; includes SSO, identity/route guards, Practicum, E-learning and SIAKAD across six local hosts |

The seven SIAKAD workflow tests exercise the local fixture API. The six targeted pagination/draft/retry tests use mocked domain responses with real local login. Browser coverage includes 320px, 768px and 1280px viewports, but is Chromium-only; it is not a cross-browser/device certification. PostgreSQL is verified separately by the Go integration tests, not by the fixture-backed browser suite.

PostgreSQL tests require an explicitly configured, disposable loopback database whose name ends in `_test`; otherwise they skip. The tests apply migrations and create synthetic data, so never point them at staging, production or a legacy database. The fresh Phase 5 verification cluster is isolated from existing databases.

Reproducible checks:

```sh
npm run lint
npm run test:frontend
npm run openapi:check
npm run build
npm run api:build
npm run api:vet
npm run api:audit
npm run test:e2e
# With ALGOHUB_TEST_DATABASE_URL configured to the disposable loopback *_test DB:
cd backend
go test -p 1 -count=1 -coverpkg=./... -coverprofile=coverage.out ./...
go test -race -p 1 -count=1 ./...
```

The default race binary failed to start on this Windows 11 host with `ThreadSanitizer failed to allocate ... (error code: 87)`, both with and without coverage. This is a race-runtime startup failure, not a detected data race. The full suite, including PostgreSQL integration, subsequently passed with the following Windows/GCC linker workaround:

```sh
go test -race -p 1 -count=1 \
  -ldflags='-linkmode=external -extldflags=-Wl,--disable-dynamicbase,--disable-high-entropy-va' \
  ./...
```

These flags disable address randomization **only for disposable test executables**. Do not set them in global `GOFLAGS`, production builds, or Windows system/exploit-protection settings. Linux CI keeps the standard `go test -race` command. PostgreSQL acceptance requires the test database environment variable in either command; a race run without it does not verify database integration.

## Retained local evidence

The final checks ran on 2026-11-09. The following shell IDs resolve beneath the local Traycer task's `commands/705c46c9-8fbe-4cf5-9116-cabeeb225917/` directory; each contains `output.log`:

- `d678eecc-7125-4b96-bab6-3405062f27a3`: lint, frontend units, six builds, OpenAPI and 50/50 Chromium tests; exit 0 at 11:51:31 UTC.
- `11d4d0fc-54af-4c95-b856-997910db78b4`: initial Windows race startup failure followed by successful non-race PostgreSQL/coverage, build, vet and vulnerability checks; final exit 0 at 11:49:53 UTC.
- `fc72d74e-f076-4dab-bb93-6da58d58fa9a`: full race suite with the test-only linker workaround and a separate fresh PostgreSQL cluster; exit 0 at 11:52:23 UTC.

Coverage data is retained at `E:/tmp/algohub-phase5-final.7YxDMd/coverage.out`. The HTML browser report is at `playwright-report/index.html` and will be replaced by a future Playwright run. Both disposable PostgreSQL servers were stopped after testing. Go formatting was also checked with `gofmt -l backend` (no output). No source-code changes or weakening of production security settings were needed for this final verification pass.

## Ownership

- Database: `backend/migrations/0007_siakad_inventory_core.sql`
- Domain: `backend/internal/entity/academic.go`, `backend/internal/repository/academic*.go`, `backend/internal/usecase/academic*.go`
- Delivery/wiring: `backend/internal/delivery/http/academic_handlers.go`, router, configuration and application factory
- Browser: `packages/auth-client/`, `packages/siakad-client/`, `apps/siakad/`, `apps/admin/src/routes/siakad/`
- Contract/regressions: OpenAPI source/bundles, `scripts/check-siakad-contract.mjs`, academic Go tests, SIAKAD client unit tests and `tests/frontend/e2e/siakad*.spec.ts`

---

## Lampiran: kontrak implementasi asli

User authorized local Phase 5 (SIAKAD and Inventory) development following the Phase 3/4 verification pattern while real-provider/staging sign-off remains pending. No production, legacy, or frozen `plan/` changes.

### Decisions and legacy evidence

Legacy `siakad/README.md` and `siakad/src/pages/*.tsx` (`ManajemenKelas`, `ManajemenJadwal`, `BuatQR`, `Absensi`, `ValidasiAbsensi`, `Inventaris`, `SewaBarang`, `ManajemenSewa`, `LaporanKeuangan`) together with `plan/phases.md`, `plan/api.md`, `plan/erd.md`, and `plan/roles-permissions.md` define:
- Academic operations: classes (`classes`), schedules (`schedules`), QR/token attendance sessions (`attendance_sessions`), student check-in and staff validation (`attendance_records`), and cross-division reports/CSV export (`reports`).
- Laboratory inventory & rentals: equipment catalog (`inventory_items`) with stock tracking (`total_quantity`, `available_quantity`, `rental_fee`) and rental lifecycle (`requested -> approved | rejected`, `approved -> returned`, plus financial payment verification `payment_verified` by `bendahara`).
- Division capability model (`0003_identity_seed.sql`):
  - `sekretaris`: `schedule.write` and `attendance.read` (`domain:siakad, division:sekretaris`)
  - `k3` and `labops`: `inventory.manage` and `rental.approve` (`domain:siakad, division:k3|labops`)
  - `bendahara`: `finance.manage` (`domain:siakad, division:bendahara`)
  - `koordinator`: `class.manage`, `schedule.write`, `attendance.read`, `inventory.manage`, `rental.approve`, `finance.manage` (`domain:siakad`).
  - Borrowers (`peminjam`, `mahasiswa`, `asisten`): may browse available inventory items and create/view their own equipment rental requests (`owner_id = user.id`).
  - Students (`mahasiswa`): may view eligible classes/schedules, check in to open attendance sessions with a valid QR token, and read their own attendance records and notifications.

### Invariants

- All mutations authenticate, check CSRF, permission/ownership, and resource scope before domain writes. Admin hostname is never an authorization bypass.
- Division permissions are strictly enforced by usecase and scope:
  - `class.manage`: create/update classes and rosters; authorization uses the generated/existing class ID and the requested shift, with both existing and destination shift checked on update.
  - `schedule.write`: create schedules and open/close QR attendance sessions; `attendance.read` alone never allows opening or closing a session.
  - `attendance.read` or `schedule.write`: list/validate attendance records in scope.
  - `inventory.manage`: create/update inventory items and stock quantities.
  - `rental.approve`: approve, reject, or mark returned equipment rentals.
  - `finance.manage`: verify rental fee payment (`payment_verified = true`) only on `approved` or `returned` rentals and view financial revenue totals in the permitted rental scopes. Verification of `requested`/`rejected` rentals returns `409 CONFLICT`. Non-finance report responses retain the same schema with `total_verified_revenue = 0` and a zero revenue row in CSV; they never expose amounts from other divisions.
- **QR / Attendance state machine:**
  - Opening an attendance session generates a random one-time `qr_token` returned only on the initial `201` response; only its SHA-256 `token_hash` is stored.
  - Student check-in requires membership in `class.student_ids`, open session (`is_open = true` and database `clock_timestamp() < expires_at`), and constant-time match of `qr_token`.
  - Check-in is idempotent per `(session_id, user_id)` while open: repeating check-in returns the existing record without duplicating. Check-in after `expires_at` or closure returns `409 ATTENDANCE_CLOSED`, including duplicate/replayed commands.
  - Session/record scope contains the actual generated session ID and class/shift, not caller-supplied scope identifiers. Student schedule eligibility and active class membership are filtered before pagination.
  - A replay of session creation returns the existing session without `qr_token`. If its original response was lost, close the session and open a new one; the plaintext token cannot be retrieved.
  - Closing an attendance session is terminal (`closed` cannot reopen).
- **Inventory concurrency and rental state machine:**
  - `0 <= available_quantity <= total_quantity` is enforced by both usecase checks and PostgreSQL `CHECK` constraints.
  - Rental transitions: `requested -> approved | rejected`, and `approved -> returned`. Terminal states (`rejected`, `returned`) cannot transition again.
  - Approving a rental locks the item and rental in transaction order (`pg_advisory_xact_lock(731905007)` + row lock), checks `available_quantity >= quantity` (returning `409 INSUFFICIENT_STOCK` on concurrent exhaustion), and decrements `available_quantity` atomically.
  - Returning an `approved` rental increments `available_quantity` by `quantity` exactly once. Trigger `academic_freeze_rental` freezes terminal status and rental details; the only post-return change allowed is one-way financial verification (`false -> true`).
  - Stock edits cannot inflate availability over outstanding approved loans: `total_quantity - available_quantity >= sum(approved rental quantities)` must hold.
  - Quantities are bounded to `1..1000` and per-unit rental fees to `0..100000000`; `total_fee` is bounded to `100000000000` with matching bigint database checks and OpenAPI schema.
  - A browser retry preserves the original request bytes, absolute `due_at`, and `Idempotency-Key` while the user's draft is unchanged; elapsed client time cannot create another request.
- Admin and student lists use bounded `limit`/`offset` pages; reports traverse all permitted pages rather than silently truncating totals.
- Status changes on attendance validation and rental decisions create owned in-app notifications (`academic_notifications`) and transactional `audit_logs` records without leaking QR tokens.
- RLS and privilege revocations deny direct `anon`/`authenticated` access on all `public.academic_*` tables.

### API wire shapes

All paths start with `/v1`. Mutations require `Idempotency-Key` (1..128 safe characters) and CSRF.

#### Student / Borrower routes

| Method | Path | Operation ID | Body / response data |
|---|---|---|---|
| GET | `/academic/classes` | `academic.classes.list` | `AcademicClass[]` |
| GET | `/academic/schedules` | `academic.schedules.list` | `AcademicScheduleView[]` (includes open attendance session summary without `qr_token`) |
| GET | `/academic/attendance` | `academic.attendance.list` | `AcademicAttendanceRecord[]` (owned records) |
| POST | `/academic/attendance` | `academic.attendance.checkin` | `{session_id, qr_token}` -> `AcademicAttendanceRecord` |
| GET | `/inventory/items` | `inventory.items.list` | `InventoryItem[]` |
| GET | `/inventory/rentals` | `inventory.rentals.list` | `InventoryRental[]` (owned rentals) |
| POST | `/inventory/rentals` | `inventory.rentals.create` | `{item_id, quantity, purpose, due_at}` -> `InventoryRental` (201) |
| GET | `/academic/notifications` | `academic.notifications.list` | `AcademicNotification[]` (owned notifications) |

#### Admin SIAKAD routes

| Method | Path | Operation ID | Body / response data |
|---|---|---|---|
| GET | `/admin/siakad/kelas` | `academic.admin.classes.list` | `AcademicClass[]` |
| POST | `/admin/siakad/kelas` | `academic.admin.classes.create` | Class input -> `AcademicClass` (201) |
| PATCH | `/admin/siakad/kelas/{id}` | `academic.admin.classes.update` | Class update + `version` -> `AcademicClass` |
| GET | `/admin/siakad/jadwal` | `academic.admin.schedules.list` | `AcademicScheduleView[]` |
| POST | `/admin/siakad/jadwal` | `academic.admin.schedules.create` | Schedule input -> `AcademicSchedule` (201) |
| POST | `/admin/siakad/absensi/sesi` | `academic.admin.attendance.open` | `{schedule_id, title, duration_minutes}` -> `AcademicAttendanceSession` (201, includes one-time `qr_token`) |
| POST | `/admin/siakad/absensi/sesi/{id}/close` | `academic.admin.attendance.close` | `{}` -> `AcademicAttendanceSession` |
| GET | `/admin/siakad/absensi/records` | `academic.admin.attendance.records` | `AcademicAttendanceRecord[]` |
| PATCH | `/admin/siakad/absensi/records/{id}` | `academic.admin.attendance.validate` | `{version, status, validated, notes}` -> `AcademicAttendanceRecord` |
| POST | `/admin/siakad/inventaris` | `inventory.admin.items.create` | Item input -> `InventoryItem` (201) |
| PATCH | `/admin/siakad/inventaris/{id}` | `inventory.admin.items.update` | Item update + `version` -> `InventoryItem` |
| GET | `/admin/siakad/penyewaan` | `inventory.admin.rentals.list` | `InventoryRental[]` |
| PATCH | `/admin/siakad/penyewaan/{id}` | `inventory.admin.rentals.decide` | `{version, action}` (`approve\|reject\|return\|verify_payment`) -> `InventoryRental` |
| GET | `/admin/siakad/laporan` | `academic.admin.reports.get` | `AcademicReportSummary` (metrics + sanitized `csv_export`) |
