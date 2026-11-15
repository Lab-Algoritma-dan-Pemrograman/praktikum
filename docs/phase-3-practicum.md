# Phase 3 — Practicum core

## Status and boundaries

Local Practicum core is connected to the existing identity, SSO, authorization and application factory. This report tracks implementation outside the frozen `plan/` directory. The original implementation contract is merged as an appendix at the end of this document.

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

---

## Lampiran: kontrak implementasi asli

User authorized local Phase 3 development with subagents while Phase 2's real-provider/staging sign-off remains pending. No production, legacy, or frozen plan changes. No Phase 4/5 implementation.

### Decisions and legacy evidence

Legacy `praktikum/backend/internal/entity/enums.go` defines course kinds `pretest`, `posttest`, `keterampilan`, `ujian_praktik`; questions are `essay` or `coding`, with `easy`, `medium`, `hard` difficulty. Question points are summed, and percentage is derived from earned/max points. This implementation preserves raw points separately from percentage; it does not silently add participation points.

Legacy activation picks questions once per course/class/shift; regular timers start together on first start, while make-up participants can have individual timers. Target activations explicitly select question IDs and support `shared` or `individual` timers. An activation represents one course and one class/shift roster. Selected questions and grading rules are snapshotted on activation, so subsequent authoring cannot change an existing assessment.

Explicit activation rosters are the local eligibility boundary, not an assumption that every mahasiswa belongs to every class. They reference existing identity user IDs. Full SIAKAD membership/report workflows are outside Phase 3. Practical-exam participants additionally require explicit instructor entry approval. This is not a claim that the legacy Laporan Besar integration or legacy data migration is complete. Closing is terminal; a retake requires a new activation and never resets old deadlines or deletes old answers.

### Invariants

- All mutations authenticate, check CSRF, permission/ownership and resource scope before domain writes. Admin hostname is not authorization.
- Definition writes require permission covering the definition scope. Class-scoped assistants cannot edit shared cross-class definitions. Monitoring/closing/grading uses the full activation scope.
- Course kinds above and question types above are the only allowed values. Each session has at most one course per kind. Points and scores are bounded nonnegative integers; question points 1..100; selected questions 1..20.
- Activation scope is `domain:praktikum`, session ID, class ID, shift1/2, plus institution/division restrictions. It must be contained in the session scope and actor's grant. Roster and selected snapshots become immutable on activation.
- Start requires active mahasiswa assignment, roster membership, matching assignment scope, open activation, valid optional token, and exam entry approval. Autosave NEVER creates an attempt.
- Acquire locks in activation -> attempt order. Database time is read after locks. Shared deadline is first-start+duration, optionally bounded by activation closing time. Individual deadline is own start+duration with the same closing bound.
- Attempts transition `in_progress -> submitted -> graded`. Submission reason is `manual|deadline|closed`. No answer mutation after submission/deadline/closure. Submission snapshots answers atomically and enqueues grading once.
- One attempt per user/activation. Start retries return the existing attempt; no timer extension. Submit, close, autosave, grading and runner requests use scoped idempotency keys; key reuse with different input returns409. Replay returns current authoritative state, not a stale open attempt snapshot.
- Autosave checks whole-attempt `revision`; stale revision returns409 without writes. One successful batch increments revision once. Manual grades use separate `grade_revision` and a reason for audited changes.
- UTC persistence; every attempt response includes server_now and Asia/Jakarta/+07:00. Client countdown uses server time plus a monotonic clock, never device wall time for eligibility.
- Runner/grading provider calls occur outside database transactions with bounded timeouts and durable leases/CAS finalization. A manual grade supersedes any in-flight suggestion.
- Shared audit_logs receive sanitized actor/action/resource/scope/request/operation/time metadata in the same domain transaction. No source/answers/access tokens in logs.
- RLS and grants deny direct anonymous/authenticated access to sensitive Practicum tables; Go API is the supported access path.

### API wire shapes

All paths below start with `/v1`. Existing `{data,meta:{request_id,operation_id}}` and error envelopes apply. Lists accept `limit` (default25, max100) and `offset`. Domain IDs are opaque strings, user IDs remain numbers. Every mutation requires `Idempotency-Key` (1..128 safe characters) and CSRF, except the separately authenticated internal cron. Body max256KiB; individual answer/source max64KiB; total answer content max256KiB; prompt max8KiB; max20 selected questions.

#### Objects

- `Session`: `id,title,sequence,scope,active,version,created_at,updated_at`.
- `Course`: `id,session_id,title,kind,duration_minutes,token_required,version`. Duration1..180 minutes. Exam entry approval is enforced for ujian_praktik.
- `Question`: `id,course_id,type,difficulty,category,prompt,points,starter_code,language,reference_answer,rubric,version`. Private reference_answer and rubric only appear in authorized authoring/review. Rubric is `[{id,description,max_score}]` and sums to points. Categories may be empty or legacy module/flowchart values.
- `Activation`: `id,session_id,course_id,scope,timer_mode,is_open,started_at,deadline_at,closes_at,question_count,version`. Student summaries omit roster, token hash and private grading rules. Admin response additionally has `student_ids,entry_approved_ids,question_ids`; plain `access_token` appears only on creation if token_required.
- `SessionDetail`: `{session,courses,activations}`.
- `Attempt`: `attempt_id,activation_id,session_id,course_id,user_id,status,submission_reason,revision,grade_revision,grading_status,started_at,deadline_at,submitted_at,server_now,timezone,timezone_offset,questions,answers,score,max_score,percentage`.
- Public attempt question: `id,type,difficulty,category,prompt,points,starter_code,language`. No reference_answer/rubric/provider prompt.
- Answer: `question_id,value,language,updated_at`; review/published grade may add `score,feedback`.
- `grading_status`: `pending|running|needs_manual|graded`. score/percentage are null until published; max_score is always known.
- `AttemptReview`: `{attempt,questions,suggestions,grading_job}`; questions include private rules and suggestions are never automatically published.
- `MonitoringRow`: `attempt_id,user_id,user_name,activation_id,session_id,course_id,status,grading_status,answer_count,question_count,deadline_at,submitted_at`; never raw answers/source.
- `GradeSummary`: `attempt_id,session_id,session_title,course_title,course_kind,grading_status,score,max_score,percentage,submitted_at`.

#### Student routes

| Method | Path | Body / response data |
|---|---|---|
| GET | /practicum/sessions | Session[]; eligible roster entries only; non-student authenticated accounts get an empty list |
| GET | /practicum/sessions/{id} | SessionDetail within eligible roster scope |
| POST | /practicum/attempts | `{activation_id,access_token?}` -> Attempt |
| GET | /practicum/attempts/{id} | owned Attempt; lazily reconciles expiration |
| PATCH | /practicum/attempts/{id}/answers | `{revision,answers:[{question_id,value,language?}]}` -> Attempt |
| POST | /practicum/attempts/{id}/submit | `{}` -> Attempt |
| POST | /practicum/attempts/{id}/run | `{question_id,source,language,version,stdin}` -> RunResult |
| GET | /practicum/grades | owned GradeSummary[] |
| GET | /practicum/runner | `{languages:[{language,version,label}],simulated,available}` |

RunResult: `status,stdout,stderr,exit_code,duration_ms,runner_version,request_id,source_hash,simulated,truncated`. Status is `success|compile_error|runtime_error|timeout|quota_exceeded|provider_error`. No browser-to-provider requests and no host execution of submitted code.

#### Admin routes

| Method | Path | Body / response data |
|---|---|---|
| GET/POST | /admin/praktikum/sesi | list Session[] / `{title,sequence,scope}` -> Session |
| GET/PATCH | /admin/praktikum/sesi/{id} | SessionDetail / `{version,title,sequence,active}` -> Session |
| POST | /admin/praktikum/courses | `{session_id,title,kind,duration_minutes,token_required}` -> Course |
| PATCH | /admin/praktikum/courses/{id} | `{version,title,duration_minutes,token_required}` -> Course |
| GET/POST | /admin/praktikum/questions | course_id filter / Question input without id/version -> Question |
| PATCH | /admin/praktikum/questions/{id} | editable Question fields plus version -> Question |
| POST | /admin/praktikum/sesi/{id}/activate | `{course_id,scope,timer_mode,student_ids,entry_approved_ids,question_ids,closes_at?}` -> Activation |
| POST | /admin/praktikum/sesi/{id}/courses/{courseId}/close | `{activation_id}` -> Activation; finalize all active attempts in it atomically |
| GET | /admin/praktikum/monitoring | optional session_id/activation_id; scoped MonitoringRow[] |
| GET | /admin/praktikum/penilaian/{attemptId} | AttemptReview, requires practicum.grade.write |
| POST | /admin/praktikum/penilaian/bulk | `{attempt_ids}` -> job summaries; max25, atomic enqueue, no background goroutine |
| POST | /admin/praktikum/penilaian/{attemptId}/grade | `{}` -> AttemptReview; bounded processing of this one job |
| PUT | /admin/praktikum/penilaian/{attemptId}/manual | `{grade_revision,grades:[{question_id,score,feedback}],publish,reason}` -> AttemptReview |
| POST | /internal/practicum/reconcile | separate Bearer cron secret; bounded expiration and up to 20 grading jobs per call; no browser proxy |

No new identity role. Existing `practicum.session.manage`, `question.manage`, `practicum.grade.write` are explicit permissions. Coordinator receives the latter two through the forward Phase3 migration and matching fixture defaults, not a role bypass. Monitoring may use session.manage or grade.write; raw answer review requires grade.write. The existing identity access editor manages instructor grants.

### Runner and grading boundary

Local fixture adapters never execute code and are visibly marked simulated. Their outputs/suggestions cannot become final grades without manual review. Normal operation defaults to unconfigured providers. A provider adapter must be explicitly configured and sandbox-approved; remote network/filesystem guarantees require real-provider validation before release. AI grading is a suggestion adapter with bounded input/output and manual fallback, never sole authority. No student data is sent to a paid AI service during this task.

Grading jobs are persisted and claimed with leases. Submit enqueues once; the cron or explicit instructor action processes them. Processing cannot overwrite a newer manual grade. Poll/refetch is used for monitoring; no persistent serverless worker/WebSocket.

### Ownership and verification

- Core agent: new practicum entity/repository/usecase/HTTP files, forward0005 migration, domain/database/API tests.
- Runner agent: shared execution DTOs as agreed, isolated runner/grading provider packages and focused tests; no domain persistence or routing.
- Frontend agent: Practicum app student UI, admin Prakticum page, typed practicum client and UI/E2E tests; no identity BFF/global scripts edits without coordination.
- Parent: shared identity/BFF/config/factory integration, OpenAPI, fixtures permissions, CI/docs and aggregate verification.

All existing Phase2 tests must remain meaningful. Keep planning files immutable; track Phase3 parity, deviations, tests and blockers here or in the Phase3 implementation report.
