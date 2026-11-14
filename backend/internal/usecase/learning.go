package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

var (
	ErrLearningLocked          = errors.New("learning level is locked")
	ErrLearningQuizIncorrect   = errors.New("lesson quiz answer is incorrect")
	ErrLearningWeeklyLimit     = errors.New("bug hunt weekly limit reached")
	ErrLearningHintQuota       = errors.New("learning hint rate limit reached")
	ErrLearningBusy            = errors.New("learning execution concurrency limit")
	ErrLearningUnavailable     = errors.New("learning runner unavailable")
	ErrLearningCodeNotVerified = errors.New("current lesson code has no passing server verification")
)

const (
	learningRead        = "learning.content.read"
	learningManage      = "learning.content.manage"
	learningMonitor     = "learning.monitor"
	bugHuntWeeklyMax    = 3
	learningHintPerMin  = 10
	learningRunnerQuota = 30
)

type LearningOptions struct {
	Repository         repository.LearningRepository
	IdentityRepository repository.IdentityRepository
	Runner             PracticumRunner
	RunnerGlobalLimit  int
	RunnerUserLimit    int
	RunnerTimeout      time.Duration
	// Only isolated local fixture applications/tests may trust simulated runs.
	AllowSimulatedVerification bool
}

type LearningService struct{ options LearningOptions }

func NewLearningService(o LearningOptions) *LearningService {
	if o.RunnerGlobalLimit <= 0 {
		o.RunnerGlobalLimit = 8
	}
	if o.RunnerUserLimit <= 0 {
		o.RunnerUserLimit = 2
	}
	if o.RunnerGlobalLimit > 100 {
		o.RunnerGlobalLimit = 100
	}
	if o.RunnerUserLimit > 10 {
		o.RunnerUserLimit = 10
	}
	if o.RunnerTimeout <= 0 || o.RunnerTimeout > 30*time.Second {
		o.RunnerTimeout = 15 * time.Second
	}
	if r, ok := o.Repository.(*repository.MemoryLearningRepository); ok {
		if identity, ok := o.IdentityRepository.(*repository.MemoryIdentityRepository); ok {
			r.BindIdentity(identity)
		}
	}
	return &LearningService{options: o}
}

type LearningCommand struct{ Key, RequestID, OperationID string }

type LearningLevelInput struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	SortOrder   int          `json:"sort_order"`
	AccessMode  string       `json:"access_mode"`
	RequiredXP  int          `json:"required_xp"`
	Scope       entity.Scope `json:"scope"`
}

type LearningLevelUpdate struct {
	Version     int64  `json:"version"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	AccessMode  string `json:"access_mode"`
	RequiredXP  int    `json:"required_xp"`
}

type LearningModuleInput struct {
	LevelID     string `json:"level_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

type LearningModuleUpdate struct {
	Version     int64  `json:"version"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

type LearningLessonInput struct {
	ModuleID    string                    `json:"module_id"`
	Title       string                    `json:"title"`
	SortOrder   int                       `json:"sort_order"`
	Language    string                    `json:"language"`
	Explanation string                    `json:"explanation"`
	CodeExample string                    `json:"code_example"`
	StarterCode string                    `json:"starter_code"`
	Solution    string                    `json:"solution"`
	Hint        string                    `json:"hint"`
	XPReward    int                       `json:"xp_reward"`
	Quiz        *entity.LearningQuiz      `json:"quiz"`
	TestCases   []entity.LearningTestCase `json:"test_cases"`
}

type LearningLessonUpdate struct {
	LearningLessonInput
	Version int64 `json:"version"`
}

type LearningProgressInput struct {
	Revision          int64  `json:"revision"`
	DraftCode         string `json:"draft_code"`
	Language          string `json:"language"`
	QuizOption        *int   `json:"quiz_option"`
	Complete          bool   `json:"complete"`
	BrowserTestPassed bool   `json:"browser_test_passed"`
	LessonVersion     int64  `json:"lesson_version"`
}

type LearningPlaygroundInput struct {
	LessonID string `json:"lesson_id"`
	Source   string `json:"source"`
	Language string `json:"language"`
	Version  string `json:"version"`
	Stdin    string `json:"stdin"`
}

type LearningHintInput struct {
	LessonID  string `json:"lesson_id"`
	DraftCode string `json:"draft_code"`
}

type LearningBugHuntInput struct {
	Language    string       `json:"language"`
	Difficulty  string       `json:"difficulty"`
	Title       string       `json:"title"`
	BuggyCode   string       `json:"buggy_code"`
	BugLine     int          `json:"bug_line"`
	Explanation string       `json:"explanation"`
	XPReward    int          `json:"xp_reward"`
	Active      bool         `json:"active"`
	Scope       entity.Scope `json:"scope"`
}

type LearningBugHuntUpdate struct {
	LearningBugHuntInput
	Version int64 `json:"version"`
}

type LearningBugHuntAttemptInput struct {
	QuestionID   string `json:"question_id"`
	SelectedLine int    `json:"selected_line"`
	Explanation  string `json:"explanation"`
}

type learningOperation struct {
	PayloadHash string `json:"payload_hash"`
	TargetID    string `json:"target_id"`
}

func CalculateLearningLevel(xp int) int {
	if xp <= 0 {
		return 1
	}
	return int(math.Floor(math.Sqrt(float64(xp)/50.0))) + 1
}

func NextLearningLevelXP(level int) int {
	if level < 1 {
		level = 1
	}
	return 50 * level * level
}

func jakartaDate(t time.Time) string {
	return t.UTC().Add(7 * time.Hour).Format("2006-01-02")
}

func updateLearningStreak(lastDate string, currentStreak int, now time.Time) (int, string) {
	today := jakartaDate(now)
	yesterday := jakartaDate(now.Add(-24 * time.Hour))
	if lastDate == today && currentStreak > 0 {
		return currentStreak, today
	}
	if lastDate == yesterday && currentStreak > 0 {
		return currentStreak + 1, today
	}
	return 1, today
}

const learningUnconfiguredLevelOne = "unconfigured-level-1"

func learningAchievements(tx repository.LearningTx) ([]entity.LearningAchievement, error) {
	levels, err := tx.List(repository.LearningLevels, repository.LearningFilter{SortOrder: 1, Limit: 2})
	if err != nil {
		return nil, err
	}
	levelOneID := learningUnconfiguredLevelOne
	if len(levels) == 1 {
		levelOneID = levels[0].ID
	}
	return []entity.LearningAchievement{
		{ID: "first-lesson", Title: "Langkah Pertama", Description: "Menyelesaikan 1 pelajaran E-learning.", RequirementType: "lesson_count", RequirementValue: "1"},
		{ID: "xp-100", Title: "Penjelajah Kode", Description: "Mengumpulkan minimal 100 XP.", RequirementType: "xp", RequirementValue: "100"},
		{ID: "streak-3", Title: "Konsisten 3 Hari", Description: "Mempertahankan streak belajar 3 hari berturut-turut.", RequirementType: "streak", RequirementValue: "3"},
		{ID: "bug-hunter", Title: "Pemburu Bug", Description: "Menemukan minimal 1 bug dengan tepat pada Bug Hunt.", RequirementType: "game_score", RequirementValue: "1"},
		{ID: "level-1-complete", Title: "Lulus Dasar C", Description: "Menyelesaikan seluruh pelajaran pada Level 1.", RequirementType: "level_completed", RequirementValue: levelOneID},
	}, nil
}

func learningID(prefix string) string {
	v, err := security.RandomToken("")
	if err != nil {
		panic(err)
	}
	return prefix + "-" + v
}

func learningHash(v any) string { data, _ := json.Marshal(v); return security.Hash(string(data)) }

func learningReadRecord[T any](tx repository.LearningTx, k repository.LearningKind, id string, lock bool) (T, error) {
	var out T
	var row repository.LearningRecord
	var err error
	if lock {
		row, err = tx.Lock(k, id)
	} else {
		row, err = tx.Get(k, id)
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(row.Data, &out)
	return out, err
}

func learningPut(tx repository.LearningTx, k repository.LearningKind, id, parent string, owner int64, scope entity.Scope, status, unique string, expires *time.Time, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return tx.Put(k, repository.LearningRecord{ID: id, ParentID: parent, OwnerID: owner, Scope: scope, Status: status, UniqueKey: unique, ExpiresAt: expires, Data: raw})
}

func learningAudit(tx repository.LearningTx, p *entity.Principal, c LearningCommand, action, id string, scope entity.Scope) error {
	now, err := tx.Now()
	if err != nil {
		return err
	}
	var actor *int64
	if p != nil {
		actor = &p.User.ID
	}
	if scope.Domain == "" {
		scope.Domain = "elearning"
	}
	return tx.Audit(entity.AuditEvent{ActorID: actor, Action: action, App: "elearning", SubjectType: "learning", SubjectID: id, Scope: scope, RequestID: c.RequestID, OperationID: c.OperationID, CreatedAt: now, Detail: map[string]any{}})
}

func learningScopes(p *entity.Principal, permissions ...string) []entity.Scope {
	return practicumScopes(p, permissions...)
}

func learningRequire(p *entity.Principal, permission string, scope entity.Scope) error {
	return (authorization.Service{}).Check(p, authorization.Requirement{Permission: permission, Scope: scope, ResourceExists: true, HideNotFound: true})
}

func learningMutate[T any](s *LearningService, ctx context.Context, p *entity.Principal, c LearningCommand, action, resource string, input any, fn func(repository.LearningTx, string) (T, string, entity.Scope, error)) (T, error) {
	var out T
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !practicumKeyPattern.MatchString(c.Key) {
		return out, ErrValidation
	}
	opID := learningHash([]any{p.User.ID, action, resource, c.Key})
	hash := learningHash(input)
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		replay := ""
		op, err := learningReadRecord[learningOperation](tx, repository.LearningOperations, opID, false)
		if err == nil {
			if op.PayloadHash != hash {
				return repository.ErrConflict
			}
			replay = op.TargetID
		} else if !errors.Is(err, repository.ErrLearningNotFound) {
			return err
		}
		var target string
		var scope entity.Scope
		out, target, scope, err = fn(tx, replay)
		if err != nil {
			return err
		}
		if replay != "" {
			return nil
		}
		if scope.Domain == "" {
			scope.Domain = "elearning"
		}
		if err = learningPut(tx, repository.LearningOperations, opID, "", p.User.ID, scope, "", opID, nil, learningOperation{PayloadHash: hash, TargetID: target}); err != nil {
			return err
		}
		return learningAudit(tx, p, c, action, target, scope)
	})
	return out, err
}

func learningProfileID(userID int64) string { return fmt.Sprintf("profile-%d", userID) }
func learningProgressID(lessonID string, userID int64) string {
	return fmt.Sprintf("progress-%s-%d", lessonID, userID)
}
func learningProgressKey(lessonID string, userID int64) string {
	return fmt.Sprintf("%s:%d", lessonID, userID)
}

func loadOrInitProfile(tx repository.LearningTx, p *entity.Principal, now time.Time, lock bool) (entity.LearningProfile, error) {
	id := learningProfileID(p.User.ID)
	profile, err := learningReadRecord[entity.LearningProfile](tx, repository.LearningProfiles, id, lock)
	if err == nil {
		profile.Timezone = "Asia/Jakarta"
		profile.TimezoneOffset = "+07:00"
		profile.Level = CalculateLearningLevel(profile.XP)
		profile.NextLevelXP = NextLearningLevelXP(profile.Level)
		if profile.UnlockedAchievements == nil {
			profile.UnlockedAchievements = []entity.LearningUnlockedAchievement{}
		}
		return profile, nil
	}
	if !errors.Is(err, repository.ErrLearningNotFound) {
		return entity.LearningProfile{}, err
	}
	return entity.LearningProfile{
		UserID:               p.User.ID,
		UserName:             p.User.Name,
		XP:                   0,
		Level:                1,
		NextLevelXP:          NextLearningLevelXP(1),
		Streak:               0,
		LastActiveDate:       "",
		CompletedLessons:     0,
		GamesPlayed:          0,
		PerfectGames:         0,
		UnlockedAchievements: []entity.LearningUnlockedAchievement{},
		Timezone:             "Asia/Jakarta",
		TimezoneOffset:       "+07:00",
		UpdatedAt:            now,
	}, nil
}

func saveProfile(tx repository.LearningTx, profile entity.LearningProfile, scope entity.Scope, now time.Time) (entity.LearningProfile, error) {
	profile.Level = CalculateLearningLevel(profile.XP)
	profile.NextLevelXP = NextLearningLevelXP(profile.Level)
	profile.Timezone = "Asia/Jakarta"
	profile.TimezoneOffset = "+07:00"
	profile.UpdatedAt = now
	if profile.UnlockedAchievements == nil {
		profile.UnlockedAchievements = []entity.LearningUnlockedAchievement{}
	}
	if scope.Domain == "" {
		scope.Domain = "elearning"
	}
	err := learningPut(tx, repository.LearningProfiles, learningProfileID(profile.UserID), "", profile.UserID, scope, "active", learningProfileID(profile.UserID), nil, profile)
	return profile, err
}

func levelUnlocked(level entity.LearningLevel, profile entity.LearningProfile, isStaff bool) bool {
	if isStaff {
		return true
	}
	switch level.AccessMode {
	case "unlocked":
		return true
	case "locked":
		return false
	default:
		return profile.XP >= level.RequiredXP
	}
}

func isLearningStaff(p *entity.Principal, scope entity.Scope) bool {
	return practicumAllowed(p, learningManage, scope) || practicumAllowed(p, learningMonitor, scope)
}

func publicLesson(l entity.LearningLesson, admin bool) entity.LearningLesson {
	if l.TestCases == nil {
		l.TestCases = []entity.LearningTestCase{}
	}
	if !admin {
		l.Solution = ""
		if l.Quiz != nil {
			q := *l.Quiz
			q.CorrectOption = nil
			l.Quiz = &q
		}
	}
	return l
}

func evaluateAchievements(tx repository.LearningTx, profile *entity.LearningProfile, completedLevels map[string]bool, now time.Time) ([]entity.LearningAchievement, error) {
	achievements, err := learningAchievements(tx)
	if err != nil {
		return nil, err
	}
	unlockedMap := map[string]time.Time{}
	for _, u := range profile.UnlockedAchievements {
		unlockedMap[u.AchievementID] = u.UnlockedAt
	}
	newly := []entity.LearningAchievement{}
	for _, ach := range achievements {
		if _, exists := unlockedMap[ach.ID]; exists {
			continue
		}
		met := false
		switch ach.RequirementType {
		case "xp":
			target, _ := strconv.Atoi(ach.RequirementValue)
			met = profile.XP >= target
		case "lesson_count":
			target, _ := strconv.Atoi(ach.RequirementValue)
			met = profile.CompletedLessons >= target
		case "streak":
			target, _ := strconv.Atoi(ach.RequirementValue)
			met = profile.Streak >= target
		case "game_score":
			target, _ := strconv.Atoi(ach.RequirementValue)
			met = profile.PerfectGames >= target
		case "level_completed":
			met = ach.RequirementValue != learningUnconfiguredLevelOne && completedLevels[ach.RequirementValue]
		}
		if met {
			profile.UnlockedAchievements = append(profile.UnlockedAchievements, entity.LearningUnlockedAchievement{
				AchievementID: ach.ID,
				UnlockedAt:    now,
			})
			unlockedAt := now
			ach.Unlocked = true
			ach.UnlockedAt = &unlockedAt
			newly = append(newly, ach)
		}
	}
	return newly, nil
}

func completedLevelsForUser(tx repository.LearningTx, userID int64) (map[string]bool, error) {
	completed := map[string]bool{}
	lessons, err := tx.List(repository.LearningLessons, repository.LearningFilter{Limit: 1000})
	if err != nil {
		return completed, err
	}
	progressRows, err := tx.List(repository.LearningProgress, repository.LearningFilter{OwnerID: userID, Status: "completed", Limit: 1000})
	if err != nil {
		return completed, err
	}
	doneSet := map[string]bool{}
	for _, r := range progressRows {
		doneSet[r.ParentID] = true
	}
	totalByLevel := map[string]int{}
	doneByLevel := map[string]int{}
	for _, r := range lessons {
		var l entity.LearningLesson
		if err = json.Unmarshal(r.Data, &l); err != nil {
			return completed, err
		}
		totalByLevel[l.LevelID]++
		if doneSet[l.ID] {
			doneByLevel[l.LevelID]++
		}
	}
	for levelID, total := range totalByLevel {
		if total > 0 && doneByLevel[levelID] == total {
			completed[levelID] = true
		}
	}
	return completed, nil
}

func (s *LearningService) RunnerInfo() entity.PracticumRunnerStatus {
	if s.options.Runner == nil {
		return entity.PracticumRunnerStatus{Languages: []entity.RunnerLanguage{}}
	}
	return entity.PracticumRunnerStatus{
		Languages: s.options.Runner.Languages(),
		Available: s.options.Runner.Available(),
		Simulated: s.options.Runner.Simulated(),
	}
}

func (s *LearningService) Levels(ctx context.Context, p *entity.Principal) (entity.LearningLevelsView, error) {
	out := entity.LearningLevelsView{Levels: []entity.LearningLevel{}, Runner: s.RunnerInfo()}
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
		levelRows, err := tx.List(repository.LearningLevels, repository.LearningFilter{Scopes: scopes, Overlap: true, Limit: 200})
		if err != nil {
			return err
		}
		moduleRows, err := tx.List(repository.LearningModules, repository.LearningFilter{Scopes: scopes, Overlap: true, Limit: 500})
		if err != nil {
			return err
		}
		lessonRows, err := tx.List(repository.LearningLessons, repository.LearningFilter{Scopes: scopes, Overlap: true, Limit: 1000})
		if err != nil {
			return err
		}
		progressRows, err := tx.List(repository.LearningProgress, repository.LearningFilter{OwnerID: p.User.ID, Limit: 1000})
		if err != nil {
			return err
		}
		progressMap := map[string]string{}
		for _, r := range progressRows {
			progressMap[r.ParentID] = r.Status
		}
		lessonsByModule := map[string][]entity.LearningLesson{}
		for _, r := range lessonRows {
			var l entity.LearningLesson
			if err = json.Unmarshal(r.Data, &l); err != nil {
				return err
			}
			l = publicLesson(l, false)
			l.Status = progressMap[l.ID]
			if l.Status == "" {
				l.Status = "not_started"
			}
			lessonsByModule[l.ModuleID] = append(lessonsByModule[l.ModuleID], l)
		}
		for k := range lessonsByModule {
			sort.Slice(lessonsByModule[k], func(i, j int) bool {
				if lessonsByModule[k][i].SortOrder == lessonsByModule[k][j].SortOrder {
					return lessonsByModule[k][i].ID < lessonsByModule[k][j].ID
				}
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
			for _, l := range m.Lessons {
				if l.Status == "completed" {
					m.CompletedLessons++
				}
			}
			modulesByLevel[m.LevelID] = append(modulesByLevel[m.LevelID], m)
		}
		for k := range modulesByLevel {
			sort.Slice(modulesByLevel[k], func(i, j int) bool {
				if modulesByLevel[k][i].SortOrder == modulesByLevel[k][j].SortOrder {
					return modulesByLevel[k][i].ID < modulesByLevel[k][j].ID
				}
				return modulesByLevel[k][i].SortOrder < modulesByLevel[k][j].SortOrder
			})
		}
		for _, r := range levelRows {
			var lvl entity.LearningLevel
			if err = json.Unmarshal(r.Data, &lvl); err != nil {
				return err
			}
			lvl.Unlocked = levelUnlocked(lvl, out.Profile, isLearningStaff(p, lvl.Scope))
			lvl.Modules = modulesByLevel[lvl.ID]
			if lvl.Modules == nil {
				lvl.Modules = []entity.LearningModule{}
			}
			for _, m := range lvl.Modules {
				lvl.TotalLessons += m.TotalLessons
				lvl.CompletedLessons += m.CompletedLessons
			}
			out.Levels = append(out.Levels, lvl)
		}
		sort.Slice(out.Levels, func(i, j int) bool {
			if out.Levels[i].SortOrder == out.Levels[j].SortOrder {
				return out.Levels[i].ID < out.Levels[j].ID
			}
			return out.Levels[i].SortOrder < out.Levels[j].SortOrder
		})
		return nil
	})
	return out, err
}

func (s *LearningService) Module(ctx context.Context, p *entity.Principal, id string) (entity.LearningModule, error) {
	var out entity.LearningModule
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		mod, err := learningReadRecord[entity.LearningModule](tx, repository.LearningModules, id, false)
		if err != nil {
			return err
		}
		lvl, err := learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, mod.LevelID, false)
		if err != nil {
			return err
		}
		staff := isLearningStaff(p, lvl.Scope)
		if !staff && !practicumAllowed(p, learningRead, lvl.Scope) {
			return authorization.ErrNotFound
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		profile, err := loadOrInitProfile(tx, p, now, false)
		if err != nil {
			return err
		}
		if !levelUnlocked(lvl, profile, staff) {
			return ErrLearningLocked
		}
		lessonRows, err := tx.List(repository.LearningLessons, repository.LearningFilter{ParentID: mod.ID, Limit: 200})
		if err != nil {
			return err
		}
		progressRows, err := tx.List(repository.LearningProgress, repository.LearningFilter{OwnerID: p.User.ID, Limit: 1000})
		if err != nil {
			return err
		}
		progressMap := map[string]string{}
		for _, r := range progressRows {
			progressMap[r.ParentID] = r.Status
		}
		mod.Lessons = []entity.LearningLesson{}
		for _, r := range lessonRows {
			var l entity.LearningLesson
			if err = json.Unmarshal(r.Data, &l); err != nil {
				return err
			}
			l = publicLesson(l, false)
			l.Status = progressMap[l.ID]
			if l.Status == "" {
				l.Status = "not_started"
			}
			if l.Status == "completed" {
				mod.CompletedLessons++
			}
			mod.Lessons = append(mod.Lessons, l)
		}
		sort.Slice(mod.Lessons, func(i, j int) bool {
			if mod.Lessons[i].SortOrder == mod.Lessons[j].SortOrder {
				return mod.Lessons[i].ID < mod.Lessons[j].ID
			}
			return mod.Lessons[i].SortOrder < mod.Lessons[j].SortOrder
		})
		mod.TotalLessons = len(mod.Lessons)
		out = mod
		return nil
	})
	return out, err
}

func loadLessonAndCheckAccess(tx repository.LearningTx, p *entity.Principal, lessonID string, now time.Time, lockProfile bool) (entity.LearningLesson, entity.LearningLevel, entity.LearningProfile, error) {
	var lesson entity.LearningLesson
	var level entity.LearningLevel
	var profile entity.LearningProfile
	lesson, err := learningReadRecord[entity.LearningLesson](tx, repository.LearningLessons, lessonID, false)
	if err != nil {
		return lesson, level, profile, err
	}
	level, err = learningReadRecord[entity.LearningLevel](tx, repository.LearningLevels, lesson.LevelID, false)
	if err != nil {
		return lesson, level, profile, err
	}
	staff := isLearningStaff(p, level.Scope)
	if !staff && !practicumAllowed(p, learningRead, level.Scope) {
		return lesson, level, profile, authorization.ErrNotFound
	}
	profile, err = loadOrInitProfile(tx, p, now, lockProfile)
	if err != nil {
		return lesson, level, profile, err
	}
	if !levelUnlocked(level, profile, staff) {
		return lesson, level, profile, ErrLearningLocked
	}
	return lesson, level, profile, nil
}

func loadOrInitProgress(tx repository.LearningTx, lesson entity.LearningLesson, userID int64, now time.Time, lock bool) (entity.LearningProgress, bool, error) {
	id := learningProgressID(lesson.ID, userID)
	progress, err := learningReadRecord[entity.LearningProgress](tx, repository.LearningProgress, id, lock)
	if err == nil {
		return progress, true, nil
	}
	if !errors.Is(err, repository.ErrLearningNotFound) {
		return entity.LearningProgress{}, false, err
	}
	return entity.LearningProgress{
		LessonID:  lesson.ID,
		ModuleID:  lesson.ModuleID,
		LevelID:   lesson.LevelID,
		UserID:    userID,
		Status:    "in_progress",
		DraftCode: lesson.StarterCode,
		Language:  lesson.Language,
		Revision:  0,
		XPAwarded: 0,
		UpdatedAt: now,
	}, false, nil
}

func (s *LearningService) Lesson(ctx context.Context, p *entity.Principal, id string) (entity.LearningLessonView, error) {
	var out entity.LearningLessonView
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.LearningTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		lesson, _, profile, err := loadLessonAndCheckAccess(tx, p, id, now, false)
		if err != nil {
			return err
		}
		progress, _, err := loadOrInitProgress(tx, lesson, p.User.ID, now, false)
		if err != nil {
			return err
		}
		out = entity.LearningLessonView{
			Lesson:        publicLesson(lesson, false),
			Progress:      progress,
			Profile:       profile,
			NewlyUnlocked: []entity.LearningAchievement{},
		}
		return nil
	})
	return out, err
}

func (s *LearningService) SaveProgress(ctx context.Context, p *entity.Principal, c LearningCommand, lessonID string, in LearningProgressInput) (entity.LearningLessonView, error) {
	if in.Revision < 0 || len(in.DraftCode) > 64<<10 || (in.Language != "c" && in.Language != "python") ||
		(!in.Complete && (in.BrowserTestPassed || in.LessonVersion != 0)) {
		return entity.LearningLessonView{}, ErrValidation
	}
	return learningMutate(s, ctx, p, c, "learning.lesson.progress", lessonID, in, func(tx repository.LearningTx, replay string) (out entity.LearningLessonView, target string, scope entity.Scope, err error) {
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		lesson, level, profile, e := loadLessonAndCheckAccess(tx, p, lessonID, now, true)
		if e != nil {
			err = e
			return
		}
		scope = level.Scope
		target = learningProgressID(lesson.ID, p.User.ID)
		if in.Language != lesson.Language {
			err = ErrValidation
			return
		}
		if lesson.Quiz != nil {
			if in.QuizOption != nil && (*in.QuizOption < 0 || *in.QuizOption >= len(lesson.Quiz.Options)) {
				err = ErrValidation
				return
			}
		} else if in.QuizOption != nil {
			err = ErrValidation
			return
		}
		progress, _, e := loadOrInitProgress(tx, lesson, p.User.ID, now, true)
		if e != nil {
			err = e
			return
		}
		if replay != "" {
			out = entity.LearningLessonView{
				Lesson:        publicLesson(lesson, false),
				Progress:      progress,
				Profile:       profile,
				NewlyUnlocked: []entity.LearningAchievement{},
			}
			return
		}
		if progress.Revision != in.Revision {
			err = repository.ErrConflict
			return
		}
		newlyUnlocked := []entity.LearningAchievement{}
		codeRequired := learningCodeRequired(lesson)
		if !codeRequired && (in.BrowserTestPassed || in.LessonVersion != 0) {
			err = ErrValidation
			return
		}
		if in.Complete {
			if codeRequired && len(strings.TrimSpace(in.DraftCode)) == 0 {
				err = ErrValidation
				return
			}
			if lesson.Quiz != nil && lesson.Quiz.CorrectOption != nil {
				if in.QuizOption == nil || *in.QuizOption != *lesson.Quiz.CorrectOption {
					err = ErrLearningQuizIncorrect
					return
				}
			}
			testPassed := false
			if codeRequired {
				testPassed = progress.TestPassed && progress.DraftCode == in.DraftCode
				if progress.Status != "completed" && !in.BrowserTestPassed {
					err = ErrLearningCodeNotVerified
					return
				}
				if in.BrowserTestPassed {
					if in.LessonVersion != lesson.Version {
						err = repository.ErrConflict
						return
					}
					testPassed = true
				}
			}
			progress.DraftCode = in.DraftCode
			progress.Language = in.Language
			progress.QuizOption = in.QuizOption
			progress.TestPassed = testPassed
			progress.Revision++
			progress.UpdatedAt = now
			// Award XP and increment completion count at most once per (user_id, lesson_id).
			if progress.Status != "completed" {
				progress.Status = "completed"
				progress.CompletedAt = &now
				progress.XPAwarded = lesson.XPReward
				if err = learningPut(tx, repository.LearningProgress, target, lesson.ID, p.User.ID, scope, progress.Status, learningProgressKey(lesson.ID, p.User.ID), nil, progress); err != nil {
					return
				}
				profile.XP += lesson.XPReward
				profile.CompletedLessons++
				profile.Streak, profile.LastActiveDate = updateLearningStreak(profile.LastActiveDate, profile.Streak, now)
				completedLevels, e := completedLevelsForUser(tx, p.User.ID)
				if e != nil {
					err = e
					return
				}
				newlyUnlocked, err = evaluateAchievements(tx, &profile, completedLevels, now)
				if err != nil {
					return
				}
				profile, err = saveProfile(tx, profile, scope, now)
				if err != nil {
					return
				}
			} else {
				if err = learningPut(tx, repository.LearningProgress, target, lesson.ID, p.User.ID, scope, progress.Status, learningProgressKey(lesson.ID, p.User.ID), nil, progress); err != nil {
					return
				}
			}
		} else {
			progress.DraftCode = in.DraftCode
			progress.Language = in.Language
			progress.QuizOption = in.QuizOption
			progress.TestPassed = false
			progress.Revision++
			progress.UpdatedAt = now
			if err = learningPut(tx, repository.LearningProgress, target, lesson.ID, p.User.ID, scope, progress.Status, learningProgressKey(lesson.ID, p.User.ID), nil, progress); err != nil {
				return
			}
			profile, err = saveProfile(tx, profile, scope, now)
			if err != nil {
				return
			}
		}
		out = entity.LearningLessonView{
			Lesson:        publicLesson(lesson, false),
			Progress:      progress,
			Profile:       profile,
			NewlyUnlocked: newlyUnlocked,
		}
		return
	})
}

func IsLearningNotFound(err error) bool { return errors.Is(err, repository.ErrLearningNotFound) }
