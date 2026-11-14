package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered", "request_id", GetRequestID(r.Context()), "panic", recovered)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Terjadi kesalahan internal.")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"details":    nil,
			"request_id": w.Header().Get("X-Request-ID"),
		},
	})
}
