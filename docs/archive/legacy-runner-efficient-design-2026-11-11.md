# Rancangan runner legacy efisien — v2

> **Historical/superseded for current Next Steps.** Implementasi final dan accepted risk dicatat pada [browser-completion decision](../elearning-browser-completion-decision-2026-11-12.md).

Status: rancangan. Belum ada source yang diubah. Phase 6 tidak disentuh.

## 0. Batas

- Yang dipertahankan adalah **sistem lama**: C dikompilasi dan dijalankan di
  browser (clang-wasm), Python memakai Pyodide, plus fallback server melalui
  Glot.
- Tidak ada VPS, container, daemon, atau clang di Vercel Function.
- Proof/penilaian tetap memakai `Runner` yang sudah ada. Rancangan ini hanya
  menyentuh jalur preview; code lesson tetap belum dapat diselesaikan sampai
  provider authoritative dipilih.
- `plan/**`, migration, dan Phase 6 tidak diubah.

## 1. Komponen lama dan nasibnya

| Lama | Lokasi | Nasib di v2 |
| --- | --- | --- |
| C compile+run di browser | `praktikum/frontend/static/clang-worker.js`, `shared.js` | Dipakai; sudah ada di `apps/portal/static/runtimes/v1` |
| Python Pyodide | `elearning/public/pyodide.worker.js` | Dipakai; sudah ada di runtime v1 |
| Inisialisasi worker persisten | `ClangInitializer.tsx`, `PyodideInitializer.tsx` | Dipakai ulang polanya; di v2 worker mati saat iframe di-dispose |
| Cache compile per source | `clang-worker.js` (`lastCompiledCode`) | Dipakai ulang; di v2 belum ada |
| Input interaktif | SAB (praktikum), async input (E-learning) | Ditambahkan kembali secara terkontrol |
| Fallback Glot | `praktikum/backend/pkg/glot/client.go` | Diport sebagai preview non-proof |
| Compile C server | `pkg/cwasm/compiler.go` | **Tidak diport**: tidak ada pemanggil, menambah clang ke image, dan bisa membocorkan secret lewat `#include` |
| Testcase + XP di klien | `LessonPage.tsx`, `progressService.ts` | **Tidak diport**: melanggar guard proof v2 |

## 2. Prinsip efisiensi

1. Satu unduhan aset per sesi, bukan per halaman atau per run.
2. Satu inisialisasi worker per bahasa per sesi.
3. Satu kompilasi C per source; ganti stdin tidak mengompilasi ulang.
4. Preview tidak menghasilkan invocation Vercel sama sekali.
5. Fallback server hanya pengecualian (runtime gagal dimuat), bukan jalur normal.
6. Output di-stream dan dibatasi saat produksi, bukan ditampung lalu dipotong.
7. Tidak ada antrean in-process; gagal cepat lebih baik daripada menahan instance.

## 3. Arsitektur

```
apps/elearning, apps/praktikum
        │  (iframe cross-origin, sandbox="allow-scripts")
        ▼
apps/portal /runtimes/v1
  frame.html → frame.js → Worker
        ├─ C: clang.wasm + lld.wasm + memfs + sysroot.tar   (sekali unduh, compile di-cache)
        └─ Python: Pyodide (sekali init, reuse globals baru per run)

Hanya jika runtime gagal dimuat:
POST /v1/playground/run → Go API → Glot.io → output (authoritative: false)
```

## 4. Optimasi runtime browser

### 4.1 Iframe dan worker tidak boleh mati saat pindah halaman

Kondisi sekarang: `BrowserPreviewClient` menghapus iframe ketika ref-count
`acquirePreview` mencapai nol, yaitu saat komponen unmount
(`packages/ui/src/browser-preview.ts`). Akibatnya pindah pelajaran memicu
unduhan ulang clang/Pyodide dan init ulang.

Perbaikan:

- `release()` hanya menandai idle; iframe dipertahankan.
- Dispose setelah idle 10 menit tanpa run, atau saat tab `pagehide`.
- Satu iframe per origin, dipakai bersama semua halaman.

Efek: unduhan 60 MB dan init Pyodide terjadi sekali per sesi.

### 4.2 Worker C: cache hasil kompilasi

`worker.js` sekarang selalu `api.compile` + `api.link` setiap run. Versi lama
memakai perbandingan string `contents === lastCompiledCode` yang hanya hidup di
memori worker, sehingga hilang saat reload atau terminate.

Tambahkan:

- hash source + versi compiler sebagai key, bukan perbandingan string;
- simpan `{ key, module }` terakhir (LRU 4 bila perlu);
- bila key sama, lewati compile+link, langsung `api.run`;
- `stdin` tetap di-set per run melalui `memfs.setStdinStr`;
- reset memfs sebelum compile berikutnya: `addFile('main.c'/'main.o'/'main.wasm')`
  pada versi lama tidak pernah dihapus, sehingga heap tumbuh setiap recompile.

Opsional bila pengukuran membenarkan: simpan `WebAssembly.Module` di IndexedDB
dengan key source+versi, agar terminate/reload tidak memaksa compile ulang.

Efek: percobaan beberapa stdin pada source yang sama tidak mengompilasi ulang.

### 4.3 Worker Python: reuse, bukan terminate

`frame.js` memanggil `stop()` setiap hasil Python, sehingga Pyodide
di-init ulang (~1–3 dtk) pada setiap Run.

Perbaikan:

- worker dipertahankan; setiap run membuat globals baru
  (`pyodide.runPython('dict(__name__="__main__")')`), prelude menetapkan
  `sys.stdout`/`sys.stderr` ke `StringIO`, lalu `globals.destroy()`.
- Versi lama menjalankan `runPythonAsync(code)` di namespace modul, sehingga
  variabel, import, dan `sys.stdout` bocor antar run; jangan diulang.
- terminate hanya saat: `cancel`, timeout, error fatal, ganti bahasa, atau
  worker sudah menjalankan 50 run (recycle untuk membatasi pertumbuhan heap).
- `setStdin` tetap dipakai untuk stdin non-interaktif.

### 4.4 Streaming compile

Aset `clang` (31,2 MB) dan `lld` (19,5 MB) tidak berekstensi `.wasm` dan
disajikan tanpa `Content-Type: application/wasm`, sehingga
`WebAssembly.compileStreaming` dimatikan (`if (false && ...)`) dan kedua file
ditampung di `ArrayBuffer`.

Perbaikan:

- sajikan sebagai `clang.wasm`, `lld.wasm`, `memfs.wasm` dengan
  `Content-Type: application/wasm` di `apps/portal/vercel.json` dan dev server;
- aktifkan `compileStreaming(fetch(url))`, dengan fallback ke `arrayBuffer`
  bila MIME salah;
- pertahankan path ber-versi (`/runtimes/v2/...`) agar cache immutable benar.

Efek: unduhan dan kompilasi tumpang tindih, memori puncak turun sekitar satu
salinan penuh (50 MB).

### 4.5 Prewarm

Temuan audit: `init_done` hanya menandakan `memfs` + `sysroot.tar` siap
(9,6 MB). `clang` dan `lld` (50,7 MB) baru diambil pada compile/link pertama di
dalam Run, tanpa perubahan status UI.

- `connect` membawa `language`; frame langsung meng-init worker dan mengunduh
  `memfs` + `sysroot`.
- Setelah `ready`, frame segera mem-prefetch `clang.wasm` dan `lld.wasm` pada
  idle, dan melaporkan progres unduhan ke UI.
- Editor mengirim `warmup` saat fokus, bukan saat tombol Run ditekan.

Efek: 50 MB keluar dari jalur kritis Run pertama, dan mode "Run terlihat
menggantung" hilang.

### 4.6 Output streaming dengan batas

- Worker mengirim `write` per potongan (stdout/stderr) selama program berjalan,
  dibatch per 16 ms atau 16 KiB, dengan cap 16 KiB per stream dihitung saat
  append.
- `result` final tetap memuat teks terbatas dan `truncated`.
- UI menampilkan bertahap; tidak ada string tak terbatas.
- Versi lama men-stream setiap `fd_write` tanpa batching dan menyimpan salinan
  ketiga (xterm + buffer + DOM aria-live). Jangan diulang; satu buffer dengan
  cap dan render per frame.

### 4.7 Input interaktif

- Bila `crossOriginIsolated` dan `SharedArrayBuffer` tersedia: mode SAB, sama
  seperti praktikum lama. Worker memblokir di `Atomics.wait`, frame mengirim
  baris input.
- Bila tidak: hanya stdin yang dikirim di awal (cukup untuk testcase).
- Untuk Python, `input()` memakai `pyodide.setStdin` dengan callback SAB; tidak
  ada penulisan ulang source dengan regex seperti versi lama.
- Header path-scoped di `/runtimes/v1/*`:
  `Cross-Origin-Opener-Policy: same-origin`,
  `Cross-Origin-Embedder-Policy: require-corp`,
  `Cross-Origin-Resource-Policy: same-origin`.
  Praktikum lama memakai COOP `same-origin` + COEP `credentialless`; E-learning
  tidak punya header sama sekali. Pilih satu konfigurasi dan uji Pyodide di
  dalamnya.
- Pyodide di-host sendiri di `/runtimes/v1/pyodide/` agar lolos COEP dan tidak
  bergantung CDN. Unduhan hanya terjadi pada pelajaran Python.

### 4.8 Timeout dan cancel

- Watchdog arm sebelum `compile` dikirim, bukan setelah compile selesai; versi
  lama baru memulai timer setelah compile, sehingga unduhan 50 MB atau clang
  yang hang tidak pernah timeout.
- Anggaran: C 20 dtk (termasuk unduhan pertama bila belum di-prefetch), Python
  10 dtk.
- Timer dihentikan saat menunggu input dan **dimulai lagi** setelah input
  dikirim; versi lama tidak pernah memulai ulang, sehingga program yang membaca
  input lalu loop selamanya tidak pernah dihentikan.
- Timeout men-terminate worker, mengembalikan `status: timeout`, lalu worker
  dibuat ulang saat run berikutnya.
- `cancel` dari UI selalu men-terminate; header SAB di-reset sebelum worker baru
  dipakai, karena versi lama meninggalkan flag cancel yang membuat semua run
  interaktif berikutnya langsung batal.

### 4.9 Cache aset

- `/runtimes/v1/*` sudah `public, max-age=31536000, immutable`; pertahankan.
- `sysroot.tar` (9 MB) dapat disajikan sebagai `.tar.gz` dan didekompresi
  dengan `DecompressionStream('gzip')`; hanya dilakukan bila pengukuran
  menunjukkan manfaat.
- Versi aset di-bump lewat path (`/runtimes/v2/...`) saat berubah, bukan query
  string, agar cache immutable tetap benar.

### 4.10 Memori

- Cache compile maksimal 4 modul.
- Recycle worker Python setelah 50 run atau 10 menit idle.
- Recycle worker C setelah 200 run.

### 4.11 Bug lama yang tidak boleh ikut diport

Audit source lama menemukan cacat yang harus dihindari, bukan direplikasi:

| Bug | Lokasi lama | Aturan di v2 |
| --- | --- | --- |
| Cancel SAB meninggalkan flag permanen | `shared.js` + `CodeEditor.svelte` | Reset header SAB sebelum worker baru dipakai |
| Flag input basi dikonsumsi run berikutnya | `cInput.ts`, `shared.js` | Reset header di awal setiap run |
| Timer 40 dtk berhenti setelah input pertama | `CodeEditor.svelte` | Timer restart setelah input dikirim |
| Compile tidak punya timeout | `CodeEditor.svelte` | Watchdog arm sebelum compile |
| Worker dibagi lalu di-terminate saat keluar route | `playground-c/+page.svelte` | Satu pemilik worker; reset state saat dispose |
| Dua run bersamaan berbagi `main.c`/`currentModule` | `clang-worker.js` | Satu run aktif; tolak run kedua |
| `injectStdoutUnbuffering` tanpa `<stdio.h>` | `useCodeRunner.ts` | Jangan port; kalau dipakai, sertakan header |
| ID respons acak dan `write` tanpa run id | worker lama | Counter monotonik + run id di setiap pesan |
| Globals Python bocor antar run | `pyodide.worker.js` | Globals baru per run |
| Rewrite `input(` secara tekstual merusak string | `pyodide.worker.js` | Jangan port; pakai stdin/SAB |
| Salinan output ganda/triple tanpa cap | `CodeEditor.svelte` | Satu buffer dengan cap |
| E-learning tanpa terminate dan tanpa timeout | `ClangInitializer.tsx` | Watchdog wajib |

## 5. Protokol frame ↔ worker

```
parent → frame   { type:'connect', language }
frame  → parent  { type:'connected' }
frame  → worker  { type:'init', language, base }
worker → frame   { type:'ready' } | { type:'init-error' }
parent → frame   { type:'warmup' }
parent → frame   { type:'run', id, language, source, stdin, interactive }
worker → frame   { type:'phase', id, phase:'compile'|'run' }
worker → frame   { type:'write', id, stream:'stdout'|'stderr', text }
worker → frame   { type:'input-request', id, prompt, stdout }
frame  → worker  { type:'input', id, value }
parent → frame   { type:'cancel', id }
worker → frame   { type:'result', id, status, stdout, stderr, truncated, durationMs }
```

Aturan:

- satu run aktif per iframe; run kedua ditolak;
- `id` wajib cocok; pesan dengan id lain diabaikan;
- frame memvalidasi tipe, panjang source (64 KiB), dan stdin (16 KiB);
- `status` hanya `success | compile_error | runtime_error | timeout`.

## 6. Fallback Glot

Endpoint baru, terpisah dari `Runner`:

`POST /v1/playground/run`

Request: `{ language: "c"|"python", source, stdin }`

Response:

```json
{
  "status": "success|compile_error|runtime_error|timeout|provider_error",
  "stdout": "...",
  "stderr": "...",
  "truncated": false,
  "duration_ms": 123,
  "provider": "glot",
  "authoritative": false
}
```

Aturan:

- dipicu hanya saat `init-error` runtime browser atau tombol eksplisit
  "Jalankan di server"; tidak otomatis saat timeout;
- autentikasi wajib; rate limit 5/menit dan 30/jam per user;
- source ≤ 64 KiB, stdin ≤ 16 KiB, UTF-8;
- `http.Client` tunggal dengan keep-alive, timeout 10 dtk, `RequestTimeout`
  25 dtk;
- body provider dibaca dengan `LimitReader` 256 KiB;
- tanpa retry; error provider tidak diteruskan mentah ke klien atau log;
- output melewati `boundedText` yang sama dengan `runner/validation.go`;
- tidak disimpan, tidak memberi XP, tidak menyentuh tabel run.

## 7. Proof dan completion

Tidak berubah:

- `POST /v1/learning/playground/run` tetap memakai `Runner` + testcase server;
- `Runner.Unconfigured` → `503`;
- tombol selesai pelajaran tetap tertahan;
- Glot tidak boleh menjadi `Runner` karena tidak memberi numeric exit code.

## 8. Kuota dan biaya

Preview browser: 0 invocation, 0 CPU, 0 memori di Vercel.

Fallback hanya pengecualian. Dengan 30 user/hari, 26 hari, asumsi 10% sesi
memakai fallback dan tiap panggilan 2 dtk pada 1 GB:

| Item | Per bulan | Kuota Hobby |
| --- | --- | --- |
| Invocation | ±780 | 1.000.000 |
| Provisioned memory | ±0,4 GB-jam | 360 GB-jam |
| Active CPU | mendekati 0 (menunggu I/O) | 4 jam |

Kesimpulan: jalur ini tidak akan mengulang kenaikan Fluid Compute selama tidak
ada clang/container di image API.

## 9. Perubahan file

Frontend/runtime:

| File | Perubahan |
| --- | --- |
| `apps/portal/static/runtimes/v1/worker.js` | Cache compile C per hash; Python reuse; streaming write; watchdog |
| `apps/portal/static/runtimes/v1/frame.js` | `connect` dengan language; prewarm; jangan stop Python per run; input SAB |
| `apps/portal/static/runtimes/v1/frame.html` | CSP disesuaikan bila Pyodide di-host sendiri |
| `apps/portal/static/runtimes/v1/pyodide/` (baru) | Pyodide core untuk COEP |
| `apps/portal/vercel.json` | MIME wasm; COOP/COEP/CORP; cache |
| `apps/portal/vite.config.ts` | Header yang sama untuk dev |
| `packages/ui/src/browser-preview.ts` | Idle dispose; warmup; protokol |
| `packages/ui/src/BrowserPreview.svelte` | Tombol fallback server; status |

Backend (fallback):

| File | Perubahan |
| --- | --- |
| `backend/internal/platform/preview/glot.go` (baru) | Klien bounded |
| `backend/internal/platform/preview/glot_test.go` (baru) | httptest |
| `backend/internal/config/config.go` | `PlaygroundFallback*` |
| `backend/internal/delivery/http/playground_handlers.go` (baru) | Handler + rate limit |
| `backend/internal/delivery/http/router.go` | Route |
| `docs/openapi/openapi.yaml` | Operation baru |

## 10. Tes dan acceptance

Fungsional:

- C: run pertama, ganti stdin tanpa compile ulang, compile error, runtime
  error, infinite loop → timeout, output bomb → `truncated`.
- Python: reuse worker, globals terisolasi antar run, exception, timeout.
- Interactive: SAB aktif saat `crossOriginIsolated`; fallback stdin saat tidak.
- Fallback: init gagal → tombol server; 429; provider 5xx; body oversize.

Metrik yang harus dibuktikan:

| Metrik | Target |
| --- | --- |
| Run C kedua, source sama | ≤ 150 ms |
| Run C, stdin berubah | ≤ 150 ms tanpa request aset baru |
| Run Python setelah init | ≤ 300 ms tanpa init ulang |
| Request aset runtime pada run kedua | 0 |
| Invocation Vercel untuk preview | 0 |
| Pertumbuhan memori 20 run | < 50 MB |
| Fallback p95 | ≤ 5 dtk |

Regression:

- completion tanpa proof tetap `409 CODE_NOT_VERIFIED`;
- tidak ada endpoint compile-c;
- tidak ada clang di image API.

## 11. Rollout dan rollback

1. Implementasi di belakang flag `RUNTIME_V2` (path aset baru
   `/runtimes/v2/`), tanpa mengubah `/runtimes/v1/`.
2. Uji di staging dengan 5 akun; bandingkan metrik.
3. Aktifkan per aplikasi (E-learning dulu, Praktikum kemudian).
4. Rollback: arahkan `runtimeOrigin`/path kembali ke v1; aset v2 dihapus.

## 12. Risiko

| Risiko | Dampak | Mitigasi |
| --- | --- | --- |
| COEP memblokir Pyodide CDN | Python gagal | Host Pyodide sendiri |
| Heap Pyodide tumbuh | Tab melambat | Recycle 50 run/10 menit |
| Worker hang | UI macet | Watchdog + terminate |
| Iframe persisten memakan memori | Perangkat lemah | Idle dispose 10 menit |
| Fallback disalahgunakan | Kuota Glot habis | Rate limit + hanya saat init gagal |
| Glot tanpa exit code | Tidak bisa jadi proof | Tetap preview-only |
| Aset besar di repo | Build/deploy lebih besar | Hanya Python yang menambah ±10 MB |

## 13. Yang tidak dikerjakan

- Phase 6.
- Provider authoritative (Vercel Sandbox/Judge0/Piston).
- Perubahan guard completion.
- Compile C server-side.
- Perubahan `plan/**`.
