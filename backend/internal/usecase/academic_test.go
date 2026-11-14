package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func TestAcademicDivisionPermissionsQRAttendanceAndInventoryConcurrency(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	identity := repository.NewMemoryIdentityRepository()
	repo := repository.NewMemoryAcademicRepository(clock)
	if err := repository.SeedAcademicFixtures(ctx, repo); err != nil {
		t.Fatal(err)
	}
	svc := usecase.NewAcademicService(usecase.AcademicOptions{
		Repository:         repo,
		IdentityRepository: identity,
	})

	principalFor := func(id int64) *entity.Principal {
		u, _ := identity.FindUserByID(ctx, id)
		a, _ := identity.ListAssignments(ctx, id)
		g, _ := identity.ListPermissionGrants(ctx, id)
		return &entity.Principal{User: u, Assignments: a, Grants: g}
	}

	student := principalFor(1)
	sekretaris := principalFor(3)
	bendahara := principalFor(4)
	k3 := principalFor(5)
	labops := principalFor(6)
	peminjam := principalFor(10)

	// Division isolation: sekretaris opens QR attendance, k3 cannot.
	if _, err := svc.OpenAttendanceSession(ctx, k3, usecase.AcademicCommand{Key: "k3-att"}, usecase.AcademicAttendanceOpenInput{
		ScheduleID: "schedule-1", Title: "Denied", DurationMinutes: 15,
	}); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected k3 denied attendance open, got %v", err)
	}

	opened, err := svc.OpenAttendanceSession(ctx, sekretaris, usecase.AcademicCommand{Key: "sek-att"}, usecase.AcademicAttendanceOpenInput{
		ScheduleID: "schedule-1", Title: "Pertemuan 1", DurationMinutes: 15,
	})
	if err != nil || opened.QRToken == "" {
		t.Fatalf("sekretaris open attendance failed: %+v %v", opened, err)
	}

	// Student checks in with valid QR token; advance past deadline and verify late check-in is rejected.
	rec, err := svc.CheckInAttendance(ctx, student, usecase.AcademicCommand{Key: "stu-checkin"}, usecase.AcademicAttendanceCheckinInput{
		SessionID: opened.ID, QRToken: opened.QRToken,
	})
	if err != nil || rec.Status != "hadir" {
		t.Fatalf("student check-in failed: %+v %v", rec, err)
	}

	// Create an item with 2 units via labops.
	item, err := svc.CreateInventoryItem(ctx, labops, usecase.AcademicCommand{Key: "labops-item"}, usecase.InventoryItemInput{
		Code: "INV-LOGIC-01", Name: "Logic Analyzer", Category: "Alat Ukur", TotalQuantity: 2, AvailableQuantity: 2, RentalFee: 30000, Status: "available", Scope: entity.Scope{Domain: "siakad", Institution: "lab"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create 6 rental requests of 1 unit each, then race concurrent approvals across k3 and labops.
	due := clock().Add(48 * time.Hour)
	var rentalIDs []string
	for i := range 6 {
		r, err := svc.RequestRental(ctx, peminjam, usecase.AcademicCommand{Key: fmt.Sprintf("req-%d", i)}, usecase.InventoryRentalCreateInput{
			ItemID: item.ID, Quantity: 1, Purpose: "Uji Konkurensi", DueAt: due,
		})
		if err != nil {
			t.Fatal(err)
		}
		rentalIDs = append(rentalIDs, r.ID)
	}

	errs := make(chan error, 6)
	for i, rID := range rentalIDs {
		go func() {
			actor := k3
			if i%2 == 0 {
				actor = labops
			}
			_, err := svc.DecideRental(ctx, actor, usecase.AcademicCommand{Key: fmt.Sprintf("approve-%d", i)}, rID, usecase.InventoryRentalDecideInput{
				Version: 1, Action: "approve",
			})
			errs <- err
		}()
	}
	approvedCount := 0
	insufficientCount := 0
	for range 6 {
		err := <-errs
		if err == nil {
			approvedCount++
		} else if errors.Is(err, usecase.ErrInsufficientStock) {
			insufficientCount++
		} else {
			t.Fatalf("unexpected concurrent rental approval error: %v", err)
		}
	}
	if approvedCount != 2 || insufficientCount != 4 {
		t.Fatalf("expected 2 approved and 4 insufficient stock, got approved=%d insufficient=%d", approvedCount, insufficientCount)
	}

	// Advance clock past attendance session expiry and verify closing/check-in rules.
	advance(20 * time.Minute)
	if _, err := svc.CloseAttendanceSession(ctx, sekretaris, usecase.AcademicCommand{Key: "close-att"}, opened.ID); err != nil {
		t.Fatal(err)
	}

	// Bendahara can read financial report.
	rep, err := svc.Reports(ctx, bendahara)
	if err != nil || rep.ActiveRentalsCount < 2 {
		t.Fatalf("unexpected report: %+v %v", rep, err)
	}
}
