-- E-learning P1: prestige history, achievement catalog, weekly standings.
-- Same aggregate pattern as 0006: indexed metadata + typed JSON payloads.
-- Quarantined legacy data: prestige_history rows live in legacy_preserve.rows
-- and are NOT imported here. A later restore can insert them directly into
-- public.learning_prestige with owner_id = users.id and data = {"user_id":..,
-- "source":..,"amount":..,"reason":..,"created_at":..}; the usecase already
-- reads that shape (entity.LearningPrestigeEntry).
do $$
declare name text;
begin
  foreach name in array array['prestige','achievements','standings'] loop
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

alter table public.learning_prestige
  add constraint learning_prestige_user_fk foreign key(owner_id) references public.users(id),
  add constraint learning_prestige_state check(status = 'active'),
  add constraint learning_prestige_amount check(((data->>'amount')::int between 0 and 100000) is true);

alter table public.learning_achievements
  add constraint learning_achievement_state check(status in ('active','inactive')),
  add constraint learning_achievement_version check(((data->>'version')::bigint >= 1) is true),
  add constraint learning_achievement_reward check(((data->>'xp_reward')::int between 0 and 10000) is true);

alter table public.learning_standings
  add constraint learning_standings_state check(status = 'active');

revoke all on function public.learning_freeze_prestige(),public.learning_freeze_standings() from public,anon,authenticated;

insert into public.permissions (code,app,description) values
 ('learning.xp.manage','elearning','Mengelola XP dan prestige mahasiswa')
on conflict (code) do update set app = excluded.app, description = excluded.description;

insert into public.role_permissions (role_code, permission_code, default_scope)
values ('koordinator', 'learning.xp.manage', '{"domain":"elearning"}'::jsonb)
on conflict (role_code, permission_code) do update set default_scope = excluded.default_scope;
