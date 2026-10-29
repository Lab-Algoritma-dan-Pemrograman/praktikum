# Rancangan — Modul Laporan Besar (Assignment + Submission + Gerbang Ujian)

> Status: **DRAFT FINAL untuk review.** Belum ada kode yang diubah.
> Konteks: Laporan Besar adalah tugas **berdiri sendiri** — di luar Modul 1–6 dan
> di luar Ujian Praktik. Setiap tugas wajib dua berkas: **laporan (PDF) + analisa (DOCX)**.

---

## 0. Keputusan yang sudah dikunci

| # | Pertanyaan | Jawaban |
|---|---|---|
| 1 | Laporan besar nempel ke modul? | **Tidak.** Entitas terpisah, level praktikum. |
| 2 | Cakupan kelas & deadline | **Per kelas, deadline BEDA**, dipilih asisten/admin. |
| 3 | Berapa banyak? | **Satu** laporan final; judul "Laporan Besar". |
| 4 | Kapan dibuka? | Terserah admin; praktiknya setelah Modul 6. |
| 5 | Hubungan ke ujian praktik | **Prasyarat / gerbang.** Belum kumpul → diblokir masuk ujian praktik. |
| 6 | Buka blokir | **Bisa**, wajib isi **alasan** + tercatat **nama asisten/admin** penginput. |
| 7 | Mahasiswa TE (kumpul via MS Teams) | **Centang manual** oleh asisten/admin (tidak upload web). |
| 8 | Upload berkas besar | **Signed URL Supabase** — langsung dari browser, backend hanya memberi izin. |

**Dua jalur pengumpulan, satu status akhir:**
- **Jalur web** — mahasiswa unggah laporan PDF + analisa DOCX → otomatis `terkumpul`.
- **Jalur manual** — asisten/admin centang "sudah kumpul (via Teams)" → `terkumpul`.

---

## 1. Konsep

```
Praktikum (semester) ─── alur yang SUDAH ada ───
├── Modul 1 (pre/post/keterampilan)
├── Modul 2 … Modul 6
└── Ujian Praktik  (aktivasi_course + token)

           ▼ ENTITAS BARU, TERPISAH ▼

Laporan Besar
├── per-kelas: tiap kelas punya window sendiri (buka → deadline → [telat] → tutup)
├── pengumpulan per mahasiswa: lewat web (unggah) ATAU centang manual (Teams)
└── GERBANG: status pengumpulan menentukan boleh-tidaknya masuk Ujian Praktik
```

### Aturan waktu (per kelas — sumber kebenaran = server)

```
now <  buka_at                          -> BELUM DIBUKA       (unggah ditolak)
buka_at <= now <  deadline_at           -> SEDANG DIKUMPUL    (normal)
deadline_at <= now <= late_until        -> MASIH BISA (TELAT) (badge telat)
now >  late_until  atau tutup_at        -> DITUTUP            (unggah ditolak)
late_until = NULL                       -> lewat deadline = langsung DITUTUP
```

Semua keputusan waktu dihitung **di backend**, bukan countdown frontend.

---

## 2. Skema Database (migration `031`, `032`, `033`)

### `031_create_laporan_besar.up.sql`

```sql
CREATE TABLE IF NOT EXISTS laporan_besar (
    id            SERIAL PRIMARY KEY,
    judul         VARCHAR(200) NOT NULL DEFAULT 'Laporan Besar',
    deskripsi     TEXT,
    is_aktif      BOOLEAN      NOT NULL DEFAULT TRUE,
    butuh_laporan BOOLEAN      NOT NULL DEFAULT TRUE,   -- PDF
    butuh_analisa BOOLEAN      NOT NULL DEFAULT TRUE,   -- DOCX
    created_by    INT          NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NULL,
    CONSTRAINT fk_lapbes_creator FOREIGN KEY (created_by)
        REFERENCES users(id) ON DELETE SET NULL
);
```

### `032_create_laporan_besar_target.up.sql` — window per kelas

```sql
CREATE TABLE IF NOT EXISTS laporan_besar_target (
    id               SERIAL PRIMARY KEY,
    laporan_besar_id INT       NOT NULL,
    kelas_id         INT       NOT NULL,
    buka_at          TIMESTAMP NULL,      -- NULL = langsung dibuka
    deadline_at      TIMESTAMP NOT NULL,  -- batas normal
    late_until       TIMESTAMP NULL,      -- batas telat; NULL = tanpa window telat
    CONSTRAINT uq_lapbes_target UNIQUE (laporan_besar_id, kelas_id),
    CONSTRAINT fk_lbt_lapbes FOREIGN KEY (laporan_besar_id)
        REFERENCES laporan_besar(id) ON DELETE CASCADE,
    CONSTRAINT fk_lbt_kelas FOREIGN KEY (kelas_id)
        REFERENCES kelas(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_lbt_kelas ON laporan_besar_target(kelas_id);
```

### `033_create_pengumpulan_laporan_besar.up.sql`

```sql
CREATE TABLE IF NOT EXISTS pengumpulan_laporan_besar (
    id               SERIAL PRIMARY KEY,
    laporan_besar_id INT          NOT NULL,
    mahasiswa_id     INT          NOT NULL,

    -- status akhir
    status           VARCHAR(20)  NOT NULL DEFAULT 'belum',  -- belum|terkumpul
    sumber           VARCHAR(10)  NULL,                      -- web|manual (NULL saat belum)

    -- berkas (jalur web)
    laporan_url      VARCHAR(500) NULL,
    laporan_path     VARCHAR(300) NULL,
    analisa_url      VARCHAR(500) NULL,
    analisa_path     VARCHAR(300) NULL,

    -- jejak pengumpulan web
    submitted_at     TIMESTAMP    NULL,
    is_late          BOOLEAN      NOT NULL DEFAULT FALSE,

    -- jejak centang manual (Teams)
    manual_oleh      INT          NULL,     -- asisten/admin yang mencentang
    manual_pada      TIMESTAMP    NULL,
    manual_catatan   VARCHAR(300) NULL,

    -- buka blokir gerbang ujian (override)
    override_oleh    INT          NULL,
    override_alasan  VARCHAR(500) NULL,
    override_pada    TIMESTAMP    NULL,

    CONSTRAINT uq_plb UNIQUE (laporan_besar_id, mahasiswa_id),
    CONSTRAINT fk_plb_lapbes FOREIGN KEY (laporan_besar_id)
        REFERENCES laporan_besar(id) ON DELETE CASCADE,
    -- ON DELETE RESTRICT: konsisten catatan data-integrity (jejak nilai tak hilang diam-diam)
    CONSTRAINT fk_plb_mhs FOREIGN KEY (mahasiswa_id)
        REFERENCES users(id) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plb_mhs ON pengumpulan_laporan_besar(mahasiswa_id);
```

> `.down.sql`: `DROP TABLE IF EXISTS ...;` (urutan terbalik: `033`, `032`, `031`).

Baris `pengumpulan_laporan_besar` dibuat **lazily** (upsert saat mahasiswa unggah,
saat asisten mencentang, atau saat gerbang mengecek). Tidak ada baris = dianggap
`belum` (sumber NULL).

---

## 3. Alur Unggahan Berkas Besar (signed URL Supabase)

Masalah: laporan PDF bisa 20–50 MB. Backend jalan di **Vercel serverless** —
body request dibatasi ~4.5 MB, dan handler upload lama membaca seluruh file ke
RAM. Jadi file **tidak lewat backend**.

```
1. Frontend minta izin   -> POST /api/praktikum/laporan-besar/upload-url
                            body: { jenis: "laporan" | "analisa", ext }
2. Backend cek:
   - login + role mahasiswa
   - cari target kelas mahasiswa ini -> hitung status waktu
   - window harus membolehkan (buka_at <= now <= late_until/deadline)
   - jenis valid; path dibangun server: lapbes/{id}/{mhs_id}/{jenis}-{ts}.{ext}
   - sign URL Supabase (service key, TTL pendek ~10 menit)
3. Balikan { upload_url, path, public_url }
4. Browser PUT file langsung ke upload_url   (tidak lewat backend)
5. Frontend konfirmasi   -> POST /api/praktikum/laporan-besar/kumpul
                            body: { laporan_path, analisa_path }
6. Backend:
   - CEK ULANG waktu (jangan percaya langkah 2 saja)
   - upsert baris: status=terkumpul, sumber=web, submitted_at, is_late
```

Tambah di `pkg/supabase`:

```go
// SignedUploadURL membuat URL unggah bertanda tangan (berlaku terbatas waktu).
func (c *Client) SignedUploadURL(path string, ttl time.Duration) (string, error) {
    // POST {base}/storage/v1/object/upload/sign/{bucket}/{path}
    // Header: Authorization: Bearer <serviceKey>
    // Respons: { "url": "/storage/v1/object/upload/sign/<bucket>/<path>?token=..." }
    // Kembalikan URL absolut (base + url).
}
```

> Kenapa bukan `@supabase/supabase-js` + anon key + RLS? Signed URL lebih terkurung:
> backend tetap satu-satunya pemberi izin dan penegak deadline. Anon key tidak
> tersebar di frontend.

---

## 3b. Akses Berkas: Privat, Preview, Download

**Keputusan: berkas PRIVAT, di bucket TERPISAH.** Praktikan lain tidak boleh
melihat berkas siapa pun. Karena itu:

- **Bucket baru khusus `laporan-besar`**, bersifat privat (tanpa URL publik).
  Bucket lama tetap apa adanya — foto asisten, gambar soal, PDF modul **tidak
  tersentuh** (risiko paling kecil).
- Semua baca laporan lewat **URL bertanda tangan ber-TTL pendek** yang diterbitkan
  backend setelah cek hak akses.

> Konfigurasi: tambah env `SUPABASE_BUCKET_LAPORAN` (default `laporan-besar`) di
> `config.go` + `.env.example`. `pkg/supabase.New(...)` dipakai untuk bucket lama;
> buat instance kedua (bucket laporan) atau perluas `Client` agar multi-bucket.

> Catatan: `PublicURL()` tidak dipakai untuk jalur ini. Unggah pakai
> `SignedUploadURL`; baca pakai `SignedDownloadURL`.

### Matriks hak akses

| Peran | Lihat milik sendiri | Lihat milik orang lain | Unduh banyak (bulk) |
|---|---|---|---|
| Mahasiswa | ✅ (hanya dirinya) | ❌ | ❌ |
| Asisten | ✅ | ✅ (kelas ampuannya / semua kelas) | ✅ |
| Koordinator/Admin | ✅ | ✅ (semua) | ✅ |

Backend **selalu** memverifikasi: mahasiswa hanya boleh minta URL untuk
`mahasiswa_id == dirinya`. Asisten/admin boleh untuk siapa pun.

### Tambah di `pkg/supabase`

```go
// SignedDownloadURL membuat URL baca bertanda tangan (berlaku terbatas waktu).
func (c *Client) SignedDownloadURL(path string, ttl time.Duration, downloadName string) (string, error) {
    // POST {base}/storage/v1/object/sign/{bucket}/{path}
    // Body JSON: { "expiresIn": <detik> }
    // Header: Authorization: Bearer <serviceKey>
    // Respons: { "signedURL": "/storage/v1/object/sign/<bucket>/<path>?token=..." }
    // downloadName (opsional) -> tambahkan ?download=<nama> agar Content-Disposition attachment.
    // Kembalikan URL absolut (base + signedURL).
}
```

### Tiga mode akses (semua via endpoint backend → redirect ke signed URL)

```
1) PREVIEW (tanpa unduh)
   GET /api/admin/laporan-besar/:id/pengumpulan/:mhsId/preview?jenis=laporan
   -> cek hak akses -> 302 redirect ke SignedDownloadURL (TTL ~5 menit, tanpa ?download)
   - PDF  : dirender browser/inline (iframe / pdf.js) langsung dari Supabase.
   - DOCX : TIDAK bisa dirender native browser -> dirender CLIENT-SIDE memakai
            pustaka (docx-preview / mammoth.js): fetch file dari signed URL,
            lalu render ke DOM. Tidak ada konversi di server.

2) DOWNLOAD SATU BERKAS
   GET /api/admin/laporan-besar/:id/pengumpulan/:mhsId/download?jenis=analisa
   -> cek hak akses -> 302 redirect ke SignedDownloadURL dengan ?download=<nama asli>
   File mengalir langsung Supabase -> browser, TIDAK melewati backend.

3) DOWNLOAD BANYAK (bulk)
   POST /api/admin/laporan-besar/:id/download-batch
   body: { items: [{ mhsId, jenis }...] }   -- satu-satu, beberapa, atau semua
```

**⚠️ Bulk: kenapa TIDAK dibungkus ZIP di backend.**
Kumpulan puluhan berkas 20 MB = ratusan MB. Backend Vercel dibatasi ukuran body
maupun response (~4.5 MB) — ZIP di server **tidak andal**. Karena itu bulk
diselesaikan **di sisi browser**:

- Backend `download-batch` mengembalikan **daftar signed URL** (satu per berkas),
  bukan binernya.
- Frontend mengunduh tiap URL (paralel terbatas, mis. 3 sekaligus). Opsional:
  dikemas ZIP **di browser** (JSZip) sebelum disimpan sebagai satu berkas.
- Keuntungan: tiap berkas mengalir penuh dari Supabase → browser (backend hanya
  menerbitkan URL, tidak menyentuh byte berkas). Nol tekanan ke backend.

Alternatif bila suatu saat di hosting non-serverless (VPS): bulk bisa dibuat
streaming `archive/zip` di server. Di Vercel: **jangan**.

---

## 4. Gerbang Ujian Praktik (inti fitur)

Titik sisip: `JawabanUsecase.Mulai()` — `backend/internal/usecase/jawaban_usecase.go:112`.
Di sana token ujian sudah diperiksa; gerbang ditaruh **sebelum** logika token.

```
Mahasiswa menekan Mulai pada course ujian_praktik
        │
        ├── cari baris pengumpulan_laporan_besar (mahasiswa ini)
        │
        ├── status == terkumpul                 ─────► LOLOS
        ├── override_pada IS NOT NULL            ─────► LOLOS (buka blokir + alasan tercatat)
        └── selain itu                           ─────► 403 TOLAK
             pesan: "Laporan Besar belum dikumpulkan. Hubungi asisten."
```

Gerbang hanya aktif bila `laporan_besar.is_aktif = true` **dan** mahasiswa kelasnya
memang punya baris di `laporan_besar_target` (kelas yang tidak di-target tidak
kena gerbang).

**Blokir & buka blokir (override):**
- Default: **blokir penuh**.
- Admin/asisten dapat membuka blokir per mahasiswa: `PUT` override dengan
  `alasan` (wajib) → tercatat `override_oleh` + `override_alasan` + `override_pada`.
- Override **tidak** mengubah `status` pengumpulan (tetap `belum`); ia hanya
  melewati gerbang. Jadi jelas terbedakan: "kumpul benar-benar" vs "dibolehkan darurat".
- Override bisa dicabut (set NULL) oleh admin.

---

## 5. Struktur Berkas & Layer (Clean Architecture)

Mengikuti pola `pedoman`: entity → dto → repository → usecase → handler → route → app.go.

| Layer | File baru |
|---|---|
| entity | `internal/entity/laporan_besar.go` (3 struct) |
| dto | `internal/dto/laporan_besar_dto.go` |
| repository | `internal/repository/laporan_besar_repository.go` |
| usecase | `internal/usecase/laporan_besar_usecase.go` |
| handler | `internal/delivery/http/handler/laporan_besar_handler.go` |
| route | tambah di `router.go` (grup `prak` + grup `admin`) |
| wiring | tambah di `internal/app/app.go` |
| supabase | tambah `SignedUploadURL` di `pkg/supabase/supabase.go` |
| gerbang | sisip di `internal/usecase/jawaban_usecase.go` (`Mulai`) |

`JawabanUsecase` perlu akses repo laporan besar → tambahkan dependensi baru
`laporanBesarRepo` di `NewJawabanUsecase(...)` dan di `app.go`.

---

## 6. Endpoint

### Mahasiswa (`/api/praktikum/laporan-besar/*` — auth + role mahasiswa)

| Method | Path | Fungsi |
|---|---|---|
| GET | `/laporan-besar` | status laporan saya + window kelas saya + sisa waktu |
| POST | `/laporan-besar/upload-url` | minta signed URL (body: `jenis`, `ext`) |
| POST | `/laporan-besar/kumpul` | finalisasi: simpan path, hitung telat |
| DELETE | `/laporan-besar/kumpul` | batalkan (hanya jika belum diverifikasi & belum ditutup) |

### Admin (`/api/admin/laporan-besar/*` — auth admin + `RequirePermission("sesi","write")`)

| Method | Path | Fungsi |
|---|---|---|
| GET | `/laporan-besar` | detail laporan besar + daftar target kelas |
| POST | `/laporan-besar` | buat laporan besar |
| PUT | `/laporan-besar/:id` | ubah laporan besar |
| PUT | `/laporan-besar/:id/target` | set/ubah window per kelas (buka/deadline/telat) |
| GET | `/laporan-besar/:id/pengumpulan` | daftar mahasiswa target + status + jejak |
| PUT | `/laporan-besar/:id/pengumpulan/:mhsId/mark` | **centang manual** (Teams): terkumpul/belum + catatan |
| PUT | `/laporan-besar/:id/pengumpulan/:mhsId/override` | **buka blokir** gerbang (alasan wajib) |
| DELETE | `/laporan-besar/:id/pengumpulan/:mhsId/override` | cabut buka blokir |
| GET | `/laporan-besar/:id/pengumpulan/:mhsId/preview` | **preview** berkas (302 → signed URL, inline) |
| GET | `/laporan-besar/:id/pengumpulan/:mhsId/download` | **unduh** satu berkas (302 → signed URL, attachment) |
| POST | `/laporan-besar/:id/download-batch` | **unduh banyak** → daftar signed URL (satu/beberapa/semua) |

---

## 7. Frontend

### Admin — `src/routes/praktikum/admin/laporan-besar/+page.svelte`
*(di bawah `admin/` agar kena guard role admin/koordinator)*

- Form laporan besar: judul (default "Laporan Besar"), deskripsi, aktif.
- Tabel **target kelas**: pilih kelas → isi `buka_at`, `deadline_at`, `late_until`.
- Tabel **pengumpulan**: mahasiswa, kelas, status, sumber (web/manual), waktu,
  tombol:
  - **Centang manual** (Teams) → `manual_oleh` otomatis si asisten.
  - **Buka blokir** → wajib isi alasan.
  - **Preview** (modal inline: PDF via iframe, DOCX via `docx-preview`).
  - **Unduh** satu berkas; **Unduh terpilih** (checkbox → bulk).
- Bulk: tombol "Unduh terpilih" mengirim daftar item → backend balikan signed URL →
  browser mengunduh (opsional ZIP di browser via JSZip).
- Panel menampilkan siapa masih `belum` + badge "override" bila diblokir-dibuka.

### Mahasiswa — `src/routes/praktikum/laporan-besar/+page.svelte`

- Kartu Laporan Besar + countdown (`Countdown.svelte` yang ada) berlabel
  Buka / Deadline / Batas Telat.
- Dua slot unggah: laporan PDF, analisa DOCX → unggah langsung ke Supabase.
- Tombol "Kumpulkan", badge status: `Belum` / `Terkumpul` / **`Telat`** /
  `Diblokir (belum kumpul)`.
- Bila kelasnya bukan target → halaman bilang laporan tidak berlaku.
- Bila TE (murni manual): halaman memberi tahu "pengumpulan diverifikasi asisten
  via MS Teams" dan menonaktifkan slot unggah.

### `api.ts`
Endpoint JSON lewat `api`; PUT berkas langsung ke Supabase (bukan `api`).
CSP frontend sudah mencakup `https://*.supabase.co` di `hooks.server.ts` &
`vercel.json` — tidak perlu perubahan.

---

## 8. Validasi & Edge Case

- **Ekstensi/MIME**: laporan wajib `.pdf`; analisa wajib `.docx`. ⚠️
  `http.DetectContentType` **tidak** mengenali DOCX (ZIP) — karena unggahan lewat
  signed URL, backend tidak melihat isi; andalkan ekstensi + `content-type` saat
  sign. Bila ingin ketat, validasi isi dilakukan belakangan (job pemeriksa).
- **Kadaluarsa di tengah unggah**: URL ditandatangani saat window masih buka;
  backend **cek ulang di langkah `kumpul`**. Bila sudah ditutup → tolak, minta hapus.
- **Path traversal**: `path` dibangun server dari id (integer); tidak ada input bebas.
- **Upsert**: satu mahasiswa satu baris per laporan (`uq_plb`). Kumpul ulang = replace.
- **Berkas yatim**: signed URL dipakai tetapi `kumpul` tak pernah dipanggil → file
  tanpa rujukan. Opsional: cron pembersih path tak dirujuk, umur > 24 jam.
- **Kelas mahasiswa berubah** setelah target dibuat: gerbang membaca target kelas
  **saat ini**. Bila mahasiswa pindah kelas tak di-target, gerbang tidak berlaku
  (tidak adil) — mitigasi: admin dapat menambah target kelas tersebut.
- **Override tanpa baris**: mahasiswa belum pernah punya baris → override membuat
  baris dulu (upsert) lalu mengisi `override_*`.
- **Berkas PRIVAT, bucket terpisah (dikunci).** Bucket baru `laporan-besar` bersifat
  non-publik; bucket lama tidak diubah. Semua baca lewat `SignedDownloadURL`
  ber-TTL pendek setelah cek hak akses. Mahasiswa lain tidak bisa melihat walaupun
  punya tautan (URL kedaluwarsa cepat + butuh token).

---

## 9. Keputusan yang Masih Terbuka

- Laporan besar **tidak dinilai** — hanya status kumpul/belum (dikunci).

---

## 10. Urutan Implementasi (bertahap, review per tahap)

1. **Migrasi 031–033 + entity + repository** (tanpa endpoint).
2. **`pkg/supabase.SignedUploadURL`** + endpoint `upload-url` + `kumpul`.
3. **Usecase + handler + route** laporan besar (admin CRUD/target/mark/override + mahasiswa lihat).
4. **Gerbang di `JawabanUsecase.Mulai`** + wiring repo di `app.go`.
5. **Frontend admin** (buat laporan, target kelas, centang manual, buka blokir).
6. **Frontend mahasiswa** (unggah + countdown + status).
7. **`make swag`**, lalu `go build ./... && go test ./...` dan `bun run check`.

---

## 11. Ringkasan Dampak

| Area | Dampak |
|---|---|
| DB | +3 tabel, migration 031/032/033 |
| Backend | +1 modul (entity/dto/repo/usecase/handler), +2 method supabase (`SignedUploadURL`, `SignedDownloadURL`), +gerbang di `Mulai`, +3 endpoint akses berkas, route baru |
| Storage | bucket **privat** (disarankan bucket terpisah `laporan-besar`) |
| Frontend | +2 halaman (admin & mahasiswa), countdown, unggah langsung Supabase, preview (PDF iframe / DOCX `docx-preview`), unduh satu & bulk |
| Risiko utama | limit body/response Vercel (diatasi signed URL + bulk di sisi browser); DOCX tak terdeteksi `DetectContentType` |
| Tidak disentuh | jalur ujian (`pengerjaan_course`, `aktivasi_*`) selain satu sisipan gerbang di `Mulai` |
