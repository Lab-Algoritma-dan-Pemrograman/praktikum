# Support Playbook — Algohub (Phase 7 template)

Playbook untuk pengguna pilot. Kontak/petugas nyata dikonfirmasi pemilik; template tidak mengarang identitas.

## 1. Laporan masalah dari pengguna

1. Catat: nama, role, waktu, URL halaman, langkah sebelum error, screenshot.
2. Minta `X-Request-ID` bila ada di layar error; tanpa itu pun lanjut.
3. Klasifikasi via runbook severity (lihat `docs/runbook.md`).

## 2. Pertanyaan umum yang diantisipasi

| Pertanyaan | Jawaban singkat |
| --- | --- |
| "Nilai saya hilang?" | Nilai Praktikum hanya muncul setelah asisten publish; hasil preview browser bukan nilai |
| "XP saya tidak naik setelah run kode" | XP hanya naik tepat sekali per lesson setelah semua testcase lulus dan completion; duplikat memang +0 |
| "Gagal submit Praktikum" | Cek timer/deadline server; refresh dan cek autosave terakhir |
| "Kode C/Python saya jalan tapi XP tidak keluar" | Pastikan source final sudah di-test ulang setelah edit terakhir; attestation invalid saat source berubah |
| "Lupa password" | Domain `algohub.test` sintetis; hubungi operator untuk reset via pemilik (tidak ada email reset) |

## 3. Batas yang harus dikomunikasikan ke pengguna

- Hasil run browser E-learning **bukan** nilai akademik dan bukan bukti correctness authoritative.
- Mode offline/autosave konflik: resolve melalui refresh yang ditawarkan UI, jangan multi-tab editing.

## 4. Proses feedback pilot

1. Pengguna mengirim laporan ke support (kanal dikonfirmasi pemilik).
2. Support mencatat ke issue log pilot (file: `docs/pilot-feedback-log.md` saat pilot berjalan).
3. Mingguan: triage bersama pemilik (blocker/major/minor) dan keputusan stop/go criteria.

## 5. Eskalasi

Mengikuti `docs/runbook.md` §6. Insiden data pribadi/credential langsung ke pemilik tanpa dibahas publik.
