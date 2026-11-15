# Scheduler Praktikum staging

Konfigurasi pada 11 November 2026, dengan persetujuan pemilik untuk mengaktifkan scheduler pada Supabase khusus rewrite dan redeploy hanya API staging. Tidak ada upgrade paket, vendor baru, perubahan produksi, atau fixture yang ditulis ke staging.

## Konfigurasi aktif

| Komponen | Nilai |
| --- | --- |
| Scheduler | Supabase `pg_cron` 1.6.4 + `pg_net` 0.20.4 |
| Secret storage | Supabase Vault 0.3.1 |
| Job | `algohub-staging-practicum-reconcile` |
| Schedule | `* * * * *`, satu kali setiap menit |
| Tujuan | `POST https://api.staging.algohub.web.id/v1/internal/practicum/reconcile` |
| Body | `{}` |
| HTTP timeout | 28.000 ms, tanpa retry HTTP otomatis dari konfigurasi job |
| Secret Vault | Nama `algohub_staging_practicum_cron` |
| Secret API | `ALGOHUB_PRACTICUM_CRON_SECRET`, encrypted pada slot Production **project staging terpisah** |

Secret dibuat acak, bukan menggunakan password database, anon key, atau service-role key. Nilai asli tidak dicantumkan di dokumen atau command cron. Job mengambilnya dari Vault saat enqueue request dan mengirimkannya sebagai Bearer ke satu tujuan HTTPS tetap.

Command tersimpan pada job:

```sql
select net.http_post(
  url := 'https://api.staging.algohub.web.id/v1/internal/practicum/reconcile',
  body := '{}'::jsonb,
  headers := jsonb_build_object(
    'Content-Type', 'application/json',
    'Authorization', 'Bearer ' || (
      select decrypted_secret
      from vault.decrypted_secrets
      where name = 'algohub_staging_practicum_cron'
    )
  ),
  timeout_milliseconds := 28000
);
```

Interval ini menghasilkan hingga 1.440 pemanggilan per hari. Itu bukan jaminan biaya atau kapasitas gratis. Deadline HTTP membatasi eksekusi request, bukan waktu tunggu di antrean `pg_net`.

## Pemeriksaan dan batas secret

- Endpoint nyata: GET 405; POST tanpa token dan dengan token salah 401; POST dengan token khusus 200, `{expired:0,jobs_processed:0}`. Respons endpoint `private, no-store`.
- Dua pemanggilan terjadwal nyata pada **11 November 2026, 00:30 dan 00:31 WIB** menghasilkan SQL `succeeded` serta HTTP 200 `{expired:0,jobs_processed:0}`, tanpa timeout/error. Request ID `pg_net` teramati 1 dan 2. Ini bukan sekadar pemanggilan manual atau keberhasilan enqueue.
- Database saat aktivasi memiliki nol attempt dan nol grading job. Pemeriksaan ini tidak membuktikan pemrosesan cohort berisi data.
- Vault tidak dapat dibaca oleh role `anon`/`authenticated`.
- `net.http_request_queue` menyimpan header Bearer dalam bentuk plaintext selama menunggu pengiriman. Grant PUBLIC milik `supabase_admin` pada antrean bawaan provider masih teramati setelah percobaan REVOKE lewat role `postgres`; jangan mengklaim antrean tidak mempunyai grant browser-role.
- Batas browser yang terverifikasi: `anon` dan `authenticated` tidak mempunyai LOGIN, dan skema `net` maupun `vault` ditolak oleh Data API dengan 406 `PGRST106`. Pemeriksaan GraphQL mengembalikan `pg_graphql extension is not enabled`, sehingga introspeksi tidak tersedia; bukan klaim bahwa skema GraphQL aktif sudah diaudit. Jangan mengekspos kedua skema melalui REST atau membuat RPC pembaca SQL/antrean generik. Akses database langsung harus dibatasi ke operator tepercaya.
- Jangan mencetak `vault.decrypted_secrets`, header/body antrean, nilai environment, atau credential pada log dan screenshot.

## Observasi pengiriman

Status `succeeded` pada `cron.job_run_details` hanya membuktikan SQL enqueue selesai. Keberhasilan HTTP harus diperiksa terpisah pada `net._http_response`.

```sql
select jobid, jobname, schedule, active
from cron.job
where jobname = 'algohub-staging-practicum-reconcile';

select runid, status, return_message, start_time, end_time
from cron.job_run_details
where jobid = (
  select jobid from cron.job
  where jobname = 'algohub-staging-practicum-reconcile'
)
order by runid desc
limit 10;

select id, status_code, timed_out, error_msg, created
from net._http_response
order by id desc
limit 10;
```

Saat pemasangan, scheduler ini satu-satunya pemakai `pg_net` pada project. Bila pemakai lain ditambahkan, jangan menganggap seluruh response table berasal dari job ini. Response `pg_net` disimpan sementara; dokumentasi provider menyebut retensi default enam jam dan tabel unlogged. Ini bukan penyimpanan bukti acceptance jangka panjang.

## Pause, resume, dan rotasi

Jalankan hanya pada database staging rewrite yang sudah diverifikasi:

```sql
select cron.alter_job(
  job_id := (
    select jobid from cron.job
    where jobname = 'algohub-staging-practicum-reconcile'
  ),
  active := false
);
```

Untuk resume, ubah `active` menjadi `true` setelah readiness API dan autentikasi diperiksa. Pause tidak membatalkan request yang sudah masuk antrean. Tidak perlu menghapus extension bersama untuk menghentikan job ini.

Rotasi secret: pause job, perbarui Vault dan encrypted API environment dengan nilai baru yang sama, redeploy hanya API staging, uji penolakan token lama/penerimaan token baru tanpa mencetak nilainya, kemudian resume. Jangan mengganti secret hanya di salah satu sisi.

## Kapasitas dan acceptance yang masih terbuka

- API staging dengan batch maksimum 20 job/invocation sudah dideploy sebagai `dpl_5jFo5vXrz91G1JKtUgjqLcsNJFPt` dan berstatus READY pada project staging terpisah. Endpoint tetap membatasi maksimal 100 attempt kedaluwarsa dan 20 grading job per panggilan.
- Lease grading kedaluwarsa dipulihkan sebelum job pending baru agar tidak terus tertunda ketika antrean pending terisi. Test lokal mencakup batch, cancellation, invocation bersamaan, dan 120 job dalam enam invocation.
- Gate kapasitas keseluruhan tetap terbuka. Acceptance di bawah membuktikan scheduler grading dan expiry pada staging, bukan code runner authoritative, provider grading nyata, contention/outage operasional, atau kapasitas produksi end-to-end.

## Acceptance batch 20 pada staging — 12 November 2026

Dataset retained bertag `STAGING-BATCH20-cdb70744` memakai 120 attempt pada 10 activation dan 12 identitas sintetis existing. Seluruh attempt dibuat dan jawabannya disimpan melalui API aplikasi; 254 request setup menghasilkan nol 429. Dataset ini tidak dibuat ulang saat verifikasi final.

- Deadline bersama: `2026-11-12T11:20:52.384Z`. Enam delivery cron nyata pada 11:21–11:26 UTC, response ID `pg_net` 2512–2517, semuanya HTTP 200 dan masing-masing memproses 20 job. Dua delivery pertama mengekspirasi 100 lalu 20 attempt.
- Seluruh 120 job mencapai `needs_manual` dalam 309.265 ms sejak deadline, yaitu 5 menit 9,265 detik. Observer dimulai setelah 100 job sudah selesai; delivery awal diverifikasi dari response/audit tersimpan, bukan diklaim diamati live sejak tick pertama.
- Tepat 120 audit claim dan 120 finalization tercatat dengan actor null. Semua jawaban beku cocok dengan hash input, seluruh attempt berstatus `submitted` dengan alasan `deadline`, dan tidak ada score terpublikasi.
- Satu job pending milik dataset diinjeksi sebagai lease running kedaluwarsa di bawah advisory lock dan audit sintetis. Cron memulihkannya sekitar 50 detik kemudian. Ini simulasi abandoned lease, bukan outage/crash worker nyata.
- Cleanup awal menutup 10 activation dan mengarsipkan session, tetapi laporan awal berhenti dengan `CLEANUP_INCOMPLETE_MANIFEST_PRESERVED`; status HTTP dan assertion penyebab tidak direkam oleh script tersebut. Failure asli tetap dipertahankan dan tidak dilabel ulang sebagai pass.
- Verifikasi final terpisah pada dataset yang sama kemudian lulus tanpa domain write: 120 submitted, 120 `needs_manual`, nol pending/running/lease, nol score, session inactive, 10 activation closed, cron tetap aktif `* * * * *`, dan nol sesi aktif milik actor run. Login verifikasi coordinator diakhiri logout dengan `/v1/auth/me` 401.
- Session, course, question, attempt, grading job, dan audit sintetis sengaja **retained**, bukan dihapus. Manifest terminal adalah `verified-and-inactive-retained`.

Bukti berada di `E:/tmp/algohub-v3-release-20261112-a83b/`: `scheduler-batch.json`, `scheduler-observation.json`, `scheduler-summary.json`, failure awal `scheduler-cleanup.json`, dan `scheduler-final-verification.json`. Bukti ini menutup bagian acceptance scheduler grading staging saja; gate kapasitas grading/server runner tetap terbuka.

## Acceptance lanjutan dengan data sintetis

Pada 11 November 2026, dengan izin data sintetis dan batas maksimal 120 attempt, dibuat **102 attempt pada enam activation untuk 17 identitas mahasiswa**, semuanya melalui API aplikasi. Jawaban disimpan sebelum deadline bersama. Tidak ada polling browser atau GET attempt yang memicu lazy reconciliation selama pengamatan expiry; verifikasi memakai query read-only.

- Deadline bersama: 03:13:10 WIB. Dua delivery cron nyata pada 03:14 dan 03:15 WIB menghasilkan `expired:100` lalu `expired:2`, masing-masing HTTP 200.
- Seluruh 102 attempt menjadi submitted dengan reason `deadline`; hash semua jawaban beku sesuai input tersimpan, tidak ada score terpublikasi, dan tepat 102 grading job.
- Satu job **milik dataset uji** diubah menjadi lease running kedaluwarsa untuk simulasi worker terputus, dengan audit event `staging.acceptance.lease_expired`. Ini injeksi state sintetis yang eksplisit, bukan crash worker nyata.
- Reconciliation terjadwal berikutnya memulihkan lease itu sekitar 41 detik kemudian, saat pending job lain masih ada. Audit `practicum.grade.claimed` ber-actor null mengonfirmasi jalur cron; lease dibersihkan dan job menjadi `needs_manual` tanpa publikasi nilai.
- Setelah bukti terkumpul, 79 tindakan grading pengajar eksplisit mengosongkan sisa backlog ke `needs_manual`. Ini bukan throughput cron otomatis. Seluruh 102 job akhirnya `needs_manual`, nol pending/running, enam activation closed dan session inactive; data/audit tidak dihapus.

Bukti berada di `E:/tmp/algohub-load-acceptance-jbFHaP/`: `scheduler-batch.json`, `scheduler-observation.json`, `lease-recovery-result.json`, dan `scheduler-cleanup.json`. Jangan mengulang script persiapan dataset; manifest sengaja menolak penciptaan batch kedua secara diam-diam.

Pengamatan dua batch membuktikan penanganan pekerjaan yang terlambat sampai invocation berikutnya. Tidak ada penghentian layanan scheduler atau perubahan jadwal/secret selama tes ini. Sejak 12 November 2026, `Reconcile` memproses hingga 20 job per invocation; test lokal mensimulasikan 120 job sintetis dan menyelesaikannya dalam enam invocation (p95 invocation 65–105 ms pada repository memori, variasi run lokal). Angka itu bukan bukti staging: load staging, provider grading nyata, dan operational outage drill belum dijalankan, sehingga gate kapasitas tetap terbuka.

Hasil aktual dan status acceptance dipelihara pada [update log](../updates/README.md).

Referensi provider: [scheduling](https://supabase.com/docs/guides/functions/schedule-functions), [pg_net](https://supabase.com/docs/guides/database/extensions/pg_net), [Vault](https://supabase.com/docs/guides/database/vault). Signature target terpasang diperiksa langsung; `http_post` pada target ini mempunyai default timeout 5.000 ms, sehingga job menetapkan 28.000 ms secara eksplisit.
