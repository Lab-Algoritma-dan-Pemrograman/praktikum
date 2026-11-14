# Code Runner dan Terminal Plan

**Status:** Target baseline berdasarkan audit E-learning dan Praktikum

## 1. Temuan legacy

### E-learning

- Python memakai Pyodide Worker di browser.
- C memakai Clang/WASM Worker dengan virtual filesystem.
- Source `useCodeRunner.ts` juga memuat adapter Judge0/Wandbox, tetapi jalur C
  yang aktif saat ini memakai `cWorker`; provider fallback harus diverifikasi,
  bukan diasumsikan aktif.
- `test_cases` dan `validation_rules` disimpan pada lesson.

### Praktikum

- `pkg/glot` mem-proxy Glot.io untuk C/Python.
- `pkg/cwasm` mengompilasi C ke wasm32-wasi dengan timeout, temp directory, dan
  semaphore; komentar source menegaskan server bukan sandbox eksekusi penuh.
- Endpoint lama adalah `/api/praktikum/run` dan `/api/praktikum/compile-c`.

## 2. Keputusan target

| Use case | Runtime target | Alasan |
|---|---|---|
| latihan interaktif ringan | WASM browser | cepat, offline-friendly, tidak membebani API |
| run/test untuk penilaian | external sandbox provider melalui Go API | sandbox dan timeout terpusat; provider tidak terekspos |
| compile C preview | WASM browser atau compile provider terkontrol | dipilih per lesson dan capability browser |
| batch grading | API function bounded + provider/queue managed | Vercel tidak menjalankan worker persisten |

Go API menjadi satu-satunya gateway. Provider dapat diganti melalui interface
`Runner`, sehingga Judge0/Glot/self-hosted runner bukan dependency business logic.

## 3. Batas wajib

- source maksimal 64 KB untuk interactive run;
- stdin, stdout, stderr, dan result JSON memiliki limit;
- timeout compile/run dan total request timeout;
- quota per user/session/route dan concurrency cap;
- bahasa dan compiler version allowlist;
- filesystem ephemeral; tidak ada network dari program mahasiswa;
- secret provider hanya backend;
- provider error dibedakan dari compile error dan runtime error;
- output disanitasi saat ditampilkan, tanpa `{@html}` dari runner;
- submission final menyimpan input, output ringkas, status, compiler version, dan
  hash source untuk reproducibility; source penuh mengikuti retention policy.

## 4. Kontrak hasil

```json
{
  "status": "success|compile_error|runtime_error|timeout|quota_exceeded|provider_error",
  "stdout": "...",
  "stderr": "...",
  "exit_code": 0,
  "duration_ms": 123,
  "runner_version": "allowlisted-version",
  "request_id": "req_..."
}
```

## 5. Test runner

- unit: language allowlist, request validation, output truncation, timeout mapping;
- integration: fake provider, retry policy, provider outage, duplicate idempotency;
- security: shell injection payload, network/file escape attempt, oversized output,
  malicious source, cross-user result access;
- E2E: editor → run → stdin → result → test-case feedback pada mobile dan desktop;
- load: burst concurrency dan 429 tanpa membuat DB connection storm.
