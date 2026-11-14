# Phase 2 — Identity, SSO, authorization, and admin shell

Scope: Phase 2 only. Practicum, E-learning, and SIAKAD domain features remain placeholders. The frozen `plan/` documents were not edited.

## Identity and persistence

- Both Go entrypoints use `internal/application.New` and the same router.
- Normal operation requires PostgreSQL and Supabase Auth configuration. The pgx pool is bounded (default four connections per instance), uses request deadlines and transaction-pooler-compatible execution, and never falls back to memory.
- Memory storage and synthetic users require `ALGOHUB_ENV=local`, `ALGOHUB_ENABLE_FIXTURES=true`, a loopback listener, no database/provider configuration, and no Vercel runtime.
- A verified Supabase identity gets an application profile linked to `auth.users`. It receives **no role or permission automatically**. Existing profile names and inactive-account state are preserved.
- Roles remain `koordinator`, `asisten`, `mahasiswa`, and `peminjam`. Divisions remain assignments, not roles. Effective grants intersect active assignments with role/division defaults; explicit grants must remain inside their assignment's scope.
- Only a globally scoped coordinator with `identity.access.manage` can replace another user's access. Changes use optimistic versions, an atomic transaction, and before/after audit records. Self-edit is rejected.

## Session and SSO contract

1. Each application starts login with a short-lived, HttpOnly PKCE verifier and expected destination cookie.
2. Auth Center accepts a password or configured Supabase OAuth login. An existing central application session avoids another login.
3. Auth Center requests a two-minute, one-use handshake bound to the destination and its PKCE challenge.
4. The destination callback validates its pending cookies and exchanges the code through its server-side identity proxy.
5. Go validates the origin, issuing session, SSO family, expiry and verifier, then consumes the code transactionally.

Browser API calls are same-origin `/api/v1/...` requests. Only approved identity routes are proxied; cookies, CSRF and the validated application origin are forwarded server-to-server. This avoids attempting to send host-only application cookies to a different API hostname.

Application cookies are host-only, `SameSite=Lax`, and `Secure` outside local development; the session cookie is HttpOnly. No access or refresh tokens are stored in URLs, localStorage, application tables, or browser-readable responses. Supabase tokens are discarded after identity verification. The old encryption-key setting is no longer needed by this fixed-lifetime session design.

- Sessions have a fixed maximum lifetime (default 12 hours).
- `/v1/auth/refresh` rotates both opaque session and CSRF tokens; it does **not** extend expiry.
- Logout revokes the login's complete SSO family across hosts, but not a separate device/login family.
- Individual revocation also invalidates handshakes issued by that session.
- Every authenticated request rechecks account state, session family and current permissions.
- Profile, access changes and session mutations are audited in the same transaction. Missing audit persistence fails the mutation.
- Private responses are `private, no-store`. Rate limits return `429` with `Retry-After` and use shared PostgreSQL counters outside fixtures.

## Admin interface

The admin layout authorizes direct routes on the server before rendering. Permission-aware navigation is only presentation, not the security boundary.

- `/`: permitted module overview and application switching.
- `/profil`: live profile editing, session listing and revocation.
- `/auditlog`: scope-filtered audit records with cursor pagination. `meta.next_before` advances even across an empty filtered page.
- `/pengaturan`: coordinator access editor for roles, divisions, scopes and explicit permissions; stale changes return `409`.
- Domain routes: authorized shells only. No Phase 3/4/5 CRUD, assessments, runner, grading, inventory or learning logic.

## Migrations and initial coordinator

`0002_identity.sql` and `0003_identity_seed.sql` are unchanged. Forward migration `0004_identity_hardening.sql` adds session families, immutable session rules, one-use SSO handshakes, access versions, assignment-bound direct grants, rate counters, and tighter RLS/privileges. Browser roles cannot read session secrets or update administrative profile columns directly.

The migration command tracks checksums and applies each migration in a transaction under an advisory lock:

```sh
ALGOHUB_ENV=staging ALGOHUB_DATABASE_URL='<fresh staging database URL>' go run ./backend/cmd/migrate
```

Use a secret manager or shell environment for real credentials, not a committed command/file. Local migrations require a loopback host. Production requires a separate explicit override; this implementation did not execute production migrations.

After the first authorized coordinator signs in once to create their profile, an authorized operator can bootstrap that **fresh staging account** in the database console:

```sql
begin;
select code from public.roles where code = 'koordinator' for update;
with assigned as (
  insert into public.user_role_assignments (user_id, role_code, scope)
  select id, 'koordinator', '{}'::jsonb from public.users
  where auth_user_id = '<verified auth UUID>'::uuid and is_active
  on conflict do nothing returning user_id
), updated as (
  update public.users set access_version = access_version + 1
  where id in (select user_id from assigned) returning id
)
insert into public.audit_logs
  (action, app, subject_type, subject_id, request_id, operation_id, detail)
select 'identity.bootstrap', 'identity', 'user', id::text,
  'manual-staging-bootstrap', 'identity.bootstrap',
  '{"source":"authorized staging bootstrap"}'::jsonb
from updated;
commit;
```

Subsequent assignments use the coordinator interface. Repeating the bootstrap does not duplicate assignments. Migration of ambiguous pre-existing explicit grants is deliberately rejected rather than guessed.

## Local development and verification

Use `.env.example` for the isolated six-host local setup: portal5173, auth5174, admin5175, siakad5176, elearning5177, praktikum5178, with distinct `*.localhost` hosts. Export the variables into each process; Go does not read `.env` automatically. The browser test launcher configures and starts the isolated apps itself.

```sh
npm run lint
npm run build
npm test
npm run api:vet
npm run api:build
npm run api:test:race
npm run api:audit
npm run test:e2e
```

`npm test` includes frontend unit tests, Go tests and 21 live OpenAPI operation contracts. Database integration tests additionally require `ALGOHUB_TEST_DATABASE_URL` pointing to a disposable loopback PostgreSQL database whose name ends in `_test`. They create a minimal Supabase Auth fixture, apply/repeat migrations, and verify RLS, constraints, rollback, production pgx execution mode and concurrent session/access operations. Never point this variable at an existing application database.

On this Windows machine, MSYS2 compiler DLLs must precede conflicting PATH entries for race tests:

```sh
PATH="/c/msys64/mingw64/bin:$PATH" npm run api:test:race
```

This is a per-command PATH adjustment, not a global environment change. The module/workspace now require Go 1.26.11, including the fix for GO-2026-6859; Go's automatic toolchain selection leaves the global installation unchanged. Current closeout verification used standard Linux race tests in disposable Docker containers, without disabling ASLR or changing host security. CI runs PostgreSQL integration, the race detector, OpenAPI checks, vulnerability scanning and Chromium browser tests on Linux. See [current verification evidence](phase-1-5-closeout.md#linux-staging-dan-audit-dependensi--11-november-2026).

## Deployment boundaries

Local fixtures, HTTP fake-provider tests and isolated PostgreSQL tests do not certify a real Supabase tenant or deployed multi-domain environment. Before staging acceptance, configure the fresh Supabase project, provider callback allowlist, database TLS/pooler, API/auth/app origins, secure cookies and the first coordinator, then repeat the browser flows there. Frontend servers need the API/auth origins; the database URL belongs only on the Go API.

No production Supabase, DNS, deployment, or legacy project was changed. No Phase 3 implementation was started.

## Local acceptance results

Final checks on Go 1.26.10 / Node 24.19.0:

| Check | Result |
| --- | --- |
| Formatting and six-app typecheck | Passed; zero errors/warnings |
| Six frontend production builds and Go build/vet | Passed |
| Frontend unit tests | 20 passed |
| Go unit, PostgreSQL/RLS integration, and race tests | Passed; PostgreSQL tests were enabled, not skipped |
| Database-backed application test | Passed across two API instances with a local HTTP identity-provider fake |
| Live OpenAPI contracts | 21 operations / 28 response cases passed |
| Chromium E2E | 26 passed, including 320/768/1280 px and JavaScript-disabled route guards |
| Cross-package Go statement coverage | 77.4% |
| Go vulnerability scan | No vulnerabilities found after the toolchain patch |
| Secret-pattern scan | No matches |
| npm dependency audit | No high/critical findings; 3 low and 4 moderate transitive advisories remain |

The npm advisories involve `deepmerge`, `glob`, and `minimatch` in the build-tool dependency chain. The suggested forced remediation downgraded SvelteKit/its adapter, so no breaking framework downgrade was applied. Review compatible upstream fixes before production release.

The HTML browser report is generated in `playwright-report/`; traces, videos and screenshots are disabled so the report does not capture session cookies or password requests. Temporary PostgreSQL was stopped after testing. Real Supabase/OAuth and deployed-host acceptance remain configuration-dependent and were not claimed as verified.
