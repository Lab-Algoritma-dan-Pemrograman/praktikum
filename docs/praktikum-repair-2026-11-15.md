# Praktikum: repair lokal dan gate rollout — 2026-11-15

## Hasil production (2026-11-15)

Rollout dijalankan pada project Supabase `zngmetvdvsfowerwngma` dan tujuh project
Vercel `algohub-v2-*` milik team `lab-algoritma-pemrograman`.

| Tahap | Hasil |
| --- | --- |
| Backup | `pg_dump` full 12,4 MB (sha256 `bbbc651e…`) + subset rehearsal, disimpan privat di luar repo |
| Rehearsal | Clone lokal dari data asli: mismatch session/scope → 0, work fingerprint identik, idempotent |
| Gate pengajar | `practicum.grade.write` **0 → 104** aktivasi, `question.manage` **0 → 104**, `session.manage` tetap 104 |
| Gate mahasiswa | Pasangan eligible **1982 → 1982** (tanpa regresi) |
| Migrasi | 0011 + 0012 diterapkan atomik; fingerprint work sebelum/sesudah identik |
| Setelah migrasi | Semua 11 pemeriksaan mismatch **0**; 1.634 attempt, 3.746 jawaban, 78.829 baris legacy utuh |
| Deploy | API, auth, admin, praktikum dipromosikan ke alias production |
| Smoke | `/healthz` ok, `/readyz` ready, anonim 401, portal/auth/praktikum 200, admin 401 |

Temuan penting: sebelum repair, **koordinator tidak punya akses menilai atau
mengelola soal untuk aktivasi legacy mana pun** (0 dari 104). Setelah 0012
konteks domain/session tersedia dan akses menjadi 104.

Data nilai: `score` per jawaban dan `total_nilai` pengerjaan **memang NULL di
sumber legacy** (3.746 dan 1.634 baris), jadi v2 menampilkan "Belum
dipublikasikan". Tidak ada nilai yang dikarang. Rinciannya di
`evidence/production-grades.json`.

Verifikasi production masih perlu satu langkah owner: **login nyata** untuk
membuktikan alur mahasiswa di browser. Skripnya sudah ada
(`scripts/praktikum-prod-authenticated.mjs`) dan hanya berjalan bila owner
menyediakan kredensial uji lewat environment.

## Status dan batas

Perubahan berada di working tree lokal, belum di-commit/deploy. Tidak ada SQL, write, reset, atau perubahan konfigurasi production yang dijalankan dalam pekerjaan ini. Pemeriksaan entry publik hanya mengonfirmasi HTTP 200; itu bukan acceptance authenticated production.

## Yang berubah

- UI daftar sesi, detail kegiatan/aktivasi, workspace soal, dan nilai/riwayat memakai token AlgoHub yang sama, dengan hierarki yang lebih jelas, loading/error/empty states, label Bahasa Indonesia, dan layout responsif.
- Navigasi soal di halaman yang sama mempertahankan draft dan tidak memicu dialog meninggalkan halaman; guard unsaved tetap berlaku untuk navigasi keluar/reload.
- AppShell tetap membawa logout + pindah aplikasi. Praktikum memiliki satu main/h1, skip link, orientasi deep route, dan safe return target SSO.
- Konfirmasi submit memakai native dialog. Bug event `close` yang terlambat setelah Escape/reopen telah diperbaiki; cancel/focus return tetap tersedia dan collection hanya melalui server.
- Histori terminal milik sendiri dapat dibaca setelah shift/roster/assignment berubah. Active attempt dan start/save/submit/run tetap membutuhkan eligibility saat ini; user lain tetap ditolak.
- Ancestry/title/scope pengajar memakai parent activation, termasuk filtering monitoring sebelum pagination. Ringkasan histori menambahkan `status` dan `can_open` secara additive.
- Roster PostgreSQL divalidasi dalam batch pada koneksi transaksi yang sama; bukan mengambil koneksi identity kedua saat koneksi pertama masih terkunci. Rollback I/O diberi deadline.
- Admission runner opsional tidak menghitung stale lease sebagai concurrency aktif. Reconciler membersihkan maksimal 100 stale lease dalam transaksi terpisah; provider/AI baru tidak ditambahkan.
- Mapping legacy diperbaiki agar attempt menggunakan parent activation untuk session ID/scope, bukan activation-session ID atau keanggotaan terakhir.
- Forward migration **[0011](../backend/migrations/0011_practicum_attempt_metadata_repair.sql)** memperbaiki metadata legacy yang terbukti, memverifikasi ancestry, dan menambah invariant DB. Tidak mengubah owner, jawaban, snapshot soal, nilai, revision, atau deadline; bukan izin menjalankannya pada production.
- Persistence attempt **dan activation** mempertahankan JSON snapshot/timestamp asli, metadata legacy/answer IDs, field roster yang sebelumnya absent, dan unique key ketika save/submit/manual grading/close/shared-timer start. Snapshot typed yang benar-benar diubah ditolak, bukan diam-diam ditimpa. Resume legacy menggunakan parent + owner sehingga tidak bergantung pada format unique key baru.

**Penutupan batch:** preservation activation telah selesai diverifikasi sub-agent. Penutupan aktivasi legacy tidak lagi ditolak akibat serialisasi snapshot, timestamp, atau field roster absent; collection attempt tetap atomik. Go + PostgreSQL, race, vet/build, dan contract lulus setelah fix. Rerun browser terintegrasi final **14/14 lulus**, termasuk navigasi soal ketika save belum selesai. Cluster PostgreSQL disposable telah dihentikan setelah verifikasi; tidak ada layanan DB test yang dibiarkan berjalan.

## Bukti verifikasi lokal

| Pemeriksaan | Bukti saat catatan ini dibuat |
| --- | --- |
| Frontend unit | 92 lulus, termasuk 30 practicum-client dan 33 auth-client |
| Svelte typecheck | Keenam app 0 error/0 warning; Praktikum dicek ulang setelah UI final |
| Build | Praktikum, Admin, dan API Go lulus |
| Contract | 91 operasi / 346 live response contracts; bundle source sinkron |
| Go + PostgreSQL | Seluruh paket lulus pada DB loopback disposable, bukan DB production |
| Race | Usecase/repository lulus; database race terpisah dari database unit |
| E2E Praktikum | 14 lulus: lifecycle mahasiswa/admin, scope denial, retry/conflict, deadline, ownership, authenticated a11y, real SSO deep link dan logout family |
| Browser/a11y | Chromium, 320/375/768/1024/1440 px, reflow font 200%, landscape, keyboard, reduced motion; **contrast tidak dimatikan** |
| Data preservation | Test PostgreSQL membuktikan repair metadata tidak mengubah work JSON di luar field metadata yang disetujui; trigger freeze/invariant tetap aktif setelah transaksi/rollback |
| Legacy writes | Kasus submit/close gagal akibat serialisasi snapshot legacy direproduksi sebelum fix, lalu lulus untuk resume/submit/manual publish/activation close dengan snapshot/timestamp/answer IDs/roster/unique key tetap utuh |

Tidak ada klaim sertifikasi WCAG penuh, acceptance browser fisik/Safari/Firefox, load production, atau UAT akun nyata. Ini juga bukan full-suite E2E seluruh produk.

Tes tidak dilemahkan agar hijau. Helper E2E login diselaraskan dengan label aktual “Email atau NIM”; test history migrasi lama yang mengunci jumlah enam file diganti dengan pemeriksaan **semua nama/checksum** terhadap katalog aktual.

## Preview sintetis

- [Daftar sesi desktop](../.tmp-shots/praktikum-audit-20261115/sessions-desktop.png)
- [Detail sesi desktop](../.tmp-shots/praktikum-audit-20261115/detail-desktop.png)
- [Workspace mobile](../.tmp-shots/praktikum-audit-20261115/workspace-mobile.png)
- [Konfirmasi submit mobile](../.tmp-shots/praktikum-audit-20261115/submit-mobile.png)
- [Riwayat mobile](../.tmp-shots/praktikum-audit-20261115/history-mobile.png)

Semua akun/jawaban pada gambar adalah fixture sintetis. Screenshot setelah perubahan, bukan pasangan before/after production. Folder preview lokal tidak otomatis ikut clean checkout.

## Gate sebelum menyentuh production

1. **Packaging dan revision.** Working tree memiliki pekerjaan pengguna lain; jangan menyapu semuanya ke satu commit. Shared UI barrel masih mengacu pada [ServicePrismCards.svelte](../packages/ui/src/ServicePrismCards.svelte)/[prism-cards.ts](../packages/ui/src/prism-cards.ts) yang sudah untracked sebelum pekerjaan ini. Local build bukan bukti clean checkout dapat build tanpa file tersebut.
2. **Riwayat migrasi.** Periksa nama/checksum dan urutan yang benar-benar applied. 0009/0010 adalah perubahan yang telah ada sebelumnya; 0010 memperluas roster frozen. Histori owner-only tidak membutuhkan pelonggaran roster, jadi jangan menerapkannya hanya untuk menampilkan nilai. Jangan mengubah checksum/menghapus history untuk melewati blocker.
3. **Backup dan dry-run.** Uji 0011 pada clone DB dengan data yang relevan. Hitung mismatch session/scope/owner/course, unknown ancestry, dan legacy rows yang kehilangan marker. Migration fail-closed; jangan memaksanya bila data tidak memenuhi guard. Bandingkan semua field work/grade/timer/owner sebelum/sesudah, bukan hanya jumlah baris.
4. **Scope legacy vs permission nyata.** Generator lama memakai scope `{}` untuk session/course/question serta class/shift-only untuk activation. 0011 hanya menyelaraskan attempt dengan parent, **tidak** mengarang domain/institution/division/session context yang hilang. Uji akses pengajar aktual dan konteks scope sebelum menganggap semua administrasi legacy siap; jangan memperbaiki kegagalan dengan grant global sembarangan.
5. **Pemeliharaan terkontrol.** Jangan melakukan repair yang mengambil table lock ketika ujian aktif tanpa koordinasi. Timer server tidak berhenti hanya karena route/flag dimatikan. Migrasi dan deployment harus disetujui tersendiri.
6. **UAT nyata.** Verifikasi mahasiswa melihat histori miliknya, jawaban/feedback tidak bocor, skor NULL tidak dipalsukan, pengajar hanya melihat kelas/shift yang berhak, serta start/resume/autosave/submit/publish/logout pada host production.
7. **Rollback kompatibel.** Siapkan artifact app/API yang kompatibel dengan invariant DB; jangan menonaktifkan trigger atau reset jawaban untuk membuat rollback lolos. Seluruh `legacy_preserve`/backup sumber tetap dipertahankan.

## Reproduksi

- `npm run check`, `npm run test:frontend`, build workspace Praktikum/Admin.
- `npm run openapi:check` memakai fixture server lokal.
- Go unit/integration dengan `ALGOHUB_TEST_DATABASE_URL` yang eksplisit menunjuk loopback disposable `*_test`; tanpa env itu PG tests skip.
- `go test -race -p 1` untuk paket usecase/repository (Windows memakai PATH MinGW lokal yang sudah tersedia, bukan perubahan production build flags).
- Playwright: [practicum.spec.ts](../tests/frontend/e2e/practicum.spec.ts) + [practicum-accessibility.spec.ts](../tests/frontend/e2e/practicum-accessibility.spec.ts), port base app 5473/API 8380; launcher membersihkan inherited secrets dan membatasi browser ke host lokal.
