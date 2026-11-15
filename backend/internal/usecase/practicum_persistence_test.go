package usecase

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func TestPracticumPersistencePreservesLegacyIdentityAndExplicitClears(t *testing.T) {
	raw := json.RawMessage(`{"legacy":{"answer_ids":[11,12]},"submitted_answers":[],"attempt":{"attempt_id":"a","activation_id":"activation","course_id":"course","user_id":1,"status":"in_progress","started_at":"2026-01-01T00:00:00+00:00","deadline_at":"2026-01-01T00:30:00+00:00","questions":[{"id":"q","prompt":"immutable","legacy":{"source_id":9}}],"answers":[{"question_id":"q","value":"first","language":"c","score":1,"feedback":"old","legacy_id":11,"updated_at":"2026-01-01T00:01:00+00:00"},{"question_id":"q","value":"second","legacy_id":12,"updated_at":"2026-01-01T00:01:00+00:00"}]}}`)
	var next entity.PracticumAttemptRecord
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	next.Attempt.Answers[0].Value = "edited"
	next.Attempt.Answers[0].Language = ""
	next.Attempt.Answers[0].Feedback = ""
	next.Attempt.Answers[0].Score = nil
	merged, err := practicumMergeAttemptData(raw, next)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Legacy struct {
			AnswerIDs []int `json:"answer_ids"`
		} `json:"legacy"`
		Attempt struct {
			StartedAt string `json:"started_at"`
			Answers   []struct {
				LegacyID                  int `json:"legacy_id"`
				Value, Language, Feedback string
				Score                     *int
				UpdatedAt                 string `json:"updated_at"`
			} `json:"answers"`
		} `json:"attempt"`
	}
	if err = json.Unmarshal(merged, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Legacy.AnswerIDs) != 2 || len(result.Attempt.Answers) != 2 || result.Attempt.Answers[0].LegacyID != 11 || result.Attempt.Answers[1].LegacyID != 12 {
		t.Fatal("duplicate historical question identities were lost")
	}
	first := result.Attempt.Answers[0]
	if first.Value != "edited" || first.Language != "" || first.Feedback != "" || first.Score != nil {
		t.Fatal("omitted fields resurrected prior data")
	}
	if first.UpdatedAt != "2026-01-01T00:01:00+00:00" || result.Attempt.StartedAt != "2026-01-01T00:00:00+00:00" {
		t.Fatal("equivalent timestamps changed immutable raw JSON")
	}
	next.Attempt.Questions[0].Prompt = "illegal snapshot rewrite"
	if _, err = practicumMergeAttemptData(raw, next); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("immutable rewrite was silently masked: %v", err)
	}
}
