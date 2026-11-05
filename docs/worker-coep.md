# Header worker Clang dan Pyodide

Perbaikan produksi terverifikasi pada 2026-11-05.

## Origin produksi

`algohub.web.id` dilayani **Instabox melalui Cloudflare Tunnel + Caddy**, bukan
VPS Paluketok, sejak migrasi 2026-11-01. Apex masuk melalui Caddy `:4175`, yang
meneruskan halaman ke `praktikum-frontend` (`:3000`) dan `/api/*` ke backend
(`:8081`). File worker dilayani langsung dari `/data/praktikum/frontend/static`.
Jangan menyalakan service Algohub di Paluketok untuk memperbaiki situs live.

## Gejala dan penyebab

Worker mendapat HTTP 200 dengan MIME JavaScript yang benar, tetapi browser
menolak memuatnya dengan `ERR_BLOCKED_BY_RESPONSE`. DevTools Protocol menunjukkan
`blockedReason: coep-frame-resource-needs-coep-header`.

Halaman memakai `Cross-Origin-Embedder-Policy: credentialless`, sedangkan respons
worker dari Caddy tidak memiliki COEP. Handler file statis melewati
[hooks.server.ts](../frontend/src/hooks.server.ts), sehingga menambah header di
SvelteKit saja tidak cukup.

## Konfigurasi Caddy

Blok berikut dipasang pada site `:4175` di konfigurasi Caddy. Pertahankan handler
halaman, API, dan cache binary WASM yang sudah ada; ini bukan pengganti seluruh
Caddyfile.

```caddyfile
@wjs path /shared.js /clang-worker.js /c-worker.js /pyodide.worker.js /playground-worker.js
handle @wjs {
    header Cross-Origin-Embedder-Policy "credentialless"
    header Cross-Origin-Resource-Policy "same-origin"
    header Cache-Control "no-cache"
    root * /data/praktikum/frontend/static
    file_server
}
```

Backup konfigurasi sebelum perubahan, jalankan `caddy validate`, lalu reload
Caddy. Jangan mematikan COOP/COEP pada halaman: input C interaktif membutuhkan
SharedArrayBuffer dan cross-origin isolation.

## Cache dan deployment frontend

[runner.ts](../frontend/src/lib/stores/runner.ts) memakai `WORKER_VERSION = 'v=10'`
untuk kedua URL: `/clang-worker.js?v=10` dan `/pyodide.worker.js?v=10`.

Naikkan versi URL ketika kode atau header worker berubah dan cache lama belum
diinvalidasi. Pada perbaikan ini edge masih mengirim TTL 14400 detik meskipun
origin memakai `no-cache`; versi URL baru diperlukan untuk melewati respons
worker lama yang belum memiliki COEP.

Bangun frontend ke direktori terpisah sebelum mengganti build aktif. Uji
entrypoint `index.js` pada port loopback yang belum digunakan; adapter-node dapat
menyimpan SSR di `server/chunks`, bukan `server/index.js`. Simpan build lama untuk
rollback dan pertahankan aset lama bernama hash agar tab yang sudah terbuka tidak
kehilangan chunk. Perbaikan ini tidak memerlukan restart backend, perubahan DNS,
atau migrasi database.

## Verifikasi browser

Diuji pada `/info/praktikum` dengan Playwright + Edge, context browser baru,
tanpa akun atau perubahan data mahasiswa:

- Kedua worker `?v=10` berhasil dimuat dan `crossOriginIsolated === true`.
- Compile + jalankan C menghasilkan `C runtime OK: 42`.
- Dua input `scanf` berurutan (`Uji`, `7`) menghasilkan `Halo Uji, kuadrat=49`.
- Kode C tidak valid menghasilkan diagnostik compiler, bukan kegagalan worker.
- Python menghasilkan `Python runtime OK: 42`; `input()` interaktif juga lulus.
- Tidak ada worker yang diblokir COEP atau page exception.

Tunggu terminal selesai dibuat sebelum menekan Run. Tombol Run yang diklik
terlalu dini dapat diabaikan; perilaku tersebut belum diubah dalam patch ini.
Warning CSP Cloudflare Analytics merupakan masalah terpisah dan tidak menghambat
compiler.

Build produksi berhasil. `bun run check` masih melaporkan 5 error dan 39 warning
di luar patch worker: `PlagiarismData.catatan` pada halaman info, serta tipe
`RescheduleCard`/`RecruitCard` pada pengaturan admin. Pemeriksaan ini bukan lulus
bersih; error tersebut tidak diperbaiki dalam patch worker.
