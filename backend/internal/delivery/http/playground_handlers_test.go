package httpdelivery

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/preview"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func newPlaygroundHarness(t *testing.T, providerURL string) *authHarness {
	t.Helper()
	h := &authHarness{repo: repository.NewMemoryIdentityRepository(), now: time.Now().UTC()}
	origins := []string{"https://" + authHost, "https://" + adminHost, "https://elearning.algohub.web.id", "https://praktikum.algohub.web.id"}
	identity := usecase.NewIdentityService(usecase.IdentityOptions{Repository: h.repo, Provider: auth.NewFixtureProvider(), Allowlist: returnto.FromOrigins(origins), Now: func() time.Time { return h.now }})
	h.handler = NewRouter(Dependencies{
		Config:     config.Config{Environment: "local", RequestTimeout: 5 * time.Second, AllowedOrigins: origins, CookieSecure: true, AuthRateLimit: 120, UserRateLimit: 2000},
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Readiness:  usecase.NewReadinessService(repository.NoopReadinessRepository{}),
		Identity:   identity,
		Repository: h.repo,
		Playground: preview.New(providerURL, "fixture-token"),
	})
	return h
}

func playgroundBody(language, source string) string {
	raw, _ := json.Marshal(map[string]string{"language": language, "source": source})
	return string(raw)
}

func TestPlaygroundFallbackIsUnavailableWithoutToken(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "mahasiswa", authHost)
	w := h.request("POST", "/v1/playground/run", playgroundBody("c", "int main(void){return 0;}"), authHost, cookies, true)
	requireStatus(t, w, http.StatusServiceUnavailable)
	if !strings.Contains(w.Body.String(), "PLAYGROUND_UNAVAILABLE") {
		t.Fatalf("unexpected body %s", w.Body.String())
	}
}

func TestPlaygroundFallbackRequiresSession(t *testing.T) {
	h := newPlaygroundHarness(t, "https://glot.invalid/api/mux")
	w := h.request("POST", "/v1/playground/run", playgroundBody("c", "int main(void){return 0;}"), authHost, nil, true)
	requireStatus(t, w, http.StatusUnauthorized)
}

func TestPlaygroundFallbackReturnsNonAuthoritativePreview(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"stdout": "42\n", "stderr": "", "error": "", "duration": 1})
	}))
	defer provider.Close()
	h := newPlaygroundHarness(t, provider.URL+"/api/mux")
	cookies := h.login(t, "mahasiswa", authHost)
	w := h.request("POST", "/v1/playground/run", playgroundBody("c", "int main(void){return 0;}"), authHost, cookies, true)
	requireStatus(t, w, http.StatusOK)
	body := responseData[playgroundResponse](t, w)
	if body.Status != preview.StatusSuccess || body.Stdout != "42\n" || body.Provider != "glot" || body.Authoritative {
		t.Fatalf("unexpected preview response %+v", body)
	}
}

func TestPlaygroundFallbackValidatesInput(t *testing.T) {
	h := newPlaygroundHarness(t, "https://glot.invalid/api/mux")
	cookies := h.login(t, "mahasiswa", authHost)
	requireStatus(t, h.request("POST", "/v1/playground/run", playgroundBody("ruby", "puts 1"), authHost, cookies, true), http.StatusUnprocessableEntity)
	requireStatus(t, h.request("POST", "/v1/playground/run", playgroundBody("c", strings.Repeat("a", 64<<10+1)), authHost, cookies, true), http.StatusUnprocessableEntity)
}

func TestPlaygroundFallbackRateLimitsPerUser(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"stdout": "", "stderr": "", "error": ""})
	}))
	defer provider.Close()
	h := newPlaygroundHarness(t, provider.URL+"/api/mux")
	cookies := h.login(t, "mahasiswa", authHost)
	for attempt := 1; attempt <= playgroundPerMinute; attempt++ {
		requireStatus(t, h.request("POST", "/v1/playground/run", playgroundBody("c", "int main(void){return 0;}"), authHost, cookies, true), http.StatusOK)
	}
	limited := h.request("POST", "/v1/playground/run", playgroundBody("c", "int main(void){return 0;}"), authHost, cookies, true)
	requireStatus(t, limited, http.StatusTooManyRequests)
	if limited.Header().Get("Retry-After") == "" {
		t.Fatal("rate limit response must include Retry-After")
	}
}

func TestPlaygroundHourWindowIsBounded(t *testing.T) {
	repo := repository.NewMemoryIdentityRepository()
	request := httptest.NewRequest(http.MethodPost, "/v1/playground/run", nil)
	window := time.Now().UTC().Truncate(time.Hour)
	for attempt := 1; attempt <= playgroundPerHour; attempt++ {
		if !playgroundLimit(httptest.NewRecorder(), request, repo, "playground:hour:fixture", window, playgroundPerHour, 60) {
			t.Fatalf("call %d within the hourly budget was rejected", attempt)
		}
	}
	recorder := httptest.NewRecorder()
	if playgroundLimit(recorder, request, repo, "playground:hour:fixture", window, playgroundPerHour, 60) {
		t.Fatal("hourly limit did not reject the extra call")
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" {
		t.Fatalf("unexpected Retry-After %q", recorder.Header().Get("Retry-After"))
	}
}
