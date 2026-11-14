# Prod Audit Kit — Legacy Database Read-Only Audit

**Status:** kit siap, belum dieksekusi (menunggu akses read-only ke DB legacy prod dari owner).

## Apa ini

Script SQL read-only untuk mengaudit database legacy (SIAKAD / E-learning /
Praktikum) sebelum cutover produksi. Menjawab:

1. Inventaris tabel: nama, estimasi row, jumlah kolom, status RLS.
2. Jumlah user per tabel identitas.
3. Duplikat NIM/email (temuan lama: 268 user SIAKAD dengan 22 NIM duplikat —
   wajib diverifikasi ulang dari data terbaru).
4. Row count tabel domain utama (jadwal, absensi, inventaris, rental,
   levels/modules/lessons/progress/game/achievement).
5. Max `updated_at` / `created_at` per tabel (untuk perencanaan delta-import).
6. Daftar storage bucket.

## Cara pakai

Tidak ada kredensial yang di-hardcode. Pakai variabel lingkungan:

```bash
export LEGACY_DATABASE_URL="postgresql://readonly_user:***@host:5432/legacy_db"
psql "$LEGACY_DATABASE_URL" -f scripts/prod-audit/legacy-schema-audit.sql \
  -o scripts/prod-audit/results/siakad-$(date +%F).txt
```

Ulangi per database legacy yang ada (SIAKAD, E-learning, Praktikum).
Buat folder `results/` secara manual bila belum ada.

## Jaminan keamanan

- 100% `SELECT` terhadap `information_schema`, `pg_catalog`, dan tabel aplikasi.
- Tanpa `INSERT/UPDATE/DELETE/DDL`, tanpa extension, tanpa function baru.
- Semua section di-guard `to_regclass(...) IS NOT NULL` — kalau tabel tidak ada
  di database tersebut, section itu kosong, script tidak gagal total.

## Cara membaca hasil

- Bandingkan Section 1 dengan daftar tabel target v2 di
  `backend/migrations/0002`–`0007` (identity + `practicum_*` 8 tabel +
  `learning_*` 9 tabel + `academic_*` 8 tabel).
- Bandingkan Section 3 dengan temuan lama di `plan/database-migration.md`.
- Bandingkan Section 4 dan 5 dengan `docs/phase-6-inventory.md` (masking
  rehearsal Phase 6) untuk melihat data baru sejak rehearsal.
- Section 5 menentukan titik mulai delta-import di hari cutover.

## Setelah audit

Lampirkan hasil ke `docs/prod-cutover-runbook.md` (bagian "Strategi data")
sebagai baseline nyata sebelum owner memutuskan tanggal cutover.
