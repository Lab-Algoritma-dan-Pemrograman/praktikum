# Phase 7 — Browser/Device dan Accessibility Report

Status: **automated isolated pass**, 14 November 2026. Tool: Playwright + `@axe-core/playwright`; scope rules WCAG 2.2 A/AA aktif kecuali `color-contrast`.

## Ringkasan eksekutif

- **12/12 automated test lulus**: 6 target halaman × viewport 320px/1280px pada Chromium.
- Test sekarang memverifikasi response sukses, origin/path URL akhir, parameter `return_to`, document title, heading utama, dan nol pelanggaran axe. Pelanggaran tidak lagi sekadar dicetak lalu dianggap pass.
- Stack Algohub dijalankan terisolasi pada port alternatif melalui `ALGOHUB_E2E_APP_PORT_BASE` dan `ALGOHUB_E2E_API_PORT`; test tidak bergantung pada server asing yang kebetulan memakai port default.
- Nol pelanggaran terdeteksi untuk ruleset yang aktif. `color-contrast` tetap dikecualikan karena rendering font headless; audit kontras manual masih wajib.
- Kontrol manual belum selesai: keyboard-only penuh, screen reader nyata, kontras, Firefox/WebKit, dan perangkat fisik. Automated pass bukan sign-off WCAG 2.2 AA penuh.

## Koreksi bukti 13 November

Run awal 13 November tidak sah sebagai bukti Algohub untuk `portal-root` dan login:

1. Port 5173 dan 5174 ternyata dipakai dua dev server lokal project lain, bukan Portal/Auth Algohub.
2. Redirect login empat app berakhir pada server asing di port 5174.
3. Assertion lama hanya `expect(summary).toBeDefined()`, sehingga daftar pelanggaran apa pun tetap membuat test lulus.
4. Selector `.size-7.text-amber-50` dan `.md:px-24` tidak ada di source Algohub.

Karena itu temuan lama `document-title`, `link-name`, dan `scrollable-region-focusable` **ditarik sebagai false target**, bukan diperbaiki di UI Algohub. Tidak ada klaim defect closure palsu.

## Cakupan dan batas bukti

| Aspek | Status | Batas |
| --- | --- | --- |
| Browser | Chromium automated | Firefox/WebKit belum diuji |
| Viewport | 320×640 dan 1280×720 | Emulasi bukan perangkat fisik |
| Target | Portal root, Auth login, redirect login Admin/SIAKAD/E-learning/Praktikum | Halaman authenticated belum tercakup oleh suite axe ini |
| Ruleset | `wcag2a`, `wcag2aa`, `wcag22a`, `wcag22aa` | `color-contrast` disabled; perlu audit manual |
| Navigasi | Status response, URL akhir, `return_to`, title, h1 | Bukan pengganti journey UAT manusia |
| Screen reader | Belum | Butuh pengujian manual |

## Hasil automated

| Target | 320px | 1280px | Expected identity |
| --- | --- | --- | --- |
| Portal root | Pass | Pass | `Algohub — Portal`; h1 `Portal Algohub` |
| Auth login | Pass | Pass | `Login — Algohub`; h1 login |
| Admin login redirect | Pass | Pass | Auth login + `return_to` Admin |
| SIAKAD login redirect | Pass | Pass | Auth login + `return_to` SIAKAD |
| E-learning login redirect | Pass | Pass | Auth login + `return_to` E-learning |
| Praktikum login redirect | Pass | Pass | Auth login + `return_to` Praktikum |

Total: **12 passed, 0 failed, 0 active-rule violations**. Durasi run terisolasi: 2,2 menit. Run gabungan identity/a11y kemudian mengulang coverage ini dalam 38/38 test lulus; JSON evidence: `docs/evidence/phase-7-e2e-identity-a11y-2026-11-14.json`.

## Fondasi statis

- Semua `app.html` memakai `<html lang="id">`.
- Tidak ditemukan `<img>` tanpa kebutuhan `alt`.
- Semua form yang discan mempunyai label valid.
- Semua button yang discan mempunyai teks atau `aria-label`.
- Catatan struktur heading, status dinamis, dan error association tetap ada di `docs/phase-7-a11y-static.md`; itu kandidat perbaikan/manual review, bukan blocker otomatis yang sudah dibuktikan axe.

## Reproduksi

```bash
ALGOHUB_E2E_APP_PORT_BASE=5273 \
ALGOHUB_E2E_API_PORT=8180 \
npx playwright test a11y --reporter=line
```

Harness terkait:

- `tests/frontend/local.ts` — port E2E dapat dioverride dan tetap masuk ke scrubbed child environment.
- `playwright.config.ts` — health check API mengikuti origin E2E terkonfigurasi.
- `tests/frontend/e2e/a11y.spec.ts` — identity assertion dan hard failure untuk violations.

Validasi tambahan setelah perubahan harness:

- `npm run check`: 6 workspace, **0 error/0 warning**.
- `npm run test:frontend`: **86 passed, 0 failed**.

## Sisa pekerjaan

- [ ] Audit kontras manual.
- [ ] Keyboard-only end-to-end pada alur authenticated.
- [ ] Screen-reader manual.
- [ ] Firefox dan WebKit bila browser tersedia.
- [ ] Perangkat mobile fisik; viewport emulation tidak boleh dilabel perangkat nyata.
