package entity

import "time"

// Practicum definitions are distinct from identity sessions.
type PracticumSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Sequence  int       `json:"sequence"`
	Scope     Scope     `json:"scope"`
	Active    bool      `json:"active"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type PracticumCourse struct {
	ID              string `json:"id"`
	SessionID       string `json:"session_id"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	DurationMinutes int    `json:"duration_minutes"`
	TokenRequired   bool   `json:"token_required"`
	Version         int64  `json:"version"`
}
type PracticumQuestion struct {
	ID              string      `json:"id"`
	CourseID        string      `json:"course_id,omitempty"`
	Type            string      `json:"type"`
	Difficulty      string      `json:"difficulty"`
	Category        string      `json:"category"`
	Prompt          string      `json:"prompt"`
	Points          int         `json:"points"`
	StarterCode     string      `json:"starter_code"`
	Language        string      `json:"language"`
	ReferenceAnswer string      `json:"reference_answer,omitempty"`
	Rubric          []Criterion `json:"rubric,omitempty"`
	Version         int64       `json:"version,omitempty"`
}
type PracticumActivation struct {
	ID               string     `json:"id"`
	SessionID        string     `json:"session_id"`
	CourseID         string     `json:"course_id"`
	Scope            Scope      `json:"scope"`
	TimerMode        string     `json:"timer_mode"`
	IsOpen           bool       `json:"is_open"`
	StartedAt        *time.Time `json:"started_at"`
	DeadlineAt       *time.Time `json:"deadline_at"`
	ClosesAt         *time.Time `json:"closes_at"`
	QuestionCount    int        `json:"question_count"`
	Version          int64      `json:"version"`
	StudentIDs       []int64    `json:"student_ids,omitempty"`
	EntryApprovedIDs []int64    `json:"entry_approved_ids,omitempty"`
	QuestionIDs      []string   `json:"question_ids,omitempty"`
	AccessToken      string     `json:"access_token,omitempty"`
}

// ActivationRecord is never an HTTP response. Rules and eligibility freeze at activation.
type PracticumActivationRecord struct {
	Activation PracticumActivation `json:"activation"`
	Course     PracticumCourse     `json:"course"`
	Questions  []PracticumQuestion `json:"questions"`
	TokenHash  string              `json:"token_hash"`
}
type PracticumAnswer struct {
	QuestionID string    `json:"question_id"`
	Value      string    `json:"value"`
	Language   string    `json:"language"`
	UpdatedAt  time.Time `json:"updated_at"`
	Score      *int      `json:"score,omitempty"`
	Feedback   string    `json:"feedback,omitempty"`
}
type PracticumAttempt struct {
	AttemptID        string              `json:"attempt_id"`
	ActivationID     string              `json:"activation_id"`
	SessionID        string              `json:"session_id"`
	CourseID         string              `json:"course_id"`
	UserID           int64               `json:"user_id"`
	Status           string              `json:"status"`
	SubmissionReason string              `json:"submission_reason"`
	Revision         int64               `json:"revision"`
	GradeRevision    int64               `json:"grade_revision"`
	GradingStatus    string              `json:"grading_status"`
	StartedAt        time.Time           `json:"started_at"`
	DeadlineAt       time.Time           `json:"deadline_at"`
	SubmittedAt      *time.Time          `json:"submitted_at"`
	ServerNow        time.Time           `json:"server_now"`
	Timezone         string              `json:"timezone"`
	TimezoneOffset   string              `json:"timezone_offset"`
	Questions        []PracticumQuestion `json:"questions"`
	Answers          []PracticumAnswer   `json:"answers"`
	Score            *int                `json:"score"`
	MaxScore         int                 `json:"max_score"`
	Percentage       *float64            `json:"percentage"`
}
type PracticumAttemptRecord struct {
	Attempt          PracticumAttempt       `json:"attempt"`
	Scope            Scope                  `json:"scope"`
	UserName         string                 `json:"user_name"`
	SubmittedAnswers []PracticumAnswer      `json:"submitted_answers"`
	ManualGrades     []PracticumManualGrade `json:"manual_grades"`
	GradeReason      string                 `json:"grade_reason"`
}
type PracticumManualGrade struct {
	QuestionID string `json:"question_id"`
	Score      int    `json:"score"`
	Feedback   string `json:"feedback"`
}
type PracticumGradingJob struct {
	ID            string           `json:"id"`
	AttemptID     string           `json:"attempt_id"`
	Status        string           `json:"status"`
	GradeRevision int64            `json:"grade_revision"`
	LeaseID       string           `json:"lease_id,omitempty"`
	LeaseUntil    *time.Time       `json:"lease_until,omitempty"`
	Suggestions   []SuggestedGrade `json:"suggestions"`
	Provider      string           `json:"provider,omitempty"`
	Simulated     bool             `json:"simulated"`
	UpdatedAt     time.Time        `json:"updated_at"`
}
type PracticumAttemptReview struct {
	Attempt     PracticumAttempt     `json:"attempt"`
	Questions   []PracticumQuestion  `json:"questions"`
	Suggestions []SuggestedGrade     `json:"suggestions"`
	GradingJob  *PracticumGradingJob `json:"grading_job"`
}
type PracticumSessionDetail struct {
	Session     PracticumSession      `json:"session"`
	Courses     []PracticumCourse     `json:"courses"`
	Activations []PracticumActivation `json:"activations"`
}
type PracticumMonitoringRow struct {
	AttemptID     string     `json:"attempt_id"`
	UserID        int64      `json:"user_id"`
	UserName      string     `json:"user_name"`
	ActivationID  string     `json:"activation_id"`
	SessionID     string     `json:"session_id"`
	CourseID      string     `json:"course_id"`
	Status        string     `json:"status"`
	GradingStatus string     `json:"grading_status"`
	AnswerCount   int        `json:"answer_count"`
	QuestionCount int        `json:"question_count"`
	DeadlineAt    time.Time  `json:"deadline_at"`
	SubmittedAt   *time.Time `json:"submitted_at"`
}
type PracticumGradeSummary struct {
	Status        string     `json:"status"`
	CanOpen       bool       `json:"can_open"`
	AttemptID     string     `json:"attempt_id"`
	SessionID     string     `json:"session_id"`
	SessionTitle  string     `json:"session_title"`
	CourseTitle   string     `json:"course_title"`
	CourseKind    string     `json:"course_kind"`
	GradingStatus string     `json:"grading_status"`
	Score         *int       `json:"score"`
	MaxScore      int        `json:"max_score"`
	Percentage    *float64   `json:"percentage"`
	SubmittedAt   *time.Time `json:"submitted_at"`
}
