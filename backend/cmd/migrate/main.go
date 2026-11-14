package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Println("Identity migrations are up to date")
}

func migrationDatabaseURL() string {
	if value := strings.TrimSpace(os.Getenv("ALGOHUB_MIGRATION_DATABASE_URL")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("ALGOHUB_DATABASE_URL"))
}

func run() error {
	databaseURL := migrationDatabaseURL()
	if databaseURL == "" {
		return errors.New("ALGOHUB_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid ALGOHUB_DATABASE_URL")
	}
	if err := validateMigrationTarget(os.Getenv("ALGOHUB_ENV"), config, os.Getenv("ALGOHUB_ALLOW_PRODUCTION_MIGRATIONS") == "true"); err != nil {
		return err
	}
	config.MaxConns, config.MinConns = 1, 0
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	config.MaxConnLifetime, config.MaxConnIdleTime = 5*time.Minute, 30*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}
	defer pool.Close()
	return migrations.Apply(ctx, pool)
}

func validateMigrationTarget(environment string, config *pgxpool.Config, allowProduction bool) error {
	if strings.HasSuffix(config.ConnConfig.Host, ".pooler.supabase.com") && config.ConnConfig.Port == 6543 {
		return errors.New("migrations require a direct or session-pooler connection, not Supavisor transaction pooling")
	}
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "local":
		hosts := []string{config.ConnConfig.Host}
		for _, fallback := range config.ConnConfig.Fallbacks {
			hosts = append(hosts, fallback.Host)
		}
		for _, host := range hosts {
			ip := net.ParseIP(host)
			if host != "localhost" && !strings.HasPrefix(host, "/") && (ip == nil || !ip.IsLoopback()) {
				return errors.New("local migrations require a loopback PostgreSQL host")
			}
		}
		return nil
	case "staging":
		return nil
	case "production":
		if allowProduction {
			return nil
		}
		return errors.New("production migrations require ALGOHUB_ALLOW_PRODUCTION_MIGRATIONS=true")
	default:
		return fmt.Errorf("migrations require ALGOHUB_ENV=local or staging; received %q", environment)
	}
}
