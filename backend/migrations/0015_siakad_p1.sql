-- SIAKAD P1 additions: student leave/reschedule requests and the student
-- finance ledger (deposit/mutation history). Same aggregate shape as 0007 and
-- 0013: indexed metadata plus a typed JSON payload in `data`.
-- Presensi sessions/scans, schedules and equipment rentals already live in
-- academic_attendance_*, academic_schedules and academic_inventory_rentals
-- (0007) and are reused here; nothing is duplicated.
do $$
declare name text;
begin
  foreach name in array array['leave_requests','finance_mutations'] loop
    execute format('create table public.academic_%I (
      id text primary key check (length(id) between 1 and 128),
      parent_id text not null default '''', owner_id bigint not null default 0,
      scope jsonb not null check (jsonb_typeof(scope) = ''object''),
      status text not null default '''', unique_key text not null default '''',
      expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = ''object''))',name);
    execute format('create unique index academic_%I_unique_key on public.academic_%I(unique_key) where unique_key <> '''',name,name);
    execute format('create index academic_%I_parent on public.academic_%I(parent_id,id)',name,name);
    execute format('create index academic_%I_owner on public.academic_%I(owner_id,id)',name,name);
    execute format('create index academic_%I_due on public.academic_%I(status,expires_at,id)',name,name);
    execute format('create index academic_%I_scope on public.academic_%I using gin(scope)',name,name);
    execute format('alter table public.academic_%I enable row level security',name);
    execute format('revoke all on public.academic_%I from public, anon, authenticated',name);
  end loop;
end $$;

alter table public.academic_leave_requests
  add constraint academic_leave_request_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_leave_request_state check(status in ('pending','approved','rejected','rescheduled')),
  add constraint academic_leave_request_kind check((data->>'kind' in ('izin','sakit','reschedule')) is true),
  add constraint academic_leave_request_reason check((length(data->>'reason') between 1 and 500) is true);

alter table public.academic_finance_mutations
  add constraint academic_finance_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_finance_state check(status = 'posted'),
  add constraint academic_finance_type check((data->>'type' in ('income','expense','adjustment')) is true),
  add constraint academic_finance_amount check(((data->>'amount')::bigint between 0 and 100000000000) is true),
  add constraint academic_finance_note check((length(data->>'note') between 1 and 500) is true);

-- One live leave request per student per attendance session; decided requests
-- keep their own row so the history is immutable.
create unique index academic_leave_request_live
  on public.academic_leave_requests(parent_id,owner_id)
  where status = 'pending';

-- One open reschedule slot per approved request.
create unique index academic_leave_request_reschedule_slot
  on public.academic_leave_requests(unique_key)
  where unique_key <> '';

insert into public.permissions (code,app,description) values
 ('leave.decide','siakad','Memutuskan pengajuan izin dan reschedule')
on conflict (code) do update set app = excluded.app, description = excluded.description;

insert into public.role_permissions (role_code,permission_code,default_scope) values
 ('koordinator','leave.decide','{"domain":"siakad"}'::jsonb)
on conflict(role_code,permission_code) do update set default_scope=excluded.default_scope;

insert into public.division_permissions(division_code,permission_code,default_scope) values
 ('sekretaris','leave.decide','{"domain":"siakad","division":"sekretaris"}'::jsonb)
on conflict(division_code,permission_code) do update set default_scope=excluded.default_scope;
