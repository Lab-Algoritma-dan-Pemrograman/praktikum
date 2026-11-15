package httpdelivery

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/middleware"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/preview"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type Dependencies struct {
	Config      config.Config
	Logger      *slog.Logger
	Readiness   usecase.ReadinessService
	OpenAPIYAML []byte
	OpenAPIJSON []byte
	Identity    usecase.IdentityService
	Repository  repository.IdentityRepository
	Practicum   *usecase.PracticumService
	Learning    *usecase.LearningService
	Academic    *usecase.AcademicService
	Laporan     *usecase.LaporanService
	Playground  *preview.Client
}

func NewRouter(deps Dependencies) http.Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		readinessHandler(w, r, deps.Readiness)
	})
	auth := authHandlers{identity: deps.Identity, config: deps.Config, repository: deps.Repository}
	mux.HandleFunc("/v1/auth/sso/exchange", auth.endpoint("POST", "auth.sso.exchange", auth.exchange))
	mux.HandleFunc("/v1/auth/sso/prepare", auth.endpoint("POST", "auth.sso.prepare", auth.prepare))
	mux.HandleFunc("/v1/auth/password", auth.endpoint("POST", "auth.password.login", auth.password))
	mux.HandleFunc("/v1/auth/callback", auth.endpoint("POST", "auth.callback", auth.callback))
	mux.HandleFunc("/v1/auth/me", auth.endpoint("GET", "auth.me", auth.me))
	mux.HandleFunc("/v1/auth/refresh", auth.endpoint("POST", "auth.refresh", auth.refresh))
	mux.HandleFunc("/v1/auth/logout", auth.endpoint("POST", "auth.logout", auth.logout))
	mux.HandleFunc("/v1/auth/sessions", auth.endpoint("GET", "auth.sessions.list", auth.sessions))
	mux.HandleFunc("/v1/auth/sessions/", auth.endpoint("DELETE", "auth.sessions.revoke", auth.revokeSession))
	mux.HandleFunc("/v1/me", func(w http.ResponseWriter, r *http.Request) {
		operation := "identity.profile"
		if r.Method == "PATCH" {
			operation = "identity.profile.update"
		}
		auth.endpoint("GET, PATCH", operation, auth.profile)(w, r)
	})
	mux.HandleFunc("/v1/admin/access", auth.endpoint("GET", "admin.access", auth.adminAccess))
	mux.HandleFunc("/v1/admin/overview", auth.endpoint("GET", "admin.overview", auth.adminOverview))
	mux.HandleFunc("/v1/admin/permissions", auth.endpoint("GET", "admin.permissions", auth.permissions))
	mux.HandleFunc("/v1/admin/auditlog", auth.endpoint("GET", "admin.auditlog", auth.auditLog))
	mux.HandleFunc("/v1/admin/permission-catalog", auth.endpoint("GET", "admin.permission-catalog", auth.permissionCatalog))
	mux.HandleFunc("/v1/admin/users", auth.endpoint("GET", "admin.users.list", auth.users))
	mux.HandleFunc("/v1/admin/users/", func(w http.ResponseWriter, r *http.Request) {
		operation := "admin.users.access"
		if r.Method == "PUT" {
			operation = "admin.users.access.update"
		}
		auth.endpoint("GET, PUT", operation, auth.userAccess)(w, r)
	})
	if deps.Config.PracticumEnabled {
		registerPracticum(mux, auth, deps.Practicum, deps.Config.PracticumCronSecret)
	}
	if deps.Config.ElearningEnabled {
		registerLearning(mux, auth, deps.Learning, deps.Config.PracticumCronSecret)
	}
	if deps.Config.SiakadEnabled {
		registerAcademic(mux, auth, deps.Academic)
		registerLaporan(mux, auth, deps.Laporan)
	}
	registerPlayground(mux, auth, deps.Playground)
	mux.HandleFunc("/docs", docsHandler)
	mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/openapi.yaml":
			if len(deps.OpenAPIYAML) == 0 {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "OpenAPI YAML belum tersedia.")
				return
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(deps.OpenAPIYAML)
		case "/docs/openapi.json":
			if len(deps.OpenAPIJSON) == 0 {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "OpenAPI JSON belum tersedia.")
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(deps.OpenAPIJSON)
		default:
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource tidak ditemukan.")
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource tidak ditemukan.")
	})

	return middleware.RequestID(
		middleware.Recovery(deps.Logger)(
			middleware.Security(deps.Config)(
				middleware.Timeout(deps.Config.RequestTimeout)(
					middleware.Logging(deps.Logger)(mux),
				),
			),
		),
	)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Operation-ID", "getHealth")
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method tidak didukung.")
		return
	}
	writeSuccess(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readinessHandler(w http.ResponseWriter, r *http.Request, service usecase.ReadinessService) {
	w.Header().Set("X-Operation-ID", "getReadiness")
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method tidak didukung.")
		return
	}
	if err := service.Check(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service belum siap.")
		return
	}
	writeSuccess(w, http.StatusOK, map[string]string{"status": "ready"})
}

func docsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method tidak didukung.")
		return
	}
	const page = `<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Algohub API Docs</title></head>
<body><main><h1>Algohub API</h1><p>OpenAPI 3.1 contract untuk API Algohub.</p><ul><li><a href="/docs/openapi.yaml">OpenAPI YAML</a></li><li><a href="/docs/openapi.json">OpenAPI JSON</a></li></ul></main></body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, page)
}
