package repository

import (
	"context"
	"errors"
)

var ErrNotConfigured = errors.New("repository is not configured")

type ReadinessRepository interface {
	Check(ctx context.Context) error
}

// NoopReadinessRepository makes the Phase 1 service honest: the API can start
// and expose liveness, while readiness stays unavailable until the database
// adapter is introduced in a later phase.
type NoopReadinessRepository struct{}

func (NoopReadinessRepository) Check(context.Context) error {
	return ErrNotConfigured
}
