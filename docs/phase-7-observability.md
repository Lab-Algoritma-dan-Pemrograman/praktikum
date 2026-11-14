# Phase 7 — Observability dan Incident Drill (draft; eksekusi butuh approval)

Status: **sebagian tervalidasi read-only**. Verifikasi alerting nyata dan incident drill staging butuh persetujuan pemilik.

## 1. Yang sudah ada (read-only validasi)

- Request-ID: middleware `requestid.go` — tiap request API membawa request-ID sama di UI/API/log.
- Logging middleware: `logging.go`; error berperjalanan frontend → API → log.
- Recovery middleware: `recovery.go` (panic → 500 + request-ID).
- Timeout middleware: `timeout.go`.
- Scheduler: pg_cron/pg_net `algohub-staging-practicum-reconcile` per menit; verifikasi sukses via `net._http_response` (retensi ~6 jam); runbook di `docs/staging-scheduler.md`.
- Recovery drill Phase 6: 7 gates restore backup → owned local `_test` DB, preseden untuk incident drill.

## 2. Gap alerting — belum delivery proof

Per `plan/deployment.md`, alert yang seharusnya dinilai: error spike, 429 spike, DB connections/quota, storage nearing limit, failed cron, backup failure. **Status: tidak ada alerting terkonfirmasi live pada staging.** Rencana alert saja bukan bukti delivery. Perlu audit nyata + approval pemilik sebelum alert ke penerima nyata.

## 3. Verifikasi error travel (dapat dijalankan read-only, memakai akun retained)

1. Login 1 akun retained → panggil 1 endpoint dengan body error → tangkap request-ID.
2. Cocokkan request-ID di log API staging (via read-only query, tanpa menyentuh token/Vault).
3. Konfirmasi status code API konsisten (422/400/401/403/404/429).

## 4. Incident drill (butuh approval — isolated → bounded staging)

**Fase 1 (isolated, local, siap sekarang):** fault injection pada owned local `_test` database + harness lokal (restore baseline, corrupt-then-restore, rollback). Tidak menyentuh staging.

**Fase 2 (bounded staging drill, butuh approval spesifik):**
- Target: satu endpoint baca, satu endpoint mutasi, scheduler reconcile.
- Tindakan: simulasikan error/connection drop terbatas pada request terisolasi (bukan mematikan seluruh service/deployment).
- Stop conditions: tanpa downtime pengguna, tanpa data loss, tanpa scheduler cadence change, tanpa menghabiskan kuota.
- Recovery: restore pola Phase 6 7-gates ke local copy; cleanup.
- Owner approval wajib sebelum Fase 2; tidak ada fault injection pada staging tanpanya.

## 5. Template alert (isi keputusan pemilik)

| Alert | Kondisi | Severity | Penerima | Saluran |
| --- | --- | --- | --- | --- |
| Error spike | error rate > threshold 5 menit | S2 | (isi) | (isi) |
| 429 spike | 429 rate abnormal | S3 | (isi) | (isi) |
| Failed cron | cron enqueue/delivery gagal | S2 | (isi) | (isi) |
| DB connection/quota | pooler/quota mendekati limit | S2 | (isi) | (isi) |
| Backup failure | restore/dump gagal | S2 | (isi) | (isi) |

## 6. Status

- Read-only ✅: request-ID/log/recovery/timeout middleware ada; error travel dapat diverifikasi.
- Blocked 🔒: verifikasi alerting delivery nyata (G7), incident drill staging (G8), alert test ke penerima nyata. Semua butuh izin spesifik pemilik.
