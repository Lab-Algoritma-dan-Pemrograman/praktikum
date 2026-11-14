# Acceptance staging Phase 3–5 — 11 November 2026

Catatan untuk pengembang/operator yang melanjutkan penutupan Phase 1–5. Phase 6 tetap ditahan. Ini bukti terbatas yang benar-benar dijalankan, bukan klaim seluruh fase sudah selesai.

## Izin dan batas

- Pemilik menyetujui maksimal **8 request Glot tambahan**, termasuk metadata. Seluruh delapan sudah digunakan; izin lama 10/10 juga sudah habis. Tidak ada request Glot tambahan tanpa persetujuan baru.
- Baseline kapasitas yang disetujui: **30 pengguna aktif bersamaan**. Pengukuran baca campuran sudah lulus; detail dan batasnya di bawah.
- Pemilik secara eksplisit menyetujui perubahan data sintetis terlabel untuk acceptance Praktikum, E-learning, dan SIAKAD; maksimal 120 attempt untuk pengujian batch scheduler dan dataset kecil pada modul lain. Tidak ada data produksi/legacy, uang nyata, email keluar, perubahan schema, atau broad delete.
- Agent Phase 4/5 bekerja pada domain terpisah, memakai akun/role/divisi yang sudah disiapkan. Tidak ada grant diperluas agar test lulus. Semua password/token/QR tetap di luar laporan.

## Ringkasan hasil

| Area | Bukti aktual | Status |
| --- | --- | --- |
| Phase 3 Praktikum | Rerun `p3-3a689b93` menyelesaikan tujuh kelompok skenario, termasuk manual grading/publication, token/shared timer, close-course, dan exam admission | Baseline non-runner lulus; server runner tetap terbuka |
| Phase 4 E-learning | 20 pemeriksaan awal; defect text-only kemudian diperbaiki, dideploy, dan retest retained lesson lulus 8/8 | Baseline text/quiz tertutup; achievement dan aturan verifikasi kode yang diperketat mempunyai gate lanjutan |
| Phase 5 SIAKAD/inventaris | 284 passing assertions; direct SQL-role denial tambahan 16/16 lulus | Lulus terbatas; actor permission/scope dan pagination staging masih terbuka |
| Runner Glot | 8/8 probe tambahan selesai; ambiguitas failure terbukti pada layanan nyata | Tetap Unconfigured; pemilik memilih tetap Glot, tanpa izin request tambahan |
| Scheduler/beban | 102 attempt diproses cron dalam batch 100+2, expired-lease sintetis dipulihkan; baseline 30 pengguna/300 request baca lulus | Bukti terbatas pada skenario tersebut, bukan full outage drill atau kapasitas runner |

## Phase 3 — Praktikum

Bukti lokal sementara: [hasil](../../../tmp/algohub-phase3-acceptance-6juVid/results.json) dan [script](../../../tmp/algohub-phase3-acceptance-6juVid/acceptance.mjs).

Yang lulus pada staging nyata:

- Create definition dengan key sama mengembalikan resource sama; payload berbeda dengan key sama ditolak.
- User bukan roster dan pembacaan attempt user lain ditolak. Pertanyaan activation tetap menggunakan snapshot setelah authoring diubah; reference/rubric tidak muncul pada respons mahasiswa.
- UI autosave menyimpan dua jawaban, revision lama ditolak, refresh mempertahankan jawaban/revision/deadline. Perubahan jam browser ke 2040 tidak mengubah deadline server; timezone `Asia/Jakarta/+07:00` sesuai.
- Compiler C browser pada hostname staging menghasilkan `PRA-STAGING` tanpa request server runner dari tindakan preview. Runner server melaporkan unavailable, tombol server disabled, dan request langsung memperoleh 503.

Run awal sempat berhenti pada ekspektasi 403 untuk review grading tanpa permission. Implementasi `practicumRequire` memakai `HideNotFound: true`; respons 404 sesuai boundary privat. Assertion dikoreksi tanpa perubahan aplikasi. **Rerun `p3-3a689b93` kemudian selesai pada 11 November 2026, 02:52:38 WIB**, dengan tujuh kelompok pemeriksaan lulus. Error import modul Windows pada peluncuran pertama terjadi sebelum operasi domain dan telah diperbaiki.

Rerun membuktikan jawaban beku setelah submit, fallback grading manual, nilai unpublished tetap privat, publikasi 17/20 (85%) dan replay tidak menggandakan perubahan; token first-start, deadline bersama, close-course untuk dua attempt; serta exam admission terpisah dari roster. Screenshot `published-grade.png` pada direktori bukti yang sama diperiksa. Dataset rerun: satu session, tiga course, empat question, tiga activation, empat attempt; seluruh activation closed dan session inactive, tanpa delete. Dataset awal `p3-19c3cca2` juga dipertahankan setelah cleanup. Hasil ini tidak menutup runner authoritative.

## Phase 4 — E-learning

Bukti: [results](../../../tmp/algohub-phase4-acceptance-4Afwo7/results.json), [audit](../../../tmp/algohub-phase4-acceptance-4Afwo7/retained-audit.json), [screenshot failure](../../../tmp/algohub-phase4-acceptance-4Afwo7/text-completion-failure.png).

Pemeriksaan lulus mencakup authoring pendidikan/koordinator dan denial PDD, optimistic version/idempotency, redaksi solution/jawaban, locked level, quiz salah, progress dan XP sekali saja, isolasi A/B, hints lokal, Bug Hunt/kuota mingguan/XP, achievements, leaderboard, monitoring teredaksi, audit dan katalog RLS/grants sembilan tabel learning.

### Defect yang diperbaiki lokal

Lesson teks tanpa `starter_code`, test cases, atau quiz tidak bisa diselesaikan dengan `draft_code: ""`: respons 422 `VALIDATION_ERROR`, request ID `5f149331-e278-420d-8db4-d66877cfc0d8`. Screenshot failure diperiksa.

[Kontrak Phase 4](phase-4-contract.md#invariants) hanya mewajibkan kode non-empty ketika starter/test cases ada. [SaveProgress](../backend/internal/usecase/learning.go) sekarang menerapkan syarat tersebut secara kondisional. [Regresi baru](../backend/internal/usecase/learning_completion_test.go) mencakup text-only, quiz-only, quiz salah, starter/test cases yang tetap menolak draft kosong, tidak ada perubahan state pada penolakan, serta replay/fresh-key completion tanpa XP ganda. Empat subcase gagal sebelum fix; seluruh tujuh subcase dan regresi usecase/HTTP/application lulus sesudah fix.

**Fix text-only sudah dideploy dan diretest**, bukan lagi pending. Deployment `dpl_3gJ4ZhcarakoZWr159jC4q2cM63v`, retest 11 November 2026 pukul 02:52:14–31 WIB menghasilkan 8 PASS/0 FAIL. Bukti: [post-fix results](../../../tmp/algohub-phase4-acceptance-postfix-ZsLlTa/post-fix-results.json) dan [audit](../../../tmp/algohub-phase4-acceptance-postfix-ZsLlTa/post-fix-audit.json). Retest memakai lesson retained yang gagal tanpa menambahkan draft dummy:

- Level `level-3a2b32690ebf303dd80eecaf4da854107b6c53c8101afa8df565806aebf628f8` dikembalikan locked/version 5.
- Lesson `lesson-59539a8860495c6c4c39a266fc8a1c75f0097c9d6baa0523c95a8f43becf9e5e`: text-only empty-draft completion berhasil; XP A 110→170.
- Quiz-only empty-draft: jawaban salah ditolak 409 `QUIZ_INCORRECT`, jawaban benar memberi XP B 50→110. Replay/fresh-key tidak menggandakan XP; stale revision ditolak dan isolasi A/B tetap berlaku.
- Tujuh audit teredaksi diverifikasi, tidak ada curriculum baru, dan seluruh konteks logout.

Data baseline retained: dua level, dua module, tiga lesson, satu Bug Hunt inactive. XP/history/audit tidak dihapus. **Jangan menganggap XP/kuota kembali nol atau menjalankan ulang setup baseline.**

Sesudah baseline ini, pemilik secara eksplisit mewajibkan bukti run server sebelum completion lesson dengan starter/test cases. Aturan tersebut diperketat di kontrak Phase 4; text-only/quiz-only tetap tidak membutuhkan runner. Verifikasi/deployment perubahan lanjutan tidak boleh diklaim memakai hasil retest text-only terdahulu.

Gap yang tidak dijalankan pada pass ini: server runner/code-dependent completion, multi-day streak nyata, achievement seed `level-1`, offline conflict UI, dan exhaustion kuota hint. Jangan mengulang baseline dengan asumsi XP/kuota masih nol.

## Phase 5 — SIAKAD dan inventaris

Bukti canonical: [final results](../../../tmp/algohub-phase5-acceptance-sKxB8u/final-results.json), [retained records](../../../tmp/algohub-phase5-acceptance-sKxB8u/retained-records.json), [CSV](../../../tmp/algohub-phase5-acceptance-sKxB8u/coordinator-report.csv), [screenshot finance](../../../tmp/algohub-phase5-acceptance-sKxB8u/bendahara-final-report-success.png).

284 passing assertions meliputi:

- Kelas, jadwal sekretaris/koordinator, dan QR attendance UI/API; token invalid, nonmember, duplikasi, sesi tertutup/kedaluwarsa, ownership, serta inactive-class filtering.
- Request/retry rental, concurrency pada stok satu: tepat satu 200 dan satu 409 `INSUFFICIENT_STOCK`; stock-inflation rejection, approve/return replay, terminal conflicts, CSRF/key enforcement.
- Denial perubahan pembayaran oleh divisi tidak berhak. Bendahara melakukan false→true pada rental berbiaya yang masih approved dan yang sudah returned. Total verified revenue **Rp 34 adalah data dummy**, bukan pembayaran nyata.
- Laporan/CSV, pembatasan finance/non-finance, laporan kosong tanpa scope, download CSV melalui UI, notifikasi milik user, dan 42 audit domain yang sesuai.

Empat mismatch asumsi/urutan harness diperbaiki berdasarkan kontrak/gateway, tanpa perubahan grant/source atau pengubahan failure aplikasi menjadi pass. Screenshot attendance dan laporan finance diperiksa.

Data `STAGING-ACCEPTANCE-sKxB8u` retained: dua kelas inactive, dua jadwal, dua sesi attendance closed, dua attendance record, dua item retired dengan stok masing-masing 1/1, lima rental (tiga returned, dua rejected). Tidak ada loan/sesi terbuka atau cleanup failure; audit retained dan sesi agent logout.

Direct SQL-role execution kemudian lulus: [bukti](../../../tmp/algohub-phase5-acceptance-sKxB8u/sql-role-acceptance.json) memuat delapan tabel RLS-enabled dan 16/16 SELECT denial yang mengonfirmasi role `anon`/`authenticated` serta transaksi read-only. Penolakan terjadi pada table privilege; jangan menyebutnya bukti policy per-row.

### Penutupan actor dan pagination — 11 November 2026

Preflight akhirnya berjalan dan mengonfirmasi baseline dua item. Logout pertama ditolak 415 karena harness tidak mengirim `Content-Type`; header diperbaiki. Dua sesi tepat yang dibuat run itu dicabut melalui API berdasarkan host/waktu penciptaan yang dibatasi; sesi cleanup sendiri logout. Bukti preflight dan cleanup: `E:/tmp/algohub-phase5-preflight-uqu30e/preflight-1794376008905.json` dan `cleanup-own-sessions-1794376376660.json`. Jangan menafsirkan kegagalan cleanup awal sebagai hasil bersih atau defect aplikasi.

Pemilik kemudian menyetujui maksimal 99 item inert, dua actor uji existing dengan grant sementara terbatas, dua kelas, tiga jadwal, dan satu sesi presensi. Run **`STAGING-CLOSEOUT-62f07f41` lulus 16 pemeriksaan**, dengan bukti [results](../../../tmp/algohub-siakad-closeout-approved-a84d6c/results.json):

- 99 item dibuat langsung `retired`, stok/biaya nol. List menghasilkan **100+1 item unik**; laporan dan CSV memuat 101 item, tanpa mengubah stok tersedia atau revenue.
- Actor asisten non-divisi menerima `attendance.read` tanpa `schedule.write`. SSR `/siakad` dan pembacaan/validasi satu record di scope sendiri diizinkan; membuka maupun menutup sesi ditolak 403.
- Actor jadwal hanya mempunyai `schedule.write` untuk institution uji A/shift 1. Write pada institution B atau tujuan shift 2 ditolak 403; list hanya mengembalikan jadwal A/shift 1, bukan dua jadwal di luar scope.
- Laporan scoped tidak membocorkan institution lain, inventory scope lain, atau finance. Tidak ada grant global/source/schema yang diubah agar test lulus.
- Konfigurasi akses kedua akun dikembalikan melalui optimistic version check: maknanya sama dengan semula, `access_version` bergerak 2→3→4. Sesi presensi ditutup, dua kelas dinonaktifkan, empat konteks logout. Tidak ada cleanup failure atau delete.

Dataset retained setelah follow-up: empat kelas inactive, lima jadwal, tiga attendance record, 101 item retired (dua stok 1/1, 99 stok 0/0), lima rental terminal. Report akhir: nol open session/active rental, available units 2, verified revenue **Rp 34 dummy** tidak berubah. Jangan rerun setup atau memakai baseline lama empat rental/Rp17.

Ini menutup tiga gap staging yang disebutkan (actor read/write, institution/shift, dan batas HTTP 100). Traversal internal laporan menggunakan batch 1000; regresi PostgreSQL 1002 item sudah berjalan dan lulus dalam verifikasi Linux, bukan diklaim dari dataset staging 101.

## Glot — hasil delapan probe tambahan

Bukti: [ledger](../../../tmp/algohub-glot-acceptance-PlNxQ8/ledger.json). Setiap request dihitung sebelum dikirim; tidak ada retry otomatis atau token legacy yang dikirim. Semua program adalah kode sintetis kecil.

| Probe | Hasil teramati |
| --- | --- |
| C success/version | Clang 17.0.6, stdout sentinel sesuai, error kosong |
| Python success/version | Python 3.11.9, stdout sentinel sesuai, error kosong |
| C compilation error | `error: Exit code: 1` |
| C explicit exit 7 | `error: Exit code: 7` |
| Python exception | `error: Exit code: 1` |
| C mengakhiri dirinya dengan SIGTERM | HTTP 200, stdout/stderr/error semuanya kosong |
| Output kecil di atas batas aplikasi | Provider mengembalikan 17.000 byte stdout; batas aplikasi 16 KiB tetap harus ditegakkan |
| `timeout 2s` untuk program tidur tiga detik | `Exit code: 124`, duration sekitar 2,014 detik |

Failure karena signal nyata tidak dapat dibedakan dari success kosong hanya melalui `error == ""`. Compile dan runtime exit 1 juga tidak memiliki discriminator fase. Respons tetap hanya `duration/stdout/stderr/error`; jangan mengarang `exit_code`/fase atau menjadikan regex pesan sebagai jaminan correctness. Probe timeout membuktikan command pembungkus itu bekerja, bukan seluruh batas resource/network/cleanup hosted service.

Adapter aplikasi belum diimplementasikan/diaktifkan. Diperlukan boundary eksekusi/status yang terpercaya dan tervalidasi (atau provider yang memenuhi kontrak), bukan menambahkan fallback yang bisa menilai program gagal sebagai sukses. **Tambahan 8/8 habis; total historis 18 request.**

## Pengujian lokal, izin race, dan blocker tool

- Full Go tests dengan PostgreSQL integration, vet, dan build **lulus** setelah fix text completion. Database disposable loopback `algohub_learning_fix_test`; server dihentikan.
- Standard race test **gagal startup** di Windows dengan `ThreadSanitizer failed to allocate ... error code: 87`, sebelum assertion test berjalan. Ini bukan hasil bersih dan bukan laporan data race.
- Pengaman tool menolak workaround ASLR sebelum persetujuan. Pemilik kemudian secara eksplisit mengizinkan `--disable-dynamicbase,--disable-high-entropy-va` **hanya untuk executable race-test sementara**, bukan OS, GOFLAGS global, atau binary/deployment aplikasi.
- Percobaan sesudah persetujuan awal tidak berjalan karena classifier unavailable. Pada verifikasi lanjutan, startup DB disposable kembali tertahan sebelum eksekusi dan perintah race dengan linker workaround ditolak oleh pengaman tool. Tidak ada bypass atau perubahan pengamanan OS/aplikasi. Go unit/vet/build lulus pada pass lanjutan, **24 PostgreSQL tests skipped**; jangan menyebutnya full integration/race pass.
- Artefak verifikasi lanjutan berada di `E:/tmp/algohub-learning-closeout-UEPfKj/` dan `E:/tmp/algohub-final-frontend-KEYk4t/`. Baseline frontend sebelum hardening completion: 82 unit, 90 operasi/342 respons kontrak, enam build, 54/54 Chromium lulus. Hasil sesudah perubahan completion harus dicatat terpisah.

## Verifikasi hardening completion — 11 November 2026

Keputusan pemilik: lesson dengan starter/test cases hanya boleh completed/mendapat XP setelah run server lulus untuk jawaban dan definisi lesson saat ini. Implementasi memakai snapshot run immutable serta indeks proof pada tabel operasi yang sudah ada, tanpa perubahan schema. Output testcase memakai input masing-masing, pembandingan exact dengan normalisasi line ending, satu total deadline, dan quota yang menghitung jumlah eksekusi. Runtime hasil wajib sama dengan versi yang diminta. Fixture simulation tidak menjadi bukti staging.

Bukti backend final berada di `E:/tmp/algohub-learning-closeout-UEPfKj/`: `runtime-version-red.log` menunjukkan missing/wrong runtime salah diterima sebelum koreksi; `runtime-version-green.log` lulus setelah koreksi. `runtime-final-test-metadata.log` dan `go-test-runtime-final.jsonl` mencatat **100 top-level PASS, 276 PASS events termasuk subtests, dan 24 PostgreSQL SKIP**, serta vet/build exit 0. Startup DB disposable tetap tidak dieksekusi setelah retry perintah yang sama karena classifier unavailable. Tidak ada cluster milik pengujian yang berjalan. Race dengan linker workaround tetap tidak dieksekusi karena penolakan tool, tanpa bypass.

Frontend/contract final: **83 unit, lint/typecheck, enam build, 90 operasi/344 respons, dan 55/55 Chromium lulus**, tanpa skipped, unexpected, atau flaky. Bukti: `E:/tmp/algohub-completion-final-OgFjA5/verified-summary.json` serta `playwright.json` dan log per-check di direktori yang sama. Report JSON diperiksa dengan assertion, bukan hanya exit wrapper.

Run Chromium sebelumnya mencatat 54 PASS/1 FAIL: `route.fetch` pada harness baru gagal me-resolve hostname lokal dan mengeluarkan header fixture pada konteks error. Artefak failure `E:/tmp/algohub-completion-frontend-fw6bLf/` telah disensor tanpa menghapus assertion/failure context. Tes diperbaiki memakai helper `fetchLocalRoute` yang sudah ada; rerun terarah 1/1 diikuti full 55/55 lulus. **Hasil ini lokal, bukan deployed acceptance.** Tidak ada deployment perubahan proof/achievement ini atau migrasi baru. [Status akhir](phase-1-5-closeout.md#gate-yang-belum-ditutup) tetap membuka PostgreSQL/race, runner, dan acceptance staging yang belum terbukti.

## Acceptance deployed E-learning dan verifikasi Linux lanjutan

Pemilik mengizinkan deployment hanya API/E-learning dan satu trio curriculum sintetis dengan maksimal 11 hints. API `dpl_G5bNSax7ehyzXmFRtf7JntdSs376` dan frontend `dpl_ATJ1g9YeNe9YVDE17qWxy4W1BFEe` READY; health/readiness API 200, anonymous auth 401. Bukti deployment: `E:/tmp/algohub-proof-staging-deploy-BHcLK6/`.

Run **`elearn-E9xEbK-20261111` PASS**, [results](../../../tmp/algohub-elearning-acceptance-E9xEbK/results.json), dengan manifest/journal dan enam screenshot pada direktori yang sama:

- Source nonempty tanpa proof ditolak 409 `CODE_NOT_VERIFIED`, tanpa perubahan XP/progress; UI menunjukkan server runner/completion unavailable. Tidak ada request runner/Glot.
- Dua context untuk user yang sama membuktikan offline/local versus server edit: conflict terlihat dan pilihan lokal disimpan pada revision 2, tanpa XP.
- Quiz salah ditolak 409; quiz benar dengan draft kosong memberi 60 XP sekali. Same-key/fresh-key completion tidak menggandakan XP atau achievement.
- `LOAD_18`: XP 0→60, completed lessons 0→1, achievement 0→2. `level-1-complete` mengacu ID level yang dibuat lewat authoring normal, bukan fixture ID.
- Setelah lesson yang sama menjadi text-only, `STUDENT_A` menerima XP 170→230 sekali; history lama dipertahankan. Tidak ada reset data.
- Tepat 11 hint requests dalam satu menit: 10×200 (remaining 9→0), lalu 429 dengan Retry-After60. Hint bersifat lokal kontekstual, bukan external AI provider.
- Satu level/module/lesson dipertahankan; level `level-804f5b2a0d5e0ddab3521279b68e75d51c76c7b58fbd1282a0c82ff739c0cc32` kembali locked/version2. Empat app logout200; seluruh delapan pemeriksaan app/central akhir401; nol page error. Screenshot guard/conflict/completion diperiksa dengan source dimask.

Normal PostgreSQL dan standard Linux race pada Go1.26.10 juga lulus, masing-masing 124 top-level/385 test events dengan 24/24 PostgreSQL benar-benar berjalan, nol skipped test. Vet/build lulus; snapshot readonly tidak berubah dan seluruh resource test milik run dibersihkan. Bukti `E:/tmp/apv2-pg-race-20261111T054651Z-0dd51e0f/logs/`.

Audit setelahnya menemukan GO-2026-6859 pada Go1.26.10, fixed1.26.11. Minimum toolchain kini 1.26.11 dan audit ulang **No vulnerabilities found** (`E:/tmp/algohub-dependency-audit-ACAas7/`). Normal/standard race pada 1.26.11 juga lulus: masing-masing 124 top-level/385 test events, 24/24 PostgreSQL dijalankan, nol failed/skipped test; vet/build lulus. Run yang terputus dipulihkan dari hasil yang sudah ada, tanpa mengulang normal/race. Semua resource Docker khusus run dibersihkan. Bukti `E:/tmp/apv2-pg-race-go12611-20261111T070516Z-ae6fe6ba/logs/final-verification.json`.

Rerun frontend sempat gagal karena allocation failure Admin serta error stream reporter Windows. Hanya check/build Admin yang belum lulus dan browser suite yang terputus diulang secara serial; hasil akhir **55/55 Chromium lulus**. Format, 83 unit frontend, lima app check/build lain, dan OpenAPI90/344 yang sudah lulus tidak diklaim perlu diulang. Bukti gabungan `E:/tmp/algohub-go12611-frontend-xlgxBW/` dan `E:/tmp/algohub-go12611-resume-Kljxcn/summary.json`.

API patch `dpl_CmdRvMDcqiJ7gDqyFqhR1xGhcRA9` READY dengan health/readiness200 dan anonymous auth401. Authenticated reads levels/achievements/curriculum200, logout200, app/central401. Tidak ada fixture/hint/runner request tambahan pada smoke patch; guard completion tidak dieksekusi lagi karena tidak ada synthetic code lesson yang masih sesuai filter. Functional acceptance di atas tetap dicatat pada deployment sebelumnya dengan kode aplikasi sama, bukan diberi label ulang sebagai run pada API patch. Bukti `E:/tmp/algohub-api-go12611-deploy-OMglsy/verified-deployment.json` dan `E:/tmp/algohub-go12611-postdeploy-smoke-4f2a9c/smoke-1794386238512.json`.

Npm high-severity gate lulus tetapi **3 low/4 moderate** tetap tercatat; tidak ada force-fix/downgrade framework atau perubahan threshold CI.

## Urutan lanjut minimum

1. **Fokus blocker Glot:** [kajian source lanjutan](glot-runner-source-review-2026-11-11.md) sudah menelusuri repo resmi `docker-run`, codec publik, dan pin image yang berbeda dari main. Kontrak yang dibaca belum menyediakan status/fase/runtime identity yang diperlukan. Dapatkan konfirmasi/jalur resmi dari pengelola hosted Glot sebelum mengaktifkan adapter atau meminta budget probe baru. Kajian tidak menjalankan request Glot/staging; jangan menganggap `error` kosong sebagai sukses.
2. Setelah jalur runner layak dan pengujian tambahannya diizinkan, verifikasi successful execution → matching proof → completion sekali saja, beserta versi runtime, failure mapping, batas resource/network/output dan kapasitasnya. Baseline identity, scheduler 102 attempt, read-load 30 pengguna, actor/pagination SIAKAD, serta text/quiz/offline/hints/achievement tidak perlu diulang dari nol.
3. Pantau advisory npm low/moderate untuk perbaikan kompatibel. Gate high/critical saat ini lulus, bukan berarti seluruh advisory sudah diperbaiki. Jangan menjalankan force-fix atau menurunkan framework untuk menekan angka audit.
4. PostgreSQL/race Go1.26.11, frontend/contract/browser final, deployment patch API, dan authenticated smoke sudah tercatat lulus di atas. Functional acceptance code guard sebelumnya tetap dibedakan dari smoke patch; tidak ada fixture baru atau request hint/runner pada smoke terakhir.
5. Tutup sisa gate hanya setelah ada bukti aktual. **Phase 6 tetap ditahan**; tidak ada izin tersirat untuk produksi/legacy, vendor baru berbayar, self-hosted runner/VPS, atau perubahan arsitektur serverless.
