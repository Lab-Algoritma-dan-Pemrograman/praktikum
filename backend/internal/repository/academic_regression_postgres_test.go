package repository_test

import (
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

func TestPostgresAcademicLifecycleRegression(t *testing.T) {
	h := newPGAcademic(t)
	reader := *h.admin
	reader.Grants = []entity.PermissionGrant{h.admin.Grants[2]}
	if _, err := h.service.OpenAttendanceSession(h.ctx, &reader, pgAcademicCommand("read-only-open"), usecase.AcademicAttendanceOpenInput{ScheduleID: h.schedule.ID, Title: "Denied", DurationMinutes: 1}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("read-only actor opened attendance session: %v", err)
	}

	checked, err := h.service.CheckInAttendance(h.ctx, h.students[0], pgAcademicCommand("check"), usecase.AcademicAttendanceCheckinInput{SessionID: h.attSession.ID, QRToken: h.attSession.QRToken})
	if err != nil || checked.Scope.SessionID != h.attSession.ID {
		t.Fatalf("check-in scope: %+v %v", checked, err)
	}
	validated, err := h.service.ValidateAttendance(h.ctx, h.admin, pgAcademicCommand("validate"), checked.ID, usecase.AcademicAttendanceValidateInput{Version: checked.Version, Status: "hadir", Validated: true})
	if err != nil || !validated.Validated {
		t.Fatalf("validation failed: %+v %v", validated, err)
	}
	if _, err := h.service.CloseAttendanceSession(h.ctx, h.admin, pgAcademicCommand("close"), h.attSession.ID); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"check", "new-check"} {
		if _, err := h.service.CheckInAttendance(h.ctx, h.students[0], pgAcademicCommand(key), usecase.AcademicAttendanceCheckinInput{SessionID: h.attSession.ID, QRToken: h.attSession.QRToken}); !errors.Is(err, usecase.ErrAttendanceClosed) {
			t.Fatalf("closed session duplicate accepted: %v", err)
		}
	}

	rental, err := h.service.RequestRental(h.ctx, h.students[0], pgAcademicCommand("rental"), usecase.InventoryRentalCreateInput{ItemID: h.item.ID, Quantity: 1, Purpose: "Lifecycle regression", DueAt: time.Now().UTC().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("early-payment"), rental.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "verify_payment"}); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("requested payment accepted: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx, `update public.academic_inventory_rentals set data=jsonb_set(data,'{payment_verified}','true') where id=$1`, rental.ID); err == nil {
		t.Fatal("database accepted requested payment")
	}
	if _, err := h.pool.Exec(h.ctx, `update public.academic_inventory_rentals set status='returned' where id=$1`, rental.ID); err == nil {
		t.Fatal("database accepted requested -> returned")
	}
	rental, err = h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("approve"), rental.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.UpdateInventoryItem(h.ctx, h.admin, pgAcademicCommand("inflate-stock"), h.item.ID, usecase.InventoryItemUpdate{Version: 2, Name: h.item.Name, Category: h.item.Category, TotalQuantity: 2, AvailableQuantity: 2, RentalFee: h.item.RentalFee, Status: "available"}); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("stock edit exposed approved units: %v", err)
	}
	rental, err = h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("return"), rental.ID, usecase.InventoryRentalDecideInput{Version: rental.Version, Action: "return"})
	if err != nil {
		t.Fatal(err)
	}
	returnedVersion := rental.Version
	rental, err = h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("payment"), rental.ID, usecase.InventoryRentalDecideInput{Version: rental.Version, Action: "verify_payment"})
	if err != nil || !rental.PaymentVerified || rental.Status != "returned" {
		t.Fatalf("returned payment: %+v %v", rental, err)
	}
	replay, err := h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("payment"), rental.ID, usecase.InventoryRentalDecideInput{Version: returnedVersion, Action: "verify_payment"})
	if err != nil || replay.Version != rental.Version {
		t.Fatalf("payment replay duplicated mutation: %+v %v", replay, err)
	}
	for _, statement := range []string{
		`update public.academic_inventory_rentals set data=jsonb_set(data,'{returned_at}','"2099-01-01T00:00:00Z"') where id=$1`,
		`update public.academic_inventory_rentals set data=jsonb_set(data,'{purpose}','"changed"') where id=$1`,
		`update public.academic_inventory_rentals set data=jsonb_set(data,'{payment_verified}','false') where id=$1`,
	} {
		if _, err := h.pool.Exec(h.ctx, statement, rental.ID); err == nil {
			t.Fatal("database accepted terminal rental mutation")
		}
	}
	if _, err := h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("return-again"), rental.ID, usecase.InventoryRentalDecideInput{Version: rental.Version, Action: "return"}); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("second return accepted: %v", err)
	}
	items, err := h.service.InventoryItems(h.ctx, h.students[0], usecase.PracticumPage{Limit: 25})
	if err != nil || len(items) != 1 || items[0].AvailableQuantity != 2 {
		t.Fatalf("return did not restore stock exactly once: %+v %v", items, err)
	}
	report, err := h.service.Reports(h.ctx, &reader)
	if err != nil || report.TotalVerifiedRevenue != 0 || !strings.Contains(report.CSVExport, "total_verified_revenue,0\n") {
		t.Fatalf("non-finance revenue leaked: %+v %v", report, err)
	}
	report, err = h.service.Reports(h.ctx, h.admin)
	if err != nil || report.TotalVerifiedRevenue != rental.TotalFee {
		t.Fatalf("finance total: %+v %v", report, err)
	}
	notes, err := h.service.Notifications(h.ctx, h.students[0], usecase.PracticumPage{Limit: 25})
	if err != nil || len(notes) != 4 {
		t.Fatalf("validation/approve/return/payment notifications: %+v %v", notes, err)
	}
	otherNotes, err := h.service.Notifications(h.ctx, h.students[1], usecase.PracticumPage{Limit: 25})
	if err != nil || len(otherNotes) != 0 {
		t.Fatalf("notification ownership leaked: %+v %v", otherNotes, err)
	}
	var audits int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.audit_logs where subject_id=$1 and action='inventory.rental.verify_payment'`, rental.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("payment audit replay: %d %v", audits, err)
	}
}

func TestPostgresAcademicMaximumRentalFee(t *testing.T) {
	h := newPGAcademic(t)
	item, err := h.service.CreateInventoryItem(h.ctx, h.admin, pgAcademicCommand("large-item"), usecase.InventoryItemInput{Code: "INV-LARGE-FEE", Name: "Large fee item", Category: "Tools", TotalQuantity: 1000, AvailableQuantity: 1000, RentalFee: 100000000, Status: "available", Scope: h.item.Scope})
	if err != nil {
		t.Fatal(err)
	}
	rental, err := h.service.RequestRental(h.ctx, h.students[0], pgAcademicCommand("large-rental"), usecase.InventoryRentalCreateInput{ItemID: item.ID, Quantity: 1000, Purpose: "Maximum valid fee", DueAt: time.Now().UTC().Add(time.Hour)})
	if err != nil || rental.TotalFee != 100000000000 {
		t.Fatalf("valid fee product rejected: %+v %v", rental, err)
	}
	rental, err = h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("large-approve"), rental.ID, usecase.InventoryRentalDecideInput{Version: 1, Action: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.DecideRental(h.ctx, h.admin, pgAcademicCommand("large-pay"), rental.ID, usecase.InventoryRentalDecideInput{Version: rental.Version, Action: "verify_payment"}); err != nil {
		t.Fatal(err)
	}
	report, err := h.service.Reports(h.ctx, h.admin)
	if err != nil || report.TotalVerifiedRevenue != 100000000000 {
		t.Fatalf("large report total: %+v %v", report, err)
	}
}

func TestPostgresAcademicStudentPaginationAndScope(t *testing.T) {
	h := newPGAcademic(t)
	otherClass, err := h.service.CreateClass(h.ctx, h.admin, pgAcademicCommand("other-class"), usecase.AcademicClassInput{Code: "OTHER", CourseCode: "IF", CourseName: "Other", PeriodName: "2026", Shift: 1, StudentIDs: []int64{h.students[1].User.ID}, Scope: h.class.Scope})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `insert into public.academic_schedules(id,parent_id,scope,data)
	 select 'a-unrelated-'||i::text,$1,$2::jsonb,jsonb_build_object('id','a-unrelated-'||i::text,'class_id',$1::text,'shift',1,'day_of_week','Senin','version',1) from generate_series(1,205) i`, otherClass.ID, fmt.Sprintf(`{"domain":"siakad","institution":"synthetic-siakad","class_id":%q,"shift":1}`, otherClass.ID)); err != nil {
		t.Fatal(err)
	}
	rows, err := h.service.Schedules(h.ctx, h.students[0], false, usecase.PracticumPage{Limit: 25})
	if err != nil || len(rows) != 1 || rows[0].Schedule.ID != h.schedule.ID {
		t.Fatalf("eligible schedule hidden: %+v %v", rows, err)
	}
	if rows[0].OpenSession == nil || rows[0].OpenSession.QRToken != "" || rows[0].OpenSession.Scope.SessionID != h.attSession.ID {
		t.Fatalf("session disclosure/scope: %+v", rows[0].OpenSession)
	}

	scoped := *h.admin
	scoped.Grants = append([]entity.PermissionGrant(nil), h.admin.Grants...)
	for i := range scoped.Grants {
		scoped.Grants[i].Scope.Shift = 1
	}
	if _, err := h.service.CreateClass(h.ctx, &scoped, pgAcademicCommand("wrong-shift"), usecase.AcademicClassInput{Code: "SHIFT-2", CourseCode: "IF", CourseName: "Other", PeriodName: "2026", Shift: 2, Scope: h.class.Scope}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("forged class scope accepted: %v", err)
	}
	if _, err := h.service.CreateSchedule(h.ctx, &scoped, pgAcademicCommand("wrong-schedule-shift"), usecase.AcademicScheduleInput{ClassID: h.class.ID, Shift: 2, DayOfWeek: "Senin", Room: "Lab", StartsAt: "08:00 WIB", EndsAt: "10:00 WIB"}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("schedule shift escaped scope: %v", err)
	}
	if _, err := h.service.UpdateClass(h.ctx, h.admin, pgAcademicCommand("deactivate"), h.class.ID, usecase.AcademicClassUpdate{Version: 1, Code: h.class.Code, CourseCode: h.class.CourseCode, CourseName: h.class.CourseName, PeriodName: h.class.PeriodName, Shift: 1, StudentIDs: h.class.StudentIDs, Active: false}); err != nil {
		t.Fatal(err)
	}
	rows, err = h.service.Schedules(h.ctx, h.students[0], false, usecase.PracticumPage{Limit: 25})
	if err != nil || len(rows) != 0 {
		t.Fatalf("inactive class schedule visible: %+v %v", rows, err)
	}
}

func TestPostgresAcademicReportsCountAllPages(t *testing.T) {
	h := newPGAcademic(t)
	if _, err := h.pool.Exec(h.ctx, `insert into public.academic_inventory_items(id,scope,status,data)
	 select 'report-item-'||i::text,$1::jsonb,'available',jsonb_build_object('total_quantity',1,'available_quantity',1,'rental_fee',0,'version',1) from generate_series(1,1001) i`, `{"domain":"siakad","institution":"synthetic-siakad"}`); err != nil {
		t.Fatal(err)
	}
	report, err := h.service.Reports(h.ctx, h.admin)
	if err != nil || report.TotalInventoryItems != 1002 || report.AvailableInventoryUnit != 1003 {
		t.Fatalf("report silently truncated: %+v %v", report, err)
	}
}
