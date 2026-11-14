package httpdelivery

import (
	"errors"
	"net/http"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func writeAcademicError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrAcademicNotFound):
		writeError(w, 404, "NOT_FOUND", "Resource tidak ditemukan.")
	case errors.Is(err, usecase.ErrAttendanceClosed):
		writeError(w, 409, "ATTENDANCE_CLOSED", "Sesi presensi sudah ditutup atau melewati batas waktu.")
	case errors.Is(err, usecase.ErrInsufficientStock):
		writeError(w, 409, "INSUFFICIENT_STOCK", "Stok alat tersedia tidak mencukupi untuk jumlah yang diminta.")
	case errors.Is(err, usecase.ErrRentalInvalid):
		writeError(w, 409, "CONFLICT", "Transisi status penyewaan tidak valid.")
	default:
		writeAuthError(w, err)
	}
}

type academicHTTP func(http.ResponseWriter, *http.Request, *entity.Principal, usecase.AcademicCommand) (any, error)

func registerAcademic(mux *http.ServeMux, auth authHandlers, service *usecase.AcademicService) {
	route := func(method, path, operation string, status int, fn academicHTTP) {
		mux.HandleFunc(method+" "+path, auth.endpoint(method, operation, func(w http.ResponseWriter, r *http.Request) {
			if service == nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "SIAKAD belum dikonfigurasi.")
				return
			}
			mutation := method != "GET"
			principal, _, ok := auth.session(w, r, mutation)
			if !ok {
				return
			}
			command := usecase.AcademicCommand{Key: r.Header.Get("Idempotency-Key"), RequestID: requestID(r), OperationID: operation}
			if mutation && (len(command.Key) < 1 || len(command.Key) > 128) {
				writeAuthError(w, usecase.ErrValidation)
				return
			}
			data, err := fn(w, r, principal, command)
			if err != nil {
				writeAcademicError(w, err)
				return
			}
			writeSuccess(w, status, data)
		}))
	}
	route("GET", "/v1/academic/classes", "academic.classes.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Classes(r.Context(), p, false, page)
	})
	route("GET", "/v1/academic/schedules", "academic.schedules.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Schedules(r.Context(), p, false, page)
	})
	route("GET", "/v1/academic/attendance", "academic.attendance.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.AttendanceRecords(r.Context(), p, false, "", page)
	})
	route("POST", "/v1/academic/attendance", "academic.attendance.checkin", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicAttendanceCheckinInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CheckInAttendance(r.Context(), p, c, in)
	})
	route("GET", "/v1/inventory/items", "inventory.items.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.InventoryItems(r.Context(), p, page)
	})
	route("GET", "/v1/inventory/rentals", "inventory.rentals.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Rentals(r.Context(), p, false, page)
	})
	route("POST", "/v1/inventory/rentals", "inventory.rentals.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.InventoryRentalCreateInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.RequestRental(r.Context(), p, c, in)
	})
	route("GET", "/v1/academic/notifications", "academic.notifications.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Notifications(r.Context(), p, page)
	})
	route("GET", "/v1/admin/siakad/kelas", "academic.admin.classes.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Classes(r.Context(), p, true, page)
	})
	route("POST", "/v1/admin/siakad/kelas", "academic.admin.classes.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicClassInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateClass(r.Context(), p, c, in)
	})
	route("PATCH", "/v1/admin/siakad/kelas/{id}", "academic.admin.classes.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicClassUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateClass(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/siakad/jadwal", "academic.admin.schedules.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Schedules(r.Context(), p, true, page)
	})
	route("POST", "/v1/admin/siakad/jadwal", "academic.admin.schedules.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicScheduleInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateSchedule(r.Context(), p, c, in)
	})
	route("POST", "/v1/admin/siakad/absensi/sesi", "academic.admin.attendance.open", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicAttendanceOpenInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.OpenAttendanceSession(r.Context(), p, c, in)
	})
	route("POST", "/v1/admin/siakad/absensi/sesi/{id}/close", "academic.admin.attendance.close", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in struct{}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CloseAttendanceSession(r.Context(), p, c, r.PathValue("id"))
	})
	route("GET", "/v1/admin/siakad/absensi/records", "academic.admin.attendance.records", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.AttendanceRecords(r.Context(), p, true, r.URL.Query().Get("session_id"), page)
	})
	route("PATCH", "/v1/admin/siakad/absensi/records/{id}", "academic.admin.attendance.validate", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.AcademicAttendanceValidateInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.ValidateAttendance(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/siakad/inventaris", "inventory.admin.items.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.InventoryItemInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateInventoryItem(r.Context(), p, c, in)
	})
	route("PATCH", "/v1/admin/siakad/inventaris/{id}", "inventory.admin.items.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.InventoryItemUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateInventoryItem(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/siakad/penyewaan", "inventory.admin.rentals.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Rentals(r.Context(), p, true, page)
	})
	route("PATCH", "/v1/admin/siakad/penyewaan/{id}", "inventory.admin.rentals.decide", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		var in usecase.InventoryRentalDecideInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.DecideRental(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/siakad/laporan", "academic.admin.reports.get", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.AcademicCommand) (any, error) {
		return service.Reports(r.Context(), p)
	})
}
