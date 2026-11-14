# Runbook cutover produksi `algohub.web.id`

Sasaran: memindahkan situs publik dari `project_ap` (legacy) ke `project_ap_v2`
tanpa kehilangan satu baris data pun. Sumber legacy hanya dibaca.

## Kapan dianggap siap

Semua syarat ini terpenuhi lebih dulu; kalau satu saja belum, cutover ditunda:

1. Ref + `SUPABASE_SERVICE_ROLE_KEY` + connection string project Supabase produksi tersedia.
2. Akses registrar/DNS `algohub.web.id` (dan subdomain `auth`, `admin`, `siakad`,
   `elearning`, `praktikum`, `api`) tersedia.
3. Backup produksi terbaru + checksum tersimpan: DB legacy (`prod-backup-*/SHA256SUMS.txt`)
   dan storage legacy (`legacy-storage-backup-*/MANIFEST.sha256`).
4. Dry-run penuh pada DB latihan selesai dengan 0 mismatch.
5. Password DB legacy sudah dirotasi.
6. Satu akun legacy nyata sudah diuji login di DB latihan (email atau NIM).

## Persiapan (T-1 hari)

```bash
cd E:/rekapProject/project_ap_v2/backend
go build -o /e/tmp/algohub-migration/legacy-migrate.exe ./cmd/legacy-migrate

# snapshot legacy terbaru + cadangan storage
cd /e/tmp/algohub-migration
pg_dump "$LEGACY_DATABASE_URL" -Fc -f legacy-prod-$(date +%Y%m%d-%H%M).dump
python backup_legacy_storage.py   # menghasilkan MANIFEST.json + MANIFEST.sha256
sha256sum -c MANIFEST.sha256
```

Isi `.env.production.local` dengan `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`,
`ALGOHUB_DATABASE_URL`, `ALGOHUB_MIGRATION_DATABASE_URL`, `ALGOHUB_API_BASE_URL`,
`ALGOHUB_AUTH_ORIGIN`, `ALGOHUB_ALLOWED_ORIGINS`, `ALGOHUB_ENV=production`,
`ALGOHUB_ENABLE_FIXTURES=false`.

## Eksekusi (T-0)

Urutannya penting: DB dulu, file kedua, DNS terakhir.

```bash
cd E:/rekapProject/project_ap_v2

# 1. skema v2 di produksi
psql "$ALGOHUB_MIGRATION_DATABASE_URL" -v ON_ERROR_STOP=1 -f backend/migrations/0008_legacy_preserve.sql
#    (migrasi 0002–0007 diterapkan oleh backend saat start; jalankan lebih dulu
#     dengan `npm run api:migrate` atau start API versi baru sekali)

# 2. impor data legacy
export LEGACY_DATABASE_URL="<legacy, read-only>"
export ALGOHUB_MIGRATION_DATABASE_URL="<produksi>"
export LEGACY_BATCH_ID="prod-real-$(date +%Y%m%d)"
/e/tmp/algohub-migration/legacy-migrate.exe extract   # snapshot semua baris legacy
/e/tmp/algohub-migration/legacy-migrate.exe verify    # harus: 0 mismatches
/e/tmp/algohub-migration/legacy-migrate.exe apply     # mapping + rekonsiliasi domain

# 3. berkas storage (96 objek)
python scripts/upload-storage.py --env .env.production.local

# 4. opsional: kecilkan DB produksi setelah semua verifikasi lolos
psql "$ALGOHUB_MIGRATION_DATABASE_URL" -f scripts/prune-legacy-duplicates.sql

# 5. deploy backend + keenam app (build yang memuat login Email/NIM)

# 6. uji sebelum DNS: panggil API produksi langsung dengan Host produksi
curl -s -o /dev/null -w '%{http_code}\n' https://api.<host-baru>/healthz

# 7. arahkan DNS ke host baru
```

## Gerbang verifikasi (stop kalau gagal)

| Cek                     | Perintah                                     | Harus                                              |
| ----------------------- | -------------------------------------------- | -------------------------------------------------- |
| Preservasi baris legacy | `legacy-migrate verify`                      | `0 mismatches`                                     |
| Rekonsiliasi domain     | `legacy-migrate apply`                       | commit sukses tanpa `domain reconciliation failed` |
| Hash password           | query `auth.users` vs `legacy_preserve.rows` | 0 baris berbeda                                    |
| Storage                 | `scripts/upload-storage.py`                  | `terverifikasi 96/96`                              |
| Login nyata             | formulir login pakai 1 akun legacy           | masuk, `/v1/auth/me` 200                           |
| Data tampil             | leaderboard/kelas/presensi                   | angka sama dengan legacy                           |

Angka acuan legacy (harus sama persis di produksi): users 396, progress 16.576,
pengerjaan 1.634, jawaban 3.746, presensi 678 (657 + 21 overflow), audit 24.279,
objek storage 96 / 34.741.189 byte.

## Rollback

DB dan storage legacy tidak pernah dimodifikasi, jadi rollback = kembalikan DNS ke
situs lama. Bila perlu mengulang impor di target, `legacy-migrate apply` idempoten
(aman dijalankan berulang) dan selalu bisa diulang dari nol.

## Setelah cutover

1. Freeze penulisan ke DB legacy (jangan dihapus; simpan minimal satu semester).
2. Pantau `audit_logs` produksi dan error rate API satu jam pertama.
3. Baru setelah stabil: arsipkan `legacy_preserve.rows` ke dump, lalu truncate
   (lihat `docs/migration-status-2026-11-15.md`).
4. Rotasi ulang kredensial yang pernah muncul di transkrip/chat.
