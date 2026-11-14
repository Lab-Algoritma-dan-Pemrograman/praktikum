# Phase 7 — Technical Closeout

Status: **REOPENED — portal landing page/public surface belum diimplementasikan**, 14 November 2026. Hasil test yang ada tetap valid hanya untuk scope yang benar-benar diuji; Phase 7 local technical-readiness tidak lagi dinyatakan complete sampai gap portal ditutup.

## Ringkasan

- Source/test harness stabil pada scope perubahan Phase 7; tidak ada backend behavior, deploy, data staging, scheduler, DNS, atau credential yang diubah.
- Validasi lokal final: 6 workspace check tanpa error/warning, 86 unit pass, dan 72 E2E named tests = 67 pass + 5 expected skip + 0 final failure/flaky.
- A11y Chromium isolated: 12/12 hard-gated pass dan nol pelanggaran ruleset aktif. Kontras/manual screen reader/browser/perangkat fisik tetap gap.
- Dependency audit terbaru: 0 high/critical, 3 moderate, 4 low.
- Phase 7 tidak dapat dinyatakan 100% sebelum UAT manusia dan pilot nyata ditutup sesuai scope yang disepakati.

## Ringkasan gate

| Gate | Status | Bukti utama | Batas/blocker |
| --- | --- | --- | --- |
| G0 Portal landing/public surface | **Reopened** | Plan mewajibkan landing tunggal di `/`; source saat ini hanya identity `AppShell` | Implementasi landing dan audit route publik wajib sebelum UAT |
| G1 UAT fungsional + sign-off | **Blocked** | `docs/uat-signoff-ledger.md` siap | Butuh tester dan keputusan manusia nyata |
| G2 Browser/device matrix | **Partial** | Chromium 320/1280 isolated 12/12 | Firefox/WebKit dan perangkat fisik belum diuji |
| G3 WCAG 2.2 AA | **Partial** | 12/12 axe, 0 active-rule violations + audit statis | Kontras, screen reader, dan keyboard authenticated penuh masih manual |
| G4 Performance p95 <500 ms | **Blocked** | Workload/stop condition siap; baseline lama p95 843,69 ms gagal target | Staging load run butuh approval spesifik |
| G5 429/quota | **Partial** | Hint/playground baseline + pola uji terdefinisi | Run tambahan dan quota monitoring butuh approval |
| G6 Security/privacy | **Passed (targeted review)** | 6 area solid; 0 code finding high/medium, 1 low; npm 0 high/critical | Bukan pentest profesional; 3 moderate/4 low dependency advisory tetap ada |
| G7 Observability/alerting | **Partial** | Request-ID/log/recovery/timeout ada | Penerima dan delivery proof alert belum ada |
| G8 Cron/incident drill | **Partial** | Scheduler/recovery baseline tersedia | Bounded staging drill butuh approval |
| G9 Runbook/playbook | **Passed (template)** | `docs/runbook.md`, `docs/support-playbook.md` | Kontak/petugas nyata harus diisi pemilik |
| G10 Pilot | **Blocked** | `docs/phase-7-pilot-plan.md` siap | Peserta, periode, undangan, support owner, feedback = keputusan manusia |

## Validasi technical-readiness lokal

| Check | Hasil |
| --- | --- |
| `npm run check` | 6 workspace; 0 error/0 warning |
| `npm run test:frontend` | 86 pass, 0 fail |
| Identity/a11y E2E | 38 pass, 0 skip/fail |
| E-learning/browser E2E | 9 pass, 5 expected skip, 0 fail |
| Praktikum/SIAKAD final E2E | 20 pass, 0 skip/fail |
| Total E2E | 72 named = 67 pass + 5 expected skip + 0 final fail/flaky |
| NPM audit high threshold | Exit 0; 0 high/critical, 3 moderate, 4 low |
| Backend | Baseline Phase 6 dipakai: 170 top-level/487 PASS; backend source tidak berubah |

Lima expected skip adalah test download/runtime Python publik yang memang opt-in. Ia tidak dihitung pass; evidence real-runtime Phase 1–5 tetap menjadi baseline.

## Koreksi integritas bukti

1. **A11y false target:** run awal memakai port 5173/5174 yang ternyata milik project lokal lain, dan assertion lama tidak menggagalkan violation. Temuan `document-title`, `link-name`, dan `scrollable-region-focusable` ditarik. Harness diisolasi, memverifikasi identity halaman, dan hard-fail; run pengganti lulus 12/12.
2. **Race test SIAKAD:** group awal menghasilkan 19 pass/1 fail karena test menekan refresh lalu memilih stale record pertama sebelum record baru masuk DOM. Failure artifact dipertahankan. Test sekarang menunggu record `Mahasiswa Fixture` yang `Belum divalidasi`; targeted rerun 1/1 dan full rerun 20/20 lulus.
3. **Sanitasi evidence:** environment sistem dan user-home path dihapus dari JSON yang dipertahankan. Tidak ada credential atau `.env` masuk evidence.

## Keputusan jadwal pemilik

Keputusan terbaru 14 November 2026: seluruh human UAT, pilot, dan testing manual ditunda sampai **Phase 7A UI/UX Audit & Consolidation** ditutup. Rencana UAT bersama asisten pada pagi 15 November 2026 tidak lagi aktif. Tidak ada tester, hasil, issue, atau sign-off yang boleh diisi sebelum kegiatan baru dijadwalkan secara eksplisit. Keputusan ini juga bukan izin untuk automated load/stress test, alert delivery, incident injection, deploy, DNS mutation, atau production mutation.

## Keputusan minimum yang dibutuhkan pemilik

1. Tunjuk tester manusia untuk UAT dan isi sign-off ledger.
2. Approve/tolak workload staging + request budget + stop conditions di `docs/phase-7-performance.md`.
3. Putuskan apakah catatan low rate-limit IP memerlukan trust-proxy terbatas.
4. Tunjuk penerima dan saluran alert sebelum delivery test.
5. Approve/tolak bounded staging incident drill.
6. Tentukan pilot: cohort, periode, support owner, kanal feedback, dan izin kontak.
7. Tentukan apakah Firefox/WebKit/perangkat fisik/screen reader menjadi gate wajib sebelum pilot.

## Residual risk

- Baseline p95 843,69 ms melebihi target 500 ms; bottleneck staging belum diukur ulang.
- 4 low/3 moderate npm advisory tetap tercatat; audit membuktikan 0 high/critical, bukan 0 advisory.
- Browser-trusted XP forgery risk tetap accepted product decision, bukan nilai akademik authoritative.
- Rejected histories, absent attachment model, file transfer, dan provider Auth recovery tetap exclusions sebelum cutover.
- Emulasi viewport bukan bukti perangkat nyata; axe bukan manual screen-reader sign-off.
- Review security terarah bukan penetration test independen.

## Evidence dan dokumen

| Artefak | Lokasi |
| --- | --- |
| Ringkasan teknis + hashes | `docs/evidence/phase-7-technical-final-2026-11-14.json` |
| Identity/a11y E2E | `docs/evidence/phase-7-e2e-identity-a11y-2026-11-14.json` |
| E-learning/browser E2E | `docs/evidence/phase-7-e2e-elearning-browser-2026-11-14.json` |
| Initial failed Praktikum/SIAKAD | `docs/evidence/phase-7-e2e-practicum-siakad-2026-11-14.json` |
| Targeted SIAKAD rerun | `docs/evidence/phase-7-e2e-siakad-1280-rerun1-2026-11-14.json` |
| Final Praktikum/SIAKAD rerun | `docs/evidence/phase-7-e2e-practicum-siakad-rerun1-2026-11-14.json` |
| NPM audit | `docs/evidence/phase-7-npm-audit-2026-11-14.json` |
| Source delta checksums | `docs/evidence/phase-7-source-checksums-final-2026-11-14.txt` |
| Browser/a11y report | `docs/phase-7-browser-a11y.md` |
| Static a11y report | `docs/phase-7-a11y-static.md` |
| Security report | `docs/phase-7-security.md` |
| Performance/429 plan | `docs/phase-7-performance.md` |
| Observability/incident plan | `docs/phase-7-observability.md` |
| UAT ledger | `docs/uat-signoff-ledger.md` |
| Pilot plan | `docs/phase-7-pilot-plan.md` |
| Runbook/support | `docs/runbook.md`, `docs/support-playbook.md` |
| Cleanup | `docs/phase-7-artifact-cleanup.md` |
| Handover | `memory/HANDOVER-2026-11-14-PHASE-7-TECHNICAL.md` |

## Phase 7A automated regression correction

The final full local E2E run after Phase 7A changes completed with **146 named tests: 139 passed, 5 expected skips, and 2 failures**. Both failures are existing SIAKAD test locator ambiguities where `getByRole('status')` resolves both the transient loading status and the mutation notice; they are not Portal/public-route failures, but they keep the full automated gate open. Phase 7A closeout therefore remains blocked pending test-harness correction and a clean rerun. No human UAT or remote operation is authorized by this result.


Technical-readiness lokal pada scope di atas selesai. **Phase 7 keseluruhan belum selesai 100%** sampai human UAT dan pilot sign-off nyata ada, serta owner memutuskan gate operasional yang masih blocked. Phase 8 belum dijalankan.
