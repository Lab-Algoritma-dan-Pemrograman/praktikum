# Preflight Checklist Sebelum Implementasi

Planning dianggap siap masuk coding setelah seluruh gate berikut memiliki bukti
tertulis. Checklist ini melengkapi phase 0 dan bukan pengganti test implementasi.

## 1. Keputusan yang sudah ditetapkan

- Target database adalah **Supabase project baru/fresh** dengan satu PostgreSQL,
  Auth, Storage, dan Realtime.
- Project legacy SIAKAD, E-learning, dan Praktikum tidak dipakai sebagai schema
  production target. Semuanya menjadi sumber migrasi/read-only selama rollback
  window.
- Role target: `koordinator`, `asisten`, `mahasiswa`, `peminjam`.
- Divisi asisten: `sekretaris`, `bendahara`, `k3`, `labops`, `pendidikan`, `pdd`.
- Go API dan frontend target berjalan stateless di Vercel; tidak ada WebSocket
  persisten atau worker daemon di Vercel.

## 2. Gate wajib sebelum coding

### Data dan schema

- [ ] Row count dan distinct role/division dari semua source terbaru.
- [ ] Duplicate NIM/email dan conflict matching sudah memiliki resolution queue.
- [ ] Semua tabel, RPC, trigger, Storage object, cron, dan route legacy masuk
  inventory atau diberi alasan deprecate.
- [ ] Target ERD, migration order, index, RLS, dan retention disetujui.

### Auth dan security

- [ ] Flow Supabase Auth/SSO, session cookie, CSRF, `return_to`, revoke, dan
  password legacy sudah diuji di staging.
- [ ] Semua credential/key yang pernah tersimpan di repo, backup, konfigurasi,
  atau log diaudit dan dirotasi sebelum deployment baru.
- [ ] Tidak ada service role key di browser bundle, preview, dokumentasi, atau
  test artifact.
- [ ] Role/division/permission matrix memiliki test account nyata/sintetis untuk
  setiap allow dan deny penting.

### Platform dan biaya

- [ ] Batas Vercel Hobby, Supabase Free, Realtime, Storage, database connection,
  cron, function duration, dan code-runner provider diuji dengan limit realistis.
- [ ] Connection pool, rate-limit counter, cache, dan provider fallback memiliki
  budget serta failure behavior.
- [ ] Backup/restore drill dan quota alert berhasil di environment staging.

### Cutover dan operasi

- [ ] DNS/TLS/SSO route, compatibility redirect, maintenance window, dan rollback
  owner terdokumentasi.
- [ ] Migration dry-run repeatable, checksum file cocok, dan data reconciliation
  lulus.
- [ ] Runbook incident memiliki langkah mencari error dari `X-Request-ID` sampai
  deployment SHA.
- [ ] UAT account mahasiswa, asisten tiap divisi, koordinator, dan peminjam siap.

## 3. Yang tidak menghambat mulai coding

Eksplorasi komponen MCP 21st, polishing copy landing, dan pilihan visual minor
boleh berjalan setelah foundation. Hal tersebut tidak boleh menunda keputusan
database, auth, permission, migration, testing, atau deployment gate di atas.
