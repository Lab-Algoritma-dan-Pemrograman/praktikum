# Keputusan completion kode E-learning berbasis browser

Status: **final dan disetujui** pada 12 November 2026 oleh pemilik project setelah dibahas bersama tim asisten dan dosen.

## Keputusan

Project menggunakan jalur $0 yang setara dengan perilaku aktif `project_ap` lama:

- C dijalankan di browser melalui Clang/WASM.
- Python dijalankan di browser melalui Pyodide.
- Testcase code lesson E-learning dijalankan dan dibandingkan di browser.
- Hasil lulus browser boleh dikirim sebagai assertion completion untuk XP gamifikasi.
- Glot hanya fallback preview opsional dengan `authoritative:false`.
- Judge0/RapidAPI dibatalkan dan bukan dependency atau pekerjaan tertunda.

## Risiko yang diterima

Pengguna yang memahami DevTools atau request HTTP dapat memalsukan `browser_test_passed`. Source, lesson, version, user, revision, idempotency, transaksi, dan constraint tetap diikat untuk mencegah kesalahan lintas resource serta XP ganda, tetapi semuanya tidak membuktikan bahwa kode benar-benar dieksekusi.

Risiko ini diterima karena target utama adalah mahasiswa pemula dan XP hanya gamifikasi. Sistem tidak boleh disebut authoritative, anti-cheat, atau setara grading server.

## Batas keras

Keputusan client-trust hanya berlaku untuk completion/XP code lesson E-learning. Hasil browser dilarang menjadi dasar langsung untuk:

- nilai akademik Praktikum;
- ujian atau admission;
- sertifikasi;
- publikasi nilai;
- klaim correctness authoritative.

Praktikum tetap memakai submission tersimpan, antrean grading, review, dan publikasi manual oleh pengajar. Glot maupun browser preview tidak mengubah nilai.

## Kontrol yang tetap wajib

- autentikasi, CSRF, access/scope, dan rate limit;
- source nonblank dan ukuran input terikat;
- attestation terikat lesson ID, lesson version, language, dan source snapshot di client;
- optimistic revision dan conflict refresh;
- idempotency key stabil untuk retry ambigu;
- completion/XP tepat sekali dalam satu transaksi;
- testcase exact-output: CRLF dinormalisasi dan newline terminal dibuang, tetapi spasi, kapitalisasi, output tambahan, dan empty-output tetap strict;
- perubahan source/version memaksa browser test ulang.

## Konsekuensi arsitektur

Production tidak membutuhkan provider authoritative. Generic runner interface, fixture, endpoint compatibility, dan lease lama boleh tetap ada untuk test/kompatibilitas tetapi tidak memberi progression authority. UI E-learning tidak boleh bergantung pada `runner.available`. Browser runtime tetap berjalan bila Glot atau endpoint compatibility gagal.

Tiga gate authoritative-runner lama tidak boleh dicentang sebagai lulus. Gate tersebut berstatus **superseded/withdrawn oleh keputusan risiko produk** dan tidak menjadi bagian denominator scope revisi; 12 gate yang dipertahankan sudah mempunyai bukti. Phase 1–5 revisi dinyatakan 100% selesai setelah implementasi dan acceptance browser-trusted lulus. Phase 6 tidak dimulai otomatis dan tetap memerlukan instruksi serta otorisasi data terpisah.

## Mengubah keputusan

Authoritative runner hanya boleh diperkenalkan kembali melalui keputusan tertulis baru dari pemilik dan dosen yang menjelaskan tujuan, biaya, provider, batas penggunaan, serta acceptance gate. Agent berikutnya tidak boleh menghidupkan Judge0/RapidAPI atau menafsirkan penghapusannya sebagai pekerjaan yang belum selesai.
