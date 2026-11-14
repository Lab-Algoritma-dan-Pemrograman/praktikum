# Database Operations, Backup, RLS, dan Quota

**Status:** Target baseline untuk satu Supabase PostgreSQL project

## 1. Migration discipline

- Semua schema change berupa SQL migration bernomor dan reversible bila aman.
- Production hanya menjalankan migration yang sudah lulus staging rehearsal.
- CI mendeteksi schema drift, duplicate migration number, missing index, dan
  destructive SQL tanpa approval marker.
- Seed permission/division dipisahkan dari data user dan idempotent.
- Tidak ada `AutoMigrate` dari request/function runtime.

## 2. Connection dan query

- Go API memakai connection pooling yang disesuaikan dengan limit Supabase Free.
- Gunakan transaction untuk state transition, idempotency, auto-submit, dan
  inventory/rental concurrency.
- Semua list memiliki pagination dan bounded result.
- Index wajib ditinjau untuk foreign key, `nim`, `auth_user_id`, status/deadline,
  scope lookup, audit timestamp, dan search yang benar-benar digunakan.
- Query plan kritikal diperiksa dengan `EXPLAIN` di staging.

## 3. RLS dan API boundary

- RLS aktif sebagai defense-in-depth, bukan pengganti authorization Go.
- Service role hanya dipakai backend untuk operasi yang memang membutuhkan bypass.
- Client tidak menerima service role dan tidak mengakses tabel privat langsung.
- Policy diuji dengan role anonymous, authenticated, user lain, asisten beda scope,
  asisten divisi lain, dan koordinator.
- Semua perubahan policy memiliki migration dan regression test.

## 4. Backup dan restore

- Backup schema dan data dilakukan sebelum migration/cutover.
- Backup diberi timestamp, checksum, environment, schema version, dan retention.
- Restore drill dilakukan ke project/database staging terpisah sebelum cutover.
- Validasi restore meliputi row count, FK, permission seed, audit log, sample user,
  progress, nilai, dan object metadata.
- RPO/RTO final disetujui sebelum production; baseline sementara mengikuti
  `database-migration.md`.

## 5. Supabase Free quota guard

- Pantau database size, connection usage, Storage usage, bandwidth, Realtime
  connections, function invocation, dan log volume.
- Alert/quota check dijalankan sebelum batas free plan terlampaui.
- Large export memakai streaming/pagination; jangan membuat satu JSON besar.
- Activity/audit log memiliki retention dan archive policy.
- Seed/demo data tidak boleh masuk production tanpa label dan cleanup plan.

## 6. Test operasional

- migration up/down atau forward-compatibility test;
- schema drift dan index check;
- RLS matrix test;
- transaction rollback/deadlock/retry test;
- backup restore rehearsal;
- quota/load smoke;
- connection pool exhaustion test;
- data reconciliation setelah import dan setelah cutover.
