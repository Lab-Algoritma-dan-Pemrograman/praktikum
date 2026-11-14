package httpdelivery

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func testRouter() http.Handler {
	return NewRouter(Dependencies{
		Config:      config.Config{RequestTimeout: 1, AllowedOrigins: []string{"http://localhost:5173"}},
		Logger:      slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)),
		Readiness:   usecase.NewReadinessService(repository.NoopReadinessRepository{}),
		OpenAPIYAML: []byte("openapi: 3.1.0\n"),
		OpenAPIJSON: []byte(`{"openapi":"3.1.0"}`),
	})
}

func TestHealthIncludesRequestIDAndEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	testRouter().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if requestID := recorder.Header().Get("X-Request-ID"); !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("expected generated request id, got %q", requestID)
	}
	if !strings.Contains(recorder.Body.String(), `"data"`) || !strings.Contains(recorder.Body.String(), `"meta"`) {
		t.Fatalf("expected success envelope, got %s", recorder.Body.String())
	}
}

func TestReadinessIsNotReportedReadyBeforeDatabaseAdapter(t *testing.T) {
	recorder := httptest.NewRecorder()
	testRouter().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("expected readiness error, got %s", recorder.Body.String())
	}
}

func TestOpenAPIDocumentEndpoints(t *testing.T) {
	for _, path := range []string{"/docs", "/docs/openapi.yaml", "/docs/openapi.json"} {
		recorder := httptest.NewRecorder()
		testRouter().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, recorder.Code)
		}
	}
}

func TestLearningHTTPCompletionAcceptsExplicitBrowserClaim(t *testing.T) {
	ctx := context.Background()
	identityRepo := repository.NewMemoryIdentityRepository()
	learningRepo := repository.NewMemoryLearningRepository()
	if err := repository.SeedLearningFixtures(ctx, learningRepo); err != nil {
		t.Fatal(err)
	}
	origins := []string{"https://elearning.algohub.web.id"}
	identity := usecase.NewIdentityService(usecase.IdentityOptions{Repository: identityRepo, Provider: auth.NewFixtureProvider(), Allowlist: returnto.FromOrigins(origins)})
	learning := usecase.NewLearningService(usecase.LearningOptions{Repository: learningRepo, IdentityRepository: identityRepo, Runner: runner.NewFixture(), AllowSimulatedVerification: true})
	h := &authHarness{repo: identityRepo, handler: NewRouter(Dependencies{
		Config: config.Config{Environment: "local", ElearningEnabled: true, RequestTimeout: 5 * time.Second, AllowedOrigins: origins, CookieSecure: true, AuthRateLimit: 120, UserRateLimit: 2000},
		Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Identity: identity, Repository: identityRepo, Learning: learning,
	})}
	cookies := h.login(t, "mahasiswa", "elearning.algohub.web.id")
	request := func(method, path, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", origins[0])
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if cookie.Name == usecase.CSRFCookieName {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		w := httptest.NewRecorder()
		h.handler.ServeHTTP(w, r)
		return w
	}
	body := `{"revision":0,"draft_code":"not valid C","language":"c","quiz_option":0,"complete":true}`
	rejected := request("PUT", "/v1/learning/lessons/lesson-1/progress", "no-proof", body)
	requireStatus(t, rejected, 409)
	if !strings.Contains(rejected.Body.String(), `"code":"CODE_NOT_VERIFIED"`) {
		t.Fatal(rejected.Body.String())
	}
	view := responseData[entity.LearningLessonView](t, request("GET", "/v1/learning/lessons/lesson-1", "", ""))
	if view.Profile.XP != 0 || view.Progress.Revision != 0 {
		t.Fatalf("rejection mutated state: %+v", view)
	}
	forged := strings.Replace(body, `"complete":true`, `"complete":true,"test_passed":true`, 1)
	requireStatus(t, request("PUT", "/v1/learning/lessons/lesson-1/progress", "legacy-field", forged), 422)
	accepted := strings.Replace(body, `"complete":true`, fmt.Sprintf(`"complete":true,"browser_test_passed":true,"lesson_version":%d`, view.Lesson.Version), 1)
	completed := request("PUT", "/v1/learning/lessons/lesson-1/progress", "complete", accepted)
	requireStatus(t, completed, 200)
	completedView := responseData[entity.LearningLessonView](t, completed)
	if completedView.Profile.XP != 60 || !completedView.Progress.TestPassed {
		t.Fatal(completed.Body.String())
	}
	replay := request("PUT", "/v1/learning/lessons/lesson-1/progress", "complete", accepted)
	requireStatus(t, replay, 200)
	if responseData[entity.LearningLessonView](t, replay).Profile.XP != 60 {
		t.Fatal(replay.Body.String())
	}
}

func TestUnknownOriginIsRejected(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://unknown.example")
	testRouter().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}
