package config

import (
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                    string
	Environment             string
	RequestTimeout          time.Duration
	AllowedOrigins          []string
	SSOAllowedOrigins       []string
	OpenAPIPath             string
	CookieSecure            bool
	SessionTTL              time.Duration
	SupabaseURL             string
	SupabaseAnonKey         string
	DatabaseURL             string
	DBMaxConns              int32
	EnableFixtures          bool
	AuthRateLimit           int
	UserRateLimit           int
	PracticumEnabled        bool
	PracticumCronSecret     string
	ElearningEnabled        bool
	SiakadEnabled           bool
	PlaygroundFallbackURL   string
	PlaygroundFallbackToken string
	GradingBaseURL          string
	GradingModel            string
	GradingAPIKey           string
	SupabaseServiceKey      string
	LaporanBucket           string
}

func Load() Config {
	environment := envOr("ALGOHUB_ENV", "local")
	origins := defaultOrigins()
	addr := ":8080"
	if environment == "local" {
		addr = "127.0.0.1:8080"
		origins = append(origins, "http://localhost:5173", "http://localhost:5174")
	}
	return Config{
		Addr:                envOr("ALGOHUB_ADDR", addr),
		Environment:         environment,
		RequestTimeout:      envDuration("ALGOHUB_REQUEST_TIMEOUT", 25*time.Second),
		AllowedOrigins:      envCSV("ALGOHUB_ALLOWED_ORIGINS", origins),
		SSOAllowedOrigins:   envCSV("ALGOHUB_SSO_ALLOWED_ORIGINS", origins),
		OpenAPIPath:         envOr("ALGOHUB_OPENAPI_PATH", "docs/openapi/openapi.yaml"),
		CookieSecure:        envBool("ALGOHUB_COOKIE_SECURE", environment != "local"),
		SessionTTL:          envDuration("ALGOHUB_SESSION_TTL", 12*time.Hour),
		SupabaseURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/"),
		SupabaseAnonKey:     strings.TrimSpace(os.Getenv("SUPABASE_ANON_KEY")),
		DatabaseURL:         strings.TrimSpace(os.Getenv("ALGOHUB_DATABASE_URL")),
		DBMaxConns:          int32(envInt("ALGOHUB_DB_MAX_CONNS", 4)),
		EnableFixtures:      envBool("ALGOHUB_ENABLE_FIXTURES", false),
		AuthRateLimit:       envInt("ALGOHUB_AUTH_RATE_LIMIT", 120),
		UserRateLimit:       envInt("ALGOHUB_USER_RATE_LIMIT", 600),
		PracticumEnabled:    envBool("ALGOHUB_PRACTICUM_ENABLED", false),
		PracticumCronSecret: strings.TrimSpace(os.Getenv("ALGOHUB_PRACTICUM_CRON_SECRET")),
		ElearningEnabled:    envBool("ALGOHUB_ELEARNING_ENABLED", false),
		SiakadEnabled:       envBool("ALGOHUB_SIAKAD_ENABLED", false),
		PlaygroundFallbackURL: strings.TrimSpace(envOr(
			"ALGOHUB_PLAYGROUND_GLOT_URL", "https://glot.io/api/mux",
		)),
		PlaygroundFallbackToken: strings.TrimSpace(os.Getenv("ALGOHUB_PLAYGROUND_GLOT_TOKEN")),
		GradingBaseURL:          strings.TrimRight(strings.TrimSpace(os.Getenv("ALGOHUB_GRADING_BASE_URL")), "/"),
		GradingModel:            strings.TrimSpace(os.Getenv("ALGOHUB_GRADING_MODEL")),
		GradingAPIKey:           strings.TrimSpace(os.Getenv("ALGOHUB_GRADING_API_KEY")),
		SupabaseServiceKey:      strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_KEY")),
		LaporanBucket:           envOr("ALGOHUB_LAPORAN_BUCKET", "laporan-besar"),
	}
}

func (c Config) Validate() error {
	if c.Environment != "local" && c.Environment != "staging" && c.Environment != "production" {
		return errors.New("ALGOHUB_ENV must be local, staging, or production")
	}
	if c.RequestTimeout <= 0 || c.RequestTimeout > 30*time.Second || c.SessionTTL < time.Minute || c.SessionTTL > 24*time.Hour {
		return errors.New("request timeout or session lifetime is outside supported limits")
	}
	if c.DBMaxConns < 1 || c.DBMaxConns > 20 || c.AuthRateLimit < 1 || c.UserRateLimit < 1 {
		return errors.New("connection and rate limits must be bounded positive values")
	}
	if c.PracticumCronSecret != "" && (len(c.PracticumCronSecret) < 32 || len(c.PracticumCronSecret) > 256 || strings.ContainsAny(c.PracticumCronSecret, " \t\r\n")) {
		return errors.New("Practicum cron secret must contain 32..256 non-whitespace characters")
	}
	if c.Environment != "local" && !c.CookieSecure {
		return errors.New("secure cookies are required outside local development")
	}
	for _, origins := range [][]string{c.AllowedOrigins, c.SSOAllowedOrigins} {
		if len(origins) == 0 {
			return errors.New("an explicit origin allowlist is required")
		}
		for _, origin := range origins {
			u, err := url.Parse(origin)
			if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
				return errors.New("origin allowlist contains an invalid origin")
			}
			if u.Scheme == "http" && (c.Environment != "local" || !loopbackHost(u.Hostname())) {
				return errors.New("HTTP origins are restricted to local loopback development")
			}
		}
	}
	for _, origin := range c.SSOAllowedOrigins {
		if !c.IsAllowedOrigin(origin) {
			return errors.New("every SSO origin must also be in the API origin allowlist")
		}
	}
	if c.PlaygroundFallbackToken != "" {
		if len(c.PlaygroundFallbackToken) < 8 || len(c.PlaygroundFallbackToken) > 512 ||
			strings.ContainsFunc(c.PlaygroundFallbackToken, func(ch rune) bool { return ch <= ' ' || ch == 0x7f }) {
			return errors.New("playground fallback token must be 8..512 printable characters")
		}
		endpoint, err := url.Parse(c.PlaygroundFallbackURL)
		if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" ||
			(endpoint.Scheme != "https" && !(c.Environment == "local" && endpoint.Scheme == "http" && loopbackHost(endpoint.Hostname()))) {
			return errors.New("playground fallback URL must use HTTPS or a local loopback fixture")
		}
	}
	if c.EnableFixtures {
		host, _, err := net.SplitHostPort(c.Addr)
		if c.Environment != "local" || os.Getenv("VERCEL") != "" || c.DatabaseURL != "" || c.SupabaseURL != "" || c.SupabaseAnonKey != "" || err != nil || !loopbackHost(host) {
			return errors.New("fixtures require an isolated loopback-only local API without Supabase or a database")
		}
		return nil
	}
	if c.GradingBaseURL != "" {
		endpoint, err := url.Parse(c.GradingBaseURL)
		if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" ||
			(endpoint.Scheme != "https" && !(c.Environment == "local" && endpoint.Scheme == "http" && loopbackHost(endpoint.Hostname()))) {
			return errors.New("grading base URL must use HTTPS or a local loopback fixture")
		}
		if c.GradingModel == "" {
			return errors.New("grading base URL requires ALGOHUB_GRADING_MODEL")
		}
		if c.GradingAPIKey != "" && (len(c.GradingAPIKey) > 512 || strings.ContainsFunc(c.GradingAPIKey, func(ch rune) bool { return ch <= ' ' || ch == 0x7f })) {
			return errors.New("grading API key must be printable characters without whitespace")
		}
	} else if c.GradingModel != "" || c.GradingAPIKey != "" {
		return errors.New("grading model or API key set without ALGOHUB_GRADING_BASE_URL")
	}
	if c.DatabaseURL == "" || c.SupabaseURL == "" || c.SupabaseAnonKey == "" {
		return errors.New("database and Supabase credentials are required; local fixtures must be explicitly enabled")
	}
	provider, err := url.Parse(c.SupabaseURL)
	if err != nil || provider.Host == "" || provider.User != nil || provider.RawQuery != "" || provider.Fragment != "" || (provider.Scheme != "https" && !(c.Environment == "local" && provider.Scheme == "http" && loopbackHost(provider.Hostname()))) {
		return errors.New("Supabase URL must use HTTPS or a local loopback fixture")
	}
	return nil
}

func loopbackHost(host string) bool {
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}

func (c Config) IsAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	for _, allowed := range c.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return value
}
func envDuration(key string, fallback time.Duration) time.Duration {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return fallback
}
func envInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
func envCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	result := []string{}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
func defaultOrigins() []string {
	return []string{"https://algohub.web.id", "https://admin.algohub.web.id", "https://auth.algohub.web.id", "https://siakad.algohub.web.id", "https://elearning.algohub.web.id", "https://praktikum.algohub.web.id"}
}
