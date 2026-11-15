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
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

const (
	prestigeSourceAdmin    = "admin_grant"
	prestigeSourceWeekly   = "weekly_leaderboard"
	learningStandingsLimit = 100
	learningXPRange        = 1000000
	hintMaxChars           = 800
)

// WeekStartWIB returns the Monday 00:00 Asia/Jakarta instant for t as UTC.
// WIB is a fixed +07:00 offset (no DST), so the boundary is stable all year.
func WeekStartWIB(t time.Time) time.Time {
	wib := t.UTC().Add(7 * time.Hour)
	days := (int(wib.Weekday()) + 6) % 7
	midnight := time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
	return midnight.Add(-7 * time.Hour)
}

func WeekLabelWIB(t time.Time) string {
	return WeekStartWIB(t).UTC().Add(7 * time.Hour).Format("2006-01-02")
}

func learningWeekID(weekStart time.Time) string {
	return "week-" + WeekLabelWIB(weekStart)
}

// learningWeekRecordID is stable across snapshot regenerations for the same week.
func learningWeekRecordID(weekStart time.Time) string {
	return "standings-" + WeekLabelWIB(weekStart)
}

func grantPrestige(tx repository.LearningTx, p *entity.Principal, userID int64, scope entity.Scope, now time.Time, source, reason string, amount int, meta map[string]any) (entity.LearningPrestigeEntry, error) {
	if meta == nil {
		meta = map[string]any{}
	}
	entry := entity.LearningPrestigeEntry{
		ID:        learningID("prestige"),
		UserID:    userID,
		Source:    source,
		Amount:    amount,
		Reason:    reason,
		Meta:      meta,
		CreatedAt: now,
	}
	if err := learningPut(tx, repository.LearningPrestige, entry.ID, learningProfileID(userID), userID, scope, "active", entry.ID, nil, entry); err != nil {
		return entity.LearningPrestigeEntry{}, err
	}
	profile, err := learningReadRecord[entity.LearningProfile](tx, repository.LearningProfiles, learningProfileID(userID), true)
	if errors.Is(err, repository.ErrLearningNotFound) {
		profile = entity.LearningProfile{UserID: userID, UserName: "", XP: 0, Level: 1, UnlockedAchievements: []entity.LearningUnlockedAchievement{}}
		if p != nil && p.User.ID == userID {
			profile.UserName = p.User.Name
		}
	} else if err != nil {
		return entity.LearningPrestigeEntry{}, err
	}
	profile.Prestige += amount
	profile, err = saveProfile(tx, profile, scope, now)
	if err != nil {
		return entity.LearningPrestigeEntry{}, err
	}
	return entry, nil
}

func (s *LearningService) Prestige(ctx context.Context, p *entity.Principal, userID int64) ([]entity.LearningPrestigeEntry, error) {
	out := []entity.LearningPrestigeEntry{}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if len(learningScopes(p, learningRead, learningManage, learningMonitor)) == 0 {
		return out, authorization.ErrForbidden
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningPrestige, repository.LearningFilter{OwnerID: userID, Limit: 200})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var entry entity.LearningPrestigeEntry
			if err = json.Unmarshal(r.Data, &entry); err != nil {
				return err
			}
			out = append(out, entry)
		}
		sort.Slice(out, func(i, j int) bool {
			if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
				return out[i].CreatedAt.After(out[j].CreatedAt)
			}
			return out[i].ID > out[j].ID
		})
		return nil
	})
	return out, err
}

type LearningPrestigeGrantInput struct {
	UserID int64  `json:"user_id"`
	Amount int    `json:"amount"`
	Reason string `json:"reason"`
}

func (s *LearningService) GrantPrestige(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningPrestigeGrantInput) (entity.LearningPrestigeEntry, error) {
	if in.UserID < 1 || in.Amount <= 0 || in.Amount > 100000 || len(in.Reason) > 500 {
		return entity.LearningPrestigeEntry{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.prestige.grant", fmt.Sprintf("user-%d", in.UserID), in, func(tx repository.LearningTx, replay string) (out entity.LearningPrestigeEntry, target string, scope entity.Scope, err error) {
		scope = entity.Scope{Domain: "elearning"}
		if err = learningRequire(p, learningXPManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningPrestigeEntry](tx, repository.LearningPrestige, replay, false)
			return out, replay, scope, err
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		target = learningID("prestige")
		out, err = grantPrestige(tx, p, in.UserID, scope, now, prestigeSourceAdmin, strings.TrimSpace(in.Reason), in.Amount, nil)
		return out, target, scope, err
	})
}

type LearningXPSetInput struct {
	UserID int64  `json:"user_id"`
	XP     int    `json:"xp"`
	Reason string `json:"reason"`
}

func (s *LearningService) SetXP(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningXPSetInput) (entity.LearningProfile, error) {
	return s.setXP(ctx, p, c, in, "learning.xp.set")
}

func (s *LearningService) setXP(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningXPSetInput, action string) (entity.LearningProfile, error) {
	if in.UserID < 1 || in.XP < 0 || in.XP > learningXPRange || len(in.Reason) > 500 {
		return entity.LearningProfile{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, action, fmt.Sprintf("user-%d", in.UserID), in, func(tx repository.LearningTx, replay string) (out entity.LearningProfile, target string, scope entity.Scope, err error) {
		scope = entity.Scope{Domain: "elearning"}
		if err = learningRequire(p, learningXPManage, scope); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		profile, e := learningReadRecord[entity.LearningProfile](tx, repository.LearningProfiles, learningProfileID(in.UserID), true)
		if errors.Is(e, repository.ErrLearningNotFound) {
			profile = entity.LearningProfile{UserID: in.UserID, UnlockedAchievements: []entity.LearningUnlockedAchievement{}}
		} else if e != nil {
			err = e
			return
		}
		target = learningProfileID(in.UserID)
		if replay != "" {
			out, err = learningReadRecord[entity.LearningProfile](tx, repository.LearningProfiles, target, false)
			return out, target, scope, err
		}
		profile.XP = in.XP
		out, err = saveProfile(tx, profile, scope, now)
		return out, target, scope, err
	})
}

func (s *LearningService) ResetXP(ctx context.Context, p *entity.Principal, c LearningCommand, userID int64, reason string) (entity.LearningProfile, error) {
	if userID < 1 || len(reason) > 500 {
		return entity.LearningProfile{}, ErrValidation
	}
	return s.setXP(ctx, p, c, LearningXPSetInput{UserID: userID, XP: 0, Reason: reason}, "learning.xp.reset")
}

type LearningAchievementInput struct {
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	RequirementType  string       `json:"requirement_type"`
	RequirementValue string       `json:"requirement_value"`
	XPReward         int          `json:"xp_reward"`
	Active           bool         `json:"active"`
	Scope            entity.Scope `json:"scope"`
}

type LearningAchievementUpdate struct {
	LearningAchievementInput
	Version int64 `json:"version"`
}

func validAchievementInput(in LearningAchievementInput) bool {
	if !validPracticumTitle(in.Title) || len(in.Description) > 2000 || in.XPReward < 0 || in.XPReward > 10000 {
		return false
	}
	switch in.RequirementType {
	case "manual", "xp", "lesson_count", "streak", "game_score", "level_completed":
	default:
		return false
	}
	if len(in.RequirementValue) > 128 {
		return false
	}
	return validLearningScope(in.Scope)
}

func (s *LearningService) AdminAchievements(ctx context.Context, p *entity.Principal) ([]entity.LearningAchievementCatalogItem, error) {
	out := []entity.LearningAchievementCatalogItem{}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	scopes := learningScopes(p, learningManage, learningMonitor)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningAchievements, repository.LearningFilter{Scopes: scopes, Overlap: true, Limit: 500})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var item entity.LearningAchievementCatalogItem
			if err = json.Unmarshal(r.Data, &item); err != nil {
				return err
			}
			out = append(out, item)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
		return nil
	})
	return out, err
}

func (s *LearningService) SaveAchievement(ctx context.Context, p *entity.Principal, c LearningCommand, id string, in LearningAchievementUpdate) (entity.LearningAchievementCatalogItem, error) {
	if !validAchievementInput(in.LearningAchievementInput) || (id != "" && in.Version < 1) {
		return entity.LearningAchievementCatalogItem{}, ErrValidation
	}
	action := "learning.achievement.create"
	if id != "" {
		action = "learning.achievement.update"
	}
	return learningMutate(s, ctx, p, c, action, id, in, func(tx repository.LearningTx, replay string) (out entity.LearningAchievementCatalogItem, target string, scope entity.Scope, err error) {
		scope = in.Scope
		if err = learningRequire(p, learningManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = learningReadRecord[entity.LearningAchievementCatalogItem](tx, repository.LearningAchievements, replay, false)
			return out, replay, scope, err
		}
		target = id
		version := int64(1)
		if id == "" {
			target = learningID("ach")
		} else {
			old, e := learningReadRecord[entity.LearningAchievementCatalogItem](tx, repository.LearningAchievements, id, true)
			if e != nil {
				err = e
				return
			}
			if old.Version != in.Version {
				err = repository.ErrConflict
				return
			}
			version = old.Version + 1
			scope = old.Scope
		}
		out = entity.LearningAchievementCatalogItem{
			ID:               target,
			Title:            in.Title,
			Description:      in.Description,
			RequirementType:  in.RequirementType,
			RequirementValue: in.RequirementValue,
			XPReward:         in.XPReward,
			Active:           in.Active,
			Scope:            scope,
			Version:          version,
		}
		status := "inactive"
		if in.Active {
			status = "active"
		}
		err = learningPut(tx, repository.LearningAchievements, target, "", 0, scope, status, "", nil, out)
		return
	})
}

// WeeklyStandings reads the stored snapshot for a week; the current week may be nil.
func (s *LearningService) WeeklyStandings(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.LearningWeeklySnapshot, error) {
	out := []entity.LearningWeeklySnapshot{}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !page.valid() {
		return out, ErrValidation
	}
	if len(learningScopes(p, learningRead, learningManage, learningMonitor)) == 0 {
		return out, authorization.ErrForbidden
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		rows, err := tx.List(repository.LearningStandings, repository.LearningFilter{Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var snap entity.LearningWeeklySnapshot
			if err = json.Unmarshal(r.Data, &snap); err != nil {
				return err
			}
			out = append(out, snap)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].WeekStart > out[j].WeekStart })
		return nil
	})
	return out, err
}

func learningRankEntries(profiles []entity.LearningProfile, limit int) []entity.LearningLeaderboardRow {
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].XP != profiles[j].XP {
			return profiles[i].XP > profiles[j].XP
		}
		if profiles[i].CompletedLessons != profiles[j].CompletedLessons {
			return profiles[i].CompletedLessons > profiles[j].CompletedLessons
		}
		return profiles[i].UserID < profiles[j].UserID
	})
	rows := []entity.LearningLeaderboardRow{}
	for idx, prof := range profiles {
		if len(rows) >= limit {
			break
		}
		rows = append(rows, entity.LearningLeaderboardRow{
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
	return rows
}

// ReconcileWeeklyStandings snapshots the current WIB week once and freezes all
// earlier weeks. Safe to call from the cron endpoint as often as needed: the
// snapshot ID is derived from the week label, so replays converge on the same
// row and history rows are never rewritten after their week closes.
func (s *LearningService) ReconcileWeeklyStandings(ctx context.Context, c LearningCommand) (map[string]any, error) {
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
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
		current := WeekStartWIB(now)
		currentID := learningWeekRecordID(current)
		snap := entity.LearningWeeklySnapshot{
			WeekStart:   WeekLabelWIB(current),
			GeneratedAt: now,
			Rows:        learningRankEntries(profiles, learningStandingsLimit),
		}
		if err = learningPut(tx, repository.LearningStandings, currentID, "", 0, entity.Scope{Domain: "elearning"}, "active", currentID, nil, snap); err != nil {
			return err
		}
		return learningAudit(tx, nil, c, "learning.standings.snapshot", currentID, entity.Scope{Domain: "elearning"})
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

type LearningHintRequest struct {
	LessonID  string `json:"lesson_id"`
	Question  string `json:"question"`
	DraftCode string `json:"draft_code"`
}

type LearningAIGeneratedHint struct {
	LessonID       string `json:"lesson_id"`
	Hint           string `json:"hint"`
	RemainingQuota int    `json:"remaining_quota"`
	Simulated      bool   `json:"simulated"`
}

const learningHintSystemPrompt = `Kamu asisten belajar untuk mahasiswa kelas karyawan pada platform E-learning Algohub.
Aturan wajib:
1. Berikan HANYA petunjuk singkat (maksimal 3 kalimat, bahasa Indonesia) yang mengarahkan cara berpikir.
2. JANGAN pernah menuliskan jawaban final, potongan kode lengkap yang langsung menyelesaikan soal, atau kunci kuis.
3. JANGAN menyingkap opsi kuis yang benar, hasil testcase, atau solusi privat.
4. Jika pertanyaan di luar konteks pelajaran, jawab bahwa kamu hanya bisa membantu dalam konteks pelajaran ini.
5. Balas dengan teks biasa, tanpa markdown, tanpa prefiks.`

func (s *LearningService) RequestAIHint(ctx context.Context, p *entity.Principal, c LearningCommand, in LearningHintRequest) (entity.LearningHintResponse, error) {
	if len(in.LessonID) > 128 || len(in.Question) > 4000 || len(in.DraftCode) > 64<<10 {
		return entity.LearningHintResponse{}, ErrValidation
	}
	if in.LessonID == "" && strings.TrimSpace(in.Question) == "" {
		return entity.LearningHintResponse{}, ErrValidation
	}
	if s.options.HintGenerator == nil {
		return entity.LearningHintResponse{}, ErrLearningHintGenerator
	}
	return learningMutate(s, ctx, p, c, "learning.hint.ai", in.LessonID, in, func(tx repository.LearningTx, replay string) (out entity.LearningHintResponse, target string, scope entity.Scope, err error) {
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		scope = entity.Scope{Domain: "elearning"}
		lessonContext := ""
		lessonTitle := ""
		if in.LessonID != "" {
			lesson, level, _, e := loadLessonAndCheckAccess(tx, p, in.LessonID, now, false)
			if e != nil {
				err = e
				return
			}
			scope = level.Scope
			lessonContext = strings.TrimSpace(lesson.Explanation)
			lessonTitle = lesson.Title
		}
		if !isLearningStaff(p, scope) && !practicumAllowed(p, learningRead, scope) {
			err = authorization.ErrNotFound
			return
		}
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
		if replay != "" {
			// Replay: regenerate is not allowed for a mutating endpoint; return an
			// honest quota-only response without re-calling the provider.
			out = entity.LearningHintResponse{LessonID: in.LessonID, RemainingQuota: max(0, learningHintPerMin-quota.Hits)}
			target = in.LessonID
			return
		}
		var b strings.Builder
		if lessonTitle != "" {
			b.WriteString("Pelajaran: " + lessonTitle + "\n")
		}
		if lessonContext != "" {
			b.WriteString("Ringkasan materi: " + lessonContext + "\n")
		}
		if strings.TrimSpace(in.Question) != "" {
			b.WriteString("Pertanyaan mahasiswa: " + strings.TrimSpace(in.Question) + "\n")
		}
		if strings.TrimSpace(in.DraftCode) != "" {
			b.WriteString("Kode draf mahasiswa:\n" + in.DraftCode)
		}
		genCtx, cancel := context.WithTimeout(ctx, s.options.HintTimeout)
		defer cancel()
		generated, genErr := s.options.HintGenerator.Complete(genCtx, learningHintSystemPrompt, b.String())
		if genErr != nil {
			err = ErrLearningHintFailed
			return
		}
		hint := strings.TrimSpace(generated)
		if hint == "" {
			err = ErrLearningHintFailed
			return
		}
		if len(hint) > hintMaxChars {
			hint = hint[:hintMaxChars]
		}
		target = in.LessonID
		out = entity.LearningHintResponse{
			LessonID:       in.LessonID,
			Hint:           hint,
			RemainingQuota: max(0, learningHintPerMin-quota.Hits),
			Simulated:      false,
		}
		return
	})
}
