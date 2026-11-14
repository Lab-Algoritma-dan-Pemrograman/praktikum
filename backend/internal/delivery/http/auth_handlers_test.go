package httpdelivery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

const authHost = "auth.algohub.web.id"
const adminHost = "admin.algohub.web.id"
const verifier = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFG"

type authHarness struct {
	handler http.Handler
	repo    *repository.MemoryIdentityRepository
	now     time.Time
}

func newAuthHarness(t *testing.T, rate int) *authHarness {
	t.Helper()
	h := &authHarness{repo: repository.NewMemoryIdentityRepository(), now: time.Now().UTC()}
	origins := []string{"https://" + authHost, "https://" + adminHost, "https://algohub.web.id", "https://siakad.algohub.web.id", "https://elearning.algohub.web.id", "https://praktikum.algohub.web.id"}
	identity := usecase.NewIdentityService(usecase.IdentityOptions{Repository: h.repo, Provider: auth.NewFixtureProvider(), Allowlist: returnto.FromOrigins(origins), Now: func() time.Time { return h.now }})
	h.handler = NewRouter(Dependencies{Config: config.Config{Environment: "local", RequestTimeout: 5 * time.Second, AllowedOrigins: origins, CookieSecure: true, AuthRateLimit: rate, UserRateLimit: 2000}, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Readiness: usecase.NewReadinessService(repository.NoopReadinessRepository{}), Identity: identity, Repository: h.repo})
	return h
}

func (h *authHarness) request(method, path, payload, host string, cookies []*http.Cookie, csrf bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(payload))
	r.Host = "api.algohub.web.id"
	r.Header.Set("Origin", "https://"+host)
	r.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		r.AddCookie(cookie)
		if csrf && cookie.Name == usecase.CSRFCookieName {
			r.Header.Set("X-CSRF-Token", cookie.Value)
		}
	}
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("want %d, got %d: %s", status, w.Code, w.Body.String())
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}

func responseData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func (h *authHarness) login(t *testing.T, user, host string) []*http.Cookie {
	t.Helper()
	w := h.request("POST", "/v1/auth/password", `{"email":"`+user+`@example.test","password":"fixture-password"}`, host, nil, false)
	requireStatus(t, w, 200)
	return w.Result().Cookies()
}

func (h *authHarness) prepare(t *testing.T, cookies []*http.Cookie, host string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	w := h.request("POST", "/v1/auth/sso/prepare", fmt.Sprintf(`{"return_to":"https://%s/","code_challenge":"%s"}`, host, challenge), authHost, cookies, true)
	requireStatus(t, w, 200)
	return responseData[struct {
		Code string `json:"code"`
	}](t, w).Code
}

func exchangeBody(code, host, value string) string {
	return fmt.Sprintf(`{"code":"%s","return_to":"https://%s/","code_verifier":"%s"}`, code, host, value)
}

func TestPasswordLoginSetsHostOnlySecureSessionContract(t *testing.T) {
	h := newAuthHarness(t, 120)
	w := h.request("POST", "/v1/auth/password", `{"email":"koordinator@example.test","password":"fixture-password"}`, authHost, nil, false)
	requireStatus(t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("want two cookies, got %d", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.Domain != "" || cookie.Path != "/" || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 {
			t.Fatalf("unsafe cookie flags for %s", cookie.Name)
		}
		if cookie.HttpOnly != (cookie.Name == usecase.SessionCookieName) {
			t.Fatalf("wrong HttpOnly on %s", cookie.Name)
		}
		if strings.Contains(w.Body.String(), cookie.Value) {
			t.Fatal("raw cookie leaked in response")
		}
	}
	if strings.Contains(w.Body.String(), "fixture-password") || strings.Contains(w.Body.String(), "refresh_token") {
		t.Fatal("credential leaked")
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private response is cacheable")
	}
}

func TestPasswordLoginRejectsWrongPassword(t *testing.T) {
	h := newAuthHarness(t, 120)
	requireStatus(t, h.request("POST", "/v1/auth/password", `{"email":"koordinator@example.test","password":"wrong"}`, authHost, nil, false), 401)
}

func TestSSOAllFiveApplicationHostsAndGlobalLogout(t *testing.T) {
	h := newAuthHarness(t, 120)
	central := h.login(t, "koordinator", authHost)
	hosts := []string{"algohub.web.id", adminHost, "siakad.algohub.web.id", "elearning.algohub.web.id", "praktikum.algohub.web.id"}
	sessions := map[string][]*http.Cookie{}
	for _, host := range hosts {
		code := h.prepare(t, central, host)
		w := h.request("POST", "/v1/auth/sso/exchange", exchangeBody(code, host, verifier), host, nil, false)
		requireStatus(t, w, 200)
		sessions[host] = w.Result().Cookies()
		me := h.request("GET", "/v1/auth/me", "", host, sessions[host], false)
		requireStatus(t, me, 200)
		if responseData[entity.Principal](t, me).User.ID != 9 {
			t.Fatal("SSO changed user")
		}
	}
	requireStatus(t, h.request("GET", "/v1/auth/me", "", "algohub.web.id", sessions[adminHost], false), 401)
	requireStatus(t, h.request("POST", "/v1/auth/logout", "", adminHost, sessions[adminHost], true), 200)
	for _, host := range hosts {
		requireStatus(t, h.request("GET", "/v1/auth/me", "", host, sessions[host], false), 401)
	}
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, central, false), 401)
}

func TestSSOHandshakeIsSingleUseAndReturnToIsAllowlisted(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "koordinator", authHost)
	code := h.prepare(t, cookies, adminHost)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", exchangeBody(code, adminHost, strings.Repeat("x", 43)), adminHost, nil, false), 401)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", exchangeBody(code, adminHost, verifier), "algohub.web.id", nil, false), 401)
	payload := exchangeBody(code, adminHost, verifier)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", payload, adminHost, nil, false), 200)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", payload, adminHost, nil, false), 401)
	requireStatus(t, h.request("POST", "/v1/auth/sso/prepare", `{"return_to":"https://evil.example/","code_challenge":"x"}`, authHost, cookies, true), 422)
	requireStatus(t, h.request("POST", "/v1/auth/sso/prepare", `{}`, authHost, nil, true), 401)
	code = h.prepare(t, cookies, adminHost)
	h.now = h.now.Add(3 * time.Minute)
	requireStatus(t, h.request("POST", "/v1/auth/sso/exchange", exchangeBody(code, adminHost, verifier), adminHost, nil, false), 401)
}

func TestConcurrentHandshakeHasOneWinner(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "koordinator", authHost)
	body := exchangeBody(h.prepare(t, cookies, adminHost), adminHost, verifier)
	codes := make(chan int, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- h.request("POST", "/v1/auth/sso/exchange", body, adminHost, nil, false).Code
		}()
	}
	wg.Wait()
	close(codes)
	winners := 0
	for code := range codes {
		if code == 200 {
			winners++
		} else if code != 401 {
			t.Fatalf("unexpected status %d", code)
		}
	}
	if winners != 1 {
		t.Fatalf("want one successful exchange, got %d", winners)
	}
}

func TestRefreshRotatesTokensWithoutExtendingLifetime(t *testing.T) {
	h := newAuthHarness(t, 120)
	old := h.login(t, "koordinator", authHost)
	session, err := h.repo.FindSessionByTokenHash(t.Context(), security.Hash(old[0].Value))
	if err != nil {
		t.Fatal(err)
	}
	h.now = h.now.Add(time.Hour)
	refreshed := h.request("POST", "/v1/auth/refresh", "", authHost, old, true)
	requireStatus(t, refreshed, 200)
	current := refreshed.Result().Cookies()
	if current[0].Value == old[0].Value || current[1].Value == old[1].Value {
		t.Fatal("tokens did not rotate")
	}
	updated, err := h.repo.FindSessionByTokenHash(t.Context(), security.Hash(current[0].Value))
	if err != nil || !updated.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatal("refresh extended expiry")
	}
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, old, false), 401)
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, current, false), 200)
	h.now = h.now.Add(12 * time.Hour)
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, current, false), 401)
}

func TestAdminRequiresPermissionAndCSRFProtectsLogout(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "asisten", adminHost)
	requireStatus(t, h.request("GET", "/v1/admin/overview", "", adminHost, cookies, false), 403)
	requireStatus(t, h.request("POST", "/v1/auth/logout", "", adminHost, cookies, false), 403)
	requireStatus(t, h.request("PATCH", "/v1/me", `{"name":"Changed"}`, adminHost, cookies, false), 403)
}

func TestAdminRoleDivisionAndRouteMatrix(t *testing.T) {
	for _, tc := range []struct {
		user, path string
		status     int
	}{
		{"mahasiswa", "/", 403}, {"peminjam", "/", 403}, {"asisten", "/", 403},
		{"sekretaris", "/siakad", 200}, {"sekretaris", "/praktikum", 200},
		{"bendahara", "/siakad", 200}, {"k3", "/siakad", 200}, {"labops", "/siakad", 200},
		{"pendidikan", "/elearning", 200}, {"pendidikan", "/siakad", 403},
		{"pdd", "/", 200}, {"pdd", "/pengaturan", 403},
		{"scoped", "/praktikum", 200}, {"scoped", "/siakad", 403},
		{"koordinator", "/pengaturan", 200}, {"koordinator", "/auditlog", 200},
	} {
		t.Run(tc.user+tc.path, func(t *testing.T) {
			h := newAuthHarness(t, 120)
			cookies := h.login(t, tc.user, adminHost)
			requireStatus(t, h.request("GET", "/v1/admin/access?path="+tc.path, "", adminHost, cookies, false), tc.status)
		})
	}
}

func TestPrivateEndpointsRejectAnonymous(t *testing.T) {
	h := newAuthHarness(t, 120)
	for _, path := range []string{"/v1/auth/me", "/v1/auth/sessions", "/v1/me", "/v1/admin/access", "/v1/admin/overview", "/v1/admin/permissions", "/v1/admin/auditlog", "/v1/admin/users", "/v1/admin/users/1/access", "/v1/admin/permission-catalog"} {
		requireStatus(t, h.request("GET", path, "", adminHost, nil, false), 401)
	}
}

func TestSessionRevokeOwnershipAndIsolation(t *testing.T) {
	h := newAuthHarness(t, 120)
	student := h.login(t, "mahasiswa", authHost)
	coordinator := h.login(t, "koordinator", authHost)
	otherSessions := responseData[[]usecase.SessionView](t, h.request("GET", "/v1/auth/sessions", "", authHost, coordinator, false))
	requireStatus(t, h.request("DELETE", "/v1/auth/sessions/"+otherSessions[0].ID, "", authHost, student, true), 404)
	ownSessions := responseData[[]usecase.SessionView](t, h.request("GET", "/v1/auth/sessions", "", authHost, student, false))
	requireStatus(t, h.request("DELETE", "/v1/auth/sessions/"+ownSessions[0].ID, "", authHost, student, true), 200)
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, student, false), 401)
	requireStatus(t, h.request("GET", "/v1/auth/me", "", authHost, coordinator, false), 200)
}

func TestAccessMutationAuthorizationVersionAndLiveRevocation(t *testing.T) {
	h := newAuthHarness(t, 120)
	coordinator := h.login(t, "koordinator", adminHost)
	student := h.login(t, "mahasiswa", adminHost)
	path := "/v1/admin/users/1/access"
	access := responseData[entity.UserAccess](t, h.request("GET", path, "", adminHost, coordinator, false))
	body := fmt.Sprintf(`{"version":%d,"assignments":[{"role":"asisten","division":"pendidikan","scope":{}}],"grants":[]}`, access.Version)
	requireStatus(t, h.request("PUT", path, body, adminHost, student, true), 403)
	requireStatus(t, h.request("PUT", path, body, adminHost, coordinator, false), 403)
	updated := h.request("PUT", path, body, adminHost, coordinator, true)
	requireStatus(t, updated, 200)
	newAccess := responseData[entity.UserAccess](t, updated)
	if newAccess.Version != access.Version+1 {
		t.Fatal("version not incremented")
	}
	requireStatus(t, h.request("PUT", path, body, adminHost, coordinator, true), 409)
	requireStatus(t, h.request("GET", "/v1/admin/access?path=/elearning", "", adminHost, student, false), 200)
	remove := fmt.Sprintf(`{"version":%d,"assignments":[],"grants":[]}`, newAccess.Version)
	requireStatus(t, h.request("PUT", path, remove, adminHost, coordinator, true), 200)
	requireStatus(t, h.request("GET", "/v1/admin/access?path=/elearning", "", adminHost, student, false), 403)
	for _, bad := range []string{
		`{"version":3}`,
		`{"version":3,"assignments":null,"grants":[]}`,
		`{"version":3,"assignments":[{"role":"asisten"}],"grants":[]}`,
		`{"version":3,"assignments":[{"role":"asisten","scope":{}}],"grants":[{"permission":"audit.read","role":"asisten"}]}`,
		`{"version":3,"assignments":[{"role":"admin","scope":{}}],"grants":[]}`,
		`{"version":3,"assignments":[{"role":"mahasiswa","division":"pendidikan","scope":{}}],"grants":[]}`,
		`{"version":3,"assignments":[{"role":"asisten","scope":{"class_id":"101"}}],"grants":[{"permission":"audit.read","role":"asisten","scope":{}}]}`,
		`{"version":3,"assignments":[{"role":"asisten","user_id":9,"scope":{}}],"grants":[]}`,
	} {
		requireStatus(t, h.request("PUT", path, bad, adminHost, coordinator, true), 422)
	}
	requireStatus(t, h.request("PUT", "/v1/admin/users/9/access", remove, adminHost, coordinator, true), 422)
}

func TestAuditUsesGeneratedRequestIDAndScopedPagination(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "asisten", adminHost)
	denied := h.request("GET", "/v1/admin/overview", "", adminHost, cookies, false)
	requireStatus(t, denied, 403)
	events, err := h.repo.ListAudit(t.Context(), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Action == "authorization.denied" {
			found = true
			if event.RequestID != denied.Header().Get("X-Request-ID") || event.OperationID != "admin.overview" {
				t.Fatal("audit correlation lost")
			}
		}
	}
	if !found {
		t.Fatal("missing denied audit")
	}
	admin := h.login(t, "koordinator", adminHost)
	page := h.request("GET", "/v1/admin/auditlog?limit=1", "", adminHost, admin, false)
	requireStatus(t, page, 200)
	var envelope struct {
		Meta struct {
			Next int64 `json:"next_before"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &envelope); err != nil || envelope.Meta.Next <= 0 {
		t.Fatal("missing audit cursor")
	}
}

func TestValidationAndMethodLimits(t *testing.T) {
	h := newAuthHarness(t, 120)
	cookies := h.login(t, "koordinator", adminHost)
	for _, body := range []string{`{"name":" "}`, `{"name":"ok","role":"koordinator"}`, `{"name":"ok"} {"name":"second"}`, `{"name":"line\nbreak"}`} {
		requireStatus(t, h.request("PATCH", "/v1/me", body, adminHost, cookies, true), 422)
	}
	requireStatus(t, h.request("PATCH", "/v1/me", `{"name":"`+strings.Repeat("x", 70<<10)+`"}`, adminHost, cookies, true), 413)
	requireStatus(t, h.request("GET", "/v1/auth/password", "", adminHost, nil, false), 405)
	requireStatus(t, h.request("POST", "/v1/auth/me", "", adminHost, cookies, true), 405)
	requireStatus(t, h.request("GET", "/v1/admin/users?limit=10000", "", adminHost, cookies, false), 422)
	requireStatus(t, h.request("PATCH", "/v1/me", `{"name":"Nama Baru"}`, adminHost, cookies, true), 200)
	profile := responseData[entity.UserProfile](t, h.request("GET", "/v1/me", "", adminHost, cookies, false))
	if profile.Name != "Nama Baru" {
		t.Fatal("profile update did not persist")
	}
}

func TestRateLimitReturnsRetryAfter(t *testing.T) {
	h := newAuthHarness(t, 2)
	for i := 0; i < 2; i++ {
		requireStatus(t, h.request("POST", "/v1/auth/password", `{"email":"koordinator@example.test","password":"wrong"}`, authHost, nil, false), 401)
	}
	w := h.request("POST", "/v1/auth/password", `{"email":"koordinator@example.test","password":"wrong"}`, authHost, nil, false)
	requireStatus(t, w, 429)
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 omitted Retry-After")
	}
}
