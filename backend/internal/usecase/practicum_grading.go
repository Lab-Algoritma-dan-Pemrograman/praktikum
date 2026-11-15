package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type practicumGradeClaim struct {
	Review   entity.PracticumAttemptReview
	Request  entity.GradeRequest
	LeaseID  string
	Revision int64
}

func practicumReview(tx repository.PracticumTx, a entity.PracticumAttemptRecord, now time.Time) (entity.PracticumAttemptReview, error) {
	activation, err := tx.Get(repository.PracticumActivations, a.Attempt.ActivationID)
	if err != nil {
		return entity.PracticumAttemptReview{}, err
	}
	out := entity.PracticumAttemptReview{Attempt: practicumAttemptView(a, now), Questions: append([]entity.PracticumQuestion{}, a.Attempt.Questions...), Suggestions: []entity.SuggestedGrade{}}
	out.Attempt.SessionID = activation.ParentID
	for _, grade := range a.ManualGrades {
		found := false
		for i := range out.Attempt.Answers {
			if out.Attempt.Answers[i].QuestionID == grade.QuestionID {
				v := grade.Score
				out.Attempt.Answers[i].Score = &v
				out.Attempt.Answers[i].Feedback = grade.Feedback
				found = true
			}
		}
		if !found {
			v := grade.Score
			out.Attempt.Answers = append(out.Attempt.Answers, entity.PracticumAnswer{QuestionID: grade.QuestionID, Score: &v, Feedback: grade.Feedback})
		}
	}
	job, err := practicumJob(tx, a.Attempt.AttemptID)
	if err != nil {
		return out, err
	}
	if job != nil {
		copy := *job
		copy.LeaseID = ""
		out.GradingJob = &copy
		out.Suggestions = append(out.Suggestions, job.Suggestions...)
	}
	return out, nil
}
func (s *PracticumService) Review(ctx context.Context, p *entity.Principal, id string, c PracticumCommand) (entity.PracticumAttemptReview, error) {
	var out entity.PracticumAttemptReview
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		a, activation, err := practicumLockAttempt(tx, id)
		if err != nil {
			return err
		}
		if err = practicumRequire(p, practicumGrade, activation.Activation.Scope); err != nil {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err != nil {
			return err
		}
		out, err = practicumReview(tx, a, now)
		return err
	})
	return out, err
}
func (s *PracticumService) ManualGrade(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumManualInput) (entity.PracticumAttemptReview, error) {
	if in.GradeRevision < 0 || len(in.Grades) < 1 || len(in.Grades) > 20 || len(strings.TrimSpace(in.Reason)) < 1 || len(in.Reason) > 500 {
		return entity.PracticumAttemptReview{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.grade.manual", id, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumAttemptReview, target string, scope entity.Scope, err error) {
		a, activation, e := practicumLockAttempt(tx, id)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = activation.Activation.Scope
		if err = practicumRequire(p, practicumGrade, scope); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err != nil {
			return
		}
		if replay != "" {
			out, err = practicumReview(tx, a, now)
			return
		}
		if a.Attempt.Status == "in_progress" || a.Attempt.GradeRevision != in.GradeRevision || a.Attempt.Status == "graded" && !in.Publish {
			err = repository.ErrConflict
			return
		}
		questions := map[string]int{}
		for _, q := range a.Attempt.Questions {
			questions[q.ID] = q.Points
		}
		seen := map[string]bool{}
		total := 0
		for _, grade := range in.Grades {
			points, ok := questions[grade.QuestionID]
			if !ok || seen[grade.QuestionID] || grade.Score < 0 || grade.Score > points || len(grade.Feedback) > 4000 {
				err = ErrValidation
				return
			}
			seen[grade.QuestionID] = true
			total += grade.Score
		}
		if len(seen) != len(questions) {
			err = ErrValidation
			return
		}
		a.ManualGrades = append([]entity.PracticumManualGrade{}, in.Grades...)
		a.GradeReason = in.Reason
		a.Attempt.GradeRevision++
		a.Attempt.GradingStatus = "needs_manual"
		if in.Publish {
			a.Attempt.Status = "graded"
			a.Attempt.GradingStatus = "graded"
			a.Attempt.Score = &total
			percentage := float64(total) * 100 / float64(a.Attempt.MaxScore)
			a.Attempt.Percentage = &percentage
			for _, grade := range in.Grades {
				found := false
				for i := range a.Attempt.Answers {
					if a.Attempt.Answers[i].QuestionID == grade.QuestionID {
						v := grade.Score
						a.Attempt.Answers[i].Score = &v
						a.Attempt.Answers[i].Feedback = grade.Feedback
						found = true
					}
				}
				if !found {
					v := grade.Score
					a.Attempt.Answers = append(a.Attempt.Answers, entity.PracticumAnswer{QuestionID: grade.QuestionID, Score: &v, Feedback: grade.Feedback, UpdatedAt: now})
				}
			}
		}
		if err = practicumSaveAttempt(tx, a); err != nil {
			return
		}
		job, e := practicumJob(tx, id)
		if e != nil {
			err = e
			return
		}
		if job != nil {
			job.Status = a.Attempt.GradingStatus
			job.GradeRevision = a.Attempt.GradeRevision
			job.LeaseID = ""
			job.LeaseUntil = nil
			job.UpdatedAt = now
			if err = practicumSaveJob(tx, *job, a.Scope); err != nil {
				return
			}
		}
		out, err = practicumReview(tx, a, now)
		return
	})
}
func (s *PracticumService) BulkGrade(ctx context.Context, p *entity.Principal, c PracticumCommand, ids []string) ([]entity.PracticumGradingJob, error) {
	if len(ids) < 1 || len(ids) > 25 {
		return nil, ErrValidation
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return nil, ErrValidation
		}
		seen[id] = true
	}
	return practicumMutate(s, ctx, p, c, "practicum.grade.bulk", "", ids, func(tx repository.PracticumTx, replay string) (out []entity.PracticumGradingJob, target string, scope entity.Scope, err error) {
		out = []entity.PracticumGradingJob{}
		target = practicumHash(ids)
		scope = entity.Scope{Domain: "praktikum"}
		for _, id := range ids {
			a, activation, e := practicumLockAttempt(tx, id)
			if e != nil {
				err = e
				return
			}
			if err = practicumRequire(p, practicumGrade, activation.Activation.Scope); err != nil {
				return
			}
			now, e := tx.Now()
			if e != nil {
				err = e
				return
			}
			if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err != nil {
				return
			}
			if a.Attempt.Status == "in_progress" {
				err = repository.ErrConflict
				return
			}
			job, e := practicumJob(tx, id)
			if e != nil {
				err = e
				return
			}
			if job == nil {
				err = repository.ErrConflict
				return
			}
			if replay == "" && a.Attempt.Status != "graded" && a.Attempt.GradeRevision == 0 && job.Status != "running" {
				job.Status = "pending"
				job.LeaseID = ""
				job.LeaseUntil = nil
				job.UpdatedAt = now
				a.Attempt.GradingStatus = "pending"
				if err = practicumSaveJob(tx, *job, a.Scope); err != nil {
					return
				}
				if err = practicumSaveAttempt(tx, a); err != nil {
					return
				}
				if err = practicumAudit(tx, p, c, "practicum.grade.enqueued", id, a.Scope); err != nil {
					return
				}
			}
			copy := *job
			copy.LeaseID = ""
			out = append(out, copy)
		}
		return
	})
}
func (s *PracticumService) claimGrade(tx repository.PracticumTx, a *entity.PracticumAttemptRecord, now time.Time, p *entity.Principal, c PracticumCommand) (practicumGradeClaim, error) {
	var claim practicumGradeClaim
	job, err := practicumJob(tx, a.Attempt.AttemptID)
	if err != nil {
		return claim, err
	}
	if job == nil || a.Attempt.Status == "in_progress" {
		return claim, repository.ErrConflict
	}
	// Any saved manual work owns the grading revision, including unpublished drafts.
	if a.Attempt.Status == "graded" || a.Attempt.GradeRevision > 0 {
		return claim, nil
	}
	if job.Status == "running" && job.LeaseUntil != nil && now.Before(*job.LeaseUntil) {
		return claim, nil
	}
	job.Status = "running"
	job.LeaseID = practicumID("grade-lease")
	until := now.Add(s.options.GradingTimeout + 15*time.Second)
	job.LeaseUntil = &until
	job.GradeRevision = a.Attempt.GradeRevision
	job.UpdatedAt = now
	a.Attempt.GradingStatus = "running"
	if err = practicumSaveJob(tx, *job, a.Scope); err != nil {
		return claim, err
	}
	if err = practicumSaveAttempt(tx, *a); err != nil {
		return claim, err
	}
	if err = practicumAudit(tx, p, c, "practicum.grade.claimed", a.Attempt.AttemptID, a.Scope); err != nil {
		return claim, err
	}
	claim.LeaseID = job.LeaseID
	claim.Revision = job.GradeRevision
	claim.Request = entity.GradeRequest{AttemptID: a.Attempt.AttemptID, Items: []entity.GradeItem{}}
	answers := map[string]string{}
	for _, answer := range a.SubmittedAnswers {
		answers[answer.QuestionID] = answer.Value
	}
	for _, q := range a.Attempt.Questions {
		claim.Request.Items = append(claim.Request.Items, entity.GradeItem{QuestionID: q.ID, Type: q.Type, Prompt: q.Prompt, Answer: answers[q.ID], MaxScore: q.Points, Rubric: q.Rubric, ReferenceAnswer: q.ReferenceAnswer, Language: q.Language})
	}
	return claim, nil
}
func validPracticumGradeResult(request entity.GradeRequest, result entity.GradeResult) bool {
	if result.Status != "suggested" || len(result.Suggestions) != len(request.Items) || len(result.ProviderVersion) > 128 {
		return false
	}
	items := map[string]int{}
	for _, q := range request.Items {
		items[q.QuestionID] = q.MaxScore
	}
	seen := map[string]bool{}
	for _, grade := range result.Suggestions {
		max, ok := items[grade.QuestionID]
		if !ok || seen[grade.QuestionID] || grade.Score < 0 || grade.Score > max || len(grade.Feedback) > 4000 {
			return false
		}
		seen[grade.QuestionID] = true
	}
	return true
}
func (s *PracticumService) finishGrade(ctx context.Context, claim practicumGradeClaim, p *entity.Principal, c PracticumCommand) error {
	if claim.LeaseID == "" {
		return nil
	}
	result := entity.GradeResult{Status: "needs_manual"}
	if s.options.Grader != nil {
		providerCtx, cancel := context.WithTimeout(ctx, s.options.GradingTimeout)
		value, err := s.options.Grader.Grade(providerCtx, claim.Request)
		cancel()
		if err == nil && validPracticumGradeResult(claim.Request, value) {
			result = value
		}
	}
	// Finalization has its own short budget even when the provider used the request
	// deadline. An abandoned claim remains durably leased and is recoverable by cron.
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.options.Repository.Transaction(finalizeCtx, func(tx repository.PracticumTx) error {
		a, _, err := practicumLockAttempt(tx, claim.Request.AttemptID)
		if err != nil {
			return err
		}
		job, err := practicumJob(tx, a.Attempt.AttemptID)
		if err != nil {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		if job == nil || job.LeaseID != claim.LeaseID || job.Status != "running" || job.LeaseUntil == nil || !now.Before(*job.LeaseUntil) || a.Attempt.GradeRevision != claim.Revision || a.Attempt.Status == "graded" {
			return nil
		}
		job.Status = "needs_manual"
		job.LeaseID = ""
		job.LeaseUntil = nil
		job.UpdatedAt = now
		job.Simulated = result.Simulated
		job.Provider = result.ProviderVersion
		job.Suggestions = append([]entity.SuggestedGrade{}, result.Suggestions...)
		a.Attempt.GradingStatus = "needs_manual"
		if err = practicumSaveJob(tx, *job, a.Scope); err != nil {
			return err
		}
		if err = practicumSaveAttempt(tx, a); err != nil {
			return err
		}
		return practicumAudit(tx, p, c, "practicum.grade.suggestions", a.Attempt.AttemptID, a.Scope)
	})
}
func (s *PracticumService) Grade(ctx context.Context, p *entity.Principal, c PracticumCommand, id string) (entity.PracticumAttemptReview, error) {
	claim, err := practicumMutate(s, ctx, p, c, "practicum.grade.process", id, struct{}{}, func(tx repository.PracticumTx, replay string) (out practicumGradeClaim, target string, scope entity.Scope, err error) {
		a, activation, e := practicumLockAttempt(tx, id)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = activation.Activation.Scope
		if err = practicumRequire(p, practicumGrade, scope); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err != nil {
			return
		}
		if replay == "" {
			out, err = s.claimGrade(tx, &a, now, p, c)
			if err != nil {
				return
			}
		}
		out.Review, err = practicumReview(tx, a, now)
		return
	})
	if err != nil {
		return entity.PracticumAttemptReview{}, err
	}
	if claim.LeaseID == "" {
		return claim.Review, nil
	}
	if err = s.finishGrade(ctx, claim, p, c); err != nil {
		return entity.PracticumAttemptReview{}, err
	}
	return s.Review(ctx, p, id, c)
}

// Matches the jobs_processed maximum in the OpenAPI contract.
const practicumReconcileBatch = 20

type PracticumReconcileResult struct {
	Expired       int `json:"expired"`
	JobsProcessed int `json:"jobs_processed"`
}

// Reconcile does bounded synchronous work. Only the independently authenticated
// internal endpoint may call this method; it does not create a background worker.
// At most practicumReconcileBatch jobs are claimed per call so one cron invocation
// cannot drain an unbounded queue.
func (s *PracticumService) Reconcile(ctx context.Context, c PracticumCommand) (PracticumReconcileResult, error) {
	var out PracticumReconcileResult
	if err := s.expireRunnerLeases(ctx); err != nil {
		return out, err
	}
	if err := s.expireDueAttempts(ctx, c, &out); err != nil {
		return out, err
	}
	for out.JobsProcessed < practicumReconcileBatch {
		if ctx.Err() != nil {
			break
		}
		claim, err := s.claimNextGrade(ctx, c)
		if err != nil {
			return out, err
		}
		if claim.LeaseID == "" {
			break
		}
		if err := s.finishGrade(ctx, claim, nil, c); err != nil {
			return out, err
		}
		out.JobsProcessed++
	}
	return out, nil
}

func (s *PracticumService) expireDueAttempts(ctx context.Context, c PracticumCommand, out *PracticumReconcileResult) error {
	return s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		rows, err := tx.List(repository.PracticumAttempts, repository.PracticumFilter{Status: "in_progress", DueBefore: &now, Limit: 100})
		if err != nil {
			return err
		}
		for _, r := range rows {
			a, activation, err := practicumLockAttempt(tx, r.ID)
			if err != nil {
				return err
			}
			lockedNow, err := tx.Now()
			if err != nil {
				return err
			}
			if err = practicumReconcileAttempt(tx, &a, activation, lockedNow, nil, c); err != nil {
				return err
			}
			if a.Attempt.Status != "in_progress" {
				out.Expired++
			}
		}
		return nil
	})
}

func (s *PracticumService) claimNextGrade(ctx context.Context, c PracticumCommand) (practicumGradeClaim, error) {
	var claim practicumGradeClaim
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		jobs, err := tx.List(repository.PracticumJobs, repository.PracticumFilter{Status: "running", DueBefore: &now, Limit: 1})
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			jobs, err = tx.List(repository.PracticumJobs, repository.PracticumFilter{Status: "pending", Limit: 1})
			if err != nil {
				return err
			}
		}
		if len(jobs) == 0 {
			return nil
		}
		var job entity.PracticumGradingJob
		if err = json.Unmarshal(jobs[0].Data, &job); err != nil {
			return err
		}
		a, _, err := practicumLockAttempt(tx, job.AttemptID)
		if err != nil {
			return err
		}
		lockedNow, err := tx.Now()
		if err != nil {
			return err
		}
		claim, err = s.claimGrade(tx, &a, lockedNow, nil, c)
		return err
	})
	return claim, err
}
