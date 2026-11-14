package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type practicumTestGrader func(context.Context, entity.GradeRequest) (entity.GradeResult, error)

func (f practicumTestGrader) Grade(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
	return f(ctx, r)
}
func practicumSuggestions(r entity.GradeRequest) entity.GradeResult {
	out := entity.GradeResult{Status: "suggested", Simulated: true, ProviderVersion: "synthetic-grader-v1"}
	for _, q := range r.Items {
		out.Suggestions = append(out.Suggestions, entity.SuggestedGrade{QuestionID: q.QuestionID, Score: q.MaxScore, Feedback: "Synthetic suggestion"})
	}
	return out
}
func (h *practicumHarness) submit(t *testing.T) entity.PracticumAttempt {
	t.Helper()
	a := h.start(t, 0)
	_, err := h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("save"), a.AttemptID, PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "submitted-answer"}}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func (h *practicumHarness) manualInput(publish bool) PracticumManualInput {
	return PracticumManualInput{Grades: []entity.PracticumManualGrade{{QuestionID: h.questions[0].ID, Score: 7, Feedback: "Reviewed essay"}, {QuestionID: h.questions[1].ID, Score: 0, Feedback: "Unanswered"}}, Publish: publish, Reason: "Instructor reviewed rubric"}
}

func TestPracticumGradingSuggestionManualPublicationAndReplay(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.submit(t)
	var calls atomic.Int32
	h.s.options.Grader = practicumTestGrader(func(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
		calls.Add(1)
		if _, ok := ctx.Deadline(); !ok {
			t.Error("provider call has no deadline")
		}
		if r.AttemptID != a.AttemptID || len(r.Items) != 2 || r.Items[0].Answer != "submitted-answer" || r.Items[0].ReferenceAnswer != "private-reference" || r.Items[0].Rubric[0].MaxScore != 10 || r.Items[1].Answer != "" {
			t.Error("provider request did not use frozen submission/rules")
		}
		return practicumSuggestions(r), nil
	})
	review, err := h.s.Grade(t.Context(), h.admin, practicumCmd("grade"), a.AttemptID)
	if err != nil || review.Attempt.GradingStatus != "needs_manual" || review.Attempt.Status != "submitted" || review.Attempt.Score != nil || len(review.Suggestions) != 2 || !review.GradingJob.Simulated || review.GradingJob.LeaseID != "" {
		t.Fatalf("suggestion published/leaked lease: %+v %v", review, err)
	}
	_, err = h.s.Grade(t.Context(), h.admin, practicumCmd("grade"), a.AttemptID)
	if err != nil || calls.Load() != 1 {
		t.Fatal("same grading key invoked provider twice")
	}
	manual := h.manualInput(false)
	review, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("draft"), a.AttemptID, manual)
	if err != nil || review.Attempt.GradeRevision != 1 || review.Attempt.Answers[0].Score == nil || review.Attempt.Score != nil {
		t.Fatalf("manual draft: %+v %v", review, err)
	}
	student, err := h.s.Attempt(t.Context(), h.students[0], a.AttemptID, practicumCmd("read-draft"))
	if err != nil || student.Score != nil || student.Answers[0].Score != nil || student.Answers[0].Feedback != "" {
		t.Fatal("unpublished manual draft leaked")
	}
	_, err = h.s.Grade(t.Context(), h.admin, practicumCmd("grade-after-draft"), a.AttemptID)
	if err != nil || calls.Load() != 1 {
		t.Fatal("provider superseded draft ownership")
	}
	_, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("stale-grade"), a.AttemptID, manual)
	expectPracticumError(t, err, repository.ErrConflict)
	manual.GradeRevision = 1
	manual.Publish = true
	review, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("publish"), a.AttemptID, manual)
	if err != nil || review.Attempt.Status != "graded" || review.Attempt.GradeRevision != 2 || review.Attempt.Score == nil || *review.Attempt.Score != 7 || review.Attempt.Percentage == nil || *review.Attempt.Percentage != 35 {
		t.Fatalf("raw points vs percentage incorrect: %+v %v", review, err)
	}
	grades, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 25})
	if err != nil || len(grades) != 1 || grades[0].Score == nil || *grades[0].Score != 7 || grades[0].MaxScore != 20 {
		t.Fatalf("published student summary: %+v %v", grades, err)
	}
	other, err := h.s.Grades(t.Context(), h.students[1], PracticumPage{Limit: 25})
	if err != nil || len(other) != 0 {
		t.Fatal("cross-user grade leak")
	}
	replay, err := h.s.ManualGrade(t.Context(), h.admin, practicumCmd("draft"), a.AttemptID, h.manualInput(false))
	if err != nil || replay.Attempt.Status != "graded" || replay.Attempt.GradeRevision != 2 {
		t.Fatal("manual replay returned stale draft")
	}
	manual.GradeRevision = 2
	manual.Publish = false
	_, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("unpublish"), a.AttemptID, manual)
	expectPracticumError(t, err, repository.ErrConflict)
	manual.Publish = true
	manual.Grades[0].Score = 8
	manual.Reason = "Corrected rubric application"
	review, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("correction"), a.AttemptID, manual)
	if err != nil || *review.Attempt.Score != 8 || review.Attempt.GradeRevision != 3 {
		t.Fatal("audited correction failed")
	}
	record := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID)
	if record.GradeReason != manual.Reason || len(record.SubmittedAnswers) != 1 || record.SubmittedAnswers[0].Score != nil {
		t.Fatal("grade changed submission snapshot/reason")
	}
}

func TestPracticumReconcileBatchRecoversExpiredLeaseBeforePendingJobs(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	abandoned := h.submit(t)
	if err := h.repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		a, _, err := practicumLockAttempt(tx, abandoned.AttemptID)
		if err != nil {
			return err
		}
		_, err = h.s.claimGrade(tx, &a, h.clock.Now(), nil, practicumCmd("abandoned-claim"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	h.clock.Set(h.clock.Now().Add(h.s.options.GradingTimeout + 16*time.Second))
	pending := h.start(t, 1)
	if _, err := h.s.Submit(t.Context(), h.students[1], practicumCmd("pending-submit"), pending.AttemptID); err != nil {
		t.Fatal(err)
	}
	var processed []string
	h.s.options.Grader = practicumTestGrader(func(_ context.Context, req entity.GradeRequest) (entity.GradeResult, error) {
		processed = append(processed, req.AttemptID)
		return entity.GradeResult{Status: "needs_manual"}, nil
	})
	want := []string{abandoned.AttemptID, pending.AttemptID}
	result, err := h.s.Reconcile(t.Context(), practicumCmd("recover-batch"))
	if err != nil || result.Expired != 0 || result.JobsProcessed != len(want) || len(processed) != len(want) || processed[0] != want[0] || processed[1] != want[1] {
		t.Fatalf("batch reconciliation: result=%+v processed=%v err=%v", result, processed, err)
	}
	for _, id := range want {
		job := practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, id)
		if job.Status != "needs_manual" || job.LeaseID != "" || job.LeaseUntil != nil {
			t.Fatalf("job not finalized: %+v", job)
		}
	}
	result, err = h.s.Reconcile(t.Context(), practicumCmd("empty-replay"))
	if err != nil || result.JobsProcessed != 0 || len(processed) != 2 {
		t.Fatalf("completed jobs executed again: %+v %v", result, err)
	}
}

func TestPracticumReconcileStopsBeforeCanceledContext(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	first := h.submit(t)
	second := h.start(t, 1)
	if _, err := h.s.Submit(t.Context(), h.students[1], practicumCmd("second-submit"), second.AttemptID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var calls atomic.Int32
	h.s.options.Grader = practicumTestGrader(func(_ context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
		if calls.Add(1) == 1 {
			cancel()
		}
		return entity.GradeResult{Status: "needs_manual"}, nil
	})
	result, err := h.s.Reconcile(ctx, practicumCmd("cancel-after-first"))
	if err != nil || result.JobsProcessed != 1 || calls.Load() != 1 {
		t.Fatalf("canceled batch: result=%+v calls=%d err=%v", result, calls.Load(), err)
	}
	pending := 0
	for _, id := range []string{first.AttemptID, second.AttemptID} {
		switch practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, id).Status {
		case "needs_manual":
		case "pending":
			pending++
		default:
			t.Fatalf("unexpected job state for %s", id)
		}
	}
	if pending != 1 {
		t.Fatal("canceled batch did not leave exactly one job pending")
	}
	result, err = h.s.Reconcile(t.Context(), practicumCmd("resume-after-cancel"))
	if err != nil || result.JobsProcessed != 1 || calls.Load() != 2 {
		t.Fatalf("resume after cancel: result=%+v calls=%d err=%v", result, calls.Load(), err)
	}
	for _, id := range []string{first.AttemptID, second.AttemptID} {
		job := practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, id)
		if job.Status != "needs_manual" || job.LeaseID != "" {
			t.Fatalf("job not finalized after resume: %+v", job)
		}
	}
}

func TestPracticumReconcileConcurrentInvocationsProcessEachJobOnce(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	first := h.submit(t)
	second := h.start(t, 1)
	if _, err := h.s.Submit(t.Context(), h.students[1], practicumCmd("concurrent-submit"), second.AttemptID); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	processed := map[string]int{}
	h.s.options.Grader = practicumTestGrader(func(_ context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
		mu.Lock()
		processed[r.AttemptID]++
		mu.Unlock()
		return entity.GradeResult{Status: "needs_manual"}, nil
	})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := h.s.Reconcile(t.Context(), practicumCmd(fmt.Sprintf("concurrent-%d", i)))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	counts := map[string]int{}
	for id, count := range processed {
		counts[id] = count
	}
	mu.Unlock()
	if len(counts) != 2 || counts[first.AttemptID] != 1 || counts[second.AttemptID] != 1 {
		t.Fatalf("concurrent reconciliation executed jobs unevenly: %v", counts)
	}
	for _, id := range []string{first.AttemptID, second.AttemptID} {
		job := practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, id)
		if job.Status != "needs_manual" || job.LeaseID != "" {
			t.Fatalf("job not finalized: %+v", job)
		}
	}
}

func TestPracticumReconcileBatchLoadSynthetic120(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	for i := range 60 {
		activation, err := h.s.Activate(t.Context(), h.admin, practicumCmd(fmt.Sprintf("load-activation-%d", i)), h.session.ID, PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "individual", StudentIDs: []int64{1, 2}, QuestionIDs: []string{h.questions[0].ID, h.questions[1].ID}})
		if err != nil {
			t.Fatal(err)
		}
		for studentIndex, student := range h.students[:2] {
			attempt, err := h.s.Start(t.Context(), student, practicumCmd(fmt.Sprintf("load-start-%d-%d", i, studentIndex)), PracticumStartInput{ActivationID: activation.ID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := h.s.Submit(t.Context(), student, practicumCmd(fmt.Sprintf("load-submit-%d-%d", i, studentIndex)), attempt.AttemptID); err != nil {
				t.Fatal(err)
			}
		}
	}
	var (
		processed   int
		invocations int
		durations   []time.Duration
	)
	for invocations < practicumReconcileBatch {
		started := time.Now()
		result, err := h.s.Reconcile(t.Context(), practicumCmd(fmt.Sprintf("load-reconcile-%d", invocations)))
		if err != nil {
			t.Fatal(err)
		}
		if result.JobsProcessed == 0 {
			break
		}
		if result.JobsProcessed > practicumReconcileBatch {
			t.Fatalf("invocation processed %d jobs, cap is %d", result.JobsProcessed, practicumReconcileBatch)
		}
		durations = append(durations, time.Since(started))
		processed += result.JobsProcessed
		invocations++
	}
	if processed != 120 || invocations != 6 {
		t.Fatalf("synthetic load: processed=%d invocations=%d", processed, invocations)
	}
	jobs := practicumRows(t, h.repo, repository.PracticumJobs)
	if len(jobs) != 120 {
		t.Fatalf("synthetic load created %d jobs, want 120", len(jobs))
	}
	for _, row := range jobs {
		var job entity.PracticumGradingJob
		if err := json.Unmarshal(row.Data, &job); err != nil {
			t.Fatal(err)
		}
		if job.Status != "needs_manual" || job.LeaseID != "" || job.LeaseUntil != nil {
			t.Fatalf("synthetic load left an open job: %+v", job)
		}
	}
	var p95 time.Duration
	for i, duration := range durations {
		if i == 0 || duration > p95 {
			p95 = duration
		}
	}
	t.Logf("synthetic load: 120 jobs in %d invocations, p95 invocation %s", invocations, p95)
}

func TestPracticumManualGradeValidation(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "shared", false)
	a := h.submit(t)
	for name, mutate := range map[string]func(*PracticumManualInput){
		"reason": func(in *PracticumManualInput) { in.Reason = "  " }, "negative": func(in *PracticumManualInput) { in.Grades[0].Score = -1 }, "over-max": func(in *PracticumManualInput) { in.Grades[0].Score = 11 }, "missing-question": func(in *PracticumManualInput) { in.Grades = in.Grades[:1] }, "duplicate": func(in *PracticumManualInput) { in.Grades[1].QuestionID = in.Grades[0].QuestionID }, "unknown": func(in *PracticumManualInput) { in.Grades[0].QuestionID = "foreign" }, "feedback": func(in *PracticumManualInput) { in.Grades[0].Feedback = strings.Repeat("x", 4001) },
	} {
		t.Run(name, func(t *testing.T) {
			in := h.manualInput(true)
			mutate(&in)
			_, err := h.s.ManualGrade(t.Context(), h.admin, practicumCmd(name), a.AttemptID, in)
			expectPracticumError(t, err, ErrValidation)
		})
	}
	record := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID)
	if record.Attempt.GradeRevision != 0 || record.Attempt.Status != "submitted" {
		t.Fatal("rejected grade mutated revision")
	}
}

func TestPracticumManualGradeSupersedesInflightProvider(t *testing.T) {
	for _, publish := range []bool{false, true} {
		t.Run(fmt.Sprint(publish), func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "shared", false)
			a := h.submit(t)
			entered := make(chan struct{})
			release := make(chan struct{}, 1)
			defer close(release)
			h.s.options.Grader = practicumTestGrader(func(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
				close(entered)
				select {
				case <-release:
					return practicumSuggestions(r), nil
				case <-ctx.Done():
					return entity.GradeResult{}, ctx.Err()
				}
			})
			finished := make(chan error, 1)
			go func() {
				_, err := h.s.Grade(t.Context(), h.admin, practicumCmd("slow-grade"), a.AttemptID)
				finished <- err
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("provider not called")
			}
			manualDone := make(chan error, 1)
			go func() {
				_, err := h.s.ManualGrade(t.Context(), h.admin, practicumCmd("manual-wins"), a.AttemptID, h.manualInput(publish))
				manualDone <- err
			}()
			select {
			case err := <-manualDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("provider holds domain transaction")
			}
			// Release the provider only after the manual transaction commits.
			release <- struct{}{}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("provider finalization stalled")
			}
			review, err := h.s.Review(t.Context(), h.admin, a.AttemptID, practicumCmd("review"))
			if err != nil {
				t.Fatal(err)
			}
			if review.Attempt.GradeRevision != 1 || len(review.Suggestions) != 0 || review.GradingJob.GradeRevision != 1 || review.GradingJob.LeaseID != "" {
				t.Fatalf("stale provider overwrote manual CAS: %+v", review)
			}
			if publish && (review.Attempt.Status != "graded" || *review.Attempt.Score != 7) {
				t.Fatal("published grade overwritten")
			}
		})
	}
}

func TestPracticumGradingLeaseExpiryAndReplacementCAS(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "shared", false)
			a := h.submit(t)
			h.s.options.Grader = practicumTestGrader(func(_ context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
				return practicumSuggestions(r), nil
			})
			claim := func() practicumGradeClaim {
				t.Helper()
				var c practicumGradeClaim
				if err := h.repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
					record, _, err := practicumLockAttempt(tx, a.AttemptID)
					if err != nil {
						return err
					}
					now, err := tx.Now()
					if err != nil {
						return err
					}
					c, err = h.s.claimGrade(tx, &record, now, h.admin, practicumCmd("claim"))
					return err
				}); err != nil {
					t.Fatal(err)
				}
				return c
			}
			first := claim()
			h.clock.Set(h.clock.Now().Add(h.s.options.GradingTimeout + 15*time.Second))
			var second practicumGradeClaim
			if replace {
				second = claim()
				if second.LeaseID == "" || second.LeaseID == first.LeaseID {
					t.Fatal("expired lease not reclaimed")
				}
			}
			if err := h.s.finishGrade(t.Context(), first, h.admin, practicumCmd("old-result")); err != nil {
				t.Fatal(err)
			}
			job := practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, a.AttemptID)
			if job.Status != "running" || len(job.Suggestions) != 0 {
				t.Fatal("expired/replaced provider lease finalized")
			}
			if replace {
				if job.LeaseID != second.LeaseID {
					t.Fatal("old completion replaced new lease")
				}
				if err := h.s.finishGrade(t.Context(), second, h.admin, practicumCmd("new-result")); err != nil {
					t.Fatal(err)
				}
				job = practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, a.AttemptID)
				if job.Status != "needs_manual" || len(job.Suggestions) != 2 {
					t.Fatal("current lease failed to finish")
				}
			}
		})
	}
}

func TestPracticumGradingProviderFallbacksAndBulkAtomicity(t *testing.T) {
	for _, mode := range []string{"unconfigured", "outage", "timeout", "over-score", "duplicate", "oversize-feedback", "wrong-status"} {
		t.Run(mode, func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "shared", false)
			a := h.submit(t)
			if mode != "unconfigured" {
				h.s.options.GradingTimeout = 20 * time.Millisecond
				h.s.options.Grader = practicumTestGrader(func(ctx context.Context, r entity.GradeRequest) (entity.GradeResult, error) {
					result := practicumSuggestions(r)
					switch mode {
					case "outage":
						return entity.GradeResult{}, errors.New("synthetic outage")
					case "timeout":
						<-ctx.Done()
						return entity.GradeResult{}, ctx.Err()
					case "over-score":
						result.Suggestions[0].Score++
					case "duplicate":
						result.Suggestions[1].QuestionID = result.Suggestions[0].QuestionID
					case "oversize-feedback":
						result.Suggestions[0].Feedback = strings.Repeat("x", 4001)
					case "wrong-status":
						result.Status = "graded"
					}
					return result, nil
				})
			}
			review, err := h.s.Grade(t.Context(), h.admin, practicumCmd("fallback"), a.AttemptID)
			if err != nil || review.Attempt.GradingStatus != "needs_manual" || review.Attempt.Score != nil || len(review.Suggestions) != 0 {
				t.Fatalf("unsafe fallback: %+v %v", review, err)
			}
			_, err = h.s.BulkGrade(t.Context(), h.admin, practicumCmd("bulk-fail"), []string{a.AttemptID, "missing"})
			expectPracticumError(t, err, repository.ErrPracticumNotFound)
			job := practicumRecord[entity.PracticumGradingJob](t, h.repo, repository.PracticumJobs, a.AttemptID)
			if job.Status != "needs_manual" {
				t.Fatal("bulk committed a partial enqueue")
			}
			jobs, err := h.s.BulkGrade(t.Context(), h.admin, practicumCmd("bulk"), []string{a.AttemptID})
			if err != nil || len(jobs) != 1 || jobs[0].Status != "pending" {
				t.Fatalf("bulk retry failed: %+v %v", jobs, err)
			}
		})
	}
}

type practicumTestRunner struct {
	run   func(context.Context, entity.RunRequest) (entity.RunResult, error)
	calls atomic.Int32
}

func (r *practicumTestRunner) Run(ctx context.Context, in entity.RunRequest) (entity.RunResult, error) {
	r.calls.Add(1)
	if r.run != nil {
		return r.run(ctx, in)
	}
	return entity.RunResult{Status: "success", Stdout: "synthetic-only", RunnerVersion: "fake-v1"}, nil
}
func (*practicumTestRunner) Languages() []entity.RunnerLanguage {
	return []entity.RunnerLanguage{{Language: "c", Version: "test-v1", Label: "Synthetic C"}}
}
func (*practicumTestRunner) Available() bool { return true }
func (*practicumTestRunner) Simulated() bool { return true }
func (h *practicumHarness) runInput() PracticumRunInput {
	return PracticumRunInput{QuestionID: h.questions[1].ID, Source: "/* synthetic source: never executed */", Language: "c", Version: "test-v1", Stdin: "synthetic input"}
}

type practicumLiveRunner struct{ result entity.RunResult }

func (r practicumLiveRunner) Run(context.Context, entity.RunRequest) (entity.RunResult, error) {
	return r.result, nil
}
func (practicumLiveRunner) Languages() []entity.RunnerLanguage {
	return []entity.RunnerLanguage{{Language: "c", Version: "test-v1", Label: "Synthetic live C"}}
}
func (practicumLiveRunner) Available() bool { return true }
func (practicumLiveRunner) Simulated() bool { return false }

func TestPracticumLiveRunnerRejectsRuntimeVersionMismatch(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "shared", false)
	a := h.start(t, 0)
	exit := 0
	h.s.options.Runner = practicumLiveRunner{result: entity.RunResult{Status: "success", Stdout: "synthetic", ExitCode: &exit, RunnerVersion: "other-v1"}}
	result, err := h.s.Run(t.Context(), h.students[0], practicumCmd("mismatch"), a.AttemptID, h.runInput())
	if err != nil || result.Status != "provider_error" {
		t.Fatalf("runtime version mismatch accepted: %+v %v", result, err)
	}
	h.s.options.Runner = practicumLiveRunner{result: entity.RunResult{Status: "success", Stdout: "synthetic", ExitCode: &exit, RunnerVersion: "test-v1"}}
	result, err = h.s.Run(t.Context(), h.students[0], practicumCmd("matching"), a.AttemptID, h.runInput())
	if err != nil || result.Status != "success" {
		t.Fatalf("matching runtime rejected: %+v %v", result, err)
	}
}

func TestPracticumRunnerDedupValidationAndPrivacy(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "shared", false)
	a := h.start(t, 0)
	runner := &practicumTestRunner{run: func(_ context.Context, in entity.RunRequest) (entity.RunResult, error) {
		if in.RequestID != "req-test" {
			t.Errorf("provider lost HTTP request correlation: %s", in.RequestID)
		}
		return entity.RunResult{Status: "success", Stdout: "synthetic-only"}, nil
	}}
	h.s.options.Runner = runner
	input := h.runInput()
	result, err := h.s.Run(t.Context(), h.students[0], practicumCmd("run"), a.AttemptID, input)
	if err != nil || result.Status != "success" || !result.Simulated || result.SourceHash == "" || result.RequestID != "req-test" {
		t.Fatalf("runner mapping: %+v %v", result, err)
	}
	_, err = h.s.Run(t.Context(), h.students[1], practicumCmd("run"), a.AttemptID, input)
	expectPracticumHidden(t, err)
	for name, mutate := range map[string]func(*PracticumRunInput){"source-size": func(in *PracticumRunInput) { in.Source = strings.Repeat("x", (64<<10)+1) }, "stdin-size": func(in *PracticumRunInput) { in.Stdin = strings.Repeat("x", (16<<10)+1) }, "language": func(in *PracticumRunInput) { in.Language = "shell" }, "version": func(in *PracticumRunInput) { in.Version = "unapproved" }, "essay": func(in *PracticumRunInput) { in.QuestionID = h.questions[0].ID }, "unknown": func(in *PracticumRunInput) { in.QuestionID = "foreign" }} {
		t.Run(name, func(t *testing.T) {
			in := input
			mutate(&in)
			_, err := h.s.Run(t.Context(), h.students[0], practicumCmd(name), a.AttemptID, in)
			expectPracticumError(t, err, ErrValidation)
		})
	}
	changed := input
	changed.Source = "different"
	_, err = h.s.Run(t.Context(), h.students[0], practicumCmd("run"), a.AttemptID, changed)
	expectPracticumError(t, err, repository.ErrConflict)
	_, err = h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	retryCommand := practicumCmd("run")
	retryCommand.RequestID = "req-retry"
	replay, err := h.s.Run(t.Context(), h.students[0], retryCommand, a.AttemptID, input)
	if err != nil || replay.RequestID != result.RequestID || replay.Stdout != result.Stdout || runner.calls.Load() != 1 {
		t.Fatal("runner duplicate executed or changed result")
	}
	_, err = h.s.Run(t.Context(), h.students[0], practicumCmd("after-submit"), a.AttemptID, input)
	expectPracticumError(t, err, ErrPracticumClosed)
	for _, kind := range []repository.PracticumKind{repository.PracticumRuns, repository.PracticumOperations} {
		for _, row := range practicumRows(t, h.repo, kind) {
			if strings.Contains(string(row.Data), input.Source) || strings.Contains(string(row.Data), input.Stdin) {
				t.Fatal("run persistence retained source/stdin")
			}
		}
	}
	audits, _ := h.audit.ListAudit(t.Context(), 1000)
	wire, _ := json.Marshal(audits)
	if strings.Contains(string(wire), input.Source) || strings.Contains(string(wire), input.Stdin) {
		t.Fatal("audit retained source/stdin")
	}
}

func TestPracticumRunnerInflightDedupLimitsAndLeaseExpiry(t *testing.T) {
	for _, expire := range []bool{false, true} {
		t.Run(fmt.Sprint(expire), func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "shared", false)
			a := h.start(t, 0)
			b := h.start(t, 1)
			entered := make(chan struct{})
			release := make(chan struct{}, 1)
			defer close(release)
			runner := &practicumTestRunner{run: func(ctx context.Context, r entity.RunRequest) (entity.RunResult, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("runner has no timeout")
				}
				close(entered)
				select {
				case <-release:
					return entity.RunResult{Status: "success"}, nil
				case <-ctx.Done():
					return entity.RunResult{}, ctx.Err()
				}
			}}
			h.s.options.Runner = runner
			h.s.options.RunnerUserLimit = 1
			h.s.options.RunnerGlobalLimit = 1
			type reply struct {
				result entity.RunResult
				err    error
			}
			done := make(chan reply, 1)
			go func() {
				result, err := h.s.Run(t.Context(), h.students[0], practicumCmd("inflight"), a.AttemptID, h.runInput())
				done <- reply{result, err}
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("runner did not start")
			}
			_, err := h.s.Run(t.Context(), h.students[0], practicumCmd("inflight"), a.AttemptID, h.runInput())
			expectPracticumError(t, err, repository.ErrConflict)
			_, err = h.s.Run(t.Context(), h.students[0], practicumCmd("user-limit"), a.AttemptID, h.runInput())
			expectPracticumError(t, err, ErrPracticumBusy)
			_, err = h.s.Run(t.Context(), h.students[1], practicumCmd("global-limit"), b.AttemptID, h.runInput())
			expectPracticumError(t, err, ErrPracticumBusy)
			if expire {
				h.clock.Set(h.clock.Now().Add(h.s.options.RunnerTimeout + 15*time.Second))
				result, err := h.s.Run(t.Context(), h.students[0], practicumCmd("inflight"), a.AttemptID, h.runInput())
				if err != nil || result.Status != "provider_error" {
					t.Fatal("expired replay did not resolve abandoned lease")
				}
			}
			_, err = h.s.Close(t.Context(), h.admin, practicumCmd("close-during-run"), h.session.ID, h.course.ID, h.activation.ID)
			if err != nil {
				t.Fatal(err)
			}
			release <- struct{}{}
			select {
			case got := <-done:
				want := "success"
				if expire {
					want = "provider_error"
				}
				if got.err != nil || got.result.Status != want {
					t.Fatalf("late runner CAS: %+v", got)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("runner finalization blocked")
			}
			record := practicumRecord[entity.PracticumAttemptRecord](t, h.repo, repository.PracticumAttempts, a.AttemptID)
			if record.Attempt.Status != "submitted" || record.Attempt.Revision != 0 || runner.calls.Load() != 1 {
				t.Fatal("runner mutated answers/state or executed duplicate")
			}
		})
	}
}

func TestPracticumRunnerQuotaAndProviderResultBounds(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.start(t, 0)
	runner := &practicumTestRunner{}
	h.s.options.Runner = runner
	for i := range 30 {
		if _, err := h.s.Run(t.Context(), h.students[0], practicumCmd(fmt.Sprintf("quota-%d", i)), a.AttemptID, h.runInput()); err != nil {
			t.Fatal(err)
		}
	}
	_, err := h.s.Run(t.Context(), h.students[0], practicumCmd("over-quota"), a.AttemptID, h.runInput())
	expectPracticumError(t, err, ErrPracticumBusy)
	if _, err := h.s.Run(t.Context(), h.students[0], practicumCmd("quota-0"), a.AttemptID, h.runInput()); err != nil || runner.calls.Load() != 30 {
		t.Fatal("replay consumed quota")
	}
	h.clock.Set(h.clock.Now().Add(time.Minute))
	if _, err := h.s.Run(t.Context(), h.students[0], practicumCmd("over-quota"), a.AttemptID, h.runInput()); err != nil {
		t.Fatal("quota window did not reset")
	}
	for _, mode := range []string{"outage", "timeout", "stdout", "stderr", "duration", "status", "spoof"} {
		t.Run(mode, func(t *testing.T) {
			h.s.options.RunnerTimeout = 20 * time.Millisecond
			runner.run = func(ctx context.Context, r entity.RunRequest) (entity.RunResult, error) {
				out := entity.RunResult{Status: "success"}
				switch mode {
				case "outage":
					return out, errors.New("synthetic outage")
				case "timeout":
					<-ctx.Done()
					return out, ctx.Err()
				case "stdout":
					out.Stdout = strings.Repeat("x", (64<<10)+1)
				case "stderr":
					out.Stderr = strings.Repeat("x", (64<<10)+1)
				case "duration":
					out.DurationMS = 120001
				case "status":
					out.Status = "arbitrary"
				case "spoof":
					out.RequestID = "foreign"
					out.SourceHash = "fake"
				}
				return out, nil
			}
			result, err := h.s.Run(t.Context(), h.students[0], practicumCmd(mode), a.AttemptID, h.runInput())
			want := "provider_error"
			if mode == "spoof" {
				want = "success"
			}
			if err != nil || result.Status != want || !result.Simulated || result.RequestID == "foreign" || result.SourceHash == "fake" {
				t.Fatalf("invalid provider output escaped: %+v %v", result, err)
			}
		})
	}
}
