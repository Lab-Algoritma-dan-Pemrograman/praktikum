# Feature Parity Matrix

**Status:** Baseline audit 0.1 — diperbarui 2026-11-04

Dokumen ini menjadi daftar kendali parity. Status `audit` berarti temuan dari
source lama, bukan persetujuan bahwa implementasi target sudah selesai.

## 1. Temuan dari project lama

| Area | Bukti audit | Fitur yang wajib dipertahankan | Target domain | Status |
|---|---|---|---|---|
| Portal | `project_ap` dan sitemap lama | landing, jadwal, asisten, laporan, modul, bantuan | `algohub.web.id` | audit |
| Praktikum | `praktikum/README.md`, Go handlers, migrations | sesi, token, course, soal, timer, autosave, auto-submit, nilai, AI grading | `praktikum` + `admin/praktikum` | audit |
| E-learning | `elearning/PRD.md`, `src/pages`, services | level, modul, lesson, quiz, progress, XP, streak, achievement, leaderboard, Bug Hunt | `elearning` + `admin/elearning` | audit |
| Code lab | `elearning/src/hooks/useCodeRunner.ts`, workers | Python/C editor, stdin, stdout, compile/runtime error, test case | `elearning`/`praktikum` | audit |
| SIAKAD | `siakad/src/pages`, RPC route dan migrations | user, jadwal, absensi, kelas, inventaris, penyewaan, laporan, notifikasi | `siakad` + `admin/siakad` | audit |
| Auth | `auth.tsx`, `api/auth`, SSO migrations | login, logout, session verify, revoke, cross-app identity | `auth` + API | audit |
| Admin | `praktikum/README.md`, `elearning/AdminDashboard`, SIAKAD management pages | konfigurasi lintas domain, RBAC, monitoring, audit | `admin.algohub.web.id` | target |

## 2. Format item parity

Setiap fitur pada saat audit lanjutan harus memiliki baris dengan atribut berikut:

| Field | Isi wajib |
|---|---|
| `feature_id` | ID stabil, contoh `PRA-ATTEMPT-TIMER` |
| `legacy_sources` | route, file, RPC, tabel, atau migration lama |
| `user_flow` | langkah user dari awal sampai selesai |
| `target_domain` | host dan route target |
| `roles` | role/divisi/scope yang dapat memakai fitur |
| `api_contract` | operation ID OpenAPI |
| `data_contract` | tabel/entity dan event yang terlibat |
| `acceptance_tests` | test unit/integration/E2E yang membuktikan fitur |
| `migration_note` | cara membawa data lama atau alasan tidak dimigrasikan |
| `status` | `audit`, `specified`, `implemented`, `verified`, `cutover` |

Tidak boleh menutup fitur hanya karena halaman sudah tampil. Fitur baru berstatus
`verified` setelah API, permission, data, error state, responsive UI, dan test
user flow lulus.

## 3. Prioritas parity

1. **P0:** identity/SSO, users, role/permission, sesi praktikum, timer,
   submission, nilai, absensi, jadwal, dan data migrasi inti.
2. **P1:** e-learning content/progress, inventaris, penyewaan, laporan,
   monitoring, audit, dan upload.
3. **P2:** gamifikasi lanjutan, Bug Hunt, AI hint/grading enhancement,
   notifikasi tambahan, dan polishing UI.

Setiap P0 harus memiliki rollback atau compatibility path sebelum cutover.
