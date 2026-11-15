package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPostgresPracticumLegacyScopeContext(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	body, err := os.ReadFile("../../migrations/0012_practicum_legacy_scope_context.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(h.ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	// Synthetic, rollback-only copies model the post-0011 legacy shape. Existing
	// runtime rows remain present, proving that the migration leaves them alone.
	_, err = tx.Exec(h.ctx, `
insert into public.practicum_sessions (id,parent_id,owner_id,scope,status,unique_key,expires_at,data)
select 'legacy-session-912001',parent_id,owner_id,'{}',status,'scope-test-session',expires_at,
 data || '{"id":"legacy-session-912001","scope":{},"legacy":{"id":912001}}'
from public.practicum_sessions where id=$1;
insert into public.practicum_courses (id,parent_id,owner_id,scope,status,unique_key,expires_at,data)
select 'legacy-course-912002','legacy-session-912001',owner_id,'{}',status,'scope-test-course',expires_at,
 data || '{"id":"legacy-course-912002","session_id":"legacy-session-912001","legacy":{"id":912002,"sesi_praktikum_id":912001}}'
from public.practicum_courses where id=$2;
insert into public.practicum_questions (id,parent_id,owner_id,scope,status,unique_key,expires_at,data)
select 'legacy-question-912003','legacy-course-912002',owner_id,'{}',status,'scope-test-question',expires_at,
 data || '{"id":"legacy-question-912003","course_id":"legacy-course-912002","legacy":{"id":912003,"course_id":912002}}'
from public.practicum_questions where id=$3;
insert into public.practicum_activations (id,parent_id,owner_id,scope,status,unique_key,expires_at,data)
select 'legacy-activation-912004','legacy-session-912001',owner_id,
 '{"class_id":"legacy-class-912007","shift":1,"division":"independent"}',status,'scope-test-activation',expires_at,
 jsonb_set(data,'{activation}',data->'activation' ||
 '{"id":"legacy-activation-912004","session_id":"legacy-session-912001","course_id":"legacy-course-912002","scope":{"class_id":"legacy-class-912007","shift":1,"division":"independent"}}')
 || '{"legacy":{"aktivasi_course":{"id":912004,"course_id":912002,"aktivasi_sesi_id":912006},"aktivasi_sesi":{"id":912006,"sesi_praktikum_id":912001,"kelas_id":912007,"shift":1}}}'
from public.practicum_activations where id=$4;
insert into public.practicum_attempts (id,parent_id,owner_id,scope,status,unique_key,expires_at,data)
select 'legacy-attempt-912005','legacy-activation-912004',owner_id,
 '{"class_id":"legacy-class-912007","shift":1,"division":"independent"}','submitted','scope-test-attempt','2000-01-01',
 jsonb_set(data,'{attempt}',data->'attempt' ||
 '{"attempt_id":"legacy-attempt-912005","activation_id":"legacy-activation-912004","session_id":"legacy-session-912001","course_id":"legacy-course-912002","deadline_at":"2000-01-01T00:00:00Z","status":"submitted","answers":[{"question_id":"legacy-question-912003","value":"PRESERVE","score":8,"feedback":"PRESERVE_FEEDBACK"}]}')
 || '{"scope":{"class_id":"legacy-class-912007","shift":1,"division":"independent"}}'
 || jsonb_build_object('legacy',jsonb_build_object('pengerjaan_course',jsonb_build_object('id',912005,'course_id',912002,'aktivasi_sesi_id',912006,'mahasiswa_id',owner_id)))
from public.practicum_attempts where id=$5;
`, pgx.QueryExecModeSimpleProtocol, h.session.ID, h.course.ID, h.questions[0].ID, h.activation.ID, a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, mutation string }{
		{"wrong_domain", `update public.practicum_sessions set scope='{"domain":"siakad"}' where id='legacy-session-912001'`},
		{"wrong_session_context", `update public.practicum_sessions set scope='{"session_id":"wrong"}' where id='legacy-session-912001'`},
		{"conflicting_restriction", `update public.practicum_sessions set scope='{"division":"one"}',data=jsonb_set(data,'{scope}','{"division":"two"}') where id='legacy-session-912001'`},
		{"wrong_identity", `update public.practicum_questions set data=jsonb_set(data,'{id}','"wrong"') where id='legacy-question-912003'`},
		{"wrong_course_ancestry", `update public.practicum_activations set data=jsonb_set(data,'{activation,course_id}','"missing"') where id='legacy-activation-912004'`},
		{"missing_marker", `update public.practicum_sessions set data=data-'legacy' where id='legacy-session-912001'`},
		{"unproven_course_restriction", `update public.practicum_courses set scope='{"institution":"unknown"}' where id='legacy-course-912002'`},
		{"conflicting_class", `update public.practicum_activations set data=jsonb_set(data,'{activation,scope,class_id}','"wrong"') where id='legacy-activation-912004'`},
		{"disabled_freeze", `alter table public.practicum_activations disable trigger practicum_activation_frozen`},
		{"disabled_metadata", `alter table public.practicum_attempts disable trigger practicum_attempt_metadata`},
		{"missing_metadata", `drop trigger practicum_attempt_metadata on public.practicum_attempts`},
		{"live_attempt", `alter table public.practicum_attempts disable trigger practicum_attempt_frozen;
 update public.practicum_attempts set status='in_progress',data=jsonb_set(data,'{attempt,deadline_at}',to_jsonb('2099-01-01T00:00:00Z'::text)) where id='legacy-attempt-912005';
 alter table public.practicum_attempts enable trigger practicum_attempt_frozen`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec("savepoint guard_case")
			exec(tc.mutation)
			if _, err := tx.Exec(h.ctx, string(body)); err == nil {
				t.Error("unsafe repair was accepted")
			}
			exec("rollback to savepoint guard_case")
		})
	}

	// Compare complete rows after removing exactly the authorized metadata paths.
	const preserved = `select jsonb_agg(v order by v->>'id') from (
 select to_jsonb(s)-'scope' #- '{data,scope}' v from public.practicum_sessions s
 union all select to_jsonb(c)-'scope' from public.practicum_courses c
 union all select to_jsonb(q)-'scope' from public.practicum_questions q
 union all select to_jsonb(ac)-'scope' #- '{data,activation,scope}' from public.practicum_activations ac
 union all select to_jsonb(at)-'scope' #- '{data,scope}' #- '{data,attempt,session_id}' from public.practicum_attempts at
) rows`
	var before, after []byte
	if err := tx.QueryRow(h.ctx, preserved).Scan(&before); err != nil {
		t.Fatal(err)
	}
	exec(string(body))
	if err := tx.QueryRow(h.ctx, preserved).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("nonwhitelisted data changed")
	}
	var canonical bool
	if err := tx.QueryRow(h.ctx, `select
 s.scope='{"domain":"praktikum","session_id":"legacy-session-912001"}'::jsonb
 and s.data->'scope'=s.scope and c.scope=s.scope and q.scope=s.scope
 and not (c.data ? 'scope') and not (q.data ? 'scope')
 and ac.scope=s.scope || '{"class_id":"legacy-class-912007","shift":1,"division":"independent"}'::jsonb
 and ac.data#>'{activation,scope}'=ac.scope and at.scope=ac.scope and at.data->'scope'=ac.scope
 and at.data#>>'{attempt,session_id}'=s.id
 from public.practicum_sessions s,public.practicum_courses c,public.practicum_questions q,
 public.practicum_activations ac,public.practicum_attempts at
 where s.id='legacy-session-912001' and c.id='legacy-course-912002' and q.id='legacy-question-912003'
 and ac.id='legacy-activation-912004' and at.id='legacy-attempt-912005'`).Scan(&canonical); err != nil || !canonical {
		t.Fatalf("canonical context missing or independent restrictions lost: %v", err)
	}
	// A rerun must be safe and the exact three guards must remain enabled.
	exec(string(body))
	var enabled int
	if err := tx.QueryRow(h.ctx, `select count(*) from pg_trigger where tgenabled='O' and
 (tgrelid='public.practicum_activations'::regclass and tgname='practicum_activation_frozen'
 or tgrelid='public.practicum_attempts'::regclass and tgname in ('practicum_attempt_frozen','practicum_attempt_metadata'))`).Scan(&enabled); err != nil || enabled != 3 {
		t.Fatalf("expected all three guards enabled: %d %v", enabled, err)
	}
}
