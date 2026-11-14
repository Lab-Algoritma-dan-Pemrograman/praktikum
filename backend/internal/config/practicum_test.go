package config

import (
	"strings"
	"testing"
)

func TestPracticumCronSecretValidation(t *testing.T) {
	t.Setenv("VERCEL", "")
	for _, secret := range []string{"", strings.Repeat("a", 32), strings.Repeat("a", 256)} {
		cfg := localFixtureConfig()
		cfg.PracticumEnabled = true
		cfg.PracticumCronSecret = secret
		if err := cfg.Validate(); err != nil {
			t.Fatalf("valid cron configuration rejected: %v", err)
		}
	}
	for _, secret := range []string{"short", strings.Repeat("a", 257), strings.Repeat("a", 32) + "\n"} {
		cfg := localFixtureConfig()
		cfg.PracticumEnabled = true
		cfg.PracticumCronSecret = secret
		if err := cfg.Validate(); err == nil {
			t.Fatal("invalid cron secret accepted")
		}
	}
}
func TestPracticumReleaseDisabledByDefault(t *testing.T) {
	t.Setenv("ALGOHUB_PRACTICUM_ENABLED", "")
	if Load().PracticumEnabled {
		t.Fatal("Practicum silently released")
	}
	t.Setenv("ALGOHUB_PRACTICUM_ENABLED", "true")
	if !Load().PracticumEnabled {
		t.Fatal("explicit release switch ignored")
	}
}
