# Phase 5 implementation contract

User authorized local Phase 5 (SIAKAD and Inventory) development following the Phase 3/4 verification pattern while real-provider/staging sign-off remains pending. No production, legacy, or frozen `plan/` changes.

## Decisions and legacy evidence

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

## Invariants

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

## API wire shapes

All paths start with `/v1`. Mutations require `Idempotency-Key` (1..128 safe characters) and CSRF.

### Student / Borrower routes

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

### Admin SIAKAD routes

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
