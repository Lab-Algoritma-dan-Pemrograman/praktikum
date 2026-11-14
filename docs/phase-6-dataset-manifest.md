# Phase 6 — masked dataset manifest (2026-11-13)

Scope: rehearsal masked selesai pada13 November2026; [closeout final](phase-6-closeout.md) mengikat batas evidence dan exclusions. Dokumen ini mencatat data yang diproses/imported/rejected tanpa baris pribadi, credential atau token. Pemilik mengizinkan local masked/staging processing dan menyerahkan pendekatan konservatif saat ditanya scope rejection/files/recovery.

## Source identity

| Item | Value |
| --- | --- |
| Primary source | Legacy PostgreSQL 17.6, 107 public tables, project fingerprint `0c59e695…19e3` |
| Secondary source | Legacy SIAKAD API, 26 relations, project fingerprint `3cc26afc…63d5c` |
| Secondary crosswalk | 320 of 323 identities matched by exact NIM; 3 unmatched rejected |
| Snapshot method | Primary PostgreSQL repeatable-read/read-only transaction, masked after commit; secondary SIAKAD REST paginated read-only extraction, not a cross-source transaction |
| Working artifact | Pseudonymized snapshot held locally; raw rows were never written to disk |

Masking: source numeric user IDs replaced with random opaque IDs, NIM replaced
with `MIG<opaque>`, names replaced with `Migration User <opaque>`, emails replaced
with `migration.<opaque>@example.test`. Passwords, password hashes, salts, tokens,
QR bearers, Supabase Auth subjects, phone/social/profile links and audit payloads
were never selected. Learner-authored answer text and feedback were replaced by a
length marker. This is pseudonymization, not a claim of irreversible anonymization.

## Dispositions

80,349 source rows were classified. Every row is either imported or rejected with
an explicit reason; nothing was silently dropped.

| Disposition | Rows |
| --- | --- |
| Imported | 19,816 |
| Rejected | 60,533 |

### Imported

| Source | Target | Rows |
| --- | --- | --- |
| users | auth.users + public.users + role assignment + canonical grants | 396 |
| student_progress | learning_progress (completed state, opening-balance XP) | 15,892 |
| jawaban_mahasiswa | practicum_attempts (embedded answers) | 2,403 |
| pengerjaan_course | practicum_attempts + grading job (`needs_manual`) | 609 |
| soal_terpilih | practicum activation snapshots | 147 |
| soal | practicum_questions | 142 |
| lessons | learning_lessons | 78 |
| aktivasi_course / aktivasi_sesi | practicum_activations | 37 / 37 |
| modules / levels | learning_modules / learning_levels | 25 / 6 |
| course / sesi_praktikum | practicum_courses / practicum_sessions | 15 / 5 |
| kelas / jadwal | academic_classes / academic_schedules | 12 / 9 |
| SIAKAD inventory_items | academic_inventory_items | 3 |

Derived target rows: 396 learning profiles (opening XP from the legacy user
total; imported progress awards zero additional XP).

### Principal rejection reasons

| Reason | Rows |
| --- | --- |
| Table outside the approved semantic adapter (audit, operations, archives, alternate generations) | 49,214 |
| Achievement semantics not provable in the target model | 5,665 |
| Attempt or question rejected by the parent disposition | 1,343 |
| Attempt not terminal or its activation was rejected | 1,025 |
| Lesson contract rejected (flowchart lessons) | 684 |
| Attendance row without provable schedule/session linkage | 681 |
| Late-admission history without lossless target | 8 |
| Archived question | 20 |
| Missing class shift | 1 |
| Unsupported flowchart lesson contract (separate from rejected progress rows) | 3 |

## Explicit non-decisions

The batch does **not** fabricate: whole-game history as per-question attempts,
missing opening XP, fractional grades, attendance schedule links, file
destinations, financial ledgers, or achievement unlocks. Those rows are rejected
with the reasons above and must be decided by the owner if they are ever needed.

## Identity and access

Every imported identity receives one role assignment and the canonical
role/division default grants, mirroring normal provisioning. Academic membership
is expressed by the class roster, not by a narrowed assignment scope, because a
narrowed scope would deactivate the canonical role grants.

Exactly one imported identity is an **acceptance login** account: it has a
password generated locally for this rehearsal and stored only in a private
evidence file. No legacy password or hash was migrated.
