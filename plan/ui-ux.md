# UI/UX dan Frontend Target

**Status:** Draft 0.1

## 1. Referensi visual

- **Landing page:** [Burrow demo](https://8b.com/templates/burrow/demo.html)
- **Dashboard:** [Gradient Able analytics](https://codedthemes.com/demos/admin-templates/gradient-able/react/free/app/dashboard/analytics)

Referensi dipakai untuk bahasa visual, komposisi, hierarchy, dan interaction
pattern. Konten, identitas brand, copy, icon, dan asset Algohub dibuat sendiri.

## 2. Landing page tunggal

Landing page resmi hanya:

```text
https://algohub.web.id/
```

Struktur visual yang diadaptasi dari arah Burrow:

1. Hero dengan headline, value proposition, dan CTA login/masuk aplikasi.
2. Section kartu untuk SIAKAD, E-learning, Praktikum, dan layanan lab.
3. Alur penggunaan atau feature steps.
4. Informasi publik paling penting.
5. FAQ/contact/footer.

`/info` tidak menjadi landing page. Route lama dapat redirect ke `/`; halaman
publik spesifik menggunakan route eksplisit dari sitemap.

## 3. Bahasa visual dashboard

Dashboard seluruh subdomain memakai design system yang sama, dengan variasi warna
untuk identitas domain:

- sidebar/topbar konsisten;
- summary cards untuk metrik utama;
- gradient accent sebagai highlight, bukan background berlebihan;
- tabel, chart, filter, badge status, empty state, loading state, dan error state
  memakai komponen bersama;
- halaman admin memiliki hierarchy jelas antara overview, data table, form, dan
  destructive action.

Admin terpusat memakai app shell sendiri di `admin.algohub.web.id`: app switcher
untuk Praktikum/E-learning/SIAKAD, breadcrumb domain, scope selector, dan menu
yang dibentuk dari permission. Domain user tidak memiliki shell atau route
`/admin` lokal.

Referensi Gradient Able digunakan sebagai arah layout analytics dashboard, bukan
sebagai salinan asset atau source code.

## 3.1 Saran agar desain tidak menjadi AI slop

Landing page dan dashboard tidak dibuat sebagai satu template generik. Keduanya
memiliki tujuan dan ritme yang berbeda, tetapi berbagi design tokens dan
komponen dasar.

### Design DNA Algohub

- Posisi brand: **calm technical lab** — akademik, modern, dan terasa dibuat untuk
  aktivitas coding/praktikum, bukan dashboard SaaS generik.
- Landing memakai editorial layout, whitespace, headline yang kuat, ilustrasi
  atau visual code yang relevan, dan data nyata dari Algohub.
- Dashboard memakai layout utilitarian yang padat informasi: sidebar, topbar,
  summary cards, tabel, status, dan action yang jelas.
- Gradient hanya menjadi accent pada area tertentu. Jangan menjadikan semua card
  gradient, glassmorphism, blob, atau animasi sebagai dekorasi default.
- Gunakan copy Bahasa Indonesia yang spesifik terhadap mahasiswa, asisten,
  kelas, sesi, dan laboratorium; hindari copy placeholder seperti “unlock your
  potential” atau “seamless experience”.
- Setiap domain boleh memiliki accent color, tetapi typography, spacing, radius,
  elevation, focus state, dan behavior tetap berasal dari design system bersama.

### Aturan anti-template

- Jangan menerima hasil generate utuh tanpa design brief, content model, dan
  screenshot review.
- Komponen dipilih karena menyelesaikan kebutuhan UI nyata, bukan karena sedang
  populer di katalog.
- Maksimal satu pola hero, satu pola card metric, satu pola data table, dan satu
  pola empty state per design system.
- Animasi harus menjelaskan perubahan state atau hierarchy; `prefers-reduced-motion`
  wajib dihormati.
- Foto, icon, ilustrasi, dan data dummy tidak boleh menjadi identitas final.
- Review dilakukan pada mobile lebih dulu untuk mencegah layout desktop yang
  dipaksa mengecil.

## 4. Responsive contract

| Kategori | Lebar uji |
|---|---:|
| Small mobile | 320–374 px |
| Large mobile | 375–767 px |
| Tablet | 768–1023 px |
| Desktop | 1024–1439 px |
| Wide desktop | 1440 px+ |

Aturan wajib:

- Mobile-first; tidak ada layout yang hanya benar pada desktop.
- Sidebar berubah menjadi drawer/compact navigation pada mobile.
- Tabel besar berubah menjadi card, accordion, atau horizontal scroll yang
  memiliki indikator jelas.
- Modal tidak boleh melebihi viewport; form memakai full-screen sheet pada mobile.
- CTA utama, timer, status save, dan tombol submit tetap terlihat dan mudah
  disentuh.
- Touch target minimal 44×44 px.
- Tidak ada teks terpotong tanpa tooltip/overflow yang disengaja.
- Grid memakai breakpoint dan fluid sizing, bukan fixed width yang menyebabkan
  horizontal overflow.
- Editor kode mendukung scroll horizontal internal tanpa merusak halaman.
- Kontras warna, focus ring, keyboard navigation, semantic heading, label form,
  dan reduced-motion mode wajib tersedia.

## 5. State yang harus didesain

Setiap halaman data dan proses wajib memiliki loading/skeleton, empty state,
error state dengan retry, unauthorized/forbidden state, offline/reconnecting
state untuk realtime, success feedback, dan destructive confirmation.

## 6. Praktikum dan terminal coding

- Timer menampilkan waktu tersisa berdasarkan `deadline_at` server.
- Browser clock tidak menjadi sumber perhitungan.
- Auto-save menampilkan `saving`, `saved at`, dan `retrying`.
- Layout editor desktop dapat split-pane; mobile memakai tab/stacked panel.
- Output terminal memiliki batas tinggi dan tombol clear/download yang jelas.
- Saat waktu habis, UI menonaktifkan input lalu menunggu konfirmasi submit server.

## 7. Frontend architecture

```text
apps/
  portal/
  auth/
  admin/
  siakad/
  elearning/
  praktikum/
packages/
  ui/
  design-tokens/
  api-client/
  auth-client/
  validation/
```

Semua app memakai tokens dan komponen UI bersama. API client memiliki typed
contract dari OpenAPI; komponen tidak melakukan query database langsung.

## 7.1 Workflow MCP 21st

MCP 21st digunakan sebagai alat eksplorasi dan pemasangan komponen, bukan sebagai
pengganti keputusan desain.

1. Tulis design brief dan tokens Algohub terlebih dahulu.
2. Cari beberapa alternatif lewat `search`/`get_inspiration` berdasarkan konteks
   landing, dashboard, mobile, dan domain praktikum.
3. Pilih komponen yang sudah memiliki preview, dependency, dan struktur jelas.
4. Pasang hanya primitive/section yang diperlukan ke shared UI package.
5. Adaptasi spacing, typography, color, copy, accessibility, dan responsive
   behavior ke design system Algohub.
6. Review screenshot pada mobile/tablet/desktop sebelum komponen dipakai lintas
   aplikasi.
7. Catat keputusan diterima/ditolak agar iterasi berikutnya tidak mengulang
   eksperimen yang sama.

MCP server project: `https://21st.dev/api/mcp`. Konfigurasi berada di root
`opencode.jsonc` dan menggunakan `x-api-key: {env:API_KEY_21ST}`.

## 8. Definition of done UI

- Lulus screenshot review pada seluruh viewport target.
- Tidak ada horizontal overflow yang tidak disengaja.
- Keyboard-only flow untuk login, navigasi, form, modal, dan submit.
- Lighthouse/performance review untuk landing dan dashboard.
- Error, loading, empty, unauthorized, dan offline state tersedia.
- Uji browser untuk SSO pindah subdomain dan deep link.
- Tidak ada komponen yang masuk production tanpa review visual dan adaptasi
  terhadap design tokens Algohub.
