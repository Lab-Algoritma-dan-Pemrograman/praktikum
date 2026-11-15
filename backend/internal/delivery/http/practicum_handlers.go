package httpdelivery

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

// The identity decoder deliberately remains at 64KiB. Only Practicum endpoints
// accept 256KiB; model-level source, answer and snapshot limits are stricter.
func decodePracticumJSON(w http.ResponseWriter, r *http.Request, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return usecase.ErrValidation
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return jsonError(err)
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return jsonError(err)
	}
	return nil
}
func practicumPage(r *http.Request) (usecase.PracticumPage, error) {
	limit, err := boundedQuery(r, "limit", 25, 1, 100)
	if err != nil {
		return usecase.PracticumPage{}, err
	}
	offset, err := boundedQuery(r, "offset", 0, 0, 1000000)
	return usecase.PracticumPage{Limit: limit, Offset: offset}, err
}
func writePracticumError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrPracticumNotFound):
		writeError(w, 404, "NOT_FOUND", "Resource tidak ditemukan.")
	case errors.Is(err, usecase.ErrPracticumClosed):
		writeError(w, 409, "ATTEMPT_CLOSED", "Attempt atau activation sudah ditutup. Muat ulang state server.")
	case errors.Is(err, usecase.ErrPracticumBusy):
		w.Header().Set("Retry-After", "30")
		writeError(w, 429, "RUNNER_QUOTA_EXCEEDED", "Batas eksekusi tercapai. Coba lagi nanti.")
	case errors.Is(err, usecase.ErrPracticumUnavailable):
		writeError(w, 503, "PRACTICUM_PROVIDER_UNAVAILABLE", "Provider belum tersedia.")
	default:
		writeAuthError(w, err)
	}
}

type practicumHTTP func(http.ResponseWriter, *http.Request, *entity.Principal, usecase.PracticumCommand) (any, error)

func registerPracticum(mux *http.ServeMux, auth authHandlers, service *usecase.PracticumService, cronSecret string) {
	route := func(method, path, operation string, status int, fn practicumHTTP) {
		mux.HandleFunc(method+" "+path, auth.endpoint(method, operation, func(w http.ResponseWriter, r *http.Request) {
			if service == nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "Praktikum belum dikonfigurasi.")
				return
			}
			mutation := method != "GET"
			principal, _, ok := auth.session(w, r, mutation)
			if !ok {
				return
			}
			command := usecase.PracticumCommand{Key: r.Header.Get("Idempotency-Key"), RequestID: requestID(r), OperationID: operation}
			if mutation && (len(command.Key) < 1 || len(command.Key) > 128) {
				writeAuthError(w, usecase.ErrValidation)
				return
			}
			data, err := fn(w, r, principal, command)
			if err != nil {
				writePracticumError(w, err)
				return
			}
			writeSuccess(w, status, data)
		}))
	}
	route("GET", "/v1/practicum/sessions", "practicum.sessions.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Sessions(r.Context(), p, false, page)
	})
	route("GET", "/v1/practicum/sessions/{id}", "practicum.sessions.detail", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		return service.SessionDetail(r.Context(), p, r.PathValue("id"), false)
	})
	route("POST", "/v1/practicum/attempts", "practicum.attempts.start", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumStartInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Start(r.Context(), p, c, in)
	})
	route("GET", "/v1/practicum/attempts/{id}", "practicum.attempts.get", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		return service.Attempt(r.Context(), p, r.PathValue("id"), c)
	})
	route("PATCH", "/v1/practicum/attempts/{id}/answers", "practicum.attempts.answers", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumSaveInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveAnswers(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/practicum/attempts/{id}/submit", "practicum.attempts.submit", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in struct{}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Submit(r.Context(), p, c, r.PathValue("id"))
	})
	route("POST", "/v1/practicum/attempts/{id}/run", "practicum.attempts.run", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumRunInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Run(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/practicum/grades", "practicum.grades.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Grades(r.Context(), p, page)
	})
	route("GET", "/v1/practicum/runner", "practicum.runner.info", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		return service.RunnerInfo(), nil
	})
	route("GET", "/v1/admin/praktikum/sesi", "practicum.admin.sessions.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Sessions(r.Context(), p, true, page)
	})
	route("POST", "/v1/admin/praktikum/sesi", "practicum.admin.sessions.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumSessionInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateSession(r.Context(), p, c, in)
	})
	route("GET", "/v1/admin/praktikum/sesi/{id}", "practicum.admin.sessions.detail", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		return service.SessionDetail(r.Context(), p, r.PathValue("id"), true)
	})
	route("PATCH", "/v1/admin/praktikum/sesi/{id}", "practicum.admin.sessions.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumSessionUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateSession(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/praktikum/courses", "practicum.admin.courses.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumCourseInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateCourse(r.Context(), p, c, in)
	})
	route("PATCH", "/v1/admin/praktikum/courses/{id}", "practicum.admin.courses.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumCourseUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateCourse(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/praktikum/questions", "practicum.admin.questions.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Questions(r.Context(), p, r.URL.Query().Get("course_id"), page)
	})
	route("POST", "/v1/admin/praktikum/questions", "practicum.admin.questions.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumQuestionInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveQuestion(r.Context(), p, c, "", usecase.PracticumQuestionUpdate{PracticumQuestionInput: in})
	})
	route("PATCH", "/v1/admin/praktikum/questions/{id}", "practicum.admin.questions.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumQuestionUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveQuestion(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/praktikum/questions/{id}/rumpang", "practicum.admin.questions.rumpang", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in struct{}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.GenerateRumpang(r.Context(), p, c, r.PathValue("id"))
	})
	route("POST", "/v1/admin/praktikum/sesi/{id}/activate", "practicum.admin.activations.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumActivationInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Activate(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/praktikum/sesi/{id}/courses/{courseId}/close", "practicum.admin.activations.close", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in struct {
			ActivationID string `json:"activation_id"`
		}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Close(r.Context(), p, c, r.PathValue("id"), r.PathValue("courseId"), in.ActivationID)
	})
	route("GET", "/v1/admin/praktikum/monitoring", "practicum.admin.monitoring", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Monitoring(r.Context(), p, r.URL.Query().Get("session_id"), r.URL.Query().Get("activation_id"), page)
	})
	route("GET", "/v1/admin/praktikum/rekap", "practicum.admin.rekap", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Rekap(r.Context(), p, r.URL.Query().Get("course_id"), page)
	})
	route("GET", "/v1/admin/praktikum/penilaian/{attemptId}", "practicum.admin.grades.review", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		return service.Review(r.Context(), p, r.PathValue("attemptId"), c)
	})
	route("POST", "/v1/admin/praktikum/penilaian/bulk", "practicum.admin.grades.bulk", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in struct {
			AttemptIDs []string `json:"attempt_ids"`
		}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.BulkGrade(r.Context(), p, c, in.AttemptIDs)
	})
	route("POST", "/v1/admin/praktikum/penilaian/{attemptId}/grade", "practicum.admin.grades.process", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in struct{}
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.Grade(r.Context(), p, c, r.PathValue("attemptId"))
	})
	route("PUT", "/v1/admin/praktikum/penilaian/{attemptId}/manual", "practicum.admin.grades.manual", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.PracticumCommand) (any, error) {
		var in usecase.PracticumManualInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.ManualGrade(r.Context(), p, c, r.PathValue("attemptId"), in)
	})
	// No auth.endpoint here: cron authentication MUST precede even identity rate
	// limit I/O. Empty configuration always disables this independent boundary.
	mux.HandleFunc("/v1/internal/practicum/reconcile", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Operation-ID", "practicum.internal.reconcile")
		w.Header().Set("Cache-Control", "private, no-store")
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			writeError(w, 405, "METHOD_NOT_ALLOWED", "Method tidak didukung.")
			return
		}
		authorization := r.Header.Get("Authorization")
		if cronSecret == "" || !strings.HasPrefix(authorization, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(authorization, "Bearer ")), []byte(cronSecret)) != 1 {
			writeError(w, 401, "UNAUTHENTICATED", "Cron credential tidak valid.")
			return
		}
		if service == nil {
			writeError(w, 503, "SERVICE_UNAVAILABLE", "Praktikum belum dikonfigurasi.")
			return
		}
		result, err := service.Reconcile(r.Context(), usecase.PracticumCommand{RequestID: requestID(r), OperationID: "practicum.internal.reconcile"})
		if err != nil {
			writePracticumError(w, err)
			return
		}
		writeSuccess(w, 200, result)
	})
}
