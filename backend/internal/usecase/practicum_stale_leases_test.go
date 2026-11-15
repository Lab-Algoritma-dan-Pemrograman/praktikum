package usecase

import (
	"fmt"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func TestPracticumRunnerAdmitsPastOneThousandExpiredLeases(t *testing.T) {
	h := newPracticumHarness(t, "pretest", "individual", false)
	a := h.start(t, 0)
	h.s.options.Runner = &practicumTestRunner{}
	err := h.repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		for i := range 1000 {
			run := practicumRunRecord{ID: fmt.Sprintf("stale-%04d", i), AttemptID: a.AttemptID, UserID: a.UserID, Status: "running", LeaseUntil: h.clock.Now().Add(-time.Minute), Result: entity.RunResult{Status: "provider_error"}}
			if err := practicumSaveRun(tx, run, h.activation.Scope); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := h.s.Run(t.Context(), h.students[0], practicumCmd("past-stale-cap"), a.AttemptID, h.runInput())
	if err != nil || result.Status != "success" {
		t.Fatalf("expired leases permanently wedged admission: %+v %v", result, err)
	}
	if _, err = h.s.Reconcile(t.Context(), practicumCmd("cleanup-stale")); err != nil {
		t.Fatal(err)
	}
	var rows []repository.PracticumRecord
	if err = h.repo.Transaction(t.Context(), func(tx repository.PracticumTx) error {
		var e error
		rows, e = tx.List(repository.PracticumRuns, repository.PracticumFilter{Status: "running", Limit: 1000})
		return e
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 900 {
		t.Fatalf("cleanup must commit exactly a bounded batch: %d", len(rows))
	}
}
