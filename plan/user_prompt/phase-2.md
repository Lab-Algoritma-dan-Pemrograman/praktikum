# Prompt Eksekusi Phase 2 — Identity, SSO, Authorization, dan Admin Shell

> File ini adalah prompt siap pakai setelah Phase 1 selesai dan disetujui.

```text
Phase 1 sudah selesai dan sudah disetujui. Lanjutkan ke Phase 2 sesuai
plan/phases.md: Identity, SSO, Authorization, dan Centralized Admin Shell.

Bertindak sebagai lead full-stack developer. Kerjakan implementasi langsung pada
workspace rewrite saat ini. Jangan mengubah project lama di:
E:\rekapProject\project_ap

## Dokumen yang wajib dibaca

Baca dan pahami dokumen berikut sebelum mengubah source code:

1. plan/user_prompt/main.md
2. plan/phases.md
3. plan/preflight.md
4. plan/roles-permissions.md
5. plan/database-migration.md
6. plan/database-operations.md
7. plan/prd.md
8. plan/sitemap.md
9. plan/api.md
10. plan/openapi.md
11. plan/erd.md
12. plan/backend.md
13. plan/testing.md
14. plan/ui-ux.md

Periksa hasil implementasi Phase 1 terlebih dahulu. Pertahankan struktur,
konvensi, dependency, konfigurasi, dan test yang sudah ada. Jangan mengulang,
menghapus, atau mengganti fondasi Phase 1 tanpa alasan teknis yang dibuktikan.

## Scope Phase 2

### 1. Identity dan profile

- Integrasikan satu Supabase Auth sebagai identity provider.
- Hubungkan profile aplikasi `users` dengan `auth.users`.
- Implementasikan role kanonik: `koordinator`, `asisten`, `mahasiswa`, dan
  `peminjam`.
- Implementasikan divisi asisten: `sekretaris`, `bendahara`, `k3`, `labops`,
  `pendidikan`, dan `pdd`.
- Jangan membuat role identity bernama `admin`, `kordas`, `praktikan`, atau
  `user`. Alias legacy hanya boleh ditangani pada boundary migrasi.
- Profile, role assignment, division assignment, permission, dan scope harus
  memiliki model, migration, repository, usecase, dan API contract yang jelas.
- Seed role/division/permission harus idempotent dan tidak memakai data produksi.

### 2. SSO dan session

Implementasikan flow login lintas host berikut:

- `auth.algohub.web.id`
- `algohub.web.id`
- `admin.algohub.web.id`
- `siakad.algohub.web.id`
- `elearning.algohub.web.id`
- `praktikum.algohub.web.id`

Wajib:

- gunakan satu identity namespace dan satu auth system;
- gunakan cookie `Secure`, `HttpOnly`, dan `SameSite=Lax` sesuai desain;
- jangan menyimpan access token, refresh token, atau credential di URL maupun
  `localStorage`;
- `return_to` harus melalui allowlist origin/path dan menolak nilai eksternal;
- cegah session fixation saat login, callback, refresh, dan pergantian user;
- sediakan logout dan revoke session;
- mutation berbasis cookie wajib memiliki CSRF protection;
- CORS hanya mengizinkan origin Algohub yang diperlukan;
- session tidak boleh bocor antar-user atau antar-environment;
- gunakan local/staging environment bila credential belum tersedia;
- jangan membuat atau mengarang secret;
- jangan mengubah Supabase production, DNS, atau deployment production tanpa
  instruksi eksplisit.

Jika konfigurasi multi-host belum bisa dijalankan secara lokal, buat adapter dan
configuration contract yang dapat diuji dengan host fixture/local domains. Catat
blocker credential atau DNS secara spesifik, tetapi tetap implementasikan dan uji
bagian yang dapat diverifikasi secara lokal.

### 3. Authorization backend

Implementasikan urutan authorization berikut pada Go API/usecase:

Authorization chain: user → role assignment → division → permission → scope →
resource ownership → action.

Minimal dukung scope: domain, institution, division, class, shift, dan practicum
session.

Aturan:

- authorization utama berada di backend, usecase, repository policy, dan RLS;
- frontend hanya menggunakan permission untuk menu dan UX, bukan security
  boundary;
- host `admin.algohub.web.id` bukan authorization bypass;
- ID pada URL bukan bukti akses;
- resource yang tidak boleh dienumerasi dapat mengembalikan `404` sesuai kontrak;
- akses tanpa login mengembalikan `401`, akses tanpa izin mengembalikan `403`;
- semua keputusan authorization penting dapat ditelusuri dengan request ID;
- gunakan transaction bila perubahan role, division, permission, atau scope
  memengaruhi beberapa tabel;
- API dan schema wajib mengikuti OpenAPI 3.1 yang sudah ditetapkan.

### 4. Centralized admin shell

Bangun shell awal pada:

Routes:

- `admin.algohub.web.id/`
- `/praktikum`
- `/elearning`
- `/siakad`
- `/auditlog`
- `/profil`
- `/pengaturan`

Shell harus memiliki app switcher, domain/scope selector, permission-aware
navigation, breadcrumb, user profile, logout, loading/empty/unauthorized/error
state, retry yang aman, responsive mobile drawer, keyboard navigation, visible
focus state, semantic HTML, dan accessible labels.

Layout wajib usable pada 320 px, 768 px, dan desktop. Implementasikan shell dan
authorization boundary saja. Jangan mengerjakan fitur domain penuh Phase 3/4
seperti CRUD praktikum, e-learning, SIAKAD, code runner, assessment, atau
migrasi data produksi.

### 5. Audit event

Buat audit event untuk login, logout, revoke session, perubahan role, perubahan
divisi, perubahan permission, perubahan scope, perubahan profile, akses admin
yang ditolak, dan perubahan konfigurasi admin.

Audit event minimal menyimpan actor, action, app/domain, subject, scope, request
ID, timestamp UTC, dan before/after ringkas yang sudah disanitasi.

Jangan menyimpan password, access token, refresh token, service role key, atau
secret dalam audit event dan log.

## Test wajib

Tambahkan dan jalankan test yang relevan. Jangan menyatakan test lulus jika belum
dijalankan.

### Auth dan session

- login berhasil dan password salah;
- session expired, logout, dan revoke session;
- session fixation;
- callback code expired atau invalid;
- `return_to` tidak valid dan origin ilegal;
- token tidak muncul di URL atau `localStorage`;
- session isolation antar-user;
- CSRF pada mutation berbasis cookie;
- secure cookie flags dan CORS allowlist.

### SSO

- callback lintas application host;
- login silang untuk portal, SIAKAD, E-learning, Praktikum, dan Admin;
- logout dari satu host sesuai kontrak session;
- callback dengan host/return path ilegal;
- callback replay dan code reuse;
- session tidak bocor antar-host atau antar-user.

### Authorization dan security

Buat matrix test untuk anonymous user, mahasiswa, asisten tanpa divisi, asisten
setiap divisi (`sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, `pdd`),
asisten dengan scope berbeda, koordinator, dan peminjam.

Uji minimal `401`, `403`, `404` untuk resource yang tidak boleh dienumerasi,
IDOR/BOLA, privilege escalation, direct URL ke halaman admin, mutation tanpa
permission, akses admin lintas scope, role/division spoofing dari client, RLS
bypass, penggunaan service role yang salah, dan audit event untuk allow/deny yang
diwajibkan.

### Frontend

- navigation berdasarkan permission;
- direct route guard;
- admin shell loading/empty/error/unauthorized state;
- mobile drawer;
- keyboard navigation dan focus state;
- responsive layout minimal 320 px, 768 px, dan desktop;
- tidak ada token atau secret di bundle/browser storage;
- logout dan session-expired UX.

### Database

- migration dan seed test;
- profile terhubung ke `auth.users`;
- unique email/NIM sesuai kontrak;
- role/division/permission seed idempotent;
- foreign key dan cascade/revoke behavior;
- RLS anonymous/authenticated;
- RLS antar-user dan antar-scope;
- transaction rollback atau forward-compatibility;
- audit event sanitization dan indexing;
- schema drift check.

### API dan quality

- OpenAPI lint;
- contract test request/response/error;
- request ID pada response dan log;
- error mapping `401`/`403`/`404`/`409`/`422` yang konsisten;
- unit test usecase/authorization;
- integration test repository/API;
- E2E smoke test admin shell;
- formatter, lint, typecheck, build, dan test suite yang tersedia.

## Aturan kerja

- Jangan mengubah project lama di `E:\rekapProject\project_ap`.
- Jangan mengubah, memindahkan, menghapus, atau menambah file di `plan/**`,
  kecuali file di `plan/user_prompt/**` bila memang diperlukan untuk prompt.
- Jangan mengubah database production, DNS, deployment, atau data migrasi
  production tanpa instruksi eksplisit.
- Jangan membuat role `admin`.
- Jangan menaruh service role key atau credential di frontend, repository, test
  artifact, screenshot, atau dokumentasi.
- Jangan menambahkan fitur Phase 3 atau Phase 4.
- Jangan menghapus pekerjaan existing tanpa membaca dan memahami dampaknya.
- Jangan menonaktifkan atau menghapus test untuk membuat build lulus.
- Jangan mengarang hasil audit, hasil test, credential, atau konfigurasi provider.
- Jika ada blocker, implementasikan bagian yang dapat diuji, lalu laporkan blocker
  beserta file/configuration yang dibutuhkan.

## Definition of Done

Phase 2 hanya boleh dinyatakan selesai jika:

1. Auth dan profile berjalan di local/staging atau keterbatasannya terdokumentasi.
2. SSO contract lintas host memiliki implementasi dan test.
3. Role, division, permission, dan scope diverifikasi di backend.
4. Centralized admin shell menolak akses tanpa permission.
5. Audit event tercatat dan tidak membocorkan secret/data sensitif.
6. Test auth, SSO, authorization, database, frontend, security, dan responsive
   dijalankan dan hasilnya dilaporkan.
7. OpenAPI, migration, seed, dan konfigurasi environment terdokumentasi.
8. Error dapat ditelusuri dengan `X-Request-ID`, operation ID, dan deployment SHA
   bila tersedia.
9. Tidak ada secret masuk repository atau browser bundle.
10. Tidak ada perubahan pada project legacy atau production tanpa otorisasi.
11. Scope Phase 3/4 belum dikerjakan.

## Format laporan akhir

Laporkan secara ringkas:

1. ringkasan implementasi;
2. daftar file yang berubah;
3. migration/seed yang ditambahkan;
4. endpoint dan route yang ditambahkan;
5. command formatter, lint, typecheck, build, dan test;
6. hasil setiap command, termasuk failure yang belum terselesaikan;
7. security checks yang dijalankan;
8. blocker credential/DNS/provider bila ada;
9. risiko atau keputusan yang membutuhkan user;
10. rekomendasi dan prerequisite Phase 3.

Jangan lanjut ke Phase 3 sebelum Phase 2, test, dan laporan ini selesai.
```
