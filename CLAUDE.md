# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Web Lab-AP v3 — a platform for managing Algorithm & Programming laboratory practicum sessions: secure exams (pre-test essay, post-test hybrid, live-coding practical), live code execution, LLM-assisted bulk grading, and admin dashboards. Comments and domain terms are in Indonesian; keep that convention when editing.

## Commands

All `make` targets run from the repo root (backend lives in `./backend`).

- `make run` — start backend (`go run ./cmd/server`), serves on `:8080`
- `make build` — compile backend to `backend/bin/server`
- `make test` — `go test -v ./...` (backend). Single test: `cd backend && go test ./internal/usecase -run TestName`
- `make migrate-up` / `make migrate-down` — apply / rollback one DB migration; `make migrate-drop` rolls back all
- `make seed` — seed admin, kelas, jadwal, sample soal
- `make swag` — regenerate OpenAPI docs (required after changing HTTP handler annotations; UI at `/swagger/index.html`)
- `make mock` — regenerate mockery mocks into `internal/repository/mocks`
- `make tidy` — `go mod tidy`
- `make fe-install` / `make fe-dev` / `make fe-build` — frontend (`:5173`)
- Frontend checks: `cd frontend && bun run check` (svelte-check), `bun run test:unit` (vitest), `bun run test:e2e` (playwright)

Frontend memakai **Bun**, bukan npm (`bun install`, `bun run …`). Repo ini belum punya workflow CI; jalankan `go build ./... && go test ./...` untuk backend dan `bun run check && bun run build` untuk frontend sebelum push.

`make migrate-sync` / `make migrate-fresh` run the Node scripts in `updateAndPRDERD/migration/` — **`migrate-fresh` is destructive** (wipes Supabase except admin `202411106` and re-imports from Firebase). Do not run without explicit user intent.

## Backend architecture (`backend/`)

Clean architecture, Go + Gin + GORM + PostgreSQL. Layers (request flows top-down):

`route` → `handler` → `usecase` → `repository` → GORM/Postgres, with `entity` (DB models), `dto` (request/response shapes), and `pkg/` (infra clients).

- **Composition root: `internal/app/app.go`.** `Build(cfg)` wires every repository → usecase → handler → router. This is the single place dependencies are assembled; both entrypoints reuse it. When adding a feature, register the new repo/usecase/handler here.
- **Two entrypoints share the engine:**
  - `cmd/server/main.go` — persistent local server. Runs the **auto-submit sweeper goroutine** (`JawabanUsecase.AutoSubmitExpired`, interval from `SWEEPER_INTERVAL_SECONDS`) and graceful shutdown.
  - `api/index.go` — Vercel serverless entrypoint. No goroutines; auto-submit is instead driven by `POST /api/cron/auto-submit` + an external cron (cron-job.org), guarded by `CronSecret`.
- **Other `cmd/`:** `cmd/migrate` (custom SQL migrator below), `cmd/regrade` (offline bulk AI re-grading, worker pool, resumable — see file header for flags). `database/seed` seeds initial data.
- **Migrations: custom, not golang-migrate.** `cmd/migrate/main.go` reads `database/migration/NNN_*.up.sql` / `.down.sql` in numeric order, tracks applied versions in a `schema_migrations` table, runs each file in a transaction splitting on `;`. To add a migration, create the next `NNN_name.up.sql` + `.down.sql` pair.
- **`pkg/` infra clients:** `jwt` (auth tokens), `hash` (Firebase scrypt — passwords were migrated from Firebase, see `firebase_scrypt.go`), `supabase` (file storage), `ollama` (LLM grading client + rubric), `glot` (remote code run), `cwasm` (clang→wasm compile), `response` (HTTP envelope).
- **Domain enums:** `internal/entity/enums.go` is the source of truth for roles, course types (`pretest`/`posttest`/`keterampilan`/`ujian_praktik`), soal types, exam categories, and pengerjaan status. Reuse these constants rather than string literals.
- **Response envelope:** all handlers return `{success, message, data, error}` via `pkg/response`. The frontend `api.ts` unwraps `.data` and treats `success:false` or non-2xx as an error.

## Frontend architecture (`frontend/`)

SvelteKit (Svelte 5 runes) + TypeScript + Tailwind, deployed to Vercel.

- **API layer: `src/lib/api.ts`.** Single `api.get/post/...` wrapper adds the JWT from `localStorage`, unwraps the `{success,data}` envelope, and on `401` clears the session and redirects to `/praktikum/login`. Use it for all backend calls. Base URL from `PUBLIC_API_BASE_URL`.
- **Auth state: `src/lib/stores/auth.ts`** (`user` store, `hasToken()`).
- **Route guards are layout-based — placement enforces access:**
  - `src/routes/praktikum/+layout.svelte` requires a token (redirects to login otherwise) and wraps pages in `AppShell`.
  - `src/routes/praktikum/admin/+layout.svelte` requires `role === 'admin'`. **Any admin-only page MUST live under `praktikum/admin/`** or it silently skips the role guard. (Backend `RequireRole` still enforces it server-side, but don't rely on that alone.)
- **Editors:** `CodeEditor.svelte` (Monaco, live coding), `RichTextEditor.svelte` + `src/lib/components/edra/` (TipTap-based WYSIWYG with KaTeX) for composing questions.
- **`Countdown.svelte`** drives exam timers; the server is authoritative on expiry (sweeper/cron), the client countdown is display + auto-save trigger only.

## Config & environment

`backend/config/config.go` loads from env (see `.env.example`, copied to `backend/.env`). Key vars: `DB_*` (Postgres), `JWT_SECRET`, `SUPABASE_*` (storage), `OLLAMA_*` (AI grading), `SWEEPER_INTERVAL_SECONDS`, `CORS_ORIGINS`, and the cron secret. Frontend env (`frontend/.env`) uses `PUBLIC_`-prefixed vars consumed via `$env/static/public`.

## Data-integrity note

`pengerjaan_course` (student grades) uses `ON DELETE RESTRICT` FKs — deleting a parent session/course/class/student is refused while grades reference it. Don't loosen this without understanding the data-loss risk it guards against.
