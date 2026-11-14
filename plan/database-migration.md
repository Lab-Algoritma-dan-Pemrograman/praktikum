# Database, Storage, dan Data Migration Plan

**Status:** Target baseline — belum dieksekusi

## 1. Keputusan utama

- Hanya **satu Supabase project baru/fresh dan satu PostgreSQL database** untuk
  seluruh Algohub.
- Project legacy SIAKAD, E-learning, dan Praktikum menjadi sumber migrasi
  read-only; tidak dipakai sebagai target schema production baru.
- Supabase Auth menjadi identity namespace tunggal.
- Browser tidak query tabel privat secara langsung; operasi domain lewat Go API.
- Migrasi memakai SQL versioned migration. `AutoMigrate` tidak dijalankan di
  production/serverless.
- Sumber kebenaran setelah cutover adalah schema target, bukan salah satu schema
  legacy yang ditempel apa adanya.

## 2. Temuan schema legacy

| Sumber | Temuan |
|---|---|
| SIAKAD | sekitar 26 tabel; memiliki `users`, jadwal, absensi, inventaris, rental, RPC, audit, `division_access`, auth bridge |
| E-learning | sekitar 14 tabel; levels/modules/lessons, progress, game, achievement, assessment legacy, activity log |
| Praktikum | schema Go sendiri dengan user, kelas, sesi, course, soal, attempt, jawaban, audit; migrasi sampai setidaknya 34 file |
| Identity | variasi `admin/kordas/praktikan/user` versus role kanonik; beberapa field memakai `nama/divisi`, lainnya `full_name/division` |
| Data quality | audit lama mencatat 268 user SIAKAD dengan 22 NIM duplikat; angka ini wajib diverifikasi ulang dari backup terbaru |

Dokumen sumber: `project_ap/docs/ERD-gabungan*.md`,
`docs/_schema_raw/*.json`, migration directories, dan backup SQL. Tidak ada data
production yang diubah oleh planning ini.

## 3. Target penggabungan

### Identity

- `auth.users` menyimpan credential.
- `users` menyimpan profil aplikasi dan `auth_user_id`.
- Role/division/permission dipindahkan ke model assignment target.
- NIM boleh nullable hanya untuk `peminjam`, tetapi unique bila terisi.
- Alias legacy dipetakan dalam tabel staging, lalu dibersihkan sebelum publish.

### Domain

- Praktikum menjadi sumber utama mesin asesmen, attempt, jawaban, timer, dan nilai.
- E-learning mempertahankan content/progress/gamification; tabel assessment yang
  duplikat tidak menjadi sumber nilai kedua.
- SIAKAD mempertahankan jadwal, kelas, absensi, inventaris, rental, dan laporan.
- Audit log digabung menjadi satu format dengan `app`, `actor`, `subject`, scope,
  request ID, dan detail before/after yang sudah disanitasi.

### Storage

Gunakan empat bucket Supabase karena tidak menambah database dan membantu policy:

| Bucket | Isi | Akses |
|---|---|---|
| `portal` | asset publik portal | public hanya untuk asset yang memang publik |
| `siakad` | dokumen akademik/lab | private + signed URL |
| `elearning` | materi/gambar pembelajaran | public/private sesuai object policy |
| `praktikum` | pedoman, submission, lampiran soal | private + signed URL |

Object path tetap dinamespace, misalnya `{domain}/{resource}/{id}/{uuid}`.
Client tidak boleh memilih bucket atau object path bebas.

## 4. Urutan migrasi data

1. Freeze schema legacy dan buat manifest backup/checksum.
2. Export schema/data/Auth metadata dari semua sumber tanpa secret.
3. Load ke staging database satu project yang sama, bukan production target.
4. Normalisasi role alias, division casing, NIM, email, timezone, dan foreign key.
5. Resolve duplicate identity dengan deterministic matching dan exception queue.
6. Import identity dan assignment; verifikasi jumlah user dan sample manual.
7. Import domain SIAKAD, Praktikum, lalu E-learning sesuai dependency.
8. Import file metadata lalu copy object dengan checksum; file privat tetap private.
9. Jalankan consistency checks, RLS tests, API contract tests, dan E2E smoke.
10. Dry-run cutover, ukur durasi, catat gap, lalu ulangi sampai repeatable.
11. Cutover dengan write freeze terjadwal, final delta import, dan DNS switch.
12. Simpan legacy read-only selama retention period; hapus hanya setelah rollback
    window lewat dan ada persetujuan eksplisit.

## 5. Rollback dan validasi

- Rollback aplikasi: kembali ke deployment sebelumnya tanpa mengubah schema target.
- Rollback data: restore snapshot target atau replay delta yang tervalidasi; tidak
  memakai `DELETE` massal tanpa dry-run.
- Validasi: row count, unique identity, FK orphan, total nilai, progress, jumlah
  object, checksum file, audit event, dan sample user per role/divisi.
- RPO/RTO final ditentukan sebelum cutover; default planning: RPO ≤ 24 jam dan
  RTO ≤ 4 jam sampai backup/restore drill membuktikan target yang lebih baik.
