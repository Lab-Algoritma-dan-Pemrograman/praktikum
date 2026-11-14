# Phase 7A host/domain migration checklist

**Status:** local planning only; no remote mutation performed.  
**Decision:** the only public landing is `algohub.web.id/` in production and `staging.algohub.web.id/` in staging. `portal.algohub.web.id` and `portal.staging.algohub.web.id` are not product hosts.

## Local source completed

- [x] Production origin list includes `https://algohub.web.id` as the portal origin.
- [x] Staging example allowlists use `https://staging.algohub.web.id`.
- [x] Application-link resolution accepts apex production/staging landing roots and app subdomains.
- [x] Illegal `portal.staging.algohub.web.id` fails closed in the identity contract test.
- [x] `portal.localhost` remains an isolated Playwright/Vite alias only.
- [x] `/info` is a local 308 compatibility redirect to `/`.
- [x] `/praktikum` is a local compatibility redirect to the dedicated practicum host.

## Remote work intentionally not performed

- [ ] DNS record removal/addition for old/new staging landing host.
- [ ] Vercel project/domain binding or verification.
- [ ] SSO allowlist/config mutation.
- [ ] Production DNS or deployment.
- [ ] Staging redeploy.

## Owner/operator sequence after Phase 7A approval

1. Confirm the apex staging landing deployment/project and its target CNAME with the owner.
2. Add/verify `staging.algohub.web.id` on the staging landing project.
3. Update remote API/browser/SSO allowlists atomically, retaining app subdomains and removing the old portal host only after traffic/link audit.
4. Verify HTTPS, `/`, `/info` redirect, auth return-to allowlist, and app switcher on the isolated staging environment.
5. Retire the old `portal.staging` binding/DNS only after the preceding checks and explicit approval.
6. Record the actual operator, timestamp, provider evidence, and rollback target in a new deployment evidence file; do not overwrite historical Phase 1–7 evidence.
