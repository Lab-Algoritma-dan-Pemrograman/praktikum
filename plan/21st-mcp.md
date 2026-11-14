# Workflow MCP 21st untuk Design System Algohub

**Status:** Terpasang pada konfigurasi project, belum memasang komponen UI ke
source code.

## 1. Konfigurasi project

MCP 21st sudah didaftarkan di:

```text
E:\rekapProject\project_ap_v2\opencode.jsonc
```

Endpoint:

```text
https://21st.dev/api/mcp
```

Credential dibaca dari environment lokal:

```text
API_KEY_21ST
```

API key tidak boleh ditulis ke `opencode.jsonc`, dokumentasi, commit, atau file
`.env` yang masuk repository. Jika belum tersedia, MCP akan tetap tercatat tetapi
belum dapat dipakai sampai environment key dikonfigurasi.

## 2. Kapabilitas yang dipakai

- `search`: mencari komponen, theme, dan template.
- `get_inspiration`: membandingkan arah desain berdasarkan konteks project.
- `get_component`: mengambil source komponen terpilih.
- `get_theme`: mengambil theme/token yang relevan.
- `search_logo`: mencari logo/icon bila dibutuhkan.

Generation penuh diperlakukan sebagai opsi terakhir. Untuk menghindari AI slop,
komponen katalog yang teruji dan adaptasi manual lebih diutamakan daripada
generate satu halaman penuh.

## 3. Aturan pemasangan komponen

- Tidak memasang komponen ke aplikasi production sebelum design tokens final.
- Tidak memasang seluruh template sebagai satu paket monolitik.
- Komponen masuk ke `packages/ui` atau struktur shared UI yang disepakati.
- Dependency dan license setiap komponen dicatat sebelum dipakai.
- Komponen harus lolos responsive review, accessibility review, dan visual review.
- Komponen tidak boleh membawa data fetching langsung atau query Supabase.
- Komponen yang tidak cocok diadaptasi atau ditolak; jangan memaksa design system
  mengikuti katalog.

## 4. Proses eksplorasi desain

### Landing page

Cari alternatif untuk editorial hero, feature steps, service cards, testimonials
atau proof section, dan responsive navigation. Arah visual utamanya mengikuti
Burrow: tenang, editorial, whitespace, dan storytelling.

### Dashboard

Cari alternatif untuk sidebar, metric cards, data table, activity feed, chart,
filter, status badge, empty state, dan responsive drawer. Arah layout utamanya
mengikuti Gradient Able: dashboard analytics yang terstruktur dan efisien.

### Praktikum coding

Cari pola untuk split editor, timer/deadline state, terminal output, autosave
indicator, question navigator, dan mobile stacked layout. Komponen harus mendukung
deadline server-side dan state reconnect.

## 5. Checklist review sebelum menerima komponen

- Apakah komponen terlihat seperti bagian dari Algohub, bukan template asal?
- Apakah content dan hierarchy sesuai konteks mahasiswa/lab?
- Apakah mobile 320–375 px tetap usable?
- Apakah keyboard, focus, contrast, dan reduced motion aman?
- Apakah loading, empty, error, forbidden, dan offline state tersedia?
- Apakah dependency, source, dan license jelas?
- Apakah komponen dapat dipakai lintas subdomain tanpa membawa business logic?
