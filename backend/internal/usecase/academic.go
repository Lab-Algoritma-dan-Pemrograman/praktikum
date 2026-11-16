package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

var (
	ErrAttendanceClosed  = errors.New("attendance session is closed or expired")
	ErrInsufficientStock = errors.New("insufficient available inventory stock")
	ErrRentalInvalid     = errors.New("invalid rental state transition")
)

const (
	academicClassManage     = "class.manage"
	academicScheduleWrite   = "schedule.write"
	academicAttendanceRead  = "attendance.read"
	academicInventoryManage = "inventory.manage"
	academicRentalApprove   = "rental.approve"
	academicLeaveDecide     = "leave.decide"
	academicFinanceManage   = "finance.manage"
)

type AcademicOptions struct {
	Repository         repository.AcademicRepository
	IdentityRepository repository.IdentityRepository
}

type AcademicService struct{ options AcademicOptions }

func NewAcademicService(o AcademicOptions) *AcademicService {
	if r, ok := o.Repository.(*repository.MemoryAcademicRepository); ok {
		if identity, ok := o.IdentityRepository.(*repository.MemoryIdentityRepository); ok {
			r.BindIdentity(identity)
		}
	}
	return &AcademicService{options: o}
}

type AcademicCommand struct{ Key, RequestID, OperationID string }

type AcademicClassInput struct {
	Code         string       `json:"code"`
	CourseCode   string       `json:"course_code"`
	CourseName   string       `json:"course_name"`
	PeriodName   string       `json:"period_name"`
	Shift        int          `json:"shift"`
	StudentIDs   []int64      `json:"student_ids"`
	AssistantIDs []int64      `json:"assistant_ids"`
	Scope        entity.Scope `json:"scope"`
}

type AcademicClassUpdate struct {
	Version      int64   `json:"version"`
	Code         string  `json:"code"`
	CourseCode   string  `json:"course_code"`
	CourseName   string  `json:"course_name"`
	PeriodName   string  `json:"period_name"`
	Shift        int     `json:"shift"`
	StudentIDs   []int64 `json:"student_ids"`
	AssistantIDs []int64 `json:"assistant_ids"`
	Active       bool    `json:"active"`
}

type AcademicScheduleInput struct {
	ClassID   string `json:"class_id"`
	DayOfWeek string `json:"day_of_week"`
	Shift     int    `json:"shift"`
	Room      string `json:"room"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
}

type AcademicAttendanceOpenInput struct {
	ScheduleID      string `json:"schedule_id"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
}

type AcademicAttendanceCheckinInput struct {
	SessionID string `json:"session_id"`
	QRToken   string `json:"qr_token"`
}

type AcademicAttendanceValidateInput struct {
	Version   int64  `json:"version"`
	Status    string `json:"status"`
	Validated bool   `json:"validated"`
	Notes     string `json:"notes"`
}

type InventoryItemInput struct {
	Code              string       `json:"code"`
	Name              string       `json:"name"`
	Category          string       `json:"category"`
	TotalQuantity     int          `json:"total_quantity"`
	AvailableQuantity int          `json:"available_quantity"`
	RentalFee         int          `json:"rental_fee"`
	Status            string       `json:"status"`
	Scope             entity.Scope `json:"scope"`
}

type InventoryItemUpdate struct {
	Version           int64  `json:"version"`
	Name              string `json:"name"`
	Category          string `json:"category"`
	TotalQuantity     int    `json:"total_quantity"`
	AvailableQuantity int    `json:"available_quantity"`
	RentalFee         int    `json:"rental_fee"`
	Status            string `json:"status"`
}

type InventoryRentalCreateInput struct {
	ItemID   string    `json:"item_id"`
	Quantity int       `json:"quantity"`
	Purpose  string    `json:"purpose"`
	DueAt    time.Time `json:"due_at"`
}

type InventoryRentalDecideInput struct {
	Version int64  `json:"version"`
	Action  string `json:"action"`
}

type AcademicLeaveCreateInput struct {
	ScheduleID string    `json:"schedule_id"`
	Kind       string    `json:"kind"`
	Reason     string    `json:"reason"`
	OriginalAt time.Time `json:"original_at"`
}

type AcademicLeaveDecideInput struct {
	Version      int64      `json:"version"`
	Action       string     `json:"action"`
	RescheduleAt *time.Time `json:"reschedule_at,omitempty"`
}

type AcademicFinanceMutationInput struct {
	TargetUserID int64  `json:"target_user_id"`
	Type         string `json:"type"`
	Note         string `json:"note"`
	Amount       int64  `json:"amount"`
	Category     string `json:"category"`
	RentalID     string `json:"rental_id,omitempty"`
}

type academicOperation struct {
	PayloadHash string `json:"payload_hash"`
	TargetID    string `json:"target_id"`
}

func academicID(prefix string) string {
	v, err := security.RandomToken("")
	if err != nil {
		panic(err)
	}
	return prefix + "-" + v
}

func academicHash(v any) string { data, _ := json.Marshal(v); return security.Hash(string(data)) }

func academicReadRecord[T any](tx repository.AcademicTx, k repository.AcademicKind, id string, lock bool) (T, error) {
	var out T
	var row repository.AcademicRecord
	var err error
	if lock {
		row, err = tx.Lock(k, id)
	} else {
		row, err = tx.Get(k, id)
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(row.Data, &out)
	return out, err
}

func academicListAll(tx repository.AcademicTx, kind repository.AcademicKind, filter repository.AcademicFilter) ([]repository.AcademicRecord, error) {
	filter.Limit = 1000
	out := []repository.AcademicRecord{}
	for {
		rows, err := tx.List(kind, filter)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if len(rows) < filter.Limit {
			return out, nil
		}
		filter.AfterID = rows[len(rows)-1].ID
	}
}

func academicPut(tx repository.AcademicTx, k repository.AcademicKind, id, parent string, owner int64, scope entity.Scope, status, unique string, expires *time.Time, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return tx.Put(k, repository.AcademicRecord{ID: id, ParentID: parent, OwnerID: owner, Scope: scope, Status: status, UniqueKey: unique, ExpiresAt: expires, Data: raw})
}

func academicAudit(tx repository.AcademicTx, p *entity.Principal, c AcademicCommand, action, id string, scope entity.Scope) error {
	now, err := tx.Now()
	if err != nil {
		return err
	}
	var actor *int64
	if p != nil {
		actor = &p.User.ID
	}
	if scope.Domain == "" {
		scope.Domain = "siakad"
	}
	return tx.Audit(entity.AuditEvent{ActorID: actor, Action: action, App: "siakad", SubjectType: "academic", SubjectID: id, Scope: scope, RequestID: c.RequestID, OperationID: c.OperationID, CreatedAt: now, Detail: map[string]any{}})
}

func academicNotify(tx repository.AcademicTx, userID int64, typ, title, message string, scope entity.Scope, now time.Time) error {
	id := academicID("notif")
	n := entity.AcademicNotification{
		ID:        id,
		UserID:    userID,
		Type:      typ,
		Title:     title,
		Message:   message,
		Read:      false,
		CreatedAt: now,
	}
	return academicPut(tx, repository.AcademicNotifications, id, "", userID, scope, "unread", "", nil, n)
}

func academicAllowed(p *entity.Principal, permission string, resourceScope entity.Scope) bool {
	if practicumAuthenticated(p) != nil {
		return false
	}
	for _, g := range p.Grants {
		if g.Permission != permission {
			continue
		}
		target := resourceScope
		if target.Division == "" && g.Scope.Division != "" {
			target.Division = g.Scope.Division
		}
		if practicumAllowed(p, permission, target) {
			return true
		}
	}
	return false
}

func academicRequire(p *entity.Principal, resourceScope entity.Scope, permissions ...string) error {
	if err := practicumAuthenticated(p); err != nil {
		return err
	}
	for _, perm := range permissions {
		if academicAllowed(p, perm, resourceScope) {
			return nil
		}
	}
	return authorization.ErrForbidden
}

func academicFilterScopes(p *entity.Principal, permissions ...string) []entity.Scope {
	out := []entity.Scope{}
	if practicumAuthenticated(p) != nil {
		return out
	}
	for _, g := range p.Grants {
		for _, perm := range permissions {
			if g.Permission == perm && practicumAllowed(p, perm, g.Scope) {
				s := g.Scope
				s.Division = ""
				out = append(out, s)
			}
		}
	}
	return out
}

func academicMutate[T any](s *AcademicService, ctx context.Context, p *entity.Principal, c AcademicCommand, action, resource string, input any, fn func(repository.AcademicTx, string) (T, string, entity.Scope, error)) (T, error) {
	var out T
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !practicumKeyPattern.MatchString(c.Key) {
		return out, ErrValidation
	}
	opID := academicHash([]any{p.User.ID, action, resource, c.Key})
	hash := academicHash(input)
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		replay := ""
		op, err := academicReadRecord[academicOperation](tx, repository.AcademicOperations, opID, false)
		if err == nil {
			if op.PayloadHash != hash {
				return repository.ErrConflict
			}
			replay = op.TargetID
		} else if !errors.Is(err, repository.ErrAcademicNotFound) {
			return err
		}
		var target string
		var scope entity.Scope
		out, target, scope, err = fn(tx, replay)
		if err != nil {
			return err
		}
		if replay != "" {
			return nil
		}
		if scope.Domain == "" {
			scope.Domain = "siakad"
		}
		if err = academicPut(tx, repository.AcademicOperations, opID, "", p.User.ID, scope, "", opID, nil, academicOperation{PayloadHash: hash, TargetID: target}); err != nil {
			return err
		}
		return academicAudit(tx, p, c, action, target, scope)
	})
	return out, err
}

func validAcademicScope(s entity.Scope) bool {
	if s.Domain != "siakad" || s.Shift < 0 || s.Shift > 2 {
		return false
	}
	for _, v := range []string{s.Institution, s.Division, s.ClassID, s.SessionID} {
		if len(v) > 128 || strings.ContainsAny(v, "\x00*\r\n") {
			return false
		}
	}
	return true
}

func validDayOfWeek(day string) bool {
	switch day {
	case "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu":
		return true
	}
	return false
}

func publicClass(c entity.AcademicClass, admin bool) entity.AcademicClass {
	if !admin {
		c.StudentIDs = nil
		c.AssistantIDs = nil
	} else {
		if c.StudentIDs == nil {
			c.StudentIDs = []int64{}
		}
		if c.AssistantIDs == nil {
			c.AssistantIDs = []int64{}
		}
	}
	return c
}

func publicAttendanceSession(s entity.AcademicAttendanceSession) entity.AcademicAttendanceSession {
	s.QRToken = ""
	return s
}

func (s *AcademicService) Classes(ctx context.Context, p *entity.Principal, admin bool, page PracticumPage) ([]entity.AcademicClass, error) {
	out := []entity.AcademicClass{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		var filter repository.AcademicFilter
		if admin {
			scopes := academicFilterScopes(p, academicClassManage, academicScheduleWrite, academicAttendanceRead)
			if len(scopes) == 0 {
				return nil
			}
			filter = repository.AcademicFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset}
		} else {
			filter = repository.AcademicFilter{MemberID: p.User.ID, Limit: page.Limit, Offset: page.Offset}
		}
		rows, err := tx.List(repository.AcademicClasses, filter)
		if err != nil {
			return err
		}
		for _, r := range rows {
			var cls entity.AcademicClass
			if err = json.Unmarshal(r.Data, &cls); err != nil {
				return err
			}
			if !admin && !cls.Active {
				continue
			}
			out = append(out, publicClass(cls, admin))
		}
		return nil
	})
	return out, err
}

func validateRosterIDs(ids []int64) bool {
	if len(ids) > 500 {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id < 1 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *AcademicService) CreateClass(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicClassInput) (entity.AcademicClass, error) {
	if !validPracticumTitle(in.Code) || !validPracticumTitle(in.CourseCode) || !validPracticumTitle(in.CourseName) || !validPracticumTitle(in.PeriodName) || in.Shift < 1 || in.Shift > 2 || !validateRosterIDs(in.StudentIDs) || !validateRosterIDs(in.AssistantIDs) || !validAcademicScope(in.Scope) {
		return entity.AcademicClass{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.class.create", "", in, func(tx repository.AcademicTx, replay string) (out entity.AcademicClass, id string, scope entity.Scope, err error) {
		if replay != "" {
			out, err = academicReadRecord[entity.AcademicClass](tx, repository.AcademicClasses, replay, false)
			if err == nil {
				err = academicRequire(p, out.Scope, academicClassManage)
			}
			return publicClass(out, true), replay, out.Scope, err
		}
		id = academicID("class")
		scope = in.Scope
		scope.Division = ""
		scope.ClassID = id
		scope.Shift = in.Shift
		if err = academicRequire(p, scope, academicClassManage); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		out = entity.AcademicClass{
			ID:           id,
			Code:         strings.TrimSpace(in.Code),
			CourseCode:   strings.TrimSpace(in.CourseCode),
			CourseName:   strings.TrimSpace(in.CourseName),
			PeriodName:   strings.TrimSpace(in.PeriodName),
			Shift:        in.Shift,
			StudentIDs:   in.StudentIDs,
			AssistantIDs: in.AssistantIDs,
			Active:       true,
			Scope:        scope,
			Version:      1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err = academicPut(tx, repository.AcademicClasses, id, "", 0, scope, "", out.Code, nil, out)
		out = publicClass(out, true)
		return
	})
}

func (s *AcademicService) UpdateClass(ctx context.Context, p *entity.Principal, c AcademicCommand, id string, in AcademicClassUpdate) (entity.AcademicClass, error) {
	if !validPracticumTitle(in.Code) || !validPracticumTitle(in.CourseCode) || !validPracticumTitle(in.CourseName) || !validPracticumTitle(in.PeriodName) || in.Shift < 1 || in.Shift > 2 || !validateRosterIDs(in.StudentIDs) || !validateRosterIDs(in.AssistantIDs) || in.Version < 1 {
		return entity.AcademicClass{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.class.update", id, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicClass, target string, scope entity.Scope, err error) {
		out, err = academicReadRecord[entity.AcademicClass](tx, repository.AcademicClasses, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = academicRequire(p, scope, academicClassManage); err != nil || replay != "" {
			out = publicClass(out, true)
			return
		}
		destination := scope
		destination.Shift = in.Shift
		if err = academicRequire(p, destination, academicClassManage); err != nil {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		out.Code = strings.TrimSpace(in.Code)
		out.CourseCode = strings.TrimSpace(in.CourseCode)
		out.CourseName = strings.TrimSpace(in.CourseName)
		out.PeriodName = strings.TrimSpace(in.PeriodName)
		out.Shift = in.Shift
		out.Scope.Shift = in.Shift
		out.StudentIDs = in.StudentIDs
		out.AssistantIDs = in.AssistantIDs
		out.Active = in.Active
		out.Version++
		out.UpdatedAt, err = tx.Now()
		if err == nil {
			err = academicPut(tx, repository.AcademicClasses, id, "", 0, out.Scope, "", out.Code, nil, out)
		}
		out = publicClass(out, true)
		return
	})
}

func (s *AcademicService) Schedules(ctx context.Context, p *entity.Principal, admin bool, page PracticumPage) ([]entity.AcademicScheduleView, error) {
	out := []entity.AcademicScheduleView{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		now, err := tx.Now()
		if err != nil {
			return err
		}
		var filter repository.AcademicFilter
		if admin {
			scopes := academicFilterScopes(p, academicScheduleWrite, academicAttendanceRead, academicClassManage)
			if len(scopes) == 0 {
				return nil
			}
			filter = repository.AcademicFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset}
		} else {
			filter = repository.AcademicFilter{MemberID: p.User.ID, Limit: page.Limit, Offset: page.Offset}
		}
		rows, err := tx.List(repository.AcademicSchedules, filter)
		if err != nil {
			return err
		}
		sessionRows, err := academicListAll(tx, repository.AcademicAttendanceSessions, repository.AcademicFilter{Status: "open"})
		if err != nil {
			return err
		}
		openBySchedule := map[string]*entity.AcademicAttendanceSession{}
		for _, r := range sessionRows {
			var rec entity.AcademicAttendanceSessionRecord
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			if rec.Session.IsOpen && now.Before(rec.Session.ExpiresAt) {
				pub := publicAttendanceSession(rec.Session)
				openBySchedule[rec.Session.ScheduleID] = &pub
			}
		}
		for _, r := range rows {
			var sched entity.AcademicSchedule
			if err = json.Unmarshal(r.Data, &sched); err != nil {
				return err
			}
			out = append(out, entity.AcademicScheduleView{
				Schedule:    sched,
				OpenSession: openBySchedule[sched.ID],
			})
		}
		return nil
	})
	return out, err
}

func (s *AcademicService) CreateSchedule(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicScheduleInput) (entity.AcademicSchedule, error) {
	if in.ClassID == "" || !validDayOfWeek(in.DayOfWeek) || in.Shift < 1 || in.Shift > 2 || !validPracticumTitle(in.Room) || !validPracticumTitle(in.StartsAt) || !validPracticumTitle(in.EndsAt) {
		return entity.AcademicSchedule{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.schedule.create", in.ClassID, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicSchedule, id string, scope entity.Scope, err error) {
		cls, e := academicReadRecord[entity.AcademicClass](tx, repository.AcademicClasses, in.ClassID, true)
		if e != nil {
			err = e
			return
		}
		scope = cls.Scope
		scope.Division = ""
		if err = academicRequire(p, scope, academicScheduleWrite); err != nil {
			return
		}
		if replay != "" {
			out, err = academicReadRecord[entity.AcademicSchedule](tx, repository.AcademicSchedules, replay, false)
			if err == nil {
				err = academicRequire(p, out.Scope, academicScheduleWrite)
			}
			return out, replay, out.Scope, err
		}
		scope.Shift = in.Shift
		if err = academicRequire(p, scope, academicScheduleWrite); err != nil {
			return
		}
		id = academicID("schedule")
		out = entity.AcademicSchedule{
			ID:         id,
			ClassID:    cls.ID,
			ClassCode:  cls.Code,
			CourseName: cls.CourseName,
			DayOfWeek:  in.DayOfWeek,
			Shift:      in.Shift,
			Room:       strings.TrimSpace(in.Room),
			StartsAt:   strings.TrimSpace(in.StartsAt),
			EndsAt:     strings.TrimSpace(in.EndsAt),
			Scope:      scope,
			Version:    1,
		}
		err = academicPut(tx, repository.AcademicSchedules, id, cls.ID, 0, scope, "", "", nil, out)
		return
	})
}

func (s *AcademicService) OpenAttendanceSession(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicAttendanceOpenInput) (entity.AcademicAttendanceSession, error) {
	if in.ScheduleID == "" || !validPracticumTitle(in.Title) || in.DurationMinutes < 1 || in.DurationMinutes > 240 {
		return entity.AcademicAttendanceSession{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.attendance.open", in.ScheduleID, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicAttendanceSession, id string, scope entity.Scope, err error) {
		sched, e := academicReadRecord[entity.AcademicSchedule](tx, repository.AcademicSchedules, in.ScheduleID, true)
		if e != nil {
			err = e
			return
		}
		scope = sched.Scope
		scope.Division = ""
		if err = academicRequire(p, scope, academicScheduleWrite); err != nil {
			return
		}
		if replay != "" {
			rec, e := academicReadRecord[entity.AcademicAttendanceSessionRecord](tx, repository.AcademicAttendanceSessions, replay, false)
			if e == nil {
				e = academicRequire(p, rec.Session.Scope, academicScheduleWrite)
			}
			return publicAttendanceSession(rec.Session), replay, rec.Session.Scope, e
		}
		id = academicID("att-session")
		scope.SessionID = id
		if err = academicRequire(p, scope, academicScheduleWrite); err != nil {
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		token, e := security.RandomToken("")
		if e != nil {
			err = e
			return
		}
		expires := now.Add(time.Duration(in.DurationMinutes) * time.Minute)
		out = entity.AcademicAttendanceSession{
			ID:         id,
			ScheduleID: sched.ID,
			ClassID:    sched.ClassID,
			Title:      strings.TrimSpace(in.Title),
			IsOpen:     true,
			OpenedAt:   now,
			ExpiresAt:  expires,
			Scope:      scope,
			Version:    1,
		}
		rec := entity.AcademicAttendanceSessionRecord{
			Session:   out,
			TokenHash: security.Hash(token),
		}
		err = academicPut(tx, repository.AcademicAttendanceSessions, id, sched.ID, 0, scope, "open", "", &expires, rec)
		out.QRToken = token
		return
	})
}

func (s *AcademicService) CloseAttendanceSession(ctx context.Context, p *entity.Principal, c AcademicCommand, id string) (entity.AcademicAttendanceSession, error) {
	return academicMutate(s, ctx, p, c, "academic.attendance.close", id, struct{}{}, func(tx repository.AcademicTx, replay string) (out entity.AcademicAttendanceSession, target string, scope entity.Scope, err error) {
		rec, e := academicReadRecord[entity.AcademicAttendanceSessionRecord](tx, repository.AcademicAttendanceSessions, id, true)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = rec.Session.Scope
		if err = academicRequire(p, scope, academicScheduleWrite); err != nil {
			return
		}
		if !rec.Session.IsOpen {
			return publicAttendanceSession(rec.Session), target, scope, nil
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		rec.Session.IsOpen = false
		rec.Session.ClosedAt = &now
		rec.Session.Version++
		err = academicPut(tx, repository.AcademicAttendanceSessions, id, rec.Session.ScheduleID, 0, scope, "closed", "", &rec.Session.ExpiresAt, rec)
		out = publicAttendanceSession(rec.Session)
		return
	})
}

func (s *AcademicService) CheckInAttendance(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicAttendanceCheckinInput) (entity.AcademicAttendanceRecord, error) {
	if in.SessionID == "" || len(in.SessionID) > 128 || len(in.QRToken) < 1 || len(in.QRToken) > 256 {
		return entity.AcademicAttendanceRecord{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.attendance.checkin", in.SessionID, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicAttendanceRecord, target string, scope entity.Scope, err error) {
		rec, e := academicReadRecord[entity.AcademicAttendanceSessionRecord](tx, repository.AcademicAttendanceSessions, in.SessionID, true)
		if e != nil {
			err = e
			return
		}
		scope = rec.Session.Scope
		cls, e := academicReadRecord[entity.AcademicClass](tx, repository.AcademicClasses, rec.Session.ClassID, false)
		if e != nil {
			err = e
			return
		}
		if !cls.Active || !containsInt(cls.StudentIDs, p.User.ID) {
			err = authorization.ErrNotFound
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		if !rec.Session.IsOpen || !now.Before(rec.Session.ExpiresAt) {
			err = ErrAttendanceClosed
			return
		}
		if !practicumEqualToken(rec.TokenHash, in.QRToken) {
			err = authorization.ErrForbidden
			return
		}
		recordID := fmt.Sprintf("att-rec-%s-%d", in.SessionID, p.User.ID)
		target = recordID
		existing, e := academicReadRecord[entity.AcademicAttendanceRecord](tx, repository.AcademicAttendanceRecords, recordID, true)
		if e == nil {
			return existing, target, scope, nil
		} else if !errors.Is(e, repository.ErrAcademicNotFound) {
			err = e
			return
		}
		out = entity.AcademicAttendanceRecord{
			ID:           recordID,
			SessionID:    rec.Session.ID,
			ScheduleID:   rec.Session.ScheduleID,
			ClassID:      rec.Session.ClassID,
			UserID:       p.User.ID,
			UserName:     p.User.Name,
			Status:       "hadir",
			VerifiedByQR: true,
			Validated:    false,
			CheckedInAt:  now,
			Scope:        scope,
			Version:      1,
		}
		err = academicPut(tx, repository.AcademicAttendanceRecords, recordID, rec.Session.ID, p.User.ID, scope, out.Status, fmt.Sprintf("%s:%d", rec.Session.ID, p.User.ID), nil, out)
		return
	})
}

func (s *AcademicService) AttendanceRecords(ctx context.Context, p *entity.Principal, admin bool, sessionID string, page PracticumPage) ([]entity.AcademicAttendanceRecord, error) {
	out := []entity.AcademicAttendanceRecord{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		var filter repository.AcademicFilter
		if admin {
			scopes := academicFilterScopes(p, academicAttendanceRead, academicScheduleWrite)
			if len(scopes) == 0 {
				return nil
			}
			filter = repository.AcademicFilter{ParentID: sessionID, Scopes: scopes, Limit: page.Limit, Offset: page.Offset}
		} else {
			filter = repository.AcademicFilter{OwnerID: p.User.ID, ParentID: sessionID, Limit: page.Limit, Offset: page.Offset}
		}
		rows, err := tx.List(repository.AcademicAttendanceRecords, filter)
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.AcademicAttendanceRecord
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			out = append(out, rec)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CheckedInAt.After(out[j].CheckedInAt) })
		return nil
	})
	return out, err
}

func (s *AcademicService) ValidateAttendance(ctx context.Context, p *entity.Principal, c AcademicCommand, id string, in AcademicAttendanceValidateInput) (entity.AcademicAttendanceRecord, error) {
	if in.Version < 1 || (in.Status != "hadir" && in.Status != "izin" && in.Status != "alpha" && in.Status != "terlambat") || len(in.Notes) > 500 {
		return entity.AcademicAttendanceRecord{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.attendance.validate", id, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicAttendanceRecord, target string, scope entity.Scope, err error) {
		out, err = academicReadRecord[entity.AcademicAttendanceRecord](tx, repository.AcademicAttendanceRecords, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = academicRequire(p, scope, academicAttendanceRead, academicScheduleWrite); err != nil || replay != "" {
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
		out.Status = in.Status
		out.Validated = in.Validated
		out.ValidatedBy = &p.User.ID
		out.ValidatedAt = &now
		out.Notes = strings.TrimSpace(in.Notes)
		out.Version++
		if err = academicPut(tx, repository.AcademicAttendanceRecords, id, out.SessionID, out.UserID, scope, out.Status, fmt.Sprintf("%s:%d", out.SessionID, out.UserID), nil, out); err != nil {
			return
		}
		err = academicNotify(tx, out.UserID, "attendance.validated", "Presensi Divalidasi", fmt.Sprintf("Status presensi Anda diperbarui menjadi %s.", out.Status), scope, now)
		return
	})
}

func IsAcademicNotFound(err error) bool { return errors.Is(err, repository.ErrAcademicNotFound) }
