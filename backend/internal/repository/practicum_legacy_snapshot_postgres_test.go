package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func TestPostgresPracticumLegacySnapshotSurvivesSubmitAndManualGrade(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	_, err := h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("legacy-initial-answer"), a.AttemptID, usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "SYNTHETIC_LEGACY_ANSWER"}}})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(h.ctx, `alter table public.practicum_attempts disable trigger practicum_attempt_frozen`); err != nil {
		t.Fatal(err)
	}
	start := strings.TrimSuffix(a.StartedAt.UTC().Format(time.RFC3339Nano), "Z") + "+00:00"
	deadline := strings.TrimSuffix(a.DeadlineAt.UTC().Format(time.RFC3339Nano), "Z") + "+00:00"
	if _, err = tx.Exec(h.ctx, `update public.practicum_attempts set data=
 jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set(data,
 '{attempt,questions,0,legacy}','{"source_id":99}'),
 '{attempt,answers,0,legacy_id}','123'),
 '{attempt,started_at}',to_jsonb($2::text)),
 '{attempt,deadline_at}',to_jsonb($3::text)),
 '{legacy}','{"answer_ids":[123],"pengerjaan_course":{"id":88}}') where id=$1`, a.AttemptID, start, deadline); err != nil {
		t.Fatal(err)
	}
	legacyKey := "legacy:pengerjaan_course:" + a.AttemptID
	if _, err = tx.Exec(h.ctx, `update public.practicum_attempts set unique_key=$2 where id=$1`, a.AttemptID, legacyKey); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(h.ctx, `alter table public.practicum_attempts enable trigger practicum_attempt_frozen`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(h.ctx); err != nil {
		t.Fatal(err)
	}
	resumed, err := h.service.Start(h.ctx, h.students[0], pgPracticumCommand("legacy-resume-second-key"), usecase.PracticumStartInput{ActivationID: h.activation.ID, AccessToken: h.activation.AccessToken})
	if err != nil || resumed.AttemptID != a.AttemptID || !resumed.DeadlineAt.Equal(a.DeadlineAt) {
		t.Fatalf("legacy resume duplicated or reset work: %+v %v", resumed, err)
	}
	var beforeQuestions string
	if err = h.pool.QueryRow(h.ctx, `select (data->'attempt'->'questions')::text from public.practicum_attempts where id=$1`, a.AttemptID).Scan(&beforeQuestions); err != nil {
		t.Fatal(err)
	}
	_, err = h.service.Submit(h.ctx, h.students[0], pgPracticumCommand("legacy-submit"), a.AttemptID)
	if err != nil {
		t.Fatalf("legacy snapshot could not be submitted: %v", err)
	}
	_, err = h.service.ManualGrade(h.ctx, h.admin, pgPracticumCommand("legacy-manual-grade"), a.AttemptID, usecase.PracticumManualInput{Grades: []entity.PracticumManualGrade{{QuestionID: h.questions[0].ID, Score: 8}, {QuestionID: h.questions[1].ID, Score: 9}}, Publish: true, Reason: "Synthetic legacy preservation"})
	if err != nil {
		t.Fatalf("legacy snapshot could not be graded: %v", err)
	}
	var questions, storedStart, storedDeadline string
	var answerID, submittedID int
	var legacy bool
	if err = h.pool.QueryRow(h.ctx, `select (data->'attempt'->'questions')::text,
 data->'attempt'->>'started_at',data->'attempt'->>'deadline_at',
 (data->'attempt'->'answers'->0->>'legacy_id')::int,
 (data->'submitted_answers'->0->>'legacy_id')::int,
 data->'legacy'->'answer_ids'='[123]'::jsonb
 from public.practicum_attempts where id=$1`, a.AttemptID).Scan(&questions, &storedStart, &storedDeadline, &answerID, &submittedID, &legacy); err != nil {
		t.Fatal(err)
	}
	var storedKey string
	if err = h.pool.QueryRow(h.ctx, `select unique_key from public.practicum_attempts where id=$1`, a.AttemptID).Scan(&storedKey); err != nil || storedKey != legacyKey {
		t.Fatalf("legacy operation identity changed: %s %v", storedKey, err)
	}
	if questions != beforeQuestions || storedStart != start || storedDeadline != deadline || answerID != 123 || submittedID != 123 || !legacy {
		t.Fatal("domain write discarded immutable raw snapshot or legacy metadata")
	}
}
