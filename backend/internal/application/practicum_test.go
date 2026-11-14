package application

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
)

func practicumApp(t *testing.T, enabled bool) App {
	t.Helper()
	t.Setenv("VERCEL", "")
	cfg := config.Config{Addr: "127.0.0.1:8080", Environment: "local", RequestTimeout: 25 * time.Second,
		AllowedOrigins:    []string{"http://auth.localhost:5174", "http://praktikum.localhost:5178", "http://admin.localhost:5175"},
		SSOAllowedOrigins: []string{"http://praktikum.localhost:5178", "http://admin.localhost:5175"},
		SessionTTL:        12 * time.Hour, DBMaxConns: 4, EnableFixtures: true, AuthRateLimit: 1000, UserRateLimit: 1000,
		PracticumEnabled: enabled, PracticumCronSecret: "local-test-cron-secret-at-least-32-characters"}
	app, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	return app
}
func appRequest(t *testing.T, app App, method, path, body string, cookies []*http.Cookie, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Origin", "http://auth.localhost:5174")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "integration-test")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
		if cookie.Name == "algohub_csrf" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	w := httptest.NewRecorder()
	app.Handler.ServeHTTP(w, req)
	return w
}
func appLogin(t *testing.T, app App, email string) []*http.Cookie {
	t.Helper()
	w := appRequest(t, app, "POST", "/v1/auth/password", `{"email":"`+email+`","password":"fixture-password"}`, nil, "")
	if w.Code != 200 {
		t.Fatalf("fixture login status %d", w.Code)
	}
	return w.Result().Cookies()
}
func appData(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d, expected %d: %s", w.Code, status, w.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
func TestPracticumReleaseSwitch(t *testing.T) {
	app := practicumApp(t, false)
	for _, path := range []string{"/v1/practicum/sessions", "/v1/internal/practicum/reconcile"} {
		if w := appRequest(t, app, "GET", path, "", nil, ""); w.Code != 404 {
			t.Fatalf("disabled endpoint %s status %d", path, w.Code)
		}
	}
}
func TestPracticumApplicationStudentLifecycle(t *testing.T) {
	app := practicumApp(t, true)
	student := appLogin(t, app, "mahasiswa@example.test")
	other := appLogin(t, app, "asisten@example.test")
	coordinator := appLogin(t, app, "koordinator@example.test")
	started := appData(t, appRequest(t, app, "POST", "/v1/practicum/attempts", `{"activation_id":"activation-1"}`, student, "start"), 200)
	id := started["attempt_id"].(string)
	if started["timezone"] != "Asia/Jakarta" || started["timezone_offset"] != "+07:00" {
		t.Fatal("business timezone missing")
	}
	if strings.Contains(string(mustJSON(t, started)), "reference_answer") {
		t.Fatal("private grading rule leaked")
	}
	path := "/v1/practicum/attempts/" + id
	denied := appRequest(t, app, "GET", path, "", other, "")
	if denied.Code != 404 && denied.Code != 403 {
		t.Fatalf("cross-user attempt read status %d", denied.Code)
	}
	answer := `{"revision":0,"answers":[{"question_id":"question-essay","value":"Synthetic answer"}]}`
	saved := appData(t, appRequest(t, app, "PATCH", path+"/answers", answer, student, "save"), 200)
	replay := appData(t, appRequest(t, app, "PATCH", path+"/answers", answer, student, "save"), 200)
	if saved["revision"] != replay["revision"] {
		t.Fatal("replay changed revision")
	}
	if w := appRequest(t, app, "PATCH", path+"/answers", answer, student, "stale-save"); w.Code != 409 {
		t.Fatalf("stale revision status %d", w.Code)
	}
	appData(t, appRequest(t, app, "POST", path+"/submit", `{}`, student, "submit"), 200)
	replay = appData(t, appRequest(t, app, "POST", "/v1/practicum/attempts", `{"activation_id":"activation-1"}`, student, "start"), 200)
	if replay["status"] != "submitted" || replay["deadline_at"] != started["deadline_at"] {
		t.Fatal("start replay reopened or extended attempt")
	}
	if w := appRequest(t, app, "PATCH", path+"/answers", `{"revision":1,"answers":[{"question_id":"question-essay","value":"Late"}]}`, student, "late"); w.Code != 409 {
		t.Fatalf("late answer status %d", w.Code)
	}
	gradePath := "/v1/admin/praktikum/penilaian/" + id
	review := appData(t, appRequest(t, app, "POST", gradePath+"/grade", `{}`, coordinator, "suggest"), 200)
	if review["attempt"].(map[string]any)["status"] == "graded" {
		t.Fatal("fixture suggestion published automatically")
	}
	manual := `{"grade_revision":0,"grades":[{"question_id":"question-essay","score":8,"feedback":"Reviewed"},{"question_id":"question-coding","score":0,"feedback":"Blank"}],"publish":true,"reason":"Local integration test"}`
	appData(t, appRequest(t, app, "PUT", gradePath+"/manual", manual, coordinator, "publish"), 200)
	final := appData(t, appRequest(t, app, "GET", path, "", student, ""), 200)
	if final["status"] != "graded" || final["score"] != float64(8) {
		t.Fatal("published grade not visible to owner")
	}
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestPracticumHTTPBoundaries(t *testing.T) {
	app := practicumApp(t, true)
	for _, path := range []string{"/v1/practicum/sessions", "/v1/practicum/runner", "/v1/admin/praktikum/sesi", "/v1/admin/praktikum/monitoring"} {
		w := appRequest(t, app, "GET", path, "", nil, "")
		if w.Code != 401 || w.Header().Get("X-Request-ID") != "integration-test" || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("private boundary %s status %d", path, w.Code)
		}
	}
	student := appLogin(t, app, "mahasiswa@example.test")
	for _, body := range []string{`{"activation_id":"activation-1","deadline_at":"2099-01-01T00:00:00Z"}`, `{"activation_id":"activation-1","role":"koordinator"}`} {
		w := appRequest(t, app, "POST", "/v1/practicum/attempts", body, student, "spoof")
		if w.Code != 422 {
			t.Fatalf("unknown field accepted %d", w.Code)
		}
	}
	w := appRequest(t, app, "POST", "/v1/practicum/attempts", `{"activation_id":"activation-1"}`, student, "")
	if w.Code != 422 {
		t.Fatalf("missing idempotency status %d", w.Code)
	}
	for _, authorization := range []string{"", "Bearer wrong", "Bearer local-test-cron-secret-at-least-32-characters"} {
		req := httptest.NewRequest("POST", "/v1/internal/practicum/reconcile", nil)
		req.Header.Set("Authorization", authorization)
		w := httptest.NewRecorder()
		app.Handler.ServeHTTP(w, req)
		want := 401
		if strings.HasSuffix(authorization, "32-characters") {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("cron boundary status %d expected %d", w.Code, want)
		}
	}
}
