package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/application"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func TestPostgresApplicationIdentityAcrossInstances(t *testing.T) {
	ctx, pool := repository.OpenTestDatabase(t)
	var authID string
	if err := pool.QueryRow(ctx, "insert into auth.users(id) values(gen_random_uuid()) returning id::text").Scan(&authID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanup, "delete from public.audit_logs where actor_id in (select id from public.users where auth_user_id=$1)", authID)
		_, _ = pool.Exec(cleanup, "delete from auth.users where id=$1", authID)
	})
	email := authID + "@example.test"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Email, Password string }
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Email != email || input.Password != "test-password" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": authID, "email": email, "user_metadata": map[string]string{"name": "Database HTTP Fixture", "role": "koordinator"}}, "refresh_token": "must-not-persist"})
	}))
	defer provider.Close()
	origins := []string{"https://auth.algohub.web.id", "https://admin.algohub.web.id"}
	cfg := config.Config{Addr: "127.0.0.1:8080", Environment: "local", RequestTimeout: 10 * time.Second, AllowedOrigins: origins, SSOAllowedOrigins: origins, CookieSecure: true, SessionTTL: time.Hour, SupabaseURL: provider.URL, SupabaseAnonKey: "test-key", DatabaseURL: os.Getenv("ALGOHUB_TEST_DATABASE_URL"), DBMaxConns: 2, AuthRateLimit: 200, UserRateLimit: 1000}
	first, err := application.New(ctx, cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := application.New(ctx, cfg, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	call := func(handler http.Handler, method, path, body, origin string, cookies []*http.Cookie, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "api.algohub.web.id"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if cookie.Name == "algohub_csrf" {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: expected %d, got %d: %s", method, path, status, w.Code, w.Body.String())
		}
		return w
	}
	call(first.Handler, "GET", "/readyz", "", origins[0], nil, 200)
	login := call(first.Handler, "POST", "/v1/auth/password", fmt.Sprintf(`{"email":"%s","password":"test-password"}`, email), origins[0], nil, 200)
	central := login.Result().Cookies()
	call(second.Handler, "GET", "/v1/auth/me", "", origins[0], central, 200)
	call(second.Handler, "GET", "/v1/admin/access", "", origins[0], central, 403)
	if _, err := pool.Exec(ctx, "insert into public.user_role_assignments(user_id,role_code,scope) select id,'koordinator','{}'::jsonb from public.users where auth_user_id=$1", authID); err != nil {
		t.Fatal(err)
	}
	call(second.Handler, "GET", "/v1/admin/access", "", origins[0], central, 200)
	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	prepare := call(first.Handler, "POST", "/v1/auth/sso/prepare", fmt.Sprintf(`{"return_to":"%s/","code_challenge":"%s"}`, origins[1], base64.RawURLEncoding.EncodeToString(sum[:])), origins[0], central, 200)
	var handshake struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(prepare.Body.Bytes(), &handshake); err != nil {
		t.Fatal(err)
	}
	exchangeBody := fmt.Sprintf(`{"code":"%s","return_to":"%s/","code_verifier":"%s"}`, handshake.Data.Code, origins[1], verifier)
	child := call(second.Handler, "POST", "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, 200).Result().Cookies()
	call(first.Handler, "POST", "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, 401)
	call(second.Handler, "PATCH", "/v1/me", `{"name":"Persisted Fixture"}`, origins[1], child, 200)
	call(first.Handler, "GET", "/v1/admin/users", "", origins[0], central, 200)
	call(first.Handler, "GET", "/v1/admin/auditlog", "", origins[0], central, 200)
	var name string
	var retained bool
	if err := pool.QueryRow(ctx, "select name from public.users where auth_user_id=$1", authID).Scan(&name); err != nil || name != "Persisted Fixture" {
		t.Fatal("HTTP profile update was not persisted")
	}
	if err := pool.QueryRow(ctx, "select exists(select 1 from public.auth_sessions s join public.users u on u.id=s.user_id where u.auth_user_id=$1 and s.provider_refresh_token_seal is not null)", authID).Scan(&retained); err != nil || retained {
		t.Fatal("provider credential retained")
	}
	call(second.Handler, "POST", "/v1/auth/logout", "", origins[1], child, 200)
	call(first.Handler, "GET", "/v1/auth/me", "", origins[0], central, 401)
}
