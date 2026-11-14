package migrationrehearsal

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// These joins cover references stored inside JSON, which ordinary SQL FKs do
// not cover. They are restricted to the current synthetic contract, not a claim
// that ambiguous legacy relationships or opening balances can be inferred.
func semanticChecks(ctx context.Context, tx pgx.Tx) ([]Check, error) {
	queries := []struct{ name, sql string }{
		{"learning_lesson_ancestry", `select count(*) from learning_lessons l join learning_modules m on m.id=l.parent_id where l.data->>'id' is distinct from l.id or l.data->>'module_id' is distinct from m.id or l.data->>'level_id' is distinct from m.parent_id`},
		{"learning_progress_ancestry", `select count(*) from learning_progress p join learning_lessons l on l.id=p.parent_id where p.data->>'lesson_id' is distinct from l.id or p.data->>'module_id' is distinct from l.parent_id or p.data->>'level_id' is distinct from l.data->>'level_id' or (p.data->>'user_id')::bigint is distinct from p.owner_id or p.data->>'status' is distinct from p.status`},
		{"learning_profile_ledger", `select count(*) from learning_profiles p where (p.data->>'user_id')::bigint is distinct from p.owner_id or (p.data->>'xp')::bigint is distinct from (select coalesce(sum((r.data->>'xp_awarded')::bigint),0) from learning_progress r where r.owner_id=p.owner_id)+(select coalesce(sum((g.data->>'xp_earned')::bigint),0) from learning_game_history g where g.owner_id=p.owner_id) or (p.data->>'completed_lessons')::bigint is distinct from (select count(*) from learning_progress r where r.owner_id=p.owner_id and r.status='completed')`},
		{"practicum_activation_ancestry", `select count(*) from practicum_activations a left join practicum_courses c on c.id=a.data->'activation'->>'course_id' where c.id is null or c.parent_id is distinct from a.parent_id or a.data->'activation'->>'id' is distinct from a.id or a.data->'activation'->>'session_id' is distinct from a.parent_id or a.data->'course'->>'id' is distinct from c.id or a.data->'course'->>'session_id' is distinct from c.parent_id`},
		{"practicum_attempt_ancestry", `select count(*) from practicum_attempts r join practicum_activations a on a.id=r.parent_id where r.data->'attempt'->>'attempt_id' is distinct from r.id or r.data->'attempt'->>'activation_id' is distinct from a.id or r.data->'attempt'->>'session_id' is distinct from a.parent_id or r.data->'attempt'->>'course_id' is distinct from a.data->'activation'->>'course_id' or (r.data->'attempt'->>'user_id')::bigint is distinct from r.owner_id or r.data->'attempt'->>'status' is distinct from r.status or not exists(select 1 from jsonb_array_elements_text(a.data->'activation'->'student_ids') s where s.value::bigint=r.owner_id)`},
		{"practicum_frozen_question_references", `select count(*) from practicum_attempts r cross join lateral jsonb_array_elements(r.data->'attempt'->'questions') q where not exists(select 1 from practicum_activations a cross join lateral jsonb_array_elements(a.data->'questions') aq where a.id=r.parent_id and aq.value->>'id'=q.value->>'id' and aq.value=q.value)`},
		{"practicum_job_identity", `select count(*) from practicum_grading_jobs j join practicum_attempts a on a.id=j.parent_id where j.id is distinct from a.id or j.data->>'id' is distinct from j.id or j.data->>'attempt_id' is distinct from a.id or j.data->>'status' is distinct from j.status or (j.data->>'grade_revision')::bigint is distinct from (a.data->'attempt'->>'grade_revision')::bigint`},
		{"academic_roster_references", `select count(*) from academic_classes c cross join lateral jsonb_array_elements_text(coalesce(c.data->'student_ids','[]'::jsonb)||coalesce(c.data->'assistant_ids','[]'::jsonb)) member where not exists(select 1 from users u where u.id=member.value::bigint)`},
		{"academic_schedule_ancestry", `select count(*) from academic_schedules s join academic_classes c on c.id=s.parent_id where s.data->>'id' is distinct from s.id or s.data->>'class_id' is distinct from c.id or s.data->>'class_code' is distinct from c.data->>'code' or s.scope->>'class_id' is distinct from c.id`},
		{"academic_attendance_session_ancestry", `select count(*) from academic_attendance_sessions a join academic_schedules s on s.id=a.parent_id where a.data->'session'->>'id' is distinct from a.id or a.data->'session'->>'schedule_id' is distinct from s.id or a.data->'session'->>'class_id' is distinct from s.parent_id`},
		{"academic_attendance_record_ancestry", `select count(*) from academic_attendance_records r join academic_attendance_sessions a on a.id=r.parent_id where r.data->>'id' is distinct from r.id or r.data->>'session_id' is distinct from a.id or r.data->>'schedule_id' is distinct from a.parent_id or r.data->>'class_id' is distinct from a.data->'session'->>'class_id' or (r.data->>'user_id')::bigint is distinct from r.owner_id or r.data->>'status' is distinct from r.status`},
		{"inventory_rental_references", `select count(*) from academic_inventory_rentals r join academic_inventory_items i on i.id=r.parent_id where r.data->>'id' is distinct from r.id or r.data->>'item_id' is distinct from i.id or (r.data->>'borrower_id')::bigint is distinct from r.owner_id or r.data->>'status' is distinct from r.status`},
		{"inventory_approved_stock", `select count(*) from academic_inventory_items i where (i.data->>'total_quantity')::bigint-(i.data->>'available_quantity')::bigint < (select coalesce(sum((r.data->>'quantity')::bigint),0) from academic_inventory_rentals r where r.parent_id=i.id and r.status='approved')`},
	}
	checks := []Check{}
	for _, q := range queries {
		var count int
		if err := tx.QueryRow(ctx, q.sql).Scan(&count); err != nil {
			return nil, errors.New("semantic check query failed: " + q.name)
		}
		if count != 0 {
			return nil, errors.New("semantic reference mismatch: " + q.name)
		}
		checks = append(checks, Check{Name: "semantic:" + q.name, Status: "passed", Reason: ReasonCheckPassed})
	}
	return checks, nil
}
