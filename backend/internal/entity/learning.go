package entity

import "time"

type LearningQuiz struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectOption *int     `json:"correct_option,omitempty"`
}

type LearningTestCase struct {
	Description    string `json:"description"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expected_output"`
}

type LearningTestResult struct {
	Description    string `json:"description"`
	Passed         bool   `json:"passed"`
	ExpectedOutput string `json:"expected_output"`
	ActualOutput   string `json:"actual_output"`
}

type LearningUnlockedAchievement struct {
	AchievementID string    `json:"achievement_id"`
	UnlockedAt    time.Time `json:"unlocked_at"`
}

type LearningProfile struct {
	UserID               int64                         `json:"user_id"`
	UserName             string                        `json:"user_name"`
	XP                   int                           `json:"xp"`
	Level                int                           `json:"level"`
	NextLevelXP          int                           `json:"next_level_xp"`
	Streak               int                           `json:"streak"`
	LastActiveDate       string                        `json:"last_active_date"`
	CompletedLessons     int                           `json:"completed_lessons"`
	GamesPlayed          int                           `json:"games_played"`
	PerfectGames         int                           `json:"perfect_games"`
	Prestige             int                           `json:"prestige"`
	PrestigeCount        int                           `json:"prestige_count"`
	UnlockedAchievements []LearningUnlockedAchievement `json:"unlocked_achievements"`
	Timezone             string                        `json:"timezone"`
	TimezoneOffset       string                        `json:"timezone_offset"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

type LearningPrestigeEntry struct {
	ID        string         `json:"id"`
	UserID    int64          `json:"user_id"`
	Source    string         `json:"source"`
	Amount    int            `json:"amount"`
	Reason    string         `json:"reason"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type LearningLesson struct {
	ID          string             `json:"id"`
	ModuleID    string             `json:"module_id"`
	LevelID     string             `json:"level_id"`
	Title       string             `json:"title"`
	SortOrder   int                `json:"sort_order"`
	Language    string             `json:"language"`
	Explanation string             `json:"explanation"`
	CodeExample string             `json:"code_example"`
	StarterCode string             `json:"starter_code"`
	Solution    string             `json:"solution,omitempty"`
	Hint        string             `json:"hint"`
	XPReward    int                `json:"xp_reward"`
	Quiz        *LearningQuiz      `json:"quiz,omitempty"`
	TestCases   []LearningTestCase `json:"test_cases"`
	Status      string             `json:"status,omitempty"`
	Scope       Scope              `json:"scope,omitempty"`
	Version     int64              `json:"version,omitempty"`
}

type LearningModule struct {
	ID               string           `json:"id"`
	LevelID          string           `json:"level_id"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	SortOrder        int              `json:"sort_order"`
	CompletedLessons int              `json:"completed_lessons"`
	TotalLessons     int              `json:"total_lessons"`
	Scope            Scope            `json:"scope,omitempty"`
	Version          int64            `json:"version"`
	Lessons          []LearningLesson `json:"lessons,omitempty"`
}

type LearningLevel struct {
	ID               string           `json:"id"`
	Title            string           `json:"title"`
	Description      string           `json:"description"`
	SortOrder        int              `json:"sort_order"`
	AccessMode       string           `json:"access_mode"`
	RequiredXP       int              `json:"required_xp"`
	Unlocked         bool             `json:"unlocked"`
	CompletedLessons int              `json:"completed_lessons"`
	TotalLessons     int              `json:"total_lessons"`
	Scope            Scope            `json:"scope"`
	Version          int64            `json:"version"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Modules          []LearningModule `json:"modules,omitempty"`
}

type LearningProgress struct {
	LessonID    string     `json:"lesson_id"`
	ModuleID    string     `json:"module_id"`
	LevelID     string     `json:"level_id"`
	UserID      int64      `json:"user_id"`
	Status      string     `json:"status"`
	DraftCode   string     `json:"draft_code"`
	Language    string     `json:"language"`
	QuizOption  *int       `json:"quiz_option"`
	TestPassed  bool       `json:"test_passed"`
	Revision    int64      `json:"revision"`
	XPAwarded   int        `json:"xp_awarded"`
	CompletedAt *time.Time `json:"completed_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type LearningAchievement struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	RequirementType  string     `json:"requirement_type"`
	RequirementValue string     `json:"requirement_value"`
	Unlocked         bool       `json:"unlocked"`
	UnlockedAt       *time.Time `json:"unlocked_at"`
}

type LearningLevelsView struct {
	Profile LearningProfile       `json:"profile"`
	Levels  []LearningLevel       `json:"levels"`
	Runner  PracticumRunnerStatus `json:"runner"`
}

type PracticumRunnerStatus struct {
	Languages []RunnerLanguage `json:"languages"`
	Simulated bool             `json:"simulated"`
	Available bool             `json:"available"`
}

type LearningLessonView struct {
	Lesson        LearningLesson        `json:"lesson"`
	Progress      LearningProgress      `json:"progress"`
	Profile       LearningProfile       `json:"profile"`
	NewlyUnlocked []LearningAchievement `json:"newly_unlocked"`
}

type LearningRunResponse struct {
	Result      RunResult            `json:"result"`
	TestResults []LearningTestResult `json:"test_results"`
	AllPassed   bool                 `json:"all_passed"`
}

type LearningHintResponse struct {
	LessonID       string `json:"lesson_id"`
	Hint           string `json:"hint"`
	RemainingQuota int    `json:"remaining_quota"`
	Simulated      bool   `json:"simulated"`
}

type LearningAchievementCatalogItem struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	RequirementType  string `json:"requirement_type"`
	RequirementValue string `json:"requirement_value"`
	XPReward         int    `json:"xp_reward"`
	Active           bool   `json:"active"`
	Scope            Scope  `json:"scope"`
	Version          int64  `json:"version"`
}

type LearningWeeklySnapshot struct {
	WeekStart   string                   `json:"week_start"`
	GeneratedAt time.Time                `json:"generated_at"`
	Rows        []LearningLeaderboardRow `json:"rows"`
}

type LearningAchievementsView struct {
	Profile      LearningProfile       `json:"profile"`
	Achievements []LearningAchievement `json:"achievements"`
}

type LearningLeaderboardRow struct {
	Rank             int    `json:"rank"`
	UserID           int64  `json:"user_id"`
	UserName         string `json:"user_name"`
	XP               int    `json:"xp"`
	Level            int    `json:"level"`
	Streak           int    `json:"streak"`
	CompletedLessons int    `json:"completed_lessons"`
	AchievementCount int    `json:"achievement_count"`
}

type LearningBugHuntQuestion struct {
	ID          string `json:"id"`
	Language    string `json:"language"`
	Difficulty  string `json:"difficulty"`
	Title       string `json:"title"`
	BuggyCode   string `json:"buggy_code"`
	BugLine     *int   `json:"bug_line,omitempty"`
	Explanation string `json:"explanation,omitempty"`
	XPReward    int    `json:"xp_reward"`
	Active      bool   `json:"active"`
	Solved      bool   `json:"solved,omitempty"`
	Scope       Scope  `json:"scope"`
	Version     int64  `json:"version,omitempty"`
}

type LearningBugHuntAttempt struct {
	AttemptID    string    `json:"attempt_id"`
	QuestionID   string    `json:"question_id"`
	UserID       int64     `json:"user_id"`
	Language     string    `json:"language"`
	SelectedLine int       `json:"selected_line"`
	Explanation  string    `json:"explanation"`
	Correct      bool      `json:"correct"`
	XPEarned     int       `json:"xp_earned"`
	Feedback     string    `json:"feedback"`
	PlayedAt     time.Time `json:"played_at"`
}

type LearningBugHuntView struct {
	Questions   []LearningBugHuntQuestion `json:"questions"`
	Attempts    []LearningBugHuntAttempt  `json:"attempts"`
	WeeklyUsed  int                       `json:"weekly_used"`
	WeeklyLimit int                       `json:"weekly_limit"`
	Profile     LearningProfile           `json:"profile"`
}

type LearningBugHuntSubmitView struct {
	Attempt       LearningBugHuntAttempt `json:"attempt"`
	Profile       LearningProfile        `json:"profile"`
	NewlyUnlocked []LearningAchievement  `json:"newly_unlocked"`
}

type LearningMonitoringRow struct {
	UserID            int64     `json:"user_id"`
	UserName          string    `json:"user_name"`
	XP                int       `json:"xp"`
	Level             int       `json:"level"`
	Streak            int       `json:"streak"`
	CompletedLessons  int       `json:"completed_lessons"`
	InProgressLessons int       `json:"in_progress_lessons"`
	GamesPlayed       int       `json:"games_played"`
	LastActiveDate    string    `json:"last_active_date"`
	UpdatedAt         time.Time `json:"updated_at"`
}
