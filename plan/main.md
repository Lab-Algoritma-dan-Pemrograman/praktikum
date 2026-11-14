# Algohub Rewrite — Planning Hub

Dokumen ini adalah index utama planning rewrite Algohub. Instruksi lengkap untuk
AI eksekutor berada di [Master Execution Brief](./user_prompt/main.md).

## Status

- Fase: **planning dan audit**
- Implementasi source code: **belum dimulai**
- Migrasi database/data: **belum dimulai**
- DNS/deployment production: **belum diubah**
- Feature parity production: **wajib divalidasi melalui audit**

## Referensi project

- Production: `algohub.web.id`
- Source existing: `E:\rekapProject\project_ap`
- Workspace rewrite/planning: `E:\rekapProject\project_ap_v2`
- Target trafik awal: 50–200 user per hari

## Keputusan arsitektur utama

- Frontend: SvelteKit
- Backend: Go
- Database: PostgreSQL melalui Supabase
- Realtime: Supabase Realtime atau managed WebSocket
- Deployment: frontend dan Go API stateless di Vercel Hobby setelah diuji
- Auth: satu sistem identity dan SSO untuk seluruh subdomain
- Storage: bucket terpisah berdasarkan domain
- Database relational: tepat satu Supabase PostgreSQL project; Redis-compatible
  counter hanya dipakai sebagai infrastructure rate-limit ephemeral bila perlu
- Praktikum dipindahkan dari `algohub.web.id/praktikum` ke
  `praktikum.algohub.web.id`
- Admin dipusatkan di `admin.algohub.web.id`

## Domain target

| Host | Fungsi |
|---|---|
| `algohub.web.id` | Satu-satunya landing page dan portal publik |
| `siakad.algohub.web.id` | SIAKAD dan administrasi akademik/laboratorium |
| `elearning.algohub.web.id` | Materi, progres, gamifikasi, dan pembelajaran |
| `praktikum.algohub.web.id` | Sesi praktikum, coding, asesmen, dan nilai |
| `admin.algohub.web.id` | Console administrasi terpusat |
| `auth.algohub.web.id` | Login, SSO, callback, logout, dan recovery |
| `api.algohub.web.id` | Go API bersama |

## Centralized admin

- Akun admin memakai role `asisten`.
- `koordinator` dapat memiliki permission administratif yang lebih luas.
- Tidak ada role identity atau auth system terpisah bernama `admin`.
- Divisi asisten yang dipetakan: `sekretaris`, `bendahara`, `k3`, `labops`,
  `pendidikan`, dan `pdd`. Divisi memberi permission/scope, bukan role baru.
- Modul admin tersedia di:
  - `admin.algohub.web.id/praktikum`
  - `admin.algohub.web.id/elearning`
  - `admin.algohub.web.id/siakad`
- Tidak ada halaman `/admin` pada host portal, SIAKAD, E-learning, atau Praktikum.
- Permission, scope, ownership, dan audit log tetap wajib diperiksa backend.

## Landing page dan UI/UX

- Landing page resmi hanya `https://algohub.web.id/`.
- `/info` bukan landing page; route lama dapat diarahkan ke `/`.
- Referensi landing: [Burrow](https://8b.com/templates/burrow/demo.html).
- Referensi dashboard:
  [Gradient Able](https://codedthemes.com/demos/admin-templates/gradient-able/react/free/app/dashboard/analytics).
- Semua UI mobile-first dan responsive untuk mobile, tablet, dan desktop.
- Design direction: **calm technical lab**, bukan template SaaS generik.
- Detail anti-AI-slop dan responsive contract ada di [UI/UX](./ui-ux.md).

## Backend dan security

- Go Clean Architecture: router → middleware → handler → usecase → repository.
- Composition root: `cmd/api/main.go`; bukan satu file `cmd.go` monolitik.
- Rate limit wajib menghasilkan `429` dan `Retry-After`.
- Endpoint private harus authenticated sebelum query domain ke database.
- Wajib melindungi XSS, IDOR/BOLA, CSRF, SQL injection, SSRF, upload abuse,
  cache leakage, dan memory exhaustion.
- Deadline praktikum dihitung server-side menggunakan `Asia/Jakarta` / GMT+7.
- Detail middleware, caching, memory, endpoint mapping, dan deadline ada di
  [Backend](./backend.md).

## Testing dan execution control

- Setiap bagian wajib memiliki unit, integration/contract, E2E, security, dan
  performance test yang sesuai. Detail ada di [Testing](./testing.md).
- Urutan implementasi dan gate ada di [Execution phases](./phases.md).
- Temuan audit parity ada di [Feature matrix](./feature-matrix.md).
- Changelog keputusan ada di [Planning changelog](./changelog.md).
- Gate terakhir sebelum coding ada di [Preflight checklist](./preflight.md).
- Saat fase eksekusi dimulai, seluruh `plan/**` immutable. Hanya
  `plan/user_prompt/**` boleh berubah; perubahan prompt wajib dicatat di
  `plan/user_prompt` sendiri.

## Dokumen planning

1. [Master Execution Brief](./user_prompt/main.md)
2. [PRD dan arsitektur](./prd.md)
3. [Sitemap](./sitemap.md)
4. [Kontrak API](./api.md)
5. [ERD target](./erd.md)
6. [UI/UX dan frontend](./ui-ux.md)
7. [Backend, security, dan operasi](./backend.md)
8. [Feature parity matrix](./feature-matrix.md)
9. [Role dan permission](./roles-permissions.md)
10. [Database dan migrasi](./database-migration.md)
11. [Database operations](./database-operations.md)
12. [Deployment dan CI/CD](./deployment.md)
13. [Code runner](./code-runner.md)
14. [OpenAPI dan `/docs`](./openapi.md)
15. [Testing dan debugging](./testing.md)
16. [Execution phases](./phases.md)
17. [Workflow MCP 21st](./21st-mcp.md)
18. [Changelog](./changelog.md)
19. [Preflight checklist](./preflight.md)

## Aturan kerja fase planning

1. Audit source existing sebelum mengklaim feature parity.
2. Jangan mengubah source code, production database, DNS, deployment, atau data
   migrasi tanpa instruksi masuk fase implementasi.
3. Setiap keputusan baru harus diperbarui pada PRD, sitemap, API, ERD, dan master
   execution brief bila relevan.
4. Jangan menyimpan secret/API key di repository.
