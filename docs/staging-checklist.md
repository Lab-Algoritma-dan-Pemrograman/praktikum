# Checklist persiapan staging dan environment

Panduan pengisian manual untuk Algohub Rewrite, Phase 2–5.

**Batas pekerjaan:** isi konfigurasi secara lokal terlebih dahulu. Dokumen ini bukan perintah atau persetujuan untuk menjalankan migrasi, membuat user, deploy, atau mengubah DNS. Preflight berikutnya tetap read-only sampai hasilnya diperiksa dan perubahan disetujui.

Implementasi sampai Phase 5 memakai keputusan final $0: C/WASM dan Pyodide di browser, Glot preview-only, serta completion gamifikasi E-learning yang client-trusted. Judge0/RapidAPI tidak diperlukan. Migration rehearsal dan UAT/cutover tetap terpisah. Ringkasan terbaru ada di [update log](updates/README.md).

## 1. File mana yang harus diisi?

Isi file pribadi pada root project, sejajar dengan `package.json`:

```text
E:\rekapProject\project_ap_v2\.env.staging.local
```

File ini sudah tersedia. Buka dan edit file tersebut, bukan file di project legacy atau di dalam `apps/`.

Perbedaan ketiga file environment:

- [`.env.staging.local`](../.env.staging.local): nilai staging asli yang kamu isi manual. Bersifat pribadi dan diabaikan oleh `.gitignore`.
- [`.env.staging.example`](../.env.staging.example): template tanpa secret, untuk referensi nama variabel dan default staging. Jangan masukkan credential asli ke sini.
- [`.env.example`](../.env.example): konfigurasi fixture lokal untuk pengembangan. Jangan dipakai untuk staging.

Jika nanti membuat checkout baru dan file pribadi belum ada, salin template staging menjadi `.env.staging.local` **hanya jika file tujuan belum ada**. Jangan menimpa file yang sudah berisi konfigurasi.

- [ ] Nama file tepat `.env.staging.local`, bukan `.env.staging.local.txt`.
- [ ] Simpan sebagai teks UTF-8 melalui editor, kemudian tekan **Ctrl+S**.
- [ ] Jangan memasukkan secret ke README, dokumen ini, chat, screenshot, log, atau commit.

## 2. Yang perlu disiapkan sekarang

- [ ] Project Supabase **baru khusus staging**, bukan project legacy/produksi.
- [ ] Nama project yang jelas dan akses dashboard project tersebut.
- [ ] Password database project staging, disimpan secara pribadi.
- [ ] Tiga nilai koneksi pada bagian berikut berasal dari **project yang sama**.

Tidak perlu mengisi semua layanan sekaligus. Mulai dari tiga nilai Supabase; Vercel, DNS, dan scheduler disiapkan setelah target diverifikasi. Browser runner tidak memerlukan provider secret.

## 3. Isi tiga nilai Supabase

### `SUPABASE_URL`

Ambil **Project URL** dari pengaturan API/Data API pada dashboard project Supabase staging.

- Gunakan URL HTTPS project, bukan URL halaman dashboard.
- Bentuk umumnya `https://PROJECT_REF.supabase.co`.
- Jangan menambahkan path endpoint, query, atau trailing slash.

### `SUPABASE_ANON_KEY`

Ambil **legacy anon/public JWT key** dari bagian API Keys project.

- Adapter backend saat ini membutuhkan key format JWT yang kompatibel.
- Jangan gunakan `service_role`, secret key, atau password database.
- Jangan mengganti dengan key format lain tanpa memastikan dukungan adapter terlebih dahulu.

### `ALGOHUB_DATABASE_URL`

Ambil URI PostgreSQL dari panel **Connect → Transaction pooler** project yang sama.

- Salin hostname, username, port, dan nama database dari dashboard; jangan menebaknya.
- Ganti placeholder password dengan password database staging.
- URL-encode karakter khusus pada **komponen password**, bukan seluruh URI. Contoh: `@` menjadi `%40`, `#` menjadi `%23`, dan `%` menjadi `%25`. Jangan encode ulang password yang sudah ter-encode.
- Gunakan TLS dengan `sslmode=require` atau mode verifikasi sertifikat yang lebih ketat.
- Jika URI belum mempunyai query, tambahkan `?sslmode=require`. Jika sudah ada query, gunakan `&sslmode=require` tanpa menduplikasi parameter yang sudah ada.

Contoh bentuk pengisian di file pribadi berikut **berisi placeholder, bukan credential yang bisa dipakai**:

```dotenv
SUPABASE_URL='https://PROJECT_REF.supabase.co'
SUPABASE_ANON_KEY='LEGACY_ANON_PUBLIC_JWT_KEY'
ALGOHUB_DATABASE_URL='postgresql://postgres.PROJECT_REF:URL_ENCODED_PASSWORD@POOLER_HOST:6543/postgres?sslmode=require'
```

Gunakan nilai koneksi sebenarnya dari dashboard. Simpan di antara tanda kutip tunggal seperti contoh, tanpa spasi tambahan di dalam nilai.

- [ ] Ketiga nilai terisi, bukan kosong atau placeholder.
- [ ] URL project, anon key, dan URI database berasal dari satu project staging.
- [ ] File tersimpan; secret tidak dibagikan melalui chat.

## 4. Periksa flag dan default staging

Untuk acceptance seluruh Phase 2–5, cocokkan konfigurasi berikut dengan template staging:

```dotenv
ALGOHUB_ENV=staging
ALGOHUB_ENABLE_FIXTURES=false
ALGOHUB_COOKIE_SECURE=true
ALGOHUB_PRACTICUM_ENABLED=true
ALGOHUB_ELEARNING_ENABLED=true
ALGOHUB_SIAKAD_ENABLED=true
```

Pada pemeriksaan 9 November 2026, file pribadi lama belum memuat flag E-learning dan SIAKAD. Kedua flag telah ditambahkan pada 10 November 2026, bersama persyaratan TLS pada URI database. Untuk file lain/checkout baru, pastikan flag tetap ada; default backend tanpa flag adalah `false`. Schema staging kemudian diterapkan dengan persetujuan terpisah, bukan otomatis karena flag aktif. Lihat [update log](updates/README.md).

- [ ] Pertahankan API/Auth origin dan kedua allowlist HTTPS sesuai hostname staging yang disepakati.
- [ ] Pertahankan timeout, session TTL, rate limits, dan batas pool dari template kecuali ada perubahan yang dibahas khusus.
- [ ] Jangan aktifkan `ALGOHUB_ALLOW_PRODUCTION_MIGRATIONS`.
- [ ] Jangan isi `ALGOHUB_TEST_DATABASE_URL` dengan koneksi staging. Variabel itu hanya untuk database lokal disposable bernama `*_test`.
- [ ] Jangan menjalankan suite integration test yang menulis fixture pada database staging.

Boleh tetap kosong untuk persiapan awal:

- `ALGOHUB_PRACTICUM_CRON_SECRET`: baru diisi setelah scheduler disiapkan. Kosong berarti endpoint cron tidak aktif; acceptance rekonsiliasi terjadwal belum selesai.
- `SUPABASE_SSO_PROVIDER`: opsional. Baseline login email/password tidak memerlukan OAuth.

Manual grading sudah diterima untuk fase ini; tidak perlu menyiapkan key AI atau execution provider. Jangan menambahkan `RUNNER_*`, Judge0, atau RapidAPI secret; browser runtime tidak memerlukannya.

## 5. Apa yang perlu diberi tahu setelah pengisian?

Cukup beri tahu melalui chat:

- Nama project Supabase dan konfirmasi bahwa itu project staging baru.
- Lokasi file, jika berbeda dari path pada bagian 1.
- Bahwa file sudah disimpan dan siap diperiksa read-only.
- Email koordinator pertama saat tahap bootstrap akun akan dipersiapkan; tidak perlu password akun.

**Jangan kirim URL database lengkap, anon key, password, atau salinan isi `.env`.** Pemeriksaan membaca file secara lokal dan hanya mengeluarkan status yang disensor.

Tidak perlu meminta pemeriksaan berulang selama pengisian belum selesai.

## 6. Preflight setelah file siap

Pemeriksaan berikut belum menjalankan perubahan:

- [ ] Validasi keberadaan nilai dan format konfigurasi tanpa mencetak secret.
- [ ] Cocokkan identitas project dari URL Supabase, anon key, dan target database.
- [ ] Pastikan target memang staging yang kamu maksud; label `ALGOHUB_ENV=staging` saja tidak membuktikan targetnya aman.
- [ ] Periksa koneksi database, TLS, dan mode pooler menggunakan batas waktu.
- [ ] Baca keberadaan serta nama/checksum/urutan pada `public.schema_migrations` dengan transaksi read-only.
- [ ] Bandingkan dengan migrasi lokal `0002`–`0007`.
- [ ] Laporkan apakah aman dilanjutkan, apa yang belum terverifikasi, dan perubahan yang perlu persetujuan.

**Jangan menjalankan `cmd/migrate` untuk sekadar mengecek status.** Perintah tersebut dapat membuat/mengubah metadata dan menerapkan migrasi.

Riwayat yang belum bisa dibaca tidak boleh dianggap kosong. Jika checksum berbeda, migrasi tidak berurutan, ada migrasi remote yang tidak dikenal, atau target project tidak cocok, hentikan proses. Jangan memperbaiki checksum tercatat secara manual. Migrasi 0005/0007 sempat berubah selama pengembangan lokal; target yang sudah memakai versi lama memerlukan penanganan forward migration.

## 7. Persiapan berikutnya, setelah preflight dan persetujuan

### Database dan akun

- [ ] Untuk setup baru, siapkan koneksi **direct atau session pooler** ke database staging yang sama. Pada workspace ini URI session pooler telah dipisahkan ke `ALGOHUB_MIGRATION_DATABASE_URL`, sementara `ALGOHUB_DATABASE_URL` memakai transaction pooler runtime. Migrator memilih variabel khusus migrasi dan menolak port transaction Supavisor.
- [ ] Pastikan role migrasi sesuai, kondisi target diketahui, serta backup/rollback disepakati jika target sudah berisi data.
- [ ] Terapkan hanya migrasi yang sudah diperiksa dan disetujui.
- [ ] Bootstrap koordinator dan akun pengujian sintetis yang diizinkan; jangan mengimpor pengguna/data produksi dahulu.

### Vercel dan DNS

- [ ] Siapkan akses Vercel untuk enam frontend dan satu Go API khusus staging.
- [ ] Tentukan project/branch/environment staging yang benar, terpisah dari aplikasi live.
- [ ] Masukkan environment ke setiap project sesuai tanggung jawabnya, bukan menyalin seluruh file ke semua frontend.
- [ ] Siapkan akses pengelola DNS, lalu gunakan target record yang diberikan Vercel untuk custom domain staging.
- [ ] Jangan menebak CNAME/IP atau mengubah record produksi.
- [ ] Pastikan HTTPS aktif sebelum pengujian secure-cookie SSO.

**File lokal tidak otomatis digunakan deployment.** Go tidak memuat dotenv sendiri; Vercel tidak mengunggah file pribadi yang diabaikan Git. Nilai perlu diberikan ke proses atau dimasukkan melalui Environment Variables project yang sesuai, lalu deployment terkait diperbarui.

Pembagian penting:

- Koneksi database, anon key untuk adapter API, dan cron secret berada di **server Go API**, bukan bundle browser.
- Enam frontend menggunakan konfigurasi server untuk API/Auth origin dan allowlist yang sesuai.
- Jangan membuat variabel `PUBLIC_*` untuk credential.
- OAuth Auth Center, jika dipilih nanti, mempunyai konfigurasi tersendiri.

Daftar variabel per tujuan dan tujuh hostname yang diusulkan ada di [panduan teknis staging](staging-setup.md#2-where-values-go-on-vercel). Hostname dalam template adalah konfigurasi yang direncanakan, bukan bukti DNS/deployment sudah tersedia.

### Browser runner, scheduler, dan acceptance

- [ ] Uji C/WASM dan Pyodide nyata melalui browser pada akun sintetis existing; pastikan code completion tidak memanggil authoritative runner API.
- [ ] Pastikan Glot, bila dikonfigurasi, tetap `authoritative:false` dan tidak memberi XP/nilai.
- [ ] Pastikan tidak ada nama environment Judge0/RapidAPI dan jangan mencetak value environment.
- [ ] Siapkan scheduler untuk `POST /v1/internal/practicum/reconcile`, termasuk secret server dan cadence yang sesuai.
- [ ] Uji login/SSO lintas host, logout/revocation, permission dan RLS dengan layanan nyata.
- [ ] Uji alur Praktikum, E-learning, presensi, concurrency stok, penyewaan, verifikasi pembayaran, laporan, dan CSV.
- [ ] Catat hasil staging dan masalah yang tersisa sebelum migration rehearsal, UAT, atau cutover produksi.

## Batas selesai persiapan manual

Pengisian manual selesai ketika tiga credential tersimpan di file yang tepat, flag/default diperiksa, dan identitas project staging dapat dikonfirmasi. Itu berarti **siap preflight**, bukan siap produksi.

Referensi:

- [Template environment staging](../.env.staging.example)
- [Panduan teknis staging](staging-setup.md)
- [Update log](updates/README.md)
