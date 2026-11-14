package middleware

import (
	"net/http"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
)

func Security(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/v1/") {
				w.Header().Set("Cache-Control", "private, no-store")
			}
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			if !cfg.IsAllowedOrigin(origin) {
				writeError(w, http.StatusForbidden, "FORBIDDEN_ORIGIN", "Origin tidak diizinkan.")
				return
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-CSRF-Token, Idempotency-Key")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Operation-ID, Retry-After")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
