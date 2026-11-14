# Rancangan runner legacy di v2 — 11 November 2026

> **Historical/superseded for current Next Steps.** Implementasi final dan accepted risk dicatat pada [browser-completion decision](elearning-browser-completion-decision-2026-11-12.md).

Dokumen ini merancang penggunaan sistem code runner project lama
(`E:\rekapProject\project_ap`) di v2. Sifatnya rancangan; belum ada source yang
diubah.

## 1. Dasar dan temuan

Pembacaan source lama menunjukkan tiga jalur, bukan satu:

| Jalur | Komponen lama | Peran sebenarnya |
| --- | --- | --- |
| Browser | `elearning/public/clang-worker.js`, `elearning/public/pyodide.worker.js`, `elearning/src/hooks/useCodeRunner.ts`; `praktikum/frontend/src/lib/components/CodeEditor.svelte` | Jalur utama. C dikompilasi dan dijalankan di browser; Python memakai Pyodide |
| Fallback server | `praktikum/backend/pkg/glot/client.go`, `POST /api/praktikum/run` | Proxy Glot.io, hanya dipanggil bila compiler browser belum siap dan pengguna login |
| Compile server | `praktikum/backend/pkg/cwasm/compiler.go`, `POST /api/praktikum/compile-c` | clang-19 native di container; **tidak ada pemanggil di frontend** |

Konsekuensi penting:

- Project lama tidak memiliki eksekusi authoritative. Penilaian kode memakai AI
  rubric (`praktikum/backend/pkg/ollama/rubric.go`) atau manual.
- E-learning lama menentukan lulus/tidak di browser (`LessonPage.tsx`) lalu
  menulis `student_progress` dan `users.xp` langsung dari klien
  (`services/progressService.ts`). Ini tidak dapat dipakai sebagai proof.
- Glot hanya mengembalikan `duration/stdout/stderr/error`; numeric exit code dan
  fase tidak tersedia. Signal-kill menghasilkan HTTP 200 dengan `error` kosong.
- `compile-c` adalah beban mati yang menambah clang/LLVM ke image runtime —
  komponen utama kenaikan Fluid Compute, bersama container monolith.

## 2. Keputusan yang diusulkan

**Pakai legacy untuk preview, jangan untuk proof.**

- Browser (sudah ada di v2, dipertahankan dan dioptimalkan) tetap satu-satunya
  jalur eksekusi interaktif.
- Glot dipakai sebagai fallback preview melalui Go API, ditandai eksplisit
  `authoritative: false`, tidak disimpan, tidak memberi XP.
- `compile-c` **tidak** dipindahkan ke Vercel Function (alasan keamanan di §7).
- `Runner` interface tetap `Unconfigured` sampai provider authoritative dipilih.
  Pelajaran kode belum dapat diselesaikan; grading tetap manual/AI.

Dengan batas ini, arsitektur tetap sesuai `plan/code-runner.md` dan
`plan/deployment.md`: browser untuk latihan, provider eksternal melalui Go API,
tidak ada eksekusi source di proses Go.

## 3. Arsitektur

```
Browser
  ├─ apps/portal/static/runtimes/v1 (clang-wasm + Pyodide, iframe CSP ketat)
  │     └─ preview / latihan / playground        ← 0 compute Vercel
  │
  └─ POST /v1/playground/run (auth, rate limit)  ← hanya bila preview gagal
        └─ Go API (stateless)
              └─ platform/preview (Glot client, bounded)
                    └─ Glot.io                ← output non-proof

POST /v1/learning/playground/run  → usecase proof → Runner.Unconfigured → 503
```

Tidak ada container, tidak ada clang di image API, tidak ada antrean in-process.

## 4. Alur

### 4.1 Preview browser

Aset yang sudah ada: `apps/portal/static/runtimes/v1/{worker.js,frame.js,clang/}`
dan `packages/ui/src/browser-preview.ts`.

Perbaikan efisiensi yang diambil dari pola lama:

1. **Worker Python tidak di-terminate setiap run.** Saat ini
   `frame.js` memanggil `stop()` setiap hasil Python, sehingga Pyodide
   di-init ulang (~1–3 dtk) pada setiap klik Run. Worker dipertahankan, state
   dibersihkan dengan globals baru per run; terminate hanya saat error fatal,
   ganti bahasa, atau `cancel`.
2. **Cache hasil compile C per source.** `clang-worker.js` lama menyimpan
   `lastCompiledCode`; v2 selalu `compile` + `link`. Simpan `WebAssembly.Module`
   terakhir beserta hash source, sehingga percobaan stdin berbeda tidak
   mengompilasi ulang.
3. **Prewarm saat frame terhubung**, bukan saat Run pertama, agar unduhan
   clang/Pyodide tidak memotong interaksi pertama.
4. Batas yang sudah ada dipertahankan: source 64 KiB, stdin 16 KiB, stdout/stderr
   16 KiB, `truncated`, satu run aktif, `cancel` men-terminate worker, CSP
   `default-src 'none'`.

### 4.2 Fallback Glot (non-proof)

Endpoint baru, terpisah dari usecase proof:

`POST /v1/playground/run`

Request:

```json
{ "language": "c|python", "source": "...", "stdin": "..." }
```

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

Aturan handler:

- Autentikasi wajib; rate limit per user 5/menit (DB-backed `TakeRateLimit`,
  key `user:<id>:playground:fallback`), `429` + `Retry-After`.
- Header `Idempotency-Key` tidak diperlukan karena tidak ada efek samping.
- Source ≤ 64 KiB, stdin ≤ 16 KiB, UTF-8 valid; language hanya `c`/`python`.
- `context.Context` diteruskan; timeout adapter 10 dtk, `RequestTimeout` 25 dtk.
- Satu `http.Client` dengan `Transport` di-reuse (`MaxIdleConnsPerHost: 4`,
  `IdleConnTimeout: 30s`, `TLSHandshakeTimeout: 5s`).
- Body provider dibaca dengan `io.LimitReader` 256 KiB; lebih besar →
  `provider_error`.
- Tanpa retry. 4xx/5xx/timeout/decoding gagal → `provider_error`; jangan
  meneruskan body provider ke klien atau log (bisa memuat source).
- Output melewati `boundedText` yang sama dengan `runner/validation.go`.
- Hasil tidak disimpan, tidak menyentuh `learning_runs`/`practicum_runs`, tidak
  memberi XP.

### 4.3 Compile server C

Tidak dipindahkan. Alasan:

1. Tidak ada pemanggil di frontend lama; browser sudah mengompilasi sendiri.
2. Menambah clang-19 + lld + wasi-libc ke image API: cold start dan memori naik.
3. **Kebocoran rahasia.** clang membaca file yang dirujuk `#include`. Source
   mahasiswa dapat memakai `#include "/proc/self/environ"`; pesan error compiler
   berpotensi memuat isi environment proses (DSN database, service role key,
   token provider). Rate limit dan timeout tidak menutup ini.
4. Butuh sandbox yang disetujui; `plan/user_prompt/phase-3.md` melarang
   pemrosesan source mahasiswa di proses Go/Vercel tanpa itu.

Bila tetap dipaksa, syarat minimumnya: proses terpisah tanpa secret sama sekali,
tanpa akses jaringan, `CLANG_*` dijalankan sebagai user tanpa izin baca
environment, dan rate limit ketat. Itu praktis berarti runner sendiri — bukan
Vercel Function.

### 4.4 Proof dan completion

Tidak berubah:

- `POST /v1/learning/playground/run` tetap memakai `Runner` + testcase server.
- `Runner.Unconfigured` → `503 LEARNING_PROVIDER_UNAVAILABLE`.
- Tombol selesai pelajaran tetap tertahan (`codeRunRequired && !runner.available`).
- Glot **tidak boleh** dijadikan `Runner`: status `success` wajib punya
  `exit_code` numerik (`runner/validation.go`), dan mengarang exit 0 dilarang.

## 5. Batas dan biaya

Angka resmi Fluid Compute sin1: Active CPU $0.160/jam, Provisioned Memory
$0.0133/GB-jam. Kuota Hobby per bulan: 4 CPU-jam, 360 GB-jam, 1 juta invocation.

Perkiraan dengan asumsi 60 pengguna/hari:

| Skenario | Invocation/bulan | Memori/bulan | CPU/bulan | Catatan |
| --- | --- | --- | --- | --- |
| Preview browser | 0 | 0 | 0 | mayoritas trafik |
| Fallback 5% (≈75/hari, 3 dtk) | 2.250 | ±6 GB-jam | ±0 (menunggu I/O) | jauh di bawah kuota |
| Fallback 100% (1.500/hari, 3 dtk) | 45.000 | ±37 GB-jam | ±0 | masih di bawah 360 GB-jam |
| Compile-c 1.500/hari (0,3 dtk CPU) | 45.000 | ±75 GB-jam | ±3,75 CPU-jam | hampir menghabiskan kuota 4 CPU-jam |

Kesimpulan biaya: fallback Glot aman; compile server yang mahal.

Kontrol yang wajib:

- tanpa antrean in-process; gagal cepat `429`/`503`, bukan menahan instance;
- tanpa `maxDuration` besar; timeout provider di bawah deadline platform;
- tanpa clang/container di image API;
- rate limit dan kuota per user, bukan hanya global;
- output dan body provider dibatasi.

## 6. Perubahan file

Backend:

| File | Perubahan |
| --- | --- |
| `backend/internal/platform/preview/glot.go` (baru) | Klien Glot bounded: validasi, timeout, limit body, mapping status |
| `backend/internal/platform/preview/glot_test.go` (baru) | `httptest`: sukses, error, timeout, body oversize, 5xx, ctx cancel |
| `backend/internal/config/config.go` | `PlaygroundFallbackEnabled`, `PlaygroundFallbackURL`, `PlaygroundFallbackToken`, `PlaygroundFallbackTimeout` |
| `backend/internal/delivery/http/router.go` | Daftarkan route `POST /v1/playground/run` |
| `backend/internal/delivery/http/playground_handlers.go` (baru) | Handler, rate limit, decode bounded |
| `docs/openapi/openapi.yaml` + `scripts` sinkron | Operation baru |

Frontend/runtime:

| File | Perubahan |
| --- | --- |
| `apps/portal/static/runtimes/v1/frame.js` | Worker Python persisten; prewarm; tetap terminate saat cancel/error |
| `apps/portal/static/runtimes/v1/worker.js` | Cache `WebAssembly.Module` C per hash source |
| `packages/ui/src/browser-preview.ts` | Kirim `warmup` saat connect; sesuaikan protokol bila perlu |
| `packages/ui/tests/browser-preview.test.ts` | Tes protokol baru |

## 7. Konfigurasi

```
ALGOHUB_PLAYGROUND_FALLBACK_ENABLED=true
GLOT_URL=https://glot.io/api/run
GLOT_TOKEN=<secret server-side>
ALGOHUB_PLAYGROUND_FALLBACK_TIMEOUT=10s
```

Token hanya di environment Vercel; tidak masuk bundle, log, artefak, atau
dokumen. Tanpa token, endpoint mengembalikan `503` dan UI tetap memakai browser.

## 8. Tes

- unit: allowlist bahasa, batas source/stdin, `boundedText`, mapping error,
  timeout/ctx, body oversize, rate limit, tidak ada source di log.
- integration: fake Glot via `httptest`; provider 500, latency, body besar.
- frontend: worker Python tidak reload antar run; cache compile C; cancel tetap
  men-terminate; output cap dan `truncated`.
- E2E: preview jalan tanpa server; fallback hanya saat diminta; completion tetap
  `409 CODE_NOT_VERIFIED` tanpa proof.
- regression biaya: pastikan tidak ada clang di image dan tidak ada endpoint
  compile-c.

## 9. Risiko dan keputusan yang dibutuhkan

| Risiko | Dampak | Keputusan |
| --- | --- | --- |
| Glot tanpa exit code | Pelajaran kode tidak bisa diselesaikan | Terima preview-only + manual grading, atau pilih provider authoritative |
| Kuota Glot hosted | Budget historis 18/18 habis; harga publik tidak terdokumentasi | Perlu token/plan baru atau self-host (berarti VPS) |
| Compile-c di API | Potensi kebocoran secret lewat `#include` | Jangan port; bila dipaksa, butuh runner terpisah tanpa secret |
| Perubahan guard completion | XP dapat diklaim tanpa eksekusi server | Tidak diusulkan; melanggar invariant Phase 4 |
| Aset WASM | Lisensi Apache-2.0/MPL-2.0 dan provenance | Sudah dicatat di `runtimes/v1/clang/manifest.json`; tambahkan NOTICE bila didistribusikan ulang |

## 10. Urutan implementasi

1. **L0** — optimasi browser (worker persisten, cache compile, prewarm). Tanpa
   endpoint baru, tanpa biaya, tidak menyentuh proof.
2. **L1** — endpoint fallback Glot + config + tes. Tetap `authoritative: false`.
3. **L2** — dokumentasikan keputusan: preview-only + manual grading, atau pilih
   provider authoritative untuk menutup gate.
4. **L3** — hanya bila diminta eksplisit: compile-c di runner terpisah tanpa
   secret; bukan di API function.
