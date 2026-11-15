package usecase

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func TestPracticumActivationPersistenceStartsSharedTimerWithoutLosingLegacyShape(t *testing.T) {
	raw := json.RawMessage(`{"legacy":{"activation_id":88},"activation":{"id":"a","session_id":"s","course_id":"c","scope":{},"timer_mode":"shared","is_open":true,"started_at":null,"deadline_at":null,"closes_at":"2026-01-01T00:30:00+00:00","question_count":1,"version":1,"student_ids":[],"entry_approved_ids":null},"course":{"id":"c","session_id":"s","legacy":{"source_id":10}},"questions":[{"id":"q","prompt":"immutable","legacy":{"source_id":99}}],"token_hash":"SYNTHETIC_HASH"}`)
	var next entity.PracticumActivationRecord
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	deadline := started.Add(10 * time.Minute)
	next.Activation.StartedAt, next.Activation.DeadlineAt = &started, &deadline
	next.Activation.Version++
	merged, err := practicumMergeActivationData(raw, next)
	if err != nil {
		t.Fatalf("initial shared timer rejected: %v", err)
	}
	var result, activation practicumRawObject
	if err = json.Unmarshal(merged, &result); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(result["activation"], &activation); err != nil {
		t.Fatal(err)
	}
	if string(activation["student_ids"]) != "[]" || string(activation["entry_approved_ids"]) != "null" {
		t.Fatal("omitempty changed the frozen roster JSON shape")
	}
	if _, exists := activation["question_ids"]; exists {
		t.Fatal("missing frozen question IDs were fabricated")
	}
	if string(activation["closes_at"]) != `"2026-01-01T00:30:00+00:00"` || len(result["legacy"]) == 0 {
		t.Fatal("immutable raw timestamp or legacy metadata changed")
	}
	next.Activation.IsOpen = false
	if _, err = practicumMergeActivationData(merged, next); err != nil {
		t.Fatalf("closing established shared timer rejected: %v", err)
	}
	changed := started.Add(time.Second)
	next.Activation.StartedAt = &changed
	if _, err = practicumMergeActivationData(merged, next); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("established timer rewrite was silently masked: %v", err)
	}
	next.Activation.StartedAt = &started
	next.Questions[0].Prompt = "illegal snapshot rewrite"
	if _, err = practicumMergeActivationData(merged, next); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("question snapshot rewrite was silently masked: %v", err)
	}
}
