# DNS staging rewrite — record dari Vercel

Target: tujuh project `algohub-v2-staging-*` pada akun/team khusus rewrite `lab-algoritma-pemrograman`. Nilai berikut adalah historical read-only evidence dari API Vercel pada 10 November 2026, bukan instruksi untuk melakukan mutasi. Phase 7A target architecture now uses `staging.algohub.web.id` as the landing root; preserve the historical `portal.staging` record below only as superseded evidence until the owner approves a separate migration.

**Jangan ubah record produksi** seperti `algohub.web.id`, `api.algohub.web.id`, `siakad.algohub.web.id`, atau `elearning.algohub.web.id`. Tambahkan hanya record staging dan TXT verifikasi berikut. Pertahankan semua TXT `_vercel` yang sudah ada untuk project lain.

## 1. CNAME

Di zona DNS `algohub.web.id`, tambahkan:

| Type | Name relatif dalam zona | Target |
| --- | --- | --- |
| CNAME | `portal.staging` (historical/superseded) | `30c182d16e1ed772.vercel-dns-017.com.` |
| CNAME | `auth.staging` | `a36522adf5ff1b48.vercel-dns-017.com.` |
| CNAME | `admin.staging` | `e2c69f0eed57b9d4.vercel-dns-017.com.` |
| CNAME | `praktikum.staging` | `f1dd1b63d8797cb5.vercel-dns-017.com.` |
| CNAME | `elearning.staging` | `ccca31de0057155e.vercel-dns-017.com.` |
| CNAME | `siakad.staging` | `c8e64692b5a8ddc2.vercel-dns-017.com.` |
| CNAME | `api.staging` | `846b660f2db4ac65.vercel-dns-017.com.` |

Gunakan TTL Auto atau 300 detik. Jika panel meminta hostname lengkap, misalnya `portal.staging.algohub.web.id`, gunakan bentuk lengkap satu kali—jangan sampai zona ditambahkan dua kali. Jika memakai Cloudflare, gunakan DNS only untuk verifikasi awal.

## 2. TXT verifikasi kepemilikan subdomain

Tambahkan **tujuh record TXT terpisah** dengan name relatif `_vercel` (hostname lengkap `_vercel.algohub.web.id`). Jangan menggabungkan nilainya menjadi satu string dan jangan mengganti TXT lama.

| Name | Value |
| --- | --- |
| `_vercel` | `vc-domain-verify=portal.staging.algohub.web.id,512457900e6743d47e93` (historical/superseded) |
| `_vercel` | `vc-domain-verify=auth.staging.algohub.web.id,ac673ba2ba60da353ea8` |
| `_vercel` | `vc-domain-verify=admin.staging.algohub.web.id,9c833f8c8ddb8ea47a40` |
| `_vercel` | `vc-domain-verify=praktikum.staging.algohub.web.id,af12ce3b237c2693a4f9` |
| `_vercel` | `vc-domain-verify=elearning.staging.algohub.web.id,a6414207ff634c13b72a` |
| `_vercel` | `vc-domain-verify=siakad.staging.algohub.web.id,962c5b18d05619d211b7` |
| `_vercel` | `vc-domain-verify=api.staging.algohub.web.id,6e96b03dd7518e7a3d2d` |

Nilai TXT ini memang untuk dipublikasikan di DNS; bukan password atau token login aplikasi. Jika domain dilepas/didaftarkan ulang, ambil ulang nilai dari Vercel sebelum memakai catatan ini.

## 3. Setelah disimpan

1. Konfirmasikan bahwa semua record sudah tersimpan. Tidak perlu membagikan credential DNS.
2. Periksa resolusi CNAME/TXT, jalankan verifikasi domain pada project staging terkait, lalu tunggu HTTPS aktif.
3. Uji health/readiness API dan header aset statis pada hostname staging.
4. Baru jalankan login/SSO lintas aplikasi dengan akun uji yang disetujui.

DNS pada provider belum diubah oleh penyiapan dokumen ini. Saat record diambil, ketujuh domain masih `verified: false` dan konfigurasi DNS belum benar.

## 4. Hasil verifikasi — 10 November 2026, 23:38 WIB

- Historical evidence: setelah pemilik menyimpan record Cloudflare, ketujuh CNAME dan ketujuh TXT cocok dengan tabel di atas pada Google, Cloudflare, serta kedua nameserver authoritative (`dion.ns.cloudflare.com`, `dolly.ns.cloudflare.com`).
- Akun Vercel diverifikasi sebagai `lordbgsdts-afk`, team `lab-algoritma-pemrograman` (`team_xEm9gJWxtM1sb09eyCeIN4js`). Portal sudah verified; verifikasi ownership enam domain staging lainnya berhasil. Ketujuh domain kini `verified: true`, `misconfigured: false`, dan `configuredBy: CNAME`.
- HTTPS ketujuh hostname berhasil. Portal yang sebelumnya 502 kini 200; API `/healthz` dan `/readyz` 200. Admin dan API `/v1/auth/me` menolak request tanpa sesi dengan 401. Halaman login Auth Center 200.
- Verifikasi ini hanya menyentuh domain pada project staging rewrite; tidak mengubah DNS atau project produksi. Tidak ada login pengguna atau pengujian SSO/revocation/permission dengan akun nyata.

Bukti smoke lebih lengkap dan gate berikutnya dicatat pada [update log](updates/README.md).
