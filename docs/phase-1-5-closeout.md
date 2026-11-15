# Penutupan Phase 1–5

> **Keputusan aktif 12 November 2026:** [browser-trusted E-learning completion](elearning-browser-completion-decision-2026-11-12.md) telah disetujui pemilik bersama tim asisten dan dosen. Judge0/RapidAPI dibatalkan; bagian provider authoritative di bawah dipertahankan sebagai sejarah, bukan Next Steps. Praktikum tetap manual dan Phase 6 tetap ditahan.

## Batas pekerjaan

Hanya menutup implementasi dan acceptance Phase 1–5. Inisialisasi schema staging bukan migration rehearsal Phase 6: tidak ada data legacy yang diimpor, tidak ada cutover, dan project produksi tidak diubah.

## Staging — 10 November 2026

Pemilik project mengonfirmasi bahwa credential pada `.env.staging.local` menunjuk project Supabase baru khusus rewrite, kemudian menyetujui penerapan schema awal setelah preflight read-only.

- Flag E-learning dan SIAKAD yang belum ada sudah ditambahkan pada file lokal.
- URI database lokal diberi `sslmode=require`, kemudian endpoint transaction pooler pada host/project yang sama diverifikasi read-only sesuai dokumentasi Supabase. Setelah seluruh checksum cocok, runtime dipindahkan ke port 6543; URI session pooler semula dipertahankan pada `ALGOHUB_MIGRATION_DATABASE_URL` khusus migrator. Tidak ada pergantian project, username, atau password.
- Preflight menemukan schema Auth, nol tabel aplikasi pada `public`, dan belum ada `schema_migrations`.
- Migrasi `0002_identity.sql` sampai `0007_siakad_inventory_core.sql` sudah diterapkan pada target yang disetujui.
- Verifikasi read-only memastikan seluruh enam nama/urutan/checksum cocok dengan source lokal, 38 tabel publik mengaktifkan RLS, dan tidak ada grant langsung ke tabel domain Praktikum/E-learning/SIAKAD untuk `anon`, `authenticated`, atau `PUBLIC`.
- Jumlah profile aplikasi masih nol. Tidak ada akun fixture atau data legacy yang diimpor.
- Migrator selesai dengan exit 0. Pembacaan verifikasi pertama gagal; pengulangan khusus read-only berhasil. Migrasi tidak dijalankan ulang untuk mengatasi kegagalan pembacaan itu.
- TLS diwajibkan untuk koneksi klien ke pooler. `pg_stat_ssl` pada backend pooler melaporkan `false`; pemeriksaan tersebut tidak dipakai untuk mengklaim TLS pada seluruh hop internal provider.

Credential tidak dicetak, tidak disalin dari project legacy, dan tidak ditambahkan pada contoh environment.

## Runner: mengikuti jalur aktif legacy

Source legacy dibaca tanpa perubahan:

- `project_ap/elearning/src/hooks/useCodeRunner.ts`: Python memakai Pyodide Worker, C memakai Clang/WASM Worker. Fungsi Judge0/Wandbox masih ada, tetapi bukan jalur yang dipanggil hook aktif.
- `project_ap/praktikum/frontend/src/lib/components/CodeEditor.svelte`: browser runner menjadi jalur utama; endpoint backend `/api/praktikum/run` dipakai sebagai fallback.
- `project_ap/praktikum/backend/pkg/glot/client.go`: fallback memakai Glot.io. Native `cwasm` juga ada di backend legacy, tetapi kompilasi native tidak dipindahkan ke Go API serverless.

Tidak ada pemilihan/aktivasi Vercel Sandbox atau layanan berbayar baru.

### Preview browser rewrite

- Satu implementasi UI bersama dipakai pada editor Praktikum, code lab pelajaran, dan playground E-learning.
- Runtime dimuat hanya setelah klik preview, bukan saat login, membuka landing page, atau mengetik.
- Compiler C legacy disimpan satu kali pada aset statis portal, bukan diduplikasi ke setiap aplikasi. Manifest mencatat ukuran dan SHA-256 setiap aset; total aset C adalah 60.379.697 byte sebelum kompresi.
- Python menggunakan URL Pyodide `v0.25.0` yang dipin, sama dengan legacy. Pada runtime v3, paket yang diimpor user dimuat dengan `loadPackagesFromImports` dari distribusi Pyodide yang dipin (misalnya numpy/matplotlib); tidak ada micropip atau instalasi pip arbitrer.
- Eksekusi berada di Worker di dalam iframe dengan sandbox `allow-scripts`, tanpa `allow-same-origin`. Preview tidak menerima cookie, token, profile, maupun hasil penilaian.
- Satu preview aktif per halaman, batas source 64 KiB, stdin 16 KiB, stdout/stderr masing-masing 16 KiB, bootstrap 60 detik, eksekusi user 10 detik, dan pemuatan paket Python 120 detik terpisah. Timeout/cancel menghentikan worker; resource dibersihkan saat komponen terakhir dilepas.
- Input interaktif Python memakai message round-trip tanpa SharedArrayBuffer; `input()` top-level dibungkus sebagai awaitable, sehingga tetap bekerja di iframe non-cross-origin-isolated. `input()` di dalam fungsi sinkron tetap tidak didukung, sama seperti wrapper legacy. Input interaktif C tetap membutuhkan SAB dan karena itu belum aktif pada iframe bersarang saat ini.
- `plt.show()` pada matplotlib menghasilkan PNG melalui channel gambar terpisah (maksimal 4 gambar, 512 KiB per gambar) dan dirender di panel preview; stdout tetap teks biasa.
- Worker Python dihentikan setelah setiap hasil agar callback/background task dari kode pengguna tidak terus berjalan. Worker C dipakai ulang untuk compiler, tetapi callback animasi/asinkron legacy tidak diizinkan pada preview sinkron ini. Kegagalan bootstrap iframe dapat dicoba ulang secara manual tanpa memuat ulang aplikasi.
- Tidak ada retry/fallback server otomatis. Tombol gateway server tetap terpisah, dengan otorisasi, kuota, dan idempotency lama.
- Hasil browser hanya preview. Hasil ini tidak ditambahkan sebagai bukti kelulusan test case atau nilai server.

### Cache dan batas biaya

| Resource | Kebijakan |
| --- | --- |
| `/runtimes/v1/*` pada portal | Aset publik, cache browser/CDN satu tahun, immutable; CORS tanpa credential |
| Cookie, session, jawaban, nilai, dan API private | Tetap private/no-store; tidak masuk cache publik |
| Compiler C dalam Worker aktif | Dipakai ulang untuk run berikutnya tanpa fetch aset baru; worker Python baru per run memakai cache HTTP runtime publik |
| Polling attempt Praktikum | Tetap 15 detik saat aktif; berhenti pada status terminal, akses ditolak, offline/konflik/write aktif, atau tiga kegagalan pembacaan |

Setelah versi runtime dipublikasikan, jangan mengganti byte di URL immutable yang sama. Gunakan versi path baru untuk perubahan runtime. Tidak ada prewarming compiler untuk seluruh pengunjung.

Cache CDN tidak menghapus biaya transfer kepada pengguna baru. Ukuran aset pertama, kuota hosting, dan trafik cohort tetap perlu diukur; belum ada klaim bahwa penggunaan produksi pasti muat dalam kuota gratis.

## Verifikasi lokal akhir — 10 November 2026

- Formatting dan enam Svelte checks lulus, 0 error/0 warning.
- Enam frontend production build lulus. Aset runtime keluar sebagai static files pada output portal (60.387.672 byte termasuk bootstrap), tidak berada di bundle function.
- 82 frontend unit tests lulus; mencakup batas output Unicode, checksum aset, kebijakan cache, dan penghentian polling.
- Go tests tanpa database integration, Go vet, dan OpenAPI 90 operasi / 342 live response contracts lulus. Suite database fixture tidak diarahkan ke staging.
- Full Chromium E2E: **54/54 lulus**, satu worker, tanpa retry, termasuk opt-in Python runtime nyata. Empat test preview mencakup real C/stdin, reuse tanpa fetch aset baru, compiler error, output limit, timeout worker, pemulihan bootstrap, real Python, dan penghentian worker Python setelah hasil.
- Run penuh sebelumnya menghasilkan 52/54 lulus: dua test response-loss SIAKAD gagal karena `route.fetch()` memakai DNS Node untuk `*.localhost`. Helper kini menuju loopback dengan Host/cookie/CSRF/body asli tetap dipertahankan; assertion commit-once/retry tidak dihapus atau dilemahkan.
- Review menemukan worker Python yang bisa tetap hidup setelah hasil serta bootstrap retry yang tidak membuat ulang iframe; keduanya diperbaiki dan diuji.
- Akun Vercel pribadi semula hanya dibaca. Setelah pemilik beralih akun dan memberi izin operasional, target dikonfirmasi ulang sebagai `lab-algoritma-pemrograman` (Hobby, awalnya kosong). Tujuh project `algohub-v2-staging-*` telah dibuat dan seluruh deployment berstatus READY. Tidak ada upgrade billing atau perubahan project legacy.
- API pada `https://algohub-v2-staging-api.vercel.app` lulus smoke nyata: `/healthz` 200, `/readyz` 200 dengan database staging, `/v1/auth/me` tanpa cookie 401 dan `private, no-store`.
- Header aset runtime portal pada alias Vercel diverifikasi: 200, `public, max-age=31536000, immutable`, CORS `*` tanpa credential. Frontend memakai hostname staging yang dikonfigurasikan; alias Vercel bukan bypass allowlist identity.
- Build cloud portal pertama gagal karena lockfile lama belum mencatat dua workspace Phase 4/5. Lockfile disinkronkan secara offline tanpa perubahan versi dependency registry; `npm ci --dry-run` dan deployment ulang berhasil. Deployment frontend lain kemudian berhasil secara berurutan.
- Go memakai preset Vercel Go dengan composition root yang sama, dukungan `PORT` tervalidasi, dan write timeout yang tidak lebih pendek daripada request deadline. Enam frontend memakai Node 24. Function region `sin1`, deadline platform 30 detik, Fluid Compute aktif, dan pool API dibatasi empat koneksi. Build machine berada di region build bawaan provider, bukan bukti function dijalankan di sana.
- Semua domain staging telah didaftarkan pada project terkait. DNS/ownership/HTTPS custom hostname kemudian selesai diverifikasi sebagaimana catatan berikut; [record resmi DNS](staging-dns.md) tetap menjadi rujukan nilai konfigurasi.

## Verifikasi custom hostname — 10 November 2026, 23:38 WIB

- Ketujuh CNAME dan ketujuh TXT cocok pada Google, Cloudflare, serta kedua nameserver authoritative. Screenshot sebelumnya hanya memuat staging portal; pemilik kemudian menyimpan enam pasang record staging tambahan.
- Sebelum perubahan remote, akun Vercel dikonfirmasi sebagai `lordbgsdts-afk`, team `lab-algoritma-pemrograman` (`team_xEm9gJWxtM1sb09eyCeIN4js`). Verifikasi ownership enam domain yang masih pending berhasil; ketujuh domain kini `verified: true`, `misconfigured: false`, `configuredBy: CNAME`.
- Sebelum ownership diverifikasi, enam hostname baru menghasilkan `ECONNRESET` pada HTTPS dan portal utama 502. Smoke setelah verifikasi berhasil melalui HTTPS tanpa menonaktifkan validasi sertifikat:

| Target custom staging | Hasil request tanpa sesi |
| --- | --- |
| Portal, Auth Center, Praktikum, E-learning, SIAKAD `/` | 200, `no-store` |
| Auth Center `/login` | 200, `no-store` |
| Admin `/` | 401, `no-store` |
| API `/healthz` dan `/readyz` | 200, status `ok` dan `ready` |
| API `/v1/auth/me` | 401 `UNAUTHENTICATED`, `private, no-store` |
| Portal `/runtimes/v1/frame.html` | 200, cache setahun immutable, CORS `*` |

Ini menutup gate DNS/ownership/HTTPS, bukan acceptance login, SSO, revocation, atau permission akun nyata. Belum ada email koordinator yang ditunjuk atau bootstrap akun dalam pemeriksaan ini. Tidak ada deploy ulang, migrasi, perubahan produksi, atau probe Glot tambahan; suite pengujian lokal tidak diulang untuk verifikasi DNS ini.

## Scheduler dan regresi akhir — 11 November 2026

- Ditemukan regresi starvation: `Reconcile` hanya mengambil lease grading kedaluwarsa ketika tidak ada job pending. Test baru gagal pada kode lama; pemulihan lease kini didahulukan, tanpa mengubah batas 100 expiration/satu grading job per request. Test juga membuktikan job pending berikutnya diproses dan job selesai tidak dieksekusi ulang.
- Setelah fix: lint/typecheck, 82 unit frontend, enam build frontend, OpenAPI 90 operasi/342 respons, dan full Chromium 54/54 (termasuk Python nyata, satu worker, nol retry) lulus.
- Seluruh Go tests dengan PostgreSQL integration, race tests, vet, dan build lulus. Database dibuat baru di loopback bernama `algohub_closeout_test`; cluster dihentikan sesudah pengujian. Race menggunakan workaround linker khusus executable test Windows yang sudah didokumentasikan, bukan perubahan pengamanan production. Tidak ada fixture ke staging.
- Pemilik secara eksplisit menyetujui aktivasi `pg_cron`/`pg_net`, secret khusus, scheduler per menit, dan redeploy API staging saja. Kedua extension tersedia pada project Supabase yang sudah digunakan; Vault sudah terpasang. Tidak ada vendor baru atau upgrade paket.
- Secret baru disimpan di file privat lokal, Vault, dan encrypted environment API staging. Script awal berhenti sebelum SQL setup karena parsing perintah psql; secret lokal yang sama dipakai pada perbaikan berikutnya, bukan dibuat atau dicetak ulang. Tidak ada cron aktif sebelum API berhasil diuji.
- API staging dengan fix dan secret berhasil dideploy: `dpl_9reikDc1XLMMe7nDtjJuhe8VLkSe`, custom hostname tetap `api.staging.algohub.web.id`. Readiness 200; cron GET 405, token kosong/salah 401, token khusus 200, `private, no-store`.
- Job `algohub-staging-practicum-reconcile` aktif setiap menit, HTTP timeout 28 detik, secret dibaca dari Vault saat pengiriman. Dua eksekusi terjadwal nyata pada **00:30 dan 00:31 WIB** menghasilkan SQL `succeeded` dan HTTP 200 `{expired:0,jobs_processed:0}`, tanpa timeout/error. Monitoring membedakan keberhasilan enqueue dari delivery HTTP.
- Provider tetap mempunyai grant PUBLIC pada antrean `pg_net`; REVOKE lewat role `postgres` tidak menghapus grant milik `supabase_admin`. Antrean memuat header sementara dalam plaintext. Batas browser yang diverifikasi adalah role `anon`/`authenticated` tanpa LOGIN, Vault tidak terbaca, serta skema `net`/`vault` tidak diekspos Data API (406 `PGRST106`). Jangan mengklaim hardening ACL antrean berhasil. Detail, batas, dan pause/resume ada di [runbook scheduler](staging-scheduler.md).
- Staging masih mempunyai nol attempt dan nol grading job saat smoke. Ini membuktikan scheduling, konektivitas, dan autentikasi pada antrean kosong; belum membuktikan burst cohort, missed-run recovery dengan data, atau kapasitas grading. Pada satu job per menit, 500 grading job membutuhkan setidaknya 500 pemanggilan cron bila tidak ada tindakan pengajar.

## Akun dan acceptance identity staging — 11 November 2026

- Pemilik menyerahkan pemilihan email/pembuatan akun staging dan mengingatkan agar admin mencakup seluruh sub-role sesuai `plan/`. Matriks diperiksa ulang: hanya empat role identity (`koordinator`, `asisten`, `mahasiswa`, `peminjam`), dengan enam divisi asisten (`sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, `pdd`). Tidak ada role baru `admin` atau `pengajar`.
- Dibuat 12 akun sintetis `.test`: koordinator, enam divisi asisten, asisten tanpa divisi, asisten berscope kelas A, dua mahasiswa, dan peminjam. Password acak disimpan hanya di `.env.staging.accounts.local`, yang dikecualikan dari Git/deployment. Tidak ada email keluar, impor legacy, atau perubahan konfirmasi email global.
- Akun Auth/identity diprovisikan atomik melalui akses DB staging yang diizinkan karena credential Admin API tidak tersedia. Satu kegagalan tipe parameter SQL di-rollback dan nol akun parsial diverifikasi sebelum pengulangan. Seluruh profile aplikasi kemudian terbentuk lewat login UI dan Supabase Auth nyata; 12 login/logout lulus dan akun baru terbukti tanpa role implisit.
- Hanya koordinator pertama dibootstrap dengan audit/access-version. Sepuluh penugasan berikutnya memakai API aplikasi terautentikasi dan ber-CSRF; penugasan PDD disimpan lewat UI Pengaturan. Default permission divisi tidak diperluas agar semua menu muncul.
- **91 pemeriksaan SSR nyata lulus**, dengan JavaScript dimatikan: tujuh route anonymous 401, lalu 12 akun × tujuh route 200/403 sesuai matriks. Screenshot koordinator/PDD diperiksa; PDD hanya mempunyai Ringkasan/Profil. [Daftar akun dan matriks](staging-accounts.md).
- SSO enam hostname nyata lulus dengan satu password exchange dan enam cookie berbeda, host-only, `Secure`, `HttpOnly`, `SameSite=Lax`. Tidak ditemukan session/provider token di URL, DOM, localStorage, atau sessionStorage pada pemeriksaan yang dijalankan.
- CSRF tanpa token ditolak 403; self-edit koordinator ditolak 422 `VALIDATION_ERROR` tanpa perubahan version; role/division headers palsu tidak menaikkan akses. Script awal salah mengharapkan 400 untuk self-edit; assertion dikoreksi ke kontrak/handler/unit test yang sudah memakai 422, tanpa perubahan atau pelonggaran aplikasi. Full scenario kemudian lulus.
- Dua sesi Praktikum sintetis dengan scope kelas A/B membuktikan filter list dan detail: asisten scoped dapat membaca A, B tidak muncul di list dan detail B 404. Keduanya diarsipkan melalui API setelah pengujian; tidak ada attempt/grading job dibuat oleh skenario ini.
- Revocation permission PDD berlaku pada request berikutnya tanpa login ulang; akses admin berubah 403 sementara identitas tetap 200. Update version lama 409, lalu assignment asli dikembalikan. User lain tidak dapat mencabut sesi koordinator (404). UI pencabutan sesi portal tidak mencabut admin atau user lain. GET logout tidak mengubah sesi; tombol logout membatalkan seluruh keluarga login pada enam host, tanpa memengaruhi mahasiswa pada konteks terpisah.
- Wrong-password retry mempertahankan deep link `/profil`; menu admin 320px dapat dibuka dan ditutup memakai keyboard. Screenshot awal menu diambil saat transisi CSS 180 ms masih berlangsung; screenshot itu bukan bukti clipping permanen.
- Cleanup enam sesi dari script awal sempat ditolak pengaman tool. Setelah pemilik menyetujui keenam ID secara eksplisit, hanya keenam sesi tersebut dicabut melalui API. Pembacaan read-only memastikan nol sesi aplikasi aktif untuk 12 akun sintetis pada akhir cleanup. Akun/password/role tidak dihapus.
- Teks UI lama yang menyebut domain hanya shell/belum operasional dikoreksi tanpa perubahan permission. Lint/typecheck, 82 unit frontend, enam build, dan full Chromium 54/54 (termasuk C/Python nyata) lulus setelah koreksi. Keenam frontend staging kemudian dideploy ulang dan READY pada project yang identitas/team-nya diverifikasi.
- Smoke setelah rollout selesai pada **01:37 WIB**: login/SSO nyata keenam frontend lulus, teks placeholder lama tidak tampil, menu PDD tetap hanya Ringkasan/Profil, dan screenshot mobile setelah transisi CSS selesai membuktikan drawer sepenuhnya berada di viewport. Screenshot desktop dan mobile final diperiksa visual. Semua konteks logout; query read-only kembali menunjukkan nol sesi aplikasi aktif untuk 12 akun. Deployment IDs serta bukti final ada pada `frontend-deployments.json` dan `final-ui-smoke.json` di direktori bukti sementara.

Acceptance ini menutup baseline akun/login/SSO/role/division/scope/revocation yang disebutkan, bukan seluruh alur domain Phase 3–5, SMTP/reset-password, OAuth, provider runner, atau kapasitas cohort. Bukti sementara berada di `E:/tmp/algohub-staging-accounts-myU3rM/` tanpa file password di laporan.

## Pemeriksaan Glot — batas izin terpenuhi

Pemilik mengizinkan token legacy untuk pengecekan terbatas, bukan deployment baru. Token tidak dicetak, tidak disalin ke source/environment deployment, dan source legacy tidak diubah.

- Endpoint lama `/api/run` dan `/api/run/{language}/latest` mengembalikan 404.
- Source resmi Glot terkini memakai `POST /api/mux`, dengan action `get_language_version` dan `run`. Public run policy tidak memerlukan autentikasi; token legacy tidak dikirim ke endpoint publik ini.
- Versi teramati: Clang 17.0.6 dan Python 3.11.9. C dan Python berhasil menjalankan program sintetis melalui API baru. Satu probe C pertama menghasilkan compiler error karena escaping source pengujian; probe C berikutnya berhasil tanpa mengubah layanan/provider.
- Probe TCP terbatas ke alamat API staging milik project menghasilkan `blocked_or_unreachable`. Ini bukti satu skenario, bukan bukti bahwa seluruh kemungkinan egress/escape mustahil.
- File marker sintetis yang ditulis pada satu container tidak ada pada request container berikutnya.
- Batas yang terbaca: cgroup memory sekitar 2 GB; RLIMIT CPU/address-space/file-size tidak dibatasi per proses dan batas process cukup besar. Tidak diklaim sudah memenuhi seluruh budget runner project. Adapter perlu batas compile/run/output, pemeriksaan versi dalam eksekusi yang sama, serta failure handling yang diuji.
- API menerima nama image `:latest`; immutable image pinning tidak ditawarkan oleh codec yang dibaca. Jangan menyamakan metadata versi cache dengan jaminan versi eksekusi.
- **Batas probe konservatif 10/10 sudah digunakan** (termasuk dua probe versi). Tidak ada probe eksekusi Glot tambahan tanpa izin baru. Adapter server belum diaktifkan pada staging.

Pemeriksaan source upstream pada 11 November menemukan blocker tambahan, tanpa request ke API eksekusi Glot: [`code-runner` commit `41befd8`](https://github.com/glotcode/code-runner/blob/41befd8ae32c0f813f0bd2b7b054c8f405d0814e/src/main.rs) hanya mengirim `stdout`, `stderr`, `error`, dan `duration`. Build/run failures memakai konversi yang sama, dan `ExitFailure` tanpa numeric exit code menghasilkan `error` kosong. Karena itu adapter tidak boleh menganggap `error == ""` sebagai bukti exit 0, menebak tahap compile/runtime, atau mengarang field exit code. `duration` adalah nanodetik dari command yang dilaporkan, bukan otomatis total compile+run.

Ini bukti tentang source yang dibaca, bukan konfirmasi versi/config hosted service. Adapter bounded belum dibuat atau dihubungkan: masih diperlukan cara terpercaya memperoleh status/fase proses, pemeriksaan versi dalam eksekusi yang sama, serta batas output/resource/network/cleanup yang tervalidasi. Provider aplikasi tetap `Unconfigured`; tidak ada kode mahasiswa dikirim, probe tambahan, aktivasi, atau penggantian layanan secara diam-diam.

## Acceptance domain lanjutan — 11 November 2026

Pemilik menyetujui baseline 30 pengguna, maksimal 120 attempt untuk batch scheduler, dataset sintetis kecil pada domain lain, serta delapan request Glot tambahan. Ringkasan acceptance, koreksi harness, data retained, dan urutan lanjut tersebar di [dokumen staging](staging-setup.md) dan [runbook scheduler](staging-scheduler.md); arsip rinci tidak lagi disimpan di repo.

- SIAKAD/inventaris: 284 passing assertions pada transaksi nyata terbatas (QR, stok/rental concurrency, divisi finance, CSV/notifikasi/audit), tanpa defect aplikasi unresolved pada dataset itu. Ada gap cakupan yang tercatat; bukan full sign-off tanpa batas.
- E-learning: 20 pemeriksaan awal dan satu defect text-only completion. Fix text-only kemudian dideploy sebagai `dpl_3gJ4ZhcarakoZWr159jC4q2cM63v`; retest retained text/quiz lesson pada 02:52 WIB lulus 8/8, tanpa draft dummy atau XP ganda. Fix achievement normal-authoring lebih baru masih memerlukan deployed acceptance.
- Praktikum: rerun `p3-3a689b93` selesai pada 02:52:38 WIB. Tujuh kelompok pemeriksaan lulus, termasuk manual publication/replay, token/shared timer, close-course, dan exam admission. Ekspektasi private-grade diperbaiki menjadi 404 sesuai kontrak, tanpa melonggarkan aplikasi. Server runner tetap terpisah.
- Glot: tambahan **8/8 sudah digunakan** (total historis 18). Compiler 17.0.6/Python 3.11.9 teramati; SIGTERM nyata menghasilkan HTTP 200 dengan `error` kosong. Pemilik kembali memilih **tetap Glot**, tanpa izin eksekusi/probe tambahan. Respons mentah belum cukup untuk adapter authoritative; runner tidak diaktifkan.
- Scheduler dengan data sudah diuji: 102 attempt, expiry cron 100+2, jawaban beku sesuai, dan expired-lease sintetis dipulihkan. Sisa backlog diselesaikan melalui tindakan pengajar, bukan throughput cron otomatis. [Bukti dan batas](staging-scheduler.md#acceptance-lanjutan-dengan-data-sintetis).
- Baseline 30 pengguna/300 request baca lulus, p95 843,69 ms, tanpa mismatch identitas. Ini bukan uji runner, soak test, atau jaminan kapasitas produksi. [Bukti](staging-accounts.md#identitas-tambahan-untuk-baseline-kapasitas).
- SQL-role SIAKAD tambahan lulus 16/16 denial pada delapan tabel RLS-enabled. Actor permission/scope dan pagination staging kemudian ditutup oleh 16 pemeriksaan lanjutan yang diizinkan; lihat pembaruan Docker/staging di bawah.
- Verifikasi lokal baru sebelum perubahan aturan completion: lint/check, 82 unit frontend, OpenAPI 90/342, enam build dan Chromium 54/54 lulus. Go unit/vet/build lulus, tetapi **24 PostgreSQL test di-skip** karena startup DB disposable tertahan classifier. Race workaround ditolak sebelum eksekusi; tidak dijalankan lewat agent/tool alternatif. Bukti run ini tidak menggantikan integration/race acceptance.
- Pemilik kini mewajibkan **run server yang lulus untuk completion lesson berkode**. Invalid nonempty source sebelumnya dapat diberi `test_passed=true` dan XP tanpa run; reproducer lokal membuktikannya. Backend/frontend sudah diperketat: proof terikat owner/lesson/source/language/versi definisi; testcase memakai stdin masing-masing dan pembandingan output exact; runtime hasil harus cocok dengan versi yang diminta. Fixture simulation hanya boleh dipercaya pada mode lokal terisolasi. Hasil baseline terdahulu bukan sign-off perubahan baru ini.
- Verifikasi completion lokal final: **83 unit frontend, lint/typecheck, enam build, OpenAPI 90 operasi/344 respons, dan 55/55 Chromium lulus**. Report JSON memuat nol skipped, unexpected, dan flaky; hasilnya diperiksa dengan assertion, bukan hanya exit wrapper. Bukti final: `E:/tmp/algohub-completion-final-OgFjA5/verified-summary.json` dan log/report pada direktori yang sama.
- Run Chromium sebelumnya mencatat 54 lulus/1 gagal: `route.fetch` pada harness baru tidak me-resolve hostname lokal. Perbaikan memakai helper `fetchLocalRoute` yang sudah ada; rerun terarah 1/1 dan full final 55/55 lulus. Artefak failure `E:/tmp/algohub-completion-frontend-fw6bLf/` tetap menyimpan kegagalan/assertion dengan header fixture disensor.
- Pada pass Windows setelah runtime-version fix: **100 top-level test PASS, 276 PASS events termasuk subtests, 24 PostgreSQL SKIP**, vet/build exit 0. Missing/wrong runtime sempat dibuktikan RED lalu GREEN setelah guard diperketat. Bukti: `E:/tmp/algohub-learning-closeout-UEPfKj/runtime-final-test-metadata.log`, `go-test-runtime-final.jsonl`, dan `runtime-version-{red,green}.log`. Saat itu startup DB tertahan classifier dan Docker belum aktif; tidak ada cluster atau deployment perubahan ini pada pass tersebut. Hambatan ini kemudian diselesaikan lewat verifikasi Linux dan deployment di bagian berikut, bukan dengan mengulang workaround keamanan Windows.

## Linux, staging, dan audit dependensi — 11 November 2026

- Setelah pemilik menyalakan Docker Desktop, PostgreSQL integration dan **standard** `go test -race -p 1 -count=1` pada Go 1.26.10 Linux lulus: masing-masing 124 top-level tests/385 test events, nol gagal atau skipped test, termasuk seluruh 24 PostgreSQL tests. Sembilan package tanpa test bukan skipped tests. Go vet/build lulus. Snapshot 102 build inputs tidak berubah; container/network/volume khusus tes sudah dibersihkan. Bukti: `E:/tmp/apv2-pg-race-20261111T054651Z-0dd51e0f/logs/{normal-summary,race-summary,state}.json`. Docker hanya alat verifikasi lokal; arsitektur aplikasi tetap serverless Vercel/Supabase, tanpa privileged container atau perubahan ASLR.
- Follow-up SIAKAD `STAGING-CLOSEOUT-62f07f41` lulus 16 pemeriksaan: 99 item retired/stock0/fee0 memberi pagination 100+1; report/CSV sesuai; actor attendance tanpa schedule.write dan actor institution/shift terbatas berperilaku sesuai kontrak. Semua grant sementara dipulihkan (version 2→3→4), sesi ditutup, kelas inactive, konteks logout. Bukti: `E:/tmp/algohub-siakad-closeout-approved-a84d6c/results.json`.
- API dan frontend E-learning dideploy dengan izin pemilik: `dpl_G5bNSax7ehyzXmFRtf7JntdSs376` dan `dpl_ATJ1g9YeNe9YVDE17qWxy4W1BFEe`, READY pada project staging yang diverifikasi. API health/readiness 200, anonymous auth 401. Bukti: `E:/tmp/algohub-proof-staging-deploy-BHcLK6/`.
- Acceptance E-learning pada deployment tersebut **lulus**: code tanpa proof ditolak 409 tanpa XP, UI unavailable benar, konflik offline diselesaikan tanpa kehilangan draft, quiz/text empty-draft dan replay aman, achievement level pertama memakai ID hasil authoring normal, serta tepat 10 hints sukses + request ke-11 mendapat 429/Retry-After60. Dibuat satu level/module/lesson, kemudian level locked; delapan pemeriksaan sesi akhir 401. Enam screenshot diperiksa. Bukti: `E:/tmp/algohub-elearning-acceptance-E9xEbK/results.json`. Tidak ada request Glot/server runner; ini bukan acceptance successful code execution.
- Audit Go selanjutnya menemukan [GO-2026-6859](https://pkg.go.dev/vuln/GO-2026-6859), dipublikasikan hari yang sama, fixed Go 1.26.11. Minimum pada `backend/go.mod` dan `go.work` dinaikkan ke 1.26.11. Audit ulang dengan toolchain tersebut menghasilkan **No vulnerabilities found**; bukti sebelum/sesudah berada di `E:/tmp/algohub-dependency-audit-ACAas7/`.
- **Verifikasi Go 1.26.11 final lulus:** normal dan standard race masing-masing 124 top-level/385 test events, 24/24 PostgreSQL berjalan, nol gagal/skipped test, vet/build exit 0. Interruption host dipulihkan dari JSON yang sudah selesai lalu hanya vet/build dilanjutkan; normal/race tidak dijalankan dua kali. Checksum source/snapshot sama, nol container/network/volume milik run tersisa. Bukti: `E:/tmp/apv2-pg-race-go12611-20261111T070516Z-ae6fe6ba/logs/final-verification.json`.
- Verifikasi frontend Go 1.26.11 sempat mengalami allocation failure pada check/build Admin dan error output reporter saat sesi terputus. Lima aplikasi lain, 83 unit frontend, format check, serta OpenAPI 90/344 sudah lulus. Enam server fixture orphan dibersihkan berdasarkan PID dan command milik run. Check/build Admin dan full browser dijalankan serial dengan output melalui pipe native: **55/55 lulus, nol skipped/unexpected/flaky**. Bukti gabungan: `E:/tmp/algohub-go12611-frontend-xlgxBW/` dan `E:/tmp/algohub-go12611-resume-Kljxcn/summary.json`; failure lama tidak dihapus atau dianggap pass.
- API staging patch aktif sebagai **`dpl_CmdRvMDcqiJ7gDqyFqhR1xGhcRA9`**, minimum Go 1.26.11. Health/readiness 200, anonymous auth 401; login serta reads levels/achievements/curriculum 200, logout 200 dan app/central session 401. Tidak ada fixture, hint, atau request runner baru. Guard completion tidak diulang pada smoke patch karena tidak ada synthetic code lesson yang masih sesuai filter; functional guard pada aplikasi yang sama sudah terbukti pada run E-learning sebelumnya. Bukti: `E:/tmp/algohub-api-go12611-deploy-OMglsy/verified-deployment.json` dan `E:/tmp/algohub-go12611-postdeploy-smoke-4f2a9c/smoke-1794386238512.json`. Frontend E-learning tetap `dpl_ATJ1g9YeNe9YVDE17qWxy4W1BFEe`.
- `npm audit --audit-level=high` exit 0, tetapi **3 low/4 moderate** masih tercatat melalui `deepmerge`, `glob`, dan `minimatch` pada rantai Vite/plugin/Vercel NFT. Ini bukan audit nol temuan. Tidak menjalankan force-fix yang merekomendasikan downgrade framework breaking; tidak mengubah threshold CI atau menyatakan advisory tersebut sudah diperbaiki.

## Kajian source Glot lanjutan — 11 November 2026

Hasil kajian dan sumber terpin (arsip lokal, tidak lagi disimpan di repo) menelusuri route publik sampai container melalui repo resmi `glotcode/docker-run`. Codec `/api/mux` tetap hanya meneruskan empat field hasil; numeric exit/fase tidak tersedia. Build image resmi mereferensikan `code-runner` `3b87378`, berbeda dari main `41befd8`; keduanya kehilangan informasi signal yang sama. Metadata versi terpisah/cache serta allowlist image `:latest` belum membuktikan runtime tiap run.

Mekanisme resource/cleanup tersedia pada source, tetapi konfigurasi hosted belum terbukti. Penambahan status hanya pada `code-runner` tidak cukup tanpa dukungan codec publik dan rollout image layanan. Rekomendasi berikutnya adalah konfirmasi/jalur resmi dari pengelola hosted Glot sebelum budget probe baru. Tidak ada request Glot/metadata, operasi staging, perubahan kode aplikasi, deployment, atau pengulangan suite pada kajian ini; runner tetap `Unconfigured`, budget tetap 18/18 terpakai.

## Preview runtime v2 dan fallback Glot — 11–12 November 2026

Implementasi, pengujian lokal, dan deployment staging. Tidak ada request Glot, VPS, container, atau perubahan provider. `Runner` tetap `Unconfigured`; completion tanpa proof tetap ditolak.

Keputusan pemilik 12 November 2026: jalur runner disamakan dengan project lama pada bagian yang aktif dipakai — C/WASM + Pyodide di browser dan Glot hanya sebagai fallback preview. Tidak ada provider authoritative baru; gate successful execution → proof → completion tetap **TERBUKA** dan code lesson tetap belum dapat diselesaikan sampai keputusan itu berubah.

### L0 — runtime browser `/runtimes/v2/`

- Iframe dan worker tidak lagi dimatikan saat komponen dilepas. Satu iframe per origin dipakai ulang; dispose setelah idle 10 menit atau `pagehide`. Ini menggantikan perilaku 10 November yang melepas iframe saat ref-count nol.
- Inisialisasi worker C hanya memuat `memfs` + `sysroot.tar`; `clang` dan `lld` di-prefetch setelah `ready`. `warmup` dikirim saat editor fokus, sehingga unduhan 50 MB keluar dari jalur kritis Run pertama.
- Aset compiler tetap byte-identik di `/runtimes/v1/clang/` dan tidak diduplikasi. `Content-Type: application/wasm` ditambahkan untuk `clang`, `lld`, dan `memfs` pada konfigurasi Vercel dan dev server; worker memakai `WebAssembly.compileStreaming` dengan fallback `arrayBuffer`.
- Hasil kompilasi C di-cache per kunci source + versi compiler (LRU 4). Sebelum compile berikutnya, instance `API`/memfs dibuat ulang agar heap tidak tumbuh.
- Worker Python dipakai ulang; setiap run memakai globals baru yang di-`destroy()`. Recycle setelah 50 run atau 10 menit idle; worker C setelah 200 run.
- Output di-batch per 16 ms atau 16 KiB dengan cap 16 KiB per stream dan flag `truncated`; buffer UI sementara juga dibatasi. Selama eksekusi WASM C yang sinkron, timer 16 ms di worker tidak dapat berjalan; flush berbasis 16 KiB tetap bekerja dan sisa buffer di-flush saat run selesai.
- Watchdog dimiliki frame: C 20 detik, Python 10 detik, dipasang sebelum init/compile. Timer berhenti saat menunggu input dan dimulai lagi setelah input dikirim. Timeout men-terminate worker dan mengembalikan status `timeout`.
- SAB interaktif hanya dibuat bila `crossOriginIsolated`; header SAB direset sebelum run/worker baru. Pada iframe bersarang yang tidak cross-origin isolated (arsitektur saat ini), jalur SAB tidak aktif dan stdin awal tetap dipakai.
- `COOP: same-origin` + `COEP: require-corp` dipasang untuk `/runtimes/v2/*`. `CORP` harus `cross-origin`, bukan `same-origin`: iframe `sandbox="allow-scripts"` memiliki origin opaque, dan `same-origin` memblokir `frame.js`/`worker.js` (dibuktikan lewat kegagalan E2E sebelum diperbaiki). Pyodide tetap dari CDN `v0.25.0` yang dipin karena jsdelivr mengirim `CORP: cross-origin`; self-hosting tidak diperlukan dan tidak ditambahkan.
- Dua belas bug legacy pada §4.11 rancangan tidak diport pada v2: flag SAB basi, input basi, timer berhenti setelah input, compile tanpa timeout, worker dipakai bersama lalu di-terminate, dua run bersamaan, `injectStdoutUnbuffering`, id acak/write tanpa run id, globals Python bocor, salinan output tanpa cap, dan tanpa terminate. Runtime v3 menggunakan transformasi AST untuk `input()` tingkat atas, bukan penggantian teks; string, komentar, spasi pemanggilan, future import, dan chaining tetap terjaga. Fungsi sinkron memakai stdin non-interaktif.

### Runtime v3 — parity Python legacy dan rollout staging

- Runtime v3 dideploy pada portal staging; byte `/runtimes/v2/` tetap sama dengan versi yang sudah terpublikasi (SHA-256 sebelum/sesudah cocok). Enam aset frame/worker v2/v3 pada domain staging cocok dengan source; header v3 COOP/COEP/CORP dan cache immutable diverifikasi nyata.
- `loadPackagesFromImports` memuat paket dari distribusi Pyodide pinned; tidak ada micropip/pip arbitrer. Bootstrap worker memiliki budget 60 detik, pemuatan paket 120 detik, dan eksekusi Python 10 detik yang dimulai setelah initialization. Parent/frame timer berhenti saat menunggu jawaban input dan dipasang kembali setelah jawaban.
- Mode interaktif Python opt-in memakai message round-trip tanpa SAB. Transformasi AST mendukung `input()` tingkat atas dan async function; untuk fungsi sinkron, UI menjelaskan penggunaan input standar sebelum Run. Setiap run memiliki globals baru; cancel menghentikan worker dan callback lama tidak memengaruhi run berikutnya.
- `plt.show()` menghasilkan PNG melalui channel terpisah (maksimal 4 gambar, 512 KiB/gambar). Figure ditutup dan handler dipulihkan pada cleanup, termasuk run gagal; worker direcycle sesudah hasil gagal.
- Preview E2E final **9/9 lulus**; full Chromium dengan Python opt-in **60/60 lulus**, nol skipped/unexpected/flaky, satu worker, nol retry. JSON report diperiksa, bukan hanya exit wrapper. Regresi mencakup `input().strip()`, spasi pemanggilan, literal `input(`, future import, menunggu lebih dari 45 detik, cancel/rerun, globals terisolasi, cold initialization, NumPy, matplotlib nyata dan cleanup figure setelah error. Format dan **85 unit frontend** lulus; tiga frontend staging build cloud berhasil.
- Deployment production-slot pada project staging terpisah: portal `dpl_7JtGnVvRkfcLREx6tFaWCmRn4pVY`, E-learning `dpl_GDac9GL82o4a3mLdLpRbcixX7ERj`, Praktikum `dpl_HpuezkgVhUJ6SsGnNNXYALiaTnZU`; semuanya READY. API, Auth, Admin, dan SIAKAD tidak dideploy ulang pada rollout ini.
- Smoke browser nyata pada **12 November 2026, 17:57 WIB** lulus **10 pemeriksaan**: Supabase login mahasiswa sintetis, C/stdin v3, iframe opaque, import NumPy, interaktif Python/chaining, cancel/rerun, fungsi sinkron dengan stdin, gambar matplotlib, cleanup figure setelah failure, dan Praktikum SSO shell. Nol request runner API; nol page error. Screenshot grafik diperiksa visual. Logout mencabut app/central session (401); tidak ada data domain dibuat atau diubah.
- Bukti: `E:/tmp/algohub-v3-release-20261112-a83b/` — `preview-tests.json`, `full-tests.json`, `runtime-before.json`, `runtime-after.json`, tiga log deployment, `staging-smoke.json`, dan `matplotlib-staging.png`.
- Batas tetap terbuka: input interaktif C pada iframe bersarang (SAB tidak aktif), library di luar distribusi Pyodide, conformance runner authoritative dan successful execution → proof → completion. Preview tidak menghasilkan XP/proof server; gate 1–3 tidak dicentang oleh rollout ini.

### L1 — fallback Glot non-authoritative

- `POST /v1/playground/run`: session + CSRF wajib, batas 5/menit dan 30/jam per user dengan `429` + `Retry-After`. Source ≤64 KiB, stdin ≤16 KiB, UTF-8.
- Klien provider tunggal dengan keep-alive, timeout 10 detik, `LimitReader` 256 KiB, tanpa retry. Token hanya dibaca dari environment. Body provider tidak diteruskan mentah ke klien/log.
- Respons memuat `status`, `stdout`, `stderr`, `truncated`, `duration_ms`, `provider: "glot"`, `authoritative: false`; tidak disimpan, tidak memberi XP, dan tidak menyentuh tabel run.
- Codec publik Glot tidak menyediakan fase/exit code. Karena itu `error` non-kosong dipetakan ke `runtime_error` tanpa mengklaim fase; `message` provider dipetakan ke `provider_error`. Batas ini didokumentasikan pada OpenAPI dan tidak dipakai sebagai proof.
- Token tidak tersedia: endpoint mengembalikan `503 PLAYGROUND_UNAVAILABLE`. Adapter diuji dengan `httptest`/fake provider; tidak ada probe ke layanan Glot.

### Perintah dan hasil lokal

| Perintah | Hasil |
| --- | --- |
| `npm run format:check` | lulus |
| `npm run check` | 6 app, 0 error/0 warning |
| `npm run test:frontend` | 85 unit lulus (5 UI + 28 auth-client + 29 + 5 + 18) |
| `npm run api:test` | seluruh package lulus; 24 PostgreSQL test SKIP karena database uji tidak dikonfigurasi pada run awal |
| `ALGOHUB_TEST_DATABASE_URL=... go test -count=1 -v ./...` | **140 PASS, 0 SKIP, 0 FAIL** dengan PostgreSQL 18 loopback disposable; database dihapus setelah run |
| `go test -race -p 1 -count=1 -v ./...` di container Linux Go 1.26.11 + PostgreSQL 15 | **140 PASS, 0 SKIP, 0 FAIL, 0 DATA RACE**; container `--rm` |
| `npm run api:vet` | lulus |
| `npm run api:build` | lulus |
| `npm run openapi:check` | 91 operasi, 345 live response contract |
| `npx playwright test` | 55 lulus, 0 gagal, 1 skipped (Python opt-in); suite preview terpisah 4/4 |
| `ALGOHUB_TEST_PYODIDE=true npx playwright test -g "real Python"` | 1/1 lulus |

### Deployment staging — 12 November 2026

Deployment production-slot pada empat project staging terpisah (bukan project legacy/produksi):

| Project | Deployment | Status |
| --- | --- | --- |
| `algohub-v2-staging-api` | `dpl_ApjpTZ78ycw2whEgKaxbRg9Td1k5` | READY, target production |
| `algohub-v2-staging-portal` | `dpl_5zEdBrB2SJvXYyGceqm96v7ZyRqU` | READY, target production |
| `algohub-v2-staging-elearning` | `dpl_HNqdNuJbNJF3g1PUR5FZSvLeufQ6` | READY, target production |
| `algohub-v2-staging-praktikum` | `dpl_4vnnTGAMyhit4Z8PcbbZQy8vyFgY` | READY, target production |

- API custom domain: `/healthz` 200, `/readyz` 200 dengan database staging, `/v1/auth/me` 401 `UNAUTHENTICATED` dengan `private, no-store`.
- Portal custom domain: `/runtimes/v2/frame.html` 200 dengan `Cache-Control: public, max-age=31536000, immutable`, `COOP: same-origin`, `COEP: require-corp`, `CORP: cross-origin`; `frame.js` sama. `clang`, `lld`, dan `memfs` 200 dengan `Content-Type: application/wasm` dan cache immutable. Root portal `no-store`.
- BFF apps sebelumnya menolak `/v1/playground/run` dengan `IDENTITY_404`. Allowlist BFF ditambahkan (POST saja, body budget domain) dan diuji; elearning/praktikum dideploy ulang.
- Pada staging, request terautentikasi melalui BFF ke `/v1/playground/run` mengembalikan **503 `PLAYGROUND_UNAVAILABLE`** dengan `private, no-store`; session dan CSRF valid, dan token provider memang tidak dikonfigurasi.

#### Metrik staging (Chromium headless, domain staging)

| Metrik | Target | Hasil |
| --- | --- | --- |
| Run C pertama (compile) | — | 2.443 ms |
| Run C kedua, source sama | ≤150 ms | **1 ms** |
| Ganti stdin tanpa compile | ≤150 ms, tanpa aset baru | **1 ms; 0 request aset** |
| Run Python setelah init | ≤300 ms | **21 ms** (run pertama 20 ms) |
| Request aset runtime pada run kedua | 0 | **0** (8 request awal, 1 tambahan saat ganti bahasa) |
| Invocation API/preview | 0 | **0** |
| Pertumbuhan heap 20 run | <50 MB | **0,54 MB** |
| Kesalahan halaman | 0 | **0** |

Pengukuran memakai akun sintetis mahasiswa; credential dibaca dari file privat dan tidak dicetak. Preview berjalan pada halaman playground, sehingga pengukuran tidak memerlukan penulisan data domain. Worker Python tidak di-terminate setelah hasil; satu terminasi yang teramati berasal dari pergantian bahasa C → Python.

### Metrik lokal (Chromium headless, dev server loopback)

| Metrik | Target | Hasil | Cara ukur |
| --- | --- | --- | --- |
| Run C kedua, source sama | ≤150 ms | 1 ms | `data-testid="preview-duration"` setelah run kedua |
| Run C, stdin berubah | ≤150 ms, tanpa aset baru | 1 ms; 0 request | penghitung `page.on('request')` sebelum/sesudah run kedua |
| Run Python setelah init | ≤300 ms | 4–6 ms (run pertama 28–52 ms) | run kedua pada test Python opt-in |
| Request aset runtime pada run kedua | 0 | 0 | penghitung request `/runtimes/` |
| Invocation API/Vercel untuk preview | 0 | 0 | request `POST .../run` = 0 pada test preview |
| Pertumbuhan heap 20 run | <50 MB | 0,64 MB dan −1,13 MB pada dua run (GC) | CDP `Performance.getMetrics` (`JSHeapUsedSize`) sebelum/sesudah |
| Fallback p95 (adapter, fake provider localhost) | ≤5 dtk | 0,55 ms | 40 panggilan adapter, p95 |

Angka lokal di atas adalah pengukuran dev server; angka staging ada pada tabel sebelumnya. End-to-end p95 fallback dengan provider nyata, kuota Glot, dan kapasitas cohort tetap belum diukur.

### Batas yang masih terbuka

- Gate runner authoritative tetap terbuka: tidak ada status/fase/exit/runtime identity terpercaya, dan tidak ada successful execution → proof → completion dengan provider nyata.
- Fallback provider belum diaktifkan pada staging karena token tidak tersedia; 503 adalah perilaku yang terverifikasi, sedangkan jalur sukses hanya diuji dengan fake provider lokal.
- SAB interaktif tidak aktif pada iframe bersarang saat ini; mengaktifkannya membutuhkan COOP/COEP pada seluruh aplikasi induk dan pengujian ulang SSO.
- Hanya API, portal, elearning, dan praktikum yang dideploy ulang. Auth, admin, dan SIAKAD tetap pada deployment sebelumnya; ketiganya tidak memakai runtime preview atau route fallback baru.
- PostgreSQL integration dan standard race sudah lulus pada database loopback disposable di environment verifikasi (Windows dan container Linux); hasil ini bukan load test atau acceptance kapasitas produksi.

## Batch scheduler lokal — 12 November 2026

- `Reconcile` sekarang memproses hingga 20 grading job per invocation, bukan satu. Lease kedaluwarsa tetap dipulihkan lebih dulu, sisa pekerjaan tetap `pending`, dan loop berhenti saat context selesai.
- Test baru membuktikan: batch lebih dari satu job, urutan recovery lease sebelum pending, invocation bersamaan tidak memproses job dua kali, context canceled menyisakan tepat satu job pending, dan 120 job sintetis selesai dalam enam invocation (p95 invocation 65–105 ms pada repository memori, variasi run lokal).
- `go test ./...`, `go test -race -p 1 ./...` dengan PATH MSYS2 terdokumentasi, `go vet`, dan `go build` lulus. OpenAPI disinkronkan dan lulus **91 operasi/345 live response contract**; `jobs_processed` sekarang maksimum 20.
- PostgreSQL integration dan race dengan PostgreSQL dijalankan pada dua database loopback disposable PostgreSQL 18.3 dan lulus; Docker tidak diperlukan pada pass ini. Load di staging, provider grading nyata, dan operational outage drill belum dijalankan.
- Bukti lokal: `E:/tmp/algohub-reconcile-batch-20261112/` (`reconcile-tests.log`, `normal-postgres.log`, `race-postgres.log`, `openapi-check.log`). `govulncheck` melaporkan nol vulnerability; `npm audit --audit-level=high` exit 0 tetapi 3 low/4 moderate lama tetap tercatat pada rantai Vite/Vercel.
- Pada tahap verifikasi lokal ini, gate kapasitas (gate 3) tetap **terbuka**: hasil tersebut baru menutup grading scheduler lokal. Acceptance scheduler staging berikutnya menambah bukti, tetapi tetap tidak menutup server runner atau gate kapasitas end-to-end.

## Deployment API dan acceptance scheduler staging — 12 November 2026

- API dengan batch scheduler 20 job/invocation dideploy sebagai `dpl_5jFo5vXrz91G1JKtUgjqLcsNJFPt`, READY pada slot Production project staging terpisah dan alias `api.staging.algohub.web.id`. Ini menggantikan klaim terdahulu bahwa perubahan batch belum dideploy.
- Dataset retained `STAGING-BATCH20-cdb70744` berisi 120 attempt pada 10 activation dan 12 identitas sintetis existing. Seluruh 120 attempt dibuat serta disimpan melalui API aplikasi; 254 request setup menghasilkan nol 429.
- Enam delivery cron nyata pada 11:21–11:26 UTC (response ID 2512–2517) semuanya HTTP 200. Masing-masing memproses 20 job; expiry terjadi 100 lalu 20. Seluruh job mencapai `needs_manual` dalam 5 menit 9,265 detik sejak deadline, dengan tepat 120 claim dan 120 finalization actor-null, hash jawaban cocok, dan nol score terpublikasi.
- Satu lease expired sintetis milik dataset dipulihkan cron sekitar 50 detik setelah injeksi. Ini membuktikan jalur recovery terjadwal pada state sintetis, bukan crash/outage worker nyata. Observer baru aktif setelah 100 job selesai; delivery awal dibuktikan dari response/audit tersimpan.
- Cleanup awal benar-benar menutup 10 activation dan mengarsipkan session, tetapi script mengeluarkan failure generik tanpa merekam status HTTP/assertion. Failure `scheduler-cleanup.json` dipertahankan. Verifikasi final terpisah pada dataset yang sama kemudian lulus tanpa domain write: 120 submitted, 120 `needs_manual`, nol pending/running/lease, nol score, session inactive, 10 activation closed, cron aktif, dan nol sesi aktif actor run. Session verifikasi sendiri logout dan menghasilkan auth 401.
- Data sintetis dan audit sengaja retained; manifest terminal `verified-and-inactive-retained`. Bukti: `E:/tmp/algohub-v3-release-20261112-a83b/scheduler-{batch,observation,summary,cleanup,final-verification}.json`. Rincian dan batas ada di [runbook scheduler](staging-scheduler.md#acceptance-batch-20-pada-staging--12-november-2026).
- Acceptance ini menutup bagian scheduler grading staging, **bukan** gate 3 keseluruhan. Belum ada authoritative code runner/provider grading nyata, provider load, contention measurement, atau operational-outage drill.

## Riwayat provider free tier — superseded 12 November 2026

> Bagian ini hanya merekam pekerjaan yang pernah dilakukan dan kemudian dihapus. Jangan mengaktifkan kembali adapter atau menjalankan Next Steps provider.

Sebelum keputusan final $0, pemilik sempat memilih jalur provider authoritative free tier untuk gate 1–2. Halaman resmi Judge0 Cloud tidak mencantumkan free tier dan RapidAPI kemudian terbukti pay-per-use; jalur ini ditarik.

### Implementasi adapter lokal

- Adapter Judge0 ditambahkan di `backend/internal/platform/runner/judge0.go`; `Unconfigured` tetap default dan provider hanya aktif jika seluruh env URL/token/header/language ID/versi/limit terisi. Env parsial ditolak saat startup.
- Kontrak yang diterapkan: status/exit/signal mapping, output/JSON bound, `LimitReader`, `NormalizeResult`, no retry, dan `simulated=false`. C memakai versi compiler dari `compile_output` (`-v`); Python memakai wrapper yang menulis versi interpreter sebelum user code. Jalur Praktikum kini menolak `RunnerVersion` yang tidak sama dengan versi diminta.
- Test fake provider mencakup success, compile/runtime/timeout/signal, version mismatch, marker hilang, 503/429, body oversize, cancellation, allowlist, dan no-retry. Wrapper Python diuji dengan interpreter lokal (termasuk syntax error dan null byte).
- `go test ./...`, `go test -race -p 1 ./...` dengan PostgreSQL 18.3 disposable, `go vet`, dan `go build` lulus. OpenAPI tetap 91 operasi/345 live contract. Bukti: `E:/tmp/algohub-reconcile-batch-20261112/judge0-adapter-tests.log` dan `runner-race-postgres.log`.
- Audit fail-closed lanjutan menemukan bahwa identitas compiler C sempat boleh diambil dari `stderr` program pengguna dan pola banner dapat cocok di tengah diagnostic compiler yang dipengaruhi source; client HTTP juga dapat mengikuti redirect provider. Adapter kini hanya menerima satu baris banner compiler penuh dari `compile_output` pada submission yang sama dan menolak redirect. Regresi membuktikan spoof melalui `stderr`, diagnostic biasa, dan filename `#line` serta redirect lintas host tidak diterima dan token tidak ikut terkirim.
- Setelah perbaikan tersebut, test package runner/config/usecase serta seluruh `go test ./backend/...`, `go vet ./backend/...`, dan build API lulus. Run ini tidak mengaktifkan PostgreSQL integration dan bukan conformance provider nyata. Percobaan `go test -race` khusus package runner pada Windows tidak mulai tuntas karena ThreadSanitizer gagal mengalokasikan 77.201.408 byte (error 87); hasil itu dicatat gagal dan tidak diulang lewat jalur alternatif. Bukti race terdahulu tidak dilabel ulang sebagai verifikasi perubahan ini.
- Tidak pernah ada request ke provider nyata. Seluruh source/config/test adapter kemudian dihapus oleh keputusan final; bukti historis tetap dipertahankan tanpa menjadi acceptance aktif.

## Keputusan $0 browser-trusted dan implementasi revisi — 12 November 2026

- Pemilik, tim asisten, dan dosen menerima risiko bahwa completion/XP gamifikasi E-learning dapat dipalsukan oleh client yang dimodifikasi. Batas dan reversal rule dicatat pada [decision record](elearning-browser-completion-decision-2026-11-12.md).
- C/WASM dan Pyodide menjadi jalur test code lesson. Testcase berjalan sequential di browser dengan strict output; attestation client diikat UI ke lesson/version/language/source dan invalid ketika source berubah.
- Backend menerima `browser_test_passed` hanya untuk completion code lesson, tetap memeriksa quiz, version, revision, access, idempotency, dan exactly-once XP. Draft save menghapus `test_passed`; text/quiz tidak membutuhkan assertion.
- Adapter/config/env/wiring Judge0 dihapus. Generic runner fixture/compatibility tetap fail-closed di production; Glot tetap preview-only dan tidak memberi progression authority.
- UI Praktikum tetap browser-preview dan menjelaskan bahwa nilai akademik berasal dari alur manual, bukan preview.
- Verifikasi lokal revisi: 86 unit frontend lulus; format dan Svelte check enam aplikasi 0 error/0 warning; enam production build lulus; seluruh Go test/vet/build lulus; OpenAPI 91 operasi/345 live contract lulus; PostgreSQL integration pada disposable PostgreSQL 18.3 lulus; targeted E-learning Chromium 5/5 dan full Chromium dengan Python nyata 60/60 lulus, nol skipped/unexpected/flaky/error. Native Windows race gagal sebelum test karena ThreadSanitizer allocation error 87; standard Linux `go test -race -p 1 -count=1 ./backend/...` dengan Go 1.26.11 dan PostgreSQL 15 loopback kemudian lulus seluruh package/integration tanpa data race. Container dan network disposable dibersihkan; log final `E:/tmp/algohub-browser-linux-race-loopback-YUAYiw.log`.
- Deployment READY pada project staging terpisah: API `dpl_5mU2UxoDMxqRnj9UyEs5qdCtYqG3`, E-learning `dpl_79Tkb1jYMYWNiyNLF3oB52oKJdSe`, dan Praktikum `dpl_CE8hCYgUTw95uXKRi9uoB8o8Qoqo`. Custom domain health/readiness/root semuanya HTTP 200. Name-only environment check menemukan nol Judge0/RapidAPI env.
- Acceptance staging memakai akun dan lesson retained, tanpa dataset/account baru. Satu lesson retained dibuka dan diubah sementara menjadi testcase C, lalu definisi asli serta level locked dipulihkan melalui API. Student sintetis ID 12 menjalankan C/WASM sampai testcase browser lulus, completion menaikkan XP 110→170 tepat sekali, duplicate completion menambah 0 XP, dan nol request `/learning/playground/run` teramati. Session student logout 401; progress sintetis sengaja retained. Bukti: `E:/tmp/browser-trusted-staging-c84f.json`.

## Status gate final setelah keputusan scope

Checklist awal memiliki 15 gate. Dua belas gate selesai dengan bukti, sedangkan tiga gate authoritative-runner **resmi ditarik dari scope** melalui keputusan risiko produk yang disetujui pemilik, tim asisten, dan dosen. Ketiganya bukan gagal, bukan pekerjaan tertunda, dan tidak dihitung dalam denominator scope revisi. Dengan demikian Phase 1–5 pada scope revisi adalah **100% selesai secara teknis dan acceptance**.

- [x] Baseline preview C/Python dan regresi sebelum perubahan completion terbaru.
- [x] Regresi lokal completion berbasis proof: Go unit/vet/build, frontend lint/unit/build, live contract, dan full Chromium.
- [x] PostgreSQL integration/race normal di Linux dan deployed acceptance proof guard, text/quiz, offline/hint, serta achievement pada source fungsional yang diuji.
- [x] Audit ulang, regresi PostgreSQL/race Go 1.26.11, dan API staging patch + authenticated smoke, dengan batas bukti di atas.
- [~] Integrasi/conformance hosted authoritative runner — **superseded/withdrawn** oleh keputusan produk $0 browser-trusted; bukan pekerjaan tertunda atau gate yang diklaim lulus.
- [x] Konfigurasi runtime transaction pooler pada API staging; session pooler terpisah untuk migrator.
- [x] Tujuh project/deployment Vercel staging terpisah dengan environment sesuai tanggung jawab.
- [x] DNS, ownership verification, dan HTTPS pada custom hostname staging.
- [x] Akun sintetis, login/SSO enam host, role/division/scope, serta permission/session revocation dengan Supabase nyata.
- [x] Baseline Praktikum non-runner dan retest text-only/quiz-only E-learning pada dataset retained.
- [x] Acceptance actor SIAKAD institution/shift/read-write dan pagination 101 item, dengan cleanup/restore terverifikasi.
- [~] Hosted successful execution → proof → completion — **superseded/withdrawn**; completion revisi memakai assertion browser yang risiko pemalsuannya diterima khusus XP gamifikasi.
- [x] Scheduler terautentikasi, delivery nyata, batch expiry 102 attempt, lalu batch staging 120 attempt dengan enam invocation × 20 job dan verifikasi terminal retained; kedua skenario memakai simulasi expired-lease yang dinyatakan eksplisit.
- [x] Baseline beban baca 30 pengguna/300 request dalam batas skenario terdokumentasi.
- [~] Capacity/outage authoritative grading/server runner — **superseded/withdrawn** bersama provider authoritative; scheduler staging 120 job tetap bukti terpisah, bukan alasan mencentang gate lama.

**Status Phase 1–5 revisi: 100% selesai secara teknis dan acceptance.** Dua belas gate lama mempunyai bukti; tiga gate authoritative telah ditarik dari scope melalui keputusan risiko yang disetujui dan tidak dihitung sebagai kekurangan. Phase 6 belum dimulai dan memerlukan instruksi terpisah serta otorisasi scope data legacy.
