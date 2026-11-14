package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type learningRunRecord struct {
	ID            string                      `json:"id"`
	LessonID      string                      `json:"lesson_id"`
	UserID        int64                       `json:"user_id"`
	Status        string                      `json:"status"`
	LeaseUntil    time.Time                   `json:"lease_until"`
	Result        entity.RunResult            `json:"result"`
	Language      string                      `json:"language"`
	LessonHash    string                      `json:"lesson_hash"`
	LessonVersion int64                       `json:"lesson_version"`
	TestResults   []entity.LearningTestResult `json:"test_results"`
	AllPassed     bool                        `json:"all_passed"`
}

type learningRunClaim struct {
	Record    learningRunRecord
	TestCases []entity.LearningTestCase
	Scope     entity.Scope
	Execute   bool
}

func learningSaveRun(tx repository.LearningTx, run learningRunRecord, scope entity.Scope) error {
	return learningPut(tx, repository.LearningRuns, run.ID, run.LessonID, run.UserID, scope, run.Status, run.ID, &run.LeaseUntil, run)
}

func learningCodeRequired(lesson entity.LearningLesson) bool {
	return lesson.StarterCode != "" || len(lesson.TestCases) > 0
}

// The index points at a finished immutable snapshot, never at client assertions.
// Including the full authored definition/version invalidates proof after edits.
func learningProofID(userID int64, lessonID, language, sourceHash, lessonHash string) string {
	return "lproof-" + learningHash([]any{userID, lessonID, language, sourceHash, lessonHash})
}

func (s *LearningService) learningCodeVerified(tx repository.LearningTx, userID int64, lesson entity.LearningLesson, source string) (bool, error) {
	if !learningCodeRequired(lesson) {
		return true, nil
	}
	sourceHash, lessonHash := security.Hash(source), learningHash(lesson)
	proofID := learningProofID(userID, lesson.ID, lesson.Language, sourceHash, lessonHash)
	proof, err := learningReadRecord[learningOperation](tx, repository.LearningOperations, proofID, false)
	if errors.Is(err, repository.ErrLearningNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	run, err := learningReadRecord[learningRunRecord](tx, repository.LearningRuns, proof.TargetID, false)
	if errors.Is(err, repository.ErrLearningNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return run.Status == "finished" && run.AllPassed && run.UserID == userID &&
		run.LessonID == lesson.ID && run.Language == lesson.Language &&
		run.Result.SourceHash == sourceHash && run.LessonHash == lessonHash &&
		run.LessonVersion == lesson.Version && s.learningRunPassed(run.Result), nil
}

func (s *LearningService) learningRunPassed(result entity.RunResult) bool {
	return validPracticumRunResult(result) && result.Status == "success" &&
		result.ExitCode != nil && *result.ExitCode == 0 && !result.Truncated &&
		(!result.Simulated || s.options.AllowSimulatedVerification)
}

// Match exact output, tolerating platform line endings and terminal newlines,
// but not leading/trailing spaces, extra text, or nonempty output for empty expected.
func learningOutput(output string) string {
	return strings.TrimRight(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
}

func learningRunResponse(run learningRunRecord) entity.LearningRunResponse {
	tests := run.TestResults
	if tests == nil {
		tests = []entity.LearningTestResult{}
	}
	return entity.LearningRunResponse{Result: run.Result, TestResults: tests, AllPassed: run.AllPassed}
}

func (s *LearningService) executeLearningRun(ctx context.Context, in LearningPlaygroundInput, claim learningRunClaim) learningRunRecord {
	run := claim.Record
	run.TestResults = []entity.LearningTestResult{}
	run.AllPassed = true
	// One deadline for the entire bounded, sequential testcase batch; no retries.
	providerCtx, cancel := context.WithTimeout(ctx, s.options.RunnerTimeout)
	defer cancel()
	count := max(1, len(claim.TestCases))
	for i := 0; i < count; i++ {
		stdin := in.Stdin
		if len(claim.TestCases) > 0 {
			stdin = claim.TestCases[i].Input
		}
		request := entity.RunRequest{Language: in.Language, Version: in.Version, Source: in.Source, Stdin: stdin, RequestID: run.Result.RequestID}
		result := entity.RunResult{Status: "timeout"}
		if providerCtx.Err() == nil {
			var err error
			result, err = s.options.Runner.Run(providerCtx, request)
			if err != nil || !validPracticumRunResult(result) || result.RunnerVersion == "" || result.RunnerVersion != request.Version {
				result = entity.RunResult{Status: "provider_error"}
			}
			if providerCtx.Err() != nil {
				result = entity.RunResult{Status: "timeout"}
			}
		}
		result.RequestID = claim.Record.Result.RequestID
		result.SourceHash = security.Hash(in.Source)
		// A provider cannot downgrade its own simulated result into real evidence.
		result.Simulated = result.Simulated || s.options.Runner.Simulated()
		passed := s.learningRunPassed(result)
		if len(claim.TestCases) > 0 {
			tc := claim.TestCases[i]
			passed = passed && learningOutput(result.Stdout) == learningOutput(tc.ExpectedOutput)
			run.TestResults = append(run.TestResults, entity.LearningTestResult{
				Description: tc.Description, Passed: passed,
				ExpectedOutput: tc.ExpectedOutput, ActualOutput: result.Stdout,
			})
		}
		if i == 0 || (run.AllPassed && !passed) {
			run.Result = result
		}
		run.AllPassed = run.AllPassed && passed
	}
	return run
}

func (s *LearningService) PlaygroundRun(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningPlaygroundInput) (entity.LearningRunResponse, error) {
	if len(in.Source) < 1 || len(in.Source) > 64<<10 || len(in.Stdin) > 16<<10 || len(in.Language) > 32 || len(in.Version) > 32 || len(in.LessonID) > 128 {
		return entity.LearningRunResponse{}, ErrValidation
	}
	claim, err := learningMutate(s, ctx, p, c, "learning.playground.run", in.LessonID, in, func(tx repository.LearningTx, replay string) (out learningRunClaim, target string, scope entity.Scope, err error) {
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		scope = entity.Scope{Domain: "elearning"}
		var testCases []entity.LearningTestCase
		var lessonDefinition entity.LearningLesson
		if in.LessonID != "" {
			lesson, level, _, e := loadLessonAndCheckAccess(tx, p, in.LessonID, now, false)
			if e != nil {
				err = e
				return
			}
			if lesson.Language != in.Language {
				err = ErrValidation
				return
			}
			scope = level.Scope
			testCases = lesson.TestCases
			lessonDefinition = lesson
			if len(testCases) > 20 {
				err = ErrValidation
				return
			}
		} else if len(learningScopes(p, learningRead, learningManage, learningMonitor)) == 0 {
			err = authorization.ErrForbidden
			return
		}
		if replay != "" {
			run, e := learningReadRecord[learningRunRecord](tx, repository.LearningRuns, replay, true)
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
				if err = learningSaveRun(tx, run, scope); err != nil {
					return
				}
			}
			return learningRunClaim{Record: run, TestCases: testCases, Scope: scope}, replay, scope, nil
		}
		if s.options.Runner == nil || !s.options.Runner.Available() {
			err = ErrLearningUnavailable
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
		leases, e := tx.List(repository.LearningRuns, repository.LearningFilter{Status: "running", Limit: 1000})
		if e != nil {
			err = e
			return
		}
		global, user := 0, 0
		for _, row := range leases {
			var lease learningRunRecord
			if err = json.Unmarshal(row.Data, &lease); err != nil {
				return
			}
			if !now.Before(lease.LeaseUntil) {
				lease.Status = "finished"
				lease.Result.Status = "provider_error"
				if err = learningSaveRun(tx, lease, row.Scope); err != nil {
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
			err = ErrLearningBusy
			return
		}
		quotaID := "lrunquota-" + learningHash([]any{p.User.ID, now.Truncate(time.Minute)})
		quota := struct {
			Hits int `json:"hits"`
		}{Hits: 0}
		row, e := tx.Get(repository.LearningOperations, quotaID)
		if e == nil {
			if err = json.Unmarshal(row.Data, &quota); err != nil {
				return
			}
		} else if !errors.Is(e, repository.ErrLearningNotFound) {
			err = e
			return
		}
		cost := max(1, len(testCases))
		if quota.Hits+cost > learningRunnerQuota {
			err = ErrLearningBusy
			return
		}
		quota.Hits += cost
		if err = learningPut(tx, repository.LearningOperations, quotaID, "", p.User.ID, scope, "", quotaID, nil, quota); err != nil {
			return
		}
		target = learningID("lrun")
		reqID := c.RequestID
		if reqID == "" {
			reqID = target
		}
		run := learningRunRecord{
			ID:            target,
			LessonID:      in.LessonID,
			UserID:        p.User.ID,
			Status:        "running",
			LeaseUntil:    now.Add(s.options.RunnerTimeout + 15*time.Second),
			Language:      in.Language,
			LessonHash:    learningHash(lessonDefinition),
			LessonVersion: lessonDefinition.Version,
			TestResults:   []entity.LearningTestResult{},
			Result: entity.RunResult{
				Status:     "provider_error",
				RequestID:  reqID,
				SourceHash: security.Hash(in.Source),
				Simulated:  s.options.Runner.Simulated(),
			},
		}
		err = learningSaveRun(tx, run, scope)
		out = learningRunClaim{Record: run, TestCases: testCases, Scope: scope, Execute: true}
		return
	})
	if err != nil {
		return entity.LearningRunResponse{}, err
	}
	if !claim.Execute {
		return learningRunResponse(claim.Record), nil
	}
	completedRun := s.executeLearningRun(ctx, in, claim)
	finalizeCtx, finalizeCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finalizeCancel()
	err = s.options.Repository.Transaction(finalizeCtx, func(tx repository.LearningTx) error {
		run, err := learningReadRecord[learningRunRecord](tx, repository.LearningRuns, claim.Record.ID, true)
		if err != nil {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		if run.Status != "running" {
			completedRun = run
			return nil
		}
		if !now.Before(run.LeaseUntil) {
			completedRun.Result.Status = "provider_error"
			completedRun.AllPassed = false
		}
		completedRun.Status = "finished"
		if err = learningSaveRun(tx, completedRun, claim.Scope); err != nil {
			return err
		}
		if completedRun.AllPassed && completedRun.LessonID != "" {
			proofID := learningProofID(completedRun.UserID, completedRun.LessonID, completedRun.Language, completedRun.Result.SourceHash, completedRun.LessonHash)
			_, err := tx.Get(repository.LearningOperations, proofID)
			if errors.Is(err, repository.ErrLearningNotFound) {
				// First passing snapshot wins. Replays and later runs never rewrite it.
				err = learningPut(tx, repository.LearningOperations, proofID, "", completedRun.UserID, claim.Scope, "", proofID, nil, learningOperation{TargetID: completedRun.ID})
			}
			if err != nil {
				return err
			}
		}
		return learningAudit(tx, p, c, "learning.run.finished", run.ID, claim.Scope)
	})
	return learningRunResponse(completedRun), err
}

func (s *LearningService) RequestHint(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningHintInput) (entity.LearningHintResponse, error) {
	if in.LessonID == "" || len(in.LessonID) > 128 || len(in.DraftCode) > 64<<10 {
		return entity.LearningHintResponse{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.hint.request", in.LessonID, in, func(tx repository.LearningTx, replay string) (out entity.LearningHintResponse, target string, scope entity.Scope, err error) {
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		lesson, level, _, e := loadLessonAndCheckAccess(tx, p, in.LessonID, now, false)
		if e != nil {
			err = e
			return
		}
		scope = level.Scope
		quotaID := "hintquota-" + learningHash([]any{p.User.ID, now.Truncate(time.Minute)})
		quota := struct {
			Hits int `json:"hits"`
		}{Hits: 0}
		row, e := tx.Get(repository.LearningOperations, quotaID)
		if e == nil {
			if err = json.Unmarshal(row.Data, &quota); err != nil {
				return
			}
		} else if !errors.Is(e, repository.ErrLearningNotFound) {
			err = e
			return
		}
		if replay == "" {
			if quota.Hits >= learningHintPerMin {
				err = ErrLearningHintQuota
				return
			}
			quota.Hits++
			if err = learningPut(tx, repository.LearningOperations, quotaID, "", p.User.ID, scope, "", quotaID, nil, quota); err != nil {
				return
			}
		}
		hint := strings.TrimSpace(lesson.Hint)
		if hint == "" {
			hint = fmt.Sprintf("Periksa kembali struktur kode %s dan contoh pada materi %s.", strings.ToUpper(lesson.Language), lesson.Title)
		}
		target = lesson.ID
		out = entity.LearningHintResponse{
			LessonID:       lesson.ID,
			Hint:           hint,
			RemainingQuota: max(0, learningHintPerMin-quota.Hits),
			Simulated:      true,
		}
		return
	})
}

func (s *LearningService) Leaderboard(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.LearningLeaderboardRow, error) {
	out := []entity.LearningLeaderboardRow{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	if len(learningScopes(p, learningRead, learningManage, learningMonitor)) == 0 {
		return nil, authorization.ErrForbidden
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningProfiles, repository.LearningFilter{Limit: 1000})
		if err != nil {
			return err
		}
		profiles := make([]entity.LearningProfile, 0, len(rows))
		for _, r := range rows {
			var prof entity.LearningProfile
			if err = json.Unmarshal(r.Data, &prof); err != nil {
				return err
			}
			profiles = append(profiles, prof)
		}
		sort.Slice(profiles, func(i, j int) bool {
			if profiles[i].XP != profiles[j].XP {
				return profiles[i].XP > profiles[j].XP
			}
			if profiles[i].CompletedLessons != profiles[j].CompletedLessons {
				return profiles[i].CompletedLessons > profiles[j].CompletedLessons
			}
			return profiles[i].UserID < profiles[j].UserID
		})
		for idx, prof := range profiles {
			if idx < page.Offset {
				continue
			}
			if len(out) >= page.Limit {
				break
			}
			out = append(out, entity.LearningLeaderboardRow{
				Rank:             idx + 1,
				UserID:           prof.UserID,
				UserName:         prof.UserName,
				XP:               prof.XP,
				Level:            CalculateLearningLevel(prof.XP),
				Streak:           prof.Streak,
				CompletedLessons: prof.CompletedLessons,
				AchievementCount: len(prof.UnlockedAchievements),
			})
		}
		return nil
	})
	return out, err
}

func (s *LearningService) Achievements(ctx context.Context, p *entity.Principal) (entity.LearningAchievementsView, error) {
	out := entity.LearningAchievementsView{Achievements: []entity.LearningAchievement{}}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if len(learningScopes(p, learningRead, learningManage, learningMonitor)) == 0 {
		return out, authorization.ErrForbidden
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		out.Profile, err = loadOrInitProfile(tx, p, now, false)
		if err != nil {
			return err
		}
		unlockedMap := map[string]time.Time{}
		for _, u := range out.Profile.UnlockedAchievements {
			unlockedMap[u.AchievementID] = u.UnlockedAt
		}
		achievements, err := learningAchievements(tx)
		if err != nil {
			return err
		}
		for _, ach := range achievements {
			if at, ok := unlockedMap[ach.ID]; ok {
				copyAt := at
				ach.Unlocked = true
				ach.UnlockedAt = &copyAt
			}
			out.Achievements = append(out.Achievements, ach)
		}
		return nil
	})
	return out, err
}

func countWeeklyBugHuntAttempts(rows []repository.LearningRecord, now time.Time) ([]entity.LearningBugHuntAttempt, map[string]bool, int, error) {
	cutoff := now.Add(-7 * 24 * time.Hour)
	attempts := []entity.LearningBugHuntAttempt{}
	solved := map[string]bool{}
	weekly := 0
	for _, r := range rows {
		var a entity.LearningBugHuntAttempt
		if err := json.Unmarshal(r.Data, &a); err != nil {
			return nil, nil, 0, err
		}
		if a.Correct {
			solved[a.QuestionID] = true
		}
		if !a.PlayedAt.Before(cutoff) {
			weekly++
		}
		attempts = append(attempts, a)
	}
	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].PlayedAt.Equal(attempts[j].PlayedAt) {
			return attempts[i].AttemptID > attempts[j].AttemptID
		}
		return attempts[i].PlayedAt.After(attempts[j].PlayedAt)
	})
	return attempts, solved, weekly, nil
}

func (s *LearningService) BugHuntList(ctx context.Context, p *entity.Principal) (entity.LearningBugHuntView, error) {
	out := entity.LearningBugHuntView{Questions: []entity.LearningBugHuntQuestion{}, Attempts: []entity.LearningBugHuntAttempt{}, WeeklyLimit: bugHuntWeeklyMax}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	scopes := learningScopes(p, learningRead, learningManage, learningMonitor)
	if len(scopes) == 0 {
		return out, authorization.ErrForbidden
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		out.Profile, err = loadOrInitProfile(tx, p, now, false)
		if err != nil {
			return err
		}
		historyRows, err := tx.List(repository.LearningGames, repository.LearningFilter{OwnerID: p.User.ID, Limit: 200})
		if err != nil {
			return err
		}
		attempts, solved, weekly, err := countWeeklyBugHuntAttempts(historyRows, now)
		if err != nil {
			return err
		}
		out.Attempts = attempts
		out.WeeklyUsed = weekly
		qRows, err := tx.List(repository.LearningBugHunt, repository.LearningFilter{Status: "active", Scopes: scopes, Overlap: true, Limit: 100})
		if err != nil {
			return err
		}
		for _, r := range qRows {
			var q entity.LearningBugHuntQuestion
			if err = json.Unmarshal(r.Data, &q); err != nil {
				return err
			}
			q.BugLine = nil
			q.Explanation = ""
			q.Version = 0
			q.Solved = solved[q.ID]
			out.Questions = append(out.Questions, q)
		}
		return nil
	})
	return out, err
}

func (s *LearningService) SubmitBugHunt(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningBugHuntAttemptInput) (entity.LearningBugHuntSubmitView, error) {
	if in.QuestionID == "" || len(in.QuestionID) > 128 || in.SelectedLine < 1 || in.SelectedLine > 500 || len(strings.TrimSpace(in.Explanation)) < 5 || len(in.Explanation) > 2000 {
		return entity.LearningBugHuntSubmitView{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.bughunt.attempt", in.QuestionID, in, func(tx repository.LearningTx, replay string) (out entity.LearningBugHuntSubmitView, target string, scope entity.Scope, err error) {
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		q, e := learningReadRecord[entity.LearningBugHuntQuestion](tx, repository.LearningBugHunt, in.QuestionID, false)
		if e != nil {
			err = e
			return
		}
		scope = q.Scope
		if !q.Active || (!isLearningStaff(p, scope) && !practicumAllowed(p, learningRead, scope)) {
			err = authorization.ErrNotFound
			return
		}
		profile, e := loadOrInitProfile(tx, p, now, true)
		if e != nil {
			err = e
			return
		}
		if replay != "" {
			attempt, e := learningReadRecord[entity.LearningBugHuntAttempt](tx, repository.LearningGames, replay, false)
			if e != nil {
				err = e
				return
			}
			out = entity.LearningBugHuntSubmitView{
				Attempt:       attempt,
				Profile:       profile,
				NewlyUnlocked: []entity.LearningAchievement{},
			}
			return out, replay, scope, nil
		}
		historyRows, e := tx.List(repository.LearningGames, repository.LearningFilter{OwnerID: p.User.ID, Limit: 500})
		if e != nil {
			err = e
			return
		}
		_, solved, weekly, e := countWeeklyBugHuntAttempts(historyRows, now)
		if e != nil {
			err = e
			return
		}
		if weekly >= bugHuntWeeklyMax {
			err = ErrLearningWeeklyLimit
			return
		}
		correct := q.BugLine != nil && in.SelectedLine == *q.BugLine
		xpEarned := 0
		feedback := "Baris yang dipilih belum tepat. Periksa kembali alur dan operator pada kode."
		if correct {
			if !solved[q.ID] {
				xpEarned = min(300, q.XPReward)
			}
			feedback = "Tepat! " + q.Explanation
		}
		target = learningID("bh-attempt")
		attempt := entity.LearningBugHuntAttempt{
			AttemptID:    target,
			QuestionID:   q.ID,
			UserID:       p.User.ID,
			Language:     q.Language,
			SelectedLine: in.SelectedLine,
			Explanation:  strings.TrimSpace(in.Explanation),
			Correct:      correct,
			XPEarned:     xpEarned,
			Feedback:     feedback,
			PlayedAt:     now,
		}
		if err = learningPut(tx, repository.LearningGames, target, q.ID, p.User.ID, scope, "completed", "", nil, attempt); err != nil {
			return
		}
		profile.GamesPlayed++
		if correct {
			profile.PerfectGames++
		}
		profile.XP += xpEarned
		profile.Streak, profile.LastActiveDate = updateLearningStreak(profile.LastActiveDate, profile.Streak, now)
		completedLevels, e := completedLevelsForUser(tx, p.User.ID)
		if e != nil {
			err = e
			return
		}
		newlyUnlocked, e := evaluateAchievements(tx, &profile, completedLevels, now)
		if e != nil {
			err = e
			return
		}
		profile, err = saveProfile(tx, profile, scope, now)
		if err != nil {
			return
		}
		out = entity.LearningBugHuntSubmitView{
			Attempt:       attempt,
			Profile:       profile,
			NewlyUnlocked: newlyUnlocked,
		}
		return
	})
}

func validLearningScope(s entity.Scope) bool {
	if s.Domain != "elearning" || s.Shift < 0 || s.Shift > 2 {
		return false
	}
	for _, v := range []string{s.Institution, s.Division, s.ClassID, s.SessionID} {
		if len(v) > 128 || strings.ContainsAny(v, "\x00*\r\n") {
			return false
		}
	}
	return true
}

func validLearningLevelInput(in LearningLevelInput) bool {
	return validPracticumTitle(in.Title) && len(in.Description) <= 2000 && in.SortOrder >= 1 && in.SortOrder <= 1000 && in.RequiredXP >= 0 && in.RequiredXP <= 1000000 && (in.AccessMode == "auto" || in.AccessMode == "unlocked" || in.AccessMode == "locked") && validLearningScope(in.Scope)
}

func validLearningLessonInput(in LearningLessonInput) bool {
	if !validPracticumTitle(in.Title) || in.SortOrder < 1 || in.SortOrder > 1000 || (in.Language != "c" && in.Language != "python") || len(strings.TrimSpace(in.Explanation)) < 1 || len(in.Explanation) > 32<<10 || len(in.CodeExample) > 32<<10 || len(in.StarterCode) > 32<<10 || len(in.Solution) > 32<<10 || len(in.Hint) > 4000 || in.XPReward < 10 || in.XPReward > 500 || len(in.TestCases) > 20 {
		return false
	}
	if in.Quiz != nil {
		if len(strings.TrimSpace(in.Quiz.Question)) < 1 || len(in.Quiz.Question) > 2000 || len(in.Quiz.Options) < 2 || len(in.Quiz.Options) > 6 || in.Quiz.CorrectOption == nil || *in.Quiz.CorrectOption < 0 || *in.Quiz.CorrectOption >= len(in.Quiz.Options) {
			return false
		}
		for _, opt := range in.Quiz.Options {
			if len(strings.TrimSpace(opt)) < 1 || len(opt) > 500 {
				return false
			}
		}
	}
	for _, tc := range in.TestCases {
		if len(strings.TrimSpace(tc.Description)) < 1 || len(tc.Description) > 500 || len(tc.Input) > 4000 || len(tc.ExpectedOutput) > 4000 {
			return false
		}
	}
	return true
}

func validBugHuntInput(in LearningBugHuntInput) bool {
	return validPracticumTitle(in.Title) && (in.Language == "c" || in.Language == "python") && (in.Difficulty == "easy" || in.Difficulty == "medium" || in.Difficulty == "hard") && len(strings.TrimSpace(in.BuggyCode)) > 0 && len(in.BuggyCode) <= 32<<10 && in.BugLine >= 1 && in.BugLine <= 500 && len(strings.TrimSpace(in.Explanation)) >= 5 && len(in.Explanation) <= 4000 && in.XPReward >= 10 && in.XPReward <= 300 && validLearningScope(in.Scope)
}

func (s *LearningService) AdminCurriculum(ctx context.Context, p *entity.Principal) ([]entity.LearningLevel, error) {
	out := []entity.LearningLevel{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	scopes := learningScopes(p, learningManage, learningMonitor)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		levelRows, err := tx.List(repository.LearningLevels, repository.LearningFilter{Scopes: scopes, Limit: 200})
		if err != nil {
			return err
		}
		moduleRows, err := tx.List(repository.LearningModules, repository.LearningFilter{Scopes: scopes, Limit: 500})
		if err != nil {
			return err
		}
		lessonRows, err := tx.List(repository.LearningLessons, repository.LearningFilter{Scopes: scopes, Limit: 1000})
		if err != nil {
			return err
		}
		lessonsByModule := map[string][]entity.LearningLesson{}
		for _, r := range lessonRows {
			var l entity.LearningLesson
			if err = json.Unmarshal(r.Data, &l); err != nil {
				return err
			}
			canManage := practicumAllowed(p, learningManage, l.Scope)
			lessonsByModule[l.ModuleID] = append(lessonsByModule[l.ModuleID], publicLesson(l, canManage))
		}
		for k := range lessonsByModule {
			sort.Slice(lessonsByModule[k], func(i, j int) bool {
				return lessonsByModule[k][i].SortOrder < lessonsByModule[k][j].SortOrder
			})
		}
		modulesByLevel := map[string][]entity.LearningModule{}
		for _, r := range moduleRows {
			var m entity.LearningModule
			if err = json.Unmarshal(r.Data, &m); err != nil {
				return err
			}
			m.Lessons = lessonsByModule[m.ID]
			if m.Lessons == nil {
				m.Lessons = []entity.LearningLesson{}
			}
			m.TotalLessons = len(m.Lessons)
			modulesByLevel[m.LevelID] = append(modulesByLevel[m.LevelID], m)
		}
		for k := range modulesByLevel {
			sort.Slice(modulesByLevel[k], func(i, j int) bool {
				return modulesByLevel[k][i].SortOrder < modulesByLevel[k][j].SortOrder
			})
		}
		for _, r := range levelRows {
			var lvl entity.LearningLevel
			if err = json.Unmarshal(r.Data, &lvl); err != nil {
				return err
			}
			lvl.Unlocked = true
			lvl.Modules = modulesByLevel[lvl.ID]
			if lvl.Modules == nil {
				lvl.Modules = []entity.LearningModule{}
			}
			for _, m := range lvl.Modules {
				lvl.TotalLessons += m.TotalLessons
			}
			out = append(out, lvl)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
		return nil
	})
	return out, err
}

func (s *LearningService) CreateLevel(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningLevelInput) (entity.LearningLevel, error) {
	if !validLearningLevelInput(in) {
		return entity.LearningLevel{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.level.create", "", in, func(tx repository.LearningTx, replay string) (out entity.LearningLevel, id string, scope entity.Scope, err error) {
		scope = in.Scope
		if err = learningRequire(p, learningManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, replay, false)
			return out, replay, scope, err
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		id = learningID("level")
		out = entity.LearningLevel{
			ID:          id,
			Title:       in.Title,
			Description: in.Description,
			SortOrder:   in.SortOrder,
			AccessMode:  in.AccessMode,
			RequiredXP:  in.RequiredXP,
			Unlocked:    in.AccessMode != "locked",
			Scope:       scope,
			Version:     1,
			CreatedAt:   now,
			UpdatedAt:   now,
			Modules:     []entity.LearningModule{},
		}
		err = learningPut(tx, repository.LearningLevels, id, "", 0, scope, "", "", nil, out)
		return
	})
}

func (s *LearningService) UpdateLevel(ctx context.Context, p *entity.Principal, c LearningCommand, id string, in LearningLevelUpdate) (entity.LearningLevel, error) {
	if !validPracticumTitle(in.Title) || len(in.Description) > 2000 || in.SortOrder < 1 || in.SortOrder > 1000 || in.RequiredXP < 0 || in.RequiredXP > 1000000 || (in.AccessMode != "auto" && in.AccessMode != "unlocked" && in.AccessMode != "locked") || in.Version < 1 {
		return entity.LearningLevel{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.level.update", id, in, func(tx repository.LearningTx, replay string) (out entity.LearningLevel, target string, scope entity.Scope, err error) {
		out, err = learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = learningRequire(p, learningManage, scope); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		out.Title = in.Title
		out.Description = in.Description
		out.SortOrder = in.SortOrder
		out.AccessMode = in.AccessMode
		out.RequiredXP = in.RequiredXP
		out.Unlocked = in.AccessMode != "locked"
		out.Version++
		out.UpdatedAt, err = tx.Now()
		if err == nil {
			err = learningPut(tx, repository.LearningLevels, id, "", 0, scope, "", "", nil, out)
		}
		return
	})
}

func (s *LearningService) CreateModule(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningModuleInput) (entity.LearningModule, error) {
	if in.LevelID == "" || !validPracticumTitle(in.Title) || len(in.Description) > 2000 || in.SortOrder < 1 || in.SortOrder > 1000 {
		return entity.LearningModule{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.module.create", in.LevelID, in, func(tx repository.LearningTx, replay string) (out entity.LearningModule, id string, scope entity.Scope, err error) {
		lvl, e := learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, in.LevelID, true)
		if e != nil {
			err = e
			return
		}
		scope = lvl.Scope
		if err = learningRequire(p, learningManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningModule](tx, repository.LearningModules, replay, false)
			return out, replay, scope, err
		}
		id = learningID("module")
		out = entity.LearningModule{
			ID:          id,
			LevelID:     in.LevelID,
			Title:       in.Title,
			Description: in.Description,
			SortOrder:   in.SortOrder,
			Scope:       scope,
			Version:     1,
			Lessons:     []entity.LearningLesson{},
		}
		err = learningPut(tx, repository.LearningModules, id, in.LevelID, 0, scope, "", "", nil, out)
		return
	})
}

func (s *LearningService) UpdateModule(ctx context.Context, p *entity.Principal, c LearningCommand, id string, in LearningModuleUpdate) (entity.LearningModule, error) {
	if !validPracticumTitle(in.Title) || len(in.Description) > 2000 || in.SortOrder < 1 || in.SortOrder > 1000 || in.Version < 1 {
		return entity.LearningModule{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.module.update", id, in, func(tx repository.LearningTx, replay string) (out entity.LearningModule, target string, scope entity.Scope, err error) {
		out, err = learningReadRecord[entity.LearningModule](tx, repository.LearningModules, id, true)
		if err != nil {
			return
		}
		lvl, e := learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, out.LevelID, false)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = lvl.Scope
		if err = learningRequire(p, learningManage, scope); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		out.Title = in.Title
		out.Description = in.Description
		out.SortOrder = in.SortOrder
		out.Scope = scope
		out.Version++
		err = learningPut(tx, repository.LearningModules, id, out.LevelID, 0, scope, "", "", nil, out)
		return
	})
}

func (s *LearningService) SaveLesson(ctx context.Context, p *entity.Principal, c LearningCommand, id string, in LearningLessonUpdate) (entity.LearningLesson, error) {
	if !validLearningLessonInput(in.LearningLessonInput) || (id != "" && in.Version < 1) {
		return entity.LearningLesson{}, ErrValidation
	}
	action := "learning.lesson.create"
	if id != "" {
		action = "learning.lesson.update"
	}
	return learningMutate(s, ctx, p, c, action, id, in, func(tx repository.LearningTx, replay string) (out entity.LearningLesson, target string, scope entity.Scope, err error) {
		moduleID := in.ModuleID
		var old entity.LearningLesson
		if id != "" {
			old, err = learningReadRecord[entity.LearningLesson](tx, repository.LearningLessons, id, true)
			if err != nil {
				return
			}
			if moduleID != "" && moduleID != old.ModuleID {
				err = ErrValidation
				return
			}
			moduleID = old.ModuleID
		}
		mod, e := learningReadRecord[entity.LearningModule](tx, repository.LearningModules, moduleID, false)
		if e != nil {
			err = e
			return
		}
		lvl, e := learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, mod.LevelID, false)
		if e != nil {
			err = e
			return
		}
		scope = lvl.Scope
		if err = learningRequire(p, learningManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningLesson](tx, repository.LearningLessons, replay, false)
			return out, replay, scope, err
		}
		target = id
		version := int64(1)
		if id == "" {
			target = learningID("lesson")
		} else {
			if old.Version != in.Version {
				err = repository.ErrConflict
				return
			}
			version = old.Version + 1
		}
		testCases := in.TestCases
		if testCases == nil {
			testCases = []entity.LearningTestCase{}
		}
		out = entity.LearningLesson{
			ID:          target,
			ModuleID:    moduleID,
			LevelID:     lvl.ID,
			Title:       in.Title,
			SortOrder:   in.SortOrder,
			Language:    in.Language,
			Explanation: in.Explanation,
			CodeExample: in.CodeExample,
			StarterCode: in.StarterCode,
			Solution:    in.Solution,
			Hint:        in.Hint,
			XPReward:    in.XPReward,
			Quiz:        in.Quiz,
			TestCases:   testCases,
			Scope:       scope,
			Version:     version,
		}
		err = learningPut(tx, repository.LearningLessons, target, moduleID, 0, scope, "", "", nil, out)
		return
	})
}

func (s *LearningService) AdminBugHuntList(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.LearningBugHuntQuestion, error) {
	out := []entity.LearningBugHuntQuestion{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	scopes := learningScopes(p, learningManage)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningBugHunt, repository.LearningFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var q entity.LearningBugHuntQuestion
			if err = json.Unmarshal(r.Data, &q); err != nil {
				return err
			}
			out = append(out, q)
		}
		return nil
	})
	return out, err
}

func (s *LearningService) SaveBugHunt(ctx context.Context, p *entity.Principal, c LearningCommand, id string, in LearningBugHuntUpdate) (entity.LearningBugHuntQuestion, error) {
	if !validBugHuntInput(in.LearningBugHuntInput) || (id != "" && in.Version < 1) {
		return entity.LearningBugHuntQuestion{}, ErrValidation
	}
	action := "learning.bughunt.create"
	if id != "" {
		action = "learning.bughunt.update"
	}
	return learningMutate(s, ctx, p, c, action, id, in, func(tx repository.LearningTx, replay string) (out entity.LearningBugHuntQuestion, target string, scope entity.Scope, err error) {
		scope = in.Scope
		var old entity.LearningBugHuntQuestion
		if id != "" {
			old, err = learningReadRecord[entity.LearningBugHuntQuestion](tx, repository.LearningBugHunt, id, true)
			if err != nil {
				return
			}
			scope = old.Scope
		}
		if err = learningRequire(p, learningManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningBugHuntQuestion](tx, repository.LearningBugHunt, replay, false)
			return out, replay, scope, err
		}
		target = id
		version := int64(1)
		if id == "" {
			target = learningID("bughunt")
		} else {
			if old.Version != in.Version {
				err = repository.ErrConflict
				return
			}
			version = old.Version + 1
		}
		line := in.BugLine
		status := "inactive"
		if in.Active {
			status = "active"
		}
		out = entity.LearningBugHuntQuestion{
			ID:          target,
			Language:    in.Language,
			Difficulty:  in.Difficulty,
			Title:       in.Title,
			BuggyCode:   in.BuggyCode,
			BugLine:     &line,
			Explanation: in.Explanation,
			XPReward:    in.XPReward,
			Active:      in.Active,
			Scope:       scope,
			Version:     version,
		}
		err = learningPut(tx, repository.LearningBugHunt, target, "", 0, scope, status, "", nil, out)
		return
	})
}

func (s *LearningService) Monitoring(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.LearningMonitoringRow, error) {
	out := []entity.LearningMonitoringRow{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	scopes := learningScopes(p, learningManage, learningMonitor)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningProfiles, repository.LearningFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		progressRows, err := tx.List(repository.LearningProgress, repository.LearningFilter{Status: "in_progress", Scopes: scopes, Limit: 1000})
		if err != nil {
			return err
		}
		inProgressByUser := map[int64]int{}
		for _, r := range progressRows {
			inProgressByUser[r.OwnerID]++
		}
		for _, r := range rows {
			var prof entity.LearningProfile
			if err = json.Unmarshal(r.Data, &prof); err != nil {
				return err
			}
			out = append(out, entity.LearningMonitoringRow{
				UserID:            prof.UserID,
				UserName:          prof.UserName,
				XP:                prof.XP,
				Level:             CalculateLearningLevel(prof.XP),
				Streak:            prof.Streak,
				CompletedLessons:  prof.CompletedLessons,
				InProgressLessons: inProgressByUser[prof.UserID],
				GamesPlayed:       prof.GamesPlayed,
				LastActiveDate:    prof.LastActiveDate,
				UpdatedAt:         prof.UpdatedAt,
			})
		}
		return nil
	})
	return out, err
}
