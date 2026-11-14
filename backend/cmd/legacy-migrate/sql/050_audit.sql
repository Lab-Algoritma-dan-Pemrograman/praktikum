-- Audit domain: legacy audit_logs (int id), activity_logs and siakad_audit_logs
-- (uuid id) all land in public.audit_logs. Uuid sources get a stable numeric id
-- derived from md5 so reruns are byte-identical and cannot collide with the
-- integer ids used by the other two sources. The original row always travels in
-- detail.row.
insert into public.audit_logs (id, actor_id, action, app, subject_type, subject_id, scope, request_id, operation_id, detail, created_at)
select
  a.legacy_id,
  case when exists (select 1 from public.users u where u.id = nullif(a.row_data->>'user_id', '')::bigint)
       then nullif(a.row_data->>'user_id', '')::bigint end,
  coalesce(nullif(btrim(a.row_data->>'action'), ''), 'legacy.event'),
  'legacy',
  'legacy.audit_logs',
  a.legacy_id::text,
  '{}'::jsonb,
  '',
  'legacy:audit_logs:' || a.legacy_id,
  jsonb_build_object('legacy_table', 'audit_logs', 'nim', a.row_data->'nim', 'nama', a.row_data->'nama',
    'role', a.row_data->'role', 'description', a.row_data->'description', 'ip_address', a.row_data->'ip_address',
    'user_agent', a.row_data->'user_agent', 'row', a.row_data),
  coalesce((a.row_data->>'created_at')::timestamptz, now())
from (
  select (row_data->>'id')::bigint as legacy_id, row_data
  from legacy_preserve.rows where source_table = 'public.audit_logs'
) a
on conflict (id) do nothing;

insert into public.audit_logs (id, actor_id, action, app, subject_type, subject_id, scope, request_id, operation_id, detail, created_at)
select
  b.legacy_id,
  case when exists (select 1 from public.users u where u.id = b.user_id) then b.user_id end,
  coalesce(nullif(btrim(b.row_data->>'event_type'), ''), 'legacy.activity'),
  'legacy',
  'legacy.activity_logs',
  b.source_id,
  '{}'::jsonb,
  '',
  'legacy:activity_logs:' || b.source_id,
  jsonb_build_object('legacy_table', 'activity_logs', 'nim', b.row_data->'nim', 'nama', b.row_data->'nama',
    'details', b.row_data->'details', 'ip_address', b.row_data->'ip_address', 'row', b.row_data),
  coalesce((b.row_data->>'timestamp')::timestamptz, now())
from (
  select ('x' || substr(md5('activity:' || (r.row_data->>'id')), 1, 15))::bit(60)::bigint as legacy_id,
         r.row_data->>'id' as source_id,
         r.row_data,
         (select u.id from public.users u where u.nim = nullif(r.row_data->>'nim', '') limit 1) as user_id
  from legacy_preserve.rows r where r.source_table = 'public.activity_logs'
) b
on conflict (id) do nothing;

insert into public.audit_logs (id, actor_id, action, app, subject_type, subject_id, scope, request_id, operation_id, detail, created_at)
select
  c.legacy_id,
  case when exists (select 1 from public.users u where u.id = c.actor_id) then c.actor_id end,
  coalesce(nullif(btrim(c.row_data->>'action_type'), ''), 'legacy.siakad'),
  'siakad',
  'legacy.siakad_audit_logs',
  c.source_id,
  '{}'::jsonb,
  '',
  'legacy:siakad_audit_logs:' || c.source_id,
  jsonb_build_object('legacy_table', 'siakad_audit_logs', 'actor_name', c.row_data->'actor_name',
    'description', c.row_data->'description', 'payload', c.row_data->'payload', 'row', c.row_data),
  coalesce((c.row_data->>'created_at')::timestamptz, now())
from (
  select ('x' || substr(md5('siakad:' || (r.row_data->>'id')), 1, 15))::bit(60)::bigint as legacy_id,
         r.row_data->>'id' as source_id,
         r.row_data,
         nullif(r.row_data->>'actor_id', '')::bigint as actor_id
  from legacy_preserve.rows r where r.source_table = 'public.siakad_audit_logs'
) c
on conflict (id) do nothing;
