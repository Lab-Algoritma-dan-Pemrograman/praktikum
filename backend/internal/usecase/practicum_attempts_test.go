package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type practicumTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *practicumTestClock) Now() time.Time    { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *practicumTestClock) Set(now time.Time) { c.mu.Lock(); defer c.mu.Unlock(); c.now = now }

type practicumTestIdentity struct {
	repository.IdentityRepository
	students map[int64]*entity.Principal
}

func (i practicumTestIdentity) FindUserByID(_ context.Context, id int64) (entity.UserProfile, error) {
	if p := i.students[id]; p != nil {
		return p.User, nil
	}
	return entity.UserProfile{}, repository.ErrIdentityNotFound
}
func (i practicumTestIdentity) ListAssignments(_ context.Context, id int64) ([]entity.RoleAssignment, error) {
	if p := i.students[id]; p != nil {
		return p.Assignments, nil
	}
	return nil, repository.ErrIdentityNotFound
}

type practicumHarness struct {
	s          *PracticumService
	repo       *repository.MemoryPracticumRepository
	audit      *repository.MemoryIdentityRepository
	clock      *practicumTestClock
	admin      *entity.Principal
	students   []*entity.Principal
	session    entity.PracticumSession
	course     entity.PracticumCourse
	questions  []entity.PracticumQuestion
	activation entity.PracticumActivation
}

func practicumCmd(key string) PracticumCommand {
	return PracticumCommand{Key: key, RequestID: "req-test", OperationID: "op-test"}
}
func practicumStaff(scope entity.Scope, permissions ...string) *entity.Principal {
	p := &entity.Principal{User: entity.UserProfile{ID: 9, Active: true}, Assignments: []entity.RoleAssignment{{Role: entity.RoleAsisten, Active: true, Scope: scope}}}
	for _, permission := range permissions {
		p.Grants = append(p.Grants, entity.PermissionGrant{Permission: permission, Role: entity.RoleAsisten, Scope: scope})
	}
	return p
}
func newPracticumHarness(t *testing.T, kind, mode string, token bool) *practicumHarness {
	t.Helper()
	h := &practicumHarness{clock: &practicumTestClock{now: time.Date(2026, 6, 1, 23, 58, 0, 0, time.FixedZone("test +07", 7*3600))}}
	h.repo = repository.NewMemoryPracticumRepository(h.clock.Now)
	h.audit = repository.NewMemoryIdentityRepository()
	h.repo.BindIdentity(h.audit)
	scope := entity.Scope{Domain: "praktikum", Institution: "lab", Division: "pendidikan"}
	h.admin = practicumStaff(scope, practicumManage, practicumQuestions, practicumGrade)
	identity := practicumTestIdentity{students: map[int64]*entity.Principal{}}
	for _, id := range []int64{1, 2, 3} {
		p := &entity.Principal{User: entity.UserProfile{ID: id, Active: true, Name: fmt.Sprintf("Synthetic %d", id)}, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true, Scope: scope}}}
		identity.students[id] = p
		h.students = append(h.students, p)
	}
	h.s = NewPracticumService(PracticumOptions{Repository: h.repo, IdentityRepository: identity})
	var err error
	h.session, err = h.s.CreateSession(t.Context(), h.admin, practicumCmd("session"), PracticumSessionInput{Title: "Synthetic session", Sequence: 1, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	h.course, err = h.s.CreateCourse(t.Context(), h.admin, practicumCmd("course"), PracticumCourseInput{SessionID: h.session.ID, Title: "Synthetic course", Kind: kind, DurationMinutes: 10, TokenRequired: token})
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"essay", "coding"} {
		q := PracticumQuestionInput{CourseID: h.course.ID, Type: typ, Difficulty: "easy", Prompt: "Original " + typ, Points: 10, ReferenceAnswer: "private-reference", Rubric: []entity.Criterion{{ID: "logic", Description: "private-rubric", MaxScore: 10}}}
		if typ == "coding" {
			q.Language = "c"
			q.StarterCode = "/* starter */"
		}
		question, err := h.s.SaveQuestion(t.Context(), h.admin, practicumCmd(typ), "", PracticumQuestionUpdate{PracticumQuestionInput: q})
		if err != nil {
			t.Fatal(err)
		}
		h.questions = append(h.questions, question)
	}
	activationScope := h.session.Scope
	activationScope.ClassID, activationScope.Shift = "class-a", 1
	h.activation, err = h.s.Activate(t.Context(), h.admin, practicumCmd("activate"), h.session.ID, PracticumActivationInput{CourseID: h.course.ID, Scope: activationScope, TimerMode: mode, StudentIDs: []int64{1, 2}, EntryApprovedIDs: []int64{1}, QuestionIDs: []string{h.questions[0].ID, h.questions[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func (h *practicumHarness) start(t *testing.T, user int) entity.PracticumAttempt {
	t.Helper()
	a, err := h.s.Start(t.Context(), h.students[user], practicumCmd("start"), PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func practicumRows(t *testing.T, repo repository.PracticumRepository, kind repository.PracticumKind) []repository.PracticumRecord {
	t.Helper()
	var rows []repository.PracticumRecord
	if err := repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		var err error
		rows, err = tx.List(kind, repository.PracticumFilter{Limit: 1000})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}
func practicumRecord[T any](t *testing.T, repo repository.PracticumRepository, kind repository.PracticumKind, id string) T {
	t.Helper()
	var value T
	if err := repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		var err error
		value, err = practicumRead[T](tx, kind, id, false)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return value
}
func expectPracticumError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
func expectPracticumHidden(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, authorization.ErrNotFound) && !authorization.Is(err, "NOT_FOUND") {
		t.Fatalf("expected hidden scope denial, got %v", err)
	}
}

func TestPracticumStartResumeSnapshotsAndServerTime(t *testing.T) {
	for _, mode := range []string{"shared", "individual"} {
		t.Run(mode, func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", mode, true)
			first := h.start(t, 0)
			if first.Timezone != "Asia/Jakarta" || first.TimezoneOffset != "+07:00" || first.StartedAt.Location() != time.UTC || !first.ServerNow.Equal(h.clock.Now()) || first.DeadlineAt.Sub(first.StartedAt) != 10*time.Minute {
				t.Fatalf("invalid server timer: %+v", first)
			}
			if got := first.DeadlineAt.In(time.FixedZone("Jakarta", 7*3600)).Format("2006-01-02 15:04"); got != "2026-06-02 00:08" {
				t.Fatalf("GMT+7 midnight rollover = %s", got)
			}
			q := h.questions[0]
			_, err := h.s.SaveQuestion(t.Context(), h.admin, practicumCmd("edit-question"), q.ID, PracticumQuestionUpdate{Version: q.Version, PracticumQuestionInput: PracticumQuestionInput{CourseID: q.CourseID, Type: q.Type, Difficulty: q.Difficulty, Prompt: "Changed", Points: 20, ReferenceAnswer: "changed-reference", Rubric: []entity.Criterion{{ID: "changed", Description: "Changed", MaxScore: 20}}}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = h.s.UpdateCourse(t.Context(), h.admin, practicumCmd("edit-course"), h.course.ID, PracticumCourseUpdate{Version: 1, Title: "Changed course", DurationMinutes: 180})
			if err != nil {
				t.Fatal(err)
			}
			h.clock.Set(h.clock.Now().Add(2 * time.Minute))
			second := h.start(t, 1)
			wantDeadline := first.DeadlineAt
			if mode == "individual" {
				wantDeadline = wantDeadline.Add(2 * time.Minute)
			}
			if !second.DeadlineAt.Equal(wantDeadline) || second.MaxScore != 20 || second.Questions[0].Prompt != "Original essay" {
				t.Fatalf("activation snapshot changed: %+v", second)
			}
			for _, key := range []string{"start", "resume-new-key"} {
				resumed, err := h.s.Start(t.Context(), h.students[0], practicumCmd(key), PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
				if err != nil || resumed.AttemptID != first.AttemptID || !resumed.DeadlineAt.Equal(first.DeadlineAt) || !resumed.ServerNow.Equal(h.clock.Now()) {
					t.Fatalf("resume changed timer/identity: %+v %v", resumed, err)
				}
			}
			if len(practicumRows(t, h.repo, repository.PracticumAttempts)) != 2 {
				t.Fatal("start duplicated an attempt")
			}
			review, err := h.s.Review(t.Context(), h.admin, first.AttemptID, practicumCmd("review"))
			if err != nil || review.Questions[0].ReferenceAnswer != "private-reference" || review.Questions[0].Rubric[0].MaxScore != 10 {
				t.Fatalf("private snapshot lost: %+v %v", review, err)
			}
			wire, _ := json.Marshal(second)
			for _, private := range []string{"reference_answer", "rubric", "token_hash", h.activation.AccessToken} {
				if strings.Contains(string(wire), private) {
					t.Fatalf("student response exposed %q", private)
				}
			}
			detail, err := h.s.SessionDetail(t.Context(), h.students[0], h.session.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			wire, _ = json.Marshal(detail)
			for _, private := range []string{"student_ids", "entry_approved_ids", "access_token", "token_hash"} {
				if strings.Contains(string(wire), private) {
					t.Fatalf("student detail exposed %q", private)
				}
			}
		})
	}
}

func TestPracticumEligibilityAndScopeDenialBeforeWrites(t *testing.T) {
	h := newPracticumHarness(t, "ujian_praktik", "shared", true)
	input := PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken}
	_, err := h.s.Start(t.Context(), nil, practicumCmd("anonymous"), input)
	expectPracticumError(t, err, authorization.ErrUnauthenticated)
	_, err = h.s.Start(t.Context(), h.students[2], practicumCmd("not-rostered"), input)
	expectPracticumHidden(t, err)
	_, err = h.s.Start(t.Context(), h.students[1], practicumCmd("no-entry"), input)
	expectPracticumError(t, err, authorization.ErrForbidden)
	_, err = h.s.Start(t.Context(), h.students[0], practicumCmd("bad-token"), PracticumStartInput{ActivationID: h.activation.ID, AccessToken: "wrong"})
	expectPracticumError(t, err, authorization.ErrForbidden)
	_, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("no-start"), "missing-attempt", PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "not started"}}})
	expectPracticumError(t, err, repository.ErrPracticumNotFound)
	if len(practicumRows(t, h.repo, repository.PracticumAttempts)) != 0 {
		t.Fatal("ineligible request created attempt")
	}
	a := h.start(t, 0)
	for name, narrow := range map[string]entity.Scope{
		"class": {Domain: "praktikum", ClassID: "class-b"}, "shift": {Domain: "praktikum", Shift: 2}, "session": {Domain: "praktikum", SessionID: "another-session"}, "domain": {Domain: "elearning"}, "institution": {Institution: "other"}, "division": {Division: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			student := *h.students[0]
			student.Assignments = []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true, Scope: narrow}}
			_, err := h.s.Start(t.Context(), &student, practicumCmd("scope-start"), input)
			expectPracticumHidden(t, err)
			_, err = h.s.Attempt(t.Context(), &student, a.AttemptID, practicumCmd("scope-read"))
			expectPracticumHidden(t, err)
			staff := practicumStaff(narrow, practicumManage, practicumQuestions, practicumGrade)
			_, err = h.s.Close(t.Context(), staff, practicumCmd("scope-close"), h.session.ID, h.course.ID, h.activation.ID)
			expectPracticumHidden(t, err)
			_, err = h.s.Review(t.Context(), staff, a.AttemptID, practicumCmd("scope-review"))
			expectPracticumHidden(t, err)
			rows, err := h.s.Monitoring(t.Context(), staff, "", "", PracticumPage{Limit: 25})
			if err != nil || len(rows) != 0 {
				t.Fatalf("monitoring scope leak: %+v %v", rows, err)
			}
		})
	}
	for _, mutate := range []func(*entity.Principal){func(p *entity.Principal) { p.User.Active = false }, func(p *entity.Principal) { p.Assignments[0].Active = false }, func(p *entity.Principal) { p.Assignments[0].Role = entity.RoleAsisten }} {
		student := *h.students[0]
		student.Assignments = append([]entity.RoleAssignment{}, student.Assignments...)
		mutate(&student)
		if _, err := h.s.Start(t.Context(), &student, practicumCmd("inactive"), input); err == nil {
			t.Fatal("inactive/non-student assignment started")
		}
	}
	_, err = h.s.Attempt(t.Context(), h.students[1], a.AttemptID, practicumCmd("other-owner"))
	expectPracticumHidden(t, err)
	_, err = h.s.SaveAnswers(t.Context(), h.students[1], practicumCmd("other-save"), a.AttemptID, PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "wrong owner"}}})
	expectPracticumHidden(t, err)
	_, err = h.s.Submit(t.Context(), h.students[1], practicumCmd("other-submit"), a.AttemptID)
	expectPracticumHidden(t, err)
	classStaff := practicumStaff(h.activation.Scope, practicumManage, practicumQuestions)
	_, err = h.s.UpdateCourse(t.Context(), classStaff, practicumCmd("shared-definition"), h.course.ID, PracticumCourseUpdate{Version: 1, Title: "Wrong", DurationMinutes: 1})
	expectPracticumHidden(t, err)
	_, err = h.s.Questions(t.Context(), classStaff, h.course.ID, PracticumPage{Limit: 25})
	expectPracticumHidden(t, err)
	_, err = h.s.Review(t.Context(), classStaff, a.AttemptID, practicumCmd("manage-not-grade"))
	expectPracticumHidden(t, err)
	for _, p := range []*entity.Principal{h.admin, h.students[2]} {
		rows, err := h.s.Sessions(t.Context(), p, false, PracticumPage{Limit: 25})
		if err != nil || len(rows) != 0 {
			t.Fatalf("ineligible sessions leaked: %+v %v", rows, err)
		}
	}
	if got := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID); got.Attempt.Status != "in_progress" || got.Attempt.Revision != 0 {
		t.Fatal("denied action mutated attempt")
	}
}

func TestPracticumAutosaveRevisionIdempotencyAndValidation(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.start(t, 0)
	input := PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "sensitive-answer"}, {QuestionID: h.questions[1].ID, Value: "sensitive-source"}}}
	first, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save"), a.AttemptID, input)
	if err != nil || first.Revision != 1 || first.Answers[1].Language != "c" {
		t.Fatalf("batch revision/language: %+v %v", first, err)
	}
	secondInput := PracticumSaveInput{Revision: 1, Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "newer"}}}
	second, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save-new"), a.AttemptID, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save"), a.AttemptID, input)
	if err != nil || !reflect.DeepEqual(replay, second) {
		t.Fatalf("replay returned obsolete state: %+v %v", replay, err)
	}
	_, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save-stale"), a.AttemptID, input)
	expectPracticumError(t, err, repository.ErrConflict)
	_, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save"), a.AttemptID, secondInput)
	expectPracticumError(t, err, repository.ErrConflict)
	for name, answers := range map[string][]PracticumAnswerInput{
		"unknown": {{QuestionID: "not-selected", Value: "x"}}, "duplicate": {{QuestionID: h.questions[0].ID}, {QuestionID: h.questions[0].ID}}, "oversize": {{QuestionID: h.questions[0].ID, Value: strings.Repeat("x", (64<<10)+1)}}, "essay-language": {{QuestionID: h.questions[0].ID, Language: "c"}}, "coding-language": {{QuestionID: h.questions[1].ID, Language: "python"}}, "partial-invalid": {{QuestionID: h.questions[0].ID, Value: "must-rollback"}, {QuestionID: "unknown", Value: "x"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd(name), a.AttemptID, PracticumSaveInput{Revision: 2, Answers: answers})
			expectPracticumError(t, err, ErrValidation)
		})
	}
	got, err := h.s.Attempt(t.Context(), h.students[0], a.AttemptID, practicumCmd("get"))
	if err != nil || !reflect.DeepEqual(got, second) {
		t.Fatal("rejected autosave modified answer/revision")
	}
	_, err = h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	replay, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save"), a.AttemptID, input)
	if err != nil || replay.Status != "submitted" || replay.Revision != 2 {
		t.Fatalf("post-submit replay: %+v %v", replay, err)
	}
	_, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("after-submit"), a.AttemptID, PracticumSaveInput{Revision: 2, Answers: input.Answers})
	expectPracticumError(t, err, ErrPracticumClosed)
	record := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID)
	if !reflect.DeepEqual(record.SubmittedAnswers, second.Answers) {
		t.Fatal("submission did not freeze latest accepted batch")
	}
	audits, err := h.audit.ListAudit(t.Context(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(audits)
	for _, secret := range []string{"sensitive-answer", "sensitive-source", "private-reference", "private-rubric"} {
		if strings.Contains(string(wire), secret) {
			t.Fatalf("audit leaked %s", secret)
		}
	}
	for _, event := range audits {
		if strings.HasPrefix(event.Action, "practicum.") && (event.RequestID != "req-test" || event.OperationID != "op-test" || event.SubjectID == "" || event.Scope.Domain != "praktikum" || event.CreatedAt.IsZero()) {
			t.Fatalf("missing transactional audit metadata: %+v", event)
		}
	}
}

func TestPracticumDeadlineBoundaryAndClosingBound(t *testing.T) {
	for _, mode := range []string{"shared", "individual"} {
		t.Run(mode, func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", mode, false)
			closeAt := h.clock.Now().Add(3 * time.Minute)
			activation, err := h.s.Activate(t.Context(), h.admin, practicumCmd("bounded"), h.session.ID, PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: mode, StudentIDs: []int64{1, 2}, QuestionIDs: []string{h.questions[0].ID}, ClosesAt: &closeAt})
			if err != nil {
				t.Fatal(err)
			}
			h.activation = activation
			a := h.start(t, 0)
			if !a.DeadlineAt.Equal(closeAt) || a.DeadlineAt.Location() != time.UTC {
				t.Fatal("closing bound not normalized/applied")
			}
			h.clock.Set(closeAt.Add(-time.Nanosecond))
			before, err := h.s.Attempt(t.Context(), h.students[0], a.AttemptID, practicumCmd("before"))
			if err != nil || before.Status != "in_progress" {
				t.Fatal("expired before exact boundary")
			}
			h.clock.Set(closeAt)
			_, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("late-save"), a.AttemptID, PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "late"}}})
			expectPracticumError(t, err, ErrPracticumClosed)
			got, err := h.s.Start(t.Context(), h.students[0], practicumCmd("start"), PracticumStartInput{ActivationID: activation.ID})
			if err != nil || got.Status != "submitted" || got.SubmissionReason != "deadline" || got.Revision != 0 || len(got.Answers) != 0 {
				t.Fatalf("deadline reconciliation/replay: %+v %v", got, err)
			}
			_, err = h.s.Start(t.Context(), h.students[1], practicumCmd("too-late"), PracticumStartInput{ActivationID: activation.ID})
			expectPracticumError(t, err, ErrPracticumClosed)
			if len(practicumRows(t, h.repo, repository.PracticumJobs)) != 1 {
				t.Fatal("deadline did not enqueue exactly once")
			}
		})
	}
}

func TestPracticumConcurrentAutosaveSubmitDeadlineAndClose(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("expired=%v", expired), func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "shared", false)
			a := h.start(t, 0)
			h.start(t, 1)
			// Competing whole-attempt revisions must accept exactly one batch.
			errs := make(chan error, 8)
			gate := make(chan struct{})
			for i := range 8 {
				go func() {
					<-gate
					_, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd(fmt.Sprintf("concurrent-%d", i)), a.AttemptID, PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: fmt.Sprint(i)}}})
					errs <- err
				}()
			}
			close(gate)
			accepted := 0
			for range 8 {
				err := <-errs
				if err == nil {
					accepted++
				} else {
					expectPracticumError(t, err, repository.ErrConflict)
				}
			}
			if accepted != 1 {
				t.Fatalf("accepted %d stale competing saves", accepted)
			}
			if expired {
				h.clock.Set(a.DeadlineAt)
			}
			gate = make(chan struct{})
			errs = make(chan error, 12)
			for i := range 12 {
				go func() {
					<-gate
					var err error
					switch i % 3 {
					case 0:
						_, err = h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID)
					case 1:
						_, err = h.s.Close(t.Context(), h.admin, practicumCmd("close"), h.session.ID, h.course.ID, h.activation.ID)
					case 2:
						_, err = h.s.Reconcile(t.Context(), practicumCmd("cron"))
					}
					errs <- err
				}()
			}
			close(gate)
			for range 12 {
				if err := <-errs; err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range practicumRows(t, h.repo, repository.PracticumAttempts) {
				var record entity.PracticumAttemptRecord
				if err := json.Unmarshal(row.Data, &record); err != nil {
					t.Fatal(err)
				}
				if record.Attempt.Status != "submitted" || record.Attempt.SubmittedAt == nil {
					t.Fatalf("invalid terminal state: %+v", record.Attempt)
				}
				if expired && record.Attempt.SubmissionReason != "deadline" {
					t.Fatal("deadline lost to manual/close")
				}
				if record.Attempt.AttemptID == a.AttemptID && (record.Attempt.Revision != 1 || len(record.SubmittedAnswers) != 1 || !reflect.DeepEqual(record.SubmittedAnswers, record.Attempt.Answers)) {
					t.Fatal("race lost submitted snapshot")
				}
			}
			if len(practicumRows(t, h.repo, repository.PracticumJobs)) != 2 {
				t.Fatal("duplicate/lost grading job")
			}
			audits, _ := h.audit.ListAudit(t.Context(), 1000)
			counts := map[string]int{}
			for _, event := range audits {
				if strings.HasPrefix(event.Action, "practicum.attempt.submitted.") {
					counts[event.SubjectID]++
				}
			}
			if len(counts) != 2 {
				t.Fatal("missing submission audit")
			}
			for _, count := range counts {
				if count != 1 {
					t.Fatal("duplicate submission audit")
				}
			}
			closed := practicumRecord[entity.PracticumActivationRecord](t, h.repo, repository.PracticumActivations, h.activation.ID)
			if closed.Activation.IsOpen {
				t.Fatal("close was lost")
			}
		})
	}
}

func TestPracticumActivationValidationAndOneTimeTokenDisclosure(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "shared", true)
	input := PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "shared", StudentIDs: []int64{1, 2}, EntryApprovedIDs: []int64{1}, QuestionIDs: []string{h.questions[0].ID, h.questions[1].ID}}
	replay, err := h.s.Activate(t.Context(), h.admin, practicumCmd("activate"), h.session.ID, input)
	if err != nil || replay.ID != h.activation.ID || h.activation.AccessToken == "" || replay.AccessToken != "" {
		t.Fatal("activation must disclose plain token only on first creation")
	}
	stored := practicumRecord[entity.PracticumActivationRecord](t, h.repo, repository.PracticumActivations, h.activation.ID)
	if stored.TokenHash == "" || stored.Activation.AccessToken != "" {
		t.Fatal("activation did not hash access token")
	}
	for name, mutate := range map[string]func(*PracticumActivationInput){
		"duplicate-student":   func(in *PracticumActivationInput) { in.StudentIDs = []int64{1, 1} },
		"unknown-student":     func(in *PracticumActivationInput) { in.StudentIDs = []int64{99999} },
		"unrostered-approval": func(in *PracticumActivationInput) { in.EntryApprovedIDs = []int64{3} },
		"duplicate-question":  func(in *PracticumActivationInput) { in.QuestionIDs = []string{h.questions[0].ID, h.questions[0].ID} },
		"missing-class":       func(in *PracticumActivationInput) { in.Scope.ClassID = "" },
		"missing-shift":       func(in *PracticumActivationInput) { in.Scope.Shift = 0 },
		"timer-mode":          func(in *PracticumActivationInput) { in.TimerMode = "reset-on-refresh" },
		"past-close":          func(in *PracticumActivationInput) { now := h.clock.Now(); in.ClosesAt = &now },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := input
			mutate(&invalid)
			_, err := h.s.Activate(t.Context(), h.admin, practicumCmd(name), h.session.ID, invalid)
			expectPracticumError(t, err, ErrValidation)
		})
	}
	student := h.students[0]
	student.Assignments[0].Scope.ClassID = "other-class"
	_, err = h.s.Activate(t.Context(), h.admin, practicumCmd("ineligible-roster"), h.session.ID, input)
	expectPracticumError(t, err, ErrValidation)
	student.Assignments[0].Scope.ClassID = ""
	student.User.Active = false
	_, err = h.s.Activate(t.Context(), h.admin, practicumCmd("inactive-roster"), h.session.ID, input)
	expectPracticumError(t, err, ErrValidation)
	student.User.Active = true
	outside := input
	outside.Scope.Institution = "another-lab"
	_, err = h.s.Activate(t.Context(), practicumStaff(entity.Scope{Domain: "praktikum"}, practicumManage), practicumCmd("outside-session"), h.session.ID, outside)
	expectPracticumHidden(t, err)
	_, err = h.s.CreateCourse(t.Context(), h.admin, practicumCmd("duplicate-kind"), PracticumCourseInput{SessionID: h.session.ID, Title: "duplicate", Kind: "pretest", DurationMinutes: 10})
	expectPracticumError(t, err, repository.ErrConflict)
	for _, key := range []string{"", strings.Repeat("x", 129), "bad\nkey"} {
		_, err := h.s.Start(t.Context(), student, practicumCmd(key), PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
		expectPracticumError(t, err, ErrValidation)
	}
	if len(practicumRows(t, h.repo, repository.PracticumActivations)) != 1 || len(practicumRows(t, h.repo, repository.PracticumAttempts)) != 0 {
		t.Fatal("invalid activation/start left partial state")
	}
}

func TestPracticumAnswerAggregateLimitAcrossBatches(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	ids := []string{}
	for i := range 5 {
		q, err := h.s.SaveQuestion(t.Context(), h.admin, practicumCmd(fmt.Sprintf("large-question-%d", i)), "", PracticumQuestionUpdate{PracticumQuestionInput: PracticumQuestionInput{CourseID: h.course.ID, Type: "essay", Difficulty: "easy", Prompt: "Synthetic", Points: 1, Rubric: []entity.Criterion{{ID: "one", Description: "One point", MaxScore: 1}}}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, q.ID)
	}
	a, err := h.s.Activate(t.Context(), h.admin, practicumCmd("large-activation"), h.session.ID, PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "individual", StudentIDs: []int64{1}, QuestionIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	h.activation = a
	attempt := h.start(t, 0)
	for i, id := range ids {
		_, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd(fmt.Sprintf("large-save-%d", i)), attempt.AttemptID, PracticumSaveInput{Revision: int64(i), Answers: []PracticumAnswerInput{{QuestionID: id, Value: strings.Repeat("x", 64<<10)}}})
		if i < 4 {
			if err != nil {
				t.Fatal(err)
			}
		} else {
			expectPracticumError(t, err, ErrValidation)
		}
	}
	got, err := h.s.Attempt(t.Context(), h.students[0], attempt.AttemptID, practicumCmd("large-refresh"))
	if err != nil || got.Revision != 4 || len(got.Answers) != 4 {
		t.Fatal("aggregate overflow overwrote prior accepted answers")
	}
}

type practicumAuditFailureRepo struct{ repository.PracticumRepository }
type practicumAuditFailureTx struct{ repository.PracticumTx }

func (practicumAuditFailureTx) Audit(entity.AuditEvent) error {
	return errors.New("synthetic audit outage")
}
func (r practicumAuditFailureRepo) Transaction(ctx context.Context, fn func(repository.PracticumTx) error) error {
	return r.PracticumRepository.Transaction(ctx, func(tx repository.PracticumTx) error { return fn(practicumAuditFailureTx{tx}) })
}
func TestPracticumRequiredAuditFailureRollsBackAggregate(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "shared", false)
	a := h.start(t, 0)
	h.s.options.Repository = practicumAuditFailureRepo{h.repo}
	if _, err := h.s.Submit(t.Context(), h.students[0], practicumCmd("rollback"), a.AttemptID); err == nil {
		t.Fatal("audit outage ignored")
	}
	record := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID)
	if record.Attempt.Status != "in_progress" || len(practicumRows(t, h.repo, repository.PracticumJobs)) != 0 {
		t.Fatal("failed transaction committed attempt/job")
	}
}

func TestPracticumGradesKeepOwnAttemptAfterScopeChange(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	h.start(t, 0)

	before, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 25})
	if err != nil || len(before) != 1 {
		t.Fatalf("baseline grades: %+v %v", before, err)
	}

	// Simulate a legacy shift change: the student's current role scope no longer
	// contains the activation scope. Their own submitted work must still appear.
	h.students[0].Assignments[0].Scope.Shift = 2
	after, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 25})
	if err != nil || len(after) != 1 {
		t.Fatalf("own attempt hidden after scope change: %+v %v", after, err)
	}

	// Cross-user isolation must be preserved.
	other, err := h.s.Grades(t.Context(), h.students[1], PracticumPage{Limit: 25})
	if err != nil || len(other) != 0 {
		t.Fatalf("cross-user grade leak: %+v %v", other, err)
	}
}
