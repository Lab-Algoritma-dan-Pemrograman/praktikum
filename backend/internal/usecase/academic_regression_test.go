package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type academicRegressionHarness struct {
	ctx            context.Context
	repo           *repository.MemoryAcademicRepository
	svc            *usecase.AcademicService
	now            time.Time
	admin, student *entity.Principal
}

func newAcademicRegression(t *testing.T) *academicRegressionHarness {
	t.Helper()
	h := &academicRegressionHarness{ctx: context.Background(), now: time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	h.repo = repository.NewMemoryAcademicRepository(func() time.Time { return h.now })
	if err := repository.SeedAcademicFixtures(h.ctx, h.repo); err != nil {
		t.Fatal(err)
	}
	h.svc = usecase.NewAcademicService(usecase.AcademicOptions{Repository: h.repo})
	h.admin = academicRegressionPrincipal("class.manage", "schedule.write", "attendance.read", "inventory.manage", "rental.approve", "finance.manage")
	h.student = &entity.Principal{User: entity.UserProfile{ID: 1, Active: true, Name: "Student"}}
	return h
}

func academicRegressionPrincipal(permissions ...string) *entity.Principal {
	p := &entity.Principal{User: entity.UserProfile{ID: 2, Active: true, Name: "Staff"}, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Active: true, Scope: entity.Scope{Domain: "siakad"}}}}
	for _, perm := range permissions {
		p.Grants = append(p.Grants, entity.PermissionGrant{Permission: perm, Role: entity.RoleKoordinator, Scope: entity.Scope{Domain: "siakad", Institution: "lab"}})
	}
	return p
}

func regressionCommand(key string) usecase.AcademicCommand { return usecase.AcademicCommand{Key: key} }

func (h *academicRegressionHarness) rental(t *testing.T, key string) entity.InventoryRental {
	t.Helper()
	r, err := h.svc.RequestRental(h.ctx, h.student, regressionCommand(key), usecase.InventoryRentalCreateInput{ItemID: "item-arduino", Quantity: 1, Purpose: "Regression", DueAt: h.now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAcademicCheckInRechecksDeadlineTokenAndScope(t *testing.T) {
	for _, mode := range []string{"expired", "closed", "invalid-token"} {
		t.Run(mode, func(t *testing.T) {
			h := newAcademicRegression(t)
			opened, err := h.svc.OpenAttendanceSession(h.ctx, h.admin, regressionCommand("open"), usecase.AcademicAttendanceOpenInput{ScheduleID: "schedule-1", Title: "Regression", DurationMinutes: 1})
			if err != nil {
				t.Fatal(err)
			}
			in := usecase.AcademicAttendanceCheckinInput{SessionID: opened.ID, QRToken: opened.QRToken}
			rec, err := h.svc.CheckInAttendance(h.ctx, h.student, regressionCommand("check"), in)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Scope.SessionID != opened.ID {
				t.Errorf("attendance scope lacks session identity: %+v", rec.Scope)
			}
			want := usecase.ErrAttendanceClosed
			switch mode {
			case "expired":
				h.now = opened.ExpiresAt
			case "closed":
				if _, err := h.svc.CloseAttendanceSession(h.ctx, h.admin, regressionCommand("close"), opened.ID); err != nil {
					t.Fatal(err)
				}
			case "invalid-token":
				in.QRToken = "invalid-token"
				want = authorization.ErrForbidden
			}
			keys := []string{"new-check"}
			if mode != "invalid-token" {
				keys = append(keys, "check")
			}
			for _, key := range keys {
				if _, err := h.svc.CheckInAttendance(h.ctx, h.student, regressionCommand(key), in); !errors.Is(err, want) {
					t.Errorf("%s replay returned %v, want %v", key, err, want)
				}
			}
		})
	}
}

func TestAcademicPaymentEligibilityRevenueAndReturnPermissions(t *testing.T) {
	h := newAcademicRegression(t)
	finance := academicRegressionPrincipal("finance.manage")
	inventory := academicRegressionPrincipal("inventory.manage")
	r := h.rental(t, "requested")
	if _, err := h.svc.DecideRental(h.ctx, finance, regressionCommand("pay-requested"), r.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "verify_payment"}); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("requested payment accepted: %v", err)
	}
	rejected := h.rental(t, "rejected")
	rejected, err := h.svc.DecideRental(h.ctx, h.admin, regressionCommand("reject"), rejected.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "reject"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DecideRental(h.ctx, finance, regressionCommand("pay-rejected"), rejected.ID, usecase.InventoryRentalDecideInput{Version: rejected.Version, Action: "verify_payment"}); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("rejected payment accepted: %v", err)
	}
	approved := h.rental(t, "approved")
	approved, err = h.svc.DecideRental(h.ctx, h.admin, regressionCommand("approve"), approved.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DecideRental(h.ctx, inventory, regressionCommand("inventory-return"), approved.ID, usecase.InventoryRentalDecideInput{Version: approved.Version, Action: "return"}); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("inventory-only actor returned rental: %v", err)
	}
	approved, err = h.svc.DecideRental(h.ctx, h.admin, regressionCommand("return"), approved.ID, usecase.InventoryRentalDecideInput{Version: approved.Version, Action: "return"})
	if err != nil {
		t.Fatal(err)
	}
	paid, err := h.svc.DecideRental(h.ctx, finance, regressionCommand("pay-returned"), approved.ID, usecase.InventoryRentalDecideInput{Version: approved.Version, Action: "verify_payment"})
	if err != nil || !paid.PaymentVerified {
		t.Fatalf("returned payment rejected: %+v %v", paid, err)
	}
	rep, err := h.svc.Reports(h.ctx, finance)
	if err != nil || rep.TotalVerifiedRevenue != paid.TotalFee {
		t.Fatalf("finance report: %+v %v", rep, err)
	}
	for _, actor := range []*entity.Principal{inventory, academicRegressionPrincipal("schedule.write"), academicRegressionPrincipal("rental.approve")} {
		rep, err = h.svc.Reports(h.ctx, actor)
		if err != nil || rep.TotalVerifiedRevenue != 0 || !strings.Contains(rep.CSVExport, "total_verified_revenue,0\n") {
			t.Errorf("non-finance revenue exposed: %+v %v", rep, err)
		}
	}
	partial := academicRegressionPrincipal("inventory.manage", "finance.manage")
	partial.Grants[1].Scope.Institution = "other"
	rep, err = h.svc.Reports(h.ctx, partial)
	if err != nil || rep.TotalVerifiedRevenue != 0 {
		t.Errorf("out-of-scope finance revenue exposed: %+v %v", rep, err)
	}
}

func TestAcademicStockEditCannotExposeApprovedUnits(t *testing.T) {
	h := newAcademicRegression(t)
	r := h.rental(t, "request")
	if _, err := h.svc.DecideRental(h.ctx, h.admin, regressionCommand("approve"), r.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "approve"}); err != nil {
		t.Fatal(err)
	}
	in := usecase.InventoryItemUpdate{Version: 2, Name: "Arduino", Category: "Electronics", TotalQuantity: 5, AvailableQuantity: 5, RentalFee: 25000, Status: "available"}
	if _, err := h.svc.UpdateInventoryItem(h.ctx, h.admin, regressionCommand("inflate"), r.ItemID, in); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("stock edit exposed checked-out units: %v", err)
	}
	in.AvailableQuantity = 4
	if _, err := h.svc.UpdateInventoryItem(h.ctx, h.admin, regressionCommand("valid-edit"), r.ItemID, in); err != nil {
		t.Errorf("valid stock edit rejected: %v", err)
	}
}

func TestAcademicClassAuthorizationUsesFinalResourceScope(t *testing.T) {
	h := newAcademicRegression(t)
	actor := academicRegressionPrincipal("class.manage", "schedule.write")
	for i := range actor.Grants {
		actor.Grants[i].Scope.Shift = 1
	}
	in := usecase.AcademicClassInput{Code: "SHIFT-2", CourseCode: "IF", CourseName: "Course", PeriodName: "Period", Shift: 2, Scope: entity.Scope{Domain: "siakad", Institution: "lab", Shift: 1}}
	if _, err := h.svc.CreateClass(h.ctx, actor, regressionCommand("wrong-shift"), in); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("create allowed forged shift: %v", err)
	}
	actor.Grants[0].Scope.ClassID = "class-101"
	in.Shift = 1
	in.Scope.ClassID = "class-101"
	if _, err := h.svc.CreateClass(h.ctx, actor, regressionCommand("wrong-class"), in); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("create allowed forged class ID: %v", err)
	}
	update := usecase.AcademicClassUpdate{Version: 1, Code: "IF-101-A", CourseCode: "IF1101", CourseName: "Course", PeriodName: "Period", Shift: 2, StudentIDs: []int64{1}, Active: true}
	if _, err := h.svc.UpdateClass(h.ctx, actor, regressionCommand("move-shift"), "class-101", update); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("update moved class outside grant: %v", err)
	}
	if _, err := h.svc.CreateSchedule(h.ctx, actor, regressionCommand("schedule-shift"), usecase.AcademicScheduleInput{ClassID: "class-101", DayOfWeek: "Senin", Shift: 2, Room: "Lab", StartsAt: "08:00 WIB", EndsAt: "10:00 WIB"}); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("schedule created outside grant: %v", err)
	}
}

func TestAcademicReadOnlyAttendanceCannotManageSessions(t *testing.T) {
	h := newAcademicRegression(t)
	reader := academicRegressionPrincipal("attendance.read")
	if _, err := h.svc.OpenAttendanceSession(h.ctx, reader, regressionCommand("reader-open"), usecase.AcademicAttendanceOpenInput{ScheduleID: "schedule-1", Title: "Denied", DurationMinutes: 1}); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("attendance reader opened session: %v", err)
	}
	opened, err := h.svc.OpenAttendanceSession(h.ctx, h.admin, regressionCommand("open"), usecase.AcademicAttendanceOpenInput{ScheduleID: "schedule-1", Title: "Open", DurationMinutes: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.CloseAttendanceSession(h.ctx, reader, regressionCommand("reader-close"), opened.ID); !errors.Is(err, authorization.ErrForbidden) {
		t.Errorf("attendance reader closed session: %v", err)
	}
}

func TestAcademicStudentSchedulesPageEligibleRowsBeforeLimit(t *testing.T) {
	h := newAcademicRegression(t)
	if err := h.repo.Transaction(h.ctx, func(tx repository.AcademicTx) error {
		other := entity.AcademicClass{ID: "other-class", Active: true, StudentIDs: []int64{99}}
		raw, _ := json.Marshal(other)
		if err := tx.Put(repository.AcademicClasses, repository.AcademicRecord{ID: other.ID, Data: raw}); err != nil {
			return err
		}
		for i := range 205 {
			sched := entity.AcademicSchedule{ID: fmt.Sprintf("a-other-%03d", i), ClassID: other.ID}
			raw, _ = json.Marshal(sched)
			if err := tx.Put(repository.AcademicSchedules, repository.AcademicRecord{ID: sched.ID, ParentID: other.ID, Data: raw}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := h.svc.Schedules(h.ctx, h.student, false, usecase.PracticumPage{Limit: 25})
	if err != nil || len(rows) != 1 || rows[0].Schedule.ID != "schedule-1" {
		t.Fatalf("eligible schedule hidden behind unrelated rows: %+v %v", rows, err)
	}
}
