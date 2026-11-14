-- Phase 4 only. No users, credentials, or curriculum fixtures are seeded here.
-- Aggregate tables use uniform indexed metadata plus typed JSON payloads.
do $$
declare name text;
begin
  foreach name in array array['levels','modules','lessons','progress','profiles','bug_hunt','game_history','operations','runner_leases'] loop
    execute format('create table public.learning_%I (
      id text primary key check (length(id) between 1 and 128),
      parent_id text not null default '''', owner_id bigint not null default 0,
      scope jsonb not null check (jsonb_typeof(scope) = ''object''),
      status text not null default '''', unique_key text not null default '''',
      expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = ''object''))',name);
    execute format('create unique index learning_%I_unique_key on public.learning_%I(unique_key) where unique_key <> ''''',name,name);
    execute format('create index learning_%I_parent on public.learning_%I(parent_id,id)',name,name);
    execute format('create index learning_%I_owner on public.learning_%I(owner_id,id)',name,name);
    execute format('create index learning_%I_due on public.learning_%I(status,expires_at,id)',name,name);
    execute format('create index learning_%I_scope on public.learning_%I using gin(scope)',name,name);
    execute format('alter table public.learning_%I enable row level security',name);
    execute format('revoke all on public.learning_%I from public, anon, authenticated',name);
  end loop;
end $$;

alter table public.learning_levels
  add constraint learning_level_access_mode check((data->>'access_mode' in ('auto','unlocked','locked')) is true),
  add constraint learning_level_sort check(((data->>'sort_order')::int between 1 and 1000) is true),
  add constraint learning_level_xp check(((data->>'required_xp')::int between 0 and 1000000) is true),
  add constraint learning_level_version check(((data->>'version')::bigint >= 1) is true);

alter table public.learning_modules
  add constraint learning_module_level_fk foreign key(parent_id) references public.learning_levels(id),
  add constraint learning_module_sort check(((data->>'sort_order')::int between 1 and 1000) is true),
  add constraint learning_module_version check(((data->>'version')::bigint >= 1) is true);

alter table public.learning_lessons
  add constraint learning_lesson_module_fk foreign key(parent_id) references public.learning_modules(id),
  add constraint learning_lesson_language check((data->>'language' in ('c','python')) is true),
  add constraint learning_lesson_xp check(((data->>'xp_reward')::int between 10 and 500) is true),
  add constraint learning_lesson_sort check(((data->>'sort_order')::int between 1 and 1000) is true),
  add constraint learning_lesson_version check(((data->>'version')::bigint >= 1) is true);

alter table public.learning_progress
  add constraint learning_progress_lesson_fk foreign key(parent_id) references public.learning_lessons(id),
  add constraint learning_progress_user_fk foreign key(owner_id) references public.users(id),
  add constraint learning_progress_state check(status in ('in_progress','completed')),
  add constraint learning_progress_revision check(((data->>'revision')::bigint >= 0 and (data->>'xp_awarded')::int between 0 and 500) is true),
  add constraint learning_progress_unique unique(parent_id,owner_id);

alter table public.learning_profiles
  add constraint learning_profile_user_fk foreign key(owner_id) references public.users(id),
  add constraint learning_profile_unique unique(owner_id),
  add constraint learning_profile_state check(status = 'active'),
  add constraint learning_profile_stats check(((data->>'xp')::int >= 0 and (data->>'level')::int >= 1 and (data->>'streak')::int >= 0 and (data->>'completed_lessons')::int >= 0 and (data->>'games_played')::int >= 0) is true);

alter table public.learning_bug_hunt
  add constraint learning_bughunt_language check((data->>'language' in ('c','python')) is true),
  add constraint learning_bughunt_difficulty check((data->>'difficulty' in ('easy','medium','hard')) is true),
  add constraint learning_bughunt_line check(((data->>'bug_line')::int between 1 and 500) is true),
  add constraint learning_bughunt_xp check(((data->>'xp_reward')::int between 10 and 300) is true),
  add constraint learning_bughunt_state check(status in ('active','inactive')),
  add constraint learning_bughunt_version check(((data->>'version')::bigint >= 1) is true);

alter table public.learning_game_history
  add constraint learning_game_question_fk foreign key(parent_id) references public.learning_bug_hunt(id),
  add constraint learning_game_user_fk foreign key(owner_id) references public.users(id),
  add constraint learning_game_state check(status = 'completed'),
  add constraint learning_game_xp check(((data->>'xp_earned')::int between 0 and 300) is true);

alter table public.learning_runner_leases
  add constraint learning_run_user_fk foreign key(owner_id) references public.users(id),
  add constraint learning_run_state check(status in ('running','finished'));

create function public.learning_freeze_progress() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  if (new.parent_id,new.owner_id,new.scope,new.data->>'lesson_id',(new.data->>'user_id')::bigint)
    is distinct from
     (old.parent_id,old.owner_id,old.scope,old.data->>'lesson_id',(old.data->>'user_id')::bigint) then
    raise exception 'progress identity is immutable' using errcode='23514';
  end if;
  if old.status='completed' and (
    new.status<>'completed' or
    (new.data->'completed_at',new.data->'xp_awarded') is distinct from (old.data->'completed_at',old.data->'xp_awarded')
  ) then
    raise exception 'completed lesson state and XP are immutable' using errcode='23514';
  end if;
  if (new.data->>'revision')::bigint < (old.data->>'revision')::bigint then
    raise exception 'progress revision cannot decrease' using errcode='23514';
  end if;
  return new;
end $$;
create trigger learning_progress_frozen before update on public.learning_progress for each row execute function public.learning_freeze_progress();

create function public.learning_freeze_profile() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  if new.owner_id <> old.owner_id or (new.data->>'user_id')::bigint <> (old.data->>'user_id')::bigint then
    raise exception 'profile owner is immutable' using errcode='23514';
  end if;
  if (new.data->>'xp')::int < (old.data->>'xp')::int or
     (new.data->>'completed_lessons')::int < (old.data->>'completed_lessons')::int or
     (new.data->>'games_played')::int < (old.data->>'games_played')::int then
    raise exception 'profile progression cannot decrease' using errcode='23514';
  end if;
  return new;
end $$;
create trigger learning_profile_frozen before update on public.learning_profiles for each row execute function public.learning_freeze_profile();

create function public.learning_freeze_game_history() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  raise exception 'game history is immutable' using errcode='23514';
end $$;
create trigger learning_game_history_frozen before update on public.learning_game_history for each row execute function public.learning_freeze_game_history();

revoke all on function public.learning_freeze_progress(),public.learning_freeze_profile(),public.learning_freeze_game_history() from public,anon,authenticated;

insert into public.role_permissions(role_code,permission_code,default_scope) values
 ('koordinator','learning.content.read','{"domain":"elearning"}'::jsonb),
 ('koordinator','learning.content.manage','{"domain":"elearning"}'::jsonb)
on conflict(role_code,permission_code) do update set default_scope=excluded.default_scope;
