-- 0010_activation_roster_backfill.sql
--
-- Add attempt owners back into their activation's student_ids roster.
--
-- Background: the legacy migration built `student_ids` from the `keanggotaan`
-- (membership) table filtered by class + shift at migration time. Some students
-- took a course while their shift differed from their final membership (the
-- legacy data had a shift fix, `_bak_users_20260929_shiftfix`), so their own
-- practicum attempts exist while they are absent from the activation roster.
-- practicumEligible then rejects their own attempts, hiding their grades.
--
-- Evidence of "did the work" is the attempt row itself (owner_id + parent
-- activation). Adding every attempt owner to its activation roster is
-- idempotent and cannot over-grant: it only restores access to work the student
-- already submitted.
--
-- `student_ids` is protected by the BEFORE UPDATE trigger
-- `practicum_activation_frozen` ("eligibility frozen at activation"). This is a
-- controlled historical backfill by the database owner, so the trigger is
-- disabled only for the duration of this statement inside the migration
-- transaction; it is re-enabled before the transaction commits and Postgres
-- restores it automatically on rollback.
--
-- The migration runner wraps this file in a transaction; no begin/commit here.

alter table public.practicum_activations disable trigger practicum_activation_frozen;

update public.practicum_activations a
set data = jsonb_set(
      a.data,
      '{activation,student_ids}',
      (
        select jsonb_agg(distinct v order by v)
        from (
          select (jsonb_array_elements_text(a.data->'activation'->'student_ids'))::bigint as v
          union
          select at.owner_id
          from public.practicum_attempts at
          where at.parent_id = a.id
        ) roster
      )
    )
where exists (
  select 1
  from public.practicum_attempts at
  where at.parent_id = a.id
    and not (a.data->'activation'->'student_ids' @> jsonb_build_array(at.owner_id))
);

alter table public.practicum_activations enable trigger practicum_activation_frozen;

-- Verify: no activation may still be missing an owner who has an attempt on it.
do $$
declare
  missing int;
begin
  select count(*) into missing
  from public.practicum_activations a
  join public.practicum_attempts at on at.parent_id = a.id
  where not (a.data->'activation'->'student_ids' @> jsonb_build_array(at.owner_id));
  if missing <> 0 then
    raise exception 'activation roster backfill incomplete: % attempt(s) still unrostered', missing;
  end if;
end $$;
