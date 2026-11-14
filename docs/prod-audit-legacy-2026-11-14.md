# Hasil Audit Legacy Database — 2026-11-14

**Mode:** read-only, zero write. Script: `scripts/prod-audit/legacy-schema-audit.sql`
+ follow-up query terverifikasi kolom. Output mentah:
`scripts/prod-audit/results/praktikum-legacy-2026-11-14.txt`.
**Proyek yang diaudit:** satu database legacy gabungan (Praktikum/E-learning/SIAKAD
bridge) di Supabase `oidwvfcxlxfyojkyurmm` — bukan tiga database terpisah.

## Angka kunci

| Item | Nilai |
|---|---|
| Total tabel (semua schema) | 161, termasuk schema Supabase (auth/realtime/storage/cron/vault/net) |
| Tabel aplikasi `public` | ~120 |
| `public.users` | **396 users** — 20 asisten, 2 koordinator, 374 mahasiswa |
| NIM null / duplikat NIM / duplikat email | **0 / 0 / 0** (temuan lama 22 duplikat sudah bersih) |
| `student_progress` | 15.948 row (+ backup `*_prestige` 15.138 — jangan di-union) |
| `jawaban_mahasiswa` | 3.744 row |
| `pengerjaan_course` | 1.633 row |
| `attendance_logs` | 597 row |
| Storage buckets | 2: `laporan-besar` (private), `public-assets` (public) |

## Timestamp terakhir per tabel (titik delta import)

| Tabel | Timestamp terakhir |
|---|---|
| users | 2026-11-09 17:17 UTC |
| student_progress | 2026-11-09 15:44 UTC |
| attendance_logs | 2026-10-31 (check_in_time) |
| jawaban_mahasiswa | 2026-11-07 12:23 UTC |
| pengerjaan_course | 2026-11-07 12:23 UTC |
| aktivasi_course | 2026-11-01 |

**Interpretasi:** aktivitas tulis terakhir sekitar 9 November 2026 → data
relatif tenang; delta import hari-H berisiko kecil.

## Temuan penting untuk pipeline import

1. **Nama kolom tidak seragam** — `users.role` memakai
   `asisten/koordinator/mahasiswa` (bukan `admin/kordas/praktikan` seperti
   tercatat di plan lama); `student_progress` memakai `nim` (bukan user_id);
   `pengerjaan_course` tidak punya `updated_at` (pakai `waktu_selesai`);
   `attendance_logs` pakai `check_in_time` (bukan `created_at`).
2. **Banyak tabel bayangan/arsip:** `*_backup_prestige`, `_bak_*`, `_arsip_*`
   (arsip 8 Sep 2026), duplikat EN/ID (`game_history` vs `game_riwayat`,
   `modules` vs `modul`, `levels` vs `level`, `inventory_items` vs
   `inventaris`). Phase 6 sudah memperingatkan: jangan union buta. Source of
   truth ditandai oleh tabel yang aktif ditulis (timestamp terisi).
3. `users` punya 47 kolom dengan alias ganda (`divisi` + `division`,
   `role` + `elearning_role`) — mapping ke model identity v2 harus
   ekspisit per kolom.
4. **Tidak ada backup manifest resmi** — semua tabel RLS-enabled kecuali
   tabel `_bak_`/arsip. Snapshot backup tetap langkah pertama sebelum
   dry-run import.

## Dampak ke runbook cutover

- Estimasi volume import naik sedikit vs Phase 6 (15.948 progress vs
  19.026 receipt staging) — masih dalam durasi terbukti (~6,5 menit).
- Dry-run harus memakai mapping kolom eksplisit dari temuan no. 1–3,
  bukan asumsi plan lama.
- Keputusan owner yang tersisa tidak berubah (runbook §6).

## Kredensial

- Tidak ada password/secret yang ditulis di dokumen ini, hasil output,
  atau commit. Akses via env file legacy yang sudah ada di luar repo v2.
- Sesi audit selesai; tidak ada koneksi yang ditinggalkan terbuka.
