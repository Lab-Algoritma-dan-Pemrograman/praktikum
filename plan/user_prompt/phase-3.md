# Prompt Eksekusi Phase 3 — Praktikum Core

> File ini adalah prompt siap pakai setelah Phase 2 selesai dan disetujui.

```text
Phase 2 sudah selesai dan sudah disetujui. Lanjutkan ke Phase 3 sesuai
plan/phases.md: Practicum Core.

Bertindak sebagai lead full-stack developer. Kerjakan implementasi langsung pada
workspace rewrite saat ini. Jangan mengubah project lama di:
E:\rekapProject\project_ap

## Dokumen yang wajib dibaca

Baca dan pahami dokumen berikut sebelum mengubah source code:

1. plan/user_prompt/main.md
2. plan/phases.md
3. plan/feature-matrix.md
4. plan/roles-permissions.md
5. plan/database-migration.md
6. plan/database-operations.md
7. plan/prd.md
8. plan/api.md
9. plan/openapi.md
10. plan/erd.md
11. plan/backend.md
12. plan/code-runner.md
13. plan/testing.md
14. plan/ui-ux.md

Periksa hasil Phase 1 dan Phase 2 terlebih dahulu. Pertahankan struktur,
konvensi, dependency, konfigurasi, authorization boundary, observability, dan
test yang sudah ada. Baca source legacy Praktikum sebagai referensi parity saja;
jangan mengubah project lama dan jangan menyalin schema atau bug legacy tanpa
penilaian.

## Batas Phase 3

Phase ini hanya mengerjakan Praktikum core dan admin Praktikum yang dibutuhkan
untuk menjalankan flow utama. Jangan mengerjakan fitur penuh E-learning, SIAKAD,
inventory, rental, leaderboard, achievement, atau migrasi data production.

## Scope implementasi

### 1. Domain Praktikum

Implementasikan model, migration, repository, usecase, handler, OpenAPI contract,
dan UI untuk minimal:

- practicum session/course;
- session activation berdasarkan kelas/shift/scope;
- assessment dan question pool;
- selected questions untuk activation;
- question type dan validation yang ditentukan contract;
- student assessment attempt;
- answer dan draft answer;
- autosave;
- server-side timer dan deadline;
- manual submit;
- automatic submit;
- attempt/result status transition;
- manual grading dan grading result;
- AI grading adapter atau bounded grading integration sesuai kontrak;
- student grade view;
- admin Praktikum untuk konfigurasi, activation, monitoring, penutupan course,
  grading, dan audit.

Gunakan entity dan status transition yang eksplisit. Jangan mengubah status
attempt secara bebas dari handler atau client. Semua transition harus divalidasi
oleh usecase dan, jika menyentuh state bersama, transaction database.

### 2. Student flow

Implementasikan flow utama berikut:

1. mahasiswa melihat sesi Praktikum yang boleh diakses;
2. mahasiswa membuka detail sesi dan status activation;
3. mahasiswa memulai attempt jika syarat terpenuhi;
4. server membuat `started_at`, `deadline_at`, `server_now`, dan timezone;
5. mahasiswa menerima selected questions sesuai activation;
6. mahasiswa mengisi jawaban;
7. client melakukan autosave dengan idempotency key;
8. mahasiswa dapat refresh/reconnect dan melanjutkan attempt;
9. mahasiswa melakukan submit manual;
10. sistem menutup attempt ketika deadline tercapai;
11. jawaban diproses untuk grading;
12. mahasiswa melihat status dan nilai yang memang diizinkan.

Flow harus memiliki loading, empty, locked/not-active, expired, conflict,
offline/reconnect, unauthorized, error, dan retry state yang aman. UI tidak boleh
menganggap submit berhasil hanya karena request dikirim; status harus berasal
dari response/database.

### 3. Admin Praktikum flow

Pada `admin.algohub.web.id`, implementasikan shell/module yang diperlukan untuk:

- membuat atau memperbarui session/course;
- mengelola assessment dan question pool;
- memilih question untuk activation;
- membuka/menutup activation untuk kelas atau shift;
- melihat monitoring attempt dan progress sesuai scope;
- menutup course dengan auto-submit yang idempotent;
- melihat attempt dan hasil grading sesuai permission;
- memicu atau mengulang grading secara aman;
- mengoreksi nilai secara manual sesuai permission;
- melihat audit event;
- menangani conflict, partial failure, dan provider failure.

Jangan memberikan akses lintas kelas, shift, sesi, atau domain hanya karena user
berada pada host admin. Gunakan authorization Phase 2 pada setiap action.

### 4. Waktu dan state machine

Gunakan aturan berikut tanpa pengecualian:

- canonical business timezone adalah `Asia/Jakarta` / UTC+07:00;
- database menyimpan timestamp sebagai UTC `timestamptz`;
- server/database menjadi sumber `started_at`, `deadline_at`, dan `server_now`;
- client clock tidak dipercaya untuk menentukan validitas waktu;
- refresh, manipulasi clock, atau perubahan timezone device tidak memperpanjang
  attempt;
- timer client hanya presentasi dari deadline server;
- semua deadline comparison menggunakan waktu server/database;
- auto-submit, manual submit, close course, dan grading trigger idempotent;
- setiap state transition harus aman terhadap retry dan request duplikat;
- attempt yang sudah submitted/expired/closed tidak boleh menerima answer baru;
- close course harus konsisten walaupun dipanggil lebih dari sekali;
- race manual submit versus auto-submit versus admin close harus menghasilkan
  satu state final yang valid, bukan nilai ganda atau jawaban hilang.

Dokumentasikan state machine, invariant, transition owner, dan perilaku conflict
sebelum atau bersama implementasi. Gunakan row lock/constraint/transaction yang
sesuai, bukan hanya pengecekan di frontend.

### 5. Autosave dan answer

Autosave wajib:

- memiliki idempotency key atau revision/version contract;
- tidak menimpa jawaban versi lebih baru dengan request lama;
- menangani retry, timeout, reconnect, dan response out-of-order;
- memiliki ukuran body dan answer limit;
- menyimpan timestamp server serta revision yang dapat diverifikasi;
- tidak menerima attempt ID milik user lain;
- mengembalikan status yang jelas saat attempt sudah closed/expired;
- tidak membocorkan answer atau question milik scope lain.

Implementasikan resume flow. Setelah refresh atau reconnect, client mengambil
state authoritative dari API dan melakukan reconciliation draft secara aman.

### 6. Code runner dan grading

Integrasikan code runner melalui Go API sebagai satu-satunya gateway. Browser
tidak boleh memanggil provider runner secara langsung.

Gunakan interface/adapter `Runner` agar provider dapat diganti. Terapkan:

- bahasa dan compiler version allowlist;
- source limit maksimal sesuai plan, default 64 KB bila belum ada konfigurasi;
- stdin, stdout, stderr, dan result JSON limit;
- compile timeout, run timeout, dan total request timeout;
- quota per user/session/route dan concurrency cap;
- filesystem ephemeral;
- network disabled untuk program mahasiswa;
- provider secret hanya di backend;
- output sanitization tanpa unsanitized HTML;
- perbedaan compile error, runtime error, timeout, quota exceeded, dan provider
  error;
- idempotency dan deduplication untuk run/grading request;
- retry hanya untuk error provider yang aman di-retry;
- no retry untuk source/program failure;
- penyimpanan runner version, status, duration, exit code, request ID, dan hash
  source untuk reproducibility.

Gunakan response status runner sesuai `plan/code-runner.md`. Jangan menyebut
compile/runtime error sebagai error internal. Jangan menjalankan source mahasiswa
di proses Go atau Vercel Function tanpa sandbox yang disetujui.

Untuk AI grading, buat adapter bounded dan auditable. AI tidak boleh menjadi satu-
satunya sumber nilai tanpa aturan rubric, fallback, status pending, dan audit.
Jangan mengirim credential, data user berlebih, atau source yang tidak diperlukan
ke provider eksternal. Jika provider belum tersedia, gunakan fake provider untuk
test dan laporkan blocker nyata.

### 7. API dan OpenAPI

Implementasikan dan dokumentasikan operation minimal berikut sesuai struktur
OpenAPI yang sudah ada:

- `GET /practicum/sessions`;
- `GET /practicum/sessions/{id}`;
- `POST /practicum/attempts`;
- `GET /practicum/attempts/{id}`;
- `PATCH /practicum/attempts/{id}/answers`;
- `POST /practicum/attempts/{id}/submit`;
- `GET /practicum/grades`;
- activation admin;
- close course admin;
- bulk/manual grading admin;
- monitoring admin;
- runner endpoint internal/domain sesuai kebutuhan.

Setiap operation wajib memiliki operationId, security requirement, request/response
schema, error schema, pagination bila list, idempotency rule, authorization scope,
dan contoh aman. Response attempt start/resume minimal memuat:

- `attempt_id`;
- `started_at`;
- `deadline_at`;
- `server_now`;
- `timezone: Asia/Jakarta`;
- `timezone_offset: +07:00`.

Semua response membawa `X-Request-ID`; mutation memakai `Idempotency-Key` bila
relevant. Jangan membuat endpoint privat yang melewati auth/authorization.

### 8. Security dan data protection

Wajib mencegah:

- IDOR/BOLA pada session, question, attempt, answer, grade, dan monitoring;
- privilege escalation melalui role/division/scope dari request body;
- answer/question enumeration;
- submit user lain;
- perubahan deadline dari client;
- duplicate grading atau duplicate XP-like side effect;
- SQL injection, unsafe sort/filter, dan mass assignment;
- oversized answer/source/stdin/output;
- runner shell injection, network escape, filesystem escape, dan provider abuse;
- cross-user cache leakage;
- logging source, answer, token, credential, atau nilai sensitif secara penuh.

RLS menjadi defense-in-depth dan authorization Go tetap wajib. Admin audit harus
mencatat actor, action, domain, subject, scope, request ID, timestamp UTC, dan
before/after ringkas yang disanitasi.

## Test wajib

Tambahkan dan jalankan test pada layer yang sesuai. Jangan menyatakan test lulus
jika belum dijalankan.

### Unit dan domain

- state machine attempt;
- server deadline calculation dan timezone conversion;
- expired/closed/submitted transition;
- answer validation dan size limits;
- autosave revision/idempotency;
- grading calculation/rubric;
- permission dan scope policy;
- runner request/result mapping;
- output truncation dan timeout mapping.

### Integration dan database

- migration, foreign key, constraint, index, dan RLS;
- create/start/resume attempt;
- answer autosave dan revision conflict;
- duplicate request dengan idempotency key;
- manual submit;
- auto-submit ketika deadline terlewati;
- admin close course dan repeated close;
- grading retry dan duplicate grading;
- repository transaction rollback;
- concurrent attempt/answer/submit behavior;
- provider fake, provider timeout, provider outage, dan retry policy.

### Race, time, dan reliability

- manual submit versus auto-submit;
- manual submit versus admin close;
- duplicate autosave dengan request out-of-order;
- duplicate grading trigger;
- refresh tepat sebelum dan tepat sesudah deadline;
- server UTC versus display GMT+7;
- client clock maju/mundur;
- reconnect setelah network failure;
- database lock/deadlock/retry behavior;
- bounded connection pool dan runner concurrency.

### Security negative tests

- anonymous access;
- mahasiswa mengakses attempt/session/grade user lain;
- asisten beda class/shift/session scope;
- direct admin URL tanpa permission;
- role/division/scope spoofing dari client;
- answer/question enumeration;
- duplicate submit dan replay request;
- oversized source/stdin/output;
- shell injection dan malicious source;
- network/filesystem escape attempt;
- cross-user result access;
- rate limit dan `429` dengan `Retry-After`;
- secret/token/source sensitif tidak muncul di log dan artifact.

### E2E dan frontend

Uji minimal flow:

- student: daftar sesi → detail → start → jawab → autosave → refresh → resume →
  submit → status/nilai;
- student: deadline tercapai → auto-submit → tidak dapat mengubah jawaban;
- student: reconnect dan reconciliation draft;
- asisten: activation → monitoring → close course → review grading;
- asisten: akses di luar scope ditolak;
- admin shell navigation dan direct route guard;
- runner: editor → run → stdin → result/test feedback;
- empty, loading, locked, expired, unauthorized, provider error, conflict, dan
  retry state;
- responsive 320 px, 768 px, dan desktop;
- keyboard navigation, focus state, semantic labels, dan reduced motion.

### Quality gate

Jalankan checks yang tersedia:

- formatter;
- lint dan typecheck;
- Go vet, unit test, dan race test;
- migration lint/integration;
- OpenAPI lint dan contract test;
- security negative test;
- build frontend/backend;
- Playwright E2E smoke;
- load/concurrency smoke untuk submit, autosave, dan runner;
- secret scan dan dependency audit.

Artifact harus menyimpan coverage, race report, migration log, API response
sample yang disanitasi, screenshot/video E2E, structured log, request ID, dan
deployment SHA bila tersedia. Jangan memasukkan credential, token, source penuh,
jawaban ujian, atau data pribadi penuh.

## Aturan kerja

- Jangan mengubah project lama di `E:\rekapProject\project_ap`.
- Jangan mengubah, memindahkan, menghapus, atau menambah file di `plan/**`,
  kecuali file di `plan/user_prompt/**` bila memang diperlukan untuk prompt.
- Jangan mengubah database production, DNS, deployment, atau data migrasi
  production tanpa instruksi eksplisit.
- Jangan mengerjakan Phase 4 E-learning atau Phase 5 SIAKAD.
- Jangan mengimplementasikan migration production sebagai bagian dari feature
  build; gunakan local/staging fixture dan migration yang dapat direview.
- Jangan menaruh runner provider key, AI key, service role key, atau credential
  di frontend, repository, screenshot, test artifact, atau dokumentasi.
- Jangan mengeksekusi source mahasiswa di host process tanpa sandbox terkontrol.
- Jangan menonaktifkan atau menghapus test untuk membuat build lulus.
- Jangan mengarang hasil test, provider behavior, credential, atau data legacy.
- Jika credential/provider belum tersedia, gunakan fake adapter yang terisolasi,
  jalankan test lokal, dan laporkan blocker nyata.
- Jika menemukan perubahan existing dari Phase 1/2, baca dan pertahankan kecuali
  ada alasan teknis yang dibuktikan oleh test atau kontrak.

## Definition of Done

Phase 3 hanya boleh dinyatakan selesai jika:

1. Sesi, activation, question, attempt, answer, autosave, timer, submit, dan
   grading memiliki implementasi backend, database, API, dan UI yang terhubung.
2. Timer memakai server/database time dan aman terhadap refresh serta clock
   manipulation.
3. Manual submit, auto-submit, close course, dan retry bersifat idempotent.
4. Race condition submit/close/auto-submit memiliki transaction/constraint dan
   test yang lulus.
5. Student flow dan admin Praktikum flow berjalan pada local/staging.
6. Runner hanya diakses melalui Go API, memiliki limit, fake-provider test, dan
   error mapping yang benar.
7. Grading dapat diaudit, diulang secara aman, dan tidak mengandalkan AI tanpa
   rubric/fallback/status yang jelas.
8. Authorization dan RLS menolak akses lintas user, class, shift, session, dan
   domain.
9. OpenAPI, migration, feature matrix, dan changelog diperbarui sesuai perubahan.
10. Unit, integration, race, security, E2E, responsive, dan quality checks yang
    relevan dijalankan dan hasilnya dilaporkan.
11. Tidak ada secret masuk repository, browser bundle, log, atau artifact.
12. Tidak ada perubahan pada project legacy atau production tanpa otorisasi.
13. Scope Phase 4/5 belum dikerjakan.

## Format laporan akhir

Laporkan secara ringkas:

1. ringkasan implementasi Praktikum;
2. daftar file yang berubah;
3. migration, seed, constraint, dan index yang ditambahkan;
4. endpoint, operationId, dan route UI yang ditambahkan;
5. state machine dan keputusan race/idempotency;
6. runner/grading adapter dan blocker provider;
7. command formatter, lint, typecheck, build, dan test;
8. hasil setiap command, termasuk failure yang belum terselesaikan;
9. security, race, time, load, dan E2E checks yang dijalankan;
10. artifact debugging dan request ID contoh yang sudah disanitasi;
11. blocker credential/provider/DNS bila ada;
12. risiko atau keputusan yang membutuhkan user;
13. rekomendasi dan prerequisite Phase 4.

Jangan lanjut ke Phase 4 sebelum Phase 3, test, dan laporan ini selesai.
```
