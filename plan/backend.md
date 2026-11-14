# Backend, Security, dan Operasi Go

**Status:** Draft 0.1

## 1. Struktur Go yang disepakati

Istilah `cmd.go` diluruskan menjadi pola Go idiomatis. Bukan satu file raksasa
yang menampung seluruh API.

```text
backend/
  cmd/
    api/main.go              # composition root/service entrypoint
    worker/main.go           # job runner lokal/staging; bukan daemon Vercel
    migrate/main.go          # migrator terpisah
  internal/
    config/
    delivery/http/           # router, handler, DTO, HTTP error mapping
    middleware/
    usecase/                 # business rules
    repository/              # interface dan implementasi PostgreSQL
    entity/                  # domain model dan enum
    authorization/            # role, permission, scope, object policy
    platform/                 # clock, storage, realtime, mail, runner
  pkg/
    response/
    observability/
  migrations/
```

Request flow:

```text
router → middleware → handler → usecase → repository → PostgreSQL/Supabase
```

`cmd/api/main.go` hanya memuat config, dependency, router, server, graceful
shutdown, dan health check. Handler tidak boleh berisi query SQL atau business
rule. Semua endpoint dipasang melalui router versioned `/v1`.

Jika deployment memakai Vercel/serverless, adapter `api/index.go` hanya memanggil
builder yang sama dengan `cmd/api/main.go`; business logic tidak diduplikasi.
Vercel Function tidak boleh bergantung pada goroutine persisten, local filesystem
permanen, atau in-memory session/attempt state. Cron dan provider managed dipakai
untuk pekerjaan terjadwal/eksternal.

## 2. Urutan middleware

1. trusted proxy/real IP;
2. request ID;
3. panic recovery;
4. security headers dan CORS allowlist;
5. request body limit dan timeout;
6. edge/IP rate limit;
7. access log dasar tanpa secret;
8. authentication/session verification;
9. user rate limit dan abuse detection;
10. role/permission/scope authorization;
11. handler/usecase;
12. response metrics dan audit event bila relevan.

Authentication dan authorization ditempatkan sebelum repository call domain.
Public endpoint harus allowlisted dan tetap memiliki rate limit; default router
adalah private/deny.

## 3. Rate limiting dan 429

| Kelompok | Contoh | Kebijakan |
|---|---|---|
| Auth abuse | login, register, reset password | per IP + identifier, paling ketat |
| Read public | landing, jadwal publik | per IP, dapat dilayani CDN |
| Read private | dashboard, nilai | per user/session + IP |
| Mutation | presensi, submit, role, upload | per user + route + idempotency |
| Expensive | AI, code runner, export | concurrency dan quota terpisah |

Counter memakai Redis/Redis-compatible distributed store atau layanan edge yang
dipilih. Memory lokal process hanya best-effort karena serverless dapat memiliki
banyak instance. Response `429` menyertakan `Retry-After` dan header quota.

## 4. Mencegah spam database tanpa auth

- Auth middleware berjalan sebelum usecase/repository.
- Endpoint private default `401` tanpa query domain.
- Public endpoint dipisahkan, di-cache bila aman, dan dibatasi rate limit.
- Rate limit edge/IP berjalan sebelum koneksi database.
- Query memakai pagination, select kolom minimum, timeout, dan result cap.
- Database pool memiliki `MaxOpenConns`, `MaxIdleConns`, dan connection lifetime.
- Health check tidak melakukan query berat.
- WAF/CDN dapat memblokir pola bot/abuse sebelum mencapai API.

## 5. Proteksi aplikasi

### XSS

- Svelte escaping tetap aktif.
- Rich text/Markdown disanitasi server-side dengan allowlist HTML/atribut.
- `{@html}` hanya menerima output sanitizer.
- CSP, `X-Content-Type-Options`, `Referrer-Policy`, dan frame policy aktif.
- File upload tidak di-inline sebagai HTML/SVG aktif tanpa sanitasi.

### IDOR/BOLA

- ID dari URL hanya locator, bukan bukti akses.
- Usecase mengambil resource dengan predicate user/scope/ownership.
- Semua endpoint detail, update, delete, download, attempt, nilai, dan file diuji
  dengan user lain.
- Resource sensitif dapat memakai `404` untuk mencegah enumeration.

### SQL injection dan input

- Repository memakai parameter binding.
- Raw SQL hanya untuk query ter-review dan parameterized.
- Sort/filter/column name berasal dari allowlist.
- DTO divalidasi ukuran, tipe, enum, range, dan business constraint.

### CSRF, CORS, dan session

- Cookie `Secure`, `HttpOnly`, `SameSite=Lax`.
- Mutation cookie request memakai CSRF token/double-submit sesuai flow SSO.
- CORS hanya origin Algohub yang terdaftar dan tidak memakai wildcard bersama
  credentials.
- Logout/revoke menghapus session dan mendukung revoke per device.

### Upload dan SSRF

- Bucket dan object key ditentukan server.
- MIME, extension, ukuran, content inspection, dan filename divalidasi.
- Signed URL berumur pendek.
- URL eksternal untuk import/webhook harus allowlisted dan dilindungi SSRF.

## 6. Caching

| Data | Cache policy |
|---|---|
| JS/CSS/image immutable | CDN public, hash filename, long max-age |
| Landing/public content | CDN/edge dengan revalidation dan purge |
| Jadwal publik | short TTL + ETag, tanpa data privat |
| Dashboard user | private/no-store atau cache key user yang ketat |
| Nilai, presensi, attempt, permission | no public cache; revalidate setelah mutation |
| Auth/session | `no-store` |
| Signed URL | short-lived, jangan disimpan publik |

Aturan:

- Tidak pernah cache response privat hanya berdasarkan URL tanpa user/scope key.
- `Vary` dan `Cache-Control` harus eksplisit.
- Mutation melakukan invalidation/tag purge yang relevan.
- ETag/If-None-Match digunakan untuk GET yang aman.
- Cache database/query bounded dan memiliki TTL/invalidation; bukan sumber
  kebenaran transaksi.

## 7. Memory dan resource management

- Semua request meneruskan `context.Context` ke repository dan external client.
- Timeout untuk HTTP, DB, Storage, AI, dan code runner.
- Batas body, JSON depth/size, pagination, export, dan upload.
- Streaming untuk export/file besar; jangan menampung seluruh file di memory.
- Worker memiliki bounded queue dan concurrency semaphore.
- Goroutine wajib memiliki lifecycle/cancel; tidak ada goroutine per request yang
  tidak diawasi.
- Connection pool, HTTP transport, dan response body ditutup/digunakan kembali.
- Code runner berjalan sandboxed dengan CPU, memory, process, filesystem, dan
  network limit.
- Provider code runner dipanggil melalui adapter backend; browser tidak boleh
  memanggil Judge0/Glot/provider lain secara langsung.
- Metrics untuk heap, goroutine, GC pause, DB pool, queue, latency, dan 5xx.

## 8. Endpoint mapping

| Prefix | Handler group | Usecase group | Data scope |
|---|---|---|---|
| `/v1/auth/*` | auth delivery | session/SSO | session/user |
| `/v1/me`, `/v1/users/*` | identity delivery | identity/RBAC | user/admin scope |
| `/v1/admin/overview` | admin delivery | admin overview | asisten/koordinator scope |
| `/v1/admin/praktikum/*` | admin practicum delivery | practicum configuration | domain permission/scope |
| `/v1/admin/elearning/*` | admin learning delivery | learning configuration | domain permission/scope |
| `/v1/admin/siakad/*` | admin academic delivery | academic configuration | domain permission/scope |
| `/v1/admin/auditlog` | admin audit delivery | audit query | allowed audit scope |
| `/v1/practicum/*` | practicum delivery | session/attempt/grading | class/shift/session |
| `/v1/learning/*` | learning delivery | lesson/progress/game | user/content |
| `/v1/academic/*` | academic delivery | class/schedule/attendance | institution/class |
| `/v1/inventory/*` | inventory delivery | item/rental | lab/borrower |
| `/v1/files/*` | file delivery | presign/attachment | domain/owner |
| `/v1/realtime/*` | realtime auth | channel authorization | scoped channel |

OpenAPI menjadi source of truth untuk method, path, request, response, auth,
status code, dan error code. Endpoint lama memakai compatibility adapter selama
migrasi, lalu diberi deprecation date.

## 9. Waktu dan deadline GMT+7

- Business timezone: `Asia/Jakarta` (`UTC+07:00`).
- Storage: UTC `timestamptz`.
- Server clock dan DB clock disinkronkan NTP.
- `started_at` dan `deadline_at` dibuat server/database.
- Deadline adalah instant absolut; client tidak dapat mengubahnya dengan timezone
  atau clock lokal.
- Scheduler/cron mengambil record yang deadline-nya sudah lewat dan menjalankan
  auto-submit secara idempotent.
- Race antara submit manual, close admin, dan auto-submit diselesaikan dalam
  transaksi dengan state transition yang valid.
- API mengirim `server_now` dan timezone agar UI melakukan countdown konsisten.

## 10. Observability dan security testing

- Structured log dengan request ID, route, status, latency, dan error code; jangan
  log token/password/jawaban sensitif.
- Alert untuk 401/403 spike, 429 spike, 5xx, DB pool exhaustion, memory, queue,
  auto-submit failure, dan suspicious access.
- Test wajib: auth boundary, IDOR/BOLA, XSS payload, CSRF, SQL injection, rate
  limit/429, cache isolation, upload abuse, timeout, race submit, dan clock
  manipulation.
- Setiap package/usecase/handler/route memiliki unit atau contract test; setiap
  domain memiliki integration dan E2E smoke test. Artifact CI menyertakan request
  ID, deployment SHA, coverage, dan sanitized logs untuk debugging.
