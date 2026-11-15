package usecase

import (
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

type entity_PracticumManualGradeStub = entity.PracticumManualGrade

func TestRekapListsTerminalAttemptsPerCourse(t *testing.T) {
	h := newPracticumHarness(t, "posttest", "shared", false)
	a := h.start(t, 0)
	if _, err := h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID); err != nil {
		t.Fatal(err)
	}
	rows, err := h.s.Rekap(t.Context(), h.admin, h.course.ID, PracticumPage{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].AttemptID != a.AttemptID || rows[0].Status != "submitted" || rows[0].Score != nil {
		t.Fatalf("rekap = %+v", rows)
	}
	// Students never get the recap.
	if _, err := h.s.Rekap(t.Context(), h.students[0], h.course.ID, PracticumPage{Limit: 50, Offset: 0}); err == nil {
		t.Fatal("student recap must be denied")
	}
	// Empty course filter invalid.
	if _, err := h.s.Rekap(t.Context(), h.admin, "", PracticumPage{Limit: 50, Offset: 0}); err == nil {
		t.Fatal("empty course_id must be rejected")
	}
}

func TestManualGradeStoresKeaktifan(t *testing.T) {
	h := newPracticumHarness(t, "posttest", "shared", false)
	a := h.start(t, 0)
	if _, err := h.s.Submit(t.Context(), h.students[0], practicumCmd("submit"), a.AttemptID); err != nil {
		t.Fatal(err)
	}
	nilai := 80
	review, err := h.s.ManualGrade(t.Context(), h.admin, practicumCmd("manual"), a.AttemptID, PracticumManualInput{
		GradeRevision: 0, Publish: true, Reason: "recap test",
		Grades:    []entity_PracticumManualGradeStub{{QuestionID: h.questions[0].ID, Score: 10, Feedback: "ok"}, {QuestionID: h.questions[1].ID, Score: 8, Feedback: "good"}},
		Keaktifan: &nilai,
	})
	if err != nil {
		t.Fatal(err)
	}
	if review.Keaktifan == nil || *review.Keaktifan != 80 {
		t.Fatalf("review keaktifan = %v", review.Keaktifan)
	}
	summaries, err := h.s.Grades(t.Context(), h.students[0], PracticumPage{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range summaries {
		if s.AttemptID == a.AttemptID {
			found = true
			if s.Keaktifan == nil || *s.Keaktifan != 80 {
				t.Fatalf("summary keaktifan = %v", s.Keaktifan)
			}
		}
	}
	if !found {
		t.Fatal("attempt missing from student grades")
	}
	// Out of range rejected.
	bad := 101
	if _, err := h.s.ManualGrade(t.Context(), h.admin, practicumCmd("manual-bad"), a.AttemptID, PracticumManualInput{
		GradeRevision: 1, Publish: true, Reason: "x",
		Grades:    []entity_PracticumManualGradeStub{{QuestionID: h.questions[0].ID, Score: 10, Feedback: ""}, {QuestionID: h.questions[1].ID, Score: 8, Feedback: ""}},
		Keaktifan: &bad,
	}); err == nil {
		t.Fatal("keaktifan 101 must be rejected")
	}
}
