package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func TestLearningCompletionOnlyRequiresBrowserPassForCodeLessons(t *testing.T) {
	for _, tc := range []struct {
		name, starter, draft string
		tests, quiz, wrong   bool
		browserPassed        bool
		wantErr              error
	}{
		{name: "text without draft"},
		{name: "text with whitespace draft", draft: " \n\t"},
		{name: "quiz without draft", quiz: true},
		{name: "incorrect quiz without draft", quiz: true, wrong: true, wantErr: usecase.ErrLearningQuizIncorrect},
		{name: "starter requires draft", starter: "int main(void) {}", wantErr: usecase.ErrValidation},
		{name: "tests require draft", tests: true, wantErr: usecase.ErrValidation},
		{name: "code quiz rejects blank draft", starter: "int main(void) {}", quiz: true, draft: " \n", wantErr: usecase.ErrValidation},
		{name: "code without browser pass", starter: "int main(void) {}", tests: true, draft: "this is not C source !!!", wantErr: usecase.ErrLearningCodeNotVerified},
		{name: "browser claim is intentionally trusted", starter: "int main(void) {}", tests: true, draft: "this is not C source !!!", browserPassed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			identity := repository.NewMemoryIdentityRepository()
			repo := repository.NewMemoryLearningRepository()
			if err := repository.SeedLearningFixtures(ctx, repo); err != nil {
				t.Fatal(err)
			}
			if err := repo.Transaction(ctx, func(tx repository.LearningTx) error {
				row, err := tx.Get(repository.LearningLessons, "lesson-1")
				if err != nil {
					return err
				}
				var lesson entity.LearningLesson
				if err := json.Unmarshal(row.Data, &lesson); err != nil {
					return err
				}
				lesson.StarterCode = tc.starter
				if !tc.tests {
					lesson.TestCases = nil
				}
				if !tc.quiz {
					lesson.Quiz = nil
				}
				row.Data, err = json.Marshal(lesson)
				if err != nil {
					return err
				}
				return tx.Put(repository.LearningLessons, row)
			}); err != nil {
				t.Fatal(err)
			}
			svc := usecase.NewLearningService(usecase.LearningOptions{Repository: repo, IdentityRepository: identity})
			student := learningPrincipal(t, identity, 1)
			current, err := svc.Lesson(ctx, student, "lesson-1")
			if err != nil || current.Lesson.Version < 1 {
				t.Fatalf("public lesson version: %+v, %v", current, err)
			}
			input := usecase.LearningProgressInput{Language: "c", DraftCode: tc.draft, Complete: true}
			if tc.browserPassed {
				input.BrowserTestPassed = true
				input.LessonVersion = current.Lesson.Version
			}
			if tc.quiz {
				option := 0
				if tc.wrong {
					option = 1
				}
				input.QuizOption = &option
			}
			cmd := usecase.LearningCommand{Key: "complete", RequestID: "req-browser-completion", OperationID: "learning.lessons.progress"}
			view, err := svc.SaveProgress(ctx, student, cmd, "lesson-1", input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				unchanged, readErr := svc.Lesson(ctx, student, "lesson-1")
				if readErr != nil || unchanged.Profile.XP != 0 || unchanged.Profile.CompletedLessons != 0 || unchanged.Progress.Revision != 0 {
					t.Fatalf("rejected completion changed progress/XP: %+v, %v", unchanged, readErr)
				}
				return
			}
			if err != nil || view.Progress.Status != "completed" || view.Progress.DraftCode != tc.draft || view.Profile.XP != 60 || view.Profile.CompletedLessons != 1 || view.Progress.TestPassed != tc.browserPassed {
				t.Fatalf("completion failed: %+v, %v", view, err)
			}
			replay, err := svc.SaveProgress(ctx, student, cmd, "lesson-1", input)
			if err != nil || replay.Profile.XP != 60 || replay.Progress.Revision != view.Progress.Revision {
				t.Fatalf("replay changed progress/XP: %+v, %v", replay, err)
			}
		})
	}
}

func TestLearningBrowserClaimVersionRetryAndExactlyOnceXP(t *testing.T) {
	h := newLearningHarness(t, runner.Unconfigured{}, []entity.LearningTestCase{{Description: "case", Input: "", ExpectedOutput: "ok"}}, 0)
	current, err := h.svc.Lesson(t.Context(), h.student, "lesson-1")
	if err != nil {
		t.Fatal(err)
	}
	answer := 0
	base := usecase.LearningProgressInput{Language: "c", DraftCode: "forged-but-accepted", QuizOption: &answer, Complete: true}
	if _, err := h.svc.SaveProgress(t.Context(), h.student, usecase.LearningCommand{Key: "missing"}, "lesson-1", base); !errors.Is(err, usecase.ErrLearningCodeNotVerified) {
		t.Fatalf("missing browser claim: %v", err)
	}
	stale := base
	stale.BrowserTestPassed = true
	stale.LessonVersion = current.Lesson.Version - 1
	if _, err := h.svc.SaveProgress(t.Context(), h.student, usecase.LearningCommand{Key: "stale"}, "lesson-1", stale); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("stale lesson version: %v", err)
	}
	accepted := base
	accepted.BrowserTestPassed = true
	accepted.LessonVersion = current.Lesson.Version
	command := usecase.LearningCommand{Key: "accepted"}
	completed, err := h.svc.SaveProgress(t.Context(), h.student, command, "lesson-1", accepted)
	if err != nil || completed.Profile.XP != 60 || completed.Profile.CompletedLessons != 1 || !completed.Progress.TestPassed {
		t.Fatalf("accepted browser claim: %+v, %v", completed, err)
	}
	replay, err := h.svc.SaveProgress(t.Context(), h.student, command, "lesson-1", accepted)
	if err != nil || replay.Profile.XP != 60 || replay.Progress.Revision != completed.Progress.Revision {
		t.Fatalf("ambiguous retry: %+v, %v", replay, err)
	}
	repeat := base
	repeat.Revision = completed.Progress.Revision
	again, err := h.svc.SaveProgress(t.Context(), h.student, usecase.LearningCommand{Key: "again"}, "lesson-1", repeat)
	if err != nil || again.Profile.XP != 60 || again.Profile.CompletedLessons != 1 {
		t.Fatalf("duplicate XP: %+v, %v", again, err)
	}
	edited, err := h.svc.SaveProgress(t.Context(), h.student, usecase.LearningCommand{Key: "edit"}, "lesson-1", usecase.LearningProgressInput{
		Language: "c", DraftCode: "changed", QuizOption: &answer, Revision: again.Progress.Revision,
	})
	if err != nil || edited.Progress.TestPassed || edited.Profile.XP != 60 || edited.Progress.Status != "completed" {
		t.Fatalf("completed draft edit: %+v, %v", edited, err)
	}
}

// This fake never compiles code or contacts a provider. It only exercises the
// dormant provider-neutral compatibility endpoint.
type learningProofRunner struct {
	mu        sync.Mutex
	calls     []entity.RunRequest
	run       func(context.Context, entity.RunRequest) (entity.RunResult, error)
	simulated bool
}

func (r *learningProofRunner) Available() bool { return true }
func (r *learningProofRunner) Simulated() bool { return r.simulated }
func (r *learningProofRunner) Languages() []entity.RunnerLanguage {
	return []entity.RunnerLanguage{{Language: "c", Version: "fake-v1"}}
}
func (r *learningProofRunner) Run(ctx context.Context, req entity.RunRequest) (entity.RunResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req)
	r.mu.Unlock()
	if r.run != nil {
		return r.run(ctx, req)
	}
	zero := 0
	return entity.RunResult{Status: "success", Stdout: req.Stdin + "\r\n", ExitCode: &zero, RunnerVersion: "fake-v1"}, nil
}
func (r *learningProofRunner) requests() []entity.RunRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]entity.RunRequest(nil), r.calls...)
}

type learningProofHarness struct {
	t        *testing.T
	repo     *repository.MemoryLearningRepository
	identity *repository.MemoryIdentityRepository
	svc      *usecase.LearningService
	student  *entity.Principal
}

func learningPrincipal(t *testing.T, identity *repository.MemoryIdentityRepository, id int64) *entity.Principal {
	t.Helper()
	user, err := identity.FindUserByID(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := identity.ListAssignments(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := identity.ListPermissionGrants(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return &entity.Principal{User: user, Assignments: assignments, Grants: grants}
}

func newLearningHarness(t *testing.T, adapter usecase.PracticumRunner, cases []entity.LearningTestCase, timeout time.Duration) *learningProofHarness {
	t.Helper()
	h := &learningProofHarness{t: t, repo: repository.NewMemoryLearningRepository(), identity: repository.NewMemoryIdentityRepository()}
	if err := repository.SeedLearningFixtures(t.Context(), h.repo); err != nil {
		t.Fatal(err)
	}
	h.editLesson("lesson-1", func(lesson *entity.LearningLesson) { lesson.TestCases = cases })
	h.svc = usecase.NewLearningService(usecase.LearningOptions{Repository: h.repo, IdentityRepository: h.identity, Runner: adapter, RunnerTimeout: timeout})
	h.student = learningPrincipal(t, h.identity, 1)
	return h
}
func (h *learningProofHarness) editLesson(id string, edit func(*entity.LearningLesson)) {
	h.t.Helper()
	if err := h.repo.Transaction(h.t.Context(), func(tx repository.LearningTx) error {
		row, err := tx.Get(repository.LearningLessons, id)
		if err != nil {
			return err
		}
		var lesson entity.LearningLesson
		if err := json.Unmarshal(row.Data, &lesson); err != nil {
			return err
		}
		edit(&lesson)
		lesson.Version++
		row.Data, err = json.Marshal(lesson)
		if err != nil {
			return err
		}
		return tx.Put(repository.LearningLessons, row)
	}); err != nil {
		h.t.Fatal(err)
	}
}
func (h *learningProofHarness) run(key, source string) (entity.LearningRunResponse, error) {
	return h.svc.PlaygroundRun(h.t.Context(), h.student, usecase.LearningCommand{Key: key}, usecase.LearningPlaygroundInput{
		LessonID: "lesson-1", Language: "c", Version: "fake-v1", Source: source, Stdin: "CLIENT_STDIN_MUST_NOT_REPLACE_CASES",
	})
}

func TestLearningCompatibilityRunIsIdempotentAndUsesAuthoredInputs(t *testing.T) {
	adapter := &learningProofRunner{}
	h := newLearningHarness(t, adapter, []entity.LearningTestCase{
		{Description: "first", Input: "one", ExpectedOutput: "one"},
		{Description: "second", Input: "two", ExpectedOutput: "two"},
	}, 0)
	run, err := h.run("run", "source")
	if err != nil || !run.AllPassed || len(run.TestResults) != 2 {
		t.Fatalf("run: %+v, %v", run, err)
	}
	requests := adapter.requests()
	if len(requests) != 2 || requests[0].Stdin != "one" || requests[1].Stdin != "two" {
		t.Fatalf("case stdin not isolated: %+v", requests)
	}
	replay, err := h.run("run", "source")
	if err != nil || !reflect.DeepEqual(run, replay) || len(adapter.requests()) != 2 {
		t.Fatalf("run replay changed results: %+v, %v", replay, err)
	}
}

func TestLearningCompatibilityRunMatchesExactOutputs(t *testing.T) {
	for _, tc := range []struct {
		name, output, expected, status            string
		version                                   *string
		exit                                      *int
		truncated, simulated, providerError, want bool
	}{
		{name: "exact", output: "answer\r\n", expected: "answer", status: "success", exit: new(0), want: true},
		{name: "empty expected accepts empty", expected: "", status: "success", exit: new(0), want: true},
		{name: "empty expected rejects text", output: "text", expected: "", status: "success", exit: new(0)},
		{name: "substring is not equality", output: "before answer after", expected: "answer", status: "success", exit: new(0)},
		{name: "spaces matter", output: "answer ", expected: "answer", status: "success", exit: new(0)},
		{name: "compile error", status: "compile_error"},
		{name: "runtime error", status: "runtime_error", exit: new(1)},
		{name: "timeout", status: "timeout"},
		{name: "provider error", status: "provider_error"},
		{name: "provider returns error with success", status: "success", exit: new(0), providerError: true},
		{name: "truncated success", status: "success", exit: new(0), truncated: true},
		{name: "missing exit status", status: "success"},
		{name: "nonzero success", status: "success", exit: new(1)},
		{name: "simulated not opted in", status: "success", exit: new(0), simulated: true},
		{name: "missing runtime", status: "success", exit: new(0), version: new("")},
		{name: "wrong runtime", status: "success", exit: new(0), version: new("wrong-version")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &learningProofRunner{run: func(context.Context, entity.RunRequest) (entity.RunResult, error) {
				var err error
				if tc.providerError {
					err = errors.New("synthetic failure")
				}
				version := "fake-v1"
				if tc.version != nil {
					version = *tc.version
				}
				return entity.RunResult{Status: tc.status, Stdout: tc.output, ExitCode: tc.exit, Truncated: tc.truncated, Simulated: tc.simulated, RunnerVersion: version}, err
			}}
			h := newLearningHarness(t, adapter, []entity.LearningTestCase{{Description: "case", ExpectedOutput: tc.expected}}, 0)
			run, err := h.run("run", "source")
			if err != nil || run.AllPassed != tc.want || run.TestResults[0].Passed != tc.want {
				t.Fatalf("run %+v, %v", run, err)
			}
		})
	}
}

func TestLearningCompatibilityRunSharesDeadline(t *testing.T) {
	var deadlines []time.Time
	adapter := &learningProofRunner{run: func(ctx context.Context, req entity.RunRequest) (entity.RunResult, error) {
		deadline, _ := ctx.Deadline()
		deadlines = append(deadlines, deadline)
		if req.Stdin == "second" {
			<-ctx.Done()
		}
		zero := 0
		return entity.RunResult{Status: "success", ExitCode: &zero, Stdout: req.Stdin, RunnerVersion: "fake-v1"}, nil
	}}
	h := newLearningHarness(t, adapter, []entity.LearningTestCase{
		{Description: "one", Input: "first", ExpectedOutput: "first"},
		{Description: "two", Input: "second", ExpectedOutput: "second"},
		{Description: "three", Input: "third", ExpectedOutput: "third"},
	}, 30*time.Millisecond)
	run, err := h.run("deadline", "source")
	if err != nil || run.AllPassed || len(deadlines) != 2 || !deadlines[0].Equal(deadlines[1]) || len(adapter.requests()) != 2 {
		t.Fatalf("deadline batch: %+v, %v, deadlines=%v", run, err, deadlines)
	}
}

func TestLearningUnconfiguredCompatibilityRunDoesNotBlockBrowserClaim(t *testing.T) {
	h := newLearningHarness(t, runner.Unconfigured{}, nil, 0)
	if _, err := h.run("unconfigured", "source"); !errors.Is(err, usecase.ErrLearningUnavailable) {
		t.Fatal(err)
	}
	current, err := h.svc.Lesson(t.Context(), h.student, "lesson-1")
	if err != nil {
		t.Fatal(err)
	}
	answer := 0
	completed, err := h.svc.SaveProgress(t.Context(), h.student, usecase.LearningCommand{Key: "browser"}, "lesson-1", usecase.LearningProgressInput{
		Language: "c", DraftCode: "source", QuizOption: &answer, Complete: true,
		BrowserTestPassed: true, LessonVersion: current.Lesson.Version,
	})
	if err != nil || completed.Profile.XP != 60 {
		t.Fatalf("browser completion depends on server runner: %+v, %v", completed, err)
	}
}
