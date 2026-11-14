package returnto

import "testing"

func TestAllowlistRejectsExternalCredentialsAndQueries(t *testing.T) {
	allowlist := Default()
	valid, err := allowlist.Validate("https://admin.algohub.web.id/praktikum")
	if err != nil || valid.Path != "/praktikum" {
		t.Fatalf("expected valid application return_to: %v", err)
	}
	for _, value := range []string{
		"https://evil.example/",
		"https://admin.algohub.web.id@evil.example/",
		"https://admin.algohub.web.id/callback?token=secret",
		"javascript:alert(1)",
	} {
		if _, err := allowlist.Validate(value); err == nil {
			t.Fatalf("expected return_to rejection for %q", value)
		}
	}
}
