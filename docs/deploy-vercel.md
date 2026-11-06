# Deployment produksi Vercel

Web/API Algohub menggunakan Vercel sejak cutover 6 November 2026. Database dan storage tetap di Supabase. Panduan VPS lama bukan jalur deployment produksi.

## Project dan source

Tim Vercel: `sistemenergis-projects`. Branch produksi: `main`.

| Project | Repository | Root Directory | Runtime | Domain |
| --- | --- | --- | --- | --- |
| `sesl-algoritma` | `Lab-Algoritma-dan-Pemrograman/praktikum` | `frontend` | SvelteKit / adapter-vercel | `algohub.web.id`, `www.algohub.web.id` |
| `labap-backend` | repository ini | `backend` | Container dari `backend/Dockerfile` | `api.algohub.web.id` |
| `algohub-elearning` | `Lab-Algoritma-dan-Pemrograman/E-Learning` | root repository | Vite + API Node | `elearning.algohub.web.id` |
| `algohub-siakad` | `Lab-Algoritma-dan-Pemrograman/Web-Lab-AP` | root repository | Vite + API Node | `siakad.algohub.web.id` |

Project frontend dan backend terpisah, bukan satu deployment multi-service. Konfigurasi yang dipakai berada di `frontend/vercel.json` dan `backend/vercel.json`.

Git integration harus menunjuk repository di tabel, bukan repository personal yang lama. Setelah integration aktif, push ke `main` memicu build produksi masing-masing project. Cek status deployment dan commit SHA setelah push; jangan menganggap Git push sama dengan deployment berhasil.

## Build dan environment

- Frontend: `bun install --frozen-lockfile`, kemudian `bun run build`.
- Backend: Dockerfile membangun binary Go, memasang Clang/WASI, dan menguji toolchain saat build. Runtime berjalan sebagai user non-root.
- `backend/api/` adalah entrypoint serverless lama. `.vercelignore` mencegahnya membuat fungsi yang bertabrakan dengan routing `/api/*` container.
- Rahasia hanya di environment Vercel. Jangan commit `.env`, private key, dump database, atau hasil export environment.
- `PUBLIC_API_BASE_URL` harus menunjuk backend produksi yang dapat dijangkau browser. Hook CSP memasukkan origin yang dikonfigurasi.
- `CORS_ORIGINS` backend harus mencakup origin utama, `www`, E-Learning, dan Siakad.
- Pengaturan DB dan Supabase tetap memakai database bersama yang sama; deploy tidak menjalankan migrasi/seed.
- Di Vercel, sweeper goroutine dinonaktifkan. Auto-submit harus dipicu scheduler melalui endpoint cron yang dilindungi secret. Verifikasi scheduler sebelum mematikan host lama.
- E-Learning memakai secret Supabase, secret Go/Web Utama, dan `SIAKAD_JWT_SECRET` server-side bila penandatangan Siakad berbeda. Secret bukan variabel frontend `VITE_*`.

## Pemeriksaan sebelum push

Dari `backend`:

```bash
go build ./...
go test ./...
```

Dari `frontend`:

```bash
bun run check
bun run build
bun run test:unit --run
```

Pada snapshot migrasi, build dan unit test lulus. `bun run check` masih melaporkan 5 error lama di luar patch deployment: `PlagiarismData.catatan` di halaman info serta tipe `RescheduleCard`/`RecruitCard` di pengaturan admin. Jangan menandai typecheck lulus sebelum masalah itu benar-benar diperbaiki.

## Pemeriksaan setelah deployment

1. Pastikan seluruh deployment berstatus Ready dan commit SHA sesuai push terbaru.
2. Buka apex dan `www`; keduanya harus memakai HTTPS valid dan memuat halaman `/info` serta form login.
3. Periksa `/api/health` pada domain API, apex, dan `www`: HTTP 200, `status: ok`.
4. Uji login dengan akun uji yang disetujui. Jangan mencetak password atau token ke log.
5. Uji token Siakad dan Go melalui E-Learning `/api/verify`; identitas tidak berubah dan token tidak valid tetap mendapat 401.
6. Pada `/info/praktikum`, jalankan C, dua input `scanf`, kode C salah, Python, dan `input()`. Kedua worker harus mendapat COEP yang sesuai dan SharedArrayBuffer tersedia.

Smoke test ini tidak menggantikan acceptance test pengumpulan ujian, grading, upload/download laporan, seluruh role, cron, atau uji beban.

## DNS dan pemulihan

Cloudflare tetap menjadi DNS provider. Record web memakai DNS only menuju target Vercel yang ditentukan masing-masing project; apex menggunakan CNAME flattening. Simpan TXT verifikasi domain.

Pada cutover, `www` sempat memiliki sertifikat apex yang tidak cocok. Sertifikat khusus `www.algohub.web.id` kemudian diterbitkan dan diverifikasi normal, tanpa mematikan validasi TLS.

Untuk regresi, promosikan deployment Vercel yang diketahui sehat atau revert commit melalui Git. Jangan mengembalikan DNS ke host lama tanpa memastikan origin/tunnel sehat dan source kompatibel. Host lama, backup, dan cron perlu diinventarisasi sebelum dihentikan. Layanan email/FTP tidak termasuk deployment web ini.
