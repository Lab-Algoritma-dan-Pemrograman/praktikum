package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func OpenTestDatabase(t *testing.T) (context.Context, *pgxpool.Pool) {
	return postgresTestDatabase(t)
}
