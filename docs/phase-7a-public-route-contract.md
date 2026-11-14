# Phase 7A Landing and Public Route Contract

**Canonical host:** `algohub.web.id/` production; `staging.algohub.web.id/` staging.  
**Local harness:** `portal.localhost` is an isolated test alias only.

## Routes

| Route | Behavior | Data policy | Status |
|---|---|---|---|
| `/` | Editorial landing with auth-aware workspace CTA | no invented operational facts | implemented/local |
| `/jadwal` | Public-safe information state | schedule publication contract required; no personal/class roster data | blocked-owner-data |
| `/asisten` | Public-safe information state | no NIM, private phone, WhatsApp, social links, or internal assignment | blocked-owner-data |
| `/laporan` | Public-safe information state | no legacy raw file URL; signed/public attachment model required | blocked-owner-data |
| `/modul` | Public-safe information state | no legacy raw file URL; target content publication model required | blocked-owner-data |
| `/bantuan` | Static help state with login action | no credentials/tokens in copy or URL | implemented/local |
| `/privacy` | Explicitly pending owner/legal text | no placeholder legal claim | blocked-owner-data |
| `/terms` | Explicitly pending owner/legal text | no placeholder legal claim | blocked-owner-data |
| `/info` | 308 redirect to `/` | compatibility only, not a second landing | implemented/local |
| `/praktikum` | 307 redirect to `https://praktikum.algohub.web.id/` | dedicated host; no local admin route | implemented/local |

All public routes implement a heading, description, status/action, responsive layout, reduced-motion fallback, and automated viewport/a11y assertions. No target API was invented and no legacy payload was copied.
