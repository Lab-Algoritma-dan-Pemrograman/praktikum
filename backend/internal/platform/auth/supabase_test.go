package authplatform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupabasePasswordAndPKCERequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/token" || r.Header.Get("apikey") != "test-public-key" || r.Method != "POST" {
			t.Error("provider request contract changed")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch r.URL.Query().Get("grant_type") {
		case "password":
			if body["email"] != "user@example.test" || body["password"] != "test-password" {
				t.Error("incorrect password mapping")
			}
		case "pkce":
			if body["auth_code"] != "test-code" || body["code_verifier"] != "test-verifier" {
				t.Error("incorrect PKCE mapping")
			}
		default:
			t.Error("unexpected provider grant")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"test-id","email":"User@Example.Test","user_metadata":{"name":"Example User","role":"koordinator"}},"access_token":"discard-me","refresh_token":"discard-me"}`))
	}))
	defer server.Close()
	provider := NewSupabaseProvider(server.URL, "test-public-key", nil)
	for _, run := range []func() (Identity, error){
		func() (Identity, error) {
			return provider.PasswordLogin(t.Context(), "user@example.test", "test-password")
		},
		func() (Identity, error) { return provider.ExchangeCode(t.Context(), "test-code", "test-verifier") },
	} {
		identity, err := run()
		if err != nil || identity.Email != "user@example.test" || identity.Name != "Example User" || identity.AuthUserID != "test-id" {
			t.Fatalf("identity mapping failed: %v", err)
		}
		encoded, err := json.Marshal(identity)
		if err != nil || strings.Contains(string(encoded), "discard-me") || strings.Contains(string(encoded), "koordinator") {
			t.Fatal("provider credentials or role metadata escaped identity boundary")
		}
	}
}

func TestProviderBadCredentialsDoNotOpenCircuit(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	provider := NewSupabaseProvider(server.URL, "test-key", nil)
	for i := 0; i < 5; i++ {
		if _, err := provider.PasswordLogin(t.Context(), "user@example.test", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("bad credential mapping: %v", err)
		}
	}
	if requests.Load() != 5 {
		t.Fatal("user errors opened circuit breaker")
	}
	if _, err := provider.ExchangeCode(t.Context(), "bad", "verifier"); !errors.Is(err, ErrInvalidAuthCode) {
		t.Fatal("invalid code mapping")
	}
}

func TestProviderOutageIsBoundedAndNeverRetried(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	provider := NewSupabaseProvider(server.URL, "test-key", nil)
	for i := 0; i < 5; i++ {
		if _, err := provider.PasswordLogin(t.Context(), "user@example.test", "password"); !errors.Is(err, ErrProviderUnavailable) {
			t.Fatalf("outage mapping: %v", err)
		}
	}
	if requests.Load() != 3 {
		t.Fatalf("expected breaker after3 calls without retries, got %d", requests.Load())
	}
}

func TestProviderRejectsRedirectAndOversizedResult(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(map[bool]string{false: "redirect", true: "oversize"}[oversized], func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if oversized {
					_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+10)))
					return
				}
				w.Header().Set("Location", "/unexpected")
				w.WriteHeader(http.StatusFound)
			}))
			defer server.Close()
			_, err := NewSupabaseProvider(server.URL, "test-key", nil).PasswordLogin(t.Context(), "user@example.test", "password")
			if !errors.Is(err, ErrProviderUnavailable) || requests.Load() != 1 {
				t.Fatal("redirect or oversized provider result was accepted")
			}
		})
	}
}

func TestProviderTimeoutAndCancelledContext(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	provider := NewSupabaseProvider(server.URL, "test-key", &http.Client{Timeout: 30 * time.Millisecond})
	started := time.Now()
	if _, err := provider.PasswordLogin(t.Context(), "user@example.test", "password"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatal("timeout not mapped")
	}
	if time.Since(started) > time.Second {
		t.Fatal("provider exceeded bounded timeout")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := provider.PasswordLogin(ctx, "user@example.test", "password"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatal("cancelled context ignored")
	}
}

func TestFixtureAuthorizationCodesAreSingleUse(t *testing.T) {
	provider := NewFixtureProvider()
	if _, err := provider.ExchangeCode(t.Context(), "fixture-coordinator", "verifier"); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ExchangeCode(t.Context(), "fixture-coordinator", "verifier"); !errors.Is(err, ErrInvalidAuthCode) {
		t.Fatal("fixture code replay accepted")
	}
}
