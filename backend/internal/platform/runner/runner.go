// Package runner defines the optional server-execution boundary used by isolated
// fixtures and provider-neutral compatibility endpoints. Production defaults to
// Unconfigured; E-learning completion uses the explicitly accepted browser-trust
// model and Praktikum academic grading remains manual.
package runner

import (
	"context"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

const (
	MaxSourceBytes = 64 << 10
	MaxStdinBytes  = 16 << 10
	MaxOutputBytes = 16 << 10  // each of stdout and stderr
	MaxResultBytes = 128 << 10 // serialized JSON, including escaping

	StatusSuccess       = "success"
	StatusCompileError  = "compile_error"
	StatusRuntimeError  = "runtime_error"
	StatusTimeout       = "timeout"
	StatusQuotaExceeded = "quota_exceeded"
	StatusProviderError = "provider_error"
)

var (
	ErrInvalidRequest        = errors.New("invalid runner request")
	ErrUnsupportedLanguage   = errors.New("unsupported runner language or version")
	ErrInvalidResult         = errors.New("invalid runner result")
	ErrProviderNotConfigured = errors.New("runner provider is not configured")
	ErrProviderUnavailable   = errors.New("runner provider is unavailable")
)

// Runner must honor context cancellation. Calls may not retry uncertain source
// submissions automatically. The use case owns durable quotas and idempotency;
// even a successful simulated result is not evidence of real code correctness.
type Runner interface {
	Run(context.Context, entity.RunRequest) (entity.RunResult, error)
	Languages() []entity.RunnerLanguage
	Available() bool
	Simulated() bool
}

type Unconfigured struct{}

func (Unconfigured) Languages() []entity.RunnerLanguage { return []entity.RunnerLanguage{} }
func (Unconfigured) Available() bool                    { return false }
func (Unconfigured) Simulated() bool                    { return false }
func (Unconfigured) Run(ctx context.Context, req entity.RunRequest) (entity.RunResult, error) {
	result := entity.RunResult{Status: StatusProviderError, RunnerVersion: "unconfigured"}
	if err := validateInput(req); err != nil {
		return result, err
	}
	result, _ = NormalizeResult(req, result)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, ErrProviderNotConfigured
}

var _ Runner = Unconfigured{}
var _ Runner = (*Fixture)(nil)
