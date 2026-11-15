import { readFileSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { psqlJson } from './praktikum-prod-env.mjs';

const root = resolve(process.argv[2]);
const connection = JSON.parse(readFileSync(join(root, 'local-pg', 'connection.json'), 'utf8'));
const local = { ALGOHUB_MIGRATION_DATABASE_URL: connection.url };

// Mirror of entity.Scope: Intersect for effectiveGrant, Contains for policy.Check,
// and the production practicum_scope_matches() used by scoped list queries.
const functions = `
create or replace function pg_temp.fld(a jsonb, k text) returns text language sql immutable as $$
  select coalesce(nullif(a->>k, ''), '')
$$;
create or replace function pg_temp.shift_of(a jsonb) returns int language sql immutable as $$
  select case when coalesce(a->>'shift','') ~ '^[0-9]+$' then (a->>'shift')::int else 0 end
$$;
create or replace function pg_temp.contains(g jsonb, r jsonb) returns boolean language sql immutable as $$
  select coalesce((
    select bool_and(case f when 'shift'
      then pg_temp.shift_of(g) = 0 or pg_temp.shift_of(g) = pg_temp.shift_of(r)
      else pg_temp.fld(g, f) in ('', '*') or pg_temp.fld(g, f) = pg_temp.fld(r, f) end)
    from unnest(array['domain','institution','division','class_id','session_id','shift']) f), true)
$$;
create or replace function pg_temp.intersect(a jsonb, b jsonb) returns jsonb language sql immutable as $$
  select case
    when (pg_temp.fld(a,'domain') not in ('','*') and pg_temp.fld(b,'domain') not in ('','*') and pg_temp.fld(a,'domain') <> pg_temp.fld(b,'domain'))
      or (pg_temp.fld(a,'institution') not in ('','*') and pg_temp.fld(b,'institution') not in ('','*') and pg_temp.fld(a,'institution') <> pg_temp.fld(b,'institution'))
      or (pg_temp.fld(a,'division') not in ('','*') and pg_temp.fld(b,'division') not in ('','*') and pg_temp.fld(a,'division') <> pg_temp.fld(b,'division'))
      or (pg_temp.fld(a,'class_id') not in ('','*') and pg_temp.fld(b,'class_id') not in ('','*') and pg_temp.fld(a,'class_id') <> pg_temp.fld(b,'class_id'))
      or (pg_temp.fld(a,'session_id') not in ('','*') and pg_temp.fld(b,'session_id') not in ('','*') and pg_temp.fld(a,'session_id') <> pg_temp.fld(b,'session_id'))
      or (pg_temp.shift_of(a) <> 0 and pg_temp.shift_of(b) <> 0 and pg_temp.shift_of(a) <> pg_temp.shift_of(b))
    then null
    else jsonb_strip_nulls(jsonb_build_object(
      'domain', case when pg_temp.fld(a,'domain') in ('','*') then nullif(pg_temp.fld(b,'domain'),'') else nullif(pg_temp.fld(a,'domain'),'') end,
      'institution', case when pg_temp.fld(a,'institution') in ('','*') then nullif(pg_temp.fld(b,'institution'),'') else nullif(pg_temp.fld(a,'institution'),'') end,
      'division', case when pg_temp.fld(a,'division') in ('','*') then nullif(pg_temp.fld(b,'division'),'') else nullif(pg_temp.fld(a,'division'),'') end,
      'class_id', case when pg_temp.fld(a,'class_id') in ('','*') then nullif(pg_temp.fld(b,'class_id'),'') else nullif(pg_temp.fld(a,'class_id'),'') end,
      'session_id', case when pg_temp.fld(a,'session_id') in ('','*') then nullif(pg_temp.fld(b,'session_id'),'') else nullif(pg_temp.fld(a,'session_id'),'') end,
      'shift', case when pg_temp.shift_of(a)=0 then nullif(pg_temp.shift_of(b),0) else nullif(pg_temp.shift_of(a),0) end))
  end
$$;
-- effectiveGrant(): assignment scope ∩ default scope (∩ division for asisten).
create or replace function pg_temp.effective(assignment_scope jsonb, division text, default_scope jsonb, required_perm text, candidate_perm text) returns jsonb language sql immutable as $$
  select case
    when candidate_perm <> required_perm then null
    when pg_temp.intersect(assignment_scope, default_scope) is null then null
    when coalesce(division,'') <> '' and pg_temp.intersect(pg_temp.intersect(assignment_scope, default_scope), jsonb_build_object('division', division)) is null then null
    when coalesce(division,'') <> '' then pg_temp.intersect(pg_temp.intersect(assignment_scope, default_scope), jsonb_build_object('division', division))
    else pg_temp.intersect(assignment_scope, default_scope)
  end
$$;
`;

const measure = (permission, scopeExpr) => `
  count(distinct at.id) from public.practicum_activations at
  join public.user_role_assignments a on a.is_active
  join public.users u on u.id = a.user_id and u.is_active
  join (
    select rp.role_code, rp.permission_code, rp.default_scope from public.role_permissions rp
    union all
    select 'asisten', dp.permission_code, dp.default_scope from public.division_permissions dp
    union all
    select null, g.permission_code, g.scope from public.user_permission_grants g where g.is_active
  ) cand on (cand.role_code is null or cand.role_code = a.role_code)
  where cand.permission_code = '${permission}'
    and pg_temp.effective(a.scope, a.division_code, cand.default_scope, '${permission}', cand.permission_code) is not null
    and pg_temp.contains(pg_temp.effective(a.scope, a.division_code, cand.default_scope, '${permission}', cand.permission_code), ${scopeExpr})
`;

const columns = (scopeExpr, phase) => `
  '${phase}' as phase,
  (select ${measure('practicum.grade.write', scopeExpr)}) as grade,
  (select ${measure('practicum.session.manage', scopeExpr)}) as manage,
  (select ${measure('question.manage', scopeExpr)}) as questions,
  (select count(*) from public.practicum_activations at
     where public.practicum_scope_matches('{"domain":"praktikum"}'::jsonb, at.scope, false)) as scope_matches_domain
`;

const sql = `${functions}
select jsonb_agg(row_to_json(t)) from (
  select ${columns("(at.scope - 'domain' - 'session_id')", 'preRepair')}
  union all
  select ${columns('at.scope', 'postRepair')}
) t`;

try {
  const result = psqlJson(local, sql, { readOnly: false, label: 'Instructor authorization simulation' });
  const before = result.find((row) => row.phase === 'preRepair');
  const after = result.find((row) => row.phase === 'postRepair');
  const verdict = {
    grade: { before: before.grade, after: after.grade, delta: after.grade - before.grade },
    manage: { before: before.manage, after: after.manage, delta: after.manage - before.manage },
    questions: { before: before.questions, after: after.questions, delta: after.questions - before.questions },
    scopeMatchesDomain: { before: before.scope_matches_domain, after: after.scope_matches_domain }
  };
  writeFileSync(join(root, 'evidence', 'instructor-authorization.json'), JSON.stringify({ verdict, raw: result }, null, 2));
  console.log(JSON.stringify(verdict, null, 2));
  if (verdict.grade.delta < 0 || verdict.manage.delta < 0 || verdict.questions.delta < 0) {
    console.error('Instructor authorization regressed after repair.');
    process.exitCode = 3;
  }
} catch (error) { console.error(error.message); process.exitCode = 1; }
