package runner

import (
	"context"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

const (
	FixtureVersion = "fixture-v1"
	// These are exact, harmless stdin values, not source-language syntax. All
	// other stdin produces the same synthetic success. Source is only hashed.
	FixtureCompileError = "[fixture:compile_error]"
	FixtureRuntimeError = "[fixture:runtime_error]"
	FixtureTimeout      = "[fixture:timeout]"
	FixtureOutage       = "[fixture:outage]"
)

// Fixture has no client, filesystem, shell, interpreter, or mutable state. It
// never compiles or parses submitted code; the selected scenario is synthetic.
type Fixture struct{}

func NewFixture() *Fixture       { return &Fixture{} }
func (*Fixture) Available() bool { return true }
func (*Fixture) Simulated() bool { return true }
func (*Fixture) Languages() []entity.RunnerLanguage {
	return []entity.RunnerLanguage{
		{Language: "c", Version: FixtureVersion, Label: "C (simulated fixture; no compiler)"},
		{Language: "python", Version: FixtureVersion, Label: "Python (simulated fixture; no interpreter)"},
	}
}

func (f *Fixture) Run(ctx context.Context, req entity.RunRequest) (entity.RunResult, error) {
	result := entity.RunResult{Status: StatusProviderError, RunnerVersion: FixtureVersion, Simulated: true}
	if err := ValidateRequest(req, f.Languages()); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		result, _ = NormalizeResult(req, result)
		return result, err
	}
	var providerErr error
	switch req.Stdin {
	case FixtureCompileError:
		result.Status = StatusCompileError
		result.Stderr = "SIMULATED fixture: compilation error scenario; source was not compiled.\n"
	case FixtureRuntimeError:
		exit := 1
		result.Status, result.ExitCode = StatusRuntimeError, &exit
		result.Stderr = "SIMULATED fixture: runtime error scenario; source was not executed.\n"
	case FixtureTimeout:
		result.Status = StatusTimeout
		result.Stderr = "SIMULATED fixture: timeout scenario; source was not executed.\n"
	case FixtureOutage:
		result.Stderr = "SIMULATED fixture: provider unavailable scenario.\n"
		providerErr = ErrProviderUnavailable
	default:
		exit := 0
		result.Status, result.ExitCode = StatusSuccess, &exit
		result.Stdout = "SIMULATED fixture: success scenario; source was not executed.\n"
	}
	result, err := NormalizeResult(req, result)
	if err != nil {
		return result, err
	}
	return result, providerErr
}
