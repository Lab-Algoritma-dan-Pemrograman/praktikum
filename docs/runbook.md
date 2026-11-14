# Runbook Operator — Algohub staging (Phase 7 template)

Template operator. Kolom kontak/petugas nyata wajib dikonfirmasi pemilik sebelum drill; AI tidak mengarang nama.

## 1. Triage awal

1. Cek `https://api.staging.algohub.web.id/healthz` (200, `ok`) dan `/readyz` (200, `ready`). Readyz gagal = database staging tak terjangkau.
2. Ambil `X-Request-ID` dari error UI/user report; cocokkan ke log API.
3. Klasifikasi severity:

| Sev | Kriteria | Respon |
| --- | --- | --- |
| S1 | Login/SSO semua host mati, data corrupt, kebocoran credential | Escalate langsung ke pemilik; freeze perubahan |
| S2 | Satu host down, scheduler berhenti >15 menit, error spike | Investasi root cause; komunikasi ke pengguna pilot |
| S3 | Fitur non-kritis, minor a11y, p95 melonjak tanpa error | Catat issue, backlog |

## 2. Request-ID lookup

- Middleware: `backend/internal/middleware/requestid.go` + `logging.go`. Setiap request API membawa request-ID yang sama di UI/API/log.
- Jangan membaca token/queue headers/Vault values saat lookup.

## 3. Perintah cepat

| Keperluan | Perintah |
| --- | --- |
| Regresi backend | `npm run api:test` (PG: `ALGOHUB_TEST_DATABASE_URL=... go test -count=1 ./...` pada loopback `_test` baru) |
| Vet/build | `npm run api:vet && npm run api:build` |
| Frontend check/build | `npm run check`, `npm run build` |
| Unit frontend | `npm run test:frontend` |
| E2E Chromium | `npx playwright test` |
| OpenAPI contract | `npm run openapi:check` (91 operasi) |

E2E harus memiliki seluruh target server. Jika port default dipakai project lain, gunakan port terisolasi, misalnya `ALGOHUB_E2E_APP_PORT_BASE=5273 ALGOHUB_E2E_API_PORT=8180 npx playwright test`. Jangan memakai config existing-server tanpa memverifikasi kepemilikan semua origin; config tersebut juga memerlukan `ALGOHUB_E2E_USE_EXISTING_SERVERS=true` secara eksplisit.
| Dependency Go | `npm run api:audit` (govulncheck) |

## 4. Scheduler (pg_cron/pg_net)

- Job: `algohub-staging-practicum-reconcile`, per menit, POST `/v1/internal/practicum/reconcile`, secret dari Vault.
- Verifikasi: `cron.job`, `cron.job_run_details`, `net._http_response` (retensi ~6 jam!). `succeeded` di cron hanya berarti enqueue; sukses HTTP lihat `net._http_response`.
- Pause/resume: `cron.alter_job(..., active := false/true)` — **butuh izin pemilik**.
- Jangan ubah cadence/secret tanpa izin. Prosedur rotasi ada di `docs/staging-scheduler.md`.

## 5. Recovery dan rollback limits

- Backup yang teruji: **sanitized public state + noncredential Auth metadata** (`docs/phase-6-closeout.md` §5). Tidak memulihkan password/MFA/provider sessions/audit/notifications/operations/leases.
- Restore hanya ke owned local `_test` database (prosedur `scripts/phase6-recovery-verify.py`); tidak pernah di atas pekerjaan pengguna tanpa preserve.
- Row rollback tidak memutar sequence; sequence restore terbukti via pg_dump.
- Rollback aplikasi = redeploy deployment ID Vercel sebelumnya (catat deployment ID tiap deploy di bukti).

## 6. Komunikasi dan eskalasi

| Peran | Nama | Kontak |
| --- | --- | --- |
| Pemilik keputusan | (konfirmasi pemilik) | |
| Koordinator pengguna nyata | (konfirmasi pemilik) | |
| Support pilot | (konfirmasi pemilik) | |

Eskalasi: tester → support → pemilik. Insiden data/security langsung pemilik.

## 7. Checklist verifikasi pasca-incident

1. healthz/readyz 200.
2. Login/SSO satu host nyata 200 + logout 401.
3. Scheduler: satu delivery cron HTTP 200 (`{expired:0,jobs_processed:0}` cukup).
4. Nol sesi acceptance aktif (`acceptance-session-final` query pattern).
5. Catat insiden: waktu, request-ID, akar masalah, tindakan, bukti path.
