-- 0009_activation_scope_shift.sql
--
-- Backfill the `shift` field into the embedded activation scope for
-- legacy-migrated practicum activations.
--
-- Background: the legacy migration (cmd/legacy-migrate/sql/030_practicum.sql)
-- wrote the requested shift into the activation row `scope` column but omitted
-- it from the duplicated `data->'activation'->'scope'` JSON. The eligibility
-- check (usecase/practicum.go practicumEligible -> Scope.Contains) reads the
-- JSON copy, so every student role assignment (which carries shift 1/2) failed
-- the Contains check and students saw no sessions, courses or grades.
--
-- The generator has been fixed, but rows already migrated still need this
-- backfill. It is idempotent: it only touches activations whose row scope has a
-- shift and whose JSON scope is still missing it, and it copies the value from
-- the authoritative row `scope` column.
--
-- The migration runner wraps this file in a transaction, so no begin/commit here.

update public.practicum_activations
set data = jsonb_set(
      data,
      '{activation,scope}',
      (data->'activation'->'scope') || jsonb_build_object('shift', (scope->>'shift')::int)
    )
where (scope ? 'shift')
  and not (data->'activation'->'scope' ? 'shift');

-- Every activation that had a row shift must now carry it in the JSON scope and
-- agree with the row column. Raises (and rolls back) otherwise.
do $$
declare
  bad int;
begin
  select count(*) into bad
  from public.practicum_activations
  where (scope ? 'shift')
    and (
      not (data->'activation'->'scope' ? 'shift')
      or (data->'activation'->'scope'->>'shift')::int <> (scope->>'shift')::int
    );
  if bad <> 0 then
    raise exception 'activation scope shift backfill incomplete: % rows', bad;
  end if;
end $$;
