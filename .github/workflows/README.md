# Auto-deploy subdomain Algohub

Satu workflow, tujuh target: setiap app Vercel di-deploy otomatis saat `main` berubah.

## Alur

```
push ke main ──► deteksi perubahan per app ──► build ──► deploy production ──► smoke check
```

- Filter path: perubahan `packages/ui`, `auth-client`, `practicum-client`, dll. memicu app yang memakainya, bukan semua app.
- `max-parallel: 4` supaya build tidak saling berebut.
- `concurrency` mencegah dua deploy bersamaan untuk ref yang sama.
- `workflow_dispatch` bisa dipakai untuk deploy satu target saja (`all` untuk semuanya).

## Secret yang wajib diisi

Di **GitHub → Settings → Secrets and variables → Actions**, tambahkan:

| Secret | Nilai |
| --- | --- |
| `VERCEL_TOKEN` | Token Vercel (Account Settings → Tokens) |
| `VERCEL_ORG_ID` | `team_xEm9gJWxtM1sb09eyCeIN4js` |
| `VERCEL_PROJECT_ID_api` | `prj_D8PyzN0jj0eJ2Ve0N59CGQbeAjXr` |
| `VERCEL_PROJECT_ID_portal` | `prj_9VLgDPCttE7TJMTr0ZObPF9VbnLo` |
| `VERCEL_PROJECT_ID_auth` | `prj_HVAOvl2wCWcqEIOIt6VXe5qELk8Q` |
| `VERCEL_PROJECT_ID_admin` | `prj_GSJ6ESIbfXfqLM7xnHV34wMWDvHj` |
| `VERCEL_PROJECT_ID_siakad` | `prj_GYc2Yhpqg1cxcV6eXKVPhFmug0kg` |
| `VERCEL_PROJECT_ID_elearning` | `prj_TF3cUSxlLoqHEuHDLDxDx4IZtRlI` |
| `VERCEL_PROJECT_ID_praktikum` | `prj_1S8FATn2E6pYpTgP9tQE1YVB4fql` |

Tanpa secret, workflow gagal di langkah deploy — build tetap jalan dan tetap memberi sinyal.

## Catatan

- Workflow ini **tidak menjalankan migrasi database**. Perubahan skema tetap lewat `cmd/migrate` dengan persetujuan terpisah, karena migrasi production bersifat irreversible.
- Smoke check memakai status yang memang benar: `api`/`portal`/`auth`/`praktikum` → 200, host admin/siakad/elearning → 401 saat belum login.
- Rahasia tidak pernah ditulis ke log; `--token` diambil dari secret GitHub.
