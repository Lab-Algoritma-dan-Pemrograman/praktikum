# Phase 7 — Pilot Plan (usulan; butuh persetujuan pemilik)

Status: **usulan**, belum ada peserta/undangan/keputusan. Menjalankan pilot butuh keputusan manusia. Template ini siap; field keputusan dikosongkan.

## 1. Tujuan

Validasi alur inti pada pengguna nyata kecil sebelum Phase 8 (cutover): login/SSO, E-learning comprehension + editor kode browser, Praktikum submission + grading manual, SIAKAD absensi/penyewaan, dan dukungan operasional.

## 2. Kohor usulan

| Aspek | Usulan | Keputusan pemilik |
| --- | --- | --- |
| Ukuran | 5–10 mahasiswa + 1–2 asisten + 1 koordinator | (isi) |
| Profil | Mahasiswa pemula (target utama), laporkan perangkat: laptop/ponsel, Chrome/Firefox/Edge | (isi) |
| Periode | 2 minggu, mis. 20 Nov – 4 Des 2026 | (isi) |
| Scope fitur | Login/SSO, ≥1 code lesson C/Python selesai, ≥1 Praktikum submit + grade manual, ≥1 absensi/penyewaan | (isi) |
| Support owner | (konfirmasi nama/kontak nyata) | (isi) |

## 3. Metrik

| Metrik | Target | Cara ukur |
| --- | --- | --- |
| Completion code lesson | ≥80% cohort | log completion lesson, tepat sekali |
| Submit Praktikum sukses | ≥90% | attempt status terminal |
| XP tepat sekali (nol ganda) | 100% | duplikat +0 XP |
| Issue blocker | 0 | log issue severity |
| Feedback response time | <1 hari kerja | support log |
| Perangkat blended | ≥2 jenis device | feedback form |

## 4. Proses feedback

- Kanal laporan: (dikonfirmasi pemilik, mis. Google Form / grup kontak).
- Pencatatan: `docs/pilot-feedback-log.md` (dibuat saat pilot berjalan).
- Triage mingguan bersama pemilik; blocker di-P0 fixed immediately.

## 5. Stop/Go criteria

**Go** bila: (a) 0 blocker, (b) metrik di atas tercapai, (c) dopamine dari feedback positif + asisten nyata bisa grading manual tanpa workaround.
**Stop/Pause** bila: (a) satu incident data/credential, (b) login/SSO downtime >4 jam, (c) cohort komplain blocker yang menghalangi 50% peserta menyelesaikan scope.

## 6. Data personal

Peserta, undangan, jadwal, dan penggunaan data personal adalah keputusan manusia. Tidak ada email/WA/undangan dikirim atau akun baru dibuat sampai pemilik menyetujui tujuan/penerima/scope secara eksplisit.

## 7. Status

**Blocked menunggu keputusan pemilik** untuk: peserta, undangan, periode final, support owner, kanal feedback, dan otorisasi kontak. Akun sintetis retained (`beban.01–18`, matriks role) dapat dipakai untuk technical readiness pilot, bukan bukti pilot pengguna nyata berjalan.
