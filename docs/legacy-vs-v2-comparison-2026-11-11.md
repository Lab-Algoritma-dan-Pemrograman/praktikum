# Perbandingan project AP lama vs AP v2

Tanggal: 11 November 2026. Sumber: pembacaan source kedua project dan dokumen
`docs/` v2. Tidak ada eksekusi atau deployment.

## 1. Ringkasan

| | Project lama (`E:\rekapProject\project_ap`) | v2 (`E:\rekapProject\project_ap_v2`) |
| --- | --- | --- |
| Sifat | Produksi berjalan, fitur lengkap | Rewrite bertahap, Phase 1–5, Phase 6 ditahan |
| Kekuatan utama | Fitur kaya dan sudah terbukti di lapangan | Kontrak, otorisasi, tes, dan batas keamanan jauh lebih ketat |
| Kelemahan utama | Arsitektur mahal (container monolith), banyak jalur implisit, XP dipercaya klien | Belum semua fitur terverifikasi; runner dan AI grading masih `Unconfigured` |
| Status runner | Browser + Glot, tanpa proof | Preview browser lebih aman; proof belum ada |

Kesimpulan singkat: v2 lebih baik sebagai fondasi jangka panjang, tetapi belum
menggantikan parity lama. Runner adalah satu-satunya area di mana sistem lama
lebih matang secara UX, bukan secara jaminan.

## 2. Struktur dan stack

| Aspek | Lama | v2 |
| --- | --- | --- |
| Struktur repo | Empat app terpisah + `_shared`; tidak ada workspace tooling | Monorepo npm workspaces: 6 app + 5 package + backend Go |
| E-learning | React 19 + Vite + TypeScript | SvelteKit |
| Praktikum | SvelteKit frontend + Go 1.24 (Gin, GORM) backend | SvelteKit + Go 1.26.11 (`net/http`, pgx) |
| SIAKAD | React + Vite + shadcn | SvelteKit |
| Admin | Tersebar di dalam app | App `admin` tersendiri |
| Portal | Halaman statis/terpisah | App `portal` + host runtime browser |
| Auth | JWT custom (`golang-jwt`), kompatibilitas Firebase scrypt | Session server + Supabase IdentityProvider + PKCE + SSO |
| Database | Supabase Postgres, GORM | Supabase Postgres, pgx, 7 migration eksplisit |
| API contract | Swagger yang di-generate dari handler | OpenAPI 90 operation, dicek otomatis |
| AI | Ollama router + Gemini (`@google/genai`) | Adapter `grading` dengan decoder ketat; provider nyata belum aktif |

## 3. Deployment dan biaya

| Aspek | Lama | v2 |
| --- | --- | --- |
| Backend | Container Vercel dari Dockerfile berisi clang-19 + lld + wasi-libc | Vercel Go Functions, stateless, region `sin1` |
| Frontend | Project Vercel terpisah | 6 app Vercel, staging terpisah |
| Database/storage | Supabase | Supabase |
| Cron | Vercel cron + endpoint backend | Scheduler bertanda tangan ke endpoint idempotent |
| Runner di server | Glot proxy + compile clang di container | Tidak ada; `Runner.Unconfigured` |
| Risiko biaya utama | Instance container besar, clang di image, `maxDuration` panjang | Tidak ada compute runner; batas 30 dtk platform |
| Kuota | Tidak terdokumentasi di repo | Hobby: 4 CPU-jam, 360 GB-jam, 1 juta invocation |

Kenaikan Fluid Compute di project lama paling masuk akal berasal dari kombinasi
container monolith, clang di runtime image, dan request panjang. v2 secara
desain menghindari ketiganya.

## 4. Autentikasi dan otorisasi

| Aspek | Lama | v2 |
| --- | --- | --- |
| Login | NIM/email + password, register, forgot/reset | Supabase + session server, PKCE, SSO antar-origin |
| Identitas lintas app | Token bertanda tangan antar app | Allowlist origin SSO + session terpusat |
| Otorisasi | Role + pengecekan di handler | Permission/role/division/scope + policy terpusat |
| Revocation | Logout | Logout + revoke session, diuji |
| Admin host | Bagian dari app | Host terpisah, bukan grant otorisasi |
| Celah yang ditutup v2 | — | IDOR/BOLA, spoofing role/scope, cross-app cache leakage, admin URL grant |

## 5. Data dan konsistensi

| Aspek | Lama | v2 |
| --- | --- | --- |
| Migration | Berbasis model GORM, tidak selalu eksplisit | 7 migration SQL bernomor, termasuk RLS |
| Idempotency | Sebagian | `Idempotency-Key` wajib untuk mutation, replay diuji |
| Timer ujian | Server time, ada cron auto-submit | Server/database time, lock transaksi, race submit/close diuji |
| Kuota runner | Rate limit per menit | Lease durable + kuota per user/aktivasi |
| XP | Ditulis klien ke Supabase | Guard proof server, XP exactly-once |
| Audit | Ada tabel audit di praktikum | Audit actor/action/scope/request ID di seluruh domain |

## 6. Parity fitur

| Domain | Lama | v2 | Catatan |
| --- | --- | --- | --- |
| Praktikum | Sesi, token, ruang, autosave, submit, nilai, AI grading, laporan besar | Sesi, attempt, autosave, timer, submit, close, grading manual/bulk, monitoring | AI grading dan laporan besar belum setara |
| E-learning | Level, modul, lesson, quiz, XP, streak, achievement, leaderboard, Bug Hunt, hints | Level, lesson, quiz, progress, XP, achievement, leaderboard, Bug Hunt, hints | Parity belum diverifikasi penuh |
| Code lab | C/Python, stdin, testcase, input interaktif, matplotlib | Editor + preview browser aman; runtime v3 menambahkan import paket Pyodide, input interaktif Python, dan gambar matplotlib; proof server belum ada | Parity preview v3 12 November 2026; proof tetap terpisah |
| SIAKAD | Jadwal, absensi, kelas, inventaris, penyewaan, laporan, notifikasi | Kelas, jadwal, absensi, inventaris, penyewaan, laporan, notifikasi | Cakupan setara; acceptance bertahap |
| Auth/SSO | Login, verify, receive-token | Session, PKCE, SSO, revoke | v2 lebih ketat |
| Leaderboard | HTML/PDF statis per periode | Leaderboard API + UI | Mekanisme berbeda |
| Laporan | Upload bucket privat, signed URL | Belum setara | Gap parity |
| Prestige/streak | Ada | Sebagian | Perlu verifikasi |

## 7. Code runner

| Aspek | Lama | v2 |
| --- | --- | --- |
| C | clang-wasm + lld + memfs + sysroot di Worker | Aset sama, di `apps/portal/static/runtimes/v1` |
| Python | Pyodide 0.25 dari CDN | Pyodide 0.25 dari CDN |
| Inisialisasi | Worker hidup selama app; E-learning tidak pernah terminate; praktikum terminate saat timeout/Stop | Iframe dihapus saat komponen unmount; worker Python di-terminate tiap run |
| Cache compile | Perbandingan string source terakhir, in-memory | Tidak ada; compile+link setiap run |
| Unduhan pertama | Init hanya 9,6 MB (memfs+sysroot); clang 31 MB + lld 19 MB baru di Run pertama | Sama, ditambah MIME wasm salah sehingga `compileStreaming` mati |
| Output | Praktikum streaming ke xterm tanpa cap; E-learning ditampung | Ditampung, cap 16 KiB + `truncated` |
| Input interaktif | SAB (praktikum) / rewrite source (E-learning) | Hanya stdin di awal; belum ada SAB |
| Timeout | C 40 dtk (praktikum), tidak ada di E-learning/Python | 10 dtk semua |
| Isolasi | Worker same-origin tanpa iframe | Iframe `sandbox="allow-scripts"` + CSP `default-src 'none'` |
| Fallback server | Glot.io, 25 dtk, 30/menit | Tidak ada |
| Compile server | clang-19 di container, tidak pernah dipanggil frontend | Tidak ada |
| Proof | XP ditulis klien | Guard server; runner belum tersedia |

Efisiensi: sistem lama unggul karena worker persisten dan cache compile;
v2 unggul karena batas output, isolasi, dan tidak ada biaya server.

## 8. Kualitas dan testing

| Aspek | Lama | v2 |
| --- | --- | --- |
| Go test | ±16 file, fokus usecase | 30 file, termasuk race dan PostgreSQL |
| Frontend unit | Sedikit (Countdown, beberapa SIAKAD) | 9 file unit + Playwright |
| Contract test | Tidak ada | OpenAPI 90 operation / 344 pemeriksaan |
| E2E | QA manual + dokumen audit | Chromium 55/55 (bukti sesi sebelumnya) |
| Acceptance staging | Tidak terdokumentasi terpusat | `docs/staging-acceptance-2026-11-11.md` |
| Bukti | Laporan QA | Artefak, checksum, request ID |

## 9. Keamanan

| Area | Lama | v2 |
| --- | --- | --- |
| Source mahasiswa | Browser; fallback Glot | Browser terisolasi; server tidak mengeksekusi |
| Output | Tidak dibatasi | 16 KiB per stream + JSON 128 KiB |
| Secret | Di environment Vercel | Sama; tidak ada di bundle |
| CSP | E-learning punya CSP; praktikum parsial | CSP per app |
| XP/completion | Klien | Guard proof server |
| Validasi input | Manual per handler | Validasi terpusat + decoder ketat |
| Audit | Terbatas | Actor/action/scope/request ID |

## 10. Kelebihan dan kekurangan

Project lama:

- Kelebihan: fitur lengkap, sudah dipakai, runner interaktif matang, AI grading
  nyata, laporan besar berjalan.
- Kekurangan: container mahal, clang di image, XP dipercaya klien, banyak bug
  lifecycle runner (SAB teracuni, timer berhenti setelah input, tidak ada
  timeout compile), contract dan test tipis.

v2:

- Kelebihan: struktur monorepo, kontrak OpenAPI, otorisasi/scoping, idempotency,
  race test, batas output, isolasi preview, biaya server mendekati nol.
- Kekurangan: runner proof belum ada, AI grading belum aktif, sebagian fitur
  lama belum parity, Phase 6 ditahan, efisiensi preview masih di bawah lama.

## 11. Rekomendasi

1. Pertahankan arah v2 untuk backend, auth, data, dan keamanan.
2. Ambil efisiensi runner lama tanpa mengambil bug-nya: worker persisten, cache
   compile, prefetch clang/lld, streaming output dengan cap.
3. Jangan port `compile-c` server dan jangan jadikan Glot sebagai proof.
4. Tutup gap parity secara eksplisit (AI grading, laporan besar, prestige) sebelum
   menyatakan cutover.
5. Runner authoritative tetap keputusan terpisah; sampai itu ada, code lesson
   belum dapat diselesaikan.
