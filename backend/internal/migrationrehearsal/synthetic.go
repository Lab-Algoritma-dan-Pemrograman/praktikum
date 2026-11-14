package migrationrehearsal

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
)

const SyntheticStudentAuthID = "b6000000-0000-4000-8000-000000000001"
const SyntheticTeacherAuthID = "b6000000-0000-4000-8000-000000000002"

// Required by the attendance storage constraint, but never an issued QR bearer.
// SHA-256("phase6-inert-closed-attendance"); its session is permanently closed.
const syntheticClosedAttendanceHash = "76274ae1f2eaaf406f3b2c7124b3e651959a610529c53d9fed1c6f3e92bfa1ef"

// SyntheticFixture contains invented identities and academic history only. It
// does not read files, environments, credentials, databases, or legacy data.
func SyntheticFixture() (*Manifest, []SourceRow) {
	cutoff := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	opened := "2026-10-26T01:00:00Z"
	submitted := "2026-10-26T01:10:00Z"
	closed := "2026-10-26T01:30:00Z"
	due := "2026-12-01T00:00:00Z"
	type obj = map[string]any
	ref := func(kind, label string) any { return obj{"$ref": "fixture/" + kind + "/" + label} }
	qref := func(kind, label string) string { return "fixture/" + kind + "/" + label }
	raw := func(v any) json.RawMessage {
		b, err := CanonicalJSON(v)
		if err != nil {
			panic(err)
		}
		return b
	}
	scope := func(domain string) obj { return obj{"domain": domain, "institution": "synthetic-lab"} }
	academicScope := scope("siakad")
	academicScope["class_id"] = ref("academic_class", "class")
	academicScope["shift"] = 1
	practicumScope := scope("praktikum")
	practicumScope["class_id"] = ref("academic_class", "class")
	practicumScope["session_id"] = ref("practicum_session", "session")
	practicumScope["shift"] = 1
	learningScope := scope("elearning")
	inventoryScope := scope("siakad")
	rows := []SourceRow{}
	add := func(kind, label string, p any) {
		rows = append(rows, SourceRow{SourceSystem: "fixture", Entity: kind, SourceRef: label, Payload: raw(p)})
	}
	domain := func(kind, label, parent, owner, status string, s obj, data obj) {
		p := genericPayload{ParentRef: parent, OwnerRef: owner, Scope: raw(s), Status: status, Data: raw(data)}
		add(kind, label, p)
	}
	add("user", "student", obj{"auth_user_id": SyntheticStudentAuthID, "nim": "SYNTH-60001", "email": "phase6.student@example.test", "name": "Synthetic Student", "is_active": true, "hidden": false})
	add("user", "teacher", obj{"auth_user_id": SyntheticTeacherAuthID, "nim": nil, "email": "phase6.teacher@example.test", "name": "Synthetic Teacher", "is_active": true, "hidden": false})
	add("assignment", "student-assignment", obj{"user_ref": qref("user", "student"), "role_code": "mahasiswa", "division_code": nil, "scope": obj{}, "is_active": true})
	add("assignment", "teacher-assignment", obj{"user_ref": qref("user", "teacher"), "role_code": "koordinator", "division_code": nil, "scope": obj{}, "is_active": true})
	domain("learning_level", "level", "", "", "", learningScope, obj{
		"id": ref("learning_level", "level"), "title": "Synthetic C Foundations", "description": "Invented rehearsal curriculum", "sort_order": 1, "access_mode": "unlocked", "required_xp": 0, "unlocked": true, "completed_lessons": 0, "total_lessons": 1, "scope": learningScope, "version": 1, "created_at": cutoff, "updated_at": cutoff,
	})
	domain("learning_module", "module", qref("learning_level", "level"), "", "", learningScope, obj{
		"id": ref("learning_module", "module"), "level_id": ref("learning_level", "level"), "title": "Synthetic Introduction", "description": "Text-only C introduction", "sort_order": 1, "completed_lessons": 0, "total_lessons": 1, "scope": learningScope, "version": 1,
	})
	domain("learning_lesson", "lesson", qref("learning_module", "module"), "", "", learningScope, obj{
		"id": ref("learning_lesson", "lesson"), "module_id": ref("learning_module", "module"), "level_id": ref("learning_level", "level"), "title": "Reading a C Program", "sort_order": 1, "language": "c", "explanation": "A C program starts in its main function. This synthetic lesson has no code execution requirement.", "code_example": "", "starter_code": "", "hint": "Read the introduction.", "xp_reward": 40, "test_cases": []any{}, "scope": learningScope, "version": 1,
	})
	domain("learning_progress", "progress", qref("learning_lesson", "lesson"), qref("user", "student"), "in_progress", learningScope, obj{
		"lesson_id": ref("learning_lesson", "lesson"), "module_id": ref("learning_module", "module"), "level_id": ref("learning_level", "level"), "user_id": ref("user", "student"), "status": "in_progress", "draft_code": "", "language": "c", "quiz_option": nil, "test_passed": false, "revision": 1, "xp_awarded": 0, "completed_at": nil, "updated_at": cutoff,
	})
	domain("learning_profile", "profile", "", qref("user", "student"), "active", learningScope, obj{
		"user_id": ref("user", "student"), "user_name": "Synthetic Student", "xp": 0, "level": 1, "next_level_xp": 50, "streak": 0, "last_active_date": "", "completed_lessons": 0, "games_played": 0, "perfect_games": 0, "unlocked_achievements": []any{}, "timezone": "Asia/Jakarta", "timezone_offset": "+07:00", "updated_at": cutoff,
	})
	domain("practicum_session", "session", "", "", "", practicumScope, obj{
		"id": ref("practicum_session", "session"), "title": "Synthetic Manual Review", "sequence": 1, "scope": practicumScope, "active": false, "version": 1, "created_at": opened, "updated_at": closed,
	})
	course := obj{"id": ref("practicum_course", "course"), "session_id": ref("practicum_session", "session"), "title": "Synthetic Essay", "kind": "pretest", "duration_minutes": 30, "token_required": false, "version": 1}
	question := obj{"id": ref("practicum_question", "question"), "course_id": ref("practicum_course", "course"), "type": "essay", "difficulty": "easy", "category": "synthetic", "prompt": "Explain why a C program has a main function.", "points": 100, "starter_code": "", "language": "", "reference_answer": "The main function is the program entry point.", "rubric": []any{obj{"id": "explanation", "description": "Correctly explains the entry point.", "max_score": 100}}, "version": 1}
	domain("practicum_course", "course", qref("practicum_session", "session"), "", "", practicumScope, course)
	domain("practicum_question", "question", qref("practicum_course", "course"), "", "", practicumScope, question)
	domain("practicum_activation", "activation", qref("practicum_session", "session"), "", "", practicumScope, obj{
		"activation": obj{"id": ref("practicum_activation", "activation"), "session_id": ref("practicum_session", "session"), "course_id": ref("practicum_course", "course"), "scope": practicumScope, "timer_mode": "shared", "is_open": false, "started_at": opened, "deadline_at": closed, "closes_at": closed, "question_count": 1, "version": 1, "student_ids": []any{ref("user", "student")}, "question_ids": []any{ref("practicum_question", "question")}}, "course": course, "questions": []any{question}, "token_hash": "",
	})
	answer := obj{"question_id": ref("practicum_question", "question"), "value": "The main function is where execution begins.", "language": "", "updated_at": submitted}
	domain("practicum_attempt", "attempt", qref("practicum_activation", "activation"), qref("user", "student"), "submitted", practicumScope, obj{
		"attempt": obj{"attempt_id": ref("practicum_attempt", "attempt"), "activation_id": ref("practicum_activation", "activation"), "session_id": ref("practicum_session", "session"), "course_id": ref("practicum_course", "course"), "user_id": ref("user", "student"), "status": "submitted", "submission_reason": "manual", "revision": 1, "grade_revision": 0, "grading_status": "needs_manual", "started_at": opened, "deadline_at": closed, "submitted_at": submitted, "server_now": cutoff, "timezone": "Asia/Jakarta", "timezone_offset": "+07:00", "questions": []any{question}, "answers": []any{answer}, "score": nil, "max_score": 100, "percentage": nil}, "scope": practicumScope, "user_name": "Synthetic Student", "submitted_answers": []any{answer}, "manual_grades": []any{}, "grade_reason": "",
	})
	domain("practicum_grading_job", "job", qref("practicum_attempt", "attempt"), "", "needs_manual", practicumScope, obj{
		"id": ref("practicum_grading_job", "job"), "attempt_id": ref("practicum_attempt", "attempt"), "status": "needs_manual", "grade_revision": 0, "suggestions": []any{}, "simulated": false, "updated_at": submitted,
	})
	domain("academic_class", "class", "", "", "", academicScope, obj{
		"id": ref("academic_class", "class"), "code": "SYNTH-P6-A", "course_code": "SYNTH-C101", "course_name": "Synthetic Programming", "period_name": "Synthetic 2026", "shift": 1, "student_ids": []any{ref("user", "student")}, "assistant_ids": []any{ref("user", "teacher")}, "active": true, "scope": academicScope, "version": 1, "created_at": opened, "updated_at": cutoff,
	})
	domain("academic_schedule", "schedule", qref("academic_class", "class"), "", "", academicScope, obj{
		"id": ref("academic_schedule", "schedule"), "class_id": ref("academic_class", "class"), "class_code": "SYNTH-P6-A", "course_name": "Synthetic Programming", "day_of_week": "Senin", "shift": 1, "room": "Synthetic Lab", "starts_at": "08:00", "ends_at": "09:00", "scope": academicScope, "version": 1,
	})
	domain("academic_attendance_session", "attendance-session", qref("academic_schedule", "schedule"), "", "closed", academicScope, obj{
		"session": obj{"id": ref("academic_attendance_session", "attendance-session"), "schedule_id": ref("academic_schedule", "schedule"), "class_id": ref("academic_class", "class"), "title": "Synthetic Historical Attendance", "is_open": false, "opened_at": opened, "expires_at": closed, "closed_at": closed, "scope": academicScope, "version": 1}, "token_hash": syntheticClosedAttendanceHash,
	})
	domain("academic_attendance_record", "attendance-record", qref("academic_attendance_session", "attendance-session"), qref("user", "student"), "hadir", academicScope, obj{
		"id": ref("academic_attendance_record", "attendance-record"), "session_id": ref("academic_attendance_session", "attendance-session"), "schedule_id": ref("academic_schedule", "schedule"), "class_id": ref("academic_class", "class"), "user_id": ref("user", "student"), "user_name": "Synthetic Student", "status": "hadir", "verified_by_qr": false, "validated": false, "checked_in_at": submitted, "validated_by": nil, "validated_at": nil, "notes": "Invented historical attendance; not QR proof.", "scope": academicScope, "version": 1,
	})
	domain("academic_inventory_item", "item", "", "", "available", inventoryScope, obj{
		"id": ref("academic_inventory_item", "item"), "code": "SYNTH-ITEM-01", "name": "Synthetic Training Kit", "category": "Training", "total_quantity": 3, "available_quantity": 3, "rental_fee": 1000, "status": "available", "scope": inventoryScope, "version": 1, "updated_at": cutoff,
	})
	domain("academic_inventory_rental", "rental", qref("academic_inventory_item", "item"), qref("user", "student"), "requested", inventoryScope, obj{
		"id": ref("academic_inventory_rental", "rental"), "item_id": ref("academic_inventory_item", "item"), "item_name": "Synthetic Training Kit", "borrower_id": ref("user", "student"), "borrower_name": "Synthetic Student", "quantity": 1, "purpose": "Synthetic rehearsal", "status": "requested", "total_fee": 1000, "payment_verified": false, "requested_at": cutoff, "due_at": due, "decided_at": nil, "returned_at": nil, "decided_by": nil, "scope": inventoryScope, "version": 1,
	})
	// Non-sensitive sentinels prove policy accounting without carrying credentials.
	add("auth_session", "excluded-auth", obj{})
	add("legacy_financial_ledger", "unsupported-finance", obj{})
	manifest := &Manifest{FormatVersion: FormatVersion, BatchID: "phase6-synthetic-v1", SourceNamespace: "phase6-synthetic-v1", SourceKind: "synthetic", Cutoff: cutoff, PolicyVersion: PolicyVersion, TargetMigrations: migrations.Catalog(), Files: []SourceFile{{Path: "rows.ndjson", SHA256: strings.Repeat("0", 64), Rows: len(rows)}}, Expected: Expected{Counts: map[string]int{string(Imported): len(rows) - 2, string(Rejected): 2, string(Unchanged): 0, string(IntentionallySkipped): 0}, Reasons: map[string]int{ReasonPlanned: len(rows) - 2, ReasonEphemeralSkipped: 1, ReasonInvalidEntity: 1}}}
	return manifest, rows
}
