# Migrasi data legacy → v2 (status 2026-11-15)

Batch: `prod-real-20261115` (idempoten, aman dijalankan ulang).
Sumber: DB legacy `project_ap` (read-only, hanya SELECT) + 96 objek storage.

- **Staging**: project `yunznjsfokrpxcqfloic` — DB + storage sudah lengkap dan terverifikasi.
- **Produksi**: project `zngmetvdvsfowerwngma` — DB + storage **sudah terisi** (2026-11-15),
  sisa deploy aplikasi + pengalihan DNS.

## Angka produksi (terverifikasi 2026-11-15)

| Item                                       | Jumlah                                                 |
| ------------------------------------------ | ------------------------------------------------------ |
| migrasi skema                              | `0002`–`0008` (38 tabel public + 3 `legacy_preserve`)  |
| `legacy_preserve.rows`                     | 78.829 baris / 111 relasi, fingerprint `72f6104801b7…` |
| `verify`                                   | 111 tabel, 78.829 baris, **0 mismatch**                |
| users / auth.users / identities / role     | 396 / 396 / 396 / 396                                  |
| learning progress / profiles               | 16.576 / 396                                           |
| practicum attempts / answers               | 1.634 / 3.746                                          |
| academic attendance records                | 657 (+21 overflow)                                     |
| audit_logs                                 | 24.279                                                 |
| storage                                    | 96 objek, 34.741.189 byte, sha256 96/96 cocok          |
| hash password beda                         | **0**                                                  |
| login GoTrue produksi (akun uji sementara) | 200, lalu akun dihapus                                 |
| ukuran DB produksi                         | 127 MB                                                 |

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

## Perbaikan 2026-11-15: mahasiswa tidak melihat data praktikum (produksi)

Gejala: mahasiswa bisa login, tetapi kelas/nilai/sesi/shift praktikum kosong. Data
sebenarnya lengkap (users 396, courses 15, questions 211, sessions 5, activations 104,
attempts 1.634, jawaban 3.746, presensi 657). Penyebabnya bukan import, tetapi bentuk
scope: `practicum_activations.scope` (kolom) memuat `shift`, sedangkan salinan di
`data->'activation'->'scope'` tidak. `practicumEligible` -> `Scope.Contains` membandingkan
scope role mahasiswa (yang membawa shift 1/2) dengan JSON itu, sehingga **0 dari 1.590**
pasangan activation-mahasiswa lolos cek.

- Generator `cmd/legacy-migrate/sql/030_practicum.sql` diperbaiki agar JSON scope
  memuat `shift` dari sumber yang sama.
- Migrasi `0009_activation_scope_shift.sql` mem-backfill `shift` ke 104 activation
  yang sudah termigrasi (idempoten, diverifikasi 0 mismatch).
- Setelah backfill: 1.590/1.590 pasangan class+shift cocok. Kelas/schedule/presensi
  akademik memakai filter MemberID/OwnerID sehingga tidak terdampak.
- Catatan nilai: `pengerjaan_course.total_nilai`, `jawaban_mahasiswa.nilai` dan `feedback`
  memang **NULL di sumber legacy** (0 dari 3.746), jadi v2 nullable = sama dengan legacy.
  Nilai versi aplikasi elearning tersimpan di `assessment_attempts` project
  `tvsawtkevzfqobsfkiag` yang kini **tidak resolve (mati)** dan tidak punya export lokal;
  perlu dicek di dashboard Supabase apakah paused (bisa dipulihkan) atau deleted.

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

## Produksi LIVE (2026-11-15)

DNS di Cloudflare (nameserver aktif: `dion/dolly.ns.cloudflare.com`; panel Cloudhost
sudah stale, bukan zona aktif). Semua domain `verified` di Vercel dan melayani 200:

| Host                       | App       | Status                          |
| -------------------------- | --------- | ------------------------------- |
| `algohub.web.id` + `www`   | portal    | 200                             |
| `auth.algohub.web.id`      | auth      | 200 (`/login` 200)              |
| `admin.algohub.web.id`     | admin     | 401 tanpa sesi (benar)          |
| `siakad.algohub.web.id`    | siakad    | 200                             |
| `elearning.algohub.web.id` | elearning | 200                             |
| `praktikum.algohub.web.id` | praktikum | 200                             |
| `api.algohub.web.id`       | backend   | `/healthz` 200, `/readyz` ready |

Uji login produksi (akun probe sementara, lalu dihapus): `POST /v1/auth/password`
200 + cookie sesi, `/v1/auth/me` 200, `/v1/learning/levels` 200,
`/v1/learning/leaderboard` 200 berisi data legacy asli (XP 5.575 peringkat 1).
Data akhir: users 396, auth.users 396, identities 396, role 396, progress 16.576,
attempts 1.634, `storage.objects` 96, `legacy_preserve.rows` 78.829.

Catatan env: nilai `ALGOHUB_*_ENABLED` harus `true` (default di kode `false`).
Jangan menyalin nilai env dari project staging lewat API — yang keluar ciphertext.

---

## Lampiran A — Inventaris sumber dan klasifikasi data (Phase 6)

Scope: read-only code/schema inventory on 2026-11-13. **Not a live database inventory or authorization to process personal data.** No legacy rows/uploads/credential values were copied or inspected.

### Source authority is unresolved

Legacy root: `E:/rekapProject/project_ap/`.

| Candidate source | Schema/code authority | Caveat |
| --- | --- | --- |
| Praktikum | `praktikum/backend/database/migration/` (001–034), `praktikum/backend/internal/entity/` | Applied version unknown; later entities contain fields absent from early migrations |
| Standalone E-learning | `elearning/docs/setup-new-database.sql`, subsequent SQL and `elearning/src/services/` | NIM-keyed identity and text curriculum IDs; service/schema drift |
| Standalone SIAKAD | `siakad/supabase/migrations/`, current RPC/page code | Older generated `types.ts` describes UUID profiles/attendances; later RPCs use BIGINT users/attendance_logs |
| Consolidated alternative | `docs/sql/004_domain.sql`, `005_elearning.sql`; Praktikum `belajar.go`, `materi.go`, `game.go` | Separate source generation, not evidence that consolidation completed |

Never union these generations or merge people by NIM/email/name automatically. An approved crosswalk must map **source system + entity + original ID** to a target identity. A legacy Supabase ID is provenance, not automatically the new Auth ID.

Only old aggregate metadata was available: June 20, 2026 backup reports 94 Praktikum users; June 21 postmigration summary reports 188 users and 1,580 course-work records. These are **not current counts, not additive populations, and not rehearsal acceptance targets**. Current row/file counts, bytes, completeness and cutoff remain unknown.

Potential snapshot roots (`backups/`, `praktikum/backups/`, historical `praktikum/updateAndPRDERD/migration/` exports) are candidates only. None is selected or authorized.

### Mapping and losslessness decisions

| Source concepts | Target | Safe mapping / unresolved semantics |
| --- | --- | --- |
| users, roles, divisions | `users`, assignments, grants | `nama/full_name → name`; explicit source-specific role alias allowlist; no password/session copying; approved Auth crosswalk required |
| kelas/jadwal/group members | `academic_classes`, `academic_schedules` | Remap roster IDs; `hari → day_of_week`, times → starts/ends; class/major/period and shift grain require crosswalk |
| sesi_praktikum/course/soal | Practicum sessions/courses/questions | Title, sequence, kind, duration, prompt, starter, reference answer; fractional legacy points must be rejected until integer policy approved |
| aktivasi_sesi/course, soal_terpilih | Activations with frozen course/question snapshots | Per-course activation join and ordered question IDs; never fabricate historical snapshots or admission/timer state |
| pengerjaan_course/jawaban_mahasiswa | Attempts, submitted answers, manual grades/jobs | Join selected-question IDs, remap user/course/activation IDs; floats, cached total, activity marks, publication state need explicit policy |
| levels/modules/lessons or level/modul/materi | Learning levels/modules/lessons | Field renames; derive level through module; decode JSON objects vs encoded JSON strings explicitly; flowchart definitions unsupported |
| student_progress/progres_belajar | Learning progress + profiles | Explicit completion mapping and XP ledger/opening-balance policy; never invent code, quiz choice, execution proof or per-lesson XP |
| elearning_progress in SIAKAD | No separate import | A synchronized cache, not another source of completions |
| game questions/history | Bug-hunt questions/question attempts | Current buggy-line vs old multiple-choice variants differ; whole-game history lacks per-question identity/selection and cannot be fabricated |
| attendance_logs/absensi/qr_sessions | Attendance sessions/records | Schedule/session/user crosswalk mandatory; do not guess schedule from date, recorder as validator, or scan type as QR proof |
| inventory_items/inventaris | Inventory items | Old quantity does not establish available stock; preserve unavailable units; approve code/category/state mapping |
| rentals/peminjaman | Inventory rentals | Currency/unit/integer policy and status mapping; no inferred payment verification, return/approval times |
| reports, financial records, prestige, unsupported content | Rejected/quarantined pending decision | No demonstrated lossless target; financial ledger is not rental revenue |

The initial executable contract is **synthetic only**, not an adapter for any of these unverified live generations. Unsupported rows require an explicit rejection reason; no silent omission.

### Target schema and load order

[Migration catalog](../../backend/migrations/migrate.go) pins the exact SQL bytes. Existing staging already applied 0002–0007; run read-only history comparison, not `Apply`, against an existing target.

- 0002–0004: identity and seeded access catalogs.
- 0005: eight Practicum tables.
- 0006: nine Learning tables.
- 0007: eight Academic/inventory tables.
- Total: 37 application tables plus `schema_migrations` = 38 public RLS tables. `auth.users` is a prerequisite outside this set.

Load users → assignments → grants after schema/catalog initialization. Independent domain branches:

1. Practicum sessions → courses → questions → activations (parent is **session**) → attempts → grading jobs.
2. Learning levels → modules → lessons → progress; bug-hunt → game history; profiles reconciled against imported activity.
3. Academic classes → schedules → attendance sessions → records; inventory items → rentals.
4. Approved audit/notification history last; ephemeral sessions, handshakes, rate counters, leases and operations excluded.

Use insert-or-equal, **not blanket UPSERT**: game history rejects every UPDATE and other triggers freeze completed/submitted state. Do not disable constraints/triggers/RLS.

Domain tables use indexed `id,parent_id,owner_id,scope,status,unique_key,expires_at` plus typed `data` JSON. Reconcile **both SQL FK and embedded JSON relationships**. Required application keys include `profile-<user>`, `progress-<lesson>-<user>`, `att-rec-<session>-<user>`, attempt `<activation>:<user>` unique key and grading-job ID equal to attempt ID. Arbitrary IDs can pass SQL yet fail application lookups.

Cross-check profile XP/counters, published manual totals, and inventory invariant `total - available >= approved rental quantities`. Do not replace unknown scope with unrestricted `{}`. Final target role vocabulary is koordinator/asisten/mahasiswa/peminjam, with six assistant divisions; no new admin/pengajar role.

### Classification / masking boundary

| Class | Examples | Default rehearsal action |
| --- | --- | --- |
| Restricted identity | Names, NIM, contact/social details, photos, crosswalks | Synthetic replacement only until approved |
| Confidential academic/behavioral | Answers, grades, feedback, attendance, class membership, progress/activity/rentals | Synthetic only; free text may include health or other sensitive information |
| Confidential operational | Answer keys, unpublished content, finance, audit snapshots, IP/user-agent | Do not copy; separately approve scope and access |
| Credentials/ephemeral authentication | Passwords/hashes/salts, sessions, tokens, QR bearers, push keys | Excluded; never logged, exported or migrated by this tooling |
| Approved public content | Published lesson assets | Public status must be established, not inferred from an accessible URL |

Actual masking rules, retention duration, authorized readers, deletion owner and allowed rejection tolerance remain owner decisions. Hashing an identity is pseudonymization, not proof of anonymization.

### File-copy plan (no copying authorized)

| Convention from code | Required handling |
| --- | --- |
| Praktikum default `public-assets`, `{folder}/{timestamp}.{ext}` | Verify actual bucket and ownership; source references include question image, guide file and profile image |
| Private `laporan-besar`, `{reportID}/{studentID}/{type}-{timestamp}.{ext}` | IDs/paths are identifying; preserve private ACL; object path rather than expiring signed URL; no confirmed target attachment model yet |
| E-learning `lesson-images`, public URLs in content | Inspect actual bytes/MIME only after approval: compression may emit JPEG while preserving the original extension |
| SIAKAD file/image URL fields | Bucket/ownership/local root unknown; URL alone does not authorize copying |

After explicit authorization: operator supplies a non-secret manifest (opaque reference, approved source object/version, size, SHA-256, verified MIME, destination key, privacy class, referring rows). Reject traversal, symlinks, collisions and missing objects. Copy into an isolated destination prefix with bounded I/O, no overwrite, private-by-default ACL, then verify count/bytes/hash and referential coverage. Retry equal existing objects as no-op; changed bytes fail. Publish reference rewrites only after validation. Rollback removes only newly copied unchanged objects from that manifest. Keep unsupported attachments quarantined rather than silently dropping them. No storage credential, object download or upload is performed in this local preparation.

---

## Lampiran B — Audit legacy database 2026-11-14

**Mode:** read-only, zero write. Script: `scripts/prod-audit/legacy-schema-audit.sql`
+ follow-up query terverifikasi kolom. Output mentah:
`scripts/prod-audit/results/praktikum-legacy-2026-11-14.txt`.
**Proyek yang diaudit:** satu database legacy gabungan (Praktikum/E-learning/SIAKAD
bridge) di Supabase `oidwvfcxlxfyojkyurmm` — bukan tiga database terpisah.

### Angka kunci

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

### Timestamp terakhir per tabel (titik delta import)

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

### Temuan penting untuk pipeline import

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

### Dampak ke runbook cutover

- Estimasi volume import naik sedikit vs Phase 6 (15.948 progress vs
  19.026 receipt staging) — masih dalam durasi terbukti (~6,5 menit).
- Dry-run harus memakai mapping kolom eksplisit dari temuan no. 1–3,
  bukan asumsi plan lama.
- Keputusan owner yang tersisa tidak berubah (runbook §6).

### Kredensial

- Tidak ada password/secret yang ditulis di dokumen ini, hasil output,
  atau commit. Akses via env file legacy yang sudah ada di luar repo v2.
- Sesi audit selesai; tidak ada koneksi yang ditinggalkan terbuka.
