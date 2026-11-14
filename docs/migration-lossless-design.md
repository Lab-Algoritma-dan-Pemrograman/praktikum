# Migrasi Lossless Legacy → v2

Status: desain + implementasi in-repo. Sumber: Supabase LEGACY `oidwvfcxlxfyojkyurmm` (project_ap).
Target: Supabase V2 `yunznjsfokrpxcqfloic` (project_ap_v2).

Aturan yang tidak boleh dilanggar:

1. Sumber hanya dibaca. Tidak ada `INSERT/UPDATE/DELETE/DDL` ke source, tidak ada `storage.remove`.
2. Setiap baris sumber dipreservasi utuh (semua kolom, nilai asli, PK asli, sha256) di `legacy_preserve`.
3. Setiap objek Storage sumber disalin byte-identik (sha256 + size cocok) dengan privacy yang sama.
4. Kredensial dipindahkan apa adanya: `users.password_hash` (bcrypt 60 char, `$2a$`/`$2b$`) menjadi `auth.users.encrypted_password`.
5. Tidak ada data yang dibuang karena "tidak punya padanan". Baris tanpa padanan domain tetap dipreservasi.
6. Apply idempotent, satu transaksi, receipt atomik, bisa diulang.

## Inventaris sumber (terukur 2026-11-15)

- 107 tabel `public`, 61 berisi, 78.597 baris.
- `auth.users` 67 (semua bcrypt + confirmed, provider=email), `auth.identities` 67.
- `storage.objects` 96 (public-assets 92 public, laporan-besar 4 private), byte total 34.741.189.
- Hash password: `public.users.password_hash` 396/396 bcrypt 60 char.
- Email asli: 46 user; 350 tanpa email.
- `users.supabase_user_id` 45 valid; 22 auth user tanpa tautan, 19 di antaranya cocok via `raw_user_meta_data->>'nim'`, 3 tidak ada padanan NIM.

Baseline exact count (dipakai sebagai acceptance): users 396, student_progress 16.576, pengerjaan_course 1.634, jawaban_mahasiswa 3.746, attendance_logs 678, schedules 12, lessons 81, modules 25, levels 6, soal 211.

## Temuan integritas (menentukan mapping, bukan asumsi)

| Relasi                                                   | Hasil                                                      |
| -------------------------------------------------------- | ---------------------------------------------------------- |
| `student_progress.lesson_id` → `lessons`                 | 0 orphan ⇒ `lessons` kanonik                               |
| `student_progress.lesson_id` → `materi`                  | 7.187 orphan ⇒ `materi` salinan lama, preservasi saja      |
| `student_progress.nim` → `users.nim`                     | 275/275 cocok                                              |
| `jawaban_mahasiswa.soal_terpilih_id` → `soal_terpilih`   | 0 orphan                                                   |
| `pengerjaan_course` → `aktivasi_sesi`, `course`          | 0 orphan                                                   |
| `soal_terpilih` → `soal`, `aktivasi_sesi`                | 0 orphan                                                   |
| `soal.course_id` → `course`                              | 0 orphan, 20 baris `is_arsip`                              |
| `keanggotaan` / `group_members` / `schedule_assignments` | 0 orphan                                                   |
| `attendance_logs.user_id` (uuid) → `auth.users`          | 0 cocok ⇒ kolom mati, identitas via `custom_user_id` (NIM) |
| `weekly_standings.nim`                                   | 1 baris tanpa user (preservasi + catat)                    |
| `storage.objects` metadata size bucket `laporan-besar`   | korup (13/5/13/15 byte) ⇒ ukuran diambil dari byte unduhan |

## Pola penyimpanan target

Semua tabel domain v2 berbentuk document store:
`id text, parent_id text, owner_id bigint, scope jsonb, status text, unique_key text, expires_at timestamptz, data jsonb`.

Konsekuensi: payload asli disimpan utuh di `data`, `id`/`owner_id`/`status`/`unique_key` diisi dari field sumber yang relevan. Tidak ada kolom yang hilang.

## Skema preservasi (migration 0008)

```sql
create schema if not exists legacy_preserve;

create table legacy_preserve.rows (
  source_table text not null,
  source_pk    text not null,
  row_data     jsonb not null,
  row_sha256   text not null,
  batch_id     text not null,
  primary key (source_table, source_pk)
);

create table legacy_preserve.objects (
  bucket      text not null,
  path        text not null,
  size        bigint not null,
  sha256      text not null,
  mimetype    text,
  is_public   boolean not null,
  source_meta jsonb,
  primary key (bucket, path)
);

create table legacy_preserve.batches (
  batch_id            text primary key,
  source_fingerprint  text not null,
  counts              jsonb not null,
  target_fingerprint  text not null,
  status              text not null,
  created_at          timestamptz not null default now(),
  applied_at          timestamptz
);
```

Skema `legacy_preserve` tidak diberi grant ke `anon`/`authenticated`; hanya service role.

## Mapping tabel → domain v2

Kolom `data` menyimpan objek JSON yang mengikuti `backend/internal/entity/*.go`.

| Sumber                                                                                                     | Target                                                                        | Catatan                                                                                                                            |
| ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `users` (396)                                                                                              | `public.users` + `auth.users` + `user_role_assignments`                       | id asli dipertahankan; `password_hash` → `encrypted_password`; role → role_code; `kelas/shift/divisi/gelombang/kelompok` → `scope` |
| `users.divisi`                                                                                             | `user_role_assignments.division_code`                                         | hanya bila kode divisi valid                                                                                                       |
| `keanggotaan`                                                                                              | `academic_classes` (`data.student_ids`) + `user_role_assignments`             | kelas per NIM                                                                                                                      |
| `kelas`                                                                                                    | `academic_classes`                                                            | code = `nama_kelas`                                                                                                                |
| `jadwal`                                                                                                   | `academic_schedules`                                                          | hari/jam/shift/kelas                                                                                                               |
| `schedules` (12)                                                                                           | `academic_schedules`                                                          | tabel kedua; simpan terpisah via unique_key `legacy:schedules:<id>`                                                                |
| `attendance_logs`                                                                                          | `academic_attendance_records`                                                 | identitas dari `custom_user_id` (NIM)                                                                                              |
| `qr_sessions`                                                                                              | `academic_attendance_sessions`                                                | token dipertahankan                                                                                                                |
| `assistant_availability`, `schedule_assignments`, `group_members`, `group_assistants`                      | `academic_operations`                                                         | dokumen operasional                                                                                                                |
| `levels`                                                                                                   | `learning_levels`                                                             | kanonik (EN)                                                                                                                       |
| `level` (7)                                                                                                | `learning_levels` via unique_key `legacy:level:<id>`                          | preservasi terpisah, tidak di-union                                                                                                |
| `modules`                                                                                                  | `learning_modules`                                                            |                                                                                                                                    |
| `modul` (29)                                                                                               | `learning_modules` via unique_key `legacy:modul:<id>`                         |                                                                                                                                    |
| `lessons`                                                                                                  | `learning_lessons`                                                            | termasuk `quiz`, `test_cases`, `flowchart_distractors`, `exercise_type`                                                            |
| `materi` (96)                                                                                              | `learning_lessons` via unique_key `legacy:materi:<id>`                        | salinan lama, bukan di-union                                                                                                       |
| `student_progress`                                                                                         | `learning_progress`                                                           | nim → owner_id; `completed/completed_at`                                                                                           |
| `achievements`                                                                                             | `learning_lessons`? tidak — `learning_game_history`? tidak                    | lihat baris bawah                                                                                                                  |
| `pencapaian` / `achievements`                                                                              | tabel profil: `learning_profiles` (achievements list) + preservasi            | definisi achievement disimpan di `legacy_preserve` dan mirror di `learning_operations`                                             |
| `unlocked_achievements`                                                                                    | `learning_profiles.data.unlocked_achievements`                                | digabung per user                                                                                                                  |
| `game_history`                                                                                             | `learning_game_history`                                                       |                                                                                                                                    |
| `game_soal` / `game_questions`                                                                             | `learning_bug_hunt`                                                           | ID berbeda; keduanya dimasukkan dengan unique_key pembeda                                                                          |
| `game_konfigurasi` / `game_settings`                                                                       | `learning_operations`                                                         | setting                                                                                                                            |
| `weekly_standings`                                                                                         | `learning_operations`                                                         | snapshot leaderboard                                                                                                               |
| `user_xp` (users.xp/level/streak/last_active/study_time)                                                   | `learning_profiles`                                                           | 1 baris per user                                                                                                                   |
| `sesi_praktikum`                                                                                           | `practicum_sessions`                                                          |                                                                                                                                    |
| `course`                                                                                                   | `practicum_courses`                                                           |                                                                                                                                    |
| `soal`                                                                                                     | `practicum_questions`                                                         | `is_arsip` dipertahankan di `data`                                                                                                 |
| `aktivasi_sesi`                                                                                            | `practicum_activations` (level sesi)                                          | `data.legacy_kind=session`                                                                                                         |
| `aktivasi_course`                                                                                          | `practicum_activations`                                                       |                                                                                                                                    |
| `soal_terpilih`                                                                                            | `practicum_activations.data.test_question_ids` + preservasi                   |                                                                                                                                    |
| `pengerjaan_course`                                                                                        | `practicum_attempts`                                                          |                                                                                                                                    |
| `jawaban_mahasiswa`                                                                                        | `practicum_attempts.data.answers[]` + preservasi per jawaban                  | nilai/feedback/history dipertahankan                                                                                               |
| `peserta_susulan`                                                                                          | `practicum_operations`                                                        |                                                                                                                                    |
| `audit_logs`, `activity_logs`, `siakad_audit_logs`                                                         | `audit_logs`                                                                  | app pembeda di `app`                                                                                                               |
| `active_sessions`, `push_subscriptions`, `revoked_tokens`, `token_blocklist`, `sso_handshakes`, `xp_bonus` | `auth_sessions`? tidak — `legacy_preserve` + `academic_operations` bila perlu | status ephemeral dicatat                                                                                                           |
| `divisi`, `divisi_izin`, `izin`, `division_access`                                                         | `divisions`, `division_permissions`, `permissions`, `role_permissions`        | seed v2 sudah ada; tambah yang belum                                                                                               |
| `konfigurasi`, `system_settings`                                                                           | `academic_operations`                                                         |                                                                                                                                    |
| `laporan_besar`, `laporan_besar_target`, `pengumpulan_laporan_besar`, `pedoman_laporan`                    | `academic_operations`                                                         | file via storage                                                                                                                   |
| `_arsip_*`, `_bak_*`, `*_backup_prestige`, `playground_*`, `materi_mahasiswa`, `student_lessons`           | `legacy_preserve.rows` saja                                                   | arsip/backup: jangan dihapus, jangan di-union                                                                                      |
| `migration_siakad.*`, `net.*`, `cron.*`, `vault.*`, `realtime.*`                                           | tidak dimigrasi (milik platform)                                              | dicatat di laporan                                                                                                                 |

Preservasi `legacy_preserve.rows` berlaku untuk **semua** tabel `public` (107), terlepas dari apakah ada padanan domain. Jadi tabel tanpa padanan tetap utuh dan bisa direkonstruksi kapan pun.

## Auth

- 396 user mendapat `auth.users` dengan `encrypted_password = users.password_hash` (bcrypt identik), `email_confirmed_at` diisi, `raw_user_meta_data.legacy_user_id = users.id`.
- Email: 46 memakai email asli; 350 memakai `NIM@users.algohub.web.id` (detail internal, bukan email terverifikasi milik orang lain). Form login diberi label "Email atau NIM" dan menerjemahkan NIM ke identitas tersebut.
- 22 auth user lama tanpa tautan: 19 dipetakan via NIM ke user yang sudah ada (tidak membuat duplikat); 3 tanpa padanan NIM dipreservasi di `legacy_preserve` dan dilaporkan.

## Verifikasi (harus lulus sebelum cutover)

1. Per tabel: count source == count `legacy_preserve.rows` == count domain target (bila dipetakan 1:1).
2. Per baris: sha256 JSON sumber == sha256 yang tersimpan.
3. Semantik jawaban: untuk setiap `jawaban_mahasiswa.id`, cocokkan di target `attempt.data.answers[question_id]` nilai, `nilai`, `feedback`, `waktu_submit` — berbasis ID jawaban, bukan count soal.
4. Storage: 96 objek, sha256 + size identik, privacy sama, 0 collision, 0 overwrite.
5. Login: NIM + password lama berhasil untuk sampel (mahasiswa/asisten/koordinator), role dan scope sesuai, akses negatif ditolak.
