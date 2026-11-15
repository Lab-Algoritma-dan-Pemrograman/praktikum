-- Forward-only canonical context repair. The migration runner supplies one transaction.
-- No grants, shared-session class/shift inference, snapshots or student work changes.
select pg_advisory_xact_lock(731903005);
lock table public.practicum_sessions, public.practicum_courses, public.practicum_questions,
           public.practicum_activations, public.practicum_attempts in access exclusive mode;

do $$
begin
  if exists (
    select 1 from (values
      ('public.practicum_activations', 'practicum_activation_frozen', 'public.practicum_freeze_activation()'),
      ('public.practicum_attempts', 'practicum_attempt_frozen', 'public.practicum_freeze_attempt()'),
      ('public.practicum_attempts', 'practicum_attempt_metadata', 'public.practicum_check_attempt_metadata()')
    ) required(relation, trigger_name, function_name)
    where not exists (select 1 from pg_trigger t
      where t.tgrelid=required.relation::regclass and t.tgname=required.trigger_name
        and t.tgfoid=required.function_name::regprocedure and t.tgenabled='O' and not t.tgisinternal)
  ) then
    raise exception 'legacy scope repair requires enabled freeze and metadata triggers';
  end if;
end $$;

-- Merge only compatible evidence; an absent value cannot erase a restriction.
-- These transaction-local helpers are removed below, not installed as application APIs.
create function pg_temp.practicum_merge_scope(a jsonb, b jsonb) returns jsonb
language plpgsql as $$
declare k text; v jsonb;
begin
  if jsonb_typeof(a) is distinct from 'object' or jsonb_typeof(b) is distinct from 'object' then
    raise exception 'legacy scope must be an object';
  end if;
  for k,v in select key,value from jsonb_each(b) loop
    if a ? k and a->k not in ('null'::jsonb, '""'::jsonb)
       and v not in ('null'::jsonb, '""'::jsonb) and a->k is distinct from v then
      raise exception 'conflicting legacy scope restriction: %', k;
    end if;
    if not (a ? k) or a->k in ('null'::jsonb, '""'::jsonb) then
      a := a || jsonb_build_object(k,v);
    end if;
  end loop;
  return a;
end $$;

create temp table practicum_scope_repair (
  kind text not null, id text not null, session_id text not null, scope jsonb not null,
  primary key(kind,id)
) on commit drop;

do $$
declare r record; target jsonb;
begin
  for r in select * from public.practicum_sessions where id like 'legacy-session-%' loop
    if r.id is distinct from 'legacy-session-' || (r.data->'legacy'->>'id')
       or r.data->>'id' is distinct from r.id then
      raise exception 'unproven legacy session identity: %', r.id;
    end if;
    target := pg_temp.practicum_merge_scope(r.scope, r.data->'scope');
    target := pg_temp.practicum_merge_scope(target, jsonb_build_object('domain','praktikum','session_id',r.id));
    insert into practicum_scope_repair values ('session',r.id,r.id,target);
  end loop;

  for r in select c.*, s.id as session_id, p.scope as parent_scope
    from public.practicum_courses c
    left join public.practicum_sessions s on s.id=c.parent_id
    left join practicum_scope_repair p on p.kind='session' and p.id=s.id
    where c.id like 'legacy-course-%' loop
    if r.parent_scope is null or r.data->>'id' is distinct from r.id
       or r.id is distinct from 'legacy-course-' || (r.data->'legacy'->>'id')
       or r.data->>'session_id' is distinct from r.session_id
       or r.session_id is distinct from 'legacy-session-' || (r.data->'legacy'->>'sesi_praktikum_id') then
      raise exception 'unproven legacy course ancestry: %', r.id;
    end if;
    target := pg_temp.practicum_merge_scope(r.parent_scope,r.scope);
    if target is distinct from r.parent_scope then
      raise exception 'legacy course has independent scope restriction: %', r.id;
    end if;
    insert into practicum_scope_repair values ('course',r.id,r.session_id,target);
  end loop;

  for r in select q.*, p.session_id, p.scope as parent_scope
    from public.practicum_questions q
    left join practicum_scope_repair p on p.kind='course' and p.id=q.parent_id
    where q.id like 'legacy-question-%' loop
    if r.parent_scope is null or r.data->>'id' is distinct from r.id
       or r.id is distinct from 'legacy-question-' || (r.data->'legacy'->>'id')
       or r.data->>'course_id' is distinct from r.parent_id
       or r.parent_id is distinct from 'legacy-course-' || (r.data->'legacy'->>'course_id') then
      raise exception 'unproven legacy question ancestry: %', r.id;
    end if;
    target := pg_temp.practicum_merge_scope(r.parent_scope,r.scope);
    if target is distinct from r.parent_scope then
      raise exception 'legacy question has independent scope restriction: %', r.id;
    end if;
    insert into practicum_scope_repair values ('question',r.id,r.session_id,target);
  end loop;

  for r in select ac.*, c.parent_id as course_session, p.scope as parent_scope
    from public.practicum_activations ac
    left join public.practicum_courses c on c.id=ac.data->'activation'->>'course_id'
    left join practicum_scope_repair p on p.kind='session' and p.id=ac.parent_id
    where ac.id like 'legacy-activation-%' loop
    if r.parent_scope is null or r.course_session is distinct from r.parent_id
       or r.data->'activation'->>'id' is distinct from r.id
       or r.data->'activation'->>'session_id' is distinct from r.parent_id
       or r.id is distinct from 'legacy-activation-' || (r.data#>>'{legacy,aktivasi_course,id}')
       or r.parent_id is distinct from 'legacy-session-' || (r.data#>>'{legacy,aktivasi_sesi,sesi_praktikum_id}')
       or r.data#>>'{legacy,aktivasi_course,aktivasi_sesi_id}' is distinct from r.data#>>'{legacy,aktivasi_sesi,id}'
       or r.data->'activation'->>'course_id' is distinct from 'legacy-course-' || (r.data#>>'{legacy,aktivasi_course,course_id}') then
      raise exception 'unproven legacy activation ancestry: %', r.id;
    end if;
    target := pg_temp.practicum_merge_scope(r.scope,r.data#>'{activation,scope}');
    target := pg_temp.practicum_merge_scope(target,r.parent_scope);
    target := pg_temp.practicum_merge_scope(target,jsonb_strip_nulls(jsonb_build_object(
      'class_id', 'legacy-class-' || (r.data#>>'{legacy,aktivasi_sesi,kelas_id}'),
      'shift', case when r.data#>>'{legacy,aktivasi_sesi,shift}' ~ '^[0-9]+$'
                    then (r.data#>>'{legacy,aktivasi_sesi,shift}')::int end)));
    insert into practicum_scope_repair values ('activation',r.id,r.parent_id,target);
  end loop;

  for r in select at.*, ac.parent_id as session_id, ac.scope as activation_scope,
      ac.data as activation_data, p.scope as target_scope
    from public.practicum_attempts at
    left join public.practicum_activations ac on ac.id=at.parent_id
    left join practicum_scope_repair p on p.kind='activation' and p.id=ac.id
    where at.id like 'legacy-attempt-%' or p.id is not null loop
    if r.target_scope is null
       or r.id is distinct from 'legacy-attempt-' || (r.data#>>'{legacy,pengerjaan_course,id}')
       or r.data#>>'{attempt,attempt_id}' is distinct from r.id
       or r.data#>>'{attempt,activation_id}' is distinct from r.parent_id
       or r.data#>>'{attempt,course_id}' is distinct from r.activation_data#>>'{activation,course_id}'
       or (r.data#>>'{attempt,user_id}')::bigint is distinct from r.owner_id
       or (r.data#>>'{legacy,pengerjaan_course,mahasiswa_id}')::bigint is distinct from r.owner_id
       or 'legacy-course-' || (r.data#>>'{legacy,pengerjaan_course,course_id}') is distinct from r.activation_data#>>'{activation,course_id}'
       or r.data#>>'{legacy,pengerjaan_course,aktivasi_sesi_id}' is distinct from r.activation_data#>>'{legacy,aktivasi_sesi,id}'
       -- 0011 must already have repaired the historical attempt drift.
       or r.data#>>'{attempt,session_id}' is distinct from r.session_id
       or r.scope is distinct from r.activation_scope or r.data->'scope' is distinct from r.activation_scope then
      raise exception 'unproven legacy attempt ancestry or missing 0011 repair: %', r.id;
    end if;
    if r.status='in_progress' and
       (coalesce((r.data#>>'{attempt,deadline_at}')::timestamptz,'infinity') > now()
        or r.expires_at > now()) then
      raise exception 'live legacy attempt blocks scope repair: %', r.id;
    end if;
    insert into practicum_scope_repair values ('attempt',r.id,r.session_id,r.target_scope);
  end loop;
end $$;

-- Only these frozen fields need a controlled historical correction. The metadata
-- invariant stays enabled and checks every attempt against its corrected activation.
alter table public.practicum_activations disable trigger practicum_activation_frozen;
alter table public.practicum_attempts disable trigger practicum_attempt_frozen;
update public.practicum_sessions s
set scope=p.scope, data=jsonb_set(s.data,'{scope}',p.scope)
from practicum_scope_repair p where p.kind='session' and p.id=s.id
  and (s.scope is distinct from p.scope or s.data->'scope' is distinct from p.scope);
update public.practicum_courses c set scope=p.scope
from practicum_scope_repair p where p.kind='course' and p.id=c.id and c.scope is distinct from p.scope;
update public.practicum_questions q set scope=p.scope
from practicum_scope_repair p where p.kind='question' and p.id=q.id and q.scope is distinct from p.scope;
update public.practicum_activations ac
set scope=p.scope, data=jsonb_set(ac.data,'{activation,scope}',p.scope)
from practicum_scope_repair p where p.kind='activation' and p.id=ac.id
  and (ac.scope is distinct from p.scope or ac.data#>'{activation,scope}' is distinct from p.scope);
update public.practicum_attempts at
set scope=p.scope, data=jsonb_set(jsonb_set(at.data,'{scope}',p.scope),'{attempt,session_id}',to_jsonb(p.session_id))
from practicum_scope_repair p where p.kind='attempt' and p.id=at.id
  and (at.scope is distinct from p.scope or at.data->'scope' is distinct from p.scope
       or at.data#>>'{attempt,session_id}' is distinct from p.session_id);
alter table public.practicum_activations enable trigger practicum_activation_frozen;
alter table public.practicum_attempts enable trigger practicum_attempt_frozen;

do $$
begin
  if exists (
    select 1 from public.practicum_attempts at
    left join public.practicum_activations ac on ac.id=at.parent_id
    where ac.id is null or at.scope is distinct from ac.scope or at.data->'scope' is distinct from ac.scope
      or at.data#>>'{attempt,session_id}' is distinct from ac.parent_id
      or at.data#>>'{attempt,activation_id}' is distinct from ac.id
      or at.data#>>'{attempt,course_id}' is distinct from ac.data#>>'{activation,course_id}'
      or at.data#>>'{attempt,attempt_id}' is distinct from at.id
      or (at.data#>>'{attempt,user_id}')::bigint is distinct from at.owner_id
  ) then
    raise exception 'attempt metadata invariant violated after legacy scope repair';
  end if;
end $$;
drop function pg_temp.practicum_merge_scope(jsonb,jsonb);
drop table practicum_scope_repair;
