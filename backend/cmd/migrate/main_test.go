package main

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationTargetSafeguards(t *testing.T) {
	for _, test := range []struct {
		name, environment, url string
		allowProduction, ok    bool
	}{
		{"local IPv4", "local", "postgres://test@127.0.0.1/test?sslmode=disable", false, true},
		{"local IPv6", "local", "postgres://test@[::1]/test?sslmode=disable", false, true},
		{"local hostname", "local", "postgres://test@localhost/test?sslmode=disable", false, true},
		{"remote local forbidden", "local", "postgres://test@database.example.test/test", false, false},
		{"remote fallback forbidden", "local", "host=127.0.0.1,database.example.test user=test dbname=test sslmode=disable", false, false},
		{"staging", "staging", "postgres://test@staging.example.test/test", false, true},
		{"transaction pooler forbidden", "staging", "postgres://test@aws-0-ap-southeast-1.pooler.supabase.com:6543/test?sslmode=require", false, false},
		{"session pooler allowed", "staging", "postgres://test@aws-0-ap-southeast-1.pooler.supabase.com:5432/test?sslmode=require", false, true},
		{"production requires safeguard", "production", "postgres://test@database.example.test/test", false, false},
		{"production explicit safeguard", "production", "postgres://test@database.example.test/test", true, true},
		{"environment required", "", "postgres://test@localhost/test", false, false},
		{"unknown environment", "prod", "postgres://test@localhost/test", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			config, err := pgxpool.ParseConfig(test.url)
			if err != nil {
				t.Fatal(err)
			}
			err = validateMigrationTarget(test.environment, config, test.allowProduction)
			if (err == nil) != test.ok {
				t.Fatalf("target accepted=%v, wanted=%v: %v", err == nil, test.ok, err)
			}
		})
	}
}

func TestMigrationConnectionTakesPrecedence(t *testing.T) {
	t.Setenv("ALGOHUB_DATABASE_URL", "runtime-transaction-uri")
	t.Setenv("ALGOHUB_MIGRATION_DATABASE_URL", "migration-session-uri")
	if got := migrationDatabaseURL(); got != "migration-session-uri" {
		t.Fatalf("migration connection = %q", got)
	}
	t.Setenv("ALGOHUB_MIGRATION_DATABASE_URL", "")
	if got := migrationDatabaseURL(); got != "runtime-transaction-uri" {
		t.Fatalf("legacy fallback connection = %q", got)
	}
}

func TestMigrationRunnerRequiresDatabaseURL(t *testing.T) {
	t.Setenv("ALGOHUB_MIGRATION_DATABASE_URL", "")
	t.Setenv("ALGOHUB_DATABASE_URL", "")
	if err := run(); err == nil {
		t.Fatal("runner accepted missing database URL")
	}
}
