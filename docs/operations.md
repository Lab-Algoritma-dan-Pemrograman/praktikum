# Operasi dan dukungan Algohub

Dokumen gabungan untuk operator dan support. Bagian pertama adalah runbook staging, bagian kedua alur dukungan pengguna, dan bagian ketiga format UAT sign-off. Kontak/petugas nyata wajib dikonfirmasi pemilik; AI tidak mengarang nama.

## Runbook operator staging

Template operator. Kolom kontak/petugas nyata wajib dikonfirmasi pemilik sebelum drill; AI tidak mengarang nama.

### 1. Triage awal

1. Cek `https://api.staging.algohub.web.id/healthz` (200, `ok`) dan `/readyz` (200, `ready`). Readyz gagal = database staging tak terjangkau.
2. Ambil `X-Request-ID` dari error UI/user report; cocokkan ke log API.
3. Klasifikasi severity:

| Sev | Kriteria | Respon |
| --- | --- | --- |
| S1 | Login/SSO semua host mati, data corrupt, kebocoran credential | Escalate langsung ke pemilik; freeze perubahan |
| S2 | Satu host down, scheduler berhenti >15 menit, error spike | Investasi root cause; komunikasi ke pengguna pilot |
| S3 | Fitur non-kritis, minor a11y, p95 melonjak tanpa error | Catat issue, backlog |

### 2. Request-ID lookup

- Middleware: `backend/internal/middleware/requestid.go` + `logging.go`. Setiap request API membawa request-ID yang sama di UI/API/log.
- Jangan membaca token/queue headers/Vault values saat lookup.

### 3. Perintah cepat

| Keperluan | Perintah |
| --- | --- |
| Regresi backend | `npm run api:test` (PG: `ALGOHUB_TEST_DATABASE_URL=... go test -count=1 ./...` pada loopback `_test` baru) |
| Vet/build | `npm run api:vet && npm run api:build` |
| Frontend check/build | `npm run check`, `npm run build` |
| Unit frontend | `npm run test:frontend` |
| E2E Chromium | `npx playwright test` |
| OpenAPI contract | `npm run openapi:check` (91 operasi) |

E2E harus memiliki seluruh target server. Jika port default dipakai project lain, gunakan port terisolasi, misalnya `ALGOHUB_E2E_APP_PORT_BASE=5273 ALGOHUB_E2E_API_PORT=8180 npx playwright test`. Jangan memakai config existing-server tanpa memverifikasi kepemilikan semua origin; config tersebut juga memerlukan `ALGOHUB_E2E_USE_EXISTING_SERVERS=true` secara eksplisit.
| Dependency Go | `npm run api:audit` (govulncheck) |

### 4. Scheduler (pg_cron/pg_net)

- Job: `algohub-staging-practicum-reconcile`, per menit, POST `/v1/internal/practicum/reconcile`, secret dari Vault.
- Verifikasi: `cron.job`, `cron.job_run_details`, `net._http_response` (retensi ~6 jam!). `succeeded` di cron hanya berarti enqueue; sukses HTTP lihat `net._http_response`.
- Pause/resume: `cron.alter_job(..., active := false/true)` — **butuh izin pemilik**.
- Jangan ubah cadence/secret tanpa izin. Prosedur rotasi ada di [staging-scheduler.md](staging-scheduler.md).

### 5. Recovery dan rollback limits

- Backup yang teruji: **sanitized public state + noncredential Auth metadata** (`docs/phase-6-closeout.md` §5). Tidak memulihkan password/MFA/provider sessions/audit/notifications/operations/leases.
- Restore hanya ke owned local `_test` database (prosedur `scripts/phase6-recovery-verify.py`); tidak pernah di atas pekerjaan pengguna tanpa preserve.
- Row rollback tidak memutar sequence; sequence restore terbukti via pg_dump.
- Rollback aplikasi = redeploy deployment ID Vercel sebelumnya (catat deployment ID tiap deploy di bukti).

### 6. Komunikasi dan eskalasi

| Peran | Nama | Kontak |
| --- | --- | --- |
| Pemilik keputusan | (konfirmasi pemilik) | |
| Koordinator pengguna nyata | (konfirmasi pemilik) | |
| Support pilot | (konfirmasi pemilik) | |

Eskalasi: tester → support → pemilik. Insiden data/security langsung pemilik.

### 7. Checklist verifikasi pasca-incident

1. healthz/readyz 200.
2. Login/SSO satu host nyata 200 + logout 401.
3. Scheduler: satu delivery cron HTTP 200 (`{expired:0,jobs_processed:0}` cukup).
4. Nol sesi acceptance aktif (`acceptance-session-final` query pattern).
5. Catat insiden: waktu, request-ID, akar masalah, tindakan, bukti path.

---

## Support playbook

Playbook untuk pengguna pilot. Kontak/petugas nyata dikonfirmasi pemilik; template tidak mengarang identitas.

### 1. Laporan masalah dari pengguna

1. Catat: nama, role, waktu, URL halaman, langkah sebelum error, screenshot.
2. Minta `X-Request-ID` bila ada di layar error; tanpa itu pun lanjut.
3. Klasifikasi via runbook severity (lihat bagian Runbook operator staging).

### 2. Pertanyaan umum yang diantisipasi

| Pertanyaan | Jawaban singkat |
| --- | --- |
| "Nilai saya hilang?" | Nilai Praktikum hanya muncul setelah asisten publish; hasil preview browser bukan nilai |
| "XP saya tidak naik setelah run kode" | XP hanya naik tepat sekali per lesson setelah semua testcase lulus dan completion; duplikat memang +0 |
| "Gagal submit Praktikum" | Cek timer/deadline server; refresh dan cek autosave terakhir |
| "Kode C/Python saya jalan tapi XP tidak keluar" | Pastikan source final sudah di-test ulang setelah edit terakhir; attestation invalid saat source berubah |
| "Lupa password" | Domain `algohub.test` sintetis; hubungi operator untuk reset via pemilik (tidak ada email reset) |

### 3. Batas yang harus dikomunikasikan ke pengguna

- Hasil run browser E-learning **bukan** nilai akademik dan bukan bukti correctness authoritative.
- Mode offline/autosave konflik: resolve melalui refresh yang ditawarkan UI, jangan multi-tab editing.

### 4. Proses feedback pilot

1. Pengguna mengirim laporan ke support (kanal dikonfirmasi pemilik).
2. Support mencatat ke issue log pilot (file: `docs/pilot-feedback-log.md` saat pilot berjalan).
3. Mingguan: triage bersama pemilik (blocker/major/minor) dan keputusan stop/go criteria.

### 5. Eskalasi

Mengikuti bagian Runbook operator staging §6. Insiden data pribadi/credential langsung ke pemilik tanpa dibahas publik.

---

## UAT sign-off ledger

Format sign-off wajib. Satu baris per skenario per tester. AI tidak mengisi hasil human UAT; kolom `actual outcome`, `issue`, dan `decision` diisi tester manusia.

### Identitas run

| Field | Nilai |
| --- | --- |
| Environment | staging (`*.staging.algohub.web.id`) |
| Dataset | retained masked Phase 6 + `STAGING-BATCH20-cdb70744` |
| Source checksum | (pin saat eksekusi dimulai) |
| Tester | (nama manusia) |
| Role tester | (mahasiswa / asisten / koordinator / peminjam) |
| Tanggal | |

### Skenario

Legend: tiap skenario punya **expected outcome**. Tester mencatat actual, issue (severity: blocker/major/minor), dan keputusan go/no-go.

#### A. Identity dan akses (semua role)

| ID | Skenario | Expected |
| --- | --- | --- |
| A1 | Login/SSO/logout keenam host | Satu password exchange, 6 cookie host-only Secure/HttpOnly, logout mencabut semua |
| A2 | Direct URL tanpa sesi | 401; deep link dipertahankan setelah login |
| A3 | Direct URL di luar permission | 403; direct 404 untuk resource tak ada/luar scope |
| A4 | Profil/sesi: revoke session | Akses admin berubah 403, identitas tetap; user lain tidak bisa revoke (404) |
| A5 | Optimistic conflict | Update version lama 409; refresh selesaikan tanpa kehilangan draft |

#### B. Role matrix (per akun retained, gunakan permission sebenarnya, jangan paksa semua menu muncul)

| ID | Skenario | Expected |
| --- | --- | --- |
| B1 | Mahasiswa a/b | Menu sesuai; class scope A vs B terpisah |
| B2 | Asisten tanpa divisi | Hanya fungsi asisten umum, tanpa menu divisi |
| B3 | Asisten ×6 divisi (sekretaris/bendahara/k3/labops/pendidikan/pdd) | PDD hanya Ringkasan/Profil; lainnya sesuai matriks 91-SSR existing |
| B4 | Asisten scoped kelas A | List/detail A 200; kelas B 404 |
| B5 | Koordinator | Self-edit 422 VALIDATION_ERROR; assignment via API/UI |
| B6 | Peminjam | Hanya fungsi penyewaan |

#### C. E-learning (mahasiswa)

| ID | Skenario | Expected |
| --- | --- | --- |
| C1 | Lesson text/quiz | Completion tanpa assertion; empty draft ditolak |
| C2 | Code lesson C: run testcase di browser sampai lulus | Completion + XP naik tepat sekali; duplikat +0 XP; banner risiko browser-trusted tampil |
| C3 | Code lesson Python (Pyodide) | Sama seperti C2; `input()` top-level jalan |
| C4 | Ubah source setelah completion | Attestation invalid; wajib test ulang |
| C5 | Autosave/offline/reconnect/conflict | Draft tidak hilang; conflict resolve tanpa duplikat XP |
| C6 | Mobile editor | Editor terpakai 375px; tidak ada layout blocker |

#### D. Praktikum (mahasiswa + asisten)

| ID | Skenario | Expected |
| --- | --- | --- |
| D1 | Eligibility/timer/autosave/submit | Timer server-authoritative; submit setelah closed ditolak |
| D2 | Review manual asisten | Draft grade/publish/replay; private grade 404 untuk mahasiswa |
| D3 | Publikasi nilai | Mahasiswa melihat setelah publish saja |

#### E. SIAKAD (mahasiswa/asisten/peminjam)

| ID | Skenario | Expected |
| --- | --- | --- |
| E1 | Jadwal/kelas/QR | QR duplicate ditolak; state validasi sesuai |
| E2 | Inventory/rental | Stok tidak pernah negatif pada concurrency; fee0 langsung payment-verified |
| E3 | Reports/export/pagination | 101+ item terbagi 100+1; CSV sesuai |
| E4 | Finance scope | Hanya bendahara/finance melihat scope keuangan |

#### F. Pilot usage nyata (diisi saat pilot)

| ID | Skenario | Expected |
| --- | --- | --- |
| F1 | Mahasiswa nyata menyelesaikan alur penuh | Feedback tercatat; stop/go criteria pilot |
| F2 | Asisten nyata grading manual | Issue operasional dicatat |

### Ringkasan keputusan

| Blok | Passed | Failed | Blocked | Not-run |
| --- | --- | --- | --- | --- |

Keputusan go/no-go akhir oleh pemilik: (belum diisi, menunggu UAT manusia)
