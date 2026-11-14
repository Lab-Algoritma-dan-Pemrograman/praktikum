package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func practicumSaveAttempt(tx repository.PracticumTx, a entity.PracticumAttemptRecord) error {
	return practicumPut(tx, repository.PracticumAttempts, a.Attempt.AttemptID, a.Attempt.ActivationID, a.Attempt.UserID, a.Scope, a.Attempt.Status, practicumAttemptKey(a.Attempt.ActivationID, a.Attempt.UserID), &a.Attempt.DeadlineAt, a)
}
func practicumSaveJob(tx repository.PracticumTx, j entity.PracticumGradingJob, scope entity.Scope) error {
	return practicumPut(tx, repository.PracticumJobs, j.ID, j.AttemptID, 0, scope, j.Status, j.AttemptID, j.LeaseUntil, j)
}
func practicumLockAttempt(tx repository.PracticumTx, id string) (entity.PracticumAttemptRecord, entity.PracticumActivationRecord, error) {
	// This first lookup reads immutable metadata without an attempt row lock.
	var a entity.PracticumAttemptRecord
	var activation entity.PracticumActivationRecord
	row, err := tx.Get(repository.PracticumAttempts, id)
	if err != nil {
		return a, activation, err
	}
	activation, err = practicumRead[entity.PracticumActivationRecord](tx, repository.PracticumActivations, row.ParentID, true)
	if err != nil {
		return a, activation, err
	}
	a, err = practicumRead[entity.PracticumAttemptRecord](tx, repository.PracticumAttempts, id, true)
	return a, activation, err
}
func practicumOwn(p *entity.Principal, a entity.PracticumAttemptRecord, activation entity.PracticumActivationRecord) error {
	if a.Attempt.UserID != p.User.ID || !practicumEligible(p, activation) {
		return authorization.ErrNotFound
	}
	return nil
}
func practicumSubmit(tx repository.PracticumTx, a *entity.PracticumAttemptRecord, now time.Time, reason string, p *entity.Principal, c PracticumCommand) error {
	if a.Attempt.Status != "in_progress" {
		return nil
	}
	a.Attempt.Status = "submitted"
	a.Attempt.SubmissionReason = reason
	a.Attempt.SubmittedAt = &now
	a.Attempt.GradingStatus = "pending"
	a.SubmittedAnswers = append([]entity.PracticumAnswer{}, a.Attempt.Answers...)
	if err := practicumSaveAttempt(tx, *a); err != nil {
		return err
	}
	job := entity.PracticumGradingJob{ID: a.Attempt.AttemptID, AttemptID: a.Attempt.AttemptID, Status: "pending", GradeRevision: a.Attempt.GradeRevision, Suggestions: []entity.SuggestedGrade{}, UpdatedAt: now}
	if err := practicumSaveJob(tx, job, a.Scope); err != nil {
		return err
	}
	return practicumAudit(tx, p, c, "practicum.attempt.submitted."+reason, a.Attempt.AttemptID, a.Scope)
}
func practicumReconcileAttempt(tx repository.PracticumTx, a *entity.PracticumAttemptRecord, activation entity.PracticumActivationRecord, now time.Time, p *entity.Principal, c PracticumCommand) error {
	if a.Attempt.Status != "in_progress" {
		return nil
	}
	if !now.Before(a.Attempt.DeadlineAt) {
		return practicumSubmit(tx, a, now, "deadline", p, c)
	}
	if !activation.Activation.IsOpen {
		return practicumSubmit(tx, a, now, "closed", p, c)
	}
	return nil
}
func (s *PracticumService) Start(ctx context.Context, p *entity.Principal, c PracticumCommand, in PracticumStartInput) (entity.PracticumAttempt, error) {
	if in.ActivationID == "" || len(in.ActivationID) > 128 || len(in.AccessToken) > 256 {
		return entity.PracticumAttempt{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.attempt.start", in.ActivationID, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumAttempt, id string, scope entity.Scope, err error) {
		activation, e := practicumRead[entity.PracticumActivationRecord](tx, repository.PracticumActivations, in.ActivationID, true)
		if e != nil {
			err = e
			return
		}
		scope = activation.Activation.Scope
		if !practicumEligible(p, activation) {
			err = authorization.ErrNotFound
			return
		}
		rows, e := tx.List(repository.PracticumAttempts, repository.PracticumFilter{UniqueKey: practicumAttemptKey(in.ActivationID, p.User.ID), Limit: 1})
		if e != nil {
			err = e
			return
		}
		var attempt entity.PracticumAttemptRecord
		if len(rows) > 0 {
			attempt, err = practicumRead[entity.PracticumAttemptRecord](tx, repository.PracticumAttempts, rows[0].ID, true)
			if err != nil {
				return
			}
			now, e := tx.Now()
			if e != nil {
				err = e
				return
			}
			err = practicumReconcileAttempt(tx, &attempt, activation, now, p, c)
			return practicumAttemptView(attempt, now), attempt.Attempt.AttemptID, scope, err
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, activation.Activation.SessionID, false)
		if e != nil {
			err = e
			return
		}
		if !session.Active || !activation.Activation.IsOpen || activation.Activation.ClosesAt != nil && !now.Before(*activation.Activation.ClosesAt) {
			err = ErrPracticumClosed
			return
		}
		if activation.Course.TokenRequired && !practicumEqualToken(activation.TokenHash, in.AccessToken) {
			err = authorization.ErrForbidden
			return
		}
		if activation.Course.Kind == "ujian_praktik" && !containsInt(activation.Activation.EntryApprovedIDs, p.User.ID) {
			err = authorization.ErrForbidden
			return
		}
		deadline := now.Add(time.Duration(activation.Course.DurationMinutes) * time.Minute)
		if activation.Activation.TimerMode == "shared" {
			if activation.Activation.StartedAt == nil {
				activation.Activation.StartedAt = &now
				if activation.Activation.ClosesAt != nil && activation.Activation.ClosesAt.Before(deadline) {
					deadline = *activation.Activation.ClosesAt
				}
				activation.Activation.DeadlineAt = &deadline
				activation.Activation.Version++
				if err = practicumSaveActivation(tx, activation); err != nil {
					return
				}
			} else {
				deadline = *activation.Activation.DeadlineAt
			}
		}
		if activation.Activation.ClosesAt != nil && activation.Activation.ClosesAt.Before(deadline) {
			deadline = *activation.Activation.ClosesAt
		}
		if !now.Before(deadline) {
			err = ErrPracticumClosed
			return
		}
		id = practicumID("attempt")
		maxScore := 0
		for _, q := range activation.Questions {
			maxScore += q.Points
		}
		attempt = entity.PracticumAttemptRecord{Attempt: entity.PracticumAttempt{AttemptID: id, ActivationID: in.ActivationID, SessionID: activation.Activation.SessionID, CourseID: activation.Course.ID, UserID: p.User.ID, Status: "in_progress", GradingStatus: "pending", StartedAt: now, DeadlineAt: deadline, Questions: activation.Questions, Answers: []entity.PracticumAnswer{}, MaxScore: maxScore}, Scope: scope, UserName: p.User.Name, SubmittedAnswers: []entity.PracticumAnswer{}, ManualGrades: []entity.PracticumManualGrade{}}
		err = practicumSaveAttempt(tx, attempt)
		out = practicumAttemptView(attempt, now)
		return
	})
}
func (s *PracticumService) Attempt(ctx context.Context, p *entity.Principal, id string, c PracticumCommand) (entity.PracticumAttempt, error) {
	var out entity.PracticumAttempt
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		a, activation, err := practicumLockAttempt(tx, id)
		if err != nil {
			return err
		}
		if err = practicumOwn(p, a, activation); err != nil {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err != nil {
			return err
		}
		out = practicumAttemptView(a, now)
		return nil
	})
	return out, err
}
func (s *PracticumService) SaveAnswers(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumSaveInput) (entity.PracticumAttempt, error) {
	if in.Revision < 0 || len(in.Answers) < 1 || len(in.Answers) > 20 {
		return entity.PracticumAttempt{}, ErrValidation
	}
	total := 0
	seen := map[string]bool{}
	for _, answer := range in.Answers {
		if answer.QuestionID == "" || seen[answer.QuestionID] || len(answer.Value) > 64<<10 || len(answer.Language) > 32 {
			return entity.PracticumAttempt{}, ErrValidation
		}
		seen[answer.QuestionID] = true
		total += len(answer.Value)
	}
	if total > 256<<10 {
		return entity.PracticumAttempt{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.attempt.answers", id, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumAttempt, target string, scope entity.Scope, err error) {
		a, activation, e := practicumLockAttempt(tx, id)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = a.Scope
		if err = practicumOwn(p, a, activation); err != nil {
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
		out = practicumAttemptView(a, now)
		if replay != "" {
			return
		}
		if a.Attempt.Status != "in_progress" {
			err = ErrPracticumClosed
			return
		}
		if a.Attempt.Revision != in.Revision {
			err = repository.ErrConflict
			return
		}
		questions := map[string]entity.PracticumQuestion{}
		for _, q := range a.Attempt.Questions {
			questions[q.ID] = q
		}
		for _, answer := range in.Answers {
			q, ok := questions[answer.QuestionID]
			if !ok {
				err = ErrValidation
				return
			}
			if q.Type == "essay" && answer.Language != "" || q.Type == "coding" && answer.Language != "" && answer.Language != q.Language {
				err = ErrValidation
				return
			}
			saved := entity.PracticumAnswer{QuestionID: answer.QuestionID, Value: answer.Value, Language: answer.Language, UpdatedAt: now}
			if q.Type == "coding" && saved.Language == "" {
				saved.Language = q.Language
			}
			found := false
			for i, v := range a.Attempt.Answers {
				if v.QuestionID == answer.QuestionID {
					a.Attempt.Answers[i] = saved
					found = true
					break
				}
			}
			if !found {
				a.Attempt.Answers = append(a.Attempt.Answers, saved)
			}
		}
		total := 0
		for _, v := range a.Attempt.Answers {
			total += len(v.Value)
		}
		if total > 256<<10 {
			err = ErrValidation
			return
		}
		a.Attempt.Revision++
		err = practicumSaveAttempt(tx, a)
		out = practicumAttemptView(a, now)
		return
	})
}
func (s *PracticumService) Submit(ctx context.Context, p *entity.Principal, c PracticumCommand, id string) (entity.PracticumAttempt, error) {
	return practicumMutate(s, ctx, p, c, "practicum.attempt.submit", id, struct{}{}, func(tx repository.PracticumTx, replay string) (out entity.PracticumAttempt, target string, scope entity.Scope, err error) {
		a, activation, e := practicumLockAttempt(tx, id)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = a.Scope
		if err = practicumOwn(p, a, activation); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		if err = practicumReconcileAttempt(tx, &a, activation, now, p, c); err == nil {
			err = practicumSubmit(tx, &a, now, "manual", p, c)
		}
		out = practicumAttemptView(a, now)
		return
	})
}
func (s *PracticumService) Close(ctx context.Context, p *entity.Principal, c PracticumCommand, sessionID, courseID, activationID string) (entity.PracticumActivation, error) {
	return practicumMutate(s, ctx, p, c, "practicum.activation.close", activationID, []string{sessionID, courseID, activationID}, func(tx repository.PracticumTx, replay string) (out entity.PracticumActivation, target string, scope entity.Scope, err error) {
		activation, e := practicumRead[entity.PracticumActivationRecord](tx, repository.PracticumActivations, activationID, true)
		if e != nil {
			err = e
			return
		}
		target = activationID
		scope = activation.Activation.Scope
		if activation.Activation.SessionID != sessionID || activation.Course.ID != courseID {
			err = authorization.ErrNotFound
			return
		}
		if err = practicumRequire(p, practicumManage, scope); err != nil {
			return
		}
		if !activation.Activation.IsOpen {
			out = practicumPublicActivation(activation.Activation, true)
			return
		}
		rows, e := tx.List(repository.PracticumAttempts, repository.PracticumFilter{ParentID: activationID, Status: "in_progress", Limit: 1000})
		if e != nil {
			err = e
			return
		}
		attempts := []entity.PracticumAttemptRecord{}
		for _, r := range rows {
			a, e := practicumRead[entity.PracticumAttemptRecord](tx, repository.PracticumAttempts, r.ID, true)
			if e != nil {
				err = e
				return
			}
			attempts = append(attempts, a)
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		activation.Activation.IsOpen = false
		activation.Activation.Version++
		if err = practicumSaveActivation(tx, activation); err != nil {
			return
		}
		for i := range attempts {
			if err = practicumReconcileAttempt(tx, &attempts[i], activation, now, p, c); err != nil {
				return
			}
		}
		out = practicumPublicActivation(activation.Activation, true)
		return
	})
}
func (s *PracticumService) Monitoring(ctx context.Context, p *entity.Principal, sessionID, activationID string, page PracticumPage) ([]entity.PracticumMonitoringRow, error) {
	out := []entity.PracticumMonitoringRow{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		scopes := practicumScopes(p, practicumManage, practicumGrade)
		if sessionID != "" {
			narrowed := []entity.Scope{}
			for _, scope := range scopes {
				if intersect, ok := scope.Intersect(entity.Scope{SessionID: sessionID}); ok {
					narrowed = append(narrowed, intersect)
				}
			}
			scopes = narrowed
		}
		rows, err := tx.List(repository.PracticumAttempts, repository.PracticumFilter{ParentID: activationID, Scopes: scopes, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			a, activation, err := practicumLockAttempt(tx, r.ID)
			if err != nil {
				return err
			}
			now, err := tx.Now()
			if err != nil {
				return err
			}
			if err = practicumReconcileAttempt(tx, &a, activation, now, p, PracticumCommand{OperationID: "practicum.monitoring"}); err != nil {
				return err
			}
			v := a.Attempt
			out = append(out, entity.PracticumMonitoringRow{AttemptID: v.AttemptID, UserID: v.UserID, UserName: a.UserName, ActivationID: v.ActivationID, SessionID: v.SessionID, CourseID: v.CourseID, Status: v.Status, GradingStatus: v.GradingStatus, AnswerCount: len(v.Answers), QuestionCount: len(v.Questions), DeadlineAt: v.DeadlineAt, SubmittedAt: v.SubmittedAt})
		}
		return nil
	})
	return out, err
}
func (s *PracticumService) Grades(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.PracticumGradeSummary, error) {
	out := []entity.PracticumGradeSummary{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		rows, err := tx.List(repository.PracticumAttempts, repository.PracticumFilter{OwnerID: p.User.ID, Scopes: practicumStudentScopes(p), Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var a entity.PracticumAttemptRecord
			if err = json.Unmarshal(r.Data, &a); err != nil {
				return err
			}
			activation, err := practicumRead[entity.PracticumActivationRecord](tx, repository.PracticumActivations, a.Attempt.ActivationID, false)
			if err != nil {
				return err
			}
			if !practicumEligible(p, activation) {
				continue
			}
			session, err := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, a.Attempt.SessionID, false)
			if err != nil {
				return err
			}
			v := practicumAttemptView(a, time.Time{})
			out = append(out, entity.PracticumGradeSummary{AttemptID: v.AttemptID, SessionID: v.SessionID, SessionTitle: session.Title, CourseTitle: activation.Course.Title, CourseKind: activation.Course.Kind, GradingStatus: v.GradingStatus, Score: v.Score, MaxScore: v.MaxScore, Percentage: v.Percentage, SubmittedAt: v.SubmittedAt})
		}
		return nil
	})
	return out, err
}
func practicumJob(tx repository.PracticumTx, id string) (*entity.PracticumGradingJob, error) {
	j, err := practicumRead[entity.PracticumGradingJob](tx, repository.PracticumJobs, id, false)
	if errors.Is(err, repository.ErrPracticumNotFound) {
		return nil, nil
	}
	return &j, err
}
