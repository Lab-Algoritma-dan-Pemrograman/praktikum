# Upgrade Compiler Clang WASM — SAB Interactive Stdin (Praktikum AP)

## Ringkasan

Porting pola **SAB blocking stdin** dari E-Learning (sudah jalan, commit `aefcb4b`)
ke Praktikum AP. Upgrade penuh: compile + run tetap 100% di browser (clang-worker.js
+ shared.js), mengganti jalur "re-run" yang meng-compile ulang tiap input.

## Masalah saat ini

1. `frontend/static/clang-worker.js` + `shared.js` = versi lama (tanpa SAB).
   Worker TIDAK pernah mengirim `need-input`, sehingga program C yang memanggil
   `scanf`/`getchar` langsung mendapat EOF. Mode ketik `cStdinMode` di CodeEditor
   tidak pernah aktif.
2. CodeEditor pakai pola **re-run**: tiap Enter → `executeCCode(accumulatedStdin)`
   → compile ulang + run ulang dari nol. State program hilang (mis. program yang
   menanyakan 3 input berturut-turut akan restart di tiap pertanyaan).
3. `runner.ts` tanpa SAB, tanpa retry, tanpa `cWorkerRef` (tidak bisa terminate
   worker yang hang / infinite loop).
4. `hooks.server.ts` hanya men-set COOP/COEP di `/playground`. Halaman lain
   (ruang ujian, info/praktikum) tidak cross-origin isolated → SAB mati.
5. Tidak ada timeout 40 detik pada run C. Program infinite loop mengunci halaman
   selamanya.

## Solusi

Salin pola E-Learning (sudah teruji):

### File yang disalin & diadaptasi

| File E-Learning | Tujuan Praktikum AP | Perubahan |
|---|---|---|
| `public/clang-worker.js` | `frontend/static/clang-worker.js` | — (identik, sudah SAB) |
| `public/shared.js` | `frontend/static/shared.js` | — (identik, sudah SAB) |
| `src/lib/cInput.ts` | `frontend/src/lib/cInput.ts` | adaptasi import (tidak pakai zustand; pakai store Svelte) |
| `src/components/ClangInitializer.tsx` | `frontend/src/lib/stores/runner.ts` (initRunner) | adaptasi ke Svelte store |
| `src/hooks/useCodeRunner.ts` | `frontend/src/lib/components/CodeEditor.svelte` (executeCCode) | adaptasi ke Svelte |

### `runner.ts` (initRunner) — upgrade

- Tambah store: `cSab` (SharedArrayBuffer | null), `cWorkerRef` (Worker | null).
- `initRunner()`:
  1. Inisialisasi SAB 4096 B HANYA kalau `crossOriginIsolated`:
     `meta = Int32Array(sab, 0, 2)`, `data = Uint8Array(sab, 8)`.
  2. Buat worker `/clang-worker.js?v=7` + `MessageChannel`.
  3. `worker.postMessage({ id:'constructor', data: port2, sab }, [port2])`
     — SAB dikirim **by value**, hanya `port2` yang masuk transfer list
     (SAB tidak boleh di-transfer, `DOMException`).
  4. Retry 2x (delay 1,5 s) saat `init_error` / `worker.onerror`, baru
     `isCLoadingStore = false`.
  5. Simpan `worker` ke `cWorkerRefStore` (untuk terminate saat timeout/reset),
     `port` ke `cWorkerStore`.
- Pastikan `initRunner()` tetap idempotent (guard `runnerStarted`).

### `CodeEditor.svelte` — upgrade jalur C

Ganti `executeCCode` menjadi protokol E-Learning:

1. `runCode()` C:
   - `port.postMessage({ id:'compile', responseId, data: injectStdoutUnbuffering(value), legacy: !interactive })`.
     - `interactive = true` kalau halaman memakai terminal interaktif
       (Ruang ujian, playground). Untuk `info/praktikum` tetap interaktif.
   - Saat run: `port.postMessage({ id:'run', responseId, data: '' })`.
   - `need-input` listener di `port`: `clearTimeout(timer)` + set
     `cStdinMode = true` (fokus terminal).
   - `sendCInput(value)` → tulis ke SAB + `Atomics.notify`.
   - `runAsync` result: kalau `sabBlocked` → tetap mode ketik; kalau tidak →
     `running = false`, output akhir.
2. **Timeout 40 detik** (pola E-Learning):
   - `setTimeout` 40.000 ms → `cWorkerRefStore?.terminate()` + reset store
     (`cWorkerStore = null`, `isCLoadingStore = true`) → initializer re-init.
   - `clearTimeout(timer)` saat menerima `need-input` — program yang sah
     menunggu input BUKAN hang.
3. Hapus pola re-run (`cStdinLines`/`cStdinOffsets`/compile ulang tiap Enter).
   - Enter saat `cStdinMode` → `sendCInput(cStdinBuffer)`.
4. `stopCode()` → `cancelCInput()` (flag 2) + terminate worker kalau hang.

### `hooks.server.ts` — COOP/COEP

Perluas COOP/COEP ke semua halaman yang memakai CodeEditor:

```ts
const SAB_PAGES = ['/playground', '/playground-c', '/praktikum', '/info/praktikum'];
if (SAB_PAGES.some((p) => event.url.pathname.startsWith(p))) {
  response.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
  response.headers.set('Cross-Origin-Embedder-Policy', 'credentialless');
}
```

- `credentialless` (bukan `require-corp`) agar resource cross-origin (gambar
  Supabase, CDN) tetap dimuat tanpa CORP.
- `CACHEABLE_STATIC` sudah mencakup `clang-worker.js`/`shared.js` — cache
  immutable, tapi versi lama mungkin ter-cache di browser user. Tambah query
  version `?v=8` di `initRunner` (bukan `?v=7`) untuk memaksa refresh.
- **PENTING**: COEP `credentialless` di halaman admin/ruang ujian berarti
  semua request cross-origin TANPA kredensial. Ini aman untuk Supabase
  (JWT dikirim via header Authorization, bukan cookie). Tapi harus diuji.

## File yang TIDAK diubah

- `frontend/static/c-worker.js` + `playground-c/+page.svelte` — jalur
  "compile server + run browser" tetap dipertahankan (private playground).
- `frontend/src/routes/playground/+page.svelte` — Python, tidak tersentuh.
- Backend `pkg/cwasm` — tetap dipakai untuk jalur server (fallback + playground-c).

## Verifikasi

1. `cd frontend && bun run check && bun run build`.
2. `node --check frontend/static/shared.js frontend/static/clang-worker.js`.
3. Manual di dev (HTTPS local atau Vercel preview):
   - `curl -sI <url>/praktikum/...` → COOP/COEP ada.
   - Console: `crossOriginIsolated = true` di halaman ruang ujian.
   - Run program `scanf` → prompt muncul, ketik, lanjut tanpa recompile.
   - Program `while (getchar() != '\n')` → menunggu input, Enter, selesai.
   - Program minta 3 input berturut-turut → state lestari antar input.
   - Reset saat hang → terminal bersih tanpa error, worker re-init.
   - `info/praktikum` (publik, tanpa login) → worker tetap jalan.

## Open Questions

1. **Scope COEP**: perluas ke `/praktikum` + `/info/praktikum` saja, atau semua
   halaman (global)? (rekomendasi: hanya halaman CodeEditor, tidak global)
2. **playground-c**: biarkan di jalur server (compile Go + c-worker.js), atau
   upgrade juga ke clang-worker SAB penuh?
3. **`?v=8`**: setuju tambah query version biar browser user dapat worker baru
   (cache immutable 1 tahun)?
