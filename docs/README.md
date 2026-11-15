# Dokumentasi Algohub

Folder ini dipisah supaya yang masih berlaku tidak tercampur dengan catatan yang
sudah lewat. Kalau mencari sesuatu, mulai dari tabel di bawah.

## Yang berlaku sekarang

| Dokumen | Isi |
| --- | --- |
| [cutover-runbook.md](cutover-runbook.md) | Runbook cutover produksi: urutan hari-H, rollback, DNS |
| [migration-status-2026-11-15.md](migration-status-2026-11-15.md) | Status migrasi data legacy → v2, angka produksi, temuan |
| [praktikum-repair-2026-11-15.md](praktikum-repair-2026-11-15.md) | Perbaikan Praktikum: UI/UX, integritas data, hasil rollout |
| [runbook.md](runbook.md) | Runbook operator staging (triage, eskalasi) |
| [support-playbook.md](support-playbook.md) | Alur dukungan pengguna |
| [uat-signoff-ledger.md](uat-signoff-ledger.md) | Catatan keputusan UAT |

## Setup dan operasi

| Dokumen | Isi |
| --- | --- |
| [staging-checklist.md](staging-checklist.md) | Checklist staging (Bahasa Indonesia), mulai dari sini |
| [staging-setup.md](staging-setup.md) | Setup teknis per project Vercel |
| [staging-accounts.md](staging-accounts.md) | Akun sintetis staging |
| [staging-dns.md](staging-dns.md) | Record DNS staging |
| [staging-scheduler.md](staging-scheduler.md) | Scheduler pencapaian |
| [dns-import-prod.txt](dns-import-prod.txt) / [dns-edit-manual-prod.txt](dns-edit-manual-prod.txt) | Record DNS produksi untuk Cloudflare |
| [phase-7a-host-migration-checklist.md](phase-7a-host-migration-checklist.md) | Item migrasi host/owner |

## Kontrak per fase

| Dokumen | Isi |
| --- | --- |
| [phase-2-identity.md](phase-2-identity.md) | Identity, SSO, kontrak database |
| [phase-3-contract.md](phase-3-contract.md) / [phase-3-practicum.md](phase-3-practicum.md) | Kontrak dan implementasi Praktikum |
| [phase-4-contract.md](phase-4-contract.md) / [phase-4-elearning.md](phase-4-elearning.md) | Kontrak dan implementasi E-learning |
| [phase-5-contract.md](phase-5-contract.md) / [phase-5-siakad.md](phase-5-siakad.md) | Kontrak dan implementasi SIAKAD |
| [phase-1-5-closeout.md](phase-1-5-closeout.md) | Closeout Phase 1–5 |
| [phase-6-inventory.md](phase-6-inventory.md) | Inventaris dataset Phase 6 |
| [elearning-browser-completion-decision-2026-11-12.md](elearning-browser-completion-decision-2026-11-12.md) | Keputusan runner browser |
| [prod-audit-legacy-2026-11-14.md](prod-audit-legacy-2026-11-14.md) | Audit legacy sebelum cutover |

## Subfolder

| Folder | Isi |
| --- | --- |
| [openapi/](openapi/) | Sumber OpenAPI 3.1 dan bundel hasil generate |
| [archive/](archive/) | Laporan fase yang selesai, keputusan yang digantikan, dan [bukti mentah](archive/evidence/) dari run tertentu |

## Catatan

- `docs/archive/` disimpan untuk jejak historis, bukan panduan aktif. Laporan di
  sana boleh menyebut kondisi yang sudah tidak berlaku.
- `docs/ux-visual/` **tidak** disimpan di git; itu output pengukuran lokal yang
  dibuat ulang oleh `scripts/ux-measure*.mjs`.
- Jangan menaruh kredensial di folder ini. Nilai rahasia hanya di environment
  Vercel atau file `.env.*.local` yang di-ignore.
