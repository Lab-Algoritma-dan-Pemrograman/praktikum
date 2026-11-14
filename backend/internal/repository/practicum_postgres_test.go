package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgPracticumHarness struct {
	ctx        context.Context
	pool       *pgxpool.Pool
	repo       *repository.PostgresPracticumRepository
	service    *usecase.PracticumService
	admin      *entity.Principal
	students   []*entity.Principal
	session    entity.PracticumSession
	course     entity.PracticumCourse
	questions  []entity.PracticumQuestion
	activation entity.PracticumActivation
}

func pgPracticumCommand(key string) usecase.PracticumCommand {
	return usecase.PracticumCommand{Key: key, RequestID: "req-pg-practicum", OperationID: "op-pg-practicum"}
}
func newPGPracticum(t *testing.T) *pgPracticumHarness {
	t.Helper()
	ctx, pool := repository.OpenTestDatabase(t)
	h := &pgPracticumHarness{ctx: ctx, pool: pool, repo: repository.NewPostgresPracticumRepository(pool)}
	identity := repository.NewPostgresIdentityRepository(pool)
	scope := entity.Scope{Domain: "praktikum", Institution: "synthetic-lab", Division: "pendidikan"}
	var authIDs []string
	var userIDs []int64
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, `delete from public.audit_logs where actor_id=any($1::bigint[]) or scope->>'session_id'=$2`, userIDs, h.session.ID); err != nil {
			t.Error(err)
		}
		for _, table := range []string{"grading_jobs", "runner_leases", "operations", "attempts", "activations", "questions", "courses", "sessions"} {
			if _, err := pool.Exec(cleanup, `delete from public.practicum_`+table+` where scope->>'session_id'=$1 or owner_id=any($2::bigint[])`, h.session.ID, userIDs); err != nil {
				t.Error(err)
			}
		}
		if _, err := pool.Exec(cleanup, `delete from auth.users where id=any($1::uuid[])`, authIDs); err != nil {
			t.Error(err)
		}
	})
	for i := range 3 {
		var authID string
		if err := pool.QueryRow(ctx, `insert into auth.users(id) values(gen_random_uuid()) returning id::text`).Scan(&authID); err != nil {
			t.Fatal(err)
		}
		authIDs = append(authIDs, authID)
		user, err := identity.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: authID, Email: authID + "@example.test", Name: "Synthetic practicum user"}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, user.ID)
		role := entity.RoleMahasiswa
		if i == 0 {
			role = entity.RoleAsisten
		}
		assignment := entity.RoleAssignment{Role: role, Scope: scope, Active: true}
		if err := identity.ReplaceAccess(ctx, user.ID, entity.UserAccess{Version: user.AccessVersion, Assignments: []entity.RoleAssignment{assignment}}); err != nil {
			t.Fatal(err)
		}
		p := &entity.Principal{User: user, Assignments: []entity.RoleAssignment{assignment}}
		if i == 0 {
			for _, permission := range []string{"practicum.session.manage", "question.manage", "practicum.grade.write"} {
				p.Grants = append(p.Grants, entity.PermissionGrant{Permission: permission, Role: role, Scope: scope})
			}
			h.admin = p
		} else {
			h.students = append(h.students, p)
		}
	}
	h.service = usecase.NewPracticumService(usecase.PracticumOptions{Repository: h.repo, IdentityRepository: identity})
	var err error
	h.session, err = h.service.CreateSession(ctx, h.admin, pgPracticumCommand("session"), usecase.PracticumSessionInput{Title: "Synthetic database assessment", Sequence: 1, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	h.course, err = h.service.CreateCourse(ctx, h.admin, pgPracticumCommand("course"), usecase.PracticumCourseInput{SessionID: h.session.ID, Title: "Synthetic pretest", Kind: "pretest", DurationMinutes: 10, TokenRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"essay", "coding"} {
		in := usecase.PracticumQuestionInput{CourseID: h.course.ID, Type: typ, Difficulty: "easy", Prompt: "Original " + typ, Points: 10, ReferenceAnswer: "private-rules", Rubric: []entity.Criterion{{ID: "logic", Description: "private-rubric", MaxScore: 10}}}
		if typ == "coding" {
			in.Language = "c"
		}
		q, err := h.service.SaveQuestion(ctx, h.admin, pgPracticumCommand(typ), "", usecase.PracticumQuestionUpdate{PracticumQuestionInput: in})
		if err != nil {
			t.Fatal(err)
		}
		h.questions = append(h.questions, q)
	}
	scope = h.session.Scope
	scope.ClassID = "class-a"
	scope.Shift = 1
	h.activation, err = h.service.Activate(ctx, h.admin, pgPracticumCommand("activate"), h.session.ID, usecase.PracticumActivationInput{CourseID: h.course.ID, Scope: scope, TimerMode: "shared", StudentIDs: []int64{h.students[0].User.ID, h.students[1].User.ID}, QuestionIDs: []string{h.questions[0].ID, h.questions[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func (h *pgPracticumHarness) start(t *testing.T, user int) entity.PracticumAttempt {
	t.Helper()
	a, err := h.service.Start(h.ctx, h.students[user], pgPracticumCommand("start"), usecase.PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func pgPracticumRead[T any](t *testing.T, h *pgPracticumHarness, kind repository.PracticumKind, id string) T {
	t.Helper()
	var out T
	if err := h.repo.Transaction(h.ctx, func(tx repository.PracticumTx) error {
		r, err := tx.Get(kind, id)
		if err != nil {
			return err
		}
		return json.Unmarshal(r.Data, &out)
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
func pgPracticumCount(t *testing.T, h *pgPracticumHarness, table string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.practicum_`+table+` where scope->>'session_id'=$1`, h.session.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPostgresPracticumConcurrentStartAutosaveAndSnapshots(t *testing.T) {
	h := newPGPracticum(t)
	// A separate pool models another serverless API instance, not a Go mutex.
	pool2, err := pgxpool.NewWithConfig(h.ctx, h.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer pool2.Close()
	other := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(pool2)})
	type reply struct {
		a   entity.PracticumAttempt
		err error
	}
	started := make(chan reply, 12)
	gate := make(chan struct{})
	for i := range 12 {
		go func() {
			<-gate
			s := h.service
			if i%2 == 0 {
				s = other
			}
			a, err := s.Start(h.ctx, h.students[0], pgPracticumCommand(fmt.Sprintf("start-%d", i)), usecase.PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
			started <- reply{a, err}
		}()
	}
	close(gate)
	var first entity.PracticumAttempt
	for range 12 {
		got := <-started
		if got.err != nil {
			t.Fatal(got.err)
		}
		if first.AttemptID == "" {
			first = got.a
		}
		if got.a.AttemptID != first.AttemptID || !got.a.DeadlineAt.Equal(first.DeadlineAt) {
			t.Fatal("concurrent start reset/duplicated attempt")
		}
	}
	if pgPracticumCount(t, h, "attempts") != 1 {
		t.Fatal("duplicate attempt rows")
	}
	if first.Timezone != "Asia/Jakarta" || first.TimezoneOffset != "+07:00" || first.DeadlineAt.Sub(first.StartedAt) != 10*time.Minute {
		t.Fatal("database timer metadata incorrect")
	}
	q := h.questions[0]
	_, err = h.service.SaveQuestion(h.ctx, h.admin, pgPracticumCommand("edit-question"), q.ID, usecase.PracticumQuestionUpdate{Version: 1, PracticumQuestionInput: usecase.PracticumQuestionInput{CourseID: q.CourseID, Type: q.Type, Difficulty: q.Difficulty, Prompt: "Changed after activation", Points: 20, ReferenceAnswer: "changed", Rubric: []entity.Criterion{{ID: "changed", Description: "changed", MaxScore: 20}}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.service.UpdateCourse(h.ctx, h.admin, pgPracticumCommand("edit-course"), h.course.ID, usecase.PracticumCourseUpdate{Version: 1, Title: "Changed", DurationMinutes: 180})
	if err != nil {
		t.Fatal(err)
	}
	second := h.start(t, 1)
	if !second.DeadlineAt.Equal(first.DeadlineAt) || second.MaxScore != 20 || second.Questions[0].Prompt != "Original essay" || second.Questions[0].ReferenceAnswer != "" || len(second.Questions[0].Rubric) != 0 {
		t.Fatal("snapshot changed or private rules leaked")
	}
	input := usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: q.ID, Value: "synthetic submitted answer"}}}
	errs := make(chan error, 10)
	gate = make(chan struct{})
	for range 10 {
		go func() {
			<-gate
			_, err := other.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("same-save"), first.AttemptID, input)
			errs <- err
		}()
	}
	close(gate)
	for range 10 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	record := pgPracticumRead[entity.PracticumAttemptRecord](t, h, repository.PracticumAttempts, first.AttemptID)
	if record.Attempt.Revision != 1 || len(record.Attempt.Answers) != 1 {
		t.Fatal("duplicate autosave changed revision")
	}
	_, err = h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("stale"), first.AttemptID, input)
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("stale save accepted: %v", err)
	}
	input.Revision = 1
	input.Answers[0].Value = "newer batch"
	_, err = h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("same-save"), first.AttemptID, input)
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("different idempotency payload accepted: %v", err)
	}
	gate = make(chan struct{})
	errs = make(chan error, 8)
	for i := range 8 {
		go func() {
			<-gate
			_, err := other.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand(fmt.Sprintf("revision-%d", i)), first.AttemptID, input)
			errs <- err
		}()
	}
	close(gate)
	accepted := 0
	for range 8 {
		err := <-errs
		if err == nil {
			accepted++
		} else if !errors.Is(err, repository.ErrConflict) {
			t.Fatal(err)
		}
	}
	if accepted != 1 {
		t.Fatalf("concurrent revision winners=%d", accepted)
	}
	foreign := *h.students[0]
	foreign.Assignments = []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true, Scope: entity.Scope{Domain: "praktikum", ClassID: "class-b"}}}
	_, err = other.Attempt(h.ctx, &foreign, first.AttemptID, pgPracticumCommand("cross-class"))
	if !errors.Is(err, authorization.ErrNotFound) {
		t.Fatalf("cross-class allowed: %v", err)
	}
	_, err = other.Submit(h.ctx, h.students[1], pgPracticumCommand("cross-user"), first.AttemptID)
	if !errors.Is(err, authorization.ErrNotFound) {
		t.Fatalf("cross-user allowed: %v", err)
	}
}

func TestPostgresPracticumSubmitCloseRaceAndProviderCAS(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	h.start(t, 1)
	_, err := h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("save"), a.AttemptID, usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "frozen answer"}}})
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 12)
	gate := make(chan struct{})
	for i := range 12 {
		go func() {
			<-gate
			var err error
			switch i % 3 {
			case 0:
				_, err = h.service.Submit(h.ctx, h.students[0], pgPracticumCommand("submit"), a.AttemptID)
			case 1:
				_, err = h.service.Close(h.ctx, h.admin, pgPracticumCommand("close"), h.session.ID, h.course.ID, h.activation.ID)
			case 2:
				_, err = h.service.Reconcile(h.ctx, pgPracticumCommand("cron"))
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
	record := pgPracticumRead[entity.PracticumAttemptRecord](t, h, repository.PracticumAttempts, a.AttemptID)
	if record.Attempt.Status != "submitted" || record.Attempt.SubmittedAt == nil || !reflect.DeepEqual(record.SubmittedAnswers, record.Attempt.Answers) || len(record.SubmittedAnswers) != 1 {
		t.Fatal("submit/close lost snapshot")
	}
	if pgPracticumCount(t, h, "grading_jobs") != 2 {
		t.Fatal("grading enqueue is not exactly once")
	}
	var submissions int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.audit_logs where subject_id=$1 and action like 'practicum.attempt.submitted.%'`, a.AttemptID).Scan(&submissions); err != nil || submissions != 1 {
		t.Fatalf("submission audit count=%d %v", submissions, err)
	}
	entered := make(chan struct{})
	release := make(chan struct{}, 1)
	defer close(release)
	grader := pgPracticumGrader(func(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
		close(entered)
		select {
		case <-release:
			out := entity.GradeResult{Status: "suggested", Simulated: true}
			for _, q := range r.Items {
				out.Suggestions = append(out.Suggestions, entity.SuggestedGrade{QuestionID: q.QuestionID, Score: 10})
			}
			return out, nil
		case <-ctx.Done():
			return entity.GradeResult{}, ctx.Err()
		}
	})
	service := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(h.pool), Grader: grader})
	done := make(chan error, 1)
	go func() { _, err := service.Grade(h.ctx, h.admin, pgPracticumCommand("grade"), a.AttemptID); done <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("grading provider not called")
	}
	review, err := h.service.ManualGrade(h.ctx, h.admin, pgPracticumCommand("manual"), a.AttemptID, usecase.PracticumManualInput{Publish: true, Reason: "Synthetic manual review", Grades: []entity.PracticumManualGrade{{QuestionID: h.questions[0].ID, Score: 7}, {QuestionID: h.questions[1].ID, Score: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	if review.Attempt.Score == nil || *review.Attempt.Score != 7 || *review.Attempt.Percentage != 35 {
		t.Fatal("manual raw score/percentage mismatch")
	}
	release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	job := pgPracticumRead[entity.PracticumGradingJob](t, h, repository.PracticumJobs, a.AttemptID)
	if job.Status != "graded" || job.GradeRevision != 1 || len(job.Suggestions) != 0 || job.LeaseID != "" {
		t.Fatal("provider completion overwrote manual grade")
	}
	frozen := pgPracticumRead[entity.PracticumAttemptRecord](t, h, repository.PracticumAttempts, a.AttemptID)
	if !reflect.DeepEqual(record.SubmittedAnswers, frozen.SubmittedAnswers) {
		t.Fatal("publication changed submission")
	}
}

type pgPracticumGrader func(context.Context, entity.GradeRequest) (entity.GradeResult, error)

func (f pgPracticumGrader) Grade(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
	return f(ctx, r)
}

func TestPostgresPracticumDeadlineUsesDatabaseTimeAfterRowLock(t *testing.T) {
	h := newPGPracticum(t)
	var closes time.Time
	if err := h.pool.QueryRow(h.ctx, `select clock_timestamp()+interval '1 second'`).Scan(&closes); err != nil {
		t.Fatal(err)
	}
	activation, err := h.service.Activate(h.ctx, h.admin, pgPracticumCommand("short-activation"), h.session.ID, usecase.PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "individual", StudentIDs: []int64{h.students[0].User.ID, h.students[1].User.ID}, QuestionIDs: []string{h.questions[0].ID}, ClosesAt: &closes})
	if err != nil {
		t.Fatal(err)
	}
	h.activation = activation
	a := h.start(t, 0)
	b := h.start(t, 1)
	if !a.DeadlineAt.Equal(closes) {
		t.Fatal("closing bound ignored")
	}
	lock, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback(context.Background())
	if _, err := lock.Exec(h.ctx, `select id from public.practicum_activations where id=$1 for update`, activation.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("late-after-lock"), a.AttemptID, usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "too late"}}})
		done <- err
	}()
	// Keep the row locked across the deadline, using the database clock rather
	// than the test machine's timezone, scheduling or a fabricated deadline.
	if _, err := lock.Exec(h.ctx, `select pg_sleep(greatest(0,extract(epoch from ($1::timestamptz-clock_timestamp())))+0.02)`, closes); err != nil {
		t.Fatal(err)
	}
	if err := lock.Commit(h.ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, usecase.ErrPracticumClosed) {
		t.Fatalf("late save was accepted: %v", err)
	}
	got, err := h.service.Attempt(h.ctx, h.students[0], a.AttemptID, pgPracticumCommand("refresh"))
	if err != nil || got.Status != "submitted" || got.SubmissionReason != "deadline" || got.Revision != 0 || len(got.Answers) != 0 || got.SubmittedAt.Before(closes) || got.ServerNow.Before(closes) {
		t.Fatalf("deadline reconciliation: %+v %v", got, err)
	}
	// At expiration, manual submit, cron auto-submit and instructor close race
	// for the second attempt; all must converge without duplicate jobs.
	raced := make(chan error, 9)
	gate := make(chan struct{})
	for i := range 9 {
		go func() {
			<-gate
			var err error
			switch i % 3 {
			case 0:
				_, err = h.service.Submit(h.ctx, h.students[1], pgPracticumCommand("deadline-submit"), b.AttemptID)
			case 1:
				_, err = h.service.Reconcile(h.ctx, pgPracticumCommand("deadline-cron"))
			case 2:
				_, err = h.service.Close(h.ctx, h.admin, pgPracticumCommand("deadline-close"), h.session.ID, h.course.ID, h.activation.ID)
			}
			raced <- err
		}()
	}
	close(gate)
	for range 9 {
		if err := <-raced; err != nil {
			t.Fatal(err)
		}
	}
	second := pgPracticumRead[entity.PracticumAttemptRecord](t, h, repository.PracticumAttempts, b.AttemptID)
	if second.Attempt.Status != "submitted" || second.Attempt.SubmissionReason != "deadline" {
		t.Fatal("deadline race produced invalid state")
	}
	if pgPracticumCount(t, h, "grading_jobs") != 2 {
		t.Fatal("auto-submit race did not persist one job per attempt")
	}
}

type pgPracticumRunner struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (r *pgPracticumRunner) Run(ctx context.Context, _ entity.RunRequest) (entity.RunResult, error) {
	r.calls.Add(1)
	close(r.entered)
	select {
	case <-r.release:
		return entity.RunResult{Status: "success", Stdout: "synthetic"}, nil
	case <-ctx.Done():
		return entity.RunResult{}, ctx.Err()
	}
}
func (*pgPracticumRunner) Languages() []entity.RunnerLanguage {
	return []entity.RunnerLanguage{{Language: "c", Version: "fake-v1"}}
}
func (*pgPracticumRunner) Available() bool { return true }
func (*pgPracticumRunner) Simulated() bool { return true }
func TestPostgresPracticumRunnerDurableLimitsAndDedup(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	b := h.start(t, 1)
	runner := &pgPracticumRunner{entered: make(chan struct{}), release: make(chan struct{}, 1)}
	defer close(runner.release)
	service := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(h.pool), Runner: runner, RunnerGlobalLimit: 1, RunnerUserLimit: 1})
	other := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(h.pool), Runner: runner, RunnerGlobalLimit: 2, RunnerUserLimit: 1})
	input := usecase.PracticumRunInput{QuestionID: h.questions[1].ID, Source: "synthetic code never executed", Language: "c", Version: "fake-v1"}
	done := make(chan error, 1)
	go func() {
		_, err := service.Run(h.ctx, h.students[0], pgPracticumCommand("run"), a.AttemptID, input)
		done <- err
	}()
	select {
	case <-runner.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not start")
	}
	_, err := other.Run(h.ctx, h.students[0], pgPracticumCommand("run"), a.AttemptID, input)
	if !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("inflight duplicate accepted: %v", err)
	}
	_, err = other.Run(h.ctx, h.students[0], pgPracticumCommand("user-limit"), a.AttemptID, input)
	if !errors.Is(err, usecase.ErrPracticumBusy) {
		t.Fatalf("user limit ignored: %v", err)
	}
	_, err = service.Run(h.ctx, h.students[1], pgPracticumCommand("global-limit"), b.AttemptID, input)
	if !errors.Is(err, usecase.ErrPracticumBusy) {
		t.Fatalf("global limit ignored: %v", err)
	}
	// A bounded burst across users/requests cannot oversubscribe the held lease.
	burst := make(chan error, 16)
	gate := make(chan struct{})
	for i := range 16 {
		go func() {
			<-gate
			user, attempt := h.students[0], a
			if i%2 != 0 {
				user, attempt = h.students[1], b
			}
			_, err := service.Run(h.ctx, user, pgPracticumCommand(fmt.Sprintf("burst-%d", i)), attempt.AttemptID, input)
			burst <- err
		}()
	}
	close(gate)
	for range 16 {
		if err := <-burst; !errors.Is(err, usecase.ErrPracticumBusy) {
			t.Fatalf("runner burst bypassed admission: %v", err)
		}
	}
	if runner.calls.Load() != 1 {
		t.Fatal("rejected burst reached provider")
	}
	runner.release <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	replay, err := other.Run(h.ctx, h.students[0], pgPracticumCommand("run"), a.AttemptID, input)
	if err != nil || replay.Status != "success" || runner.calls.Load() != 1 || !replay.Simulated {
		t.Fatal("durable replay executed duplicate")
	}
	if pgPracticumCount(t, h, "runner_leases") != 1 {
		t.Fatal("duplicate runner leases")
	}
}

func TestPostgresPracticumScopeFilterMatchesDomainPolicy(t *testing.T) {
	h := newPGPracticum(t)
	h.start(t, 0)
	scopes := []entity.Scope{
		{}, {Domain: "praktikum"}, {Domain: "elearning"}, h.activation.Scope,
		{ClassID: "0"}, {Institution: "0"}, {Division: "0"}, {SessionID: "0"},
		{ClassID: "class-a"}, {ClassID: "class-b"}, {ClassID: "*"}, {Shift: 1}, {Shift: 2},
	}
	for _, granted := range scopes {
		for _, resource := range scopes {
			for _, overlap := range []bool{false, true} {
				grantJSON, _ := json.Marshal(granted)
				resourceJSON, _ := json.Marshal(resource)
				want := granted.Contains(resource)
				if overlap {
					_, want = granted.Intersect(resource)
				}
				var got bool
				if err := h.pool.QueryRow(h.ctx, `select public.practicum_scope_matches($1::jsonb,$2::jsonb,$3)`, string(grantJSON), string(resourceJSON), overlap).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("SQL scope mismatch: granted=%s resource=%s overlap=%v got=%v want=%v", grantJSON, resourceJSON, overlap, got, want)
				}
			}
		}
	}
	staff := *h.admin
	staff.Assignments = []entity.RoleAssignment{{Role: entity.RoleAsisten, Active: true, Scope: entity.Scope{Domain: "praktikum", ClassID: "0"}}}
	staff.Grants = []entity.PermissionGrant{{Role: entity.RoleAsisten, Permission: "practicum.session.manage", Scope: staff.Assignments[0].Scope}}
	rows, err := h.service.Monitoring(h.ctx, &staff, "", "", usecase.PracticumPage{Limit: 25})
	if err != nil || len(rows) != 0 {
		t.Fatalf("literal class ID zero expanded monitoring access: count=%d err=%v", len(rows), err)
	}
}

func TestPostgresPracticumConstraintsImmutabilityRollbackAndRLS(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	_, err := h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("save"), a.AttemptID, usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "frozen"}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.service.Submit(h.ctx, h.students[0], pgPracticumCommand("submit"), a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, sql string
		args      []any
	}{
		{"foreign-parent", `insert into public.practicum_courses(id,parent_id,scope,data) values('missing-parent','missing','{}','{"kind":"pretest","duration_minutes":10}')`, nil},
		{"duplicate-course-kind", `insert into public.practicum_courses(id,parent_id,scope,data) select id||'-dup',parent_id,scope,data from public.practicum_courses where id=$1`, []any{h.course.ID}},
		{"invalid-course-kind", `update public.practicum_courses set data=jsonb_set(data,'{kind}','"invalid"') where id=$1`, []any{h.course.ID}},
		{"missing-course-kind", `update public.practicum_courses set data=data-'kind' where id=$1`, []any{h.course.ID}},
		{"invalid-duration", `update public.practicum_courses set data=jsonb_set(data,'{duration_minutes}','181') where id=$1`, []any{h.course.ID}},
		{"invalid-points", `update public.practicum_questions set data=jsonb_set(data,'{points}','0') where id=$1`, []any{h.questions[0].ID}},
		{"missing-points", `update public.practicum_questions set data=data-'points' where id=$1`, []any{h.questions[0].ID}},
		{"activation-roster", `update public.practicum_activations set data=jsonb_set(data,'{activation,student_ids}','[]') where id=$1`, []any{h.activation.ID}},
		{"activation-rules", `update public.practicum_activations set data=jsonb_set(data,'{questions,0,prompt}','"tampered"') where id=$1`, []any{h.activation.ID}},
		{"activation-timer", `update public.practicum_activations set data=jsonb_set(data,'{activation,deadline_at}','"2099-01-01T00:00:00Z"') where id=$1`, []any{h.activation.ID}},
		{"attempt-owner", `update public.practicum_attempts set owner_id=$2 where id=$1`, []any{a.AttemptID, h.students[1].User.ID}},
		{"attempt-deadline", `update public.practicum_attempts set data=jsonb_set(data,'{attempt,deadline_at}','"2099-01-01T00:00:00Z"') where id=$1`, []any{a.AttemptID}},
		{"submitted-snapshot", `update public.practicum_attempts set data=jsonb_set(data,'{submitted_answers}','[]') where id=$1`, []any{a.AttemptID}},
		{"submitted-answer-value", `update public.practicum_attempts set data=jsonb_set(data,'{attempt,answers,0,value}','"tampered"') where id=$1`, []any{a.AttemptID}},
		{"submitted-answer-removal", `update public.practicum_attempts set data=jsonb_set(data,'{attempt,answers}','[]') where id=$1`, []any{a.AttemptID}},
		{"reopen-attempt", `update public.practicum_attempts set status='in_progress' where id=$1`, []any{a.AttemptID}},
		{"duplicate-attempt", `insert into public.practicum_attempts(id,parent_id,owner_id,scope,status,data) select id||'-dup',parent_id,owner_id,scope,status,data from public.practicum_attempts where id=$1`, []any{a.AttemptID}},
		{"duplicate-job", `insert into public.practicum_grading_jobs(id,parent_id,scope,status,data) select id||'-dup',parent_id,scope,status,data from public.practicum_grading_jobs where parent_id=$1`, []any{a.AttemptID}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.pool.Begin(h.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(h.ctx, tc.sql, tc.args...); err == nil {
				t.Fatal("database accepted invalid state")
			}
		})
	}
	_, err = h.service.Close(h.ctx, h.admin, pgPracticumCommand("close"), h.session.ID, h.course.ID, h.activation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `update public.practicum_activations set data=jsonb_set(data,'{activation,is_open}','true') where id=$1`, h.activation.ID); err == nil {
		t.Fatal("closed activation reopened")
	}
	before := pgPracticumCount(t, h, "sessions")
	sentinel := errors.New("synthetic rollback")
	err = h.repo.Transaction(h.ctx, func(tx repository.PracticumTx) error {
		raw, _ := json.Marshal(h.session)
		if err := tx.Put(repository.PracticumSessions, repository.PracticumRecord{ID: h.session.ID + "-rollback", Scope: h.session.Scope, Data: raw}); err != nil {
			return err
		}
		if err := tx.Audit(entity.AuditEvent{ActorID: &h.admin.User.ID, Action: "practicum.rollback.probe", App: "praktikum", SubjectID: h.session.ID, Scope: h.session.Scope, CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || pgPracticumCount(t, h, "sessions") != before {
		t.Fatal("repository transaction did not roll back")
	}
	var audits int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.audit_logs where action='practicum.rollback.probe' and subject_id=$1`, h.session.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("audit escaped rolled-back transaction")
	}
	for _, role := range []string{"anon", "authenticated"} {
		for _, table := range []string{"sessions", "courses", "questions", "activations", "attempts", "grading_jobs", "operations", "runner_leases"} {
			t.Run(role+"/"+table, func(t *testing.T) {
				var rls, canSelect bool
				if err := h.pool.QueryRow(h.ctx, `select c.relrowsecurity,has_table_privilege($1,c.oid,'SELECT') from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relname=$2`, role, "practicum_"+table).Scan(&rls, &canSelect); err != nil || !rls || canSelect {
					t.Fatalf("missing RLS/grant restriction: rls=%v select=%v %v", rls, canSelect, err)
				}
				tx, err := h.pool.Begin(h.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(h.ctx, "set local role "+role); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(h.ctx, "select * from public.practicum_"+table)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
					t.Fatalf("browser role directly read table: %v", err)
				}
			})
		}
	}
	var auditText string
	if err := h.pool.QueryRow(h.ctx, `select coalesce(jsonb_agg(to_jsonb(a))::text,'[]') from public.audit_logs a where scope->>'session_id'=$1`, h.session.ID).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{h.activation.AccessToken, "private-rules", "private-rubric", "\"frozen\""} {
		if strings.Contains(auditText, secret) {
			t.Fatal("transactional audit leaked private content")
		}
	}
}
