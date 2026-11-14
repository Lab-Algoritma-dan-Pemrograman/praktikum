# Phase 7A Component Decisions

**Date:** 2026-11-14  
**Scope:** local source only; no paid retrieval, dependency install, deployment, or remote mutation.

## Decisions

| Component/pattern | Decision | Rationale/license |
|---|---|---|
| Portal editorial hero | **Accepted, manual Svelte/CSS** | Matches calm technical lab and Burrow rhythm without copying veterinary copy/assets/source. |
| Portal service cards | **Accepted, manual Svelte/CSS** | Concrete Algohub services; no generated full-page template. |
| Portal public status page | **Accepted, manual shared component** | Consistent loading/blocked/available presentation; fail-closed for missing data contracts. |
| Shared token layer | **Accepted, minimal CSS token file** | `packages/ui/src/tokens.css`; no dependency or runtime data. |
| Existing AppShell/AdminShell | **Retain and incrementally align** | Behavior/permission logic is already exercised; no rewrite. |
| Shared RequestError/IdentityError | **Accepted** | Existing safe retry/login semantics and request ID display. |
| 21st editorial candidates | **Direction only** | Prior review accepted shape/hierarchy ideas and rejected React/Tailwind source. No source or dependency installed. |
| Burrow reference | **Direction only** | Local MIT reference; adapted rhythm/shapes only. No veterinary copy/photo/form/source shipped. Attribution remains in landing footer. |
| Unsplash/Google Sans assets | **Rejected** | Not needed by Algohub landing; avoid unverified asset dependency. |
| Full-page generated template | **Rejected** | Violates anti-template rule and increases blast radius. |
| Charts/metrics/fake proof | **Rejected for Phase 7A** | No verified product metrics source; avoid fabricated claims. |

## License and dependency record

- No new npm package was installed.
- Existing SvelteKit/Svelte/Vite, Playwright, and axe dependencies are retained.
- The local Burrow reference is MIT for its source; only design direction is adapted and historical attribution is retained.
- MCP 21st API key remains environment-only; no key was read, copied, or logged.
