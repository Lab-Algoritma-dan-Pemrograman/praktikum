# Staging setup — Phase 2–5 acceptance

For manual preparation, start with the [Indonesian staging and environment checklist](staging-checklist.md). This document is the technical reference for environment placement, migration safeguards, domains and remaining acceptance gates. Preparing documentation or filling the local file does not authorize remote changes.

## 1. Where to put values now

Use the private staging file at the workspace root: `.env.staging.local`. Fill these three values manually using the **new staging Supabase project**, not a legacy or production project. Do not overwrite an existing populated file with the template:

```dotenv
SUPABASE_URL='https://<project-ref>.supabase.co'
SUPABASE_ANON_KEY='<legacy anon/public JWT key>'
ALGOHUB_DATABASE_URL='<PostgreSQL transaction-pooler URI using TLS>'
```

- Get the project URL and anon/public key from the Supabase project's API settings/API keys area. The current Go adapter sends the anon key in both `apikey` and bearer headers; use the compatible legacy anon/public JWT key, **not** `service_role`, a secret key, or the database password.
- Get the database connection from the **Connect** panel. Use the transaction pooler for the serverless API. Include `sslmode=require` and URL-encode special characters in the password component. Do not paste an unescaped password into a URI.
- Use single quotes around local dotenv values. Do not paste credentials into chat, screenshots, commits, or documentation.
- `.env.staging.local` is covered by `.gitignore`. `.env.staging.example` is the shareable, secret-free template. Never put credentials in either example file.
- Do not change `.env.example` into a staging file: it intentionally enables isolated local fixtures.
- Do not put `ALGOHUB_TEST_DATABASE_URL` into staging. That variable is for disposable loopback integration-test databases only.

This file is a local configuration handoff, **not automatic runtime wiring**. Go does not load dotenv files by itself, and Vercel does not deploy this ignored file. Environment variables must be explicitly provided to the process or configured in the relevant Vercel project. On 2026-11-10 the owner confirmed this fresh rewrite target and explicitly approved initial schema setup. Migrations 0002–0007 were applied and their checksums/RLS metadata verified read-only; no legacy data or fixture users were imported. See [Phase 1–5 closeout](phase-1-5-closeout.md) for current evidence and remaining gates.

## 2. Where values go on Vercel

Use **Project → Settings → Environment Variables** for each staging deployment. Scope values to the staging branch/environment. If dedicated staging-only projects use Vercel's Production environment slot for their stable domains, those are still separate staging projects—never copy staging credentials into the live application's projects.

| Destination                                              | Variables                                                                                                                                                                                                                                                                                                |
| -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Every staging project: six SvelteKit apps and the Go API | `ALGOHUB_ENV=staging`, `ALGOHUB_ENABLE_FIXTURES=false`, `ALGOHUB_ALLOWED_ORIGINS`, `ALGOHUB_SSO_ALLOWED_ORIGINS`                                                                                                                                                                                         |
| Six SvelteKit app servers                                | `ALGOHUB_API_BASE_URL`, `ALGOHUB_AUTH_ORIGIN`                                                                                                                                                                                                                                                            |
| Go API only                                              | `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `ALGOHUB_DATABASE_URL`, `ALGOHUB_COOKIE_SECURE`, `ALGOHUB_SESSION_TTL`, `ALGOHUB_REQUEST_TIMEOUT`, `ALGOHUB_AUTH_RATE_LIMIT`, `ALGOHUB_USER_RATE_LIMIT`, `ALGOHUB_DB_MAX_CONNS`, `ALGOHUB_PRACTICUM_ENABLED`, `ALGOHUB_ELEARNING_ENABLED`, `ALGOHUB_SIAKAD_ENABLED` |
| Go API only, after scheduler setup                       | `ALGOHUB_PRACTICUM_CRON_SECRET`                                                                                                                                                                                                                                                                          |
| Auth Center only, if OAuth is enabled later              | `SUPABASE_URL`, `SUPABASE_SSO_PROVIDER`                                                                                                                                                                                                                                                                  |

Do **not** import the whole private environment file into each frontend project. Database and cron credentials belong only to the Go API. No `PUBLIC_*` variable should contain a database URI, private key, session secret or provider credential. Redeploy the relevant staging project after changing its environment configuration.

The API and migrator use different connection modes to the **same fresh database**. `ALGOHUB_DATABASE_URL` is the verified transaction-pooler URI for runtime. `ALGOHUB_MIGRATION_DATABASE_URL` retains the direct/session-pooler URI locally; never deploy that variable to the API or frontends. The migration command prefers the migration-specific variable, falls back to the original runtime variable for backward compatibility, and rejects Supavisor transaction-pooler port 6543. Migrations `0002` through `0007` were applied with owner approval and checksums verified on 2026-11-10. Migrations 0005 and 0007 were changed during local development; a different target with older applied checksums requires a forward migration rather than checksum edits. See [Phase 1–5 closeout](phase-1-5-closeout.md).

## 3. Proposed staging hostnames

The public landing uses the apex staging host. The old `portal.staging` hostname is not a product host and must not be added to new origin allowlists. `portal.localhost` remains only as an isolated local harness alias.

| Application       | Host                               |
| ----------------- | ---------------------------------- |
| Portal landing    | `staging.algohub.web.id`           |
| Auth Center       | `auth.staging.algohub.web.id`      |
| Centralized admin | `admin.staging.algohub.web.id`     |
| Practicum         | `praktikum.staging.algohub.web.id` |
| E-learning        | `elearning.staging.algohub.web.id` |
| SIAKAD            | `siakad.staging.algohub.web.id`    |
| Shared Go API     | `api.staging.algohub.web.id`       |

The project owner approved staging operations in the dedicated rewrite Vercel account, `lab-algoritma-pemrograman` (Hobby), on 2026-11-10. Seven isolated `algohub-v2-staging-*` projects are configured for `sin1`, bounded 30-second functions, and responsibility-scoped encrypted environments. Their custom staging domains passed DNS/ownership/HTTPS verification. See [staging-dns.md](staging-dns.md#4-hasil-verifikasi--10-november-2026-2338-wib) for recorded evidence, not instructions to reapply records. The previous personal Vercel account is not the deployment target; production DNS/projects remain outside this scope.

## 4. Browser runner and optional preview fallback

Vercel hosts all six SvelteKit apps and the stateless Go API. Supabase provides identity and PostgreSQL. This remains a serverless application architecture.

The final approved $0 design runs C through browser WASM and Python through Pyodide. E-learning browser testcase success is intentionally client-trusted for gamification XP under the documented accepted risk. It is not academic evidence. Praktikum answers remain submission-based and manually graded.

Glot `/api/mux` may be configured only as an optional non-authoritative preview fallback; its result has `authoritative:false`, creates no proof, and never awards XP or grades. Judge0/RapidAPI is abandoned. Do not add `RUNNER_*`, Judge0, or RapidAPI secrets. See the [binding decision record](elearning-browser-completion-decision-2026-11-12.md) and [closeout](phase-1-5-closeout.md).

## 5. Accepted grading scope and scheduler

On 2026-11-06, the project owner accepted **manual grading for Phase 3**, with real AI grading deferred. Manual review/publication is implemented; no AI or execution-provider key is needed. Browser execution supports practice, while academic publication remains manual.

Scheduled reconciliation is configured through Supabase `pg_cron`/`pg_net` with a dedicated Vault/API secret, one-minute cadence and 28-second HTTP timeout. The authenticated endpoint remains `POST /v1/internal/practicum/reconcile`. Delivery, the historical 102-attempt batch, and the newer six-invocation × 20 acceptance for 120 attempts have recorded evidence, including synthetic expired-lease recovery. Follow [the scheduler runbook](staging-scheduler.md); do not reinstall or rotate its secret as part of ordinary verification.

## 6. Next steps after the three values are filled

1. Confirm the file is populated without sharing its contents; validate configuration without printing credentials.
2. Confirm the Supabase project is the intended fresh staging target and inspect migration history before requesting execution approval.
3. Configure staging projects, obtain DNS targets, and complete HTTPS.
4. Apply approved migrations and bootstrap the designated coordinator; create authorized synthetic test users/scopes, not production imports.
5. Verify real Supabase login, five-application SSO, revocation, RLS, authorization, Practicum persistence/manual grading, E-learning progression, and SIAKAD attendance, stock/rental concurrency and scoped finance/CSV.
6. Verify browser C/Python completion, scheduler/manual grading, and staging concurrency/load/UI acceptance, then update the Phase 2–5 reports.

Existing staging has 30 synthetic identities and a configured coordinator. See [the account guide](staging-accounts.md); do not repeat account provisioning or request a new initial coordinator for this environment. Email/password login is the accepted baseline; OAuth can be added if separately selected. Staging permission does not authorize changes to production.
