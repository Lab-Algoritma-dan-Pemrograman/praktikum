# PRD dan Arsitektur Target Algohub

**Status:** Draft 0.1  
**Target trafik awal:** 50–200 user per hari

## 1. Tujuan

Rewrite harus mempertahankan fitur production pada tiga area:

1. Portal dan praktikum laboratorium.
2. Sistem akademik/laboratorium pada SIAKAD.
3. E-learning interaktif dan gamifikasi.

Perubahan utama adalah memindahkan praktikum dari
`algohub.web.id/praktikum` ke `praktikum.algohub.web.id`, memakai satu auth lintas
subdomain, memakai satu Supabase PostgreSQL project, dan menetapkan satu landing
page resmi di `algohub.web.id/`. Storage tetap dipisah secara logical per domain
untuk policy dan lifecycle.

## 2. Domain target

| Host | Fungsi |
|---|---|
| `algohub.web.id` | Satu-satunya landing page dan halaman publik utama |
| `siakad.algohub.web.id` | Administrasi akademik/laboratorium |
| `elearning.algohub.web.id` | Materi, progres, gamifikasi, dan asesmen belajar |
| `praktikum.algohub.web.id` | Sesi praktikum, soal, pengerjaan, dan nilai |
| `admin.algohub.web.id` | Console administrasi terpusat untuk semua domain |
| `auth.algohub.web.id` | Login, SSO, callback, logout, dan recovery |
| `api.algohub.web.id` | Go API bersama |

`algohub.web.id/info` bukan landing page. Route tersebut menjadi compatibility
redirect ke `/` atau dipensiunkan setelah seluruh link lama diperbarui. Informasi
publik yang tetap diperlukan dipetakan ke route eksplisit seperti `/jadwal`,
`/asisten`, `/laporan`, dan `/modul`.

## 3. Scope fitur parity

### 3.1 Portal dan praktikum

- Lobby, pengumuman, quick links, jadwal, daftar asisten, pedoman laporan, dan
  modul PDF.
- Dashboard mahasiswa, profil, jadwal, sesi, presensi/token, dan nilai.
- Pre-test, post-test, program keterampilan, dan ujian praktik.
- Timer server-authoritative, auto-save, auto-submit, editor kode, flowchart,
  eksekusi kode, dan upload lampiran.
- Admin: user, asisten, jadwal, sesi, course, bank soal, aktivasi kelas/shift,
  penilaian, export, dan AI grading.

### 3.2 E-learning

- Level → modul → lesson.
- Rich text/Markdown, KaTeX, code editor, C/Python runner, quiz, test case,
  playground, dan course explorer.
- XP, level, streak, achievement, leaderboard, dan Bug Hunt.
- Asesmen, monitoring realtime, audit log, serta pengelolaan kurikulum.

### 3.3 SIAKAD/laboratorium

Fitur aktual harus dipetakan dari UI, API, RPC, migration, dan database
production. Baseline audit mencakup jadwal, absensi, inventaris, penyewaan alat,
feedback, laporan, dan integrasi/notifikasi.

Mesin asesmen hanya satu sebagai sumber nilai. E-learning tidak boleh membuat
sumber nilai kedua yang berbeda dari praktikum.

## 4. Role dan permission

| Role target | Cakupan |
|---|---|
| `koordinator` | Administrasi global dan konfigurasi lintas domain |
| `asisten` | Operasional kelas/sesi sesuai divisi dan scope |
| `mahasiswa` | Materi, asesmen, praktikum, presensi, dan data pribadi |
| `peminjam` | Penyewaan alat untuk pengguna tanpa NIM |

Permission berasal dari tabel permission dan scope, bukan dari nama divisi atau
suffix kode asisten. Scope dapat berupa kelas, shift, sesi, divisi, atau institusi.

Divisi operasional yang ditemukan pada project lama dan dipertahankan dalam target
adalah `sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, dan `pdd`.
`sekretaris`/`bendahara`/`k3` bukan role identity baru; semua tetap memakai akun
role `asisten` dengan explicit permission grant dan scope.

### 4.1 Identitas admin

`admin` adalah konteks aplikasi, bukan role identity baru. Akun dengan role
`asisten` dapat login ke `admin.algohub.web.id`; menu dan aksi yang tersedia
ditentukan oleh permission serta scope asisten tersebut. `koordinator` dapat
memiliki permission administratif yang lebih luas. Tidak ada password, tabel user,
atau sistem auth kedua khusus admin.

Semua perubahan konfigurasi dari console admin menyimpan actor, domain, scope,
resource, before/after ringkas, waktu, dan request ID ke audit log.

## 5. Arsitektur teknis

```text
Browser
  ├─ algohub.web.id / siakad / elearning / praktikum (SvelteKit)
  ├─ admin.algohub.web.id (centralized admin SvelteKit)
  └─ auth.algohub.web.id (SSO)
          │
          └── api.algohub.web.id (Go stateless)
                    ├── Supabase Auth
                    ├── PostgreSQL
                    ├── Supabase Storage
                    └── Realtime / managed WebSocket
```

Semua operasi domain melewati Go API. Browser tidak melakukan query database
Supabase langsung. Jalur request standar adalah:

```text
HTTP request → router → middleware → handler/delivery → usecase/service
            → repository interface → PostgreSQL/Supabase
```

Admin console tetap memakai API domain yang sama, tetapi UI admin terpusat berada
di host `admin.algohub.web.id` dan endpoint administrasinya memakai namespace
`/v1/admin/{domain}`. Host domain pengguna tidak memiliki halaman `/admin`.

### 5.1 Vercel Hobby dan WebSocket

- SvelteKit cocok dideploy ke Vercel.
- Go API diputuskan berjalan sebagai Vercel Serverless Function stateless setelah
  batas durasi, cold-start, dan koneksi database diuji.
- Tidak ada worker daemon atau goroutine persisten. Cron menangani pekerjaan
  terjadwal melalui endpoint idempotent; code runner memakai sandbox eksternal.
- Vercel serverless tidak dipakai untuk WebSocket persisten. Gunakan Supabase
  Realtime atau provider WebSocket terkelola.
- Cron auto-submit/reconciliation harus idempotent dan memakai secret terpisah.

### 5.5 Testing dan debugging

Setiap domain memiliki unit, integration/contract, E2E, security, dan performance
test. CI menyimpan request ID, deployment SHA, operation ID, coverage, screenshot,
video, migration report, dan structured log yang sudah disanitasi. Lihat
[testing.md](./testing.md), [deployment.md](./deployment.md), dan
[phases.md](./phases.md).

### 5.2 Auth lintas subdomain

Satu Supabase Auth project menjadi sumber identity. Login dipusatkan di
`auth.algohub.web.id`; setelah berhasil, aplikasi tujuan menerima one-time code
dan membuat cookie session host-only.

```text
app → auth.algohub.web.id/login?return_to=app
    → login Supabase Auth
    → one-time code ke callback app
    → app backend menukar code
    → cookie Secure + HttpOnly + SameSite=Lax pada host app
```

Dengan ini satu login berlaku di semua subdomain tanpa token di URL atau
`localStorage`. Parameter `return_to` wajib allowlisted. Cookie
`Domain=.algohub.web.id` boleh dipertimbangkan kemudian, tetapi memperluas dampak
kompromi satu subdomain ke subdomain lain.

### 5.3 Waktu dan deadline praktikum

- Zona waktu bisnis canonical: `Asia/Jakarta`, offset `UTC+07:00` (GMT+7).
- Database menyimpan instant sebagai `timestamptz`/UTC.
- Jadwal tanggal/jam lokal dikonversi memakai `Asia/Jakarta` sebelum disimpan.
- Deadline attempt dihitung server-side dari `started_at` dan durasi tersimpan;
  jam device tidak pernah dipercaya.
- API mengembalikan `server_now`, `started_at`, `deadline_at`, dan `timezone`.
- Browser hanya menampilkan countdown dari nilai server dan resync berkala.
- Server, database, worker, dan cron wajib sinkron NTP.

### 5.4 Storage dan caching

| Subdomain | Bucket |
|---|---|
| Portal | `portal` |
| SIAKAD | `siakad` |
| E-learning | `elearning` |
| Praktikum | `praktikum` |

File privat memakai signed URL berumur pendek. Asset publik immutable boleh
memakai CDN cache. Response personal, nilai, presensi, permission, dan auth tidak
boleh di-cache publik.

Bucket berbeda membantu isolasi permission dan lifecycle. Bucket saja bukan
mekanisme load balancing; kapasitas Storage dan CDN tetap perlu dipantau.

## 6. Persyaratan non-fungsional

- HTTPS semua host.
- API stateless, pagination, request ID, rate limit, dan structured logging.
- Rate limit berlapis menghasilkan `429 Too Many Requests` dan `Retry-After`.
- `admin.algohub.web.id` memakai rate limit dan session policy yang lebih ketat
  daripada halaman publik.
- Endpoint privat tidak dapat memicu query domain sebelum authentication dan
  authorization dasar selesai.
- XSS, CSRF, SQL injection, SSRF, IDOR, dan insecure file upload memiliki kontrol
  eksplisit di middleware/usecase.
- Body size, timeout, concurrency, result set, dan upload size memiliki batas.
- Caching dibedakan antara asset publik, response publik, response privat, dan
  data transaksi; data user tidak boleh tercampur antar-user.
- RLS aktif sebagai defense-in-depth; service role tidak boleh ke browser.
- Perubahan role, nilai, presensi, submission, dan file masuk audit log.
- Waktu disimpan UTC dan dikirim ISO-8601.
- Backup dan restore database diuji sebelum migrasi.
- Target awal p95 API normal < 500 ms dan error rate < 1%.

## 7. Tahapan kerja

1. Audit parity: route, fitur, role, RPC, tabel, Storage, cron, dan integrasi.
2. Foundation: monorepo SvelteKit, Go API dengan clean architecture, migration,
   auth SSO, UI package, environment, dan observability.
3. Migrasi identity, role, permission, scope, audit, dan file metadata.
4. Migrasi mesin asesmen dan domain praktikum.
5. Migrasi e-learning/gamifikasi.
6. Migrasi SIAKAD/laboratorium.
7. Import data, UAT, pilot, DNS cutover, monitoring, dan rollback.

## 8. Gate sebelum implementasi penuh

- Feature matrix production disetujui.
- Mapping role lama ke role target disetujui.
- Strategi satu Supabase project dan migrasi data disetujui.
- SSO lintas lima application host lulus browser test: portal, admin, SIAKAD,
  E-learning, dan Praktikum.
- Retention, backup, upload limit, rate limit, dan rollback disetujui.
- Mobile, tablet, desktop, keyboard navigation, dan reduced-motion UI review lulus.
- Security test untuk auth boundary, IDOR, XSS, rate limit, dan deadline lulus.
