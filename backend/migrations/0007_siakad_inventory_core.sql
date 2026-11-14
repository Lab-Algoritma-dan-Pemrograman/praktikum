-- Phase 5 only. No users, credentials, or domain fixtures are seeded here.
-- Aggregate tables use uniform indexed metadata plus typed JSON payloads.
do $$
declare name text;
begin
  foreach name in array array['classes','schedules','attendance_sessions','attendance_records','inventory_items','inventory_rentals','notifications','operations'] loop
    execute format('create table public.academic_%I (
      id text primary key check (length(id) between 1 and 128),
      parent_id text not null default '''', owner_id bigint not null default 0,
      scope jsonb not null check (jsonb_typeof(scope) = ''object''),
      status text not null default '''', unique_key text not null default '''',
      expires_at timestamptz, data jsonb not null check (jsonb_typeof(data) = ''object''))',name);
    execute format('create unique index academic_%I_unique_key on public.academic_%I(unique_key) where unique_key <> ''''',name,name);
    execute format('create index academic_%I_parent on public.academic_%I(parent_id,id)',name,name);
    execute format('create index academic_%I_owner on public.academic_%I(owner_id,id)',name,name);
    execute format('create index academic_%I_due on public.academic_%I(status,expires_at,id)',name,name);
    execute format('create index academic_%I_scope on public.academic_%I using gin(scope)',name,name);
    execute format('alter table public.academic_%I enable row level security',name);
    execute format('revoke all on public.academic_%I from public, anon, authenticated',name);
  end loop;
end $$;

alter table public.academic_classes
  add constraint academic_class_shift check(((data->>'shift')::int between 1 and 2) is true),
  add constraint academic_class_version check(((data->>'version')::bigint >= 1) is true);

alter table public.academic_schedules
  add constraint academic_schedule_class_fk foreign key(parent_id) references public.academic_classes(id),
  add constraint academic_schedule_day check((data->>'day_of_week' in ('Senin','Selasa','Rabu','Kamis','Jumat','Sabtu')) is true),
  add constraint academic_schedule_shift check(((data->>'shift')::int between 1 and 2) is true),
  add constraint academic_schedule_version check(((data->>'version')::bigint >= 1) is true);

alter table public.academic_attendance_sessions
  add constraint academic_att_session_schedule_fk foreign key(parent_id) references public.academic_schedules(id),
  add constraint academic_att_session_state check(status in ('open','closed')),
  add constraint academic_att_session_token_hash check((data->>'token_hash' ~ '^[0-9a-f]{64}$') is true),
  add constraint academic_att_session_version check(((data->'session'->>'version')::bigint >= 1) is true);

alter table public.academic_attendance_records
  add constraint academic_att_record_session_fk foreign key(parent_id) references public.academic_attendance_sessions(id),
  add constraint academic_att_record_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_att_record_state check(status in ('hadir','izin','alpha','terlambat')),
  add constraint academic_att_record_version check(((data->>'version')::bigint >= 1) is true),
  add constraint academic_att_record_unique unique(parent_id,owner_id);

alter table public.academic_inventory_items
  add constraint academic_item_state check(status in ('available','maintenance','retired')),
  add constraint academic_item_quantities check((
    (data->>'total_quantity')::int between 0 and 10000 and
    (data->>'available_quantity')::int between 0 and 10000 and
    (data->>'available_quantity')::int <= (data->>'total_quantity')::int and
    (data->>'rental_fee')::int between 0 and 100000000 and
    (data->>'version')::bigint >= 1
  ) is true);

alter table public.academic_inventory_rentals
  add constraint academic_rental_item_fk foreign key(parent_id) references public.academic_inventory_items(id),
  add constraint academic_rental_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_rental_state check(status in ('requested','approved','rejected','returned')),
  add constraint academic_rental_quantities check((
    (data->>'quantity')::int between 1 and 1000 and
    (data->>'total_fee')::bigint between 0 and 100000000000 and
    (data->>'version')::bigint >= 1
  ) is true);

alter table public.academic_notifications
  add constraint academic_notification_user_fk foreign key(owner_id) references public.users(id),
  add constraint academic_notification_state check(status in ('unread','read'));

create function public.academic_freeze_attendance_session() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  if (new.parent_id,new.scope,new.expires_at,new.data->>'token_hash',new.data->'session'->'opened_at',new.data->'session'->'expires_at')
    is distinct from
     (old.parent_id,old.scope,old.expires_at,old.data->>'token_hash',old.data->'session'->'opened_at',old.data->'session'->'expires_at') then
    raise exception 'attendance session identity and timer are immutable' using errcode='23514';
  end if;
  if old.status='closed' and (new.status<>'closed' or new.data->'session'->>'is_open'<>'false') then
    raise exception 'closed attendance session is terminal' using errcode='23514';
  end if;
  if (new.data->'session'->>'version')::bigint < (old.data->'session'->>'version')::bigint then
    raise exception 'attendance session version cannot decrease' using errcode='23514';
  end if;
  return new;
end $$;
create trigger academic_attendance_session_frozen before update on public.academic_attendance_sessions for each row execute function public.academic_freeze_attendance_session();

create function public.academic_freeze_attendance_record() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  if (new.parent_id,new.owner_id,new.data->>'session_id',(new.data->>'user_id')::bigint,new.data->'checked_in_at')
    is distinct from
     (old.parent_id,old.owner_id,old.data->>'session_id',(old.data->>'user_id')::bigint,old.data->'checked_in_at') then
    raise exception 'attendance record identity is immutable' using errcode='23514';
  end if;
  if (new.data->>'version')::bigint < (old.data->>'version')::bigint then
    raise exception 'attendance record version cannot decrease' using errcode='23514';
  end if;
  return new;
end $$;
create trigger academic_attendance_record_frozen before update on public.academic_attendance_records for each row execute function public.academic_freeze_attendance_record();

create function public.academic_freeze_rental() returns trigger
language plpgsql set search_path=pg_catalog,public as $$
begin
  if (new.parent_id,new.owner_id,new.scope,new.expires_at,(new.data->>'quantity')::int,(new.data->>'total_fee')::bigint,new.data->'requested_at')
    is distinct from
     (old.parent_id,old.owner_id,old.scope,old.expires_at,(old.data->>'quantity')::int,(old.data->>'total_fee')::bigint,old.data->'requested_at') then
    raise exception 'rental identity and quantity are immutable' using errcode='23514';
  end if;
  if new.status<>old.status and not (
     (old.status='requested' and new.status in ('approved','rejected')) or
     (old.status='approved' and new.status='returned')) then
    raise exception 'rental status transition is invalid' using errcode='23514';
  end if;
  if (old.status='rejected' and new.data is distinct from old.data) or
     (old.status='returned' and (new.data - 'payment_verified' - 'version') is distinct from (old.data - 'payment_verified' - 'version')) then
    raise exception 'terminal rental details are immutable' using errcode='23514';
  end if;
  if not (old.data->>'payment_verified')::boolean and (new.data->>'payment_verified')::boolean and new.status not in ('approved','returned') then
    raise exception 'only approved or returned rental payment can be verified' using errcode='23514';
  end if;
  if (old.data->>'payment_verified')::boolean and not (new.data->>'payment_verified')::boolean then
    raise exception 'verified payment cannot be unverified' using errcode='23514';
  end if;
  if (new.data->>'version')::bigint < (old.data->>'version')::bigint then
    raise exception 'rental version cannot decrease' using errcode='23514';
  end if;
  return new;
end $$;
create trigger academic_rental_frozen before update on public.academic_inventory_rentals for each row execute function public.academic_freeze_rental();

revoke all on function public.academic_freeze_attendance_session(),public.academic_freeze_attendance_record(),public.academic_freeze_rental() from public,anon,authenticated;

insert into public.role_permissions(role_code,permission_code,default_scope) values
 ('koordinator','schedule.write','{"domain":"siakad"}'::jsonb),
 ('koordinator','attendance.read','{"domain":"siakad"}'::jsonb),
 ('koordinator','inventory.manage','{"domain":"siakad"}'::jsonb),
 ('koordinator','rental.approve','{"domain":"siakad"}'::jsonb),
 ('koordinator','finance.manage','{"domain":"siakad"}'::jsonb)
on conflict(role_code,permission_code) do update set default_scope=excluded.default_scope;

insert into public.division_permissions(division_code,permission_code,default_scope) values
 ('sekretaris','attendance.read','{"domain":"siakad","division":"sekretaris"}'::jsonb)
on conflict(division_code,permission_code) do update set default_scope=excluded.default_scope;
