# Phase 4 implementation contract

> **Amendemen final 12 November 2026:** [keputusan browser-trusted](elearning-browser-completion-decision-2026-11-12.md) menggantikan invariant server-proof di versi historis dokumen ini. Risiko client assertion diterima hanya untuk XP gamifikasi E-learning; Praktikum tetap manual.

User authorized local Phase 4 (E-learning) development following the Phase 3 verification pattern while real-provider/staging sign-off remains pending. No production, legacy, or frozen `plan/` changes. No Phase 5 (SIAKAD/inventory) implementation.

## Decisions and legacy evidence

Legacy `elearning/PRD.md`, `elearning/ERD.md`, `elearning/src/types.ts`, and `elearning/src/services/*.ts` define:
- Hierarchy: `Level -> Module -> Lesson`, ordered by `sort_order`.
- Level access modes: `auto` (unlocked when user XP meets `required_xp`), `unlocked`, or `locked`.
- Lesson content: rich explanation text, `code_example`, `starter_code`, private `solution`, optional `hint`, optional single multiple-choice `quiz` (`question, options, correct_option`), `test_cases` (`[{description, input, expected_output}]`), and `xp_reward` (default 60, bounded 10..500).
- Gamification formula: `level = floor(sqrt(xp / 50)) + 1`; daily `streak` tracked by `Asia/Jakarta` calendar date (`+07:00`); achievements unlocked by `xp`, `lesson_count`, `streak`, `game_score`, or `level_completed`.
- Bug Hunt minigame: C or Python code containing a buggy line (`bug_line`), student selects line number and writes explanation; bounded XP reward (`<= 300`) and weekly attempt limit (`default 3`).
- Single assessment engine rule (`plan/prd.md`): E-learning does not create a second exam/grading database; formal assessments remain on the Practicum engine.

## Invariants

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

## API wire shapes

All paths below start with `/v1`. Existing `{data,meta:{request_id,operation_id}}` and error envelopes apply. Mutations require `Idempotency-Key` (1..128 safe characters) and CSRF.

### Objects

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

### Student routes

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

### Admin routes

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
