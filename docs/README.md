# Dokumentasi Algohub

Folder ini hanya berisi dokumen operasional yang masih berlaku. Catatan historis
disimpan di [updates/](updates/) atau di git history.

## Mulai dari sini

| Dokumen | Isi |
| --- | --- |
| [updates/](updates/) | Update log kronologis: perubahan, verifikasi, dan tindak lanjut |
| [updates/2026-11-15-migration.md](updates/2026-11-15-migration.md) | Status migrasi legacy → v2, angka produksi, inventaris data, audit legacy |
| [cutover-runbook.md](cutover-runbook.md) | Runbook cutover produksi: urutan hari-H, rollback, DNS |
| [operations.md](operations.md) | Runbook operator staging, support playbook, dan UAT sign-off ledger |

## Setup dan operasi staging

| Dokumen | Isi |
| --- | --- |
| [staging/checklist.md](staging/checklist.md) | Checklist staging (Bahasa Indonesia), mulai dari sini |
| [staging/setup.md](staging/setup.md) | Setup teknis per project Vercel |
| [staging/accounts.md](staging/accounts.md) | Akun sintetis staging |
| [staging/dns.md](staging/dns.md) | Record DNS staging |
| [staging/scheduler.md](staging/scheduler.md) | Scheduler rekonsiliasi Praktikum |
| [dns/import-prod.txt](dns/import-prod.txt) / [dns/edit-manual-prod.txt](dns/edit-manual-prod.txt) | Record DNS produksi untuk Cloudflare |

## Subfolder

| Folder | Isi |
| --- | --- |
| [staging/](staging/) | Checklist, setup, akun, DNS, dan scheduler staging |
| [dns/](dns/) | File record DNS produksi |
| [updates/](updates/) | Update log dan template entri baru |
| [openapi/](openapi/) | Sumber OpenAPI 3.1 dan bundel hasil generate |
| [evidence/](evidence/) | Screenshot bukti untuk laporan yang masih aktif |

## Catatan

- Dokumen fase lama (`phase-*.md`, closeout, decision record) tidak lagi
  disimpan di working tree. Ambil versi lama dengan
  `git show <commit>:docs/<nama-file>`, misalnya
  `git show f1c0958:docs/phase-5-siakad.md`.
- `docs/ux-visual/` **tidak** disimpan di git; itu output pengukuran lokal yang
  dibuat ulang oleh `scripts/ux-measure*.mjs`.
- Jangan menaruh kredensial di folder ini. Nilai rahasia hanya di environment
  Vercel atau file `.env.*.local` yang di-ignore.
- Untuk update baru, salin [template](updates/_template.md) ke
  `updates/YYYY-MM-DD-<topik>.md` dan tautkan dari [updates/README.md](updates/README.md).
