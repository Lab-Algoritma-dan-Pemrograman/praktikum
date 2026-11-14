package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgAcademicHarness struct {
	ctx        context.Context
	pool       *pgxpool.Pool
	repo       *repository.PostgresAcademicRepository
	service    *usecase.AcademicService
	admin      *entity.Principal
	students   []*entity.Principal
	class      entity.AcademicClass
	schedule   entity.AcademicSchedule
	attSession entity.AcademicAttendanceSession
	item       entity.InventoryItem
}

func pgAcademicCommand(key string) usecase.AcademicCommand {
	return usecase.AcademicCommand{Key: key, RequestID: "req-pg-academic", OperationID: "op-pg-academic"}
}

func newPGAcademic(t *testing.T) *pgAcademicHarness {
	t.Helper()
	ctx, pool := repository.OpenTestDatabase(t)
	h := &pgAcademicHarness{ctx: ctx, pool: pool, repo: repository.NewPostgresAcademicRepository(pool)}
	identity := repository.NewPostgresIdentityRepository(pool)
	scope := entity.Scope{Domain: "siakad", Institution: "synthetic-siakad"}
	var authIDs []string
	var userIDs []int64
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, `delete from public.audit_logs where actor_id=any($1::bigint[]) or scope->>'institution'='synthetic-siakad'`, userIDs); err != nil {
			t.Error(err)
		}
		for _, table := range []string{"notifications", "operations", "inventory_rentals", "inventory_items", "attendance_records", "attendance_sessions", "schedules", "classes"} {
			if _, err := pool.Exec(cleanup, `delete from public.academic_`+table+` where scope->>'institution'='synthetic-siakad' or owner_id=any($1::bigint[])`, userIDs); err != nil {
				t.Error(err)
			}
		}
		if _, err := pool.Exec(cleanup, `delete from auth.users where id=any($1::uuid[])`, authIDs); err != nil {
			t.Error(err)
		}
	})
	for i := range 3 {
		var authID string
		if err := pool.QueryRow(ctx, `insert into auth.users(id) values(gen_random_uuid()) returning id::text`).Scan(&authID); err != nil {
			t.Fatal(err)
		}
		authIDs = append(authIDs, authID)
		user, err := identity.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: authID, Email: authID + "@example.test", Name: "Synthetic SIAKAD user"}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, user.ID)
		role := entity.RoleMahasiswa
		if i == 0 {
			role = entity.RoleKoordinator
		}
		assignment := entity.RoleAssignment{Role: role, Scope: scope, Active: true}
		if err := identity.ReplaceAccess(ctx, user.ID, entity.UserAccess{Version: user.AccessVersion, Assignments: []entity.RoleAssignment{assignment}}); err != nil {
			t.Fatal(err)
		}
		p := &entity.Principal{User: user, Assignments: []entity.RoleAssignment{assignment}}
		if i == 0 {
			for _, perm := range []string{"class.manage", "schedule.write", "attendance.read", "inventory.manage", "rental.approve", "finance.manage"} {
				p.Grants = append(p.Grants, entity.PermissionGrant{Permission: perm, Role: role, Scope: scope})
			}
			h.admin = p
		} else {
			h.students = append(h.students, p)
		}
	}
	h.service = usecase.NewAcademicService(usecase.AcademicOptions{Repository: h.repo, IdentityRepository: identity})
	var err error
	h.class, err = h.service.CreateClass(ctx, h.admin, pgAcademicCommand("class"), usecase.AcademicClassInput{
		Code: "IF-DB-01", CourseCode: "IF1101", CourseName: "Algoritma DB", PeriodName: "Ganjil 2026", Shift: 1,
		StudentIDs: []int64{h.students[0].User.ID, h.students[1].User.ID}, AssistantIDs: []int64{}, Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.schedule, err = h.service.CreateSchedule(ctx, h.admin, pgAcademicCommand("schedule"), usecase.AcademicScheduleInput{
		ClassID: h.class.ID, DayOfWeek: "Senin", Shift: 1, Room: "Lab DB 1", StartsAt: "08:00 WIB", EndsAt: "10:00 WIB",
	})
	if err != nil {
		t.Fatal(err)
	}
	h.attSession, err = h.service.OpenAttendanceSession(ctx, h.admin, pgAcademicCommand("open-att"), usecase.AcademicAttendanceOpenInput{
		ScheduleID: h.schedule.ID, Title: "Sesi DB 1", DurationMinutes: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.item, err = h.service.CreateInventoryItem(ctx, h.admin, pgAcademicCommand("item"), usecase.InventoryItemInput{
		Code: "INV-DB-01", Name: "Multimeter DB", Category: "Alat Ukur", TotalQuantity: 2, AvailableQuantity: 2, RentalFee: 20000, Status: "available", Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPostgresAcademicConcurrencyImmutabilityRollbackAndRLS(t *testing.T) {
	h := newPGAcademic(t)
	pool2, err := pgxpool.NewWithConfig(h.ctx, h.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer pool2.Close()
	other := usecase.NewAcademicService(usecase.AcademicOptions{Repository: repository.NewPostgresAcademicRepository(pool2)})

	// Concurrent QR check-ins by the same student across two pools produce exactly one record.
	gate := make(chan struct{})
	errs := make(chan error, 8)
	for i := range 8 {
		go func() {
			<-gate
			s := h.service
			if i%2 == 0 {
				s = other
			}
			_, err := s.CheckInAttendance(h.ctx, h.students[0], pgAcademicCommand(fmt.Sprintf("checkin-%d", i)), usecase.AcademicAttendanceCheckinInput{
				SessionID: h.attSession.ID, QRToken: h.attSession.QRToken,
			})
			errs <- err
		}()
	}
	close(gate)
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	records, err := h.service.AttendanceRecords(h.ctx, h.admin, true, h.attSession.ID, usecase.PracticumPage{Limit: 25})
	if err != nil || len(records) != 1 {
		t.Fatalf("expected 1 attendance record after concurrent check-in, got %d (%v)", len(records), err)
	}

	// Concurrent rental approvals for a 2-unit item across two pools must never oversell stock.
	due := time.Now().UTC().Add(48 * time.Hour)
	var rentals []entity.InventoryRental
	for i := range 5 {
		r, err := h.service.RequestRental(h.ctx, h.students[i%2], pgAcademicCommand(fmt.Sprintf("rent-%d", i)), usecase.InventoryRentalCreateInput{
			ItemID: h.item.ID, Quantity: 1, Purpose: "Uji DB", DueAt: due,
		})
		if err != nil {
			t.Fatal(err)
		}
		rentals = append(rentals, r)
	}
	gate = make(chan struct{})
	errs = make(chan error, 5)
	for i, r := range rentals {
		go func() {
			<-gate
			s := h.service
			if i%2 == 0 {
				s = other
			}
			_, err := s.DecideRental(h.ctx, h.admin, pgAcademicCommand(fmt.Sprintf("decide-%d", i)), r.ID, usecase.InventoryRentalDecideInput{
				Version: 1, Action: "approve",
			})
			errs <- err
		}()
	}
	close(gate)
	approved := 0
	for range 5 {
		err := <-errs
		if err == nil {
			approved++
		} else if !errors.Is(err, usecase.ErrInsufficientStock) {
			t.Fatal(err)
		}
	}
	if approved != 2 {
		t.Fatalf("expected exactly 2 approved rentals, got %d", approved)
	}

	// Close attendance session and verify database immutability triggers and constraints.
	if _, err := h.service.CloseAttendanceSession(h.ctx, h.admin, pgAcademicCommand("close-att"), h.attSession.ID); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, sql string
		args      []any
	}{
		{"foreign-schedule-class", `insert into public.academic_schedules(id,parent_id,scope,data) values('bad-sched','missing-class','{}','{"day_of_week":"Senin","shift":1,"version":1}')`, nil},
		{"reopen-closed-att-session", `update public.academic_attendance_sessions set status='open' where id=$1`, []any{h.attSession.ID}},
		{"tamper-att-token-hash", `update public.academic_attendance_sessions set data=jsonb_set(data,'{token_hash}','"0000000000000000000000000000000000000000000000000000000000000000"') where id=$1`, []any{h.attSession.ID}},
		{"duplicate-att-record", `insert into public.academic_attendance_records(id,parent_id,owner_id,scope,status,data) select id||'-dup',parent_id,owner_id,scope,status,data from public.academic_attendance_records where id=$1`, []any{records[0].ID}},
		{"negative-available-stock", `update public.academic_inventory_items set data=jsonb_set(data,'{available_quantity}','-1') where id=$1`, []any{h.item.ID}},
		{"available-exceeds-total", `update public.academic_inventory_items set data=jsonb_set(data,'{available_quantity}','99') where id=$1`, []any{h.item.ID}},
		{"tamper-rental-quantity", `update public.academic_inventory_rentals set data=jsonb_set(data,'{quantity}','99') where id=$1`, []any{rentals[0].ID}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.pool.Begin(h.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(h.ctx, tc.sql, tc.args...); err == nil {
				t.Fatal("database accepted invalid state")
			}
		})
	}

	sentinel := errors.New("synthetic academic rollback")
	err = h.repo.Transaction(h.ctx, func(tx repository.AcademicTx) error {
		raw, _ := json.Marshal(h.class)
		if err := tx.Put(repository.AcademicClasses, repository.AcademicRecord{ID: h.class.ID + "-rollback", Scope: h.class.Scope, Data: raw}); err != nil {
			return err
		}
		if err := tx.Audit(entity.AuditEvent{ActorID: &h.admin.User.ID, Action: "academic.rollback.probe", App: "siakad", SubjectID: h.class.ID, Scope: h.class.Scope, CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal("repository transaction did not return sentinel")
	}
	var audits int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.audit_logs where action='academic.rollback.probe' and subject_id=$1`, h.class.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("audit escaped rolled-back academic transaction")
	}

	for _, role := range []string{"anon", "authenticated"} {
		for _, table := range []string{"classes", "schedules", "attendance_sessions", "attendance_records", "inventory_items", "inventory_rentals", "notifications", "operations"} {
			t.Run(role+"/"+table, func(t *testing.T) {
				var rls, canSelect bool
				if err := h.pool.QueryRow(h.ctx, `select c.relrowsecurity,has_table_privilege($1,c.oid,'SELECT') from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relname=$2`, role, "academic_"+table).Scan(&rls, &canSelect); err != nil || !rls || canSelect {
					t.Fatalf("missing RLS/grant restriction: rls=%v select=%v %v", rls, canSelect, err)
				}
				tx, err := h.pool.Begin(h.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(h.ctx, "set local role "+role); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(h.ctx, "select * from public.academic_"+table)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
					t.Fatalf("browser role directly read table: %v", err)
				}
			})
		}
	}

	var auditText string
	if err := h.pool.QueryRow(h.ctx, `select coalesce(jsonb_agg(to_jsonb(a))::text,'[]') from public.audit_logs a where scope->>'institution'='synthetic-siakad'`).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditText, h.attSession.QRToken) {
		t.Fatal("transactional audit leaked QR token")
	}
}
