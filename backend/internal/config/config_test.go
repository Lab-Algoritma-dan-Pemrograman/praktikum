package config

import (
	"testing"
	"time"
)

func localFixtureConfig() Config {
	return Config{Addr: "127.0.0.1:8080", Environment: "local", RequestTimeout: 10 * time.Second, AllowedOrigins: []string{"http://auth.localhost:5174", "http://admin.localhost:5175"}, SSOAllowedOrigins: []string{"http://admin.localhost:5175"}, SessionTTL: 12 * time.Hour, DBMaxConns: 4, EnableFixtures: true, AuthRateLimit: 120, UserRateLimit: 600}
}

func TestFixturesRequireExplicitIsolatedLocalConfiguration(t *testing.T) {
	t.Setenv("VERCEL", "")
	base := localFixtureConfig()
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.Environment = "production" },
		func(c *Config) { c.Environment = "staging" },
		func(c *Config) { c.Addr = "0.0.0.0:8080" },
		func(c *Config) { c.DatabaseURL = "postgres://example" },
		func(c *Config) { c.SupabaseURL = "https://example.supabase.co" },
		func(c *Config) { c.EnableFixtures = false },
		func(c *Config) { c.AllowedOrigins = []string{"http://untrusted.example"} },
		func(c *Config) { c.SSOAllowedOrigins = []string{"http://other.localhost:5175"} },
		func(c *Config) { c.SessionTTL = 48 * time.Hour },
		func(c *Config) { c.DBMaxConns = 1000 },
		func(c *Config) { c.UserRateLimit = 0 },
	} {
		cfg := base
		change(&cfg)
		if err := cfg.Validate(); err == nil {
			t.Fatalf("unsafe config accepted: environment=%s", cfg.Environment)
		}
	}
	t.Setenv("VERCEL", "1")
	if err := base.Validate(); err == nil {
		t.Fatal("fixture provider allowed on deployed serverless runtime")
	}
}

func TestProductionConfigurationRequiresTLSAndSecureCookies(t *testing.T) {
	cfg := Config{Addr: ":8080", Environment: "production", RequestTimeout: 10 * time.Second, AllowedOrigins: []string{"https://auth.algohub.web.id"}, SSOAllowedOrigins: []string{"https://auth.algohub.web.id"}, SessionTTL: 12 * time.Hour, DBMaxConns: 4, CookieSecure: true, AuthRateLimit: 120, UserRateLimit: 600, SupabaseURL: "https://example.supabase.co", SupabaseAnonKey: "test-public-key", DatabaseURL: "postgres://placeholder"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.CookieSecure = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("insecure production cookies accepted")
	}
	cfg.CookieSecure = true
	cfg.SupabaseURL = "http://example.supabase.co"
	if err := cfg.Validate(); err == nil {
		t.Fatal("plaintext remote identity provider accepted")
	}
}
