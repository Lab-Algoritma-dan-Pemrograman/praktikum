// Package grading defines bounded suggestions, not a grade-publication API.
// Local adapters make no AI/provider calls. Every result requires manual review;
// simulated suggestions are demonstration data, never evidence of correctness.
package grading

import (
	"context"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

const (
	StatusNeedsManual = "needs_manual"
	StatusSuggested   = "suggested"
	FixtureVersion    = "fixture-grader-v1"

	MaxRequestBytes  = 256 << 10
	MaxResultBytes   = 128 << 10
	MaxAnswerBytes   = 64 << 10
	MaxPromptBytes   = 8 << 10
	MaxFeedbackBytes = 4 << 10
	MaxQuestions     = 20
	MaxCriteria      = 20
)

var (
	ErrInvalidRequest = errors.New("invalid grading request")
	ErrInvalidResult  = errors.New("invalid grading result")
)

// Grader implementations must honor context deadlines and must not retry an
// uncertain submission automatically. The calling use case owns job leases,
// bounded call deadlines, audit metadata, CAS and all manual approval/publishing.
// No adapter can publish a grade through this interface.
type Grader interface {
	Grade(context.Context, entity.GradeRequest) (entity.GradeResult, error)
	Available() bool
	Simulated() bool
}

type Unconfigured struct{}

func (Unconfigured) Available() bool { return false }
func (Unconfigured) Simulated() bool { return false }
func (Unconfigured) Grade(ctx context.Context, req entity.GradeRequest) (entity.GradeResult, error) {
	if err := ctx.Err(); err != nil {
		return manualFallback("unconfigured", false, "cancelled"), err
	}
	if err := ValidateRequest(req); err != nil {
		return manualFallback("unconfigured", false, "invalid_request"), err
	}
	return manualFallback("unconfigured", false, "provider_not_configured"), nil
}

type Fixture struct{}

func NewFixture() *Fixture       { return &Fixture{} }
func (*Fixture) Available() bool { return true }
func (*Fixture) Simulated() bool { return true }
func (*Fixture) Grade(ctx context.Context, req entity.GradeRequest) (entity.GradeResult, error) {
	if err := ctx.Err(); err != nil {
		return manualFallback(FixtureVersion, true, "cancelled"), err
	}
	if err := ValidateRequest(req); err != nil {
		return manualFallback(FixtureVersion, true, "invalid_request"), err
	}
	result := entity.GradeResult{
		Status: StatusSuggested, ProviderVersion: FixtureVersion, Simulated: true,
		Suggestions: make([]entity.SuggestedGrade, 0, len(req.Items)),
	}
	for _, item := range req.Items {
		// A fixed midpoint per criterion exercises the review UI while remaining
		// inside each rubric bound. No answer or reference is evaluated at all.
		score := 0
		for _, criterion := range item.Rubric {
			score += criterion.MaxScore / 2
		}
		result.Suggestions = append(result.Suggestions, entity.SuggestedGrade{
			QuestionID: item.QuestionID, Score: score,
			Feedback: "SIMULATED fixture suggestion only. The answer was not evaluated; instructor review is required.",
		})
	}
	return AcceptResult(req, result)
}

func manualFallback(version string, simulated bool, code string) entity.GradeResult {
	if !safeID(version) {
		version = "unverified"
	}
	return entity.GradeResult{Status: StatusNeedsManual, Suggestions: []entity.SuggestedGrade{}, ProviderVersion: version, Simulated: simulated, ErrorCode: code}
}

// AcceptResult rejects the entire response on any error, never salvaging a
// partial grade. It returns only sanitized error codes, not provider payloads.
func AcceptResult(req entity.GradeRequest, result entity.GradeResult) (entity.GradeResult, error) {
	if err := ValidateRequest(req); err != nil {
		return manualFallback(result.ProviderVersion, result.Simulated, "invalid_request"), err
	}
	if err := ValidateResult(req, result); err != nil {
		return manualFallback(result.ProviderVersion, result.Simulated, "invalid_result"), err
	}
	if result.Suggestions == nil {
		result.Suggestions = []entity.SuggestedGrade{}
	}
	return result, nil
}

var _ Grader = Unconfigured{}
var _ Grader = (*Fixture)(nil)
