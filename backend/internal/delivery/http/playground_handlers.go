package httpdelivery

import (
	"errors"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/platform/preview"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

const (
	playgroundPerMinute = 5
	playgroundPerHour   = 30
)

type playgroundInput struct {
	Language string `json:"language"`
	Source   string `json:"source"`
	Stdin    string `json:"stdin"`
}

type playgroundResponse struct {
	Status        string `json:"status"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
	Truncated     bool   `json:"truncated"`
	DurationMS    int64  `json:"duration_ms"`
	Provider      string `json:"provider"`
	Authoritative bool   `json:"authoritative"`
}

func registerPlayground(mux *http.ServeMux, auth authHandlers, client *preview.Client) {
	mux.HandleFunc("POST /v1/playground/run", auth.endpoint("POST", "playground.run", func(w http.ResponseWriter, r *http.Request) {
		principal, _, ok := auth.session(w, r, true)
		if !ok {
			return
		}
		if client == nil || !client.Enabled() {
			writeError(w, http.StatusServiceUnavailable, "PLAYGROUND_UNAVAILABLE", "Fallback server belum dikonfigurasi.")
			return
		}
		userKey := strconv.FormatInt(principal.User.ID, 10)
		now := time.Now().UTC()
		if !playgroundLimit(w, r, auth.repository, "playground:minute:"+userKey, now.Truncate(time.Minute), playgroundPerMinute, 60-now.Second()) {
			return
		}
		if !playgroundLimit(w, r, auth.repository, "playground:hour:"+userKey, now.Truncate(time.Hour), playgroundPerHour, 3600-(now.Minute()*60+now.Second())) {
			return
		}
		var input playgroundInput
		if err := decodePracticumJSON(w, r, &input); err != nil {
			writeAuthError(w, err)
			return
		}
		if (input.Language != "c" && input.Language != "python") ||
			len(input.Source) > 64<<10 || len(input.Stdin) > 16<<10 ||
			!utf8.ValidString(input.Source) || !utf8.ValidString(input.Stdin) {
			writeAuthError(w, usecase.ErrValidation)
			return
		}
		result, err := client.Run(r.Context(), input.Language, input.Source, input.Stdin)
		switch {
		case errors.Is(err, preview.ErrNotConfigured):
			writeError(w, http.StatusServiceUnavailable, "PLAYGROUND_UNAVAILABLE", "Fallback server belum dikonfigurasi.")
			return
		case errors.Is(err, preview.ErrProvider):
			result = preview.Result{Status: preview.StatusProviderError, DurationMS: result.DurationMS}
		case errors.Is(err, preview.ErrTimeout):
			result = preview.Result{Status: preview.StatusTimeout, DurationMS: result.DurationMS}
		case err != nil:
			writeError(w, http.StatusServiceUnavailable, "PLAYGROUND_UNAVAILABLE", "Fallback server belum tersedia.")
			return
		}
		writeSuccess(w, http.StatusOK, playgroundResponse{
			Status: result.Status, Stdout: result.Stdout, Stderr: result.Stderr,
			Truncated: result.Truncated, DurationMS: result.DurationMS,
			Provider: "glot", Authoritative: false,
		})
	}))
}

func playgroundLimit(w http.ResponseWriter, r *http.Request, store repository.IdentityRepository, key string, window time.Time, limit int, retryAfter int) bool {
	allowed, err := store.TakeRateLimit(r.Context(), security.Hash(key), window, limit)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Layanan preview sedang tidak tersedia.")
		return false
	}
	if !allowed {
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan preview. Coba lagi nanti.")
		return false
	}
	return true
}
