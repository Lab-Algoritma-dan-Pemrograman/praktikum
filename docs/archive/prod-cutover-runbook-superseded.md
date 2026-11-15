# Prod Cutover Runbook — Algohub v2

**Status:** plan siap, eksekusi belum diizinkan. Tidak ada mutasi remote yang dilakukan oleh dokumen ini.

**Prinsip:** mengikuti `plan/database-migration.md` (urutan 12 langkah) dan
`docs/phase-7a-host-migration-checklist.md` (urutan host/owner). Phase 6 adalah
masked-rehearsal ke staging, bukan import produksi — import prod adalah scope
baru yang wajib disetujui owner.

---

## 1. Prasyarat gate (semua harus hijau sebelum hari-H)

| # | Item | Status | Bloker? | Alasan |
|---|---|---|---|---|
| 1 | Full E2E hijau (146/146, sisa skip terdokumentasi) | IN PROGRESS — fix locator SIAKAD sedang diverifikasi | BLOCKER | Gate CI wajib di `plan/deployment.md` §3 |
| 2 | Lima keputusan owner (lihat §6) | OPEN | BLOCKER | Copy legal, scope publik, host apex menentukan isi deploy |
| 3 | Audit DB legacy read-only (kit: `scripts/prod-audit/`) | KIT SIAP, MENUNGGU AKSES | BLOCKER | Menghitung gap data & titik delta tanpa menyentuh prod |
| 4 | Manifest backup legacy + checksum (`plan/database-migration.md` §4.1) | BELUM | BLOCKER | Tanpa backup manifest tidak ada rollback data |
| 5 | Dry-run import ke staging copy dari snapshot prod asli | BELUM (Phase 6 hanya masked) | BLOCKER | Membuktikan durasi & repeatable sebelum hari-H |
| 6 | Host migration checklist (`docs/phase-7a-host-migration-checklist.md`) | 6 item remote OPEN | BLOCKER | DNS/Vercel/SSO adalah jalur masuk user |
| 7 | RPO ≤ 24 jam, RTO ≤ 4 jam dikonfirmasi owner | DEFAULT PLAN, BELUM DIKONFIRMASI | NOT-BLOCKER | Bisa dikunci saat dry-run; tidak menahan persiapan |
| 8 | Smoke test list hari-H (login, SSO, jadwal, absensi, praktikum submit) | DAPAT DISUSUN DARI E2E ADA | NOT-BLOCKER | Turunan dari suite E2E yang sudah ada |

## 2. Strategi data (konek DB lama prod)

Jalur yang direkomendasikan — sama polanya dengan rehearsal Phase 6, naik
kelasnya dari masked ke real:

1. **Snapshot legacy** (freeze read): `pg_dump` per database legacy + checksum
   manifest. Legacy tidak pernah ditulis; v2 hanya membaca.
2. **Restore ke staging copy** (bukan ke target prod): satu project Supabase
   staging khusus dry-run.
3. **Transform + import dry-run** memakai pipeline Phase 6
   (`scripts/phase6-transform-live.py` dan kawan-kawan sebagai baseline),
   catat durasi, rejected, dan reconcile checks.
4. **Ulangi dry-run sampai repeatable** (Phase 6 membuktikan apply dua kali
   menghasilkan 19.026 unchanged tanpa INSERT — targetnya angka serupa pada
   data asli).
5. **Hari-H:** write freeze terjadwal → final delta import dari titik
   `max(updated_at)` audit → reconcile → DNS switch.

Yang **belum pernah** dilakukan dan itu memang scope baru: membaca data asli
(unmasked), menulis ke target produksi, dan memindahkan file/storage asli.

## 3. Urutan eksekusi hari-H

| # | Langkah | Pemilik | Operator |
|---|---|---|---|
| 1 | Umumkan maintenance window, freeze tulis legacy | Owner (keputusan) | Operator (eksekusi freeze) |
| 2 | Final delta import + reconcile checks | — | Operator |
| 3 | Deploy backend + semua app ke Vercel production | — | Operator |
| 4 | DNS: apex + subdomain mengikuti host checklist §2–3 | Owner (akses registrar/Vercel) | Operator menyiapkan perubahan |
| 5 | SSO allowlist update atomik | Owner | Operator |
| 6 | Smoke: login, return-to, app switcher, jadwal, absensi, rental, praktikum submit, elearning progress | — | Operator |
| 7 | Pemantauan 24 jam (error rate, auth failure, cold start, DB conn) | — | Operator |
| 8 | Legacy tetap read-only selama retention window | Owner (hapus hanya setelah rollback window lewat + persetujuan eksplisit) | — |

## 4. Rollback plan

- **Aplikasi:** kembali ke deployment Vercel sebelumnya; schema target tidak
  diubah untuk app rollback.
- **Data:** restore snapshot target, atau replay delta tervalidasi. Tanpa
  `DELETE` massal tanpa dry-run.
- **DNS:** kembalikan record ke binding lama (wajib dicatat rollback target
  di evidence deployment sebelum step 4 dimulai).
- **Target waktu:** RPO ≤ 24 jam, RTO ≤ 4 jam (default plan, dikonfirmasi
  ulang setelah dry-run).

## 5. Timeline estimasi (baseline: rehearsal Phase 6)

| Tahap | Estimasi | Sumber angka |
|---|---|---|
| Audit legacy (3 DB, kit siap) | 0.5–1 hari kerja | Asumsi; kit sudah siap |
| Backup manifest + checksum | 0.5 hari | Asumsi, standar |
| Dry-run import round 1 | ~7 menit import inti + 1 hari kerja verifikasi | Import staging Phase 6: 392 s untuk 19.026 rows; data asli lebih besar |
| Dry-run berulang + fix | 2–4 hari | Asumsi dari pola rehearsal |
| Hari-H (freeze → delta → DNS → smoke) | 3–5 jam window | Estimasi; delta kecil jika audit bersih |
| Pemantauan | 24 jam | Default plan |

**Jawaban kapan:** setelah E2E hijau + keputusan owner + akses audit masuk
(minggu ini), dry-run butuh ~3–5 hari kerja, lalu hari-H bisa dijadwalkan.
Realistis: **cutover 1,5–2 minggu dari sekarang** bila tidak ada schema drift.

## 6. Keputusan owner yang masih dibuka

1. **Copy legal/privacy/terms + kontak owner** — kalau tetap dibuka, route
   publik tetap fail-closed (tidak fatal untuk cutover internal, fatal untuk
   publikasi umum).
2. **Kanal kontak asisten publik** — kalau dibuka, halaman asisten tetap
   menampilkan state informasi.
3. **Scope publikasi jadwal** — kalau dibuka, jadwal tetap privat.
4. **Policy dokumen/modul + signed URL** — kalau dibuka, dokumen tetap
   tidak publik.
5. **Migrasi host apex staging (DNS/Vercel/SSO)** — BLOCKER keras: ini
   prasyarat teknis urutan hari-H (checklist §24–31).

---

## Lampiran: checklist eksekusi (print-friendly)

- [ ] Semua gate §1 hijau
- [ ] Backup manifest legacy tersimpan + checksum cocok
- [ ] Dry-run repeatable tercatat (2× hasil identik)
- [ ] Rollback target DNS tercatat sebelum perubahan
- [ ] Window diumumkan ke pengguna
- [ ] Smoke test §3.6 lulus semua
- [ ] Evidence deployment baru dibuat (operator, timestamp, rollback target)
