package grading

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func ValidateRequest(req entity.GradeRequest) error {
	if !safeID(req.AttemptID) || len(req.Items) < 1 || len(req.Items) > MaxQuestions {
		return ErrInvalidRequest
	}
	seen := make(map[string]bool, len(req.Items))
	for _, item := range req.Items {
		if !safeID(item.QuestionID) || seen[item.QuestionID] ||
			(item.Type != "essay" && item.Type != "coding") ||
			!validText(item.Prompt, MaxPromptBytes) || strings.TrimSpace(item.Prompt) == "" ||
			!validText(item.Answer, MaxAnswerBytes) || !validText(item.ReferenceAnswer, MaxAnswerBytes) ||
			item.MaxScore < 1 || item.MaxScore > 100 || len(item.Rubric) < 1 || len(item.Rubric) > MaxCriteria {
			return ErrInvalidRequest
		}
		if item.Type == "coding" && item.Language != "c" && item.Language != "python" ||
			item.Type == "essay" && item.Language != "" {
			return ErrInvalidRequest
		}
		seen[item.QuestionID] = true
		criteria := make(map[string]bool, len(item.Rubric))
		sum := 0
		for _, criterion := range item.Rubric {
			if !safeID(criterion.ID) || criteria[criterion.ID] ||
				!validText(criterion.Description, MaxPromptBytes) || strings.TrimSpace(criterion.Description) == "" ||
				criterion.MaxScore < 0 || criterion.MaxScore > item.MaxScore {
				return ErrInvalidRequest
			}
			criteria[criterion.ID] = true
			sum += criterion.MaxScore
		}
		if sum != item.MaxScore {
			return ErrInvalidRequest
		}
	}
	encoded, err := json.Marshal(req)
	if err != nil || len(encoded) > MaxRequestBytes {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateResult(req entity.GradeRequest, result entity.GradeResult) error {
	if err := ValidateRequest(req); err != nil {
		return err
	}
	if !safeID(result.ProviderVersion) || !validErrorCode(result.ErrorCode) || len(result.Suggestions) > MaxQuestions {
		return ErrInvalidResult
	}
	switch result.Status {
	case StatusNeedsManual:
		if len(result.Suggestions) != 0 {
			return ErrInvalidResult
		}
	case StatusSuggested:
		if len(result.Suggestions) != len(req.Items) || result.ErrorCode != "" {
			return ErrInvalidResult
		}
	default:
		return ErrInvalidResult
	}
	bounds := make(map[string]int, len(req.Items))
	for _, item := range req.Items {
		bounds[item.QuestionID] = item.MaxScore
	}
	seen := make(map[string]bool, len(result.Suggestions))
	for _, suggestion := range result.Suggestions {
		max, ok := bounds[suggestion.QuestionID]
		if !ok || seen[suggestion.QuestionID] || suggestion.Score < 0 || suggestion.Score > max ||
			!validFeedback(suggestion.Feedback) {
			return ErrInvalidResult
		}
		seen[suggestion.QuestionID] = true
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > MaxResultBytes {
		return ErrInvalidResult
	}
	return nil
}

func validErrorCode(code string) bool {
	switch code {
	case "", "provider_not_configured", "provider_unavailable", "timeout", "cancelled", "invalid_request", "invalid_result":
		return true
	default:
		return false
	}
}

func validText(value string, max int) bool {
	return len(value) <= max && utf8.ValidString(value)
}

func validFeedback(value string) bool {
	if !validText(value, MaxFeedbackBytes) {
		return false
	}
	for _, ch := range value {
		if unicode.IsControl(ch) && ch != '\n' && ch != '\t' || unicode.Is(unicode.Cf, ch) {
			return false
		}
	}
	// Feedback is plain text; the UI must not insert it as HTML or Markdown HTML.
	return true
}

func safeID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("-_.:", ch)) {
			return false
		}
	}
	return true
}
