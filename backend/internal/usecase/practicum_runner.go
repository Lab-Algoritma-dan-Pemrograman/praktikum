package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type PracticumRunnerInfo struct {
	Languages []entity.RunnerLanguage `json:"languages"`
	Simulated bool                    `json:"simulated"`
	Available bool                    `json:"available"`
}

func (s *PracticumService) RunnerInfo() PracticumRunnerInfo {
	if s.options.Runner == nil {
		return PracticumRunnerInfo{Languages: []entity.RunnerLanguage{}}
	}
	return PracticumRunnerInfo{Languages: s.options.Runner.Languages(), Available: s.options.Runner.Available(), Simulated: s.options.Runner.Simulated()}
}

type practicumRunRecord struct {
	ID         string           `json:"id"`
	AttemptID  string           `json:"attempt_id"`
	UserID     int64            `json:"user_id"`
	Status     string           `json:"status"`
	LeaseUntil time.Time        `json:"lease_until"`
	Result     entity.RunResult `json:"result"`
}
type practicumRunClaim struct {
	Record  practicumRunRecord
	Execute bool
}

func practicumSaveRun(tx repository.PracticumTx, run practicumRunRecord, scope entity.Scope) error {
	return practicumPut(tx, repository.PracticumRuns, run.ID, run.AttemptID, run.UserID, scope, run.Status, run.ID, &run.LeaseUntil, run)
}

// Stale cleanup commits separately from admission: an over-quota request cannot
// roll back cleanup and strand every subsequent run behind expired leases.
func (s *PracticumService) expireRunnerLeases(ctx context.Context) error {
	return s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		rows, err := tx.List(repository.PracticumRuns, repository.PracticumFilter{Status: "running", DueBefore: &now, Limit: 100})
		if err != nil {
			return err
		}
		for _, row := range rows {
			var lease practicumRunRecord
			if err := json.Unmarshal(row.Data, &lease); err != nil {
				return err
			}
			if now.Before(lease.LeaseUntil) {
				continue
			}
			lease.Status = "finished"
			lease.Result.Status = "provider_error"
			if err := practicumSaveRun(tx, lease, row.Scope); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *PracticumService) Run(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumRunInput) (entity.RunResult, error) {
	if in.QuestionID == "" || len(in.Source) < 1 || len(in.Source) > 64<<10 || len(in.Stdin) > 16<<10 || len(in.Language) > 32 || len(in.Version) > 32 {
		return entity.RunResult{}, ErrValidation
	}
	claim, err := practicumMutate(s, ctx, p, c, "practicum.attempt.run", id, in, func(tx repository.PracticumTx, replay string) (out practicumRunClaim, target string, scope entity.Scope, err error) {
		a, activation, e := practicumLockAttempt(tx, id)
		if e != nil {
			err = e
			return
		}
		scope = activation.Activation.Scope
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
		if replay != "" {
			run, e := practicumRead[practicumRunRecord](tx, repository.PracticumRuns, replay, true)
			if e != nil {
				err = e
				return
			}
			if run.Status == "running" {
				if now.Before(run.LeaseUntil) {
					err = repository.ErrConflict
					return
				}
				run.Status = "finished"
				run.Result.Status = "provider_error"
				if err = practicumSaveRun(tx, run, scope); err != nil {
					return
				}
			}
			return practicumRunClaim{Record: run}, replay, scope, nil
		}
		if a.Attempt.Status != "in_progress" {
			err = ErrPracticumClosed
			return
		}
		found := false
		for _, q := range a.Attempt.Questions {
			if q.ID == in.QuestionID && q.Type == "coding" && q.Language == in.Language {
				found = true
			}
		}
		if !found {
			err = ErrValidation
			return
		}
		if s.options.Runner == nil || !s.options.Runner.Available() {
			err = ErrPracticumUnavailable
			return
		}
		allowed := false
		for _, language := range s.options.Runner.Languages() {
			if language.Language == in.Language && language.Version == in.Version {
				allowed = true
			}
		}
		if !allowed {
			err = ErrValidation
			return
		}
		leases, e := tx.List(repository.PracticumRuns, repository.PracticumFilter{Status: "running", LiveAfter: &now, Limit: 1000})
		if e != nil {
			err = e
			return
		}
		if len(leases) >= 1000 {
			err = ErrPracticumBusy
			return
		}
		global, user := 0, 0
		for _, row := range leases {
			var lease practicumRunRecord
			if err = json.Unmarshal(row.Data, &lease); err != nil {
				return
			}
			if !now.Before(lease.LeaseUntil) {
				lease.Status = "finished"
				lease.Result.Status = "provider_error"
				if err = practicumSaveRun(tx, lease, row.Scope); err != nil {
					return
				}
				continue
			}
			global++
			if lease.UserID == p.User.ID {
				user++
			}
		}
		if global >= s.options.RunnerGlobalLimit || user >= s.options.RunnerUserLimit {
			err = ErrPracticumBusy
			return
		}
		// Fixed-window admission is stored in the same transaction as the lease.
		quotaID := "runquota-" + practicumHash([]any{p.User.ID, a.Attempt.ActivationID, now.Truncate(time.Minute)})
		quota := struct {
			Hits int `json:"hits"`
		}{Hits: 0}
		row, e := tx.Get(repository.PracticumOperations, quotaID)
		if e == nil {
			if err = json.Unmarshal(row.Data, &quota); err != nil {
				return
			}
		} else if !errors.Is(e, repository.ErrPracticumNotFound) {
			err = e
			return
		}
		if quota.Hits >= 30 {
			err = ErrPracticumBusy
			return
		}
		quota.Hits++
		if err = practicumPut(tx, repository.PracticumOperations, quotaID, "", p.User.ID, scope, "", quotaID, nil, quota); err != nil {
			return
		}
		target = practicumID("run")
		// Keep the originating request correlation stable on cached retries. The
		// lease's internal ID remains the durable execution/idempotency identity.
		requestID := c.RequestID
		if requestID == "" {
			requestID = target
		}
		run := practicumRunRecord{ID: target, AttemptID: id, UserID: p.User.ID, Status: "running", LeaseUntil: now.Add(s.options.RunnerTimeout + 15*time.Second), Result: entity.RunResult{Status: "provider_error", RequestID: requestID, SourceHash: security.Hash(in.Source), Simulated: s.options.Runner.Simulated()}}
		err = practicumSaveRun(tx, run, scope)
		out = practicumRunClaim{Record: run, Execute: true}
		return
	})
	if err != nil {
		return entity.RunResult{}, err
	}
	if !claim.Execute {
		return claim.Record.Result, nil
	}
	request := entity.RunRequest{Language: in.Language, Version: in.Version, Source: in.Source, Stdin: in.Stdin, RequestID: claim.Record.Result.RequestID}
	providerCtx, cancel := context.WithTimeout(ctx, s.options.RunnerTimeout)
	result, providerErr := s.options.Runner.Run(providerCtx, request)
	cancel()
	if providerErr != nil || !validPracticumRunResult(result) || (!s.options.Runner.Simulated() && result.RunnerVersion != request.Version) {
		result = entity.RunResult{Status: "provider_error"}
	}
	result.RequestID = claim.Record.Result.RequestID
	result.SourceHash = security.Hash(in.Source)
	result.Simulated = s.options.Runner.Simulated()
	finalizeCtx, finalizeCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finalizeCancel()
	err = s.options.Repository.Transaction(finalizeCtx, func(tx repository.PracticumTx) error {
		a, _, err := practicumLockAttempt(tx, id)
		if err != nil {
			return err
		}
		run, err := practicumRead[practicumRunRecord](tx, repository.PracticumRuns, claim.Record.ID, true)
		if err != nil {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		if run.Status != "running" {
			result = run.Result
			return nil
		}
		if !now.Before(run.LeaseUntil) {
			result = run.Result
			result.Status = "provider_error"
		}
		run.Status = "finished"
		run.Result = result
		if err = practicumSaveRun(tx, run, a.Scope); err != nil {
			return err
		}
		return practicumAudit(tx, p, c, "practicum.run.finished", run.ID, a.Scope)
	})
	return result, err
}
func validPracticumRunResult(r entity.RunResult) bool {
	if len(r.Stdout) > 64<<10 || len(r.Stderr) > 64<<10 || len(r.RunnerVersion) > 128 || r.DurationMS < 0 || r.DurationMS > 120000 {
		return false
	}
	switch r.Status {
	case "success", "compile_error", "runtime_error", "timeout", "quota_exceeded", "provider_error":
		return true
	}
	return false
}
