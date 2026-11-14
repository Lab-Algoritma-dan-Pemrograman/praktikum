// Command legacy-migrate performs the lossless migration of the legacy
// project_ap database into the v2 schema. The source database is only ever
// read inside a repeatable-read, read-only transaction.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type config struct {
	sourceURL string
	targetURL string
	batchID   string
}

func main() {
	log.SetFlags(log.Ltime)
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("legacy-migrate: %v", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: legacy-migrate <extract|apply|verify>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	switch command := args[0]; command {
	case "extract":
		return runExtract(ctx, cfg)
	case "apply":
		return runApply(ctx, cfg)
	case "verify":
		return runVerify(ctx, cfg)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func loadConfig() (config, error) {
	cfg := config{
		sourceURL: strings.TrimSpace(os.Getenv("LEGACY_DATABASE_URL")),
		targetURL: strings.TrimSpace(os.Getenv("ALGOHUB_MIGRATION_DATABASE_URL")),
		batchID:   strings.TrimSpace(os.Getenv("LEGACY_BATCH_ID")),
	}
	if cfg.sourceURL == "" {
		return cfg, errors.New("LEGACY_DATABASE_URL is required")
	}
	if cfg.targetURL == "" {
		return cfg, errors.New("ALGOHUB_MIGRATION_DATABASE_URL is required")
	}
	if cfg.batchID == "" {
		cfg.batchID = "legacy-" + time.Now().UTC().Format("20060102T150405Z")
	}
	return cfg, nil
}

func openPool(ctx context.Context, url string, readOnly bool) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid database url")
	}
	poolCfg.MaxConns = 2
	if readOnly {
		poolCfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	}
	poolCfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	// A migration run issues each statement once; skipping the statement cache
	// avoids prepared-statement name collisions across pooled connections.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	return pgxpool.NewWithConfig(ctx, poolCfg)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func joinLines(values []string) string {
	return strings.Join(values, "\n")
}
