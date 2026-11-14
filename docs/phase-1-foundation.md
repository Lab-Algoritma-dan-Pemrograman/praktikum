# Phase 1 Foundation

Phase 1 establishes contracts and composition boundaries only.

## Boundaries

- SvelteKit applications are separate deployable hosts under `apps/`.
- Shared presentation primitives live in `packages/ui`.
- The Go API uses `router → middleware → handler → usecase → repository`.
- `backend/api/index.go` is the Vercel serverless adapter and uses the same
  router builder as the local API entrypoint.
- `/healthz` is liveness. `/readyz` remains unavailable until a real database
  adapter is introduced; this avoids reporting an unconfigured dependency as
  healthy.
- `docs/openapi/openapi.yaml` and `openapi.json` are the Phase 1 contract for
  the health endpoints and are served from `/docs`.

No production deployment, database migration, DNS change, or legacy project
modification is part of this phase.
