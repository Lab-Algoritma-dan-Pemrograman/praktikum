# Buka/Tutup Course per Peserta — Rancangan

> **Status: DIIMPLEMENTASIKAN** (backend + frontend + test). Doc ini = rancangan asli.

## Masalah

Saat ini `BukaTutupCourse` (admin) bekerja **massal**: menutup course
auto-submit SEMUA peserta, membuka ulang melepas kunci SEMUA peserta. Asisten
butuh menangani **1 atau beberapa peserta** saja (mis. peserta yang minta
ulang, error jaringan, ketinggalan) tanpa memengaruhi peserta lain.

## Model keputusan

`aktivasi_course.is_open` tetap **global** (gerbang seluruh peserta).
Per peserta, kondisi "bisa mengetik" ditentukan oleh status
`pengerjaan_course`:
- `selesai` → editor read-only (`locked = !is_open || status === 'selesai'`)
- `belum_dikerjakan` / `sedang_dikerjakan` → editable (selama `is_open`)

Jadi aksi per peserta = ubah status `pengerjaan_course` + status submit jawaban
peserta itu saja. `is_open` course tidak disentuh.

## Aksi per peserta

| Aksi | Effect |
|---|---|
| **Buka Kunci** (unlock) | `pengerjaan_course.status` → `belum_dikerjakan`, `waktu_mulai/selesai` nil; `jawaban_mahasiswa.is_submitted` → false (isi jawaban TIDAK dihapus). Peserta bisa mengetik & mengerjakan ulang. |
| **Tutup / Submit** | `pengerjaan_course.status` → `selesai`, `waktu_selesai` NOW; `jawaban_mahasiswa.is_submitted` → true (auto-submit). Peserta terkunci. |

Catatan: `waktu_mulai` per peserta direset saat unlock → timer peserta tersebut
mulai ulang (tidak mengganggu anchor global `aktivasi_course.started_at`).

## Backend

### Repository

`pengerjaan_repository.go` (interface + impl):
- `ResetForMahasiswa(mahasiswaID, aktivasiSesiID, courseID int) (int64, error)`
  — `WHERE mahasiswa_id = ? AND aktivasi_sesi_id = ? AND course_id = ? AND status <> 'belum_dikerjakan'` → set status belum + waktu nil.
- `MarkSelesaiForMahasiswa(mahasiswaID, aktivasiSesiID, courseID int) error`
  — `WHERE ... AND status <> 'selesai'` → set selesai + waktu_selesai NOW.

`jawaban_repository.go`:
- `UnmarkSubmittedForMahasiswaCourse(mahasiswaID, aktivasiSesiID, courseID int) (int64, error)` — kebalikan per-user (yang massal sudah ada).
- `MarkSubmittedForMahasiswaCourse` **sudah ada**.

### DTO

```go
type BukaTutupPesertaRequest struct {
    AktivasiCourseID int   `json:"aktivasi_course_id" binding:"required"`
    MahasiswaIDs     []int `json:"mahasiswa_ids" binding:"required,min=1,dive,gt=0"`
    Aksi             string `json:"aksi" binding:"required,oneof=unlock submit"`
}
```

### Usecase

`BukaTutupPeserta(req) (int, error)`:
1. `FindCourseByID` → validasi course ada.
2. Loop `req.MahasiswaIDs`:
   - `unlock` → `ResetForMahasiswa` + `UnmarkSubmittedForMahasiswaCourse`.
   - `submit` → `MarkSelesaiForMahasiswa` + `MarkSubmittedForMahasiswaCourse`.
3. Return jumlah peserta terdampak.

### Handler & Route

- `POST /api/admin/aktivasi-course/buka-tutup-peserta` (RequirePermission aktivasi write).
- Audit log: `BUKA_TUTUP_PESERTA` dengan `aktivasi_course_id`, `jumlah_peserta`, `aksi`.

## Frontend (halaman admin aktivasi)

- **Tabel monitor peserta** (yang sudah ada): tambah kolom **checkbox** per baris
  + filter default per course aktif.
- Di tiap **card course**, tambah dua tombol:
  - `Buka Kunci Terpilih` → POST `{aksi:'unlock', mahasiswa_ids:[...]}`
  - `Tutup Terpilih` → POST `{aksi:'submit', mahasiswa_ids:[...]}`
- Tombol disabled kalau tidak ada checkbox tercentang.
- Setelah sukses → `loadPeserta` ulang (status ter-refresh).

## Edge cases

- Peserta yang belum punya baris `pengerjaan_course` (belum join): `ResetForMahasiswa`/`MarkSelesaiForMahasiswa` tidak kena baris apa pun (0 rows) — aman.
- Course `is_open=false`: unlock per peserta TIDAK membuat ruang ujian terbuka (gerbang global tetap tertutup). Peserta baru bisa mengetik setelah course dibuka. Ini perilaku aman & jelas.
- `waktu_selesai` di-set hanya saat `status <> 'selesai'` (tidak menimpa waktu selesai lama).
- `waktu_mulai` direset → peserta susulan dengan timer sendiri (waktu_mulai pribadi) mulai ulang juga.

## Verifikasi

- `go build ./... && go test ./...`.
- Test usecase: submit per 1 peserta tidak menyentuh peserta lain; unlock reset hanya peserta target.
- `bun run check` frontend.
- Manual: pilih 1 peserta → Tutup → hanya dia yang selesai; Buka Kunci → hanya dia yang bisa ngetik lagi.

## Open Questions

1. **UI**: checkbox di tabel monitor yang ada (per course), atau buat panel terpisah di tiap card course? (rekomendasi: checkbox di tabel, filter per course)
2. Apakah perlu aksi "Tutup" per peserta, atau cukup "Buka Kunci" saja? (rekomendasi: dua-duanya, simetris dengan massal)
3. Konfirmasi: saat `submit`, apakah `waktu_selesai` di-set ke NOW (wajar) atau dibiarkan?
