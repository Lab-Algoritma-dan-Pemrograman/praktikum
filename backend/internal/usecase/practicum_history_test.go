package usecase

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func TestPracticumTerminalHistoryReadableAfterEligibilityChange(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(map[bool]string{false: "submitted", true: "graded"}[published], func(t *testing.T) {
			h := newPracticumHarness(t, "pretest", "individual", false)
			a := h.start(t, 0)
			_, err := h.s.Submit(t.Context(), h.students[0], practicumCmd("submit-history"), a.AttemptID)
			if err != nil {
				t.Fatal(err)
			}
			if published {
				_, err = h.s.ManualGrade(t.Context(), h.admin, practicumCmd("publish-history"), a.AttemptID, PracticumManualInput{Grades: []entity.PracticumManualGrade{{QuestionID: h.questions[0].ID, Score: 5}, {QuestionID: h.questions[1].ID, Score: 6}}, Publish: true, Reason: "Synthetic history test"})
				if err != nil {
					t.Fatal(err)
				}
			}
			before := practicumRows(t, h.repo, repository.PracticumAttempts)
			h.students[0].Assignments = nil
			got, err := h.s.Attempt(t.Context(), h.students[0], a.AttemptID, practicumCmd("history-read"))
			if err != nil || got.AttemptID != a.AttemptID {
				t.Fatalf("own terminal history: %+v %v", got, err)
			}
			if !reflect.DeepEqual(before, practicumRows(t, h.repo, repository.PracticumAttempts)) {
				t.Fatal("terminal history read changed stored work")
			}
			if got.Questions[0].ReferenceAnswer != "" || len(got.Questions[0].Rubric) != 0 {
				t.Fatal("history leaked private grading rules")
			}
			if _, err = h.s.Attempt(t.Context(), h.students[1], a.AttemptID, practicumCmd("foreign-read")); !errors.Is(err, authorization.ErrNotFound) && !authorization.Is(err, "NOT_FOUND") {
				t.Fatalf("foreign history: %v", err)
			}
			if _, err = h.s.SaveAnswers(t.Context(), h.students[0], practicumCmd("history-write"), a.AttemptID, PracticumSaveInput{Answers: []PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "forbidden"}}}); !errors.Is(err, authorization.ErrNotFound) && !authorization.Is(err, "NOT_FOUND") {
				t.Fatalf("history eligibility relaxed for mutation: %v", err)
			}
		})
	}
}

func TestPracticumHistoryUsesActivationAncestryAndGraderScope(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.start(t, 0)
	_, err := h.s.Submit(t.Context(), h.students[0], practicumCmd("submit-ancestry"), a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	err = h.repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		row, err := tx.Get(repository.PracticumAttempts, a.AttemptID)
		if err != nil {
			return err
		}
		var record entity.PracticumAttemptRecord
		if err = json.Unmarshal(row.Data, &record); err != nil {
			return err
		}
		record.Attempt.SessionID = "legacy-session-wrong-activation-session-id"
		record.Scope.Shift = 2
		row.Scope = record.Scope
		row.Data, err = json.Marshal(record)
		if err != nil {
			return err
		}
		return tx.Put(repository.PracticumAttempts, row)
	})
	if err != nil {
		t.Fatal(err)
	}
	history, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 25})
	if err != nil || len(history) != 1 || history[0].SessionID != h.session.ID {
		t.Fatalf("wrong history ancestry: %+v %v", history, err)
	}
	staff := practicumStaff(h.activation.Scope, practicumManage, practicumGrade)
	wrong := h.activation.Scope
	wrong.Shift = 2
	laterStaff := practicumStaff(wrong, practicumManage, practicumGrade)
	if _, err = h.s.Review(t.Context(), staff, a.AttemptID, practicumCmd("review-parent")); err != nil {
		t.Fatalf("activation grader denied: %v", err)
	}
	if _, err = h.s.Review(t.Context(), laterStaff, a.AttemptID, practicumCmd("review-later")); !errors.Is(err, authorization.ErrNotFound) && !authorization.Is(err, "NOT_FOUND") {
		t.Fatalf("later membership grader allowed: %v", err)
	}
	rows, err := h.s.Monitoring(t.Context(), staff, h.session.ID, "", PracticumPage{Limit: 1})
	if err != nil || len(rows) != 1 || rows[0].SessionID != h.session.ID {
		t.Fatalf("monitoring canonical pagination: %+v %v", rows, err)
	}
	rows, err = h.s.Monitoring(t.Context(), laterStaff, h.session.ID, "", PracticumPage{Limit: 1})
	if err != nil || len(rows) != 0 {
		t.Fatalf("wrong shift monitoring: %+v %v", rows, err)
	}
}

func TestPracticumActiveHistoryDoesNotPromiseIneligibleDetail(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.start(t, 0)
	h.students[0].Assignments[0].Scope.Shift = 2
	if _, err := h.s.Attempt(t.Context(), h.students[0], a.AttemptID, practicumCmd("active-history")); !errors.Is(err, authorization.ErrNotFound) && !authorization.Is(err, "NOT_FOUND") {
		t.Fatalf("active scope bypass: %v", err)
	}
	rows, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 25})
	if err != nil || len(rows) != 1 || rows[0].Status != "in_progress" || rows[0].CanOpen {
		t.Fatalf("misleading active history: %+v %v", rows, err)
	}
}
