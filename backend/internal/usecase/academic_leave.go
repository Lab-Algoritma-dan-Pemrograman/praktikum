package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

var ErrLeaveInvalid = errors.New("invalid leave request state transition")

func (s *AcademicService) CreateLeave(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicLeaveCreateInput) (entity.AcademicLeaveRequest, error) {
	if !validPracticumTitle(in.ScheduleID) || !validLeaveKind(in.Kind) || len(in.Reason) < 1 || len(in.Reason) > 500 || in.OriginalAt.IsZero() {
		return entity.AcademicLeaveRequest{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.leave.create", in.ScheduleID, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicLeaveRequest, target string, scope entity.Scope, err error) {
		sched, e := academicReadRecord[entity.AcademicSchedule](tx, repository.AcademicSchedules, in.ScheduleID, true)
		if e != nil {
			err = e
			return
		}
		scope = sched.Scope
		scope.SessionID = ""
		cls, e := academicReadRecord[entity.AcademicClass](tx, repository.AcademicClasses, sched.ClassID, false)
		if e != nil {
			err = e
			return
		}
		if !cls.Active || !containsInt(cls.StudentIDs, p.User.ID) {
			err = authorization.ErrNotFound
			return
		}
		if replay != "" {
			out, err = academicReadRecord[entity.AcademicLeaveRequest](tx, repository.AcademicLeaveRequests, replay, false)
			return out, replay, out.Scope, err
		}
		id := academicID("leave")
		out = entity.AcademicLeaveRequest{
			ID:         id,
			UserID:     p.User.ID,
			UserName:   p.User.Name,
			ScheduleID: sched.ID,
			ClassID:    sched.ClassID,
			Kind:       in.Kind,
			Reason:     strings.TrimSpace(in.Reason),
			OriginalAt: in.OriginalAt.UTC(),
			Status:     "pending",
			Scope:      scope,
			Version:    1,
		}
		target = id
		err = academicPut(tx, repository.AcademicLeaveRequests, id, in.ScheduleID, p.User.ID, scope, out.Status, "", nil, out)
		return
	})
}

func validLeaveKind(kind string) bool {
	switch kind {
	case "izin", "sakit", "reschedule":
		return true
	}
	return false
}

func (s *AcademicService) MyLeaves(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.AcademicLeaveRequest, error) {
	out := []entity.AcademicLeaveRequest{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicLeaveRequests, repository.AcademicFilter{OwnerID: p.User.ID, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.AcademicLeaveRequest
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			out = append(out, rec)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].OriginalAt.After(out[j].OriginalAt) })
		return nil
	})
	return out, err
}

func (s *AcademicService) LeaveRequests(ctx context.Context, p *entity.Principal, page PracticumPage) ([]entity.AcademicLeaveRequest, error) {
	out := []entity.AcademicLeaveRequest{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	scopes := academicFilterScopes(p, academicLeaveDecide, academicAttendanceRead, academicScheduleWrite)
	if len(scopes) == 0 {
		return out, nil
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicLeaveRequests, repository.AcademicFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.AcademicLeaveRequest
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			out = append(out, rec)
		}
		sort.Slice(out, func(i, j int) bool {
			if (out[i].Status == "pending") != (out[j].Status == "pending") {
				return out[i].Status == "pending"
			}
			return out[i].OriginalAt.After(out[j].OriginalAt)
		})
		return nil
	})
	return out, err
}

func (s *AcademicService) DecideLeave(ctx context.Context, p *entity.Principal, c AcademicCommand, id string, in AcademicLeaveDecideInput) (entity.AcademicLeaveRequest, error) {
	if in.Version < 1 || (in.Action != "approve" && in.Action != "reject" && in.Action != "reschedule") {
		return entity.AcademicLeaveRequest{}, ErrValidation
	}
	if in.Action == "reschedule" && (in.RescheduleAt == nil || in.RescheduleAt.IsZero()) {
		return entity.AcademicLeaveRequest{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "academic.leave."+in.Action, id, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicLeaveRequest, target string, scope entity.Scope, err error) {
		out, err = academicReadRecord[entity.AcademicLeaveRequest](tx, repository.AcademicLeaveRequests, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = academicRequire(p, scope, academicLeaveDecide); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version || out.Status != "pending" {
			err = ErrLeaveInvalid
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		out.DecidedAt = &now
		out.DecidedBy = &p.User.ID
		out.Version++
		switch in.Action {
		case "approve":
			out.Status = "approved"
		case "reject":
			out.Status = "rejected"
		case "reschedule":
			out.Status = "rescheduled"
			at := in.RescheduleAt.UTC()
			out.RescheduleAt = &at
			out.RescheduledBy = &p.User.ID
		}
		err = academicPut(tx, repository.AcademicLeaveRequests, id, out.ScheduleID, out.UserID, scope, out.Status, "", nil, out)
		if err == nil {
			err = academicNotify(tx, out.UserID, "leave."+in.Action, "Status Izin Diperbarui", "Pengajuan "+out.Kind+" Anda: "+out.Status+".", scope, now)
		}
		return
	})
}
