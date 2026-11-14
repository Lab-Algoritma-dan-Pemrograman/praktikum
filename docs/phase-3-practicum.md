# Phase 3 — Practicum core

## Status and boundaries

Local Practicum core is connected to the existing identity, SSO, authorization and application factory. This report tracks implementation outside the frozen `plan/` directory. The original Phase 3 implementation contract is in [phase-3-contract.md](phase-3-contract.md).

**This is not production or full Phase 3 release sign-off.** Real Supabase/deployed-host acceptance from Phase 2 and real runner sandbox/provider validation remain pending. No production database, DNS, deployment, legacy source or Phase 4/5 features were changed.

## Implemented flows

- Student: eligible sessions → activation → start/resume → answer/code editor → revision-controlled autosave → refresh/reconnect → submit → published grade.
- Instructor: scoped session/course/question authoring → immutable activation roster/question snapshots → monitoring → terminal close/auto-submit → rubric review → manual grade publication/correction.
- Runner: authenticated Go gateway, language/version allowlist, bounded source/stdin/output, durable deduplication, per-user/activation quota and global/user concurrency leases.
- Grading: durable jobs and bounded suggestion adapter. A suggestion is never a published grade. An instructor must review and explicitly publish; unavailable providers fall back to manual grading.
- Existing scoped audit log receives transactional domain events. The admin audit link opens the existing permission-filtered log; there is no new per-session audit filter.

### Routes

Student UI: `/`, `/sesi/[id]`, `/attempts/[id]`, `/nilai` on the Practicum host. Administration remains on `/praktikum` on the centralized admin host, with authoring, activation, monitoring and grading panels. There is no admin route on the student host.

The OpenAPI source documents 26 Practicum operations in addition to the 21 existing Phase 2 operations. See [openapi.yaml](openapi/openapi.yaml) for exact operation IDs and request/response schemas. Operation prefixes are:

- `practicum.sessions.*`, `practicum.attempts.*`, `practicum.grades.list`, `practicum.runner.info`;
- `practicum.admin.sessions.*`, `practicum.admin.courses.*`, `practicum.admin.questions.*`, `practicum.admin.activations.*`, `practicum.admin.monitoring`, `practicum.admin.grades.*`;
- `practicum.internal.reconcile` — separate bearer credential, never available through the browser proxy.

## Persistence and concurrency

Unshipped migration `0005_practicum_core.sql` adds eight aggregate tables: `practicum_sessions`, `practicum_courses`, `practicum_questions`, `practicum_activations`, `practicum_attempts`, `practicum_grading_jobs`, `practicum_operations`, and `practicum_runner_leases`. It also explicitly grants coordinator question-management and grade-writing permissions; matching local fixture defaults are updated.

Answers, rosters and selected question/rubric snapshots are bounded JSON aggregates, not separate relational answer/roster tables. Constraints, foreign keys, immutable-state triggers, RLS/browser privilege denial and indexes enforce course-kind uniqueness, one attempt per user/activation, one grading job per attempt, immutable timers/snapshots and terminal attempt states. Explicit local rosters reference identity users; full academic membership integration and legacy data migration are deferred.

State transitions:

```text
in_progress --manual/deadline/closed--> submitted --manual publication--> graded
```

- Deadline and closed are submission reasons, not mutable client statuses.
- Start retries return the existing attempt without extending the deadline.
- Database time is read after locks; timestamps are UTC and responses identify `Asia/Jakarta` / `+07:00`.
- Client countdown uses server time and a monotonic clock. Device wall-clock changes do not extend eligibility.
- Autosave requires whole-attempt revision and an idempotency key. Reusing a key with different input or sending an older revision conflicts; successful retries do not increment revision again.
- Submit, deadline reconciliation and close freeze answers and enqueue grading once in a transaction. Late answers are rejected.
- Manual grading has a separate revision and audit reason. Durable provider leases and finalization checks prevent expired work or older suggestions from overwriting newer manual grading.
- Repository transactions use activation → attempt ordering plus a **domain-wide PostgreSQL advisory lock**. This is conservative serialization, not proof of production throughput. Load acceptance must measure contention before release.

The migration was corrected during isolated pre-release testing. Do not edit a previously applied migration in an external environment: a deployed copy would require a new forward migration. No application database was migrated in this task. Tests use fresh disposable loopback databases and preserve prior migration checksums rather than rewriting history.

## Local parity evidence

Legacy references were read-only: `praktikum/README.md` (student/admin flows and raw-point scoring) and `praktikum/backend/internal/entity/enums.go` (course kinds, essay/coding types and difficulty). This is core-flow parity, not a claim that every legacy feature has been ported.

| Feature ID | Local implementation / acceptance evidence | Remaining difference |
| --- | --- | --- |
| PRA-SESSION-ACTIVATION | Four course kinds, token-capable class/shift activation, selected immutable questions and explicit eligibility roster; API/DB/browser tests | No full academic membership/import workflow |
| PRA-ATTEMPT-TIMER | Shared/individual server deadline, UTC storage and Jakarta display; resume, clock and deadline tests | Deployed clock/scheduler acceptance pending |
| PRA-AUTOSAVE-SUBMIT | Versioned answers, idempotent retry, submit/close/expiry races and frozen submission | Offline unsaved drafts remain in tab memory, not durable browser storage |
| PRA-GRADING | Raw points and derived percentage, rubric suggestions and audited manual publication; provider/manual race tests | Real AI adapter deliberately unconfigured; no automatic final grade |
| PRA-CODE-RUNNER | Authenticated gateway, bounded validation, durable dedup/quota and visibly simulated results | Real compiler/sandbox validation pending; editor is a plain code textarea |
| PRA-ADMIN-MONITOR | Permission-scoped configuration, activation, close, monitoring, review and existing audit log | No live presence registry, export/import or destructive legacy resets |

Legacy random selection rules, rich-text/flowchart attachments, participation/report integration and broad non-core administration are not silently claimed as implemented. Explicit selection and entry approval are the Phase 3 contract described in the implementation brief.

## Runtime and security controls

`ALGOHUB_PRACTICUM_ENABLED=true` explicitly enables the backend routes. It defaults to false. The local environment template and browser test launcher enable it. Turning it off disables Practicum endpoints; it does not erase attempts or reset deadlines.

Normal database-backed operation uses the existing bounded pgx pool. Fixture storage and simulated providers require the existing isolated local configuration; they cannot silently replace an unavailable production database.

The same-origin BFF now allowlists concrete Practicum paths and forwards `Idempotency-Key`, CSRF, host-bound cookies and request IDs. It keeps the existing 64 KiB identity body cap while allowing 256 KiB domain bodies, with bounded responses and a 30-second domain proxy timeout. The default Go request deadline is 25 seconds, leaving room around bounded provider calls. Only the successful activation-creation route can return its one-time classroom token; OAuth/session credentials remain filtered. A retried creation returns the existing activation but deliberately does not reconstruct the plain token. If the initial response/token is lost, close that activation and create a new activation with a new key; do not reset existing attempts or deadlines.

Private responses remain `no-store`. Runner calls and persisted results retain the originating HTTP request ID; a cached replay keeps that original runner correlation while its HTTP envelope has the new request's ID. Students receive only their eligible resources and owned attempts/grades, without private rubric/reference answers. Admin hostname is never an authorization grant. Code output is escaped text. No source is executed in the API process.

### Reconciliation

Set `ALGOHUB_PRACTICUM_CRON_SECRET` only in the API environment to a separate random 32–256-character secret. Empty configuration disables cron authentication. An authorized scheduler invokes `POST /v1/internal/practicum/reconcile` with that bearer credential. Each call performs bounded work (at most 100 expired attempts and 20 grading jobs) and is safe to repeat. This task did not configure or deploy a scheduler; a production scheduler must support POST and its cadence/throughput must be validated. Browser resume and domain actions also reconcile expired attempts lazily.

### Local demo

Use the root README environment setup. Start auth on 5174, admin on 5175, Practicum on 5178 and the API on 8080.

- Student: `mahasiswa@example.test`.
- Coordinator: `koordinator@example.test`.
- Local fixture password: `fixture-password`.
- Seed: `session-1`, `course-pretest`, `activation-1`, with one essay and one coding question.

These are synthetic fixture accounts only. Runner and grading UI clearly identify simulated outputs. No provider credential is required or embedded in a browser bundle.

## Verification

Final local aggregate results (2026-11-06) are recorded below. PostgreSQL integration was enabled using a fresh disposable loopback database; those tests were run, not skipped. The temporary PostgreSQL server was stopped after verification. Subsequent integration runs require `ALGOHUB_TEST_DATABASE_URL` targeting an isolated loopback database whose name ends in `_test`.

| Check | Result |
| --- | --- |
| Formatting / six-app typecheck | Passed; all six apps report 0 errors / 0 warnings |
| Frontend unit tests | 51 passed (25 identity/BFF + 26 Practicum client/workspace) |
| Go unit and PostgreSQL/RLS integration | Passed across all packages; PostgreSQL tests enabled |
| Go race detector | Passed across all packages with PostgreSQL enabled; serialized packages and documented Windows PATH workaround |
| Go cross-package statement coverage | 77.5% |
| OpenAPI and live response contracts | 47 operations / 190 cases passed in the final aggregate run |
| Frontend and Go builds / Go vet | Passed; all six frontend production builds and final Go build/vet |
| Chromium student/admin/SSO/responsive E2E | 33 passed, including the 7 Practicum journeys and existing Phase 2 regressions |
| Go vulnerability scan | Passed: no vulnerabilities found |
| npm dependency audit | No high/critical; 3 low and 4 moderate transitive advisories remain |
| Secret-pattern scan | No matching private keys or provider-key patterns found |

Local artifacts: `playwright-report/` contains the final 33-test browser report; `coverage/frontend-checks.log` records formatting, typechecks, builds and browser results. `coverage/backend-checks.log` contains the passing database, contract, race, vet and build output; `coverage/go.out` and `coverage/go-summary.txt` retain Go coverage. `X-Request-ID: integration-test` is a synthetic HTTP-test correlation example, not a production request. No deployment SHA is available because this workspace is not a Git checkout and nothing was deployed.

Commands executed:

```sh
npm run lint
npm run build
npm run test:e2e
# Export ALGOHUB_TEST_DATABASE_URL for a fresh disposable loopback *_test DB.
npm test
PATH="/c/msys64/mingw64/bin:$PATH" go test -race -p 1 -count=1 -timeout=180s \
  -coverpkg=./backend/... -coverprofile=coverage/go.out ./backend/...
go tool cover -func=coverage/go.out
npm run api:vet
npm run api:build
npm audit --audit-level=high
npm run api:audit
```

Initial failures were resolved rather than ignored: missing student routing and SvelteKit imports, frontend formatting, outdated seed assertions, and the documented Windows race-runtime DLL issue. Failing database/security regressions drove the migration and lease fixes. Initial browser-test fixture assumptions and response-loss injection were corrected; final full-suite output is clean. No tests were removed to obtain a pass.

The npm advisories concern `deepmerge`, `glob`, and `minimatch` in existing build-tool dependencies. Forced remediation suggests breaking framework/adapter downgrades and was not applied.

Test coverage includes ownership/scope denial, CSRF and spoofed fields, revision/idempotency conflicts, frozen private snapshots, submit/close/deadline races, lease concurrency/quota, manual/provider grading races, database rollback, RLS and malformed JSON/schema constraints. Browser tests use synthetic data and disable traces/videos/password screenshots; artifacts must not retain cookies, tokens, real student answers or source.

## Implementation changelog — 2026-11-06

- Connected the previously separate Practicum backend, fixture providers, BFF and student/admin screens; added an explicit release switch and cron credential validation.
- Repaired missing student routing and SvelteKit 3 imports, autosave/reconnect handling, scoped activation controls and explicit runner-retry keys.
- Added typed OpenAPI operations and live contract checks, client/BFF tests, HTTP lifecycle tests, database/RLS/state tests and responsive browser journeys.
- Hardened the unshipped migration and grading finalization based on failing regression tests rather than weakening the checks.
- Updated setup/CI/implementation documentation only; frozen planning documents and legacy source remain unchanged.

## Files and ownership

- Runtime: `backend/internal/application/`, `backend/internal/config/`, `backend/internal/delivery/http/router.go`, identity fixture permission defaults.
- Domain: `backend/internal/entity/practicum.go`, `backend/internal/usecase/practicum*.go`, `backend/internal/repository/practicum*.go`, runner/grading platform adapters, migration 0005 and focused tests.
- Browser: `packages/auth-client/src/server.server.ts` and proxy tests; `packages/practicum-client/`; `apps/praktikum/`; `apps/admin/src/routes/praktikum/`.
- Contract: `docs/openapi/`, generated backend `platform/openapi/spec.*`, and the live contract checker scripts.
- Quality/setup: root npm scripts, `.env.example`, `.gitignore` for browser results, CI workflow, local browser fixture launcher, Practicum E2E tests and README.

## Accepted scope clarification — 2026-11-06

The project owner accepted manual grading for Phase 3, with real AI grading deferred. A real AI provider is therefore **not a Phase 3 acceptance blocker**; the existing manual review and publication flow must still pass staging acceptance. The application remains serverless on Vercel. A managed sandbox for untrusted student code is a separate, still-unselected integration, not a requirement to maintain an always-on application server. Staging hostname preparation is approved; the owner will add DNS records. See [staging-setup.md](staging-setup.md) for the private environment file and per-project configuration.

## Release blockers and Phase 4 prerequisites

1. Complete Phase 2 real Supabase login and multi-host staging acceptance (OAuth only if selected).
2. Select and explicitly approve a real runner deployment; prove network isolation, ephemeral filesystem, language/compiler versions, CPU/memory/output limits and failure behavior. Fake malicious-source tests cannot prove a real sandbox cannot escape.
3. Deploy and monitor the bounded reconciliation scheduler; verify missed runs, lease recovery, timezone/deadline behavior and enough capacity for the cohort.
4. Measure PostgreSQL advisory-lock contention, pool capacity and normal API p95 under realistic simultaneous students. Current concurrency tests are functional smoke tests, not a staging performance certification.
5. Review roster limits/academic integration, aggregate-query pagination, retention of attempts/runner results/audit records and backup/restore procedures before cutover.
6. Finish full browser/accessibility/performance acceptance beyond local Chromium checks, review transitive dependency advisories and obtain release sign-off.

Phase 4 was not started. A local passing test suite does not replace these release gates.
