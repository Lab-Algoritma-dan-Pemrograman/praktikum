# Phase 7 — Performance, Load, dan 429 Plan (draft; eksekusi butuh approval)

Status: **usulan workload**. Remote load run dan uji 429 di staging butuh persetujuan pemilik (target, tindakan, request budget, stop conditions, recovery). Dokumen ini menetapkan desainnya; tidak ada run dijalankan.

## 1. Target evaluasi (bukan klaim tercapai)

- p95 API normal < 500 ms; error rate < 1% (`plan/prd.md`).
- **Gap terbukti:** baseline baca lama 30 user/300 request menghasilkan p95 843,69 ms — melebihi target. Bukti `E:/tmp/algohub-load-acceptance-jbFHaP/`. Load run baru wajib mengukur ulang dan mencari bottleneck, bukan melonggarkan threshold tanpa owner decision.

## 2. Workload design

| Parameter | Nilai usulan |
| --- | --- |
| Endpoints | login/SSO exchange; /v1/auth/me; curriculum list; lesson detail + completion (code browser-trusted); practicum list/detail/autosave/submit; siakad schedule/attendance/rental/inventory list (dengan pagination 100 item) |
| Dataset | retained masked Phase 6 (426 users, 426 assignments, 481 grants) + `STAGING-BATCH20-cdb70744` |
| Cohort | 30 VU (12 akun matriks + 18 beban) — sesuai baseline lama agar comparable |
| Concurrency | ramp 1 → 5 → 15 → 30 VU, 2 menit per step |
| Durasi | ±12 menit per endpoint group; total < 30 menit |
| Request budget | maksimal 20.000 request total, hard cap |
| Warm/cold | cold: first-request per VU dicatat terpisah (incl. cold start Vercel); warm: exclude first request |
| Metrik | p50/p95/p99 per endpoint, error rate, 429 rate, DB connection count, function invocation count |
| Tooling | Playwright atau script Node fetch loop (tanpa install tool baru); request-ID dicatat |

## 3. Stop conditions (sebelum run)

1. Error rate > 5% pada step mana pun → stop, catat.
2. p95 > 3 s sustained 60 s → stop.
3. HTTP 5xx dari Vercel platform (bukan aplikasi) → stop.
4. Supabase pooler connection exhausted / timeout → stop.
5. Kuota Supabase Free mendekati limit (DB size/bandwidth) → stop.
6. Tidak ada DoS: tanpa request burst di luar pola di atas, tanpa menghabiskan pool/free quota, tanpa mematikan scheduler.

## 4. Uji 429 (pola sempit terotorisasi)

| Kasus | Pola | Expected |
| --- | --- | --- |
| Auth login brute limit | 6 percobaan salah password menit sama dari 1 akun | 429 + Retry-After; akun tidak terkunci permanen |
| Hint limit | 11th hint dalam 1 menit (pola terbukti) | 429 Retry-After 60; 0 XP ganda |
| Playground run | 6 run/menit (limit 5) | 429; tanpa mutasi |
| Retry behavior | Retry setelah Retry-After | Berhasil; tanpa 429 beruntun; backoff eksponensial di client |
| Mutasi safety | Submit/autosave/rental saat 429 | Tidak ada mutasi ganda/XP ganda/stok minus |

## 5. Lokal vs staging

Benchmark lokal (localhost PostgreSQL, dev server) dicatat terpisah dan tidak dihitung production measurement. Pooler staging (transaction mode, 4 koneksi API) adalah konfigurasi yang diuji; hasil tetap bounded, bukan kapasitas production.

## 6. Jika target gagal

Ukur bottleneck (endpoint paling lambat, DB query, cold start, pooler), rekomendasi perbaikan (indexing, koneksi, caching header), dan presentasikan ke pemilik. Threshold tidak diubah agar dashboard hijau tanpa owner decision.

## 7. Status

**Blocked menunggu approval pemilik** untuk: load run staging (G4), run 429 tambahan di luar pola hint yang sudah terbukti. Stop conditions di atas wajib disetujui bersama sebelum eksekusi.
