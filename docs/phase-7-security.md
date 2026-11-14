# Phase 7 — Security Report

Status: **read-only source review**, 13 November 2026. Tool: manual code inspection terarah pada `backend/internal/` dan `apps/`. Scope: defensive review untuk aplikasi staging internal milik pemilik.

## Ringkasan eksekutif

**Tidak ada temuan high atau medium.** Arsitektur security-by-default konsisten di 6 area fokus.

## Detail per area

### Auth/SSO Cookie — SOLID
- Cookie flags benar: `Secure`, `SameSite=Lax`, `HttpOnly`, session-only.
- CSRF: double-submit terikat session hash server-side (`usecase/identity.go:311`).
- Session host-bound (`identity.go:289`); session family revocation (`identity.go:259-271, 355`).
- Revoke/Logout verifikasi ownership (`identity.go:371`).
- SSO PKCE S256 + kode sekali-pakai + returnTo allowlist.

### IDOR/BOLA — SOLID
- `practicumOwn()` cek `UserID` di semua akses attempt (`practicum_attempts.go:36`).
- List grades filter `OwnerID` (`practicum_attempts.go:405`).
- Learning progress/profile di-key ke `p.User.ID`.
- Output sanitization hapus `Solution`/`CorrectOption` untuk non-admin (`learning.go:405-418`).
- Revoke session validasi path & ownership (`auth_handlers.go:231`, `identity.go:371`).

### XSS — SOLID
- Nol `{@html}` di Svelte components aplikasi (hanya di node_modules/vendor).
- Nol html/template unsafe di backend.
- CSP `default-src 'none'` + `X-Frame-Options DENY` (`security.go:25`).

### Rate Limit (429) — SOLID, 1 catatan
- Implementasi DB atomik (`upsert` + `where hits < limit`, `postgres_identity.go:467-471`) — tidak bisa di-race.
- `Retry-After` header benar (`auth_handlers.go:85`).
- **Catatan (low):** IP berasal dari `RemoteAddr` tanpa `X-Forwarded-For`. Semua user di balik proxy yang sama berbagi satu limit per-menit. Bukan celah keamanan (tidak bisa di-spoof), tapi bisa kurang nyaman saat banyak user di jaringan sama.

### Idempotency/XP Replay — SOLID
- `learningMutate` hash `(user, action, resource, key)` + payload hash comparison + single DB transaction (`learning.go:288-327`).
- XP授予 hanya saat `status != "completed"` (`learning.go:858`).
- Key validated regex (`learning_handlers.go:52`).

### Input/Output Bounds — SOLID
- `MaxBytesReader`: 64 KB (identity) / 256 KB (practicum + learning).
- `DisallowUnknownFields`; reject trailing JSON.
- Pagination 1–100.
- Draft code 64 KB limit (`learning.go:771`).

## Dependency / secret scan

Audit Go (`govulncheck`, Go 1.26.11): **No vulnerabilities found** (Phase 6 closeout).
NPM (`npm audit --audit-level=high`, 14 November 2026): exit 0; **0 high/critical**, 4 low, dan 3 moderate pada rantai tooling/dependency. Bukti: `docs/evidence/phase-7-npm-audit-2026-11-14.json`. Advisory total tetap 7; distribusinya berubah dari catatan Phase 6 (3 low/4 moderate).

## Status gate

| Gate | Hasil |
| --- | --- |
| Security review | **6 SOLID, 1 low (non-critical)** |

Satu catatan rate-limit IP noted untuk dokumentasi; bukan blocker. Review terarah ini scope-nya kode sendiri, bukan audit penetration tester profesional.

## Sisa pekerjaan

- [ ] Konfirmasi pemilik: apakah rate-limit catatan IP perlu di-address (misal XFF dengan trust proxy terbatas).
- [ ] Incident response drill (G8): isolated local fault injection, bounded staging drill setelah approval.
