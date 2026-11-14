package authplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SupabaseProvider struct {
	baseURL   string
	apiKey    string
	client    *http.Client
	mu        sync.Mutex
	failures  int
	openUntil time.Time
	probe     bool
}

func NewSupabaseProvider(baseURL, anonKey string, client *http.Client) IdentityProvider {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(anonKey) == "" {
		return UnconfiguredProvider{}
	}
	bounded := http.Client{Timeout: 8 * time.Second}
	if client != nil {
		bounded = *client
	}
	if bounded.Timeout <= 0 || bounded.Timeout > 8*time.Second {
		bounded.Timeout = 8 * time.Second
	}
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &SupabaseProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: anonKey, client: &bounded}
}

type supabaseTokenResponse struct {
	User struct {
		ID           string         `json:"id"`
		Email        string         `json:"email"`
		UserMetadata map[string]any `json:"user_metadata"`
	} `json:"user"`
}

func (p *SupabaseProvider) ExchangeCode(ctx context.Context, code, verifier string) (Identity, error) {
	return p.exchange(ctx, "pkce", map[string]string{"auth_code": code, "code_verifier": verifier})
}
func (p *SupabaseProvider) PasswordLogin(ctx context.Context, email, password string) (Identity, error) {
	return p.exchange(ctx, "password", map[string]string{"email": email, "password": password})
}

func (p *SupabaseProvider) permit() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.openUntil.IsZero() {
		return true
	}
	if time.Now().Before(p.openUntil) || p.probe {
		return false
	}
	p.probe = true
	return true
}

func (p *SupabaseProvider) record(healthy bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if healthy {
		p.failures, p.openUntil, p.probe = 0, time.Time{}, false
		return
	}
	p.failures++
	if p.failures >= 3 || p.probe {
		p.openUntil = time.Now().Add(15 * time.Second)
	}
	p.probe = false
}

func (p *SupabaseProvider) exchange(ctx context.Context, grantType string, payload map[string]string) (Identity, error) {
	if !p.permit() {
		return Identity{}, ErrProviderUnavailable
	}
	healthy := false
	defer func() { p.record(healthy) }()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return Identity{}, ErrProviderUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/auth/v1/token?grant_type="+grantType, bytes.NewReader(body))
	if err != nil {
		return Identity{}, ErrProviderUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("apikey", p.apiKey)
	request.Header.Set("Authorization", "Bearer "+p.apiKey)
	response, err := p.client.Do(request)
	if err != nil {
		return Identity{}, ErrProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized {
		healthy = true
		if grantType == "pkce" {
			return Identity{}, ErrInvalidAuthCode
		}
		return Identity{}, ErrInvalidCredentials
	}
	if response.StatusCode != http.StatusOK {
		return Identity{}, ErrProviderUnavailable
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(responseBody) > 1<<20 {
		return Identity{}, ErrProviderUnavailable
	}
	var token supabaseTokenResponse
	if err := json.Unmarshal(responseBody, &token); err != nil || token.User.ID == "" || token.User.Email == "" || len(token.User.ID) > 128 || len(token.User.Email) > 320 {
		return Identity{}, ErrProviderUnavailable
	}
	name := ""
	for _, key := range []string{"full_name", "name", "display_name"} {
		if value, ok := token.User.UserMetadata[key].(string); ok && strings.TrimSpace(value) != "" {
			name = strings.TrimSpace(value)
			break
		}
	}
	healthy = true
	return Identity{AuthUserID: token.User.ID, Email: strings.ToLower(token.User.Email), Name: name}, nil
}
