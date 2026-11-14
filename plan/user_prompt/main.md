# Master Execution Brief — Rewrite Algohub

## 1. Instruksi utama

Gunakan dokumen ini sebagai prompt utama untuk AI eksekutor project rewrite
Algohub. Baca dokumen planning yang ditautkan sebelum membuat keputusan teknis.

**Fase saat ini: planning.** Jangan mengubah source code aplikasi, database
production, DNS, deployment, atau data migrasi sebelum ada instruksi eksplisit
untuk masuk fase implementasi.

## 2. Konteks project

- Produk production: `algohub.web.id`
- Source project existing untuk audit:
  `E:\rekapProject\project_ap`
- Workspace planning rewrite:
  `E:\rekapProject\project_ap_v2`
- Target user harian: 50–200 user per hari.
- Fitur rewrite harus parity dengan production setelah diaudit.
- Jangan menganggap route, fitur, role, RPC, atau schema sebagai final hanya dari
  rancangan; validasi terhadap project existing dan production.

## 3. Tujuan rewrite

Rewrite seluruh sistem dengan stack berikut:

- Frontend: SvelteKit
- Backend: Go
- Database: PostgreSQL melalui Supabase
- Realtime: Supabase Realtime atau managed WebSocket
- Deployment: frontend dan Go API stateless di Vercel Hobby setelah diuji
- Storage: bucket terpisah per domain
- Database relational hanya satu: satu Supabase PostgreSQL project baru/fresh.
  Project legacy menjadi sumber migrasi read-only. Redis-compatible
  counter boleh dipakai sebagai infrastructure rate-limit ephemeral, bukan DB
  domain kedua.

Praktikum dipindahkan dari:

```text
algohub.web.id/praktikum
```

menjadi:

```text
praktikum.algohub.web.id
```

## 4. Domain target

| Host | Fungsi |
|---|---|
| `algohub.web.id` | Satu-satunya landing page dan portal publik |
| `siakad.algohub.web.id` | SIAKAD dan administrasi akademik/laboratorium |
| `elearning.algohub.web.id` | Materi, progres, gamifikasi, dan pembelajaran |
| `praktikum.algohub.web.id` | Sesi praktikum, coding, asesmen, dan nilai |
| `admin.algohub.web.id` | Console administrasi terpusat |
| `auth.algohub.web.id` | Satu sistem login, SSO, callback, logout, recovery |
| `api.algohub.web.id` | Go API bersama |

## 5. Aturan auth dan Storage

### Auth

- AUTH hanya satu sistem untuk seluruh subdomain.
- Gunakan satu identity namespace Supabase Auth.
- Satu user dapat berpindah portal, SIAKAD, E-learning, Praktikum, dan Admin
  melalui SSO.
- Jangan menaruh token di URL atau `localStorage`.
- `return_to` callback harus allowlisted.
- Gunakan cookie yang aman (`Secure`, `HttpOnly`, `SameSite=Lax`) sesuai desain
  SSO.
- `admin` bukan role identity baru.

### Role admin

- Akun admin adalah akun dengan role `asisten`.
- `koordinator` dapat memiliki permission administratif yang lebih luas.
- Divisi asisten: `sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, `pdd`.
  Divisi adalah assignment permission/scope, bukan role identity baru.
- Permission dan scope menentukan menu serta aksi yang tersedia.
- Jangan membuat user table, password, atau auth system kedua untuk admin.

### Bucket

Bucket dipisahkan berdasarkan domain:

- `portal`
- `siakad`
- `elearning`
- `praktikum`

Bucket berbeda dipakai untuk isolasi permission dan lifecycle. Jangan menyebut
pemisahan bucket sebagai load balancing otomatis.

## 6. Centralized admin console

Semua administrasi dilakukan di `admin.algohub.web.id`.

```text
admin.algohub.web.id/
├── /praktikum
├── /elearning
└── /siakad
```

Tidak boleh ada lagi halaman admin pada:

- `algohub.web.id/admin`
- `siakad.algohub.web.id/admin`
- `elearning.algohub.web.id/admin`
- `praktikum.algohub.web.id/admin`

Modul utama:

- `/praktikum`: users, asisten, jadwal, pedoman, modul, sesi, soal, penilaian,
  monitoring, audit log, pengaturan.
- `/elearning`: users, kurikulum, assessments, Bug Hunt, monitoring, audit log,
  pengaturan.
- `/siakad`: users, kelas, mata kuliah, jadwal, absensi, nilai, inventaris,
  penyewaan, laporan, role-permission, pengaturan.

Host admin bukan authorization bypass. Setiap request tetap memeriksa role,
permission, scope, ownership, dan audit event.

## 7. Landing page dan UI/UX

### Landing page

Landing page resmi hanya berada di:

```text
https://algohub.web.id/
```

`/info` tidak lagi menjadi landing page. Route lama boleh menjadi compatibility
redirect ke `/`. Halaman publik spesifik memakai route eksplisit seperti
`/jadwal`, `/asisten`, `/laporan`, dan `/modul`.

Referensi landing page:

- URL: `https://8b.com/templates/burrow/demo.html`
- Local reference: `E:\rekapProject\project_ap_v2\plan\ref\landingPage`

Gunakan referensi untuk layout, whitespace, storytelling, dan hierarchy. Jangan
menyalin copy, foto, asset, atau identitas veterinary template.

### Dashboard

Referensi dashboard:

```text
https://codedthemes.com/demos/admin-templates/gradient-able/react/free/app/dashboard/analytics
```

Gunakan arah layout analytics seperti sidebar, topbar, metric cards, table,
chart, filter, activity, dan status. Bangun design system Algohub sendiri.

### Anti-AI-slop

- Design DNA: **calm technical lab**.
- Landing bersifat editorial dan berbasis konteks Algohub.
- Dashboard bersifat utilitarian dan informatif.
- Gradient, glassmorphism, blob, dan animasi bukan dekorasi default.
- Gunakan copy Bahasa Indonesia yang spesifik untuk mahasiswa, asisten, kelas,
  sesi, coding, dan laboratorium.
- Gunakan motif khas Algohub seperti flowchart, execution trace, notebook lab,
  atau code structure secara konsisten.
- Jangan generate satu halaman penuh tanpa design brief dan review.
- Pilih komponen karena kebutuhan UI nyata, bukan karena populer.

### Responsive frontend

Semua UI harus mobile-first dan dinamis untuk:

- 320–374 px
- 375–767 px
- 768–1023 px
- 1024–1439 px
- 1440 px ke atas

Wajib diperhatikan:

- sidebar menjadi drawer/compact navigation pada mobile;
- tabel menjadi card, accordion, atau horizontal scroll yang terkontrol;
- modal, form, editor, timer, autosave, dan submit tetap usable di mobile;
- touch target minimal 44×44 px;
- tidak ada horizontal overflow yang tidak disengaja;
- keyboard navigation, focus state, contrast, semantic HTML, dan reduced motion.

## 8. Backend Go dan clean architecture

Istilah `cmd.go` yang dimaksud diterapkan sebagai pola Go idiomatis:

```text
backend/
  cmd/
    api/main.go
    worker/main.go
    migrate/main.go
  internal/
    delivery/http/
    middleware/
    usecase/
    repository/
    entity/
    authorization/
    platform/
```

Alur wajib:

```text
router → middleware → handler → usecase → repository → PostgreSQL/Supabase
```

- `cmd/api/main.go` adalah composition root, bukan tempat seluruh business logic.
- Handler tidak berisi query SQL.
- Semua API domain melewati Go API.
- Browser tidak melakukan query database privat secara langsung.
- Serverless adapter harus memakai builder/application yang sama.
- OpenAPI menjadi source of truth endpoint, request, response, dan error.
- Go API berjalan stateless sebagai Vercel Function; jangan mengandalkan memory
  instance, persistent goroutine, atau WebSocket persisten.
- Cron memanggil endpoint idempotent untuk auto-submit/reconciliation. Code runner
  memakai provider sandbox eksternal melalui Go API.

## 9. Backend protection wajib

1. **Rate limit:** berlapis per IP, user/session, route, resource sensitif, dan
   concurrency. Pelanggaran mengembalikan `429` + `Retry-After`.
2. **Database abuse:** auth dan rate limit berjalan sebelum query domain. Public
   endpoint harus allowlisted, dibatasi, dan di-cache bila aman.
3. **XSS:** escaping Svelte, sanitizer rich text server-side, CSP, dan tidak ada
   unsanitized HTML.
4. **IDOR/BOLA:** ID URL bukan bukti akses; usecase memeriksa user, ownership,
   permission, dan scope.
5. **SQL injection:** parameter binding dan allowlist untuk sort/filter/column.
6. **CSRF/CORS:** cookie protection, CSRF protection untuk mutation, dan CORS
   hanya untuk origin Algohub.
7. **Upload/SSRF:** bucket/object key ditentukan server, MIME/size validation,
   signed URL, dan allowlist URL eksternal.
8. **Caching:** asset publik boleh CDN cache; auth, nilai, presensi, permission,
   attempt, dan data user tidak boleh public-cache.
9. **Memory:** context timeout, bounded body/result, streaming export, bounded
   worker queue, controlled goroutine, DB pool limit, dan sandbox code runner.
10. **Middleware:** request ID, recovery, security headers, body limit, timeout,
    edge rate limit, auth, user rate limit, authorization, handler, metrics.

## 10. Waktu praktikum dan coding terminal

- Zona waktu bisnis canonical: `Asia/Jakarta` / `UTC+07:00` / GMT+7.
- Database menyimpan waktu sebagai UTC `timestamptz`.
- `started_at`, `deadline_at`, dan `server_now` dibuat dari server/database.
- Jam perangkat user tidak pernah dipercaya.
- Client hanya menampilkan countdown dari deadline server.
- Refresh atau perubahan clock device tidak memperpanjang waktu.
- Auto-submit, submit manual, dan penutupan admin menggunakan state transition
  transaksional dan idempotent.
- Server, database, worker, dan cron harus sinkron NTP.

## 11. Realtime

Gunakan Supabase Realtime atau managed WebSocket. Vercel serverless tidak dipakai
untuk koneksi WebSocket persisten. Channel harus scoped, misalnya:

- `user:{userId}`
- `class:{classId}`
- `practicum:session:{sessionId}`
- `assessment:{attemptId}`

Client wajib reconnect, deduplicate event, dan refetch setelah reconnect. Event
bukan sumber kebenaran transaksi; database tetap menjadi sumber kebenaran.

## 12. MCP 21st

MCP 21st terdaftar pada `opencode.jsonc` project.

- Endpoint: `https://21st.dev/api/mcp`
- Credential: environment `API_KEY_21ST`
- Jangan menulis API key ke prompt, source code, dokumentasi, atau commit.
- Gunakan `search` dan `get_inspiration` untuk membandingkan alternatif.
- Gunakan `get_component`/`get_theme` hanya setelah komponen dipilih.
- Prioritaskan primitive/section yang dapat diadaptasi ke shared UI.
- Jangan memasang full template monolitik.
- Catat dependency, license, alasan pemilihan, dan hasil responsive/accessibility
  review.
- Komponen tidak boleh membawa business logic atau query database.

Kategori pencarian awal:

- landing: editorial hero, feature steps, service cards, proof/statistics,
  responsive navigation, FAQ;
- dashboard: app shell, sidebar, KPI cards, data table, chart, activity feed,
  notification center, empty/error state, app switcher;
- admin: RBAC matrix, scope selector, audit log, monitoring queue, settings,
  command palette, bulk action;
- praktikum: split editor, resizable panel, timer, autosave status, terminal,
  test result, reconnect banner, question navigator, mobile stacked editor.

## 13. Dokumen referensi wajib

1. [PRD dan arsitektur](../prd.md)
2. [Sitemap](../sitemap.md)
3. [Kontrak API](../api.md)
4. [ERD target](../erd.md)
5. [UI/UX dan frontend](../ui-ux.md)
6. [Backend, security, dan operasi](../backend.md)
7. [Workflow MCP 21st](../21st-mcp.md)
8. [Feature parity matrix](../feature-matrix.md)
9. [Role dan permission](../roles-permissions.md)
10. [Database dan migrasi](../database-migration.md)
11. [Database operations](../database-operations.md)
12. [Deployment dan CI/CD](../deployment.md)
13. [Code runner](../code-runner.md)
14. [OpenAPI dan `/docs`](../openapi.md)
15. [Testing dan debugging](../testing.md)
16. [Execution phases](../phases.md)
17. [Planning changelog](../changelog.md)
18. [Preflight checklist](../preflight.md)

## 14. Output planning yang diharapkan

Sebelum implementasi dimulai, AI eksekutor harus menghasilkan dan memvalidasi:

- feature parity matrix dari production;
- sitemap final;
- API/OpenAPI endpoint matrix;
- ERD dan mapping schema legacy;
- role/permission/scope matrix;
- design tokens dan UI component inventory;
- strategi auth SSO dan Storage bucket;
- security threat/control matrix;
- migration, backup, rollback, dan cutover plan;
- database operations: RLS, pooling, index, backup/restore, quota, dan retention;
- acceptance criteria untuk mobile, desktop, security, realtime, dan deadline.

## 15. Quality dan folder freeze

Setiap implementasi harus membawa test yang relevan pada unit, integration,
contract, E2E, security, dan performance layer. Error harus dapat ditelusuri dari
`X-Request-ID`, deployment SHA, operation ID, dan artifact CI tanpa membocorkan
secret atau data sensitif.

Saat user memberi instruksi masuk fase eksekusi, jangan mengubah, memindahkan,
menghapus, atau menambah file di `plan/**`, kecuali file di
`plan/user_prompt/**`. Source code, migration, dan konfigurasi implementasi
berada di luar folder `plan`.

## 16. Prompt eksekusi per fase

- [Prompt Phase 2 — Identity, SSO, Authorization, dan Admin Shell](./phase-2.md)
- [Prompt Phase 3 — Praktikum Core](./phase-3.md)
