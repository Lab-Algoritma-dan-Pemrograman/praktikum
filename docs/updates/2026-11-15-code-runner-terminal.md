# 2026-11-15 — Code runner: output terminal & input interaktif

## Ruang lingkup

- Environment: `lokal` (diimplementasi & terverifikasi 2026-11-15).
- Area: `elearning`, `praktikum`, `packages/ui`, `apps/portal` (runtime).
- Pelaku: agen (analisis + rancangan).
- Keluhan sumber: pada halaman praktikum, runner tidak interaktif dan tidak ada
  terminal sebagai output saat kode error / lancar.

## 1. Temuan

### 1.1 Project lama — `project_ap/elearning` (React + Vite)

Terminal interaktif yang benar sudah pernah jalan:

| File | Peran |
| --- | --- |
| `src/components/CodeEditor.tsx` | Editor Monaco, tombol Jalankan, shortcut Ctrl+Enter |
| `src/hooks/useCodeRunner.ts` | Python lewat Pyodide Web Worker; C lewat Clang/WASM worker + fallback Judge0/Wandbox |
| `src/pages/TerminalDemo.tsx` | Terminal demo (`@xterm/xterm` + addon-fit) |
| `src/pages/Playground.tsx` | Playground penuh: editor + terminal + input interaktif |
| `public/pyodide.worker.js`, `public/clang-worker.js` | Worker eksekusi |

Pola interaktif lama:

- `xterm.onKey` menangkap ketikan → buffer → kirim ke worker saat Enter.
- Protokol pesan worker: `RUN_INTERACTIVE`, `INPUT_REQUEST`, `RESPONSE`,
  `RUN_DONE`, `RUN_ERROR`, `RUN_CANCELLED`.
- Output ANSI berwarna (`\x1b[1;32m...`), prompt inline di dalam aliran output.
- Gambar matplotlib diurai dari stdout (`parseOutputWithImages`).

### 1.2 V2 — `project_ap_v2` (SvelteKit monorepo)

Engine eksekusi V2 **lebih matang** daripada lama dan sudah mendukung semua yang
dibutuhkan:

- `apps/portal/static/runtimes/v3/frame.js` + `worker.js`:
  - C → Clang/WASM (compile cache, worker reuse, recycle); Python → Pyodide.
  - Streaming output per-chunk: `write` `{stream, text}` → `onWrite`.
  - Input interaktif: `input-request` → `onInput(prompt)`; C memakai
    `SharedArrayBuffer` (aktif karena iframe `crossOriginIsolated`), Python
    memakai injeksi top-level `await _async_input`.
  - Matplotlib → `image`; watchdog timeout (C 20s / Python 10s); cap output 16 KiB.
  - Header COOP/COEP sudah dipasang di `apps/portal/vercel.json` (`/runtimes/v3/*`).
- `packages/ui/src/browser-preview.ts` + `BrowserPreview.svelte`: klien iframe +
  MessagePort. Sudah menangani prefetch, `live.stdout/live.stderr` selama berjalan,
  form input, verifikasi testcase, dan fallback server (Glot).
- Pemakai: `apps/elearning` (`routes/playground`, `lib/LessonView.svelte`) dan
  `apps/praktikum` (`lib/CodeEditor.svelte`).

### 1.3 Celah yang dirasakan pengguna

Bukan kekurangan engine, melainkan **presentasi**:

1. Output akhir dirender dua blok `<pre>` terpisah (`stdout preview` dan
   `stderr preview`), bukan satu scrollback ala terminal. Saat error hanya
   muncul heading `Preview browser: compile_error` + pre.
2. Prompt input muncul sebagai `<form>` di luar aliran output (tidak inline),
   tanpa riwayat baris perintah.
3. Tidak ada status berwarna, tidak ada scrollback tunggal, tidak ada kesan
   "terminal".

## 2. Keputusan adopsi (cek copycat)

**Tidak menyalin xterm.js dari project lama. Menyalin perilakunya.**

Alasan (tanggung jawab: hemat, bukan ceroboh):

- Engine V2 sudah streaming output dan sudah menangani stdin interaktif. Satu-
  satunya yang hilang adalah wadah tampilannya.
- Menambah `@xterm/xterm` + `@xterm/addon-fit` + CSS berarti dependency baru,
  integrasi Svelte 5, dan aksesibilitas baru (xterm memakai textarea/canvas)
  hanya untuk masalah rendering.
- Alternatif native (`<pre>` scrollback + `aria-live` + input inline) = nol
  dependency dan sudah menjadi pola di `BrowserPreview.svelte`.

`ponytail:` batas — tanpa xterm tidak ada emulasi ANSI penuh / PTY / curses.
Pasang xterm hanya bila nanti butuh kurva kontrol ANSI lengkap atau program
interaktif layar penuh; saat itu enkapsulasi di satu komponen UI baru.

## 3. Rancangan

Tujuan: satu panel terminal tunggal yang menampilkan stdout+stderr+prompt+input
dalam urutan waktu, plus status, tanpa mengubah engine maupun isolasi iframe.

### 3.1 Komponen

- Ubah area output `packages/ui/src/BrowserPreview.svelte` menjadi **terminal
  scrollback**:
  - Gabungkan `onWrite` stdout/stderr menjadi satu buffer baris
    `{ kind: 'stdout'|'stderr'|'input'|'system', text }`.
  - stderr diberi warna bahaya (token `--ah-console-danger`) tapi tetap satu aliran.
  - Prompt `onInput` dirender inline (baris terakhir), input diketik di satu
    baris `<input>` yang menempel di dasar panel.
  - Echo input pengguna masuk ke scrollback sebagai baris `input`.
  - Baris status: `compile_error` / `runtime_error` / `timeout` / `success`
    dengan warna, bukan heading terpisah.
  - Auto-scroll ke bawah; `aria-live="polite"`; `tabindex="0"` pada scrollback
    agar bisa digulir dengan keyboard (mengikuti pola `.runner-output` sekarang).
  - Batas tetap: cap 16 KiB (sudah ada di engine) + `max-height` scroll.

- Opsi minimal (dipilih): tempel di `BrowserPreview.svelte`; tidak membuat
  komponen baru. Bila untuk praktikum ingin tanpa verifikasi testcase, prop
  `verify` sudah memisahkan alur.

### 3.1.1 Gaya (CSS)

Kelas `.runner-output` tinggal di `packages/ui/src/console.css`. Gaya terminal
(scrollback, warna baris, prompt inline) ditaruh di blok `<style>` komponen
`BrowserPreview.svelte` supaya tidak menyentuh token global; kelas status
memakai variabel `--ah-console-*` yang sudah ada. Tidak menambah file CSS baru.

### 3.1.2 Kontrak tampilan yang WAJIB dipertahankan

Perilaku berikut dipakai test dan tidak boleh hilang saat aliran digabung:

| Kontrak | Alasan |
| --- | --- |
| Teks nota `Output dipotong sesuai batas ukuran.` | diperiksa `browser-preview.spec.ts` |
| Node `data-testid="preview-duration"` | metrik runtime/kecepatan run di E2E |
| Gambar matplotlib tetap dirender (label `Grafik hasil preview N`) | `browser-preview-regressions.spec.ts` |
| Label input = teks prompt (mis. `Name:`) | `getByLabel('Name:')` di regressions |
| Label input prompt tetap unik saat mode interaktif | hindari tabrakan label |

**Perubahan kontrak yang disengaja:** panel perantara `stdout sementara` /
`Keluaran sementara` **dihapus** — output sementara kini mengalir langsung ke
scrollback terminal. Test yang menegakkannya
(`browser-preview-regressions.spec.ts`) wajib disesuaikan.

### 3.2 Pemakaian

- `apps/elearning/src/routes/playground/+page.svelte` dan
  `apps/elearning/src/lib/LessonView.svelte`: tidak berubah API, hanya tampilan.
- `apps/praktikum/src/lib/CodeEditor.svelte`: sama; panel "Opsi runner server"
  (eksekusi server/Glot untuk penilaian) tetap terpisah dan tidak diubah.

### 3.3 Kontrak yang terdampak

Label E2E lama (`stdout preview browser`, `stderr preview browser`,
`preview-duration`, `Preview browser: success|compile_error`) akan berubah saat
aliran digabung. Wajib ikut diperbarui:

- `tests/frontend/e2e/browser-preview.spec.ts`
- `tests/frontend/e2e/browser-preview-regressions.spec.ts` (termasuk `stdout
  sementara`, `Hentikan preview`)
- `tests/frontend/e2e/elearning.spec.ts` (heading `Preview browser: success`)

**Tidak perlu diubah:**

- `packages/ui/tests/browser-preview.test.ts` — menguji `boundedPreview` murni
  (fungsi, tanpa DOM), tidak terpengaruh perubahan tampilan.
- `tests/frontend/e2e/practicum.spec.ts` — menyentuh panel server runner
  (`Opsi runner server`, `Hasil runner soal N`) di `apps/praktikum`, bukan
  `BrowserPreview`. Tetap terpisah.

Sediakan penanda stabil untuk test: `data-testid="preview-duration"`,
`data-testid="terminal-scrollback"`, `data-testid="terminal-status"`, dan
`aria-label` pada baris input yang sama dengan teks prompt.

## 4. Verifikasi (setelah implementasi)

Status: **dijalankan 2026-11-15.** Perintah & hasil nyata:

| Check | Cara | Hasil |
| --- | --- | --- |
| Unit UI | `npm run test:unit --workspace=@algohub/ui` | 6 pass, 0 fail |
| Typecheck | `npm run check` | 6/6 app: 0 error, 0 warning |
| E2E preview | `npm run test:e2e -- browser-preview` (dgn `ALGOHUB_TEST_PYODIDE=true`) | 9 pass, 0 fail |
| E2E preview (tanpa opt-in) | `npm run test:e2e -- browser-preview` | 4 pass, 5 skip (runtime Python opt-in) |
| E2E elearning | `npm run test:e2e -- elearning` | 5 pass, 0 fail |
| A11y | `tests/frontend/e2e/a11y.spec.ts` | 35 pass, 30 fail **pre-existing** (judul `Login` vs `Masuk`; tidak menyentuh playground) |
| Interaktif C | scanf lalu ketik di terminal; SAB aktif (COEP sudah ada) | lolos di `browser-preview.spec.ts` |
| Interaktif Python | mode `Input interaktif` → prompt inline, label input = teks prompt | lolos di `browser-preview-regressions.spec.ts` |
| Error/lancar | compile_error, runtime_error, timeout, success → tampil di terminal | lolos (`terminal-status`) |
| Gambar | matplotlib `plt.show()` tetap render `Grafik hasil preview N` di terminal | lolos (`browser-preview-regressions.spec.ts`) |
| Pemotongan | output > 16 KiB tampilkan nota `Output dipotong sesuai batas ukuran.` | lolos (`.term-line` total = 16384 B) |
| Metrik | `data-testid="preview-duration"` masih terisi durasi run | lolos |

Catatan implementasi: penanda `data-testid="terminal-scrollback"` dan
`data-testid="terminal-status"` dipakai test; baris input prompt tetap memakai
teks prompt sebagai label (`getByLabel('Name:')`).

## 5. Risiko dan tindak lanjut

- [x] Keputusan owner: gabung jadi terminal (memenuhi keluhan). Tampilan
      stdout/stderr terpisah dihapus; kontrak E2E diperbarui.
- [x] Isolasi iframe tidak disentuh; sandbox/COOP/COEP tidak dilonggarkan.
- [x] Output tetap di-cap 16 KiB (`truncated` dipertahankan).
- [x] Tidak menambah provider/AI baru; fallback Glot tetap opt-in lewat tombol.
- [ ] (Lanjutan) `a11y.spec.ts` pre-existing gagal karena mismatch judul
      `Login — Algohub` vs `Masuk — Algohub`; perbaiki terpisah dari topik ini.

## 6. Tautan

- Engine V2: [frame.js](../../apps/portal/static/runtimes/v3/frame.js),
  [worker.js](../../apps/portal/static/runtimes/v3/worker.js)
- Klien: [browser-preview.ts](../../packages/ui/src/browser-preview.ts),
  [BrowserPreview.svelte](../../packages/ui/src/BrowserPreview.svelte)
- Referensi lama:
  `project_ap/elearning/src/pages/Playground.tsx`,
  `project_ap/elearning/src/hooks/useCodeRunner.ts`
- Rencana terkait: [plan/code-runner.md](../../plan/code-runner.md)
