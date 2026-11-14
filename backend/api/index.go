package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/algohub/algohub-rewrite/backend/internal/application"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/middleware"
)

var (
	handlerOnce sync.Once
	app         http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	handlerOnce.Do(func() {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		built, err := application.New(context.Background(), config.Load(), logger)
		if err == nil {
			app = built.Handler
			return
		}
		logger.Error("API configuration rejected", "error", err)
		app = middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "private, no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "SERVICE_UNAVAILABLE", "message": "API belum dikonfigurasi.", "request_id": middleware.GetRequestID(r.Context())}})
		}))
	})
	app.ServeHTTP(w, r)
}
