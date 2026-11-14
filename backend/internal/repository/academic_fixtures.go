package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// SeedAcademicFixtures is opt-in local/test data, never a migration or production seed.
func SeedAcademicFixtures(ctx context.Context, repo AcademicRepository) error {
	return repo.Transaction(ctx, func(tx AcademicTx) error {
		if _, err := tx.Get(AcademicClasses, "class-101"); err == nil {
			return nil
		} else if !errors.Is(err, ErrAcademicNotFound) {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		put := func(kind AcademicKind, id, parent, status, unique string, scope entity.Scope, value any) error {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return tx.Put(kind, AcademicRecord{ID: id, ParentID: parent, Scope: scope, Status: status, UniqueKey: unique, Data: raw})
		}

		classScope := entity.Scope{Domain: "siakad", Institution: "lab", ClassID: "class-101", Shift: 1}
		cls := entity.AcademicClass{
			ID:           "class-101",
			Code:         "IF-101-A",
			CourseCode:   "IF1101",
			CourseName:   "Algoritma dan Pemrograman I",
			PeriodName:   "Ganjil 2026/2027",
			Shift:        1,
			StudentIDs:   []int64{1},
			AssistantIDs: []int64{2, 3},
			Active:       true,
			Scope:        classScope,
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err = put(AcademicClasses, cls.ID, "", "", cls.Code, classScope, cls); err != nil {
			return err
		}

		schedScope := classScope
		sched := entity.AcademicSchedule{
			ID:         "schedule-1",
			ClassID:    cls.ID,
			ClassCode:  cls.Code,
			CourseName: cls.CourseName,
			DayOfWeek:  "Senin",
			Shift:      1,
			Room:       "Lab Dasar 1",
			StartsAt:   "08:00 WIB",
			EndsAt:     "10:00 WIB",
			Scope:      schedScope,
			Version:    1,
		}
		if err = put(AcademicSchedules, sched.ID, cls.ID, "", "", schedScope, sched); err != nil {
			return err
		}

		invScope := entity.Scope{Domain: "siakad", Institution: "lab"}
		items := []entity.InventoryItem{
			{
				ID:                "item-arduino",
				Code:              "INV-ARD-01",
				Name:              "Kit Mikrokontroler Arduino Uno",
				Category:          "Elektronika",
				TotalQuantity:     5,
				AvailableQuantity: 5,
				RentalFee:         25000,
				Status:            "available",
				Scope:             invScope,
				Version:           1,
				UpdatedAt:         now,
			},
			{
				ID:                "item-oscilloscope",
				Code:              "INV-OSC-01",
				Name:              "Osiloskop Digital 2-Channel",
				Category:          "Alat Ukur",
				TotalQuantity:     2,
				AvailableQuantity: 2,
				RentalFee:         50000,
				Status:            "available",
				Scope:             invScope,
				Version:           1,
				UpdatedAt:         now,
			},
		}
		for _, item := range items {
			if err = put(AcademicInventoryItems, item.ID, "", item.Status, item.Code, invScope, item); err != nil {
				return err
			}
		}
		return nil
	})
}
