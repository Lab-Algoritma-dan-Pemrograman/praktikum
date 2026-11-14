# Algohub Rewrite

SvelteKit 3 / Svelte 5 and Go 1.26.10 monorepo. Phase 1 foundations and Phase 2 identity, SSO, authorization and admin identity screens are implemented. Phase 3 Practicum core, Phase 4 E-learning, and Phase 5 SIAKAD and inventory core (classes/rosters, schedules, token attendance, equipment rentals, scoped payment verification, reports and CSV export) are implemented. Final Phase 5 local verification completed on 2026-11-09: 75 frontend unit tests, 50 Chromium E2E tests, 90 operations / 342 live API response contracts, six-app checks/builds and Go/PostgreSQL/RLS/race tests passed. See [the verification report](docs/phase-5-siakad.md) for measured coverage and the Windows test-only race workaround. Real-provider and deployed staging acceptance are still pending.

## Workspace

- `apps/`: six application hosts; browser identity calls use same-origin proxies.
- `packages/ui/`, `packages/auth-client/`, `packages/practicum-client/`, `packages/elearning-client/`, `packages/siakad-client/`: shared presentation, identity/BFF and typed domain clients.
- `backend/`: Go API, serverless adapter, repositories and versioned identity/Practicum/E-learning/SIAKAD migrations.
- `docs/openapi/`: typed OpenAPI 3.1 source and generated JSON.
- `plan/`: frozen planning reference, not a live completion tracker.

## Checks

```sh
npm ci
npm run lint
npm run build
npm test
npm run api:vet
npm run api:build
npm run api:test:race
npm run api:audit
npx playwright install chromium
npm run test:e2e
```

The browser suite starts its own isolated fixture API and six local application hosts. PostgreSQL integration tests require an explicitly configured disposable local database; otherwise those tests are skipped. See [Phase 2 setup, contracts and database testing](docs/phase-2-identity.md).

## Local fixture development

Export the non-secret fixture template in each development terminal (Git Bash/POSIX):

```sh
set -a
. ./.env.example
set +a
go run ./backend/cmd/api
```

In other terminals, export the same variables and start each required app, for example:

```sh
npm run dev --workspace=@algohub/auth -- --host 127.0.0.1 --port 5174
npm run dev --workspace=@algohub/admin -- --host 127.0.0.1 --port 5175
npm run dev --workspace=@algohub/praktikum -- --host 127.0.0.1 --port 5178
```

Open `http://admin.localhost:5175`. Auth Center is `http://auth.localhost:5174`. Other fixture hosts and ports are listed in `.env.example`. The Go API provides `/healthz`, `/readyz` and `/docs`.

Real staging operation requires the fresh Supabase project, PostgreSQL migrations and an authorized initial coordinator. Local fixture mode is rejected outside loopback development. No production deployment or database change is performed by these commands.

Practicum routes require `ALGOHUB_PRACTICUM_ENABLED=true` (enabled by the local template; disabled by default). Local accounts `mahasiswa@example.test` and `koordinator@example.test` use `fixture-password`; they exist only in isolated fixture mode. Open `http://praktikum.localhost:5178` for the student flow or `/praktikum` on the admin host. Runner output and grading suggestions from the **fixture API** are explicitly simulated, never actual execution or automatic final grades. The separate browser preview executes real C/Python code in an isolated frame/worker; its output is never accepted as a Praktikum academic grade.

See [Phase 3 implementation and verification](docs/phase-3-practicum.md) and [Phase 4 E-learning implementation and verification](docs/phase-4-elearning.md) for state transitions, migration details, API/UI coverage and release blockers. E-learning routes require `ALGOHUB_ELEARNING_ENABLED=true` (enabled in `.env.example`; open `http://elearning.localhost:5177` for the student flow or `/elearning` on the admin host).

SIAKAD routes require `ALGOHUB_SIAKAD_ENABLED=true` (enabled in the local template; disabled by default). Open `http://siakad.localhost:5176` for classes, attendance and rentals, or `/siakad` on the admin host. Fixture accounts `sekretaris@example.test`, `k3@example.test`, `labops@example.test`, `bendahara@example.test`, and `peminjam@example.test` use `fixture-password` only in isolated local fixture mode. See [Phase 5 contract](docs/phase-5-contract.md) and [Phase 5 implementation and verification](docs/phase-5-siakad.md).

## Phase 1–5 closeout

Current work is limited to closing Phase 1–5; Phase 6 and later are not started. The dedicated staging schema (0002–0007), seven staging deployments, DNS/HTTPS, baseline identity/SSO, non-runner Practicum flows, retained text/quiz completion retests, and bounded scheduler/read-load acceptance have recorded evidence. SIAKAD's 284-assertion baseline is supplemented by direct SQL-role denials and 16 approved actor/pagination checks (101 inert inventory rows). Final backend PostgreSQL and standard race suites passed in disposable Linux test containers, without changing the serverless deployment architecture. The verified API/E-learning changes have been deployed to the existing staging projects; targeted E-learning acceptance is tracked separately. The final $0 runner decision uses C/WASM and Pyodide in the browser, with Glot as an optional non-authoritative preview fallback. Browser testcase success is intentionally trusted only for E-learning gamification XP; Praktikum academic grading remains manual. Judge0/RapidAPI is abandoned. Read the binding [browser-completion decision record](docs/elearning-browser-completion-decision-2026-11-12.md) and [closeout report](docs/phase-1-5-closeout.md); frozen `plan/` files and historical phase reports are not live completion trackers.

## Staging preparation

Start with the [manual staging and environment checklist (Bahasa Indonesia)](docs/staging-checklist.md): what to prepare, where to put `.env.staging.local`, which values to fill, and the read-only preflight required before any remote changes.

Use the [technical staging setup guide](docs/staging-setup.md) for per-project Vercel variables, migration safeguards, domains and acceptance gates. The secret-free [.env.staging.example](.env.staging.example) is a reference only. Actual credentials belong in the ignored [.env.staging.local](.env.staging.local) or the appropriate Vercel project's environment settings. Never use the local fixture template for staging, and do not paste secrets into chat or documentation.
