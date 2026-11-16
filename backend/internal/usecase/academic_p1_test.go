package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type siakadP1Harness struct {
	ctx   context.Context
	repo  *repository.MemoryAcademicRepository
	svc   *usecase.AcademicService
	now   time.Time
	admin *entity.Principal
}

func siakadP1Command(key string) usecase.AcademicCommand { return usecase.AcademicCommand{Key: key} }

func newSiakadP1(t *testing.T) *siakadP1Harness {	t.Helper()
	h := &siakadP1Harness{ctx: context.Background(), now: time.Date(2026, 11, 16, 8, 0, 0, 0, time.UTC)}
	h.repo = repository.NewMemoryAcademicRepository(func() time.Time { return h.now })
	if err := repository.SeedAcademicFixtures(h.ctx, h.repo); err != nil {
		t.Fatal(err)
	}
	h.svc = usecase.NewAcademicService(usecase.AcademicOptions{Repository: h.repo,
		IdentityRepository: repository.NewMemoryIdentityRepository()})
	h.admin = academicRegressionPrincipal("class.manage", "schedule.write", "attendance.read",
		"inventory.manage", "rental.approve", "leave.decide", "finance.manage")
	return h
}

func TestSiakadP1LeaveRequestLifecycle(t *testing.T) {
	h := newSiakadP1(t)
	student := &entity.Principal{User: entity.UserProfile{ID: 1, Active: true, Name: "Student"}}

	req, err := h.svc.CreateLeave(h.ctx, student, siakadP1Command("leave-1"),
		usecase.AcademicLeaveCreateInput{ScheduleID: "schedule-1", Kind: "izin",
			Reason: "Ada acara keluarga.", OriginalAt: h.now})
	if err != nil {
		t.Fatalf("create leave: %v", err)
	}
	if req.Status != "pending" || req.Kind != "izin" || req.UserID != 1 {
		t.Fatalf("unexpected leave request %+v", req)
	}

	// Idempotency replay returns the same row, not a duplicate.
	replay, err := h.svc.CreateLeave(h.ctx, student, siakadP1Command("leave-1"),
		usecase.AcademicLeaveCreateInput{ScheduleID: "schedule-1", Kind: "izin",
			Reason: "Ada acara keluarga.", OriginalAt: h.now})
	if err != nil || replay.ID != req.ID {
		t.Fatalf("idempotency replay mismatch: %v %+v", err, replay)
	}

	// A student may not decide their own request.
	if _, err := h.svc.DecideLeave(h.ctx, student, siakadP1Command("decide-1"), req.ID,
		usecase.AcademicLeaveDecideInput{Version: 1, Action: "approve"}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected student denial, got %v", err)
	}

	decided, err := h.svc.DecideLeave(h.ctx, h.admin, siakadP1Command("decide-2"), req.ID,
		usecase.AcademicLeaveDecideInput{Version: 1, Action: "approve"})
	if err != nil {
		t.Fatalf("decide leave: %v", err)
	}
	if decided.Status != "approved" || decided.DecidedBy == nil || *decided.DecidedBy != 2 {
		t.Fatalf("unexpected decided request %+v", decided)
	}
}

func TestSiakadP1RescheduleRequiresTargetTime(t *testing.T) {
	h := newSiakadP1(t)
	student := &entity.Principal{User: entity.UserProfile{ID: 1, Active: true, Name: "Student"}}
	req, err := h.svc.CreateLeave(h.ctx, student, siakadP1Command("resched-1"),
		usecase.AcademicLeaveCreateInput{ScheduleID: "schedule-1", Kind: "reschedule",
			Reason: "Bentrok jadwal.", OriginalAt: h.now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DecideLeave(h.ctx, h.admin, siakadP1Command("resched-2"), req.ID,
		usecase.AcademicLeaveDecideInput{Version: 1, Action: "reschedule"}); !errors.Is(err, usecase.ErrValidation) {
		t.Fatalf("expected validation error for missing reschedule_at, got %v", err)
	}
}

func TestSiakadP1FinanceBalanceAndAdminPost(t *testing.T) {
	h := newSiakadP1(t)
	student := &entity.Principal{User: entity.UserProfile{ID: 1, Active: true, Name: "Student"}}

	if _, err := h.svc.PostFinanceMutation(h.ctx, student, siakadP1Command("fin-denied"),
		usecase.AcademicFinanceMutationInput{TargetUserID: 1, Type: "income", Note: "setoran", Amount: 1000}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected student denial on finance post, got %v", err)
	}

	income, err := h.svc.PostFinanceMutation(h.ctx, h.admin, siakadP1Command("fin-1"),
		usecase.AcademicFinanceMutationInput{TargetUserID: 1, Type: "income", Note: "Deposit kas", Amount: 50000, Category: "Kas"})
	if err != nil {
		t.Fatalf("post income: %v", err)
	}
	if _, err := h.svc.PostFinanceMutation(h.ctx, h.admin, siakadP1Command("fin-2"),
		usecase.AcademicFinanceMutationInput{TargetUserID: 1, Type: "expense", Note: "Sewa alat", Amount: 25000, Category: "Alat"}); err != nil {
		t.Fatalf("post expense: %v", err)
	}

	balance, err := h.svc.FinanceBalance(h.ctx, student, usecase.PracticumPage{Limit: 25, Offset: 0})
	if err != nil {
		t.Fatalf("finance balance: %v", err)
	}
	if balance.Saldo != 25000 || balance.Income != 50000 || balance.Expense != 25000 || len(balance.Mutations) != 2 {
		t.Fatalf("unexpected balance %+v", balance)
	}

	// Idempotency replay must not double-post income.
	replay, err := h.svc.PostFinanceMutation(h.ctx, h.admin, siakadP1Command("fin-1"),
		usecase.AcademicFinanceMutationInput{TargetUserID: 1, Type: "income", Note: "Deposit kas", Amount: 50000, Category: "Kas"})
	if err != nil || replay.ID != income.ID {
		t.Fatalf("finance idempotency replay mismatch: %v %+v", err, replay)
	}
	balance, _ = h.svc.FinanceBalance(h.ctx, student, usecase.PracticumPage{Limit: 25, Offset: 0})
	if balance.Saldo != 25000 || len(balance.Mutations) != 2 {
		t.Fatalf("replay changed saldo: %+v", balance)
	}
}

func TestSiakadP1AdminLeaveListScopesDeniedWithoutPermission(t *testing.T) {
	h := newSiakadP1(t)
	plain := academicRegressionPrincipal("learning.monitor")
	rows, err := h.svc.LeaveRequests(h.ctx, plain, usecase.PracticumPage{Limit: 25, Offset: 0})
	if err != nil {
		t.Fatalf("leave list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows without leave scope, got %d", len(rows))
	}
}
