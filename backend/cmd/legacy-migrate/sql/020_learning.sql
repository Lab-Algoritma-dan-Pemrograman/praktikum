-- Learning domain: levels/modules/lessons, per-user progress, profiles, game
-- history, and bug-hunt questions. Twin legacy tables (level/modul/materi/
-- game_soal) are preserved in legacy_preserve and intentionally not projected so
-- the live catalogue keeps exactly one canonical copy of each lesson.
drop view if exists legacy_levels, legacy_modules, legacy_lessons, legacy_progress, legacy_game;

create temp view legacy_levels as
select row_data, row_data->>'id' as legacy_id from legacy_preserve.rows where source_table = 'public.levels';

create temp view legacy_modules as
select row_data, row_data->>'id' as legacy_id from legacy_preserve.rows where source_table = 'public.modules';

create temp view legacy_lessons as
select row_data, row_data->>'id' as legacy_id from legacy_preserve.rows where source_table = 'public.lessons';

create temp view legacy_progress as
select row_data from legacy_preserve.rows where source_table = 'public.student_progress';

create temp view legacy_game as
select row_data from legacy_preserve.rows where source_table = 'public.game_history';

insert into public.learning_levels (id, parent_id, owner_id, scope, status, unique_key, data)
select l.legacy_id, '', 0, '{}'::jsonb, 'active', 'legacy:levels:' || l.legacy_id,
  jsonb_build_object(
    'id', l.legacy_id,
    'title', coalesce(nullif(btrim(l.row_data->>'title'), ''), l.legacy_id),
    'description', coalesce(l.row_data->>'description', ''),
    'sort_order', greatest(coalesce((l.row_data->>'sort_order')::int, 1), 1),
    'access_mode', case when l.row_data->>'access_mode' in ('auto', 'unlocked', 'locked')
                        then l.row_data->>'access_mode' else 'auto' end,
    'required_xp', least(greatest(coalesce((l.row_data->>'locked')::boolean = false, true)::int * 0, 0), 1000000),
    'unlocked', coalesce((l.row_data->>'locked')::boolean, false) = false,
    'completed_lessons', 0,
    'total_lessons', (select count(*) from legacy_lessons s
                      join legacy_modules m on m.legacy_id = s.row_data->>'module_id'
                      where m.row_data->>'level_id' = l.legacy_id),
    'scope', '{}'::jsonb,
    'version', 1,
    'created_at', now(),
    'updated_at', now(),
    'legacy', l.row_data
  )
from legacy_levels l
on conflict (id) do update set data = excluded.data, scope = excluded.scope, status = excluded.status;

insert into public.learning_modules (id, parent_id, owner_id, scope, status, unique_key, data)
select m.legacy_id, coalesce(m.row_data->>'level_id', ''), 0, '{}'::jsonb, 'active',
  'legacy:modules:' || m.legacy_id,
  jsonb_build_object(
    'id', m.legacy_id,
    'level_id', coalesce(m.row_data->>'level_id', ''),
    'title', coalesce(nullif(btrim(m.row_data->>'title'), ''), m.legacy_id),
    'description', '',
    'sort_order', greatest(coalesce((m.row_data->>'sort_order')::int, 1), 1),
    'completed_lessons', 0,
    'total_lessons', (select count(*) from legacy_lessons s where s.row_data->>'module_id' = m.legacy_id),
    'scope', '{}'::jsonb,
    'version', 1,
    'legacy', m.row_data
  )
from legacy_modules m
where exists (select 1 from public.learning_levels lv where lv.id = m.row_data->>'level_id')
on conflict (id) do update set parent_id = excluded.parent_id, data = excluded.data, status = excluded.status;

insert into public.learning_lessons (id, parent_id, owner_id, scope, status, unique_key, data)
select s.legacy_id, coalesce(s.row_data->>'module_id', ''), 0, '{}'::jsonb,
  case when s.row_data->>'exercise_type' = 'flowchart' then 'flowchart' else 'active' end,
  'legacy:lessons:' || s.legacy_id,
  jsonb_build_object(
    'id', s.legacy_id,
    'module_id', coalesce(s.row_data->>'module_id', ''),
    'level_id', coalesce(mm.row_data->>'level_id', ''),
    'title', coalesce(nullif(btrim(s.row_data->>'title'), ''), s.legacy_id),
    'sort_order', least(greatest(coalesce((s.row_data->>'sort_order')::int, 1), 1), 1000),
    'language', case when coalesce(mm.row_data->>'level_id', '') like 'py%'
                      or coalesce(s.row_data->>'module_id', '') like 'py%' then 'python' else 'c' end,
    'explanation', coalesce(s.row_data->>'explanation', ''),
    'code_example', coalesce(s.row_data->>'code_example', ''),
    'starter_code', coalesce(s.row_data->>'initial_code', ''),
    'solution', coalesce(s.row_data->>'solution', ''),
    'hint', coalesce(s.row_data->>'hint', ''),
    'xp_reward', least(greatest(coalesce((s.row_data->>'xp_reward')::int, 10), 10), 500),
    'quiz', case when jsonb_typeof(s.row_data->'quiz') = 'object' then s.row_data->'quiz' end,
    'test_cases', case when jsonb_typeof(s.row_data->'test_cases') = 'array' then s.row_data->'test_cases' else '[]'::jsonb end,
    'exercise_type', coalesce(s.row_data->>'exercise_type', 'code'),
    'flowchart_distractors', s.row_data->'flowchart_distractors',
    'validation_rules', s.row_data->'validation_rules',
    'scope', '{}'::jsonb,
    'version', 1,
    'legacy', s.row_data
  )
from legacy_lessons s
left join legacy_modules mm on mm.legacy_id = s.row_data->>'module_id'
where exists (select 1 from public.learning_modules md where md.id = s.row_data->>'module_id')
on conflict (id) do update set parent_id = excluded.parent_id, data = excluded.data, status = excluded.status;

insert into public.learning_progress (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-progress-' || coalesce(p.row_data->>'id', md5(p.row_data::text)),
  p.row_data->>'lesson_id',
  u.id,
  '{}'::jsonb,
  case when coalesce((p.row_data->>'completed')::boolean, false) then 'completed' else 'in_progress' end,
  'legacy:student_progress:' || coalesce(p.row_data->>'id', md5(p.row_data::text)),
  jsonb_build_object(
    'lesson_id', p.row_data->>'lesson_id',
    'module_id', coalesce(ls.data->>'module_id', ''),
    'level_id', coalesce(ls.data->>'level_id', ''),
    'user_id', u.id,
    'status', case when coalesce((p.row_data->>'completed')::boolean, false) then 'completed' else 'in_progress' end,
    'draft_code', '',
    'language', coalesce(ls.data->>'language', 'c'),
    'quiz_option', null,
    'test_passed', coalesce((p.row_data->>'completed')::boolean, false),
    'revision', 1,
    'xp_awarded', case when coalesce((p.row_data->>'completed')::boolean, false)
                       then coalesce((ls.data->>'xp_reward')::int, 0) else 0 end,
    'completed_at', case when coalesce((p.row_data->>'completed')::boolean, false)
                         then coalesce((p.row_data->>'completed_at')::timestamptz, now()) else null end,
    'updated_at', coalesce((p.row_data->>'completed_at')::timestamptz, now()),
    'legacy', p.row_data
  )
from legacy_progress p
join legacy_preserve.rows ur on ur.source_table = 'public.users' and ur.row_data->>'nim' = p.row_data->>'nim'
join public.users u on u.id = (ur.row_data->>'id')::bigint
join public.learning_lessons ls on ls.id = p.row_data->>'lesson_id'
on conflict (parent_id, owner_id) do nothing;

insert into public.learning_profiles (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-profile-' || u.id, '', u.id, '{}'::jsonb, 'active', 'legacy:profile:' || u.id,
  jsonb_build_object(
    'user_id', u.id,
    'user_name', u.name,
    'xp', greatest(coalesce((ur.row_data->>'xp')::int, 0), 0),
    'level', greatest(coalesce((ur.row_data->>'level')::int, 1), 1),
    'next_level_xp', 0,
    'streak', greatest(coalesce((ur.row_data->>'streak')::int, 0), 0),
    'last_active_date', coalesce(left(ur.row_data->>'last_active', 10), ''),
    'completed_lessons', (select count(*) from public.learning_progress lp
                          where lp.owner_id = u.id and lp.status = 'completed'),
    'games_played', (select count(*) from legacy_game g where g.row_data->>'nim' = u.nim),
    'perfect_games', 0,
    'unlocked_achievements', coalesce((
      select jsonb_agg(jsonb_build_object(
        'achievement_id', coalesce(a.row_data->>'achievement_id', ''),
        'unlocked_at', coalesce((a.row_data->>'unlocked_at')::timestamptz, now())))
      from legacy_preserve.rows a
      where a.source_table = 'public.unlocked_achievements' and a.row_data->>'nim' = u.nim), '[]'::jsonb),
    'timezone', 'Asia/Jakarta',
    'timezone_offset', '+0700',
    'updated_at', coalesce((ur.row_data->>'last_active')::timestamptz, now()),
    'legacy', jsonb_build_object(
      'study_time', ur.row_data->'study_time',
      'prestige_tier', ur.row_data->'prestige_tier',
      'prestige_mult', ur.row_data->'prestige_mult',
      'prestige_count', ur.row_data->'prestige_count',
      'last_prestige_at', ur.row_data->'last_prestige_at',
      'assessment_access', ur.row_data->'assessment_access',
      'level_access_overrides', ur.row_data->'level_access_overrides',
      'elearning_role', ur.row_data->'elearning_role')
  )
from legacy_preserve.rows ur
join public.users u on u.id = (ur.row_data->>'id')::bigint
where ur.source_table = 'public.users'
on conflict (owner_id) do nothing;

-- Legacy game history has no question reference, but learning_game_history.parent_id
-- is a NOT NULL foreign key. One explicit container row keeps the FK honest.
insert into public.learning_bug_hunt (id, parent_id, owner_id, scope, status, unique_key, data)
values ('legacy-game-history-container', '', 0, '{}'::jsonb, 'inactive', 'legacy:game-container',
  jsonb_build_object('id', 'legacy-game-history-container', 'language', 'c', 'difficulty', 'easy',
    'title', 'Legacy game history container', 'buggy_code', '', 'bug_line', 1,
    'explanation', 'Container row for imported legacy game history.',
    'xp_reward', 10, 'active', false, 'scope', '{}'::jsonb, 'version', 1))
on conflict (id) do nothing;

insert into public.learning_game_history (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-game-' || coalesce(g.row_data->>'id', md5(g.row_data::text)),
  'legacy-game-history-container',
  u.id,
  '{}'::jsonb,
  'completed',
  'legacy:game_history:' || coalesce(g.row_data->>'id', md5(g.row_data::text)),
  jsonb_build_object(
    'game_type', coalesce(g.row_data->>'game_type', ''),
    'xp_earned', least(greatest(coalesce((g.row_data->>'xp_earned')::int, 0), 0), 300),
    'played_at', coalesce((g.row_data->>'played_at')::timestamptz, now()),
    'legacy', g.row_data
  )
from legacy_game g
join legacy_preserve.rows ur on ur.source_table = 'public.users' and ur.row_data->>'nim' = g.row_data->>'nim'
join public.users u on u.id = (ur.row_data->>'id')::bigint
on conflict (id) do nothing;

insert into public.learning_bug_hunt (id, parent_id, owner_id, scope, status, unique_key, data)
select 'legacy-bughunt-' || coalesce(q.row_data->>'id', md5(q.row_data::text)),
  '', 0, '{}'::jsonb, 'active',
  'legacy:game_questions:' || coalesce(q.row_data->>'id', md5(q.row_data::text)),
  jsonb_build_object(
    'id', 'legacy-bughunt-' || coalesce(q.row_data->>'id', md5(q.row_data::text)),
    'language', case when q.row_data->>'language' = 'python' then 'python' else 'c' end,
    'difficulty', case when q.row_data->>'difficulty' in ('easy', 'medium', 'hard')
                       then q.row_data->>'difficulty' else 'medium' end,
    'title', coalesce(nullif(btrim(q.row_data->>'title'), ''), 'Bug hunt ' || coalesce(q.row_data->>'id', '')),
    'buggy_code', coalesce(q.row_data->>'code', ''),
    'bug_line', least(greatest(coalesce((q.row_data->>'bug_line')::int, 1), 1), 500),
    'explanation', coalesce(q.row_data->>'explanation', ''),
    'xp_reward', 10,
    'active', true,
    'scope', '{}'::jsonb,
    'version', 1,
    'legacy', q.row_data
  )
from legacy_preserve.rows q
where q.source_table = 'public.game_questions'
on conflict (id) do update set data = excluded.data, status = excluded.status;
