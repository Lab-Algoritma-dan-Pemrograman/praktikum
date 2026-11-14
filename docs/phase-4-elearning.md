# Phase 4 — E-learning

> **Amendemen final 12 November 2026:** [keputusan browser-trusted](elearning-browser-completion-decision-2026-11-12.md) menggantikan server-proof hardening historis di bawah. XP code lesson adalah gamifikasi client-trusted; Judge0/RapidAPI dibatalkan; Praktikum tetap manual.

## Status and boundaries

Local E-learning core is connected to the existing identity, SSO, authorization, code-runner gateway and application factory. This report tracks implementation outside the frozen `plan/` directory. The Phase 4 implementation contract is in [phase-4-contract.md](phase-4-contract.md).

**This is not production or full release sign-off.** The original local-phase evidence below is historical. By 2026-11-11, baseline staging identity/SSO and retained text-only/quiz-only completion retests had passed. Final PostgreSQL and standard race suites for the newer code-verification/achievement source then passed in disposable Linux test containers; the API and E-learning frontend were deployed to the existing Vercel staging projects. Targeted deployed acceptance is tracked separately, and the Glot server runner remains unconfigured. See [current Phase 1–5 closeout](phase-1-5-closeout.md#gate-yang-belum-ditutup) and [staging evidence](staging-acceptance-2026-11-11.md). Phase 6 remains held. No production, legacy, or frozen `plan/` changes are authorized by this report.

### Completion browser-trusted — 12 November 2026

Code lesson menjalankan testcase melalui C/WASM atau Pyodide di browser. Hasil lulus dikirim sebagai assertion client yang terikat UI ke lesson/version/language/source; missing assertion menghasilkan `409 CODE_NOT_VERIFIED` dan stale version menghasilkan conflict. Risiko request palsu diterima secara eksplisit hanya untuk XP gamifikasi.

Output tetap dibandingkan exact setelah normalisasi CRLF dan terminal newline. Source/version change memaksa rerun. Completion, XP, streak, dan achievement tetap transactional serta idempotent. Generic server-run endpoint/lease dipertahankan untuk fixture/compatibility tetapi tidak memberi progression authority.

Frontend mempertahankan draft, conflict reconciliation, dan completion retry key. Completion code tidak lagi bergantung pada provider server. Praktikum tidak memakai trust model ini untuk nilai akademik.

## Implemented flows

- Student: curriculum overview (`Level -> Module -> Lesson`) with `unlocked | auto | locked` level access → interactive lesson workspace (explanation, code example, quiz, C/Python code editor with revision-controlled autosave, offline/reconnect draft reconciliation, runner execution with test-case evaluation, rate-limited contextual hints, and idempotent completion) → free C/Python Playground → Bug Hunt minigame with weekly quota and one-time XP per solved challenge → Achievement badges and XP Leaderboard.
- Instructor / Admin (`/elearning` on centralized `admin` host): scoped curriculum authoring (levels, modules, lessons, quizzes, test cases) with optimistic version checks → Bug Hunt challenge authoring → student learning progress and gamification monitoring → existing scoped audit log.
- Formal assessments remain unified on the Practicum engine (`plan/prd.md`: single assessment engine as source of grades).

### Routes

Student UI on the E-learning host (`elearning.algohub.web.id` / port 5177): `/`, `/lesson/[id]`, `/playground`, `/bug-hunt`, `/leaderboard`. Administration is on `/elearning` on the centralized admin host (`admin.algohub.web.id` / port 5175), with curriculum, Bug Hunt, and monitoring panels. There is no `/admin` route on the student E-learning host.

The OpenAPI 3.1 source documents 21 E-learning operations (`learning.*` and `learning.admin.*`) in addition to the 21 Phase 2 operations and 26 Phase 3 Practicum operations (total: 68 operations).

## Persistence and concurrency

Unshipped forward migration `0006_elearning_core.sql` adds nine aggregate tables: `learning_levels`, `learning_modules`, `learning_lessons`, `learning_progress`, `learning_profiles`, `learning_bug_hunt`, `learning_game_history`, `learning_operations`, and `learning_runner_leases`. It also grants `koordinator` default `learning.content.read` and `learning.content.manage` permissions on `domain:elearning`; matching local fixture defaults in `backend/internal/repository/identity.go` are updated.

Invariants enforced by usecases, PostgreSQL constraints, triggers, and RLS:
- `learning_progress` enforces `unique(parent_id, owner_id)` (one progress record per lesson and user). Immutability trigger `learning_freeze_progress` prevents decreasing `revision`, reopening a `completed` lesson, or mutating `completed_at` / `xp_awarded` once completed.
- `learning_profiles` enforces `unique(owner_id)` and trigger `learning_freeze_profile` prevents owner changes or decreasing `xp`, `completed_lessons`, or `games_played`.
- `learning_game_history` is append-only (`learning_freeze_game_history` rejects updates).
- Level formula: `level = floor(sqrt(xp / 50)) + 1`; daily `streak` uses `Asia/Jakarta` (`+07:00`) calendar dates derived from database UTC time.
- Completing a lesson awards `xp_reward` at most once per `(user_id, lesson_id)`. Replaying with the same `Idempotency-Key` or sending subsequent completion requests on an already completed lesson never grants duplicate XP or duplicate achievements.
- Solving a Bug Hunt challenge awards `xp_reward` (capped at `<= 300`) at most once per `(user_id, question_id)` and enforces a weekly play limit (3 attempts per 7-day rolling window).
- Repository transactions use `pg_advisory_xact_lock(731904006)` for conservative serialization of XP progression, idempotency, and runner/hint quota admission.
- RLS is enabled and table privileges are revoked from `public`, `anon`, and `authenticated` on all `public.learning_*` tables.

## Local parity evidence

| Feature ID | Local implementation / acceptance evidence | Remaining difference |
| --- | --- | --- |
| ELE-CURRICULUM | Level → Module → Lesson hierarchy, `unlocked`/`auto`/`locked` access modes, quiz and test cases; API/DB/E2E tests | Rich document (.docx/PDF) AI import deferred |
| ELE-PROGRESS-XP | Draft autosave with optimistic `revision`, offline/reconnect reconciliation, idempotent completion, `floor(sqrt(xp/50))+1` leveling, `Asia/Jakarta` streak | Offline drafts remain in tab memory before server sync |
| ELE-CODE-LAB | Lesson code runner with test-case verification, free C/Python Playground, 64 KiB source bound, durable concurrency leases and 30/min quota | Simulated fixture runner locally; real sandbox selection shared with Phase 3 |
| ELE-HINTS | Contextual lesson hints with 10/min per-user rate limit (`429` + `Retry-After: 60`) and solution non-disclosure | Uses bounded lesson hint adapter; external paid LLM not required |
| ELE-GAMIFICATION | 5 canonical achievements (`first-lesson`, `xp-100`, `streak-3`, `bug-hunter`, `level-1-complete`), Leaderboard, and Bug Hunt minigame | Custom per-user level override UI deferred |
| ELE-ADMIN-MONITOR | Scoped curriculum CRUD, Bug Hunt CRUD, student progress monitoring and transactional shared audit logs | No persistent WebSocket presence daemon on Vercel serverless |

## Runtime and security controls

`ALGOHUB_ELEARNING_ENABLED=true` explicitly enables the backend E-learning routes (default `false`; enabled in `.env.example`, `.env.staging.example`, and local test launchers).

The same-origin BFF (`packages/auth-client/src/server.server.ts`) allowlists `/v1/learning/*` and `/v1/admin/elearning/*`, enforces CSRF and `Idempotency-Key` on mutations, blocks `/v1/admin/*` from non-admin hosts, and keeps responses `private, no-store`. Private lesson `solution`, `quiz.correct_option`, and Bug Hunt `bug_line`/`explanation` are stripped from student responses and never written to `audit_logs`.

## Verification

| Check | Result |
| --- | --- |
| Formatting / six-app typecheck (`npm run lint`) | Passed; all six apps report 0 errors / 0 warnings |
| Frontend unit tests (`npm run test:frontend`) | 56 passed (26 identity/BFF + 26 Practicum + 4 E-learning client/workspace) |
| Go unit and PostgreSQL/RLS integration | Passed across all packages with disposable loopback PostgreSQL (`*_test`) enabled |
| Go race detector | Passed across all packages with PostgreSQL enabled (`go test -race -p 1 -count=1`) |
| Go cross-package statement coverage | 77.4% |
| OpenAPI and live response contracts (`npm run openapi:check`) | 68 operations / 285 live response contracts passed |
| Six frontend production builds and Go build/vet | Passed |
| Chromium E2E (`npm run test:e2e`) | 37 passed (including 4 new E-learning journeys at 320/768/1280 px, Practicum, SSO, and route guards) |
| Go vulnerability scan (`npm run api:audit`) | Passed: no vulnerabilities found |

## Files and ownership

- Database: `backend/migrations/0006_elearning_core.sql`
- Backend: `backend/internal/entity/learning.go`, `backend/internal/repository/learning*.go`, `backend/internal/usecase/learning*.go`, `backend/internal/delivery/http/learning_handlers.go`, `backend/internal/config/config.go`, `backend/internal/application/app.go`, `backend/internal/delivery/http/router.go`
- Browser & packages: `packages/auth-client/src/server.server.ts`, `packages/elearning-client/`, `apps/elearning/`, `apps/admin/src/routes/elearning/`
- Contract & E2E: `docs/openapi/openapi.yaml`, `docs/openapi/openapi.json`, `backend/internal/platform/openapi/spec.*`, `scripts/check-learning-contract.mjs`, `scripts/check-openapi.mjs`, `tests/frontend/e2e/elearning.spec.ts`
