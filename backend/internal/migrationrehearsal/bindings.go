package migrationrehearsal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func parentEntity(kind string) string {
	switch kind {
	case "practicum_course", "practicum_activation":
		return "practicum_session"
	case "practicum_question":
		return "practicum_course"
	case "practicum_attempt":
		return "practicum_activation"
	case "practicum_grading_job":
		return "practicum_attempt"
	case "learning_module":
		return "learning_level"
	case "learning_lesson":
		return "learning_module"
	case "learning_progress":
		return "learning_lesson"
	case "academic_schedule":
		return "academic_class"
	case "academic_attendance_session":
		return "academic_schedule"
	case "academic_attendance_record":
		return "academic_attendance_session"
	case "academic_inventory_rental":
		return "academic_inventory_item"
	}
	return ""
}
func ownerRequired(kind string) bool {
	switch kind {
	case "practicum_attempt", "learning_progress", "learning_profile", "academic_attendance_record", "academic_inventory_rental":
		return true
	}
	return false
}

// The only symbolic leaves in this contract. Wildcards address array elements,
// never object keys. Free-text content, answers and arbitrary objects are not a
// substitution language.
func referencePaths(kind string) map[string]string {
	paths := map[string]string{"scope.class_id": "academic_class", "scope.session_id": "practicum_session"}
	add := func(expected string, names ...string) {
		for _, name := range names {
			paths["data."+name] = expected
		}
	}
	scoped := func(prefix string) {
		add("academic_class", prefix+"scope.class_id")
		add("practicum_session", prefix+"scope.session_id")
	}
	switch kind {
	case "practicum_session":
		add(kind, "id")
		scoped("")
	case "practicum_course":
		add(kind, "id")
		add("practicum_session", "session_id")
	case "practicum_question":
		add(kind, "id")
		add("practicum_course", "course_id")
	case "practicum_activation":
		add(kind, "activation.id")
		add("practicum_session", "activation.session_id", "course.session_id")
		add("practicum_course", "activation.course_id", "course.id", "questions.*.course_id")
		add("practicum_question", "activation.question_ids.*", "questions.*.id")
		add("user", "activation.student_ids.*", "activation.entry_approved_ids.*")
		scoped("activation.")
	case "practicum_attempt":
		add(kind, "attempt.attempt_id")
		add("practicum_activation", "attempt.activation_id")
		add("practicum_session", "attempt.session_id")
		add("practicum_course", "attempt.course_id", "attempt.questions.*.course_id")
		add("user", "attempt.user_id")
		add("practicum_question", "attempt.questions.*.id", "attempt.answers.*.question_id", "submitted_answers.*.question_id", "manual_grades.*.question_id")
		scoped("")
	case "practicum_grading_job":
		add(kind, "id")
		add("practicum_attempt", "attempt_id")
	case "learning_level":
		add(kind, "id")
		scoped("")
	case "learning_module":
		add(kind, "id")
		add("learning_level", "level_id")
		scoped("")
	case "learning_lesson":
		add(kind, "id")
		add("learning_module", "module_id")
		add("learning_level", "level_id")
		scoped("")
	case "learning_progress":
		add("learning_lesson", "lesson_id")
		add("learning_module", "module_id")
		add("learning_level", "level_id")
		add("user", "user_id")
	case "learning_profile":
		add("user", "user_id")
	case "academic_class":
		add(kind, "id")
		add("user", "student_ids.*", "assistant_ids.*")
		scoped("")
	case "academic_schedule":
		add(kind, "id")
		add("academic_class", "class_id")
		scoped("")
	case "academic_attendance_session":
		add(kind, "session.id")
		add("academic_schedule", "session.schedule_id")
		add("academic_class", "session.class_id")
		scoped("session.")
	case "academic_attendance_record":
		add(kind, "id")
		add("academic_attendance_session", "session_id")
		add("academic_schedule", "schedule_id")
		add("academic_class", "class_id")
		add("user", "user_id", "validated_by")
		scoped("")
	case "academic_inventory_item":
		add(kind, "id")
		scoped("")
	case "academic_inventory_rental":
		add(kind, "id")
		add("academic_inventory_item", "item_id")
		add("user", "borrower_id", "decided_by")
		scoped("")
	}
	return paths
}
func rewriteReferences(raw json.RawMessage, kind, root, symbol string, resolve func(string, string) (any, error)) (json.RawMessage, error) {
	if _, err := ParseCanonicalJSON(raw); err != nil {
		return nil, err
	}
	var node any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&node); err != nil {
		return nil, err
	}
	paths := referencePaths(kind)
	var visit func(any, string) (any, error)
	visit = func(v any, path string) (any, error) {
		expected, declared := paths[path]
		if declared {
			if v == nil || v == "" {
				return v, nil
			}
			obj, ok := v.(map[string]any)
			if !ok || len(obj) != 1 {
				return nil, fmt.Errorf("typed ID at %s requires an exact %s object", path, symbol)
			}
			ref, ok := obj[symbol].(string)
			if !ok || ref == "" {
				return nil, fmt.Errorf("malformed reference at %s", path)
			}
			return resolve(ref, expected)
		}
		switch value := v.(type) {
		case map[string]any:
			if _, ok := value["$ref"]; ok {
				return nil, errors.New("reference outside declared typed path")
			}
			if _, ok := value["$token"]; ok {
				return nil, errors.New("token outside declared typed path")
			}
			for k, c := range value {
				next, err := visit(c, path+"."+k)
				if err != nil {
					return nil, err
				}
				value[k] = next
			}
			return value, nil
		case []any:
			for i, c := range value {
				next, err := visit(c, path+".*")
				if err != nil {
					return nil, err
				}
				value[i] = next
			}
			return value, nil
		default:
			return v, nil
		}
	}
	out, err := visit(node, root)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(out)
}
func dependencies(row PlanRow) (map[string]string, error) {
	d := row.Desired
	if d == nil {
		return nil, errors.New("missing desired record")
	}
	deps := map[string]string{}
	add := func(token, expected string) error {
		if token == "" {
			return nil
		}
		if expected == "" {
			return errors.New("unexpected dependency")
		}
		if prior, ok := deps[token]; ok && prior != expected {
			return errors.New("conflicting reference types")
		}
		deps[token] = expected
		return nil
	}
	if err := add(d.ParentToken, parentEntity(row.Entity)); err != nil {
		return nil, err
	}
	if err := add(d.OwnerToken, "user"); err != nil {
		return nil, err
	}
	if err := add(d.UserToken, "user"); err != nil {
		return nil, err
	}
	if err := add(d.AssignmentToken, "assignment"); err != nil {
		return nil, err
	}
	if entitySpecs[row.Entity].kind == "generic" {
		collect := func(token, expected string) (any, error) {
			if err := add(token, expected); err != nil {
				return nil, err
			}
			return map[string]any{"$token": token}, nil
		}
		if _, err := rewriteReferences(d.Data, row.Entity, "data", "$token", collect); err != nil {
			return nil, err
		}
		if _, err := rewriteReferences(d.Scope, row.Entity, "scope", "$token", collect); err != nil {
			return nil, err
		}
	}
	return deps, nil
}

// bindDomain never mutates a plan or the caller's maps. The numeric IDs belong
// to inserted/equal users; keys are the actual application IDs of prior rows.
func bindDomain(row PlanRow, ids map[string]int64, keys map[string]string) (*DesiredRecord, error) {
	if row.Desired == nil || entitySpecs[row.Entity].kind != "generic" {
		return nil, errors.New("not a typed domain row")
	}
	d := *row.Desired
	if parentEntity(row.Entity) != "" {
		parent, ok := keys[d.ParentToken]
		if !ok || parent == "" {
			return nil, errors.New("unresolved parent token")
		}
		if d.ParentID != "" && d.ParentID != parent {
			return nil, errors.New("parent metadata mismatch")
		}
		d.ParentID = parent
	} else if d.ParentToken != "" || d.ParentID != "" {
		return nil, errors.New("unexpected parent")
	}
	owner := int64(0)
	if ownerRequired(row.Entity) {
		owner = ids[d.OwnerToken]
		if owner <= 0 {
			return nil, errors.New("unresolved owner token")
		}
	} else if d.OwnerToken != "" {
		return nil, errors.New("unexpected owner")
	}
	uid := strconv.FormatInt(owner, 10)
	switch row.Entity {
	case "learning_profile":
		d.ID = "profile-" + uid
	case "learning_progress":
		d.ID = "progress-" + d.ParentID + "-" + uid
	case "academic_attendance_record":
		d.ID = "att-rec-" + d.ParentID + "-" + uid
	case "practicum_grading_job":
		d.ID = d.ParentID
	}
	resolve := func(token, expected string) (any, error) {
		if expected == "user" {
			id := ids[token]
			if id <= 0 {
				return nil, errors.New("unresolved numeric user token")
			}
			return id, nil
		}
		if token == row.RowToken {
			if expected != row.Entity {
				return nil, errors.New("wrong self reference")
			}
			return d.ID, nil
		}
		key, ok := keys[token]
		if !ok || key == "" {
			return nil, errors.New("unresolved domain token")
		}
		return key, nil
	}
	var err error
	d.Data, err = rewriteReferences(d.Data, row.Entity, "data", "$token", resolve)
	if err != nil {
		return nil, err
	}
	d.Scope, err = rewriteReferences(d.Scope, row.Entity, "scope", "$token", resolve)
	if err != nil {
		return nil, err
	}
	if err = validateBound(row.Entity, &d, owner); err != nil {
		return nil, err
	}
	return &d, nil
}
func invalidTyped() error {
	return errors.New("typed data or indexed metadata violates synthetic policy")
}
func nonempty(v ...string) bool {
	for _, s := range v {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}
func timeSet(t time.Time) bool          { return !t.IsZero() && t.Location() == time.UTC }
func scopeEqual(a, b entity.Scope) bool { return a == b }
func validScope(s entity.Scope, kind string) bool {
	domain := ""
	switch {
	case strings.HasPrefix(kind, "learning_"):
		domain = "elearning"
	case strings.HasPrefix(kind, "practicum_"):
		domain = "praktikum"
	case strings.HasPrefix(kind, "academic_"):
		domain = "siakad"
	}
	return s.Domain == domain && s.Institution != "" && s.Institution != "*" && s.Shift >= 0 && s.Shift <= 10
}
func validQuestion(q entity.PracticumQuestion) bool {
	return nonempty(q.ID, q.CourseID, q.Prompt) && q.Type == "essay" && q.Points == 100 && q.Version == 1 && q.Language == "" && q.StarterCode == "" && len(q.Rubric) == 1 && q.Rubric[0].ID != "" && q.Rubric[0].Description != "" && q.Rubric[0].MaxScore == 100 && (q.Difficulty == "easy" || q.Difficulty == "medium" || q.Difficulty == "hard")
}
func validCourse(c entity.PracticumCourse) bool {
	return nonempty(c.ID, c.SessionID, c.Title) && (c.Kind == "pretest" || c.Kind == "posttest" || c.Kind == "keterampilan" || c.Kind == "ujian_praktik") && c.DurationMinutes > 0 && c.DurationMinutes <= 180 && !c.TokenRequired && c.Version == 1
}
func validUserList(ids []int64) bool {
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// State support is intentionally small: text learning with zero awarded XP,
// closed historical practicum awaiting manual review, historical attendance,
// and requested (not approved or paid) rentals. No legacy state is inferred.
func validateBound(kind string, d *DesiredRecord, owner int64) error {
	var scope entity.Scope
	if decodeStrict(d.Scope, &scope) != nil || !validScope(scope, kind) {
		return invalidTyped()
	}
	status, unique := "", ""
	var expires *time.Time
	switch kind {
	case "learning_level":
		var v entity.LearningLevel
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || !nonempty(v.Title) || v.AccessMode != "unlocked" || v.RequiredXP != 0 || v.Version != 1 || v.CompletedLessons != 0 || v.TotalLessons != 1 || len(v.Modules) != 0 || !scopeEqual(v.Scope, scope) || !timeSet(v.CreatedAt) || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
	case "learning_module":
		var v entity.LearningModule
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.LevelID != d.ParentID || !nonempty(v.Title) || v.Version != 1 || v.CompletedLessons != 0 || v.TotalLessons != 1 || len(v.Lessons) != 0 || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
	case "learning_lesson":
		var v entity.LearningLesson
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.ModuleID != d.ParentID || !nonempty(v.LevelID, v.Title, v.Explanation) || v.Language != "c" || v.XPReward != 40 || v.Version != 1 || v.Quiz != nil || len(v.TestCases) != 0 || v.StarterCode != "" || v.Solution != "" || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
	case "learning_progress":
		var v entity.LearningProgress
		if decodeStrict(d.Data, &v) != nil || v.LessonID != d.ParentID || v.UserID != owner || !nonempty(v.ModuleID, v.LevelID) || v.Status != "in_progress" || v.Language != "c" || v.Revision != 1 || v.XPAwarded != 0 || v.CompletedAt != nil || v.TestPassed || v.QuizOption != nil || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
		status = v.Status
		unique = v.LessonID + ":" + strconv.FormatInt(owner, 10)
	case "learning_profile":
		var v entity.LearningProfile
		if decodeStrict(d.Data, &v) != nil || v.UserID != owner || !nonempty(v.UserName) || v.XP != 0 || v.Level != 1 || v.NextLevelXP != 50 || v.CompletedLessons != 0 || v.GamesPlayed != 0 || v.PerfectGames != 0 || v.Streak != 0 || len(v.UnlockedAchievements) != 0 || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
		status = "active"
		unique = d.ID
	case "practicum_session":
		var v entity.PracticumSession
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || !nonempty(v.Title) || v.Active || v.Version != 1 || v.Sequence < 1 || !scopeEqual(v.Scope, scope) || !timeSet(v.CreatedAt) || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
	case "practicum_course":
		var v entity.PracticumCourse
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.SessionID != d.ParentID || !validCourse(v) {
			return invalidTyped()
		}
		unique = v.SessionID + ":" + v.Kind
	case "practicum_question":
		var v entity.PracticumQuestion
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.CourseID != d.ParentID || !validQuestion(v) {
			return invalidTyped()
		}
	case "practicum_activation":
		var v entity.PracticumActivationRecord
		if decodeStrict(d.Data, &v) != nil {
			return invalidTyped()
		}
		a := v.Activation
		if a.ID != d.ID || a.SessionID != d.ParentID || a.CourseID != v.Course.ID || v.Course.SessionID != a.SessionID || a.IsOpen || a.AccessToken != "" || v.TokenHash != "" || a.Version != 1 || a.TimerMode != "shared" || a.QuestionCount != 1 || len(v.Questions) != 1 || len(a.QuestionIDs) != 1 || a.QuestionIDs[0] != v.Questions[0].ID || v.Questions[0].CourseID != a.CourseID || !validQuestion(v.Questions[0]) || !validCourse(v.Course) || !scopeEqual(a.Scope, scope) || a.StartedAt == nil || a.DeadlineAt == nil || a.ClosesAt == nil || a.StartedAt.After(*a.DeadlineAt) || a.DeadlineAt.After(*a.ClosesAt) || !validUserList(a.StudentIDs) || len(a.StudentIDs) != 1 || len(a.EntryApprovedIDs) != 0 {
			return invalidTyped()
		}
		expires = a.ClosesAt
	case "practicum_attempt":
		var v entity.PracticumAttemptRecord
		if decodeStrict(d.Data, &v) != nil {
			return invalidTyped()
		}
		a := v.Attempt
		if a.AttemptID != d.ID || a.ActivationID != d.ParentID || a.UserID != owner || !nonempty(a.SessionID, a.CourseID, v.UserName) || a.Status != "submitted" || a.GradingStatus != "needs_manual" || a.SubmissionReason != "manual" || a.Revision != 1 || a.GradeRevision != 0 || a.Score != nil || a.Percentage != nil || a.MaxScore != 100 || a.SubmittedAt == nil || !timeSet(a.StartedAt) || !timeSet(a.DeadlineAt) || a.StartedAt.After(*a.SubmittedAt) || a.SubmittedAt.After(a.DeadlineAt) || len(a.Questions) != 1 || !validQuestion(a.Questions[0]) || a.Questions[0].CourseID != a.CourseID || len(a.Answers) != 1 || len(v.SubmittedAnswers) != 1 || len(v.ManualGrades) != 0 || v.GradeReason != "" || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
		answer := a.Answers[0]
		if answer.QuestionID != a.Questions[0].ID || answer.Score != nil || !nonempty(answer.Value) || !timeSet(answer.UpdatedAt) || answer != v.SubmittedAnswers[0] {
			return invalidTyped()
		}
		status = a.Status
		unique = a.ActivationID + ":" + strconv.FormatInt(owner, 10)
		expires = &a.DeadlineAt
	case "practicum_grading_job":
		var v entity.PracticumGradingJob
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.AttemptID != d.ParentID || v.ID != v.AttemptID || v.Status != "needs_manual" || v.GradeRevision != 0 || v.LeaseID != "" || v.LeaseUntil != nil || len(v.Suggestions) != 0 || v.Provider != "" || v.Simulated || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
		status = v.Status
		unique = v.AttemptID
	case "academic_class":
		var v entity.AcademicClass
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || !nonempty(v.Code, v.CourseCode, v.CourseName, v.PeriodName) || !v.Active || v.Version != 1 || v.Shift != 1 || scope.ClassID != v.ID || scope.Shift != v.Shift || len(v.StudentIDs) != 1 || !validUserList(v.StudentIDs) || !validUserList(v.AssistantIDs) || !scopeEqual(v.Scope, scope) || !timeSet(v.CreatedAt) || !timeSet(v.UpdatedAt) {
			return invalidTyped()
		}
		unique = v.Code
	case "academic_schedule":
		var v entity.AcademicSchedule
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.ClassID != d.ParentID || !nonempty(v.ClassCode, v.CourseName, v.Room) || v.DayOfWeek != "Senin" || v.Shift != 1 || v.Version != 1 || scope.ClassID != v.ClassID || scope.Shift != v.Shift || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
		start, e1 := time.Parse("15:04", v.StartsAt)
		end, e2 := time.Parse("15:04", v.EndsAt)
		if e1 != nil || e2 != nil || !end.After(start) {
			return invalidTyped()
		}
	case "academic_attendance_session":
		var v entity.AcademicAttendanceSessionRecord
		if decodeStrict(d.Data, &v) != nil {
			return invalidTyped()
		}
		s := v.Session
		if s.ID != d.ID || s.ScheduleID != d.ParentID || s.ClassID != scope.ClassID || !nonempty(s.Title) || s.IsOpen || s.QRToken != "" || v.TokenHash != syntheticClosedAttendanceHash || s.Version != 1 || s.ClosedAt == nil || !timeSet(s.OpenedAt) || !timeSet(s.ExpiresAt) || s.OpenedAt.After(*s.ClosedAt) || s.ClosedAt.After(s.ExpiresAt) || !scopeEqual(s.Scope, scope) {
			return invalidTyped()
		}
		status = "closed"
		expires = &s.ExpiresAt
	case "academic_attendance_record":
		var v entity.AcademicAttendanceRecord
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.SessionID != d.ParentID || v.UserID != owner || !nonempty(v.ScheduleID, v.ClassID, v.UserName) || v.ClassID != scope.ClassID || v.Status != "hadir" || v.VerifiedByQR || v.Validated || v.ValidatedAt != nil || v.ValidatedBy != nil || v.Version != 1 || !timeSet(v.CheckedInAt) || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
		status = v.Status
		unique = v.SessionID + ":" + strconv.FormatInt(owner, 10)
	case "academic_inventory_item":
		var v entity.InventoryItem
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || !nonempty(v.Code, v.Name, v.Category) || v.TotalQuantity != 3 || v.AvailableQuantity != 3 || v.RentalFee != 1000 || v.Status != "available" || v.Version != 1 || !timeSet(v.UpdatedAt) || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
		status = v.Status
		unique = v.Code
	case "academic_inventory_rental":
		var v entity.InventoryRental
		if decodeStrict(d.Data, &v) != nil || v.ID != d.ID || v.ItemID != d.ParentID || v.BorrowerID != owner || !nonempty(v.ItemName, v.BorrowerName, v.Purpose) || v.Quantity != 1 || v.TotalFee != 1000 || v.Status != "requested" || v.PaymentVerified || v.DecidedAt != nil || v.ReturnedAt != nil || v.DecidedBy != nil || v.Version != 1 || !timeSet(v.RequestedAt) || !timeSet(v.DueAt) || !v.DueAt.After(v.RequestedAt) || !scopeEqual(v.Scope, scope) {
			return invalidTyped()
		}
		status = v.Status
		expires = &v.DueAt
	default:
		return errors.New("unsupported typed entity")
	}
	if d.Status != status || d.UniqueKey != "" && d.UniqueKey != unique || d.ExpiresAt != nil && !equalTimePtr(d.ExpiresAt, expires) {
		return invalidTyped()
	}
	d.UniqueKey = unique
	d.ExpiresAt = expires
	return nil
}
