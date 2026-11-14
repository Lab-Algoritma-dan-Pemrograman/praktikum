# Phase 6 — source, target, and data classification inventory

Scope: read-only code/schema inventory on 2026-11-13. **Not a live database inventory or authorization to process personal data.** No legacy rows/uploads/credential values were copied or inspected.

## Source authority is unresolved

Legacy root: `E:/rekapProject/project_ap/`.

| Candidate source | Schema/code authority | Caveat |
| --- | --- | --- |
| Praktikum | `praktikum/backend/database/migration/` (001–034), `praktikum/backend/internal/entity/` | Applied version unknown; later entities contain fields absent from early migrations |
| Standalone E-learning | `elearning/docs/setup-new-database.sql`, subsequent SQL and `elearning/src/services/` | NIM-keyed identity and text curriculum IDs; service/schema drift |
| Standalone SIAKAD | `siakad/supabase/migrations/`, current RPC/page code | Older generated `types.ts` describes UUID profiles/attendances; later RPCs use BIGINT users/attendance_logs |
| Consolidated alternative | `docs/sql/004_domain.sql`, `005_elearning.sql`; Praktikum `belajar.go`, `materi.go`, `game.go` | Separate source generation, not evidence that consolidation completed |

Never union these generations or merge people by NIM/email/name automatically. An approved crosswalk must map **source system + entity + original ID** to a target identity. A legacy Supabase ID is provenance, not automatically the new Auth ID.

Only old aggregate metadata was available: June 20, 2026 backup reports 94 Praktikum users; June 21 postmigration summary reports 188 users and 1,580 course-work records. These are **not current counts, not additive populations, and not rehearsal acceptance targets**. Current row/file counts, bytes, completeness and cutoff remain unknown.

Potential snapshot roots (`backups/`, `praktikum/backups/`, historical `praktikum/updateAndPRDERD/migration/` exports) are candidates only. None is selected or authorized.

## Mapping and losslessness decisions

| Source concepts | Target | Safe mapping / unresolved semantics |
| --- | --- | --- |
| users, roles, divisions | `users`, assignments, grants | `nama/full_name → name`; explicit source-specific role alias allowlist; no password/session copying; approved Auth crosswalk required |
| kelas/jadwal/group members | `academic_classes`, `academic_schedules` | Remap roster IDs; `hari → day_of_week`, times → starts/ends; class/major/period and shift grain require crosswalk |
| sesi_praktikum/course/soal | Practicum sessions/courses/questions | Title, sequence, kind, duration, prompt, starter, reference answer; fractional legacy points must be rejected until integer policy approved |
| aktivasi_sesi/course, soal_terpilih | Activations with frozen course/question snapshots | Per-course activation join and ordered question IDs; never fabricate historical snapshots or admission/timer state |
| pengerjaan_course/jawaban_mahasiswa | Attempts, submitted answers, manual grades/jobs | Join selected-question IDs, remap user/course/activation IDs; floats, cached total, activity marks, publication state need explicit policy |
| levels/modules/lessons or level/modul/materi | Learning levels/modules/lessons | Field renames; derive level through module; decode JSON objects vs encoded JSON strings explicitly; flowchart definitions unsupported |
| student_progress/progres_belajar | Learning progress + profiles | Explicit completion mapping and XP ledger/opening-balance policy; never invent code, quiz choice, execution proof or per-lesson XP |
| elearning_progress in SIAKAD | No separate import | A synchronized cache, not another source of completions |
| game questions/history | Bug-hunt questions/question attempts | Current buggy-line vs old multiple-choice variants differ; whole-game history lacks per-question identity/selection and cannot be fabricated |
| attendance_logs/absensi/qr_sessions | Attendance sessions/records | Schedule/session/user crosswalk mandatory; do not guess schedule from date, recorder as validator, or scan type as QR proof |
| inventory_items/inventaris | Inventory items | Old quantity does not establish available stock; preserve unavailable units; approve code/category/state mapping |
| rentals/peminjaman | Inventory rentals | Currency/unit/integer policy and status mapping; no inferred payment verification, return/approval times |
| reports, financial records, prestige, unsupported content | Rejected/quarantined pending decision | No demonstrated lossless target; financial ledger is not rental revenue |

The initial executable contract is **synthetic only**, not an adapter for any of these unverified live generations. Unsupported rows require an explicit rejection reason; no silent omission.

## Target schema and load order

[Migration catalog](../backend/migrations/migrate.go) pins the exact SQL bytes. Existing staging already applied 0002–0007; run read-only history comparison, not `Apply`, against an existing target.

- 0002–0004: identity and seeded access catalogs.
- 0005: eight Practicum tables.
- 0006: nine Learning tables.
- 0007: eight Academic/inventory tables.
- Total: 37 application tables plus `schema_migrations` = 38 public RLS tables. `auth.users` is a prerequisite outside this set.

Load users → assignments → grants after schema/catalog initialization. Independent domain branches:

1. Practicum sessions → courses → questions → activations (parent is **session**) → attempts → grading jobs.
2. Learning levels → modules → lessons → progress; bug-hunt → game history; profiles reconciled against imported activity.
3. Academic classes → schedules → attendance sessions → records; inventory items → rentals.
4. Approved audit/notification history last; ephemeral sessions, handshakes, rate counters, leases and operations excluded.

Use insert-or-equal, **not blanket UPSERT**: game history rejects every UPDATE and other triggers freeze completed/submitted state. Do not disable constraints/triggers/RLS.

Domain tables use indexed `id,parent_id,owner_id,scope,status,unique_key,expires_at` plus typed `data` JSON. Reconcile **both SQL FK and embedded JSON relationships**. Required application keys include `profile-<user>`, `progress-<lesson>-<user>`, `att-rec-<session>-<user>`, attempt `<activation>:<user>` unique key and grading-job ID equal to attempt ID. Arbitrary IDs can pass SQL yet fail application lookups.

Cross-check profile XP/counters, published manual totals, and inventory invariant `total - available >= approved rental quantities`. Do not replace unknown scope with unrestricted `{}`. Final target role vocabulary is koordinator/asisten/mahasiswa/peminjam, with six assistant divisions; no new admin/pengajar role.

## Classification / masking boundary

| Class | Examples | Default rehearsal action |
| --- | --- | --- |
| Restricted identity | Names, NIM, contact/social details, photos, crosswalks | Synthetic replacement only until approved |
| Confidential academic/behavioral | Answers, grades, feedback, attendance, class membership, progress/activity/rentals | Synthetic only; free text may include health or other sensitive information |
| Confidential operational | Answer keys, unpublished content, finance, audit snapshots, IP/user-agent | Do not copy; separately approve scope and access |
| Credentials/ephemeral authentication | Passwords/hashes/salts, sessions, tokens, QR bearers, push keys | Excluded; never logged, exported or migrated by this tooling |
| Approved public content | Published lesson assets | Public status must be established, not inferred from an accessible URL |

Actual masking rules, retention duration, authorized readers, deletion owner and allowed rejection tolerance remain owner decisions. Hashing an identity is pseudonymization, not proof of anonymization.

## File-copy plan (no copying authorized)

| Convention from code | Required handling |
| --- | --- |
| Praktikum default `public-assets`, `{folder}/{timestamp}.{ext}` | Verify actual bucket and ownership; source references include question image, guide file and profile image |
| Private `laporan-besar`, `{reportID}/{studentID}/{type}-{timestamp}.{ext}` | IDs/paths are identifying; preserve private ACL; object path rather than expiring signed URL; no confirmed target attachment model yet |
| E-learning `lesson-images`, public URLs in content | Inspect actual bytes/MIME only after approval: compression may emit JPEG while preserving the original extension |
| SIAKAD file/image URL fields | Bucket/ownership/local root unknown; URL alone does not authorize copying |

After explicit authorization: operator supplies a non-secret manifest (opaque reference, approved source object/version, size, SHA-256, verified MIME, destination key, privacy class, referring rows). Reject traversal, symlinks, collisions and missing objects. Copy into an isolated destination prefix with bounded I/O, no overwrite, private-by-default ACL, then verify count/bytes/hash and referential coverage. Retry equal existing objects as no-op; changed bytes fail. Publish reference rewrites only after validation. Rollback removes only newly copied unchanged objects from that manifest. Keep unsupported attachments quarantined rather than silently dropping them. No storage credential, object download or upload is performed in this local preparation.
