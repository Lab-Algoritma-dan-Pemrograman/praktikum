# Audit Aksesibilitas Statis — Phase 7 (LOKAL)

Tanggal: 2026-11-13 · Target: WCAG 2.2 AA · Scope: `apps/**/+page.svelte`, `+layout.svelte`, komponen lib, `packages/ui`

## Ringkasan per App

| App | label form | heading | alt gambar | teks tombol/link | aria-live | lang | Temuan utama |
|---|---|---|---|---|---|---|---|
| portal | 0 | 1 (via AppShell h1) | n/a (tidak ada `<img>`) | 0 | 0 | 0 | Hanya AppShell; tanpa masalah material |
| auth | 0 | 1 (halaman login ada h1; root + callback/logout ada h1) | n/a | 0 | 1 (login: role=alert ada, tapi error di atas form tanpa fokus — minor) | 0 | Baik |
| elearning | 0 | 3 | n/a | 0 | 2 | 0 | root halaman mulai dari `h2` (h1 hanya di AppShell, level campur) |
| praktikum | 0 | 3 | n/a | 0 | 3 | 0 | root halaman mulai dari `h2` (h1 hanya di AppShell) |
| siakad | 0 | 4 | n/a | 0 | 3 | 0 | root halaman mulai dari `h2`; subsection langsung `h3`→`h4` tanpa h1 di main |
| admin | 1 | 6 | n/a | 2 (banyak tombol deklaratif; sedikit icon-only perlu cek) | 15 | 0 | Terbanyak; tapi role=status/alert konsisten |

Catatan: tidak ditemukan satu pun tag `<img>` di seluruh `apps/*/src` — kategori alt gambar bersih (0 temuan). Seluruh `<html lang="id">` benar di `apps/*/src/app.html:2`.

## Temuan Detail (file:line)

### 1. Hierarki heading terputus (h1 AppShell → h2 halaman, tanpa h1 konten)
- `apps/elearning/src/routes/+page.svelte:43` — `<h2 id="curriculum-title">` sebagai heading pertama main; h1 hanya di AppShell (packages/ui/src/AppShell.svelte:57).
- `apps/praktikum/src/routes/+page.svelte:37` — `<h2 id="sessions-title">` heading pertama.
- `apps/siakad/src/routes/+page.svelte:84` — `<h2 id="siakad-overview-title">` heading pertama.
- Info: axe `page-has-heading-one` mungkin pass karena AppShell punya h1, tapi struktur main sebaiknya konsisten. **Kategori ini BUKAN blocker** — dicatat sebagai catatan struktur.

### 2. `aria-live` region sangat sedikit
- Hanya 3 di seluruh repo: `apps/praktikum/src/lib/CodeEditor.svelte:141`, `packages/ui/src/BrowserPreview.svelte:287`, `packages/ui/src/BrowserPreview.svelte:318`.
- Feedback async di siakad/elearning/praktikum memakai `role="status"`/`role="alert"` (baik), tapi admin pages paling banyak (15+ region). Tidak ditemukan `aria-live` di portal/auth.

### 3. Icon-only button (potensi, perlu verifikasi runtime)
- `packages/ui/src/AdminShell.svelte:136` — button "Buka navigasi" (punya aria-label, OK).
- Semua button yang discan memiliki teks atau aria-label; tidak ditemukan button ikon tanpa label. Kategori ini bersih pada scan statis.

### 4. Label form
- Semua input/select/textarea di admin memakai pola `<label>…<input … /></label>` (wrapper implicit association) — valid.
- `apps/auth/src/routes/login/+page.svelte:32-49` — label eksplisit dengan `for`/`id`, sangat baik.
- Kategori ini bersih pada scan statis.

### 5. Error form login tanpa fokus / association
- `apps/auth/src/routes/login/+page.svelte:25` — error `role="alert"` ada, tapi tidak di-associate ke field (aria-describedby) dan tidak menerima fokus. Minor.

## Limat temuan terburuk (top 5)
1. **Heading h1-h2 gap di halaman utama siakad/elearning/praktikum** (lihat detail #1).
2. **aria-live minim** — hanya 3 di repo; feedback dinamis bergantung role=status/alert yang tidak selalu announce perubahan (detail #2).
3. `apps/auth/src/routes/login/+page.svelte:25` — error box tanpa aria-describedby/fokus ke field.
4. `apps/admin/src/routes/praktikum/Activations.svelte:173` — section notice hanya `aria-label` tanpa heading terlihat; ok tapi tergantung konteks SR.
5. Tidak ada temuan alt-gambar/lang/tombol — fondasi dasar sudah solid.

## Catatan metode
- Scan statis manual pada source; validasi runtime terisolasi ada di `tests/frontend/e2e/a11y.spec.ts`.
- Run Chromium 14 November 2026 lulus 12/12 dengan nol pelanggaran ruleset aktif; `color-contrast` tetap dikecualikan dan memerlukan audit manual.
- Temuan `document-title`, `link-name`, dan `scrollable-region-focusable` dari run awal ditarik karena target port ternyata milik project lain; detail koreksi ada di `docs/phase-7-browser-a11y.md`.

## Sisa pekerjaan
1. Audit association/focus error form login secara manual.
2. Evaluasi announcement feedback dinamis pada screen reader nyata.
3. Audit kontras dan keyboard-only end-to-end pada halaman authenticated.
