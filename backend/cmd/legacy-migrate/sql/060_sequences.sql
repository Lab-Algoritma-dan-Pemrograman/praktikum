-- Legacy rows arrive with explicit ids, so every identity sequence has to be
-- moved past the imported maximum. Without this the first freshly created row
-- collides with an imported id and the API answers 409 on login.
do $$
declare
  target record;
  max_id bigint;
  seq text;
begin
  for target in
    select * from (values
      ('public.users', 'id'),
      ('public.user_role_assignments', 'id'),
      ('public.user_permission_grants', 'id'),
      ('public.audit_logs', 'id')
    ) as t(tbl, col)
  loop
    if to_regclass(target.tbl) is null then
      continue;
    end if;
    seq := pg_get_serial_sequence(target.tbl, target.col);
    if seq is null then
      continue;
    end if;
    execute format('select coalesce(max(%I), 0) from %s', target.col, target.tbl) into max_id;
    if max_id > 0 then
      perform setval(seq, greatest(max_id, (select coalesce(last_value, 0) from pg_sequences where schemaname = 'public' and sequencename = split_part(seq, '.', 2))), true);
    end if;
  end loop;
end $$;
