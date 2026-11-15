package httpdelivery

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func writeLaporanError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrAcademicNotFound):
		writeError(w, 404, "NOT_FOUND", "Laporan besar tidak ditemukan.")
	case errors.Is(err, usecase.ErrLaporanWindow):
		writeError(w, 409, "LAPORAN_WINDOW", err.Error())
	case errors.Is(err, usecase.ErrLaporanStorage):
		writeError(w, 503, "LAPORAN_STORAGE_UNAVAILABLE", "Penyimpanan berkas laporan besar belum dikonfigurasi.")
	case errors.Is(err, repository.ErrAcademicConflict):
		writeError(w, 409, "CONFLICT", "Data laporan besar berubah, muat ulang lalu coba lagi.")
	default:
		writeAcademicError(w, err)
	}
}

type laporanHTTP func(http.ResponseWriter, *http.Request, *entity.Principal, usecase.LaporanCommand) (any, error)

func registerLaporan(mux *http.ServeMux, auth authHandlers, service *usecase.LaporanService) {
	route := func(method, path, operation string, status int, fn laporanHTTP) {
		mux.HandleFunc(method+" "+path, auth.endpoint(method, operation, func(w http.ResponseWriter, r *http.Request) {
			if service == nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "Laporan besar belum dikonfigurasi.")
				return
			}
			mutation := method != "GET"
			principal, _, ok := auth.session(w, r, mutation)
			if !ok {
				return
			}
			command := usecase.LaporanCommand{Key: r.Header.Get("Idempotency-Key"), RequestID: requestID(r), OperationID: operation}
			if mutation && (len(command.Key) < 1 || len(command.Key) > 128) {
				writeAuthError(w, usecase.ErrValidation)
				return
			}
			data, err := fn(w, r, principal, command)
			if err != nil {
				writeLaporanError(w, err)
				return
			}
			writeSuccess(w, status, data)
		}))
	}
	route("GET", "/v1/laporan/saya", "laporan.status", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		return service.StatusSaya(r.Context(), p)
	})
	route("POST", "/v1/laporan/upload-url", "laporan.upload.url", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanUploadURLRequest
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UploadURL(r.Context(), p, c, in)
	})
	route("POST", "/v1/laporan/kumpul", "laporan.kumpul", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanKumpulInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Kumpul(r.Context(), p, c, in)
	})
	route("POST", "/v1/laporan/batalkan", "laporan.batalkan", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in struct{}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.BatalkanKumpul(r.Context(), p, c)
	})
	route("GET", "/v1/laporan/{id}/berkas/{userId}", "laporan.berkas", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		download, err := strconv.ParseBool(r.URL.Query().Get("download"))
		if err != nil {
			download = false
		}
		return service.SignedBerkas(r.Context(), p, r.PathValue("id"), r.PathValue("userId"), r.URL.Query().Get("jenis"), download)
	})
	route("GET", "/v1/admin/laporan", "laporan.admin.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		return service.List(r.Context(), p)
	})
	route("POST", "/v1/admin/laporan", "laporan.admin.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanCreateInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Create(r.Context(), p, c, in)
	})
	route("PUT", "/v1/admin/laporan/{id}", "laporan.admin.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanUpdateInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Update(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/laporan/{id}/pengumpulan", "laporan.admin.pengumpulan", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Pengumpulan(r.Context(), p, r.PathValue("id"), page)
	})
	route("POST", "/v1/admin/laporan/{id}/mark/{userId}", "laporan.admin.mark", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanMarkInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Mark(r.Context(), p, c, r.PathValue("id"), r.PathValue("userId"), in)
	})
	route("POST", "/v1/admin/laporan/{id}/override/{userId}", "laporan.admin.override", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanOverrideInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SetOverride(r.Context(), p, c, r.PathValue("id"), r.PathValue("userId"), in)
	})
	route("DELETE", "/v1/admin/laporan/{id}/override/{userId}", "laporan.admin.revokeOverride", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LaporanCommand) (any, error) {
		var in usecase.LaporanOverrideInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.RevokeOverride(r.Context(), p, c, r.PathValue("id"), r.PathValue("userId"), in)
	})
}
