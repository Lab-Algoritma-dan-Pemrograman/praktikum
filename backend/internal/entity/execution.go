package entity

// RunRequest contains execution data only. Authorization, durable quotas and
// idempotency are owned by the calling use case, never by a provider.
type RunRequest struct {
	Language  string `json:"language"`
	Version   string `json:"version"`
	Source    string `json:"source"`
	Stdin     string `json:"stdin"`
	RequestID string `json:"request_id"`
}

type RunResult struct {
	Status        string `json:"status"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	ExitCode      *int   `json:"exit_code"`
	DurationMS    int64  `json:"duration_ms"`
	RunnerVersion string `json:"runner_version"`
	RequestID     string `json:"request_id"`
	SourceHash    string `json:"source_hash"`
	Simulated     bool   `json:"simulated"`
	Truncated     bool   `json:"truncated"`
}

type RunnerLanguage struct {
	Language string `json:"language"`
	Version  string `json:"version"`
	Label    string `json:"label"`
}

type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	MaxScore    int    `json:"max_score"`
}

// GradeRequest deliberately has no user identity, credential, scope or provider
// options. Items and private grading rules come from the submitted snapshot.
type GradeRequest struct {
	AttemptID string      `json:"attempt_id"`
	Items     []GradeItem `json:"items"`
}

type GradeItem struct {
	QuestionID      string      `json:"question_id"`
	Type            string      `json:"type"`
	Prompt          string      `json:"prompt"`
	Answer          string      `json:"answer"`
	MaxScore        int         `json:"max_score"`
	Rubric          []Criterion `json:"rubric"`
	ReferenceAnswer string      `json:"reference_answer"`
	Language        string      `json:"language"`
}

// GradeResult is a suggestion, never a published grade or an attempt transition.
// Status is needs_manual or suggested; even suggested requires manual approval.
type GradeResult struct {
	Status          string           `json:"status"`
	Suggestions     []SuggestedGrade `json:"suggestions"`
	ProviderVersion string           `json:"provider_version"`
	Simulated       bool             `json:"simulated"`
	ErrorCode       string           `json:"error_code"`
}

type SuggestedGrade struct {
	QuestionID string `json:"question_id"`
	Score      int    `json:"score"`
	Feedback   string `json:"feedback"`
}
