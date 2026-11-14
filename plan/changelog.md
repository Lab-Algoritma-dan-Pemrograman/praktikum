# Planning Changelog

Format: tanggal — perubahan — alasan — dampak — dokumen terkait.

## 2026-11-04

- Menetapkan satu Supabase project/PostgreSQL untuk seluruh domain.
- Menetapkan project target harus fresh; project legacy hanya read-only source
  migrasi dan rollback window.
- Menetapkan Go API berjalan sebagai Vercel Serverless Functions; tidak ada state
  persisten atau worker daemon di Vercel.
- Menetapkan empat bucket Storage namespaced per domain.
- Menetapkan role target `koordinator`, `asisten`, `mahasiswa`, `peminjam`.
- Menetapkan divisi asisten: `sekretaris`, `bendahara`, `k3`, `labops`,
  `pendidikan`, `pdd`; divisi memberi permission, bukan role identity.
- Menambahkan feature parity, role matrix, database migration, deployment,
  database operations, code-runner, OpenAPI, testing, dan execution phases.
- Menambahkan aturan test di setiap bagian dengan request ID dan artifact debugging.
- Menetapkan saat fase eksekusi dimulai, seluruh `plan/**` dibekukan kecuali
  `plan/user_prompt/**`.

## Riwayat sebelumnya

- Centralized admin dipindahkan ke `admin.algohub.web.id` dengan modul Praktikum,
  E-learning, dan SIAKAD.
- Prompt lama dirapikan menjadi `plan/user_prompt/main.md`.
- `plan/main.md` dijadikan planning hub.
