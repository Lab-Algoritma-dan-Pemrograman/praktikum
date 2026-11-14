package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

const advisoryLockID int64 = 0x414c474f485542

// Definition identifies one immutable embedded migration. Catalog and
// Definitions return defensive copies so callers cannot mutate the catalog.
type Definition struct {
	Name     string `json:"name"`
	Checksum string `json:"sha256"`
}

type embeddedDefinition struct {
	Definition
	body []byte
}

func embeddedCatalog() ([]embeddedDefinition, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	catalog := make([]embeddedDefinition, 0, len(names))
	for _, name := range names {
		body, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		catalog = append(catalog, embeddedDefinition{
			Definition: Definition{Name: name, Checksum: hex.EncodeToString(sum[:])},
			body:       body,
		})
	}
	return catalog, nil
}

// Catalog returns the deterministic, lexically sorted migration catalog.
func Catalog() []Definition {
	catalog, err := embeddedCatalog()
	if err != nil {
		panic(fmt.Sprintf("read embedded migration catalog: %v", err))
	}
	definitions := make([]Definition, len(catalog))
	for i := range catalog {
		definitions[i] = catalog[i].Definition
	}
	return definitions
}

// Definitions is an explicit alias for Catalog.
func Definitions() []Definition { return Catalog() }

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	catalog, err := embeddedCatalog()
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `select pg_advisory_lock($1)`, advisoryLockID); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(cleanup, `select pg_advisory_unlock($1)`, advisoryLockID); err != nil {
			_ = conn.Conn().Close(cleanup)
		}
	}()
	if _, err := conn.Exec(ctx, `
		create table if not exists public.schema_migrations (
		  name text primary key,
		  checksum text not null check (checksum ~ '^[0-9a-f]{64}$'),
		  applied_at timestamptz not null default now()
		);
		alter table public.schema_migrations enable row level security;
		revoke all on public.schema_migrations from public;
	`); err != nil {
		return fmt.Errorf("initialize migration history: %w", err)
	}
	rows, err := conn.Query(ctx, `select name, checksum from public.schema_migrations order by name`)
	if err != nil {
		return err
	}
	applied := make(map[string]string)
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			rows.Close()
			return err
		}
		applied[name] = checksum
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	known := make(map[string]bool, len(catalog))
	pending := false
	for _, migration := range catalog {
		name, checksum := migration.Name, migration.Checksum
		known[name] = true
		if _, exists := applied[name]; exists {
			if pending {
				return fmt.Errorf("migration history is out of order before %s", name)
			}
		} else {
			pending = true
		}
		if previous, ok := applied[name]; ok && previous != checksum {
			return fmt.Errorf("migration checksum drift: %s", name)
		}
	}
	for name := range applied {
		if !known[name] {
			return fmt.Errorf("applied migration is missing from embedded files: %s", name)
		}
	}
	for _, migration := range catalog {
		name, checksum, body := migration.Name, migration.Checksum, migration.body
		if _, ok := applied[name]; ok {
			continue
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err == nil {
			_, err = tx.Exec(ctx, `insert into public.schema_migrations (name, checksum) values ($1, $2)`, name, checksum)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			rollbackErr := tx.Rollback(cleanup)
			if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				_ = conn.Conn().Close(cleanup)
			}
			cancel()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}
