package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func validInventoryStatus(status string) bool {
	return status == "available" || status == "maintenance" || status == "retired"
}

func (s *AcademicService) InventoryItems(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.InventoryItem, error) {
	out := []entity.InventoryItem{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicInventoryItems, repository.AcademicFilter{Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var item entity.InventoryItem
			if err = json.Unmarshal(r.Data, &item); err != nil {
				return err
			}
			out = append(out, item)
		}
		return nil
	})
	return out, err
}

func (s *AcademicService) CreateInventoryItem(ctx context.Context, p *entity.Principal, c AcademicCommand, in InventoryItemInput) (entity.InventoryItem, error) {
	if !validPracticumTitle(in.Code) || !validPracticumTitle(in.Name) || !validPracticumTitle(in.Category) || in.TotalQuantity < 0 || in.TotalQuantity > 10000 || in.AvailableQuantity < 0 || in.AvailableQuantity > in.TotalQuantity || in.RentalFee < 0 || in.RentalFee > 100000000 || !validInventoryStatus(in.Status) || !validAcademicScope(in.Scope) {
		return entity.InventoryItem{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "inventory.item.create", "", in, func(tx repository.AcademicTx, replay string) (out entity.InventoryItem, id string, scope entity.Scope, err error) {
		scope = in.Scope
		scope.Division = ""
		if err = academicRequire(p, scope, academicInventoryManage); err != nil {
			return
		}
		if replay != "" {
			out, err = academicReadRecord[entity.InventoryItem](tx, repository.AcademicInventoryItems, replay, false)
			return out, replay, scope, err
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		id = academicID("item")
		out = entity.InventoryItem{
			ID:                id,
			Code:              strings.TrimSpace(in.Code),
			Name:              strings.TrimSpace(in.Name),
			Category:          strings.TrimSpace(in.Category),
			TotalQuantity:     in.TotalQuantity,
			AvailableQuantity: in.AvailableQuantity,
			RentalFee:         in.RentalFee,
			Status:            in.Status,
			Scope:             scope,
			Version:           1,
			UpdatedAt:         now,
		}
		err = academicPut(tx, repository.AcademicInventoryItems, id, "", 0, scope, out.Status, out.Code, nil, out)
		return
	})
}

func (s *AcademicService) UpdateInventoryItem(ctx context.Context, p *entity.Principal, c AcademicCommand, id string, in InventoryItemUpdate) (entity.InventoryItem, error) {
	if !validPracticumTitle(in.Name) || !validPracticumTitle(in.Category) || in.TotalQuantity < 0 || in.TotalQuantity > 10000 || in.AvailableQuantity < 0 || in.AvailableQuantity > in.TotalQuantity || in.RentalFee < 0 || in.RentalFee > 100000000 || !validInventoryStatus(in.Status) || in.Version < 1 {
		return entity.InventoryItem{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "inventory.item.update", id, in, func(tx repository.AcademicTx, replay string) (out entity.InventoryItem, target string, scope entity.Scope, err error) {
		out, err = academicReadRecord[entity.InventoryItem](tx, repository.AcademicInventoryItems, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = academicRequire(p, scope, academicInventoryManage); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		rentals, e := academicListAll(tx, repository.AcademicInventoryRentals, repository.AcademicFilter{ParentID: id, Status: "approved"})
		if e != nil {
			err = e
			return
		}
		borrowed := 0
		for _, row := range rentals {
			var rental entity.InventoryRental
			if err = json.Unmarshal(row.Data, &rental); err != nil {
				return
			}
			borrowed += rental.Quantity
		}
		if in.TotalQuantity-in.AvailableQuantity < borrowed {
			err = repository.ErrConflict
			return
		}
		out.Name = strings.TrimSpace(in.Name)
		out.Category = strings.TrimSpace(in.Category)
		out.TotalQuantity = in.TotalQuantity
		out.AvailableQuantity = in.AvailableQuantity
		out.RentalFee = in.RentalFee
		out.Status = in.Status
		out.Version++
		out.UpdatedAt, err = tx.Now()
		if err == nil {
			err = academicPut(tx, repository.AcademicInventoryItems, id, "", 0, scope, out.Status, out.Code, nil, out)
		}
		return
	})
}

func (s *AcademicService) RequestRental(ctx context.Context, p *entity.Principal, c AcademicCommand, in InventoryRentalCreateInput) (entity.InventoryRental, error) {
	if in.ItemID == "" || len(in.ItemID) > 128 || in.Quantity < 1 || in.Quantity > 1000 || !validPracticumTitle(in.Purpose) {
		return entity.InventoryRental{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "inventory.rental.create", in.ItemID, in, func(tx repository.AcademicTx, replay string) (out entity.InventoryRental, id string, scope entity.Scope, err error) {
		item, e := academicReadRecord[entity.InventoryItem](tx, repository.AcademicInventoryItems, in.ItemID, true)
		if e != nil {
			err = e
			return
		}
		scope = item.Scope
		if replay != "" {
			out, err = academicReadRecord[entity.InventoryRental](tx, repository.AcademicInventoryRentals, replay, false)
			return out, replay, scope, err
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		due := in.DueAt.UTC()
		if !due.After(now) || due.After(now.Add(180*24*time.Hour)) {
			err = ErrValidation
			return
		}
		if item.Status != "available" || item.AvailableQuantity < in.Quantity {
			err = ErrInsufficientStock
			return
		}
		id = academicID("rental")
		out = entity.InventoryRental{
			ID:              id,
			ItemID:          item.ID,
			ItemName:        item.Name,
			BorrowerID:      p.User.ID,
			BorrowerName:    p.User.Name,
			Quantity:        in.Quantity,
			Purpose:         strings.TrimSpace(in.Purpose),
			Status:          "requested",
			TotalFee:        in.Quantity * item.RentalFee,
			PaymentVerified: item.RentalFee == 0,
			RequestedAt:     now,
			DueAt:           due,
			Scope:           scope,
			Version:         1,
		}
		err = academicPut(tx, repository.AcademicInventoryRentals, id, item.ID, p.User.ID, scope, out.Status, "", &due, out)
		return
	})
}

func (s *AcademicService) Rentals(ctx context.Context, p *entity.Principal, admin bool, page PracticumPage) ([]entity.InventoryRental, error) {
	out := []entity.InventoryRental{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		var filter repository.AcademicFilter
		if admin {
			scopes := academicFilterScopes(p, academicRentalApprove, academicInventoryManage, academicFinanceManage)
			if len(scopes) == 0 {
				return nil
			}
			filter = repository.AcademicFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset}
		} else {
			filter = repository.AcademicFilter{OwnerID: p.User.ID, Limit: page.Limit, Offset: page.Offset}
		}
		rows, err := tx.List(repository.AcademicInventoryRentals, filter)
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rental entity.InventoryRental
			if err = json.Unmarshal(r.Data, &rental); err != nil {
				return err
			}
			out = append(out, rental)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
		return nil
	})
	return out, err
}

func (s *AcademicService) DecideRental(ctx context.Context, p *entity.Principal, c AcademicCommand, id string, in InventoryRentalDecideInput) (entity.InventoryRental, error) {
	if in.Version < 1 || (in.Action != "approve" && in.Action != "reject" && in.Action != "return" && in.Action != "verify_payment") {
		return entity.InventoryRental{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "inventory.rental."+in.Action, id, in, func(tx repository.AcademicTx, replay string) (out entity.InventoryRental, target string, scope entity.Scope, err error) {
		row, e := tx.Get(repository.AcademicInventoryRentals, id)
		if e != nil {
			err = e
			return
		}
		item, e := academicReadRecord[entity.InventoryItem](tx, repository.AcademicInventoryItems, row.ParentID, true)
		if e != nil {
			err = e
			return
		}
		out, err = academicReadRecord[entity.InventoryRental](tx, repository.AcademicInventoryRentals, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		switch in.Action {
		case "approve", "reject", "return":
			err = academicRequire(p, scope, academicRentalApprove)
		case "verify_payment":
			err = academicRequire(p, scope, academicFinanceManage)
		}
		if err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		switch in.Action {
		case "approve":
			if out.Status != "requested" {
				err = repository.ErrConflict
				return
			}
			if item.Status != "available" || item.AvailableQuantity < out.Quantity {
				err = ErrInsufficientStock
				return
			}
			item.AvailableQuantity -= out.Quantity
			item.Version++
			item.UpdatedAt = now
			if err = academicPut(tx, repository.AcademicInventoryItems, item.ID, "", 0, item.Scope, item.Status, item.Code, nil, item); err != nil {
				return
			}
			out.Status = "approved"
			out.DecidedAt = &now
			out.DecidedBy = &p.User.ID
		case "reject":
			if out.Status != "requested" {
				err = repository.ErrConflict
				return
			}
			out.Status = "rejected"
			out.DecidedAt = &now
			out.DecidedBy = &p.User.ID
		case "return":
			if out.Status != "approved" {
				err = repository.ErrConflict
				return
			}
			if item.AvailableQuantity+out.Quantity > item.TotalQuantity {
				err = repository.ErrConflict
				return
			}
			item.AvailableQuantity += out.Quantity
			item.Version++
			item.UpdatedAt = now
			if err = academicPut(tx, repository.AcademicInventoryItems, item.ID, "", 0, item.Scope, item.Status, item.Code, nil, item); err != nil {
				return
			}
			out.Status = "returned"
			out.ReturnedAt = &now
		case "verify_payment":
			if out.Status != "approved" && out.Status != "returned" {
				err = repository.ErrConflict
				return
			}
			if out.PaymentVerified {
				return
			}
			out.PaymentVerified = true
		}
		out.Version++
		if err = academicPut(tx, repository.AcademicInventoryRentals, out.ID, out.ItemID, out.BorrowerID, scope, out.Status, "", &out.DueAt, out); err != nil {
			return
		}
		err = academicNotify(tx, out.BorrowerID, "rental."+in.Action, "Status Penyewaan Alat", fmt.Sprintf("Permohonan sewa %s (%d unit) diperbarui: %s.", out.ItemName, out.Quantity, out.Status), scope, now)
		return
	})
}

func (s *AcademicService) Notifications(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.AcademicNotification, error) {
	out := []entity.AcademicNotification{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicNotifications, repository.AcademicFilter{OwnerID: p.User.ID, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var n entity.AcademicNotification
			if err = json.Unmarshal(r.Data, &n); err != nil {
				return err
			}
			out = append(out, n)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
		return nil
	})
	return out, err
}

func (s *AcademicService) Reports(ctx context.Context, p *entity.Principal) (entity.AcademicReportSummary, error) {
	out := entity.AcademicReportSummary{
		AttendanceByStatus: map[string]int{"hadir": 0, "izin": 0, "alpha": 0, "terlambat": 0},
		Timezone:           "Asia/Jakarta",
		TimezoneOffset:     "+07:00",
	}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	scopes := academicFilterScopes(p, academicClassManage, academicScheduleWrite, academicAttendanceRead, academicInventoryManage, academicRentalApprove, academicFinanceManage)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		out.GeneratedAt = now
		classes, err := academicListAll(tx, repository.AcademicClasses, repository.AcademicFilter{Scopes: scopes})
		if err != nil {
			return err
		}
		out.ClassesCount = len(classes)
		schedules, err := academicListAll(tx, repository.AcademicSchedules, repository.AcademicFilter{Scopes: scopes})
		if err != nil {
			return err
		}
		out.SchedulesCount = len(schedules)
		attSessions, err := academicListAll(tx, repository.AcademicAttendanceSessions, repository.AcademicFilter{Status: "open", Scopes: scopes})
		if err != nil {
			return err
		}
		for _, r := range attSessions {
			if r.ExpiresAt != nil && now.Before(*r.ExpiresAt) {
				out.OpenAttendanceSessions++
			}
		}
		attRecords, err := academicListAll(tx, repository.AcademicAttendanceRecords, repository.AcademicFilter{Scopes: scopes})
		if err != nil {
			return err
		}
		for _, r := range attRecords {
			out.AttendanceByStatus[r.Status]++
		}
		items, err := academicListAll(tx, repository.AcademicInventoryItems, repository.AcademicFilter{Scopes: scopes})
		if err != nil {
			return err
		}
		out.TotalInventoryItems = len(items)
		for _, r := range items {
			var item entity.InventoryItem
			if err = json.Unmarshal(r.Data, &item); err != nil {
				return err
			}
			out.AvailableInventoryUnit += item.AvailableQuantity
		}
		rentals, err := academicListAll(tx, repository.AcademicInventoryRentals, repository.AcademicFilter{Scopes: scopes})
		if err != nil {
			return err
		}
		for _, r := range rentals {
			var rental entity.InventoryRental
			if err = json.Unmarshal(r.Data, &rental); err != nil {
				return err
			}
			if rental.Status == "approved" {
				out.ActiveRentalsCount++
			}
			if academicAllowed(p, academicFinanceManage, rental.Scope) && rental.PaymentVerified && (rental.Status == "approved" || rental.Status == "returned") {
				out.TotalVerifiedRevenue += rental.TotalFee
			}
		}
		out.CSVExport = fmt.Sprintf(
			"metric,value\nclasses_count,%d\nschedules_count,%d\nopen_attendance_sessions,%d\nattendance_hadir,%d\nattendance_izin,%d\nattendance_alpha,%d\nattendance_terlambat,%d\ntotal_inventory_items,%d\navailable_inventory_units,%d\nactive_rentals_count,%d\ntotal_verified_revenue,%d\n",
			out.ClassesCount,
			out.SchedulesCount,
			out.OpenAttendanceSessions,
			out.AttendanceByStatus["hadir"],
			out.AttendanceByStatus["izin"],
			out.AttendanceByStatus["alpha"],
			out.AttendanceByStatus["terlambat"],
			out.TotalInventoryItems,
			out.AvailableInventoryUnit,
			out.ActiveRentalsCount,
			out.TotalVerifiedRevenue,
		)
		return nil
	})
	return out, err
}
