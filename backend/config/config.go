package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi aplikasi yang dimuat dari environment.
type Config struct {
	AppPort string
	AppEnv  string

	CORSOrigins []string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBParams   string

	JWTSecret      string
	JWTExpireHours int

	// AuthMode: legacy (HS256 lokal saja), dual (Supabase RS256 atau HS256), supabase (hanya RS256)
	AuthMode          string
	SupabaseJWKSURL   string
	SupabaseJWTIssuer string
	SupabaseJWTAud    string
	EmailAllowItpln   bool

	// CronSecret: rahasia header pemicu endpoint cron (auto-submit) dari cron-job.org.
	CronSecret string

	OnlineTTL       time.Duration
	SweeperInterval time.Duration

	SupabaseURL        string
	SupabaseServiceKey string
	SupabaseBucket     string

	OllamaURL    string
	OllamaModel  string
	OllamaAPIKey string

	// Glot.io (Run Code)
	GlotURL   string
	GlotToken string

	// Compile C -> wasm32-wasi (clang + wasi-sdk). Kosong = fitur nonaktif.
	CwasmClang   string
	CwasmSysroot string

	// Parameter hash Firebase Auth (untuk verifikasi password lama saat migrasi).
	// Kosong = fitur nonaktif (login fallback ke bcrypt saja).
	FbScryptSignerKey     string
	FbScryptSaltSeparator string
	FbScryptRounds        int
	FbScryptMemCost       int
}

// Load membaca .env (jika ada) lalu environment OS.
func Load() *Config {
	// Coba muat .env dari beberapa lokasi umum (root backend / root project).
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	cfg := &Config{
		AppPort:               getEnv("PORT", getEnv("APP_PORT", "8080")),
		AppEnv:                getEnv("APP_ENV", "development"),
		CORSOrigins:           splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173")),
		DBHost:                getEnv("DB_HOST", "127.0.0.1"),
		DBPort:                getEnv("DB_PORT", "5432"),
		DBUser:                getEnv("DB_USER", "postgres"),
		DBPassword:            getEnv("DB_PASSWORD", ""),
		DBName:                getEnv("DB_NAME", "lab_ap"),
		DBParams:              getEnv("DB_PARAMS", "sslmode=disable TimeZone=Asia/Jakarta"),
		JWTSecret:             getEnv("JWT_SECRET", "dev-secret-change-me"),
		JWTExpireHours:        getEnvInt("JWT_EXPIRE_HOURS", 12),
		AuthMode:              getEnv("AUTH_MODE", "legacy"),
		SupabaseJWKSURL:       getEnv("SUPABASE_JWKS_URL", ""),
		SupabaseJWTIssuer:     getEnv("SUPABASE_JWT_ISSUER", ""),
		SupabaseJWTAud:        getEnv("SUPABASE_JWT_AUD", "authenticated"),
		EmailAllowItpln:       getEnv("ALLOW_ITPLN_DOMAIN", "false") == "true",
		CronSecret:            getEnv("CRON_SECRET", ""),
		OnlineTTL:             time.Duration(getEnvInt("ONLINE_TTL_SECONDS", 120)) * time.Second,
		SweeperInterval:       time.Duration(getEnvInt("SWEEPER_INTERVAL_SECONDS", 30)) * time.Second,
		SupabaseURL:           getEnv("SUPABASE_URL", ""),
		SupabaseServiceKey:    getEnv("SUPABASE_SERVICE_KEY", ""),
		SupabaseBucket:        getEnv("SUPABASE_BUCKET", "public-assets"),
		OllamaURL:             getEnv("OLLAMA_URL", "https://router.bynara.id/v1/chat/completions"),
		OllamaModel:           getEnv("OLLAMA_MODEL", "mimo-v2.5-pro-hermes"),
		OllamaAPIKey:          getEnv("OLLAMA_API_KEY", ""),
		GlotURL:               getEnv("GLOT_URL", "https://glot.io/api/run"),
		GlotToken:             getEnv("GLOT_TOKEN", ""),
		CwasmClang:            getEnv("CWASM_CLANG", ""),
		CwasmSysroot:          getEnv("WASI_SYSROOT", ""),
		FbScryptSignerKey:     getEnv("FB_SCRYPT_SIGNER_KEY", ""),
		FbScryptSaltSeparator: getEnv("FB_SCRYPT_SALT_SEPARATOR", ""),
		FbScryptRounds:        getEnvInt("FB_SCRYPT_ROUNDS", 8),
		FbScryptMemCost:       getEnvInt("FB_SCRYPT_MEM_COST", 14),
	}
	// MEDIUM (code review): TOLAK fallback dev-secret-change-me di production.
	// Satu salah-konfigurasi deploy = siapa pun bisa forge token HS256 admin.
	if cfg.AppEnv == "production" && (cfg.JWTSecret == "" || cfg.JWTSecret == "dev-secret-change-me") {
		panic("JWT_SECRET wajib di-set (non-default) saat APP_ENV=production")
	}
	return cfg
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s %s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBParams)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
