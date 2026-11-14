package httpdelivery

import (
	"errors"
	"net/http"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func writeLearningError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrLearningNotFound):
		writeError(w, 404, "NOT_FOUND", "Resource tidak ditemukan.")
	case errors.Is(err, usecase.ErrLearningLocked):
		writeError(w, 403, "LEVEL_LOCKED", "Level ini masih terkunci. Kumpulkan XP yang dibutuhkan atau hubungi instruktur.")
	case errors.Is(err, usecase.ErrLearningQuizIncorrect):
		writeError(w, 409, "QUIZ_INCORRECT", "Jawaban kuis belum tepat. Periksa kembali materi sebelum menyelesaikan pelajaran.")
	case errors.Is(err, usecase.ErrLearningCodeNotVerified):
		writeError(w, 409, "CODE_NOT_VERIFIED", "Jalankan kode jawaban saat ini di browser dan pastikan seluruh testcase lulus sebelum menyelesaikan pelajaran.")
	case errors.Is(err, usecase.ErrLearningWeeklyLimit):
		writeError(w, 409, "WEEKLY_LIMIT_REACHED", "Batas bermain Bug Hunt minggu ini sudah tercapai.")
	case errors.Is(err, usecase.ErrLearningHintQuota):
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "HINT_QUOTA_EXCEEDED", "Batas permintaan hint per menit tercapai. Coba lagi sebentar lagi.")
	case errors.Is(err, usecase.ErrLearningBusy):
		w.Header().Set("Retry-After", "30")
		writeError(w, 429, "RUNNER_QUOTA_EXCEEDED", "Batas eksekusi tercapai. Coba lagi nanti.")
	case errors.Is(err, usecase.ErrLearningUnavailable):
		writeError(w, 503, "LEARNING_PROVIDER_UNAVAILABLE", "Runner belum tersedia.")
	default:
		writeAuthError(w, err)
	}
}

type learningHTTP func(http.ResponseWriter, *http.Request, *entity.Principal, usecase.LearningCommand) (any, error)

func registerLearning(mux *http.ServeMux, auth authHandlers, service *usecase.LearningService) {
	route := func(method, path, operation string, status int, fn learningHTTP) {
		mux.HandleFunc(method+" "+path, auth.endpoint(method, operation, func(w http.ResponseWriter, r *http.Request) {
			if service == nil {
				writeError(w, 503, "SERVICE_UNAVAILABLE", "E-learning belum dikonfigurasi.")
				return
			}
			mutation := method != "GET"
			principal, _, ok := auth.session(w, r, mutation)
			if !ok {
				return
			}
			command := usecase.LearningCommand{Key: r.Header.Get("Idempotency-Key"), RequestID: requestID(r), OperationID: operation}
			if mutation && (len(command.Key) < 1 || len(command.Key) > 128) {
				writeAuthError(w, usecase.ErrValidation)
				return
			}
			data, err := fn(w, r, principal, command)
			if err != nil {
				writeLearningError(w, err)
				return
			}
			writeSuccess(w, status, data)
		}))
	}
	route("GET", "/v1/learning/levels", "learning.levels.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.Levels(r.Context(), p)
	})
	route("GET", "/v1/learning/modules/{id}", "learning.modules.get", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.Module(r.Context(), p, r.PathValue("id"))
	})
	route("GET", "/v1/learning/lessons/{id}", "learning.lessons.get", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.Lesson(r.Context(), p, r.PathValue("id"))
	})
	route("PUT", "/v1/learning/lessons/{id}/progress", "learning.lessons.progress", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningProgressInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveProgress(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/learning/playground/run", "learning.playground.run", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningPlaygroundInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.PlaygroundRun(r.Context(), p, c, in)
	})
	route("POST", "/v1/learning/hints", "learning.hints.create", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningHintInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.RequestHint(r.Context(), p, c, in)
	})
	route("GET", "/v1/learning/leaderboard", "learning.leaderboard.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Leaderboard(r.Context(), p, page)
	})
	route("GET", "/v1/learning/achievements", "learning.achievements.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.Achievements(r.Context(), p)
	})
	route("GET", "/v1/learning/bug-hunt", "learning.bughunt.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.BugHuntList(r.Context(), p)
	})
	route("POST", "/v1/learning/bug-hunt/attempts", "learning.bughunt.attempt", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningBugHuntAttemptInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SubmitBugHunt(r.Context(), p, c, in)
	})
	route("GET", "/v1/admin/elearning/kurikulum", "learning.admin.curriculum.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		return service.AdminCurriculum(r.Context(), p)
	})
	route("POST", "/v1/admin/elearning/levels", "learning.admin.levels.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningLevelInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateLevel(r.Context(), p, c, in)
	})
	route("PATCH", "/v1/admin/elearning/levels/{id}", "learning.admin.levels.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningLevelUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateLevel(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/elearning/modules", "learning.admin.modules.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningModuleInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.CreateModule(r.Context(), p, c, in)
	})
	route("PATCH", "/v1/admin/elearning/modules/{id}", "learning.admin.modules.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningModuleUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.UpdateModule(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("POST", "/v1/admin/elearning/lessons", "learning.admin.lessons.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningLessonInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveLesson(r.Context(), p, c, "", usecase.LearningLessonUpdate{LearningLessonInput: in})
	})
	route("PATCH", "/v1/admin/elearning/lessons/{id}", "learning.admin.lessons.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningLessonUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveLesson(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/elearning/bug-hunt", "learning.admin.bughunt.list", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.AdminBugHuntList(r.Context(), p, page)
	})
	route("POST", "/v1/admin/elearning/bug-hunt", "learning.admin.bughunt.create", 201, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningBugHuntInput
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveBugHunt(r.Context(), p, c, "", usecase.LearningBugHuntUpdate{LearningBugHuntInput: in})
	})
	route("PATCH", "/v1/admin/elearning/bug-hunt/{id}", "learning.admin.bughunt.update", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		var in usecase.LearningBugHuntUpdate
		if err := decodePracticumJSON(w, r, &in); err != nil {
			return nil, err
		}
		return service.SaveBugHunt(r.Context(), p, c, r.PathValue("id"), in)
	})
	route("GET", "/v1/admin/elearning/monitoring", "learning.admin.monitoring", 200, func(w http.ResponseWriter, r *http.Request, p *entity.Principal, c usecase.LearningCommand) (any, error) {
		page, err := practicumPage(r)
		if err != nil {
			return nil, err
		}
		return service.Monitoring(r.Context(), p, page)
	})
}
