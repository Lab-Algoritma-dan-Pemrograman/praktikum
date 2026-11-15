# Phase 7 — Artifact dan Process Cleanup

Tanggal: 14 November 2026. Scope: pekerjaan technical-readiness lokal Phase 7; tidak ada deploy, staging mutation, akun baru, perubahan scheduler, atau fault injection remote.

## Status akhir

- Tidak ada listener tersisa pada port dev/test 5173–5178, 5273–5278, 8080, atau 8180.
- Playwright mematikan API dan enam Vite server terisolasi setelah setiap run.
- Tidak ada database sementara, container, network, atau watcher baru yang dibuat.
- Tidak ada credential, `.env`, token, snapshot data, atau Auth secret yang dimasukkan ke evidence.
- Environment sistem dan user-home path telah dihapus dari JSON Playwright yang dipertahankan.

## Process yang dihentikan

| Resource | Alasan | Ownership/otorisasi | Hasil |
| --- | --- | --- | --- |
| Local project lain pada 5173/5174/8080 | Menggunakan port default dan membebani laptop | Dihentikan setelah permintaan eksplisit pemilik; file project tidak disentuh | Stopped |
| Algohub Vite lama pada 5175–5178 | Berbenturan dengan `.svelte-kit` saat run terisolasi | Command line membuktikan milik `project_ap_v2` | Stopped |
| Algohub test stack 5273–5278/8180 | Per-run Playwright ownership | Auto-cleanup oleh webServer lifecycle | Stopped |

## Artefak yang dipertahankan

| Artefak | Tujuan |
| --- | --- |
| `docs/evidence/phase-7-technical-final-2026-11-14.json` | Ringkasan final technical readiness + binding hash |
| `docs/evidence/phase-7-e2e-identity-a11y-2026-11-14.json` | 38 test identity/a11y |
| `docs/evidence/phase-7-e2e-elearning-browser-2026-11-14.json` | 9 pass + 5 expected skip E-learning/runtime |
| `docs/evidence/phase-7-e2e-practicum-siakad-2026-11-14.json` | Failed run awal 19 pass/1 race; dipertahankan, tidak direlabel |
| `docs/evidence/phase-7-e2e-siakad-1280-rerun1-2026-11-14.json` | Targeted rerun 1/1 pass |
| `docs/evidence/phase-7-e2e-practicum-siakad-rerun1-2026-11-14.json` | Full rerun 20/20 pass |
| `docs/evidence/phase-7-npm-audit-2026-11-14.json` | Dependency audit 0 high/critical |
| `docs/evidence/phase-7-source-checksums-final-2026-11-14.txt` | Hash lima source/test harness yang berubah |
| `E:/tmp/algohub-phase7-a11y-harness-20261114-000916/` | Salinan pre-change + `SHA256SUMS`; tanpa `.env`/credential |

## Artefak yang tidak dianggap bukti

- Run a11y awal terhadap server existing pada port 5173/5174: target ternyata bukan Algohub dan assertion lama tidak hard-fail.
- Failure infrastruktur `UNKNOWN: write` setelah proses Playwright pertama dipindah ke background: tidak ada test result aplikasi yang dapat diklaim.
- Emulasi viewport tidak dianggap perangkat mobile fisik.
- Lima expected skip runtime Python tidak dihitung pass; evidence Phase 1–5 tetap menjadi baseline untuk run opt-in tersebut.

## Remote state

Staging retained data, acceptance sessions, scheduler cadence, Vault secret, deployment, DNS, dan akun tidak disentuh dalam continuation ini. Baseline dan exclusions Phase 6 tetap berlaku.
