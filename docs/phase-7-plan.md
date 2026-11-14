# Phase 7 — Rencana UAT dan Pilot

Status: **plan**, 13 November 2026. Dokumen ini memetakan gate, baseline yang dapat dipakai, gap yang perlu diuji, lokasi bukti, batas operasi, dan pemilik keputusan. Ia tidak menggantikan persetujuan manusia dengan asumsi.

## 1. Baseline yang dapat dipakai (tanpa rerun)

| Aset | Detail | Lokasi bukti |
| --- | --- | --- |
| Staging 7 hostname | `portal\|auth\|admin\|praktikum\|elearning\|siakad\|api.staging.algohub.web.id`, Vercel `algohub-v2-staging-*`, region sin1 | `docs/staging-setup.md` |
| Akun retained | 12 akun matriks role + 18 akun beban (`beban.01–18.staging@algohub.test`), password di `.env.staging.accounts.local` | `docs/staging-accounts.md` |
| Dataset retained | Masked Phase 6 (426 users, 481 grants); `STAGING-BATCH20-cdb70744` (120 attempt, terminal `verified-and-inactive-retained`) | `docs/phase-6-closeout.md`, `docs/staging-scheduler.md` |
| Scheduler aktif | pg_cron/pg_net `algohub-staging-practicum-reconcile`, per menit, timeout 28s; runbook pause/resume/rotasi siap | `docs/staging-scheduler.md` |
| Playwright | Root `playwright.config.ts`, 1 worker, Chrome only; 9 spec e2e; 85+ unit frontend | `tests/frontend/e2e/` |
| Backend regression | Go 1.26.11: 170 top-level tests/487 PASS, race Linux lulus, vet/build/govulncheck nol | `docs/phase-6-closeout.md` §2 |
| Rate limit existing | 429 `RATE_LIMITED`/`SESSION_LIMIT` di `backend/internal/delivery/http/auth_handlers.go`; hint 10/menit → 429 Retry-After 60 terbukti | closeout Phase 1–5 |
| Middleware observability | `requestid.go`, `logging.go`, `security.go`, `timeout.go`, `recovery.go` di `backend/internal/middleware/` | source |
| Recovery drill preseden | 7 gates restore backup → owned local `_test` DB | `docs/evidence/phase-6-final-2026-11-13.json` |

Keputusan frozen yang mengikat Phase 7: browser-trusted XP (risiko forgery diterima), Praktikum grading manual, Judge0/RapidAPI abandoned, Glot preview-only non-authoritative, tanpa cutover/DNS/production write.

## 2. Matriks gate Phase 7

| # | Gate | Baseline dipakai | Gap yang diuji | Bukti target | Pemilik |
| --- | --- | --- | --- | --- | --- |
| G1 | UAT fungsional per role | 91 SSR checks, SSO 6 host, scoped asisten | Skenario end-to-end mahasiswa/asisten×6 divisi/koordinator/peminjam pada data masked; deep link; optimistic conflict; mobile editor | UAT sign-off ledger (human) | Pemilik + tester manusia |
| G2 | Browser/perangkat matrix | Chromium only saat ini | Firefox/WebKit bila tersedia; viewport 320–1440+; keyboard-only; reduced-motion; touch | `docs/phase-7-browser-a11y.md` | AI (eksekusi lokal) + pemilik (sign-off) |
| G3 | WCAG 2.2 AA | Tidak ada axe-core; menu 320px keyboard-only sudah terbukti sekali | Tambah @axe-core/playwright; audit label/focus/contrast/reflow 320px; manual screen-reader bila alat ada | Automated axe pass + catatan gap manual | AI + pemilik |
| G4 | Performance p95<500ms, error<1% | Baseline baca 300 req p95 **843,69 ms (gagal target)**; metrik runtime preview lulus | Load workload terdefinisi pada staging; warm/cold; p50/p95/p99; identifikasi bottleneck | `docs/phase-7-performance.md`; rekomendasi bila gagal, tanpa ubah threshold | Pemilik (approve load run) |
| G5 | 429/quota | Hint 429 + playground 5/menit 30/jam terbukti | Pola sempit terotorisasi lintas endpoint kritis; Retry-After; tidak ada mutasi ganda; monitoring kuota Supabase | `docs/phase-7-performance.md` | AI (bounded) |
| G6 | Security/privacy | CSRF 403, IDOR scoped 404, cookie flags, govulncheck nol; 3 low/4 moderate npm tersisa | Review terarah: auth/SSO cookie, BOLA, replay/idempotency, XSS render, private-cache; dependency scan terkini | `docs/phase-7-security.md` | AI (terarah, tanpa multi-agent scan) |
| G7 | Observability/alerting | Request-ID/logging ada; cron terverifikasi; **belum ada alerting nyata** | Verifikasi perjalanan error frontend→API→log; audit alert yang benar-benar tersedia vs plan saja | `docs/phase-7-observability.md` | Pemilik (setiap alert ke penerima nyata) |
| G8 | Cron/incident drill | Batch 120 job + lease recovery ~50s; restore drill 7 gates | Isolated/local fault injection dulu; bounded staging drill hanya setelah approval | `docs/phase-7-observability.md` | Pemilik |
| G9 | Runbook/support playbook | Runbook scheduler ada | Triage, severity, request-ID lookup, eskalasi, rollback limits, checklist | `docs/runbook.md`, `docs/support-playbook.md` | AI (template) + pemilik (kontak nyata) |
| G10 | Pilot | Akun sintetis retained = technical readiness saja | Usulan cohort, periode, scope, metrik, feedback, stop/go | `docs/phase-7-pilot-plan.md` | **Pemilik penuh** (peserta, undangan, data personal) |

## 3. Urutan eksekusi

1. **Lokal/read-only dulu** (tanpa approval): G9 runbook/playbook template, UAT scenario + sign-off ledger format, G2/G3 persiapan (tambah axe-core dev-dependency, audit statis a11y pada 26 halaman), G6 review terarah source.
2. **Bounded staging read-only**: G3 axe pass pada hostname staging; G5 429 pola sempit dengan akun retained; G7 verifikasi log/request-ID.
3. **Butuh approval spesifik**: G4 load run (target, budget, stop conditions dulu), G8 staging drill, G7 alert ke penerima nyata, G10 pilot (peserta/undangan).
4. **Human UAT**: G1 sign-off oleh tester manusia per role; AI menyediakan format dan skenario, tidak mengarang hasil.

Regresi: unit/contract/targeted dulu, full suite (`npm run check`, `test:frontend`, `api:test`, `api:vet`, `openapi:check`, Playwright) setelah source stabil. Pin checksum source per run.

## 4. Batas operasi

- Tidak ada: production/legacy write, DNS cutover, secret rotation, scheduler pause/cadence change, billing baru, akun baru, email/undangan, incident injection staging, load test — semuanya butuh izin baru.
- 429 diuji dengan pola sempit terotorisasi; tidak melemahkan rate limiter; tidak DoS layanan umum.
- Database integration hanya pada loopback `_test` baru; fixture tidak mengarah ke staging.
- Workspace bukan Git repo: archive/checksum sebelum perubahan berisiko, tanpa `.env`/credential.
- Credential hanya lewat file privat existing (`*.local`), tidak pernah di-print.
- NPM audit terbaru 14 November 2026: 4 low/3 moderate, 0 high/critical; ukur ulang lagi sebelum klaim resolved.
- Emulasi viewport ≠ perangkat nyata; axe pass ≠ manual screen-reader sign-off; localhost benchmark ≠ production measurement. Semua bukti ditandai dengan batasnya.

## 5. Definition of done

Dokumen: plan ini, UAT sign-off ledger, laporan browser/a11y, performance/429, security, observability/incident, pilot plan, runbook, support playbook, cleanup manifest, `docs/phase-7-closeout.md` (tabel passed/failed/blocked/not-run + residual risk + owner decisions), handover Phase 8.

"Phase 7 selesai 100%" memerlukan gate teknis **dan** UAT/pilot sign-off manusia nyata. Bagian yang menunggu peserta/approval ditutup sebagai blocked dengan blocker konkret, bukan dilabel lulus.
