# Status migrasi data legacy → v2 (2026-11-15)

Batch: `prod-real-20261115` (idempoten, aman dijalankan ulang).
Sumber: DB legacy `project_ap` (read-only, hanya SELECT) + 96 objek storage.
Tujuan: staging Supabase `yunznjsfokrpxcqfloic` (DB yang dipakai `api.staging.algohub.web.id`).

## Hasil yang sudah di staging

| Domain                                                                      | Jumlah                                          |
| --------------------------------------------------------------------------- | ----------------------------------------------- |
| `legacy_preserve.rows` (semua baris legacy, apa pun mappingnya)             | 78.829 baris / 65 tabel berisi                  |
| `legacy_preserve.objects` (metadata objek, sha256 + ukuran)                 | 96 objek / 34.741.189 byte                      |
| Bucket staging `public-assets` (public) + `laporan-besar` (private, 10 MiB) | 96 objek terunggah, sha256 cocok 96/96          |
| `public.users` + `auth.users` + `auth.identities`                           | 396 / 396 / 396                                 |
| `user_role_assignments`                                                     | 396 (20 asisten, 2 koordinator, 374 mahasiswa)  |
| `learning_levels` / `modules` / `lessons`                                   | 6 / 25 / 81                                     |
| `learning_progress`                                                         | 16.576 (semua completed, sesuai legacy)         |
| `learning_profiles`                                                         | 396 (total XP & achievement sama dengan legacy) |
| `learning_game_history` / bug hunt soal                                     | 299 / 141                                       |
| `practicum_sessions` / `courses` / `questions`                              | 5 / 15 / 211                                    |
| `practicum_activations` / `attempts`                                        | 104 / 1.634                                     |
| jawaban tersimpan di dalam attempt                                          | 3.746 (rekonsiliasi **per id jawaban**)         |
| `academic_classes` / `schedules`                                            | 13+1 container / 9+1 container                  |
| `academic_attendance_sessions` / `records`                                  | 62 (57 qr session tercakup semua) / 657         |
| baris presensi berlebih yang tak bisa masuk unique key                      | 21 → disimpan di `academic_operations`          |
| `audit_logs` (audit 15.141 + activity 7.538 + siakad 1.600)                 | 24.279                                          |

Attendance: 657 + 21 = 678 baris legacy, semuanya terhitung.

## Bukti verifikasi

- `legacy-migrate verify`: 111 tabel sumber, 78.829 baris, 0 mismatch.
- `legacy-migrate apply` menjalankan `sql/090_verify_domain.sql` di dalam transaksi
  yang sama: rekonsiliasi per-domain lulus (termasuk per-id jawaban, XP total,
  achievement, jumlah presensi, audit).
- Password tidak diubah: 396/396 `auth.users.encrypted_password` identik dengan
  `public.users.password_hash` legacy (bcrypt).
- Login end-to-end di staging: `POST /v1/auth/password` → 200 + cookie, `GET /v1/auth/me`
  → 200, `/v1/learning/levels` dan `/v1/learning/leaderboard` → 200 dengan data asli
  (leaderboard menampilkan mahasiswa legacy dan XP-nya).
- Test suite: `api:build`, `api:vet`, `api:test`, `test:frontend` (29+29+5+18) semuanya lulus.

## Temuan penting saat verifikasi

1. **Sequence identitas tidak ikut naik** — baris legacy masuk dengan id eksplisit,
   sehingga `users_id_seq` tertinggal di 609 padahal `users.id` sudah 757. Akibatnya
   setiap pendaftaran sesi login baru gagal dengan 409. Diperbaiki permanen di
   `sql/060_sequences.sql` (dijalankan setiap apply).
2. **11 qr session tidak punya baris presensi** — sesi presensi sekarang dibuat untuk
   semua 57 qr session, bukan hanya yang punya attendance.
3. **21 baris presensi ganda** dalam satu (sesi, meeting, mahasiswa) tidak bisa masuk
   unique key v2; disimpan utuh di `academic_operations` (kind `attendance_overflow`).
4. **Login berbasis NIM** — 350 dari 396 user legacy tidak punya email. Alamat
   sintetis `<nim>@users.algohub.web.id` sekarang otomatis dibentuk dari NIM, form
   login jadi "Email atau NIM" (`packages/auth-client/src/server.server.ts`,
   `apps/auth/src/routes/login/+page.svelte`, unit test baru).
5. 3 akun auth legacy tidak punya NIM yang cocok (`ivan2211083`, `mhd2211001`,
   `muhammad2211066`) — tetap dipreservasi, perlu keputusan owner.
6. Metadata ukuran objek `laporan-besar` di DB legacy rusak (13/5/13/15 byte);
   ukuran sebenarnya diambil dari byte hasil unduhan (ada di `legacy_preserve.objects`).

## Integritas akhir staging

- 300 mahasiswa tercakup di 13 kelas (9 kelas berisi, sisanya kosong di legacy).
- 0 progress tanpa lesson, 0 attempt tanpa activation, 0 presensi tanpa sesi,
  0 user tanpa role.
- Storage: 92 objek `public-assets` + 4 objek `laporan-besar`, total 34.741.189 byte,
  sha256 semua objek dicocokkan ulang setelah unggah (96/96). URL publik `public-assets`
  merespons 200 tanpa kredensial, `laporan-besar` tetap privat.

## Ukuran DB staging (setelah dirapikan)

135 MB → **121 MB**. Yang dirapikan: salinan baris legacy di dalam
`audit_logs.detail.row` dan `academic_operations.data.row` dibuang (tidak dipakai
kode mana pun; isi asli tetap di `legacy_preserve.rows`), lalu `VACUUM FULL`.
Audit 22 MB → 12 MB, operations 12 MB → 7,3 MB. Pipeline dijalankan ulang setelah
pembersihan: `apply` + `verify` tetap hijau (0 mismatch).

Sisa komponen terbesar: `legacy_preserve.rows` 40 MB (snapshot 78.829 baris legacy,
besar karena JSONB + index + sha256 per baris). Ini sengaja dipertahankan karena
menjadi acuan verifikasi ulang; salinannya sudah diarsipkan sebagai
`E:/tmp/algohub-migration/staging-legacy-preserve-20261115.dump` (6,7 MB, teruji
restore: 78.829 baris / 96 objek / 1 batch). Kalau nanti ingin DB makin kecil:

```bash
psql "$ALGOHUB_MIGRATION_DATABASE_URL" -c 'truncate legacy_preserve.rows'
# pulihkan kapan saja:
pg_restore -d "$ALGOHUB_MIGRATION_DATABASE_URL" --data-only staging-legacy-preserve-20261115.dump
```

## Belum selesai (butuh owner)

1. **Cutover produksi**: butuh ref + service key project Supabase produksi, akses
   registrar/DNS `algohub.web.id`, dan environment produksi untuk keenam app.
2. **Rotasi password DB legacy** — sempat bocor ke transkrip lewat pesan error psql.
3. 4 asisten legacy tidak punya baris `divisi`; divisi terisi untuk 19 dari 20 asisten.

## Checklist cutover produksi

1. Isi `.env.production.local` (atau env Vercel) dengan `SUPABASE_URL`,
   `SUPABASE_SERVICE_ROLE_KEY`, `ALGOHUB_MIGRATION_DATABASE_URL`, `ALGOHUB_DATABASE_URL`
   project produksi.
2. Terapkan migrasi `0002`–`0008` pada DB produksi (0008 = skema `legacy_preserve`).
3. Jalankan `legacy-migrate extract` → `verify` → `apply` dengan
   `LEGACY_BATCH_ID=prod-real-20261115` (sumber tetap legacy, read-only).
4. Jalankan `upload_storage.py` dengan env produksi (idempoten, memverifikasi sha256).
5. Deploy keenam app + backend API (build baru berisi login Email/NIM).
6. Arahkan DNS `algohub.web.id` beserta subdomain `auth`, `admin`, `siakad`,
   `elearning`, `praktikum`, `api` ke host baru, lalu uji login satu mahasiswa nyata.
7. Setelah cutover stabil, hentikan penulisan ke DB legacy (jangan dihapus).

## Cara menjalankan ulang

```bash
cd backend && go build -o /e/tmp/algohub-migration/legacy-migrate.exe ./cmd/legacy-migrate
export LEGACY_DATABASE_URL="<legacy, hanya SELECT>"
export ALGOHUB_MIGRATION_DATABASE_URL="<target>"
export LEGACY_BATCH_ID="prod-real-20261115"
./legacy-migrate extract   # snapshot baris legacy ke legacy_preserve
./legacy-migrate verify    # bandingkan snapshot vs sumber
./legacy-migrate apply     # mapping + verifikasi domain dalam satu transaksi
```
