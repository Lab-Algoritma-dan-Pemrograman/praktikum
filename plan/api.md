# Kontrak API Target Algohub

**Base URL:** `https://api.algohub.web.id/v1`  
**Status:** Draft 0.1

## 1. Konvensi umum

- JSON, kecuali upload langsung ke signed URL.
- Waktu ISO-8601 UTC.
- List memakai `page`, `limit`, `sort`, dan filter.
- Default `limit=25`, maksimum `100`.
- Mutasi yang dapat diulang menerima `Idempotency-Key`.
- Response selalu membawa `X-Request-ID`.
- Frontend tidak mengakses tabel privat secara langsung.
- OpenAPI 3.1 menjadi source of truth dan UI dokumentasi tersedia di
  `https://api.algohub.web.id/docs`.
- Endpoint privat menjalankan authentication dan authorization sebelum query
  domain ke database.
- Public endpoint hanya yang secara eksplisit di-allowlist.
- Waktu deadline tidak berasal dari timestamp client.

Response sukses:

```json
{
  "data": {},
  "meta": { "request_id": "req_123" }
}
```

Response error:

```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Anda tidak memiliki akses.",
    "details": null,
    "request_id": "req_123"
  }
}
```

Status minimum: `400`, `401`, `403`, `404`, `409`, `422`, `429`, `500`.

## 1.1 Header dan rate limit

Response rate limit:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 30
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1730678400
```

Rate limit diterapkan berlapis berdasarkan IP, user/session, route group, dan
resource sensitif. Auth, password recovery, AI, code runner, upload presign, dan
mutation memiliki limit lebih ketat. Counter berada pada store terdistribusi;
memory process hanya optimasi tambahan, bukan sumber kebenaran.

## 2. Auth/SSO

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/auth/me` | User, role, permission, scope |
| POST | `/auth/sso/exchange` | Tukar one-time code menjadi session |
| POST | `/auth/logout` | Logout dan revoke session |
| GET | `/auth/sessions` | Daftar session user |
| DELETE | `/auth/sessions/{id}` | Cabut session tertentu |

Cookie session harus `Secure`, `HttpOnly`, dan `SameSite=Lax`. Access token tidak
boleh diletakkan di URL, log, atau `localStorage`.

## 3. Core API

| Method | Endpoint | Scope |
|---|---|---|
| GET | `/me` | User login |
| PATCH | `/me` | User sendiri |
| GET | `/users` | Asisten/koordinator sesuai scope |
| PATCH | `/users/{id}/role` | Koordinator atau permission khusus |
| GET | `/permissions` | User login |
| GET | `/notifications` | User sendiri |
| POST | `/files/presign` | Permission domain |
| POST | `/files/complete` | Permission domain |
| DELETE | `/files/{id}` | Owner/admin |

Admin console memakai endpoint terpusat berikut. Endpoint ini tetap memakai
permission dan scope domain, sehingga host admin bukan bypass authorization:

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/admin/overview` | Ringkasan domain yang diizinkan |
| GET | `/admin/praktikum/{resource}` | Resource administrasi praktikum |
| GET | `/admin/elearning/{resource}` | Resource administrasi e-learning |
| GET | `/admin/siakad/{resource}` | Resource administrasi SIAKAD |
| GET | `/admin/auditlog` | Audit lintas domain sesuai scope |
| GET | `/admin/permissions` | Permission dan scope aktif |
| GET | `/docs/openapi.json` | OpenAPI JSON untuk documentation UI |
| GET | `/docs/openapi.yaml` | OpenAPI YAML untuk tooling |

Mutation admin memakai method sesuai resource dan selalu membuat audit event.
Tidak ada endpoint admin yang hanya mengandalkan subdomain sebagai bukti akses.

Resource admin awal yang dipetakan:

```text
/admin/praktikum/users|asisten|jadwal|pedoman|modul|sesi|soal|penilaian|monitoring|pengaturan
/admin/elearning/users|kurikulum|assessments|bug-hunt|monitoring|pengaturan
/admin/siakad/users|kelas|mata-kuliah|jadwal|absensi|nilai|inventaris|penyewaan|laporan|role-permission|pengaturan
```

`{resource}` adalah placeholder kelompok endpoint pada tabel di atas; kontrak
OpenAPI final memecahnya menjadi path konkret dan method yang sesuai.

Setiap operation ID, schema, status code, error code, auth requirement, pagination,
dan idempotency rule harus tertulis di OpenAPI. CI melakukan lint dan contract test
sebelum endpoint boleh dipakai frontend.

## 4. Praktikum API

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/practicum/sessions` | Daftar sesi sesuai user |
| GET | `/practicum/sessions/{id}` | Detail dan status akses |
| POST | `/practicum/attempts` | Mulai attempt dengan timer server |
| PATCH | `/practicum/attempts/{id}/answers` | Auto-save idempotent |
| POST | `/practicum/attempts/{id}/submit` | Submit manual/auto |
| GET | `/practicum/attempts/{id}` | Resume attempt |
| GET | `/practicum/grades` | Nilai sendiri/scope pengajar |
| POST | `/admin/praktikum/sesi/{id}/activate` | Aktivasi kelas/shift |
| POST | `/admin/praktikum/sesi/{id}/courses/{courseId}/close` | Tutup dan auto-submit |
| POST | `/admin/praktikum/penilaian/bulk` | AI/manual grading |
| GET | `/admin/praktikum/monitoring` | User aktif dan progress |

Response start/resume attempt wajib memuat:

```json
{
  "data": {
    "attempt_id": "...",
    "started_at": "2026-11-04T01:00:00Z",
    "deadline_at": "2026-11-04T02:30:00Z",
    "server_now": "2026-11-04T01:10:00Z",
    "timezone": "Asia/Jakarta",
    "timezone_offset": "+07:00"
  }
}
```

`deadline_at` dihitung server/database dari durasi tersimpan. Client hanya
menampilkan countdown dan tidak dapat mengubah deadline dengan clock device.

## 5. E-learning API

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/learning/levels` | Struktur level |
| GET | `/learning/modules/{id}` | Modul dan lesson |
| GET | `/learning/lessons/{id}` | Detail lesson |
| PUT | `/learning/lessons/{id}/progress` | Simpan progress |
| POST | `/learning/playground/run` | Jalankan kode dengan limit |
| GET | `/learning/leaderboard` | Leaderboard |
| POST | `/learning/bug-hunt/attempts` | Attempt Bug Hunt |
| POST | `/learning/hints` | Hint AI dengan rate limit |
| GET | `/learning/achievements` | Achievement user |

Asesmen e-learning memakai mesin asesmen praktikum yang sama.

## 6. SIAKAD API

| Method | Endpoint | Fungsi |
|---|---|---|
| GET | `/academic/classes` | Kelas dan scope |
| GET | `/academic/schedules` | Jadwal |
| GET/POST | `/academic/attendance` | Presensi |
| GET/POST/PATCH | `/academic/courses` | Mata kuliah |
| GET | `/academic/grades` | Nilai |
| GET/POST/PATCH | `/inventory/items` | Inventaris |
| GET/POST/PATCH | `/inventory/rentals` | Sewa/pinjam alat |
| GET | `/academic/reports` | Laporan sesuai permission |

Compatibility adapter diperlukan untuk RPC/endpoint lama selama migrasi.

## 7. File upload

Client mengirim domain, nama file, MIME, dan ukuran ke `/files/presign`. Server
menentukan bucket dan object key. Setelah upload langsung ke Storage, client
memanggil `/files/complete`.

| Domain | Bucket |
|---|---|
| portal | `portal` |
| siakad | `siakad` |
| elearning | `elearning` |
| praktikum | `praktikum` |

Client tidak boleh menentukan bucket bebas tanpa validasi server.

## 8. Realtime

Channel harus scoped:

- `user:{userId}` — notifikasi personal
- `class:{classId}` — perubahan kelas
- `practicum:session:{sessionId}` — status sesi/monitoring
- `assessment:{attemptId}` — status grading berwenang

Event minimal:

```json
{
  "id": "evt_123",
  "type": "attempt.graded",
  "occurred_at": "2026-11-04T00:00:00Z",
  "resource": { "type": "attempt", "id": "..." },
  "version": 1
}
```

Client harus reconnect, deduplicate event, dan refetch setelah reconnect.

## 9. Keamanan

- CORS hanya mengizinkan host Algohub.
- Rate limit auth, read/write, AI, code runner, dan presign dipisah.
- Handler memeriksa role, permission, scope, dan ownership.
- Perubahan role, nilai, presensi, submission, dan file diaudit.
- Secret Supabase/service role hanya berada di backend.
- Input rich text disanitasi dengan allowlist sebelum disimpan/dirender; frontend
  tidak memakai unsanitized HTML.
- Query memakai parameter binding; field sort/filter memakai allowlist.
- Object authorization memakai user ID dan scope dari session, bukan hanya ID
  pada URL. Resource lintas user mengembalikan `403` atau `404` sesuai kebijakan.
- State-changing cookie request memakai CSRF protection dan strict CORS.
- Upload memakai limit ukuran/MIME, object key dari server, dan signed URL.
- Request body, timeout, pagination, export, dan concurrency memiliki batas untuk
  mencegah memory exhaustion.
