# Phase 3 implementation contract

User authorized local Phase 3 development with subagents while Phase 2's real-provider/staging sign-off remains pending. No production, legacy, or frozen plan changes. No Phase 4/5 implementation.

## Decisions and legacy evidence

Legacy `praktikum/backend/internal/entity/enums.go` defines course kinds `pretest`, `posttest`, `keterampilan`, `ujian_praktik`; questions are `essay` or `coding`, with `easy`, `medium`, `hard` difficulty. Question points are summed, and percentage is derived from earned/max points. This implementation preserves raw points separately from percentage; it does not silently add participation points.

Legacy activation picks questions once per course/class/shift; regular timers start together on first start, while make-up participants can have individual timers. Target activations explicitly select question IDs and support `shared` or `individual` timers. An activation represents one course and one class/shift roster. Selected questions and grading rules are snapshotted on activation, so subsequent authoring cannot change an existing assessment.

Explicit activation rosters are the local eligibility boundary, not an assumption that every mahasiswa belongs to every class. They reference existing identity user IDs. Full SIAKAD membership/report workflows are outside Phase 3. Practical-exam participants additionally require explicit instructor entry approval. This is not a claim that the legacy Laporan Besar integration or legacy data migration is complete. Closing is terminal; a retake requires a new activation and never resets old deadlines or deletes old answers.

## Invariants

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

## API wire shapes

All paths below start with `/v1`. Existing `{data,meta:{request_id,operation_id}}` and error envelopes apply. Lists accept `limit` (default25, max100) and `offset`. Domain IDs are opaque strings, user IDs remain numbers. Every mutation requires `Idempotency-Key` (1..128 safe characters) and CSRF, except the separately authenticated internal cron. Body max256KiB; individual answer/source max64KiB; total answer content max256KiB; prompt max8KiB; max20 selected questions.

### Objects

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

### Student routes

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

### Admin routes

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

## Runner and grading boundary

Local fixture adapters never execute code and are visibly marked simulated. Their outputs/suggestions cannot become final grades without manual review. Normal operation defaults to unconfigured providers. A provider adapter must be explicitly configured and sandbox-approved; remote network/filesystem guarantees require real-provider validation before release. AI grading is a suggestion adapter with bounded input/output and manual fallback, never sole authority. No student data is sent to a paid AI service during this task.

Grading jobs are persisted and claimed with leases. Submit enqueues once; the cron or explicit instructor action processes them. Processing cannot overwrite a newer manual grade. Poll/refetch is used for monitoring; no persistent serverless worker/WebSocket.

## Ownership and verification

- Core agent: new practicum entity/repository/usecase/HTTP files, forward0005 migration, domain/database/API tests.
- Runner agent: shared execution DTOs as agreed, isolated runner/grading provider packages and focused tests; no domain persistence or routing.
- Frontend agent: Practicum app student UI, admin Prakticum page, typed practicum client and UI/E2E tests; no identity BFF/global scripts edits without coordination.
- Parent: shared identity/BFF/config/factory integration, OpenAPI, fixtures permissions, CI/docs and aggregate verification.

All existing Phase2 tests must remain meaningful. Keep planning files immutable; track Phase3 parity, deviations, tests and blockers here or in the Phase3 implementation report.
