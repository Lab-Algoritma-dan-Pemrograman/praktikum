-- Forward-only repair for operational metadata produced by legacy import.
-- Migration runner supplies the transaction. Never re-import or reset student work.
-- Only proven legacy rows are repaired; unexplained inconsistencies abort rollout.
lock table public.practicum_attempts in access exclusive mode;
lock table public.practicum_activations in share mode;

do $$
begin
  if not exists (select 1 from pg_trigger where tgrelid='public.practicum_attempts'::regclass
                 and tgname='practicum_attempt_frozen' and tgenabled='O') then
    raise exception 'expected enabled practicum attempt freeze trigger';
  end if;
  if exists (
    select 1 from public.practicum_attempts at
    left join public.practicum_activations ac on ac.id=at.parent_id
    left join public.practicum_sessions s on s.id=ac.parent_id
    where at.id like 'legacy-attempt-%' and (
      not coalesce(at.data->'legacy' ? 'pengerjaan_course', false)
      or ac.id is null or s.id is null
      or ac.data->'activation'->>'session_id' is distinct from ac.parent_id
      or at.data->'attempt'->>'activation_id' is distinct from at.parent_id
      or at.data->'attempt'->>'course_id' is distinct from ac.data->'activation'->>'course_id'
      or at.data->'attempt'->>'attempt_id' is distinct from at.id
      or (at.data->'attempt'->>'user_id')::bigint is distinct from at.owner_id
    )
  ) then
    raise exception 'legacy attempt ancestry cannot be established safely';
  end if;
end $$;

alter table public.practicum_attempts disable trigger practicum_attempt_frozen;
update public.practicum_attempts at
set scope=ac.scope,
    data=jsonb_set(jsonb_set(at.data, '{scope}', ac.scope),
                   '{attempt,session_id}', to_jsonb(ac.parent_id))
from public.practicum_activations ac
where at.parent_id=ac.id and at.id like 'legacy-attempt-%'
  and at.data->'legacy' ? 'pengerjaan_course'
  and (at.scope is distinct from ac.scope
       or at.data->'scope' is distinct from ac.scope
       or at.data->'attempt'->>'session_id' is distinct from ac.parent_id);
alter table public.practicum_attempts enable trigger practicum_attempt_frozen;

-- New writes cannot recreate row/JSON/activation ancestry drift. This does not
-- touch answer content, snapshots, grades, identities, revisions or deadlines.
create function public.practicum_check_attempt_metadata() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
declare
  parent public.practicum_activations%rowtype;
begin
  select * into parent from public.practicum_activations where id=new.parent_id;
  if not found
     or new.scope is distinct from parent.scope
     or new.data->'scope' is distinct from parent.scope
     or new.data->'attempt'->>'session_id' is distinct from parent.parent_id
     or new.data->'attempt'->>'activation_id' is distinct from new.parent_id
     or new.data->'attempt'->>'course_id' is distinct from parent.data->'activation'->>'course_id'
     or new.data->'attempt'->>'attempt_id' is distinct from new.id
     or (new.data->'attempt'->>'user_id')::bigint is distinct from new.owner_id then
    raise exception 'attempt metadata must match its activation and owner' using errcode='23514';
  end if;
  return new;
end $$;
revoke all on function public.practicum_check_attempt_metadata() from public,anon,authenticated;
create trigger practicum_attempt_metadata before insert or update on public.practicum_attempts
for each row execute function public.practicum_check_attempt_metadata();

do $$
begin
  if exists (
    select 1 from public.practicum_attempts at
    join public.practicum_activations ac on ac.id=at.parent_id
    where at.scope is distinct from ac.scope or at.data->'scope' is distinct from ac.scope
      or at.data->'attempt'->>'session_id' is distinct from ac.parent_id
      or at.data->'attempt'->>'activation_id' is distinct from at.parent_id
      or at.data->'attempt'->>'course_id' is distinct from ac.data->'activation'->>'course_id'
      or at.data->'attempt'->>'attempt_id' is distinct from at.id
      or (at.data->'attempt'->>'user_id')::bigint is distinct from at.owner_id
  ) then
    raise exception 'attempt metadata repair incomplete; transaction rolled back';
  end if;
end $$;
