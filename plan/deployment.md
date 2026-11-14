# Deployment, Environment, dan CI/CD Plan

**Status:** Target baseline untuk Vercel Hobby + Supabase Free

## 1. Keputusan deployment

- Semua frontend SvelteKit dideploy ke Vercel.
- Go API dideploy sebagai Vercel Serverless Functions dan harus stateless.
- Tidak ada WebSocket persisten, goroutine sweeper, atau worker daemon di Vercel.
- Cron Vercel memanggil endpoint idempotent untuk auto-submit/reconciliation.
- Realtime memakai Supabase Realtime atau provider managed.
- Code runner memakai provider sandbox eksternal melalui Go API; browser tidak
  memanggil provider secara langsung.
- Satu Supabase project menyediakan Auth, Postgres, Storage, dan Realtime.

Keputusan ini menerima trade-off cold start, execution timeout, dan connection
pool. Semua batas Vercel/Supabase Free yang berlaku saat implementasi wajib diuji
di staging sebelum fitur ditetapkan sebagai production-ready.

## 2. Environment matrix

| Environment | Tujuan | Data | Domain |
|---|---|---|---|
| local | development cepat | Supabase local/mock | localhost |
| preview | review PR | data sintetis/seed | URL preview Vercel |
| staging | UAT, migration rehearsal | clone/snapshot non-production | `staging.*` |
| production | user nyata | production | domain Algohub |

Secret disimpan di Vercel Project Environment/secret manager. Tidak ada secret di
`.env` commit, browser bundle, log, atau planning.

## 3. Required CI pipeline

Urutan wajib pada setiap pull request:

1. secret scan dan dependency audit;
2. format/lint/typecheck frontend;
3. `go vet`, lint, unit test Go, race test package yang relevan;
4. migration lint dan schema drift check;
5. OpenAPI lint + generated contract check;
6. repository/integration test dengan database ephemeral;
7. security test minimum (auth boundary, IDOR/BOLA, XSS, CSRF, SQLi);
8. build semua app dan deploy preview;
9. Playwright smoke untuk login, routing, critical user flow;
10. upload artifact test report, coverage, logs, dan bundle summary.

Merge ke production memerlukan status checks hijau dan approval sesuai gate fase.

## 4. Observability

- request ID lintas frontend → API → database/provider;
- structured log tanpa token, password, source code sensitif, atau jawaban ujian;
- metric p50/p95/p99, 4xx/5xx/429, cold start, DB connections, storage, queue,
  runner latency, cron result, dan auth failures;
- alert untuk error spike, quota, storage nearing limit, failed cron, dan backup
  failure;
- setiap failure menyimpan correlation ID yang dapat dicari di dashboard.

## 5. Vercel-specific checks

- function timeout dan response size diuji dengan payload maksimum;
- cold start dan koneksi Postgres diuji pada concurrency realistis;
- tidak menyimpan state user/attempt di memory instance;
- cron aman terhadap retry dan duplicate invocation;
- preview tidak boleh memakai production service role atau data privat;
- domain, TLS, CORS, CSP, redirect, dan cache header diuji di deployment preview.
