package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// ValidateRequest never interprets source or stdin; they are opaque UTF-8 data.
// Language and version matches are exact, with no alias or latest-version fallback.
func ValidateRequest(req entity.RunRequest, languages []entity.RunnerLanguage) error {
	if err := validateInput(req); err != nil {
		return err
	}
	for _, language := range languages {
		if language.Language == req.Language && language.Version == req.Version {
			return nil
		}
	}
	return ErrUnsupportedLanguage
}

func validateInput(req entity.RunRequest) error {
	if len(req.Source) == 0 || len(req.Source) > MaxSourceBytes || !utf8.ValidString(req.Source) ||
		len(req.Stdin) > MaxStdinBytes || !utf8.ValidString(req.Stdin) ||
		(req.Language != "c" && req.Language != "python") || !safeID(req.Version) || !safeID(req.RequestID) {
		return ErrInvalidRequest
	}
	return nil
}

func safeID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("-_.:", ch)) {
			return false
		}
	}
	return true
}

// NormalizeResult bounds text and the actual JSON wire size, and overwrites
// correlation metadata with the caller's data. It does not establish sandbox
// safety or change simulated provenance. UI consumers must render text, not HTML.
func NormalizeResult(req entity.RunRequest, result entity.RunResult) (entity.RunResult, error) {
	if err := validateInput(req); err != nil {
		return entity.RunResult{Status: StatusProviderError}, err
	}
	hash := sha256.Sum256([]byte(req.Source))
	result.RequestID, result.SourceHash = req.RequestID, hex.EncodeToString(hash[:])
	if !safeID(result.RunnerVersion) || result.DurationMS < 0 || result.DurationMS > 30_000 || !validStatus(result.Status) ||
		(result.ExitCode != nil && (*result.ExitCode < 0 || *result.ExitCode > 255)) ||
		(result.Status == StatusSuccess && (result.ExitCode == nil || *result.ExitCode != 0)) {
		return entity.RunResult{Status: StatusProviderError, RequestID: result.RequestID, SourceHash: result.SourceHash, Simulated: result.Simulated}, ErrInvalidResult
	}
	var changed bool
	result.Stdout, changed = boundedText(result.Stdout, MaxOutputBytes)
	result.Truncated = result.Truncated || changed
	result.Stderr, changed = boundedText(result.Stderr, MaxOutputBytes)
	result.Truncated = result.Truncated || changed
	// HTML escaping and JSON escapes can expand even bounded UTF-8 strings.
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return entity.RunResult{Status: StatusProviderError}, ErrInvalidResult
		}
		if len(encoded) <= MaxResultBytes {
			return result, nil
		}
		result.Stdout, _ = boundedText(result.Stdout, len(result.Stdout)/2)
		result.Stderr, _ = boundedText(result.Stderr, len(result.Stderr)/2)
		result.Truncated = true
	}
}

func validStatus(value string) bool {
	switch value {
	case StatusSuccess, StatusCompileError, StatusRuntimeError, StatusTimeout, StatusQuotaExceeded, StatusProviderError:
		return true
	default:
		return false
	}
}

// BoundText applies the same normalization and byte cap used for runner output.
func BoundText(text string, limit int) (string, bool) { return boundedText(text, limit) }

func boundedText(text string, limit int) (string, bool) {
	// Inspect at most limit bytes: output bombs must not trigger unbounded work.
	original := text
	if len(text) > limit {
		text = strings.ToValidUTF8(text[:limit], "")
	}
	text = strings.Map(func(ch rune) rune {
		if ch == utf8.RuneError || (unicode.IsControl(ch) && ch != '\n' && ch != '\t') || unicode.Is(unicode.Cf, ch) {
			return -1
		}
		return ch
	}, text)
	return text, text != original
}
