# Web Lab-AP v3 — Platform Praktikum Algoritma & Pemrograman

Platform web untuk mengelola praktikum Laboratorium Algoritma & Pemrograman: ujian
aman (pre-test essay, post-test hybrid, keterampilan live-coding, ujian praktik),
eksekusi kode langsung, penilaian massal berbantuan LLM, dan dashboard admin.

> Domain & komentar kode berbahasa Indonesia — pertahankan konvensi ini saat mengedit.

- **Production**: https://algohub.web.id
- **API docs (Swagger UI)**: `<backend>/swagger/index.html`
- **Spesifikasi lengkap**: [`updateAndPRDERD/PRD _AND_ERD.md`](updateAndPRDERD/PRD%20_AND_ERD.md)

---

## Daftar Isi

1. [Ringkasan Fitur](#ringkasan-fitur)
2. [Tech Stack](#tech-stack)
3. [Arsitektur](#arsitektur)
4. [Model Data (ERD)](#model-data-erd)
5. [Menjalankan Secara Lokal](#menjalankan-secara-lokal)
6. [Variabel Environment](#variabel-environment)
7. [Deployment](#deployment)
8. [AI Grading](#ai-grading)
9. [Eksekusi & Kompilasi Kode](#eksekusi--kompilasi-kode)
10. [Migrasi Database](#migrasi-database)
11. [Dokumentasi API (OpenAPI)](#dokumentasi-api-openapi)
12. [Riwayat Perubahan (update*.md)](#riwayat-perubahan)

---

## Ringkasan Fitur

### A. Web Dashboard Publik (`/info`) — tanpa login
- **Lobby** — sambutan, pengumuman, quick links.
- **Jadwal** (`/info/jadwal`) — tabel internal read-only **atau** tautan Google Drive (dipilih admin).
- **Daftar Asisten** (`/info/asisten`) — profil asisten dinamis dari DB (foto, NIM, WA, medsos).
- **Pedoman Laporan** (`/info/laporan`) — tombol download file dinamis (1..n).
- **Modul Praktikum** (`/info/modul`) — 1 file PDF modul global.

### B. Alur Mahasiswa (`/praktikum`) — perlu login
- **First-time login berbasis NIM**: data mahasiswa (NIM, nama, kelas, shift) sudah
  di-seed admin; mahasiswa set password saat register **jika** admin membuka akses
  register untuk kelasnya (`is_register_open`). Mahasiswa tidak bisa login sebelum register.
- **Dashboard user** — profil, nilai per sesi, jadwal pribadi (per kelas+shift),
  notifikasi sesi aktif.
- **Pengerjaan course** dalam sebuah sesi:
  | Course | Soal | Jenis | Editor |
  | --- | --- | --- | --- |
  | Pre-test | 5 | Essay/isian (1 easy, 2 medium, 2 hard) | — |
  | Post-test | 3 | 2 essay + 1 coding | Monaco |
  | Keterampilan | 1 | Live coding | Monaco |
  | Ujian Praktik | 6 (1 per kategori) | Live coding | Monaco |
- **Token/PIN** wajib dimasukkan untuk memulai sesi (memastikan kehadiran fisik).
- **Auto-save** berkala + **auto-submit** saat timer habis **atau** admin menutup course.
- **Timer server-authoritative**: deadline dihitung dari `waktu_mulai + durasi_menit` di
  server; refresh browser tidak me-reset timer. Ujian Praktik hanya muncul di sesi terakhir.

### C. Fungsionalitas Admin (`/praktikum/admin`) — role `admin` (= asisten lab)
- **Dashboard** — user online real-time (registry in-memory di server stateful, bukan dari
  `last_login_at`), statistik per kelas/shift, progress pengerjaan, quick actions.
- **Manajemen User** — CRUD mahasiswa, reset password, buka/tutup register per kelas,
  **bulk import CSV** dengan laporan baris bermasalah.
- **Manajemen Asisten** — CRUD data asisten (langsung `users` role admin; tampil di `/info/asisten`).
- **Jadwal & Sesi** — konfigurasi jadwal (GDrive/internal), CRUD sesi & course, pool soal
  (WYSIWYG Edra: tabel, format, KaTeX; kolom `kunci_jawaban` sebagai acuan AI grading).
- **Pedoman & Modul** — upload file ke Supabase Storage.
- **Manajemen Sesi Praktikum**:
  - **Aktivasi per kelas+shift** dengan token acak (akses **terisolasi** per kombinasi).
  - **Gacha pre-test/post-test** eksplisit saat aktivasi (hanya salah satu dibuatkan barisnya).
  - **Buka/tutup course independen per aktivasi** (`aktivasi_course.is_open`, urutan diatur admin).
  - **Tutup course = auto-submit massal** bagi yang belum submit.
  - **Mahasiswa susulan** (`peserta_susulan`) — akses menumpang ke kelas/shift lain.
  - **Pengacakan soal dari pool** per sesi (distribusi difficulty / 1 per kategori ujian);
    semua mahasiswa dalam sesi sama mendapat soal sama.
- **Penilaian & Rekap** — nilai manual per soal (0..poin), rekap global, pivot nilai akhir,
  export Excel/CSV, bulk actions (reset/hapus), dan **AI Grading** (lihat di bawah).

### Aturan Skor
- Tiap soal punya **poin** (skor maksimal); `nilai` admin berada di rentang 0..poin.
- `total_nilai` course = Σ nilai seluruh soal (di-recalc tiap update). Disarankan Σ poin = 100.

### Keamanan
- RBAC di backend (`RequireRole`) untuk semua endpoint admin sensitif.
- Guard route berbasis layout di frontend (`praktikum/admin/+layout.svelte`). **Halaman admin
  WAJIB di bawah `praktikum/admin/`** — kalau tidak, role guard frontend terlewat diam-diam
  (backend tetap enforce, tapi jangan bergantung pada itu saja).
- Password Firebase scrypt (hasil migrasi) + bcrypt; anti-SQL-injection via GORM.
- Soft delete pada tabel krusial; FK `ON DELETE RESTRICT` pada `pengerjaan_course`.

---

## Tech Stack

| Lapisan | Teknologi |
| --- | --- |
| Frontend | SvelteKit (Svelte 5 runes), TypeScript, Tailwind, Monaco, TipTap+KaTeX; deploy Vercel |
| Backend | Go, Gin, GORM, PostgreSQL, JWT |
| Storage | Supabase Storage (PDF modul, pedoman, foto, gambar flowchart) |
| AI Grading | Endpoint OpenAI-compatible (`pkg/ollama`) — lihat [AI Grading](#ai-grading) |
| Run code | glot.io (remote) + clang→wasm (`pkg/cwasm`) |
| API docs | OpenAPI/Swagger via `swaggo/swag` |

---

## Arsitektur

### Backend — Clean Architecture (`backend/`)

Alur request (top-down):

```
route → handler → usecase → repository → GORM/Postgres
                     │
      entity (model DB) · dto (bentuk req/resp) · pkg/ (klien infra)
```

- **Composition root**: [`internal/app/app.go`](backend/internal/app/app.go) — `Build(cfg)`
  merakit semua repo → usecase → handler → router. Titik tunggal untuk mendaftarkan fitur baru.
- **Dua entrypoint berbagi engine yang sama**:
  - [`cmd/server/main.go`](backend/cmd/server/main.go) — server lokal **stateful**;
    menjalankan **goroutine sweeper auto-submit** (`SWEEPER_INTERVAL_SECONDS`) + graceful shutdown.
  - [`api/index.go`](backend/api/index.go) — entrypoint **serverless Vercel**; tanpa goroutine,
    auto-submit dijalankan lewat `POST /api/cron/auto-submit` + cron eksternal (dijaga `CronSecret`).
- **`cmd/` lain**: `cmd/migrate` (migrator SQL kustom), `cmd/regrade` (re-grading AI massal
  offline, worker pool, resumable), `database/seed`.
- **`pkg/` klien infra**: `jwt`, `hash` (Firebase scrypt), `supabase`, `ollama` (klien LLM +
  rubrik grading), `glot` (run code), `cwasm` (clang→wasm), `response` (envelope HTTP).
- **Envelope response**: semua handler membalas `{success, message, data, error}` via `pkg/response`.
  Frontend membuka `.data` dan menganggap `success:false`/non-2xx sebagai error.
- **Enum domain**: [`internal/entity/enums.go`](backend/internal/entity/enums.go) sumber kebenaran
  role, jenis course, jenis soal, kategori ujian, status pengerjaan.

**Peran tiap layer**: `entity` (domain model) ← `dto` (bentuk req/resp) ← `repository`
(satu-satunya penyentuh DB) ← `usecase` (aturan bisnis, tanpa HTTP/GORM) ← `delivery`
(handler/middleware/route). Usecase hanya memanggil *interface* repository sehingga bisa di-unit-test dengan mock.

### Frontend (`frontend/`)

- **API layer**: [`src/lib/api.ts`](frontend/src/lib/api.ts) — wrapper `api.get/post/...`
  menambah JWT dari `localStorage`, membuka envelope `{success,data}`, redirect ke
  `/praktikum/login` saat 401. Base URL dari `PUBLIC_API_BASE_URL`.
- **Auth store**: [`src/lib/stores/auth.ts`](frontend/src/lib/stores/auth.ts) (`user`, `hasToken()`).
- **Route guard berbasis layout**: `praktikum/+layout.svelte` (butuh token) dan
  `praktikum/admin/+layout.svelte` (butuh `role==='admin'`).
- **Editor**: `CodeEditor.svelte` (Monaco), `RichTextEditor.svelte` + `edra/` (TipTap+KaTeX).
- **`Countdown.svelte`** menampilkan timer; server tetap otoritatif atas expiry.

---

## Model Data (ERD)

Skema lengkap (DBML) ada di [`PRD _AND_ERD.md`](updateAndPRDERD/PRD%20_AND_ERD.md#erd-).
Tabel inti:

| Tabel | Fungsi |
| --- | --- |
| `users` | Mahasiswa (`user`) & asisten (`admin`); NIM = username |
| `kelas`, `jadwal` | Kelas + jadwal per kelas+shift |
| `konfigurasi` | Key-value global (GDrive URL, modul URL, `ai_model`, dll) |
| `pedoman_laporan` | File pedoman dinamis |
| `sesi_praktikum`, `course` | Modul & course-nya (pretest/posttest/keterampilan/ujian_praktik) |
| `soal` | Pool soal (jenis, difficulty, kategori_ujian, gambar flowchart, poin, kunci) |
| `aktivasi_sesi`, `aktivasi_course` | Aktivasi per kelas+shift + status buka/tutup course |
| `peserta_susulan` | Akses susulan lintas kelas/shift |
| `soal_terpilih` | Hasil acak soal per aktivasi |
| `jawaban_mahasiswa` | Jawaban per soal (auto-save, nilai, feedback) |
| `pengerjaan_course` | Tracking status + `total_nilai` (cached Σ nilai) |

> `pengerjaan_course` memakai FK `ON DELETE RESTRICT` — menghapus induk ditolak selama masih
> ada nilai yang merujuknya. Jangan dilonggarkan tanpa memahami risiko kehilangan data.

---

## Menjalankan Secara Lokal

**Prasyarat**: Go 1.21+, Node 18+, PostgreSQL, (opsional) akun Supabase.

```bash
cp .env.example backend/.env      # lalu isi kredensial
make migrate-up                   # jalankan migrasi
make seed                         # seed admin, kelas, jadwal, contoh soal
make run                          # backend di :8080
make fe-install && make fe-dev    # frontend di :5173
```

Perintah lain (dari root repo):

| Perintah | Fungsi |
| --- | --- |
| `make build` | Compile backend ke `backend/bin/server` |
| `make test` | `go test -v ./...` (backend). Single: `cd backend && go test ./internal/usecase -run TestName` |
| `make migrate-up` / `migrate-down` | Terapkan / rollback 1 migrasi |
| `make migrate-drop` | Rollback semua migrasi |
| `make seed` | Seed data awal |
| `make swag` | Regenerate OpenAPI (wajib setelah ubah anotasi handler) |
| `make mock` | Regenerate mock mockery ke `internal/repository/mocks` |
| `make tidy` | `go mod tidy` |
| `make fe-build` | Build frontend |

> ### ⚠️ KITA TIDAK PAKAI npm — PAKAI bun
>
> Package manager frontend proyek ini **bun**. Lockfile resmi: `frontend/bun.lock`.
>
> ```bash
> cd frontend && bun install      # bukan npm install
> cd frontend && bun run dev      # bukan npm run dev
> ```
>
> Jangan jalankan `npm install`: dia bikin `package-lock.json` liar (±7 ribu baris) yang
> menduplikasi resolusi dependensi dan bentrok dengan `bun.lock`. Kalau file itu muncul,
> hapus lalu `bun install` ulang:
>
> ```bash
> cd frontend && rm -f package-lock.json && rm -rf node_modules && bun install
> ```
>
> `make fe-install` / `fe-dev` / `fe-build` / `fe-check` sudah memakai bun.

**Cek sebelum push:**
```bash
cd backend && go build ./... && go test ./...
cd frontend && bun install && bun run check && bun run build
```
> Catatan: `.github/workflows/ci.yml` **belum ada** di repo ini, jadi cek di atas dijalankan manual.

---

## Variabel Environment

Backend (`backend/.env`, lihat [`.env.example`](.env.example)):

| Var | Fungsi |
| --- | --- |
| `PORT` / `APP_PORT` | Port server (default 8080) |
| `APP_ENV` | `development` / `production` |
| `CORS_ORIGINS` | Daftar origin (CSV) |
| `DB_HOST/PORT/USER/PASSWORD/NAME/PARAMS` | Koneksi PostgreSQL |
| `JWT_SECRET`, `JWT_EXPIRE_HOURS` | Token auth |
| `CRON_SECRET` | Guard endpoint cron auto-submit (serverless) |
| `ONLINE_TTL_SECONDS` | TTL entri registry online |
| `SWEEPER_INTERVAL_SECONDS` | Interval sweeper auto-submit (server stateful) |
| `SUPABASE_URL/SERVICE_KEY/BUCKET` | Storage file |
| `OLLAMA_URL` | Endpoint AI grading (OpenAI-compatible `/v1/chat/completions`) |
| `OLLAMA_MODEL` | Model AI default |
| `OLLAMA_API_KEY` | Bearer key provider AI |
| `GLOT_URL`, `GLOT_TOKEN` | Run code remote |
| `CWASM_CLANG`, `WASI_SYSROOT` | Kompilasi C→wasm (kosong = nonaktif) |
| `FB_SCRYPT_*` | Parameter verifikasi password Firebase lama |

Frontend (`frontend/.env`): `PUBLIC_API_BASE_URL` (dipakai via `$env/static/public`).

---

## Deployment

- **Frontend**: Vercel (SvelteKit). Set `PUBLIC_API_BASE_URL`.
- **Backend serverless (Vercel)**: entrypoint `api/index.go`. Auto-submit **tidak** pakai
  goroutine — daftarkan cron eksternal (cron-job.org) ke `POST /api/cron/auto-submit` dengan
  secret sesuai `CRON_SECRET`.
- **Backend stateful (VM/container)**: `make build` → binary menjalankan sweeper sendiri.
- **Env di Vercel**: `.env` **tidak** ter-commit (gitignored). Set semua var (khususnya
  `OLLAMA_API_KEY`) di dashboard Vercel → Settings → Environment Variables, lalu redeploy.

Detail tambahan: [`updateAndPRDERD/deploy.md`](updateAndPRDERD/deploy.md).

---

## AI Grading

Penilaian otomatis essay & coding memakai LLM lewat endpoint **OpenAI-compatible**
(`POST {OLLAMA_URL}`, mis. `/v1/chat/completions`). Klien: [`backend/pkg/ollama/`](backend/pkg/ollama/).

- **Provider aktif**: konfigurasi via env (`OLLAMA_URL`, `OLLAMA_MODEL`, `OLLAMA_API_KEY`).
  Override model runtime lewat konfigurasi `ai_model` (tampil di panel Penilaian; endpoint
  `GET /api/admin/konfigurasi/ai-model` mengembalikan `{effective, override, default}`).
- **Rubrik**: [`pkg/ollama/rubric.go`](backend/pkg/ollama/rubric.go) — esai diklasifikasi ke
  kategori berbobot (`benar` / `benar_penjelasan` / `salah_penjelasan` / `salah` / `kosong`);
  coding dinilai 3 sub-kriteria (sesuai petunjuk, berjalan tanpa error, tepat waktu). Nilai AI
  diskalakan ke poin soal dan di-clamp 0..poin.
- **Bulk grading**: panel Penilaian bisa menilai banyak jawaban **atau lintas sesi & kelas**
  sekaligus. Grading berjalan sinkron per-jawaban dari frontend; request diluncurkan **berjarak
  ~1.1 detik** (overlap/paralel) agar patuh limit provider **60 req/menit** apa pun latency-nya.
  Progress, stop, dan retry-yang-gagal tersedia di UI.
- **Regrade offline**: [`cmd/regrade`](backend/cmd/regrade/) untuk re-grading massal (worker pool,
  resumable) — lihat header file untuk flag.

---

## Eksekusi & Kompilasi Kode

- **Run code (remote)**: `pkg/glot` → glot.io (`GLOT_URL`, `GLOT_TOKEN`).
- **Live runner (browser)**: kompilasi C→wasm32-wasi via clang + wasi-sdk (`pkg/cwasm`,
  `CWASM_CLANG`, `WASI_SYSROOT`). Kosong = fitur nonaktif.

---

## Migrasi Database

Migrator **kustom** (bukan golang-migrate): [`cmd/migrate/main.go`](backend/cmd/migrate/main.go)
membaca `database/migration/NNN_*.up.sql` / `.down.sql` berurutan, melacak versi di tabel
`schema_migrations`, tiap file dijalankan dalam transaksi. Untuk menambah migrasi, buat pasangan
`NNN_name.up.sql` + `.down.sql` berikutnya.

> `make migrate-sync` / `make migrate-fresh` menjalankan skrip Node di `updateAndPRDERD/migration/`.
> **`migrate-fresh` destruktif** (menghapus Supabase kecuali admin `202411106` lalu re-import dari
> Firebase). Jangan dijalankan tanpa maksud eksplisit.

---

## Dokumentasi API (OpenAPI)

Spesifikasi digenerate dari anotasi handler Go (`swaggo/swag`):

- **Swagger UI**: `<backend>/swagger/index.html`
- **File**: [`backend/docs/swagger.json`](backend/docs/swagger.json),
  [`backend/docs/swagger.yaml`](backend/docs/swagger.yaml),
  [`backend/docs/openapi.yaml`](backend/docs/openapi.yaml)
- **Regenerate**: `make swag` (wajib setiap kali anotasi handler berubah).

---

## Riwayat Perubahan

Folder [`updateAndPRDERD/`](updateAndPRDERD/) berisi PRD/ERD + catatan evolusi fitur:

| File | Topik |
| --- | --- |
| `PRD _AND_ERD.md` | Spesifikasi produk + ERD (kanonik) |
| `update.md` | Rencana pengembangan awal v3 |
| `update2.md` | Bug fix & optimasi performa (tahap 2) |
| `update3.md` | Konsep automated testing |
| `update4.md` | Perbaikan UI/UX masif |
| `update5.md` | Backend serverless (pola project_mikon) |
| `update6.md` | Run code, editor lega, empty-state, anti-emoji |
| `update7.md` | Live code runner (WASM di browser) |
| `update8.md` | Revamp rasa UI (font, warna, motion) |
| `update9.md` | Optimasi kecepatan (hilangkan delay) |
| `update10.md` | Bank soal / factory soal |
| `update11.md` | Mega migration Firebase → Supabase |
| `update12.md` | Hardening project |
| `update13.md` | Migrasi jawaban per-soal (fase 2) |
| `update14.md` | Sync berkala Firebase → Supabase |
| `update15.md` | Migrasi password (Firebase Auth → Supabase) |
| `update16.md` | Keaktifan + guard nilai arsip |
| `update17.md` | Koreksi pembobotan AI grading (rubrik resmi) |
| `update18.md` | Buka kunci pengerjaan (ngerjain ulang setelah dihapus) |
| `update19.md` | Perubahan lanjutan |
| `update20.md` | Skenario masa praktikum |
| `update21.md` | Ganti provider & model AI grading (OpenAI-compatible) |

---

## Konvensi

- Bahasa Indonesia untuk komentar & istilah domain.
- Reuse konstanta enum di `entity/enums.go`, bukan string literal.
- Ikuti envelope response `pkg/response` di semua handler.
- Jalankan `make swag` setelah mengubah anotasi handler, dan cek CI sebelum push.
