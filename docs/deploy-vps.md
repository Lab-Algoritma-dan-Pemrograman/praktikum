# Arsip: Deploy Lab-AP di VPS Paluketok

> **Bukan panduan produksi saat ini.** Sejak **2026-11-01**, `algohub.web.id`
> dilayani **Instabox melalui Cloudflare Tunnel + Caddy**, bukan Paluketok.
> Lihat [status migrasi dan ingress aktif](../../_docs.old/migrasi/00-STATUS.md).
> `scripts/deploy-vps.sh` masih menargetkan server lama; jangan menjalankannya
> untuk memperbarui situs live atau menyalakan kembali service Algohub di Paluketok.

Origin aktif: `/data/praktikum/frontend`, `praktikum-frontend.service` (`:3000`),
`praktikum-backend.service` (`:8081`), dan Caddy (`:4175`). File worker dilayani
langsung dari `frontend/static` oleh Caddy, sehingga header di `hooks.server.ts`
saja tidak cukup untuk worker. Pengaturan DNS/perintah di bawah merupakan arsip.

**Kondisi historis pada 2026-10-24:** seluruh stack dipindah ke VPS yang sama
dengan PaluKetok + Naupatrol karena Vercel terkena `DEPLOYMENT_DISABLED`
(402 Payment Required).

## Peta

| Bagian | Lokasi / port |
| --- | --- |
| Go API | `/opt/lab-ap/backend`, `lab-ap-api.service`, `127.0.0.1:8082` |
| Web (SvelteKit node) | `/opt/lab-ap/frontend`, `lab-ap-web.service`, `127.0.0.1:3002` |
| Env backend | `/opt/lab-ap/backend/.env` (0600, root, TIDAK di repo) |
| Nginx vhost | `/etc/nginx/sites-available/lab-algohub` |
| `algohub.web.id` | reverse proxy ke `:3002` |
| `api.algohub.web.id` | reverse proxy ke `:8082` |
| Runtime | Go 1.24 di `/usr/lib/go-1.24`, Node 22 di `/opt/node22`, Bun di `~/.bun/bin` |
| Deploy | `bash scripts/deploy-vps.sh` (dari root repo ini) |

Perubahan dari setup Vercel:

- `frontend/svelte.config.js` + `package.json`: `@sveltejs/adapter-vercel` -> `@sveltejs/adapter-node`.
- `backend/api/index.go` dan `vercel.json` tidak dipakai lagi. Server persisten
  (`cmd/server`) memakai sweeper goroutine, jadi cron-job.org untuk auto-submit
  tidak perlu lagi.
- CORS `CORS_ORIGINS` di VPS: `algohub.web.id`, `www`, `api`, `elearning`.
- Port baru dipilih supaya tidak bentrok: SFM pakai 8080, PaluKetok 8081.

## Belum selesai: DNS

TTL record `api.algohub.web.id` di Cloudflare masih 1 hari. Yang harus diubah,
di zona `algohub.web.id`:

1. `A algohub.web.id` -> `103.119.237.29` (Proxy status **DNS only**, jangan
   oranye. Nginx pakai certbot HTTP-01; proxy Cloudflare memutus verifikasi dan
   menyembunyikan IP asli).
2. `A www.algohub.web.id` -> `103.119.237.29` (DNS only) atau CNAME ke `algohub.web.id`.
3. `A api.algohub.web.id` -> `103.119.237.29` (DNS only). Hapus record Vercel.

Setelah propagasi, terbitkan cert asli:

```bash
ssh paluketok 'certbot certonly --webroot -w /var/www/html \
  -d algohub.web.id -d www.algohub.web.id -d api.algohub.web.id \
  --non-interactive --agree-tos --register-unsafely-without-email \
  && systemctl reload nginx'
```

Saat ini `/etc/letsencrypt/live/algohub.web.id/` berisi cert self-signed
sementara (2 hari) supaya nginx bisa start sebelum DNS pindah.

## Verifikasi

```bash
curl -s https://api.algohub.web.id/api/health          # {"service":"lab-ap","status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' https://algohub.web.id/praktikum/login   # 200
ssh paluketok 'journalctl -u lab-ap-api -u lab-ap-web -n 50 --no-pager'
```

## Catatan operasional

- `POST /api/cron/auto-submit` + `X-Cron-Secret` masih ada di backend tapi tidak
  lagi wajib: sweeper goroutine `cmd/server` sudah menangani auto-submit tiap
  `SWEEPER_INTERVAL_SECONDS`.
- Swagger sengaja mati karena `APP_ENV=production` (perilaku lama, lihat
  `internal/delivery/http/route/router.go`).
- `proxy_buffer_size 32k` di vhost wajib ada: header dari SvelteKit (env publik +
  CSP) lebih besar dari default 4k dan menghasilkan 502 `upstream sent too big header`.
