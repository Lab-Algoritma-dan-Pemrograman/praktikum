-- ============================================================
-- Legacy schema audit (READ-ONLY)
-- Project: Algohub v2 cutover preparation
--
-- Purpose: run this against ONE legacy production database
-- (SIAKAD, E-learning, or Praktikum) to answer:
--   1. What tables exist and how big are they?
--   2. How many users, and how many duplicate NIM/email?
--   3. What are the max timestamps per key table (delta-import planning)?
--   4. What storage buckets exist?
--
-- Usage (no credentials hardcoded, pass via env or -d):
--   psql "postgresql://user:pass@host:5432/db" -f legacy-schema-audit.sql
--   psql "$LEGACY_DATABASE_URL" -f legacy-schema-audit.sql
--
-- Guarantees:
--   * Every query is SELECT-only against information_schema /
--     pg_catalog / app tables. No writes, no DDL, no extensions.
--   * Every section is guarded with to_regclass() so the script
--     degrades gracefully when a table does not exist in this
--     particular legacy database.
-- ============================================================

\set ON_ERROR_STOP off
\timing off

\echo ''
\echo '==================== SECTION 1: TABLE INVENTORY ===================='
SELECT n.nspname AS schema,
       c.relname AS table_name,
       CASE c.relkind WHEN 'r' THEN 'table' WHEN 'p' THEN 'partitioned' ELSE c.relkind::text END AS kind,
       c.reltuples::bigint AS est_rows,
       (SELECT count(*) FROM information_schema.columns col
         WHERE col.table_schema = n.nspname AND col.table_name = c.relname) AS column_count,
       c.relrowsecurity AS rls_enabled
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  AND c.relkind IN ('r','p')
ORDER BY est_rows DESC NULLS LAST, schema, table_name;

\echo ''
\echo '==================== SECTION 2: IDENTITY - USER COUNTS ===================='
-- SIAKAD-style users table (role/name/division aliases are legacy-known)
SELECT 'siakad.users' AS source, count(*) AS total_rows
FROM public.users WHERE to_regclass('public.users') IS NOT NULL
UNION ALL
SELECT 'elearning.profiles', count(*)
FROM public.profiles WHERE to_regclass('public.profiles') IS NOT NULL
UNION ALL
SELECT 'practicum users table (informational)', 0
WHERE false;

\echo ''
\echo '==================== SECTION 3: DUPLICATE NIM / EMAIL ===================='
-- Legacy finding to re-verify: 268 SIAKAD users with 22 duplicate NIMs.
-- Column names vary across legacy schemas (nim / username / identifier).
SELECT 'nim' AS id_column, COALESCE(nim,'(null)') AS value, count(*) AS occurrences
FROM public.users
WHERE to_regclass('public.users') IS NOT NULL AND nim IS NOT NULL
GROUP BY nim HAVING count(*) > 1
ORDER BY occurrences DESC, value
LIMIT 100;

\echo ''
\echo '==================== SECTION 4: KEY DOMAIN TABLE COUNTS ===================='
-- SIAKAD domain
SELECT 'siakad.jadwal' AS table_guess, count(*) AS rows FROM public.jadwal WHERE to_regclass('public.jadwal') IS NOT NULL
UNION ALL SELECT 'siakad.absensi', count(*) FROM public.absensi WHERE to_regclass('public.absensi') IS NOT NULL
UNION ALL SELECT 'siakad.inventaris', count(*) FROM public.inventaris WHERE to_regclass('public.inventaris') IS NOT NULL
UNION ALL SELECT 'siakad.rental', count(*) FROM public.rental WHERE to_regclass('public.rental') IS NOT NULL
UNION ALL SELECT 'elearning.levels', count(*) FROM public.levels WHERE to_regclass('public.levels') IS NOT NULL
UNION ALL SELECT 'elearning.modules', count(*) FROM public.modules WHERE to_regclass('public.modules') IS NOT NULL
UNION ALL SELECT 'elearning.lessons', count(*) FROM public.lessons WHERE to_regclass('public.lessons') IS NOT NULL
UNION ALL SELECT 'elearning.progress', count(*) FROM public.progress WHERE to_regclass('public.progress') IS NOT NULL
UNION ALL SELECT 'elearning.game', count(*) FROM public.game WHERE to_regclass('public.game') IS NOT NULL
UNION ALL SELECT 'elearning.achievement', count(*) FROM public.achievement WHERE to_regclass('public.achievement') IS NOT NULL;

\echo ''
\echo '==================== SECTION 5: MAX TIMESTAMPS (DELTA PLANNING) ===================='
SELECT 'siakad.users' AS table_guess, max(updated_at) AS max_updated, max(created_at) AS max_created
FROM public.users WHERE to_regclass('public.users') IS NOT NULL
UNION ALL
SELECT 'siakad.jadwal', max(updated_at), max(created_at) FROM public.jadwal WHERE to_regclass('public.jadwal') IS NOT NULL
UNION ALL
SELECT 'siakad.absensi', max(updated_at), max(created_at) FROM public.absensi WHERE to_regclass('public.absensi') IS NOT NULL
UNION ALL
SELECT 'elearning.progress', max(updated_at), max(created_at) FROM public.progress WHERE to_regclass('public.progress') IS NOT NULL
UNION ALL
SELECT 'elearning.lessons', max(updated_at), max(created_at) FROM public.lessons WHERE to_regclass('public.lessons') IS NOT NULL;

\echo ''
\echo '==================== SECTION 6: STORAGE BUCKETS ===================='
SELECT id, name, public, created_at, updated_at
FROM storage.buckets
WHERE to_regclass('storage.buckets') IS NOT NULL
ORDER BY id;

\echo ''
\echo '==================== DONE ===================='
\echo 'Save the output to scripts/prod-audit/results/<db-name>-<date>.txt'
\echo 'Diff against docs/phase-6-inventory.md and plan/database-migration.md findings.'
