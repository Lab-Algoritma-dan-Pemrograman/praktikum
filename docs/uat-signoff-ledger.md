# UAT Sign-off Ledger — Phase 7

Format sign-off wajib. Satu baris per skenario per tester. AI tidak mengisi hasil human UAT; kolom `actual outcome`, `issue`, dan `decision` diisi tester manusia.

## Identitas run

| Field | Nilai |
| --- | --- |
| Environment | staging (`*.staging.algohub.web.id`) |
| Dataset | retained masked Phase 6 + `STAGING-BATCH20-cdb70744` |
| Source checksum | (pin saat eksekusi dimulai) |
| Tester | (nama manusia) |
| Role tester | (mahasiswa / asisten / koordinator / peminjam) |
| Tanggal | |

## Skenario

Legend: tiap skenario punya **expected outcome**. Tester mencatat actual, issue (severity: blocker/major/minor), dan keputusan go/no-go.

### A. Identity dan akses (semua role)

| ID | Skenario | Expected |
| --- | --- | --- |
| A1 | Login/SSO/logout keenam host | Satu password exchange, 6 cookie host-only Secure/HttpOnly, logout mencabut semua |
| A2 | Direct URL tanpa sesi | 401; deep link dipertahankan setelah login |
| A3 | Direct URL di luar permission | 403; direct 404 untuk resource tak ada/luar scope |
| A4 | Profil/sesi: revoke session | Akses admin berubah 403, identitas tetap; user lain tidak bisa revoke (404) |
| A5 | Optimistic conflict | Update version lama 409; refresh selesaikan tanpa kehilangan draft |

### B. Role matrix (per akun retained, gunakan permission sebenarnya, jangan paksa semua menu muncul)

| ID | Skenario | Expected |
| --- | --- | --- |
| B1 | Mahasiswa a/b | Menu sesuai; class scope A vs B terpisah |
| B2 | Asisten tanpa divisi | Hanya fungsi asisten umum, tanpa menu divisi |
| B3 | Asisten ×6 divisi (sekretaris/bendahara/k3/labops/pendidikan/pdd) | PDD hanya Ringkasan/Profil; lainnya sesuai matriks 91-SSR existing |
| B4 | Asisten scoped kelas A | List/detail A 200; kelas B 404 |
| B5 | Koordinator | Self-edit 422 VALIDATION_ERROR; assignment via API/UI |
| B6 | Peminjam | Hanya fungsi penyewaan |

### C. E-learning (mahasiswa)

| ID | Skenario | Expected |
| --- | --- | --- |
| C1 | Lesson text/quiz | Completion tanpa assertion; empty draft ditolak |
| C2 | Code lesson C: run testcase di browser sampai lulus | Completion + XP naik tepat sekali; duplikat +0 XP; banner risiko browser-trusted tampil |
| C3 | Code lesson Python (Pyodide) | Sama seperti C2; `input()` top-level jalan |
| C4 | Ubah source setelah completion | Attestation invalid; wajib test ulang |
| C5 | Autosave/offline/reconnect/conflict | Draft tidak hilang; conflict resolve tanpa duplikat XP |
| C6 | Mobile editor | Editor terpakai 375px; tidak ada layout blocker |

### D. Praktikum (mahasiswa + asisten)

| ID | Skenario | Expected |
| --- | --- | --- |
| D1 | Eligibility/timer/autosave/submit | Timer server-authoritative; submit setelah closed ditolak |
| D2 | Review manual asisten | Draft grade/publish/replay; private grade 404 untuk mahasiswa |
| D3 | Publikasi nilai | Mahasiswa melihat setelah publish saja |

### E. SIAKAD (mahasiswa/asisten/peminjam)

| ID | Skenario | Expected |
| --- | --- | --- |
| E1 | Jadwal/kelas/QR | QR duplicate ditolak; state validasi sesuai |
| E2 | Inventory/rental | Stok tidak pernah negatif pada concurrency; fee0 langsung payment-verified |
| E3 | Reports/export/pagination | 101+ item terbagi 100+1; CSV sesuai |
| E4 | Finance scope | Hanya bendahara/finance melihat scope keuangan |

### F. Pilot usage nyata (diisi saat pilot)

| ID | Skenario | Expected |
| --- | --- | --- |
| F1 | Mahasiswa nyata menyelesaikan alur penuh | Feedback tercatat; stop/go criteria pilot |
| F2 | Asisten nyata grading manual | Issue operasional dicatat |

## Ringkasan keputusan

| Blok | Passed | Failed | Blocked | Not-run |
| --- | --- | --- | --- | --- |

Keputusan go/no-go akhir oleh pemilik: (belum diisi, menunggu UAT manusia)
