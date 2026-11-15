# Phase 4 — E-learning

> **Amendemen final 12 November 2026:** [keputusan browser-trusted](elearning-browser-completion-decision-2026-11-12.md) menggantikan server-proof hardening historis di bawah. XP code lesson adalah gamifikasi client-trusted; Judge0/RapidAPI dibatalkan; Praktikum tetap manual.

## Status and boundaries

Local E-learning core is connected to the existing identity, SSO, authorization, code-runner gateway and application factory. This report tracks implementation outside the frozen `plan/` directory. The original implementation contract is merged as an appendix at the end of this document.

**This is not production or full release sign-off.** The original local-phase evidence below is historical. By 2026-11-11, baseline staging identity/SSO and retained text-only/quiz-only completion retests had passed. Final PostgreSQL and standard race suites for the newer code-verification/achievement source then passed in disposable Linux test containers; the API and E-learning frontend were deployed to the existing Vercel staging projects. Targeted deployed acceptance is tracked separately, and the Glot server runner remains unconfigured. See [current Phase 1–5 closeout](phase-1-5-closeout.md#gate-yang-belum-ditutup) and the [staging scheduler runbook](staging-scheduler.md). Phase 6 remains held. No production, legacy, or frozen `plan/` changes are authorized by this report.

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

---

## Lampiran: kontrak implementasi asli

> **Amendemen final 12 November 2026:** [keputusan browser-trusted](elearning-browser-completion-decision-2026-11-12.md) menggantikan invariant server-proof di versi historis dokumen ini. Risiko client assertion diterima hanya untuk XP gamifikasi E-learning; Praktikum tetap manual.

User authorized local Phase 4 (E-learning) development following the Phase 3 verification pattern while real-provider/staging sign-off remains pending. No production, legacy, or frozen `plan/` changes. No Phase 5 (SIAKAD/inventory) implementation.

### Decisions and legacy evidence

Legacy `elearning/PRD.md`, `elearning/ERD.md`, `elearning/src/types.ts`, and `elearning/src/services/*.ts` define:
- Hierarchy: `Level -> Module -> Lesson`, ordered by `sort_order`.
- Level access modes: `auto` (unlocked when user XP meets `required_xp`), `unlocked`, or `locked`.
- Lesson content: rich explanation text, `code_example`, `starter_code`, private `solution`, optional `hint`, optional single multiple-choice `quiz` (`question, options, correct_option`), `test_cases` (`[{description, input, expected_output}]`), and `xp_reward` (default 60, bounded 10..500).
- Gamification formula: `level = floor(sqrt(xp / 50)) + 1`; daily `streak` tracked by `Asia/Jakarta` calendar date (`+07:00`); achievements unlocked by `xp`, `lesson_count`, `streak`, `game_score`, or `level_completed`.
- Bug Hunt minigame: C or Python code containing a buggy line (`bug_line`), student selects line number and writes explanation; bounded XP reward (`<= 300`) and weekly attempt limit (`default 3`).
- Single assessment engine rule (`plan/prd.md`): E-learning does not create a second exam/grading database; formal assessments remain on the Practicum engine.

### Invariants

- All mutations authenticate, check CSRF, permission/ownership, and resource scope before domain writes. Admin hostname is never an authorization bypass.
- Reading E-learning content requires `learning.content.read` covering the resource scope (granted by default to `mahasiswa`, `asisten`, and `koordinator` in `domain:elearning`).
- Authoring curriculum and Bug Hunt definitions requires `learning.content.manage` covering the resource scope. Monitoring student progress requires `learning.monitor` or `learning.content.manage` covering the scope.
- Public student responses for `Lesson` omit private `solution` and `quiz.correct_option`. Public student responses for `BugHuntQuestion` omit `bug_line` and `explanation` until evaluated in an attempt result. Monitoring rows omit student draft source code.
- Locked levels (`access_mode = 'locked'`, or `access_mode = 'auto'` when user `xp < required_xp`) reject direct module/lesson progress or hint mutations with `403 LEVEL_LOCKED` unless the caller holds `learning.content.manage` or `learning.monitor`.
- Lesson progress transitions `in_progress -> completed`. Draft saves check optimistic `revision`; stale `revision` returns `409 CONFLICT`.
- Lesson completion requires answering the lesson's quiz correctly (if defined). Lessons with starter code or test cases require an explicit browser testcase assertion bound by the client to the current lesson ID/version, language, and source snapshot. Missing assertion returns `409 CODE_NOT_VERIFIED`; stale lesson version returns `409 CONFLICT`. This assertion is deliberately client-trusted and forgeable, accepted only for gamification XP—not academic grading.
- Authored test cases execute in the isolated browser runtime with their own stdin. Output comparison normalizes CRLF and terminal newlines, preserves other whitespace, and compares equality (not substrings). A source or lesson-version change invalidates the ephemeral assertion and requires rerunning tests.
- **Exactly-once XP and achievement idempotency:** Completing a lesson awards `xp_reward` to the student's `LearningProfile` at most once per `(user_id, lesson_id)`. Replaying completion (same `Idempotency-Key` or subsequent requests on an already `completed` lesson) never increments XP a second time and never duplicates achievement unlocks.
- Solving a Bug Hunt challenge awards `xp_reward` at most once per `(user_id, question_id)`. Weekly Bug Hunt attempts per user are bounded (default 3 per 7-day rolling/calendar window in `Asia/Jakarta`).
- Code-lesson testcase execution uses bounded C/WASM or Pyodide in the browser. The generic Go `Runner` gateway, durable leases, and 30/minute quota remain compatibility/fixture infrastructure and have no progression authority. Contextual hints retain their 10 requests/minute per-user quota (`429` with `Retry-After: 60`) and never expose the private solution.
- Shared `audit_logs` record sanitized domain mutations in the same database transaction.
- RLS and privilege revocations deny direct `anon`/`authenticated` table access on all `public.learning_*` tables.

### API wire shapes

All paths below start with `/v1`. Existing `{data,meta:{request_id,operation_id}}` and error envelopes apply. Mutations require `Idempotency-Key` (1..128 safe characters) and CSRF.

#### Objects

- `LearningProfile`: `user_id, user_name, xp, level, next_level_xp, streak, last_active_date, completed_lessons, games_played, perfect_games, unlocked_achievements, timezone, timezone_offset`.
- `LearningLevel`: `id, title, description, sort_order, access_mode, required_xp, unlocked, completed_lessons, total_lessons, scope, version, modules?`.
- `LearningModule`: `id, level_id, title, description, sort_order, completed_lessons, total_lessons, version, lessons?`.
- `LearningLesson`: `id, module_id, level_id, title, sort_order, language, explanation, code_example, starter_code, solution?, hint, xp_reward, quiz?, test_cases, status?, version`; public version is required for browser-attestation invalidation.
- `LearningProgress`: `lesson_id, module_id, level_id, user_id, status, draft_code, language, quiz_option, test_passed, revision, xp_awarded, completed_at, updated_at`.
- `LearningLessonView`: `{lesson, progress, profile, newly_unlocked}`.
- `LearningRunResponse`: `{result: RunResult, test_results: [{description, passed, expected_output, actual_output}], all_passed: bool}`.
- `LearningHintResponse`: `{lesson_id, hint, remaining_quota, simulated}`.
- `LearningAchievement`: `id, title, description, requirement_type, requirement_value, unlocked, unlocked_at`.
- `LearningLeaderboardRow`: `rank, user_id, user_name, xp, level, streak, completed_lessons, achievement_count`.
- `LearningBugHuntQuestion`: `id, language, difficulty, title, buggy_code, bug_line?, explanation?, xp_reward, active, solved?, scope, version`.
- `LearningBugHuntAttempt`: `attempt_id, question_id, user_id, language, selected_line, explanation, correct, xp_earned, feedback, played_at`.
- `LearningBugHuntView`: `{questions, attempts, weekly_used, weekly_limit, profile}`.
- `LearningMonitoringRow`: `user_id, user_name, xp, level, streak, completed_lessons, in_progress_lessons, games_played, last_active_date, updated_at`.

#### Student routes

| Method | Path | Operation ID | Body / response data |
|---|---|---|---|
| GET | `/learning/levels` | `learning.levels.list` | `{profile, levels, runner}` |
| GET | `/learning/modules/{id}` | `learning.modules.get` | `LearningModule` with ordered lesson summaries and progress status |
| GET | `/learning/lessons/{id}` | `learning.lessons.get` | `LearningLessonView` (public lesson + owned progress + profile) |
| PUT | `/learning/lessons/{id}/progress` | `learning.lessons.progress` | `{revision, draft_code, language, quiz_option?, complete}` -> `LearningLessonView` |
| POST | `/learning/playground/run` | `learning.playground.run` | `{lesson_id?, source, language, version, stdin}` -> `LearningRunResponse` |
| POST | `/learning/hints` | `learning.hints.create` | `{lesson_id, draft_code?}` -> `LearningHintResponse` |
| GET | `/learning/leaderboard` | `learning.leaderboard.list` | `LearningLeaderboardRow[]` |
| GET | `/learning/achievements` | `learning.achievements.list` | `{profile, achievements: LearningAchievement[]}` |
| GET | `/learning/bug-hunt` | `learning.bughunt.list` | `LearningBugHuntView` |
| POST | `/learning/bug-hunt/attempts` | `learning.bughunt.attempt` | `{question_id, selected_line, explanation}` -> `{attempt: LearningBugHuntAttempt, profile: LearningProfile, newly_unlocked: LearningAchievement[]}` |

#### Admin routes

| Method | Path | Operation ID | Body / response data |
|---|---|---|---|
| GET | `/admin/elearning/kurikulum` | `learning.admin.curriculum.list` | `LearningLevel[]` (with modules and full lessons in scope) |
| POST | `/admin/elearning/levels` | `learning.admin.levels.create` | `{title, description, sort_order, access_mode, required_xp, scope}` -> `LearningLevel` (201) |
| PATCH | `/admin/elearning/levels/{id}` | `learning.admin.levels.update` | `{version, title, description, sort_order, access_mode, required_xp}` -> `LearningLevel` |
| POST | `/admin/elearning/modules` | `learning.admin.modules.create` | `{level_id, title, description, sort_order}` -> `LearningModule` (201) |
| PATCH | `/admin/elearning/modules/{id}` | `learning.admin.modules.update` | `{version, title, description, sort_order}` -> `LearningModule` |
| POST | `/admin/elearning/lessons` | `learning.admin.lessons.create` | Lesson input -> `LearningLesson` (201) |
| PATCH | `/admin/elearning/lessons/{id}` | `learning.admin.lessons.update` | Lesson input + `version` -> `LearningLesson` |
| GET | `/admin/elearning/bug-hunt` | `learning.admin.bughunt.list` | `LearningBugHuntQuestion[]` (with private `bug_line` and `explanation`) |
| POST | `/admin/elearning/bug-hunt` | `learning.admin.bughunt.create` | Bug Hunt question input -> `LearningBugHuntQuestion` (201) |
| PATCH | `/admin/elearning/bug-hunt/{id}` | `learning.admin.bughunt.update` | Bug Hunt question input + `version` -> `LearningBugHuntQuestion` |
| GET | `/admin/elearning/monitoring` | `learning.admin.monitoring` | `LearningMonitoringRow[]` |
