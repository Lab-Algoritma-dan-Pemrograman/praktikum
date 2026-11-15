# Akun dan matriks akses staging

Akun sintetis dibuat pada 11 November 2026 setelah pemilik menyerahkan pemilihan email, pembuatan akun, dan penyiapan akses staging. Target hanya Supabase/Vercel rewrite; bukan pengguna atau data legacy.

## Kredensial

- Login admin: <https://admin.staging.algohub.web.id/login>
- Portal landing: <https://staging.algohub.web.id>
- Email dan password acak berada di file privat [`.env.staging.accounts.local`](../.env.staging.accounts.local). File ini dikecualikan oleh `.gitignore` dan `.vercelignore`.
- Password tidak dicantumkan di dokumen, chat, source, atau log pengujian. Jangan mengunggah file kredensial bersama bukti/screenshot.
- Domain `algohub.test` adalah domain sintetis. Tidak ada inbox, pengiriman email, atau bukti kelulusan SMTP/reset-password. Konfirmasi email global project tidak dimatikan.

## Model akses yang dipakai

Mengikuti [roles-permissions.md](../plan/roles-permissions.md) dan [sitemap admin](../plan/sitemap.md#3-centralized-admin--adminalgohubwebid):

- Role identity hanya `koordinator`, `asisten`, `mahasiswa`, `peminjam`.
- Tidak dibuat role baru bernama `admin`, `pengajar`, `superadmin`, atau nama divisi.
- Admin operasional adalah `asisten` dengan assignment divisi/permission/scope.
- Divisi target: `sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, `pdd`.
- Akses halaman domain tidak berarti seluruh CRUD di domain tersebut diizinkan. Setiap operasi tetap memeriksa permission, scope, dan kepemilikan resource.

| Email | Role | Divisi / skenario | Akses console admin yang teruji |
| --- | --- | --- | --- |
| `koordinator.staging@algohub.test` | koordinator | assignment global; permission dari seed | Ringkasan, Profil, Praktikum, E-learning, SIAKAD, Audit log, Pengaturan |
| `sekretaris.staging@algohub.test` | asisten | sekretaris | Ringkasan, Profil, Praktikum, SIAKAD |
| `bendahara.staging@algohub.test` | asisten | bendahara | Ringkasan, Profil, SIAKAD |
| `k3.staging@algohub.test` | asisten | k3 | Ringkasan, Profil, SIAKAD |
| `labops.staging@algohub.test` | asisten | labops | Ringkasan, Profil, SIAKAD |
| `pengajar.staging@algohub.test` | asisten | pendidikan; nama akun bukan role baru | Ringkasan, Profil, E-learning |
| `pdd.staging@algohub.test` | asisten | pdd | Ringkasan dan Profil saja |
| `asisten.staging@algohub.test` | asisten | tanpa divisi/explicit admin grant | Seluruh route console ditolak |
| `asisten.scoped.staging@algohub.test` | asisten | grant eksplisit terbatas, lihat bawah | Ringkasan, Profil, Praktikum |
| `mahasiswa.a.staging@algohub.test` | mahasiswa | user A | Seluruh route console ditolak |
| `mahasiswa.b.staging@algohub.test` | mahasiswa | user B, pengujian isolasi antar-user | Seluruh route console ditolak |
| `peminjam.staging@algohub.test` | peminjam | peminjaman tanpa role admin | Seluruh route console ditolak |

Akun scoped hanya diberi `admin.overview.read` dan `practicum.session.manage` untuk domain `praktikum`, institution `staging`, class `staging-a`, shift 1. Ini tidak memberi akses pengaturan user, domain lain, atau class `staging-b`.

Default divisi tetap memakai seed yang disepakati. Contohnya, PDD tidak diberi akses seluruh domain agar menunya terlihat penuh; bendahara tidak diberi hak mengelola soal; sekretaris tidak otomatis memperoleh seluruh permission penilaian.

## Provisioning yang dilakukan

- Kredensial Admin API/management Supabase tidak tersedia; akses database staging sudah diizinkan. Operator membuat record Auth dan identity email khusus akun sintetis dalam transaksi, menggunakan password hash bcrypt dan metadata email yang telah dikonfirmasi. Tidak mengubah konfigurasi email global atau memuat suite fixture aplikasi ke staging.
- Percobaan transaksi pertama gagal akibat tipe parameter SQL identity; rollback diverifikasi sebelum perbaikan dan pengulangan. Tidak ada akun parsial yang dibiarkan.
- Seluruh 12 akun kemudian login melalui UI deployment dan Supabase Auth asli. Profile aplikasi terbentuk melalui jalur login aplikasi, bukan diisi sebagai fixture. Akun baru terbukti memiliki nol assignment implisit, dan logout terverifikasi.
- Hanya koordinator pertama dibootstrap melalui prosedur database yang terdokumentasi, dengan audit event dan kenaikan access version. Role akun lainnya diberikan melalui API aplikasi ber-CSRF sebagai koordinator. Assignment PDD juga disimpan melalui UI Pengaturan yang nyata.
- Tidak ada NIM/data akademik pengguna produksi yang dibuat atau diimpor.

## Bukti matriks server-side

Dengan JavaScript browser dimatikan, **91 pemeriksaan SSR lulus**:

- Tujuh route untuk anonymous menghasilkan 401.
- Dua belas akun × tujuh route menghasilkan 200 atau 403 persis sesuai tabel.
- Route: `/`, `/praktikum`, `/elearning`, `/siakad`, `/auditlog`, `/profil`, `/pengaturan`.
- Screenshot koordinator dan PDD diperiksa secara visual: menu koordinator lengkap sesuai seed; menu PDD hanya Ringkasan/Profil. Halaman bukan sekadar blank shell hasil status HTTP.

Bukti sementara berada di `E:/tmp/algohub-staging-accounts-myU3rM/`: `profiles.json`, `role-matrix.json`, dan screenshot. File kredensial tetap hanya pada lokasi privat di root workspace. Hasil SSO/revocation/scope dan status acceptance yang lebih luas dicatat pada [update log](updates/README.md).

## Bukti SSO, scope, dan revocation

Pengujian browser pada layanan staging nyata juga lulus:

- Satu password exchange untuk enam host; enam cookie session berbeda dengan `Secure`, `HttpOnly`, `SameSite=Lax`, dan domain host-only.
- Session/provider token tidak ditemukan di URL, DOM, localStorage, atau sessionStorage yang diperiksa.
- Mutasi tanpa CSRF ditolak 403. Self-edit akses koordinator ditolak 422 sesuai kontrak, tanpa perubahan access version. Header role/divisi palsu tidak memberikan hak tambahan.
- Asisten scoped dapat membaca sesi sintetis kelas A; kelas B tidak muncul pada list dan detail B mengembalikan 404. Kedua sesi tersebut diarsipkan setelah pemeriksaan.
- Penghapusan sementara assignment PDD menghasilkan 403 pada request berikutnya, tanpa menghilangkan autentikasi. Version lama ditolak 409, lalu assignment PDD dikembalikan seperti semula.
- User lain tidak bisa mencabut sesi koordinator. UI pencabutan sesi portal hanya mencabut sesi terpilih. GET halaman logout tidak mengubah state; tombol logout mencabut keluarga login pada enam host dan tidak memengaruhi user lain.
- Password salah dapat diperbaiki tanpa kehilangan deep link `/profil`. Navigasi admin pada viewport 320px dapat dibuka dan ditutup melalui keyboard.
- Enam sesi sisa dari script awal dicabut setelah pemilik menyetujui ID persisnya. Pembacaan read-only membuktikan nol sesi aplikasi aktif untuk ke-12 akun sesudah cleanup; akun dan role dipertahankan.

Script awal sempat mengharapkan 400 untuk self-edit; kontrak, handler, dan unit test sebenarnya menetapkan 422. Assertion diperbaiki ke kontrak tersebut dan full scenario diulang sampai lulus; aplikasi tidak dilonggarkan untuk meloloskan test.

## Tampilan dan deployment final

Keenam frontend staging telah dideploy ulang setelah perbaikan teks status lama. Lint/typecheck, 82 unit frontend, enam build, dan 54/54 Chromium lulus. Smoke setelah deploy pada 11 November 2026 pukul 01:37 WIB membuktikan login/SSO tetap bekerja pada keenam frontend, menu PDD tetap terbatas, serta tidak ada teks yang menyatakan semua fitur domain belum tersedia.

Screenshot desktop/mobile final diperiksa. Untuk screenshot drawer, tunggu transisi CSS selesai; capture saat animasi 180 ms berlangsung dapat membuat tepi kiri tampak terpotong meskipun posisi akhir benar. Pengukuran posisi akhir dan screenshot final menunjukkan drawer sepenuhnya di dalam viewport 320px.

Bukti tambahan: `frontend-deployments.json`, `final-ui-smoke.json`, `admin-final.png`, `portal-final.png`, `pdd-final.png`, dan `coordinator-mobile-final.png` pada direktori sementara yang sama. Seluruh sesi pengujian pada smoke final telah logout; nol sesi aplikasi aktif dikonfirmasi read-only.

Pengujian memakai Playwright yang sudah terpasang karena `chromium-cli` tidak tersedia. Jika pengujian staging ini perlu diulang rutin, alurnya dapat dibakukan melalui `/run-skill-generator`; tidak ada framework preflight atau skill baru yang ditambahkan dalam pekerjaan ini.

## Identitas tambahan untuk baseline kapasitas

Setelah baseline role di atas diverifikasi, pemilik menyetujui target 30 pengguna aktif. Ditambahkan **18 akun mahasiswa sintetis** (`beban.01.staging@algohub.test` sampai `beban.18.staging@algohub.test`), tanpa privilege admin. Password berada pada key `STAGING_LOAD_01_PASSWORD` sampai `STAGING_LOAD_18_PASSWORD` di file privat yang sama.

Total kini **30 identitas Auth dan 30 profile aplikasi**: 12 akun matriks role ditambah 18 mahasiswa beban. Seluruh profile tambahan terbentuk melalui login nyata; assignment mahasiswa diberikan lewat API koordinator, bukan fixture/staging migration. ID profile tidak boleh ditebak dari urutan nama: database sequence dapat melompat karena login/upsert.

Baseline 30-user menjalankan 300 request baca campuran melalui BFF dengan 30 konteks terpisah: 300 HTTP 200, tanpa mismatch identitas, p95 843,69 ms. Warm-up login berada di luar pengukuran; ini bukan load server runner, soak test, atau jaminan kapasitas produksi. Semua konteks pengujian logout; pemeriksaan akhir menemukan nol sesi aplikasi aktif untuk identitas sintetis.

Bukti tambahan berada di `E:/tmp/algohub-load-acceptance-jbFHaP/`, terutama `profiles.json`, `load-results.json`, dan `final-state.json`. Jangan menjalankan provisioning ulang pada identitas yang sudah ada.

## Batas pemakaian

Akun ini untuk staging, bukan akun operasional produksi. Jangan menggunakan password yang sama pada layanan lain atau mengaktifkan pengiriman email ke alamat sintetis. Sebelum memberikan akses staging kepada pihak lain, batasi siapa yang menerima file password dan cabut sesi pengujian yang tidak lagi diperlukan.

Matriks login/authorization tidak menggantikan acceptance seluruh alur Praktikum, E-learning, inventaris/pembayaran, provider runner, atau kapasitas cohort.
