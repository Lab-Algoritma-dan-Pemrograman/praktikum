# Phase 7A Design Context

**Status:** local implementation context, 2026-11-14  
**Audience:** mahasiswa, asisten, koordinator, dan peminjam Algohub.

## Design DNA

Algohub adalah **calm technical lab**: akademik, praktis, terstruktur, dan spesifik pada kelas, sesi, lab, materi, submission, dan permission. Landing memakai ritme editorial; app shell memakai hierarchy utilitarian. Domain boleh memiliki accent sendiri, tetapi focus, spacing, radius, state behavior, dan responsive rules berasal dari token bersama.

## Content rules

- Bahasa Indonesia yang konkret dan tidak menjanjikan fitur yang belum tersedia.
- Tidak membuat statistik, testimonial, kontak, nama, jadwal, NIM, atau data operasi.
- Data publik yang belum memiliki kontrak aman ditampilkan sebagai blocked/empty state yang menjelaskan batasnya.
- Nilai akademik tetap dipisahkan dari preview/run code browser.
- Error state menampilkan tindakan aman: retry, login kembali, atau kembali ke route resmi.

## Visual system

Tokens non-secret berada di `packages/ui/src/tokens.css` dan dipakai oleh shared Svelte primitives. Portal mempertahankan token editorial lokal melalui alias `--ah-*`; ini menghindari perubahan business flow sambil menyediakan jalur konsolidasi bertahap.

- Ink: `#142724`
- Cream: `#f5f0e7`
- Surface: `#e8e2d6`
- Coral/action: `#ee785c`
- SIAKAD blue: `#7eb7c4`
- Learning yellow: `#e4bb5e`
- Lab green: `#62aa8f`
- Minimum control: 44px
- Focus: 3px coral outline with offset
- Motion: short state/interaction transitions only; reduced motion disables transitions/animations
- No gradient, glassmorphism, stock-photo dependency, icon package, or new UI dependency

## Layout rules

- Mobile-first at 320–374px; large mobile 375–767px; tablet 768–1023px; desktop 1024–1439px; wide 1440px+.
- Public landing and public information pages use a centered max-width and no intentional horizontal page overflow.
- Admin sidebar becomes a keyboard-trapped drawer on compact screens.
- Large data views remain bounded by their own scroll/card layout; the page itself must not overflow.
- Public route footer links expose privacy/terms paths without claiming legal approval.

## State rules

Every new public route has a heading, explanatory description, status state, action, responsive layout, and no data request until a safe target contract exists. This is deliberate fail-closed behavior for legacy PII and file URL risks.
