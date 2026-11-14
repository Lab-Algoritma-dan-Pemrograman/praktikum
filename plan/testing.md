# Testing, CI/CD Quality Gate, dan Debugging

**Status:** Wajib untuk semua fase implementasi

## 1. Prinsip

Setiap feature harus punya test pada layer yang sesuai sebelum statusnya berubah
menjadi `verified`. Test bukan hanya happy path; wajib mencakup unauthorized,
empty state, timeout, retry, duplicate request, mobile layout, dan observability.

## 2. Matrix test per bagian

| Bagian | Unit | Integration/contract | E2E/smoke | Security/performance |
|---|---|---|---|---|
| Auth/SSO | token/session/return_to | Supabase Auth + cookie | login silang lima host, logout, revoke | CSRF, session fixation, brute force |
| Authorization | role/division/scope policy | API 401/403/404 matrix | admin menu dan direct URL | IDOR/BOLA, privilege escalation |
| Database | mapper/validation | migration + FK/RLS + repository | critical CRUD | SQLi, lock/deadlock, pool |
| API/OpenAPI | DTO/error/idempotency | schema validation | `/docs` + critical endpoints | rate limit/429, body limit |
| Praktikum | timer/state transition/grading | attempt/autosave/submit transaction | start → answer → submit → grade | race submit, clock tampering |
| E-learning | progress/XP/streak | lesson/progress/game persistence | lesson → run → complete | XP duplication, cross-user read |
| SIAKAD | schedule/attendance/rental rules | RPC replacement/repository | jadwal → QR → validasi | scope leakage, audit |
| Admin | menu/permission matrix | bulk action/audit | switcher → module → mutation | CSRF, audit completeness |
| Storage | path/MIME/size | signed URL/object metadata | upload → view/download | traversal, public leakage |
| Code runner | limits/result mapping | fake provider/retry | editor → run/test | sandbox escape, quota |
| Frontend | component/form/state | API mock/contract | Playwright responsive flow | axe/keyboard/CSP |
| Deployment | config/health | preview smoke/cron | staging cutover rehearsal | secret scan/load/chaos |

## 3. CI job wajib

```text
secret-scan
dependency-audit
format-lint-typecheck
go-vet-unit-race
db-migration-lint-integration
openapi-lint-contract
security-negative-tests
build
playwright-smoke
artifact-upload
```

PR tidak boleh merge bila test gagal atau artifact debugging tidak tersedia.

## 4. Debugging contract

Setiap error wajib dapat ditelusuri dengan:

- `X-Request-ID` yang sama pada UI, API, log, dan provider;
- environment, deployment SHA, operation ID, user scope hash, dan timestamp UTC;
- sanitized request summary (tanpa password/token/source sensitif);
- error code stabil dan human-readable message;
- link artifact CI: test report, screenshot/video E2E, coverage, migration log,
  response sample, dan structured log.

Test tidak boleh mencetak credential, service key, token, jawaban ujian, atau data
pribadi penuh ke artifact.

## 5. Definition of Done feature

1. Unit dan contract test lulus.
2. Integration test meliputi data kosong, invalid, unauthorized, duplicate, retry.
3. E2E critical flow lulus pada mobile dan desktop.
4. Security negative test lulus.
5. Metrics/log/error code tersedia.
6. Migration/rollback note diperbarui bila schema berubah.
7. OpenAPI, feature matrix, dan changelog diperbarui.
