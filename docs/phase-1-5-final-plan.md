# Rencana penutupan final Phase 1–5

Status: direvisi 12 November 2026 setelah keputusan $0 browser-trusted. Implementasi, regresi penuh, deployment staging, dan acceptance revisi sudah mempunyai bukti; Phase 1–5 pada scope revisi **100% selesai**. Phase 6 belum dimulai.

## 1. Keputusan final

Rujukan mengikat: [keputusan completion browser E-learning](elearning-browser-completion-decision-2026-11-12.md).

| Area | Keputusan |
| --- | --- |
| C | Clang/WASM di browser |
| Python | Pyodide di browser |
| E-learning code completion | Testcase browser dipercaya untuk XP gamifikasi; risiko DevTools diterima pemilik, tim asisten, dan dosen |
| Glot | Fallback preview opsional, `authoritative:false`, tanpa proof/nilai |
| Judge0/RapidAPI | Dibatalkan; bukan dependency dan bukan pekerjaan tertunda |
| Praktikum | Browser hanya preview; submission, review, dan publikasi nilai tetap manual |
| Biaya runner | $0 |

Keputusan ini menggantikan workstream Judge0/RapidAPI sebelumnya. Provider authoritative hanya boleh dibahas kembali melalui keputusan tertulis baru. Frozen `plan/**` tetap arsip; dokumen ini dan closeout adalah tracker aktif.

## 2. Trust boundary

Browser boleh menyatakan `browser_test_passed=true` hanya untuk completion/XP E-learning. Assertion client diikat oleh UI ke lesson ID, lesson version, language, dan source snapshot, tetapi pengguna yang memodifikasi client tetap dapat memalsukannya. Sistem tidak boleh mengklaim anti-cheat atau correctness authoritative.

Backend tetap wajib memeriksa autentikasi, akses lesson, quiz, language, source nonblank, current lesson version, optimistic revision, idempotency, dan exactly-once XP. Keputusan client-trust dilarang untuk nilai Praktikum, ujian, admission, sertifikasi, atau publikasi akademik.

## 3. Workstream implementasi

1. Jalankan testcase lesson secara sequential melalui runtime browser existing; exact output hanya menormalisasi CRLF dan newline terminal.
2. Invalidasi attestation ketika source/language/lesson/version berubah, run gagal/cancel, conflict mengganti draft, atau navigasi.
3. Kirim `browser_test_passed` dan `lesson_version` hanya pada request completion; autosave tidak membawa klaim lulus.
4. Terima assertion secara sempit di `SaveProgress`, tanpa durable runner proof; pertahankan transaction, revision, audit, dan XP tepat sekali.
5. Hapus adapter/config/env/wiring Judge0. Pertahankan generic runner/fixture/lease sebagai compatibility seam dan Glot sebagai preview-only.
6. Hapus dependency UI E-learning pada `runner.available`; Praktikum tetap memperlihatkan browser preview dan menyembunyikan server-run unavailable tanpa mengubah grading manual.
7. Sinkronkan OpenAPI, unit/integration/E2E, dokumentasi, dan handover.

Tidak ada migrasi schema, provisioning dataset, impor legacy, project baru, atau perubahan produksi dalam workstream ini.

## 4. Acceptance

### Lokal

- Format, Svelte/typecheck, seluruh unit frontend, enam build.
- Seluruh Go test, vet, build, dan OpenAPI live contract.
- PostgreSQL integration dan standard race pada database disposable; kegagalan host sebelum test berjalan harus dilaporkan, bukan dianggap pass.
- Chromium real C/Python: browser testcase pass/fail, strict output, source/version invalidation, offline/reconnect, retry ambigu, XP tepat sekali, text/quiz tetap bekerja, dan Praktikum manual-only.
- Tidak ada request `/learning/playground/run` saat code lesson diselesaikan melalui browser.

### Staging existing

- Deploy hanya API, E-learning, dan Praktikum project staging existing sesuai perubahan aktual.
- Reuse akun dan lesson sintetis retained; jangan provision dataset scheduler atau akun baru.
- Buktikan C/Python browser completion, duplicate retry tanpa XP ganda, text/quiz, Glot non-authoritative, logout, dan Praktikum manual grading tetap utuh.
- Periksa nama environment tanpa mencetak value; tidak boleh ada env Judge0/RapidAPI.

## 5. Scheduler dan kapasitas

Scheduler batch 20 sudah dideploy dan memproses 120 job retained melalui enam delivery cron nyata dalam 5 menit 9,265 detik. Final verification membuktikan 120 `needs_manual`, nol pending/running/lease/score, session inactive, 10 activation closed, cron aktif, dan actor logout. Satu expired lease adalah injeksi sintetis, bukan outage nyata. Detail ada di [runbook scheduler](staging-scheduler.md#acceptance-batch-20-pada-staging--12-november-2026).

Bukti ini menutup acceptance scheduler yang disepakati, bukan klaim provider load atau outage authoritative runner—karena provider authoritative telah ditarik dari scope.

## 6. Status gate

- 12 gate lama mempunyai bukti dan tetap tercatat di [closeout](phase-1-5-closeout.md).
- Tiga gate authoritative-runner lama **tidak ditandai lulus**. Statusnya `superseded/withdrawn by approved product-risk decision`:
  1. conformance hosted authoritative runner;
  2. hosted execution → proof → completion;
  3. capacity/outage server runner end-to-end.
- Penutupan revisi dinyatakan selesai setelah implementasi, regresi penuh, dan acceptance staging browser-trusted lulus; seluruhnya sudah terbukti.
- Phase 6 tidak dimulai otomatis. Diperlukan instruksi terpisah dan otorisasi scope data legacy untuk migration rehearsal.

## 7. Rollback

Jika browser completion bermasalah, rollback UI/API ke deployment staging sebelumnya. Jangan mengaktifkan Judge0 atau mengirim source ke provider lain sebagai fallback diam-diam. Browser preview dan manual grading Praktikum harus tetap tersedia; XP yang sudah diberikan tetap immutable dan tidak dikurangi otomatis.
