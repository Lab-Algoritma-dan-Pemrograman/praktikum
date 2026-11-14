-- ============================================================
-- Target schema inventory (READ-ONLY)
-- Project: Algohub v2
--
-- Purpose: run this against the v2 TARGET database (staging or
-- prod target) to produce the same Section-1 shape as
-- legacy-schema-audit.sql, so both outputs can be diffed.
--
-- Usage:
--   psql "$TARGET_DATABASE_URL" -f target-schema-map.sql
--
-- Target schema reference: backend/migrations/0002-0007
--   * identity: users, roles, divisions, permissions,
--     role_permissions, division_permissions, user_role_assignments,
--     user_permission_grants, auth_sessions, auth_rate_limits,
--     sso_handshakes, audit_logs
--   * dynamic tables (created via DO blocks):
--       practicum_{sessions,courses,questions,activations,attempts,
--                  grading_jobs,operations,runner_leases}          (8)
--       learning_{levels,modules,lessons,progress,profiles,
--                 bug_hunt,game_history,operations,runner_leases}   (9)
--       academic_{classes,schedules,attendance_sessions,
--                 attendance_records,inventory_items,
--                 inventory_rentals,notifications,operations}       (8)
-- ============================================================

\set ON_ERROR_STOP off

\echo ''
\echo '==================== TARGET: TABLE INVENTORY ===================='
SELECT n.nspname AS schema,
       c.relname AS table_name,
       c.reltuples::bigint AS est_rows,
       (SELECT count(*) FROM information_schema.columns col
         WHERE col.table_schema = n.nspname AND col.table_name = c.relname) AS column_count,
       c.relrowsecurity AS rls_enabled
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  AND c.relkind IN ('r','p')
ORDER BY schema, table_name;

\echo ''
\echo '==================== TARGET: EXPECTED-TABLE PRESENCE CHECK ===================='
-- Lists any expected v2 table missing from this database.
-- Sources of expectation: backend/migrations 0005/0006/0007 arrays.
WITH expected(schema_name, table_name) AS (
  VALUES
    ('public','practicum_sessions'),('public','practicum_courses'),
    ('public','practicum_questions'),('public','practicum_activations'),
    ('public','practicum_attempts'),('public','practicum_grading_jobs'),
    ('public','practicum_operations'),('public','practicum_runner_leases'),
    ('public','learning_levels'),('public','learning_modules'),
    ('public','learning_lessons'),('public','learning_progress'),
    ('public','learning_profiles'),('public','learning_bug_hunt'),
    ('public','learning_game_history'),('public','learning_operations'),
    ('public','learning_runner_leases'),
    ('public','academic_classes'),('public','academic_schedules'),
    ('public','academic_attendance_sessions'),
    ('public','academic_attendance_records'),
    ('public','academic_inventory_items'),
    ('public','academic_inventory_rentals'),
    ('public','academic_notifications'),('public','academic_operations'),
    ('public','users'),('public','roles'),('public','divisions'),
    ('public','permissions'),('public','role_permissions'),
    ('public','division_permissions'),
    ('public','user_role_assignments'),
    ('public','user_permission_grants'),
    ('public','auth_sessions'),('public','auth_rate_limits'),
    ('public','sso_handshakes'),('public','audit_logs')
)
SELECT e.table_name AS missing_table
FROM expected e
LEFT JOIN pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  ON n.nspname = e.schema_name AND c.relname = e.table_name
WHERE c.oid IS NULL
ORDER BY e.table_name;

\echo ''
\echo '==================== TARGET: MIGRATION VERSION ===================='
-- Shows how far migrations have been applied, if a tracking table exists.
SELECT * FROM public.schema_migrations WHERE to_regclass('public.schema_migrations') IS NOT NULL;

\echo ''
\echo '==================== DONE ===================='
\echo 'Diff this output with legacy-schema-audit.sql results to produce the gap report.'
