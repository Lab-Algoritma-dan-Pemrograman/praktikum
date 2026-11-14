-- Phase 3 only. No users, credentials, or assessment fixtures are seeded here.
-- Typed JSON snapshots keep historical rules independent of mutable definitions;
-- indexed metadata, foreign keys and unique indexes enforce aggregate identity.
do $$
declare name text;
begin
  foreach name in array array['sessions','courses','questions','activations','attempts','grading_jobs','operations','runner_leases'] loop
    execute format('create table public.practicum_%I (
      id text primary key check (length(id) between 1 and 128),
      parent_id text not null default '''', owner_id bigint not null default 0,
      scope jsonb not null check (jsonb_typeof(scope) = ''object''),
      status text not null default '''', unique_key text not null default '''',
      expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = ''object''))',name);
    execute format('create unique index practicum_%I_unique_key on public.practicum_%I(unique_key) where unique_key <> ''''',name,name);
    execute format('create index practicum_%I_parent on public.practicum_%I(parent_id,id)',name,name);
    execute format('create index practicum_%I_owner on public.practicum_%I(owner_id,id)',name,name);
    execute format('create index practicum_%I_due on public.practicum_%I(status,expires_at,id)',name,name);
    execute format('create index practicum_%I_scope on public.practicum_%I using gin(scope)',name,name);
    execute format('alter table public.practicum_%I enable row level security',name);
    execute format('revoke all on public.practicum_%I from public, anon, authenticated',name);
  end loop;
end $$;

alter table public.practicum_courses add constraint practicum_course_session_fk foreign key(parent_id) references public.practicum_sessions(id),
  add constraint practicum_course_kind check((data->>'kind' in ('pretest','posttest','keterampilan','ujian_praktik')) is true),
  add constraint practicum_course_duration check(((data->>'duration_minutes')::int between 1 and 180) is true);
create unique index practicum_course_kind_unique on public.practicum_courses(parent_id,(data->>'kind'));
alter table public.practicum_questions add constraint practicum_question_course_fk foreign key(parent_id) references public.practicum_courses(id),
  add constraint practicum_question_type check((data->>'type' in ('essay','coding')) is true),
  add constraint practicum_question_points check(((data->>'points')::int between 1 and 100) is true);
alter table public.practicum_activations add constraint practicum_activation_session_fk foreign key(parent_id) references public.practicum_sessions(id),
  add constraint practicum_activation_timer check((data->'activation'->>'timer_mode' in ('shared','individual')) is true),
  add constraint practicum_activation_count check((jsonb_array_length(data->'questions') between 1 and 20) is true),
  add constraint practicum_activation_token_hash check((data->>'token_hash' = '' or data->>'token_hash' ~ '^[0-9a-f]{64}$') is true);
alter table public.practicum_attempts add constraint practicum_attempt_activation_fk foreign key(parent_id) references public.practicum_activations(id),
  add constraint practicum_attempt_user_fk foreign key(owner_id) references public.users(id),
  add constraint practicum_attempt_state check(status in ('in_progress','submitted','graded')),
  add constraint practicum_attempt_revision check(((data->'attempt'->>'revision')::bigint >= 0 and (data->'attempt'->>'grade_revision')::bigint >= 0) is true),
  add constraint practicum_attempt_unique unique(parent_id,owner_id);
alter table public.practicum_grading_jobs add constraint practicum_job_attempt_fk foreign key(parent_id) references public.practicum_attempts(id),
  add constraint practicum_job_attempt_unique unique(parent_id),
  add constraint practicum_job_state check(status in ('pending','running','needs_manual','graded'));
alter table public.practicum_runner_leases add constraint practicum_run_attempt_fk foreign key(parent_id) references public.practicum_attempts(id),
  add constraint practicum_run_user_fk foreign key(owner_id) references public.users(id),
  add constraint practicum_run_state check(status in ('running','finished'));

create function public.practicum_scope_matches(granted jsonb, resource jsonb, overlap boolean)
returns boolean language sql immutable parallel safe set search_path=pg_catalog as $$
 select not exists(select 1 from jsonb_each_text(granted) f
 where f.value not in ('','*') and not (f.key='shift' and f.value='0') and
   case when overlap then coalesce(resource->>f.key,'') not in ('','*',f.value)
     and not (f.key='shift' and resource->>f.key='0')
   else coalesce(resource->>f.key,'') <> f.value end)
$$;
revoke all on function public.practicum_scope_matches(jsonb,jsonb,boolean) from public,anon,authenticated;

create function public.practicum_freeze_activation() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
 if (new.parent_id,new.scope,new.data->'course',new.data->'questions',new.data->>'token_hash',
     new.data->'activation'->'student_ids',new.data->'activation'->'entry_approved_ids',
     new.data->'activation'->'question_ids',new.data->'activation'->'timer_mode',new.data->'activation'->'closes_at')
   is distinct from
    (old.parent_id,old.scope,old.data->'course',old.data->'questions',old.data->>'token_hash',
     old.data->'activation'->'student_ids',old.data->'activation'->'entry_approved_ids',
     old.data->'activation'->'question_ids',old.data->'activation'->'timer_mode',old.data->'activation'->'closes_at') then
   raise exception 'activation snapshot is immutable' using errcode='23514';
 end if;
 if old.data->'activation'->>'is_open'='false' and new.data->'activation'->>'is_open'<>'false' then
   raise exception 'closed activation is terminal' using errcode='23514';
 end if;
 if old.data->'activation'->>'started_at' is not null and
   (new.data->'activation'->'started_at',new.data->'activation'->'deadline_at') is distinct from
   (old.data->'activation'->'started_at',old.data->'activation'->'deadline_at') then
   raise exception 'activation timer is immutable' using errcode='23514';
 end if;
 return new;
end $$;
create trigger practicum_activation_frozen before update on public.practicum_activations for each row execute function public.practicum_freeze_activation();

create function public.practicum_freeze_attempt() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
 if (new.parent_id,new.owner_id,new.scope,new.data->'attempt'->'started_at',new.data->'attempt'->'deadline_at',new.data->'attempt'->'questions') is distinct from
    (old.parent_id,old.owner_id,old.scope,old.data->'attempt'->'started_at',old.data->'attempt'->'deadline_at',old.data->'attempt'->'questions') then
   raise exception 'attempt identity and timer are immutable' using errcode='23514';
 end if;
 if (old.status in ('submitted','graded') and new.status='in_progress') or (old.status='graded' and new.status<>'graded') then
   raise exception 'attempt transition is terminal' using errcode='23514';
 end if;
 if old.status<>'in_progress' and new.data->'submitted_answers' is distinct from old.data->'submitted_answers' then
   raise exception 'submitted answers are immutable' using errcode='23514';
 end if;
 -- Publication may add score/feedback and blank entries for unanswered questions,
 -- but may never rewrite, remove, or add student-authored answer content.
 if old.status<>'in_progress' and (
   exists (
     select 1 from jsonb_array_elements(old.data->'attempt'->'answers') prior
     where not exists (
       select 1 from jsonb_array_elements(new.data->'attempt'->'answers') current_answer
       where current_answer-'score'-'feedback' = prior-'score'-'feedback'
     )
   ) or exists (
     select 1 from jsonb_array_elements(new.data->'attempt'->'answers') current_answer
     where not exists (
       select 1 from jsonb_array_elements(old.data->'attempt'->'answers') prior
       where prior-'score'-'feedback'=current_answer-'score'-'feedback'
     ) and (
       exists (
         select 1 from jsonb_array_elements(old.data->'attempt'->'answers') prior
         where prior->>'question_id'=current_answer->>'question_id'
       ) or
       coalesce(current_answer->>'value','')<>'' or coalesce(current_answer->>'language','')<>'' or
       not exists (
         select 1 from jsonb_array_elements(old.data->'attempt'->'questions') question
         where question->>'id'=current_answer->>'question_id'
       )
     )
   )
 ) then
   raise exception 'submitted answer content is immutable' using errcode='23514';
 end if;
 if (new.data->'attempt'->>'revision')::bigint < (old.data->'attempt'->>'revision')::bigint or
    (new.data->'attempt'->>'grade_revision')::bigint < (old.data->'attempt'->>'grade_revision')::bigint then
   raise exception 'revision cannot decrease' using errcode='23514';
 end if;
 return new;
end $$;
create trigger practicum_attempt_frozen before update on public.practicum_attempts for each row execute function public.practicum_freeze_attempt();
revoke all on function public.practicum_freeze_activation(),public.practicum_freeze_attempt() from public,anon,authenticated;

insert into public.role_permissions(role_code,permission_code,default_scope) values
 ('koordinator','question.manage','{"domain":"praktikum"}'::jsonb),
 ('koordinator','practicum.grade.write','{"domain":"praktikum"}'::jsonb)
on conflict(role_code,permission_code) do update set default_scope=excluded.default_scope;
