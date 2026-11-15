# Dokumentasi Algohub

Folder ini hanya berisi dokumen yang masih berlaku. Histori lama tetap bisa
diakses lewat git history, bukan lewat subfolder arsip.

## Mulai dari sini

| Dokumen | Isi |
| --- | --- |
| [updates/](updates/) | Update log kronologis: perubahan, verifikasi, dan tindak lanjut |
| [migration.md](migration.md) | Status migrasi legacy → v2, angka produksi, inventaris data, audit legacy |
| [cutover-runbook.md](cutover-runbook.md) | Runbook cutover produksi: urutan hari-H, rollback, DNS |
| [operations.md](operations.md) | Runbook operator staging, support playbook, dan UAT sign-off ledger |

## Setup dan operasi staging

| Dokumen | Isi |
| --- | --- |
| [staging-checklist.md](staging-checklist.md) | Checklist staging (Bahasa Indonesia), mulai dari sini |
| [staging-setup.md](staging-setup.md) | Setup teknis per project Vercel |
| [staging-accounts.md](staging-accounts.md) | Akun sintetis staging |
| [staging-dns.md](staging-dns.md) | Record DNS staging |
| [staging-scheduler.md](staging-scheduler.md) | Scheduler rekonsiliasi Praktikum |
| [host-migration-checklist.md](host-migration-checklist.md) | Item migrasi host/domain |
| [dns-import-prod.txt](dns-import-prod.txt) / [dns-edit-manual-prod.txt](dns-edit-manual-prod.txt) | Record DNS produksi untuk Cloudflare |

## Kontrak dan laporan fase

Setiap dokumen fase menggabungkan kontrak implementasi dan laporan hasilnya.

| Dokumen | Isi |
| --- | --- |
| [phase-1-5-closeout.md](phase-1-5-closeout.md) | Penutupan Phase 1–5 |
| [phase-2-identity.md](phase-2-identity.md) | Identity, SSO, authorization, dan kontrak database |
| [phase-3-practicum.md](phase-3-practicum.md) | Praktikum: kontrak, implementasi, verifikasi |
| [phase-4-elearning.md](phase-4-elearning.md) | E-learning: kontrak, implementasi, verifikasi |
| [phase-5-siakad.md](phase-5-siakad.md) | SIAKAD dan inventory: kontrak, implementasi, verifikasi |
| [elearning-browser-completion-decision-2026-11-12.md](elearning-browser-completion-decision-2026-11-12.md) | Keputusan runner browser E-learning |
| [updates/2026-11-15-praktikum-repair.md](updates/2026-11-15-praktikum-repair.md) | Perbaikan Praktikum: UI/UX, integritas data, hasil rollout |

## Subfolder

| Folder | Isi |
| --- | --- |
| [updates/](updates/) | Update log dan template entri baru |
| [openapi/](openapi/) | Sumber OpenAPI 3.1 dan bundel hasil generate |
| [evidence/](evidence/) | Screenshot bukti untuk laporan yang masih aktif |

## Catatan

- `docs/ux-visual/` **tidak** disimpan di git; itu output pengukuran lokal yang
  dibuat ulang oleh `scripts/ux-measure*.mjs`.
- Jangan menaruh kredensial di folder ini. Nilai rahasia hanya di environment
  Vercel atau file `.env.*.local` yang di-ignore.
- Untuk update baru, salin [template](updates/_template.md) ke
  `updates/YYYY-MM-DD-<topik>.md` dan tautkan dari [updates/README.md](updates/README.md).
