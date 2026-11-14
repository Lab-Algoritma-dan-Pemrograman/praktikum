package httpdelivery

import (
	"encoding/json"
	"net/http"

	"github.com/algohub/algohub-rewrite/backend/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeSuccess(w http.ResponseWriter, status int, data any) {
	meta := map[string]string{"request_id": middleware.GetRequestIDFromHeader(w)}
	if operationID := w.Header().Get("X-Operation-ID"); operationID != "" {
		meta["operation_id"] = operationID
	}
	writeJSON(w, status, map[string]any{
		"data": data,
		"meta": meta,
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	details := map[string]any(nil)
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":         code,
			"message":      message,
			"details":      details,
			"request_id":   w.Header().Get("X-Request-ID"),
			"operation_id": w.Header().Get("X-Operation-ID"),
		},
	})
}
