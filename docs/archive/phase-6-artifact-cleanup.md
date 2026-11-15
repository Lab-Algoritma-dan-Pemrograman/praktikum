# Phase 6 — artifact retention and cleanup

Cleaned 13 November 2026 after Phase 6 closeout. This document tells the next
operator what still exists, where it is, and what deliberately was deleted.

## Deleted (generated only)

82 files / 875,281,033 bytes were removed after a reviewed manifest:

- Go executables built per rehearsal run (`rehearsal.exe`, `live-import.exe`,
  import binary) inside disposable evidence directories.
- The stray workspace build `phase6-live-import.exe` (14.4 MB).
- Python bytecode caches (`scripts/__pycache__/phase6-*.pyc`).
- Duplicate or expired local test credentials (`acceptance-private.json`) from
  local, superseded, or failed runs.

Each deletion required all of:
`ownership.json` present, declared `data_directory` equal to that run's
`pgdata`, the cluster already absent, and the file's size/SHA-256 matching the
manifest. The original staging acceptance credential was excluded explicitly.
Manifests: `E:/tmp/algohub-phase6-live-20261113-f9aelu0c/artifact-cleanup-candidates.json`
and `artifact-cleanup-result.json`. All builds are reproducible with
`go build ./backend/cmd/...`.

## Retained

| Item | Location |
| --- | --- |
| Plan v3, masked snapshot, staging reports, browser/session reports | `E:/tmp/algohub-phase6-live-20261113-f9aelu0c/` |
| Original committed receipt and pre-import dumps | `E:/tmp/algohub-phase6-stage-5_6uzevg/` |
| Post-import sanitized dump and recovery verification | `E:/tmp/algohub-phase6-recovery-87b_147f/` |
| Final local rehearsal/regression report | `E:/tmp/algohub-phase6-masked-local-w1d1os3y/` |
| Duplicate failure evidence, logs, source archive | `E:/tmp/algohub-phase6-*` |
| Original staging acceptance credential | `E:/tmp/algohub-phase6-stage-5_6uzevg/acceptance-private.json` |
| Durable non-secret summary | `docs/evidence/phase-6-final-2026-11-13.json` |

Failure artifacts are retained on purpose. Do not delete them to make the
history look clean, and do not relabel failures as passes.

## Not touched

No legacy/production database, no staging rows, no scheduler, no DNS, no
deployment. No live cluster or container was running. No retained backup or
receipt was modified.
