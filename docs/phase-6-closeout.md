# Phase 6 — migration rehearsal closeout

Status: **100% selesai pada scope rehearsal masked yang disepakati**, 13 November 2026.

Ini penutupan **rehearsal**, bukan persetujuan cutover produksi, migrasi lossless semua data legacy, atau recovery penuh Supabase Auth. Pemilik mengizinkan pembacaan legacy secara read-only, pemrosesan lokal terisolasi, dan import hasil masking ke dedicated staging existing. Ketika ditanya batas data rejected/file/recovery, pemilik menyerahkan pemilihan pendekatan; pendekatan konservatif di bawah dipilih dan dijelaskan sebelum penutupan.

## 1. Scope final

- Sumber utama: PostgreSQL legacy 17.6, 107 tabel public; sumber sekunder: SIAKAD REST, 26 relasi. Tidak ada write ke sumber legacy.
- Snapshot utama diambil dalam repeatable-read/read-only transaction; transaksi ditutup sebelum CPU-bound masking. Ekstraksi REST sekunder memakai pagination terikat; **bukan** satu transaksi database konsisten lintas sumber.
- 80.349 baris sumber terklasifikasi: **19.816 imported, 60.533 rejected dengan alasan**. Sebagian baris sumber menjadi nested snapshots, sehingga jumlah source dispositions berbeda dari jumlah target rows.
- Target staging mendapat **19.026 receipt rows**: 396 Auth users, 396 public users, 396 assignments, 17.838 domain rows. Canonical permission grants dimaterialisasi sebagai state turunan; total staging setelah import adalah 481 grants, termasuk grants existing.
- Nama/NIM/email diganti, user ID dipseudonimkan, jawaban/feedback mahasiswa diganti marker panjang. Password/hash/salt/token legacy, QR bearer, contact/social/profile links tidak dipindahkan. Raw working rows tidak pernah ditulis ke disk.
- XP profile adalah **opening balance** legacy; progress hasil import mempertahankan completion tetapi memberi nol XP tambahan. Nilai Praktikum tetap manual; attempt import berstatus submitted/needs_manual tanpa nilai buatan.
- Unsupported history (achievement/game/flowchart/absensi tanpa relasi/financial ledger/arsip/operational records) ditolak, tidak direkayasa. Data asli tetap utuh. Penutupan ini tidak membuat rejected rows menjadi migrated.
- File-copy **tidak berlaku pada scope final**: target attachment model belum tersedia. Inventory/mapping dan prosedur copy aman disimpan; tidak ada download/upload objek atau klaim file sudah dipindahkan.

Detail: [manifest dataset](phase-6-dataset-manifest.md), [inventory](phase-6-inventory.md), [runbook](phase-6-rehearsal.md). Keputusan runner Phase1–5 tidak dibuka ulang: E-learning browser-trusted XP dan Praktikum manual tetap final.

## 2. Gate yang terbukti

| Gate | Hasil nyata |
| --- | --- |
| Source/target inventory | Metadata, counts, source ambiguity, PII classification, field mappings dan dispositions tersedia |
| Deterministic transform | Plan v3 dipin dengan SHA-256; source IDs qualified; retry menggunakan artifact snapshot yang sama |
| Staging preflight | Enam migration0002–0007 cocok; schema/RLS dan identity/business conflicts diperiksa read-only; migrations tidak dijalankan ulang |
| Import staging | 19.026 rows atomik, commit; durasi392.046ms (sekitar6m32s); receipt pertama dipertahankan |
| Repeatability | Apply dua kali lulus di local real-masked rehearsal; staging resumed replay memverifikasi receipt dan menghasilkan19.026 unchanged tanpa INSERT;15.446ms. Bukan klaim remote Apply baru dijalankan dua kali |
| Content unchanged | Fingerprint28 tabel public tersanitasi sebelum/sesudah replay dan browser identik; Auth stable identity metadata juga identik |
| Final reconciliation | **157 passing checks**, termasuk62 FK provider/public,77 unique indexes public, metadata/payload/accounting checks. Semua receipt rows cocok; hanya satu Auth user punya perubahan last_sign_in_at/updated_at akibat login yang dibuktikan terpisah |
| Backup baseline | Backup **sebelum import** asli dipertahankan dan berhasil direstore ke database lokal baru; baseline30 public profiles terbukti |
| Backup populated | Snapshot public tersanitasi **sesudah import** terpisah; restore cocok counts/content/schema, seluruh sequence last_value/is_called, dan Auth metadata non-credential |
| Rollback rehearsal | Local masked Apply→rollback menghapus hanya dataset; baseline restore diuji terpisah. Tidak ada rollback/delete massal pada staging online |
| Imported-data service/HTTP flows | Sembilan subtest + parent =10 test PASS events pada populated local restore; identity/access, fake-provider SSO, completion XP+0, grading manual publish/replay, QR duplicate/close, rental replay, report dan identity preservation |
| Restored staging flows | Backup populated staging yang memuat data existing direstore; **7/7 recovery gates lulus**, termasuk test bernama benar-benar berjalan dan pass. Pagination inventory104 items diperiksa, bukan halaman pertama saja |
| Browser/Supabase nyata | **31/31 checks** pada frontend staging existing: satu password exchange, imported identity, exact curriculum/progress/owned attempts/class, SSO6 host, Secure/HttpOnly/host-only cookies, no credential leak/page error, logout401 di seluruh host |
| Backend PostgreSQL regression | Go1.26.11 + PostgreSQL18.3: **170 top-level /487 PASS events, nol failure**. Dua named opt-in tests skipped pada suite ordinary karena dijalankan terpisah pada database import;10 package tanpa test bukan skipped tests |
| Vet/build | Seluruh backend vet/build exit0; latest post-acceptance verifier package test/vet/build juga lulus |
| Cleanup | Nol sesi aktif pada akun acceptance; dua keluarga sesi gagal dicabut dengan izin exact-ID; seluruh owned pgdata removed, nol container/network dibuat; report/backups/failure evidence retained |
| Final artifacts | Closeout, evidence hashes, runbook dan handover tersedia; tidak ada production changes/cutover |
| Artifact hygiene | 82 generated build/bytecode/credential-duplicate files (875.281.033 byte) dihapus setelah manifest, ownership, size dan SHA-256 diverifikasi; seluruh backup/receipt/report/failure evidence dan original staging credential dipertahankan. Lihat [artifact cleanup](phase-6-artifact-cleanup.md) |

## 3. Evidence dan provenance

Workspace bukan Git repository; tidak ada commit/SHA Git yang dibuat. Source-before archive354 files: `E:/tmp/algohub-phase6-20261113-zkaqtokq/source-before.zip`, SHA-256 `bdc586643489d9ea3ece1f4af53b469a3e6967975d8c650a35dab712ab45d74a`.

| Evidence | Lokasi |
| --- | --- |
| Source snapshot/plan/masking + staging final reports | `E:/tmp/algohub-phase6-live-20261113-f9aelu0c/` |
| Original committed receipt + pre-import dumps | `E:/tmp/algohub-phase6-stage-5_6uzevg/` |
| Final local masked rehearsal/regression | `E:/tmp/algohub-phase6-masked-local-w1d1os3y/summary.json` — passed,7 gates,135 source inputs unchanged during this run |
| Staging restore verification | `E:/tmp/algohub-phase6-recovery-87b_147f/summary.json` — passed,7 gates |
| Real browser acceptance | `E:/tmp/algohub-phase6-live-20261113-f9aelu0c/browser-acceptance-v3/summary.json` — passed31 checks |
| Final reconcile | `staging-reconcile-postacceptance.json` in source evidence root |
| Final sessions | `acceptance-session-final.json` in source evidence root — active_sessions0 |
| Durable shareable summary | [phase-6-final-2026-11-13.json](evidence/phase-6-final-2026-11-13.json) |

Plan v3 file SHA-256: `2da24c02db1dc5bea3458f405eddbf57bd5bca05bf5f70b4247dc32e1ceb21cb`.

Original receipt SHA-256: `6eae63659f9ac2556d0e8b30b2cacfc6f11781b683223e820ca541335f11255c`.

Baseline dump SHA-256: `16b5b1eee3ce0ab6a1ced9ab66635efa6f42c17d0dc668c75c46cea88e613c09`.

Populated sanitized dump SHA-256: `ffba48796c8a11cfd4a6e168f21aa194cf37aaf7314d41c4ae62cafca8cea4ee`.

## 4. Failure history dan batas bukti

Failure awal tetap disimpan, bukan dilabel ulang sebagai pass. Koreksi yang terbukti meliputi: Windows daemon inherited PIPE; database comment `shobj_description`; parameter JSON/UUID/time casting; mandatory Auth shim columns; day alias/sort-order normalization; explicit grants; optional-field entity round-trip untuk freeze trigger; zero-fee payment no-op; batch verifier schema/typed arrays; pre/post-import backup confusion; PostgreSQL17/18 NOT NULL metadata representation; inventory pagination; browser Auth field redaction/logout JSON header; dan package-without-tests reporter classification.

Original staging harness gagal setelah commit karena per-row reconciliation terlalu lambat. Driver lama `phase6-stage-rehearsal.py` dan API-only `phase6-staging-acceptance.py` **disabled**, dipertahankan untuk provenance. Gunakan recovery/browser verifier baru; jangan resume driver lama atau mengulang import.

Strict receipt reconcile setelah browser awalnya gagal karena Supabase mengubah timestamp Auth. Mode `reconcile-after-acceptance` membuktikan hash kembali identik jika **hanya** last_sign_in_at dikembalikan null dan updated_at dikembalikan created_at pada satu acceptance Auth ID. Ia tidak mengizinkan perubahan public user/domain/password/metadata; receipt lama tidak diedit. Strict rollback/replay tidak memakai pengecualian ini.

## 5. Recovery, keamanan dan kelanjutan

Backup yang diuji adalah **sanitized public-state recovery**, tidak berisi audit/notifications/operations/leases/sessions/provider secrets. Auth recovery hanya metadata synthetic identities; password test dibuat baru pada local copy. Jangan menganggap backup ini memulihkan password/MFA/provider sessions atau seluruh state layanan Supabase.

Row rollback tidak memutar ulang sequence. Exact sequence restore terbukti lewat pg_dump ke database baru. Staging masih memuat accepted masked dataset sebagai retained rehearsal, dengan no grading scores fabricated dan session acceptance revoked. Akun acceptance credential tersimpan hanya di private artifact dari original run; jangan masukkan ke prompt, log, Git, deployment atau dokumen publik.

Prompt eksekusi Phase 7: `memory/PHASE-7-AI-PROMPT-2026-11-13.md`. **Fase7 dapat dimulai** untuk UAT/pilot preparation sesuai roadmap dan izin operasionalnya. AI berikutnya membaca closeout/handover terbaru, tidak mengulang fase1–6, tidak memperlakukan rejected/file scope sebagai lossless success, dan tidak melakukan Phase8 cutover/DNS switch tanpa keputusan baru. Migrasi penuh rejected history/files dan provider/Auth recovery produksi tetap membutuhkan desain, izin, serta gate terpisah sebelum cutover.
