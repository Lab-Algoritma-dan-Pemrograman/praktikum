# Phase 7 — Landing Page Design Review

Status: **local draft under review**, 14 November 2026. Landing belum dideploy ke staging.

## Design brief

**Audience:** mahasiswa, asisten, koordinator, dan peminjam yang perlu memahami Algohub lalu masuk ke aplikasi sesuai role.

**Tujuan:** menjadikan `/` satu-satunya landing resmi sekaligus pintu masuk menuju SIAKAD, E-learning, Praktikum, dan Admin tanpa mencampur landing dengan dashboard.

**Design DNA:** calm technical lab — editorial, akademik, modern, spesifik pada pembelajaran/praktikum/laboratorium, dan tidak menyerupai landing SaaS generik.

**Batas konten:** tidak mengarang nama asisten, statistik, testimonial, kontak, jadwal, atau data operasional. Informasi dinamis/public routes dipisahkan dari landing sampai kontrak datanya tersedia.

## Sumber keputusan

1. `plan/ui-ux.md`: hero, service cards, steps, public information, FAQ/contact/footer; mobile-first dan anti-AI-slop.
2. `plan/ref/landingPage/`: Burrow sebagai referensi hierarchy, whitespace, flat shapes, arch language, dark interruption band, dan reduced-motion fallback.
3. `plan/feature-matrix.md`: parity Portal mencakup landing, jadwal, asisten, laporan, modul, dan bantuan.
4. Legacy `project_ap/praktikum/frontend/src/routes/info/`: sumber perilaku/data untuk audit route publik; tidak disalin langsung.
5. MCP 21st: eksplorasi alternatif section, bukan generator halaman penuh.

## Content model draft

| Section | Kebutuhan | Implementasi draft |
| --- | --- | --- |
| Navigation | Brand, anchors, login/workspace CTA, mobile menu | Ada; `<details>` tanpa library |
| Hero | Headline, value proposition, CTA, visual code/lab | Ada; ilustrasi CSS tanpa stock asset |
| Services | SIAKAD, E-learning, Praktikum, operasional | Empat card dengan origins dari config |
| Steps | Cara kerja identitas dan scope | Tiga langkah |
| Public information | Tautan ruang utama | Ada; tidak mengarang data publik |
| Trust/proof | Bukti nyata atau batas produk | Menggunakan product boundary browser-XP/manual-grade, bukan fake metrics |
| FAQ | Identity, menu scoped, nilai, rental | Empat pertanyaan faktual |
| Contact/footer | Kontak nyata, legal, attribution | Footer ada; kontak nyata masih menunggu owner |
| Authenticated state | Resume session + app switcher | Ada; hanya tampil bila principal tersedia |

## Tokens draft

| Token | Nilai | Alasan |
| --- | --- | --- |
| Ink | `#142724` | Near-black green untuk technical/lab tone |
| Cream | `#f5f0e7` | Editorial background dari arah Burrow |
| Surface | `#e8e2d6` | Flat secondary surface |
| Coral | `#ee785c` | CTA dan arch accent |
| Blue | `#7eb7c4` | SIAKAD/service accent |
| Yellow | `#e4bb5e` | Learning accent |
| Green | `#62aa8f` | Lab/operation accent |

Tidak ada gradient, glassmorphism, stock photo, icon package, atau animation dependency. Header memakai background solid; motion hanya hover kecil dan dimatikan pada reduced motion.

## Eksplorasi MCP 21st

Search 14 November 2026 memakai kebutuhan editorial education/laboratory, responsive navigation, service cards, steps, proof, dan FAQ.

| Kandidat | Keputusan | Alasan |
| --- | --- | --- |
| Shape Landing Hero — kokonutd | **Ambil arah** | Flat shape language cocok; source tidak diambil karena React/Tailwind dan draft Svelte sudah memakai primitive CSS |
| Editorial service list — ibrandify | **Ambil hierarchy** | Numbered editorial rows cocok untuk information section; diadaptasi manual tanpa dependency |
| DevTool Landing Page — mokshithgujjeti | **Ambil ide visual code** | Konteks developer relevan, tetapi full-page block ditolak agar tidak monolitik/AI-slop |
| Hero Section with Responsive Navigation — socolled | **Tolak source** | Terlalu generik SaaS dan stack React/Tailwind tidak sesuai |
| Modern Landing Hero — mokshithgujjeti | **Tolak** | Tidak cukup spesifik pada konteks Algohub |
| Experience Hero — hardikkashiyani123456788 | **Tolak** | GSAP/WebGL/high-motion bertentangan dengan simplicity, performance, dan reduced-motion direction |
| Editorial Collage Hero — felipemenezes098 | **Tolak untuk sekarang** | Bergantung imagery; Algohub belum memiliki asset final terverifikasi |

Tidak ada source komponen 21st yang diunduh, dependency yang ditambah, atau license baru yang masuk aplikasi.

## Adaptasi Burrow

Dipakai:

- cream editorial ground dan near-black interruption band;
- headline besar dengan whitespace;
- arch/circle/square sebagai CSS shapes;
- flat service panels dan rounded arch CTA;
- section rhythm: opening → services → steps → information → FAQ → closing;
- static readable layout pada reduced motion.

Tidak dipakai:

- copy, foto, data, form, kontak, atau identitas veterinary;
- inline motion kit/GSAP/Lenis;
- stock assets dan Google Fonts;
- source template monolitik.

Referensi lokal berlisensi MIT. Footer draft mempertahankan tautan “Design direction: 8B Burrow”.

## Review teknis sementara

- Portal Svelte/type check: 0 error/0 warning.
- A11y isolated: 12/12 pass, 0 active-rule violations untuk scope test.
- Portal E2E: 4/4 pass pada 320, 768, 1280 dan authenticated state.
- No horizontal overflow pada tiga viewport tersebut.
- Reduced motion diaktifkan pada E2E.

## Gap sebelum design sign-off

1. Screenshot review aktual pada 320/375/768/1024/1440.
2. Keyboard-only visual review dan manual contrast check.
3. Owner memilih atau memberi kontak resmi; tidak boleh dibuat-buat.
4. Audit route publik `/jadwal`, `/asisten`, `/laporan`, `/modul`, `/bantuan`, `/privacy`, `/terms` terhadap legacy + target API.
5. Lighthouse/performance review.
6. Deploy staging hanya setelah owner menyetujui visual lokal.
