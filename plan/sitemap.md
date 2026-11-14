# Sitemap Target Algohub

Sitemap ini memindahkan fungsi lama `/praktikum` ke host
`praktikum.algohub.web.id`.

## 1. Portal — `algohub.web.id`

- `/`
- `/jadwal`
- `/asisten`
- `/laporan`
- `/modul`
- `/praktikum` — compatibility redirect/CTA ke `praktikum.algohub.web.id`
- `/bantuan`
- `/privacy`
- `/terms`
- `/404`
- `/500`

`/` adalah satu-satunya landing page resmi. `/info` tidak lagi menjadi landing
page; untuk backward compatibility, `/info` diarahkan ke `/` setelah link lama
dimigrasikan. Jika `/info/*` masih diperlukan selama transisi, route tersebut
harus berupa halaman informasi spesifik dan bukan landing page.

## 2. Auth — `auth.algohub.web.id`

- `/login`
- `/callback`
- `/register` — jika registrasi mandiri diizinkan
- `/logout`
- `/lupa-password`
- `/reset-password`
- `/verify-email`
- `/sessions`

Auth hanya mengurus identity dan SSO; data akademik diakses melalui API.

## 3. Centralized admin — `admin.algohub.web.id`

Admin console memakai akun yang sama dengan aplikasi lain. Role `asisten` adalah
akun admin operasional; role `koordinator` dapat memiliki akses lintas-domain
sesuai permission. Tidak ada role auth baru bernama `admin`.

### Shell admin

- `/` — dashboard global dan app switcher
- `/login` — redirect ke auth center bila belum login
- `/profil`
- `/notifikasi`
- `/auditlog` — audit sesuai scope
- `/pengaturan` — pengaturan console

### Konfigurasi Praktikum — `/praktikum`

- `/praktikum` — dashboard praktikum
- `/praktikum/users`
- `/praktikum/asisten`
- `/praktikum/jadwal`
- `/praktikum/pedoman`
- `/praktikum/modul`
- `/praktikum/sesi`
- `/praktikum/soal`
- `/praktikum/penilaian`
- `/praktikum/monitoring`
- `/praktikum/auditlog`
- `/praktikum/pengaturan`

### Konfigurasi E-learning — `/elearning`

- `/elearning` — dashboard e-learning
- `/elearning/users`
- `/elearning/kurikulum`
- `/elearning/assessments`
- `/elearning/bug-hunt`
- `/elearning/monitoring`
- `/elearning/auditlog`
- `/elearning/pengaturan`

### Konfigurasi SIAKAD — `/siakad`

- `/siakad` — dashboard SIAKAD
- `/siakad/users`
- `/siakad/kelas`
- `/siakad/mata-kuliah`
- `/siakad/jadwal`
- `/siakad/absensi`
- `/siakad/nilai`
- `/siakad/inventaris`
- `/siakad/penyewaan`
- `/siakad/laporan`
- `/siakad/role-permission`
- `/siakad/pengaturan`

Semua route di bagian ini berada di host admin, bukan di host domain masing-masing.

## 4. Praktikum — `praktikum.algohub.web.id`

### Mahasiswa

- `/`
- `/login`
- `/register`
- `/lupa-password`
- `/reset-password`
- `/dashboard`
- `/profil`
- `/sesi`
- `/sesi/[sesiId]`
- `/sesi/[sesiId]/pretest`
- `/sesi/[sesiId]/posttest`
- `/sesi/[sesiId]/keterampilan`
- `/sesi/[sesiId]/ujian`
- `/nilai`
- `/notifikasi`

Tidak ada `/admin` di host praktikum. Asisten berpindah ke
`admin.algohub.web.id/praktikum` melalui SSO.

## 5. E-learning — `elearning.algohub.web.id`

- `/`
- `/courses`
- `/courses/[levelId]`
- `/courses/[levelId]/[moduleId]`
- `/lesson/[lessonId]`
- `/playground`
- `/leaderboard`
- `/profile`
- `/achievements`
- `/bug-hunt`
- `/assessments`
- `/monitoring`
- `/auditlog`
- `/terminal-demo`

Tidak ada `/admin` di host e-learning. Administrasi dilakukan melalui
`admin.algohub.web.id/elearning`.

## 6. SIAKAD — `siakad.algohub.web.id`

Route final wajib dicocokkan dengan audit UI production. Struktur awal:

- `/`
- `/dashboard`
- `/profil`
- `/jadwal`
- `/absensi`
- `/kelas`
- `/mata-kuliah`
- `/nilai`
- `/inventaris`
- `/penyewaan`
- `/laporan`
Route yang belum terverifikasi tidak dianggap sebagai fitur parity.

Tidak ada `/admin` di host SIAKAD. Administrasi dilakukan melalui
`admin.algohub.web.id/siakad`.

## 7. Navigasi lintas host

- Header menampilkan user, role, notifikasi, dan menu pindah aplikasi.
- Deep link dipertahankan setelah SSO.
- Tidak ada JWT di query string.
- Route guard frontend hanya untuk UX; API menjadi sumber keputusan permission.
- File privat dibuka dengan signed URL.

Admin app switcher hanya menampilkan domain yang boleh diakses oleh permission
user. Link admin dari aplikasi domain mengarah ke host terpusat, bukan ke route
`/admin` lokal.

## 8. Responsive behavior

- Mobile menggunakan single-column layout, compact navigation, dan tabel yang
  berubah menjadi card/list atau horizontal scroll yang terkontrol.
- Tablet menggunakan grid dua kolom bila ruang cukup.
- Desktop menggunakan sidebar dashboard, topbar, dan grid multi-kolom.
- Editor kode, timer, status auto-save, dan tombol submit tetap usable pada
  viewport kecil; tidak boleh mengandalkan hover atau fixed width.
- Semua halaman diuji pada minimal 320 px, 375 px, 768 px, 1024 px, dan 1440 px.
