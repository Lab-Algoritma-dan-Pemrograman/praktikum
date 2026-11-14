package returnto

import (
	"errors"
	"net/url"
	"path"
	"strings"
	"unicode"
)

var ErrNotAllowed = errors.New("return_to is not allowlisted")

type Rule struct {
	Origin      string
	PathPrefix  string
	LocalOrigin string
}

type Allowlist struct {
	rules []Rule
}

func New(rules []Rule) Allowlist { return Allowlist{rules: append([]Rule(nil), rules...)} }

func (a Allowlist) Validate(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || parsed.User != nil || parsed.Host == "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, ErrNotAllowed
	}
	decodedPath := parsed.Path
	if decodedPath == "" {
		decodedPath = "/"
	}
	if strings.Contains(decodedPath, "\\") || strings.Contains(decodedPath, "%") || strings.HasPrefix(decodedPath, "//") || strings.TrimSuffix(decodedPath, "/") != strings.TrimSuffix(path.Clean(decodedPath), "/") {
		return nil, ErrNotAllowed
	}
	for _, char := range decodedPath {
		if unicode.IsControl(char) {
			return nil, ErrNotAllowed
		}
	}
	for _, rule := range a.rules {
		origin := parsed.Scheme + "://" + parsed.Host
		if origin != rule.Origin && origin != rule.LocalOrigin {
			continue
		}
		prefix := strings.TrimSuffix(rule.PathPrefix, "/")
		if prefix == "" || decodedPath == prefix || strings.HasPrefix(decodedPath, prefix+"/") {
			return parsed, nil
		}
	}
	return nil, ErrNotAllowed
}

func FromOrigins(origins []string) Allowlist {
	rules := make([]Rule, 0, len(origins))
	for _, origin := range origins {
		rules = append(rules, Rule{Origin: origin, PathPrefix: "/"})
	}
	return New(rules)
}

func Default() Allowlist {
	return New([]Rule{
		{Origin: "https://algohub.web.id", LocalOrigin: "http://localhost:5173", PathPrefix: "/"},
		{Origin: "https://admin.algohub.web.id", LocalOrigin: "http://localhost:5174", PathPrefix: "/"},
		{Origin: "https://siakad.algohub.web.id", PathPrefix: "/"},
		{Origin: "https://elearning.algohub.web.id", PathPrefix: "/"},
		{Origin: "https://praktikum.algohub.web.id", PathPrefix: "/"},
	})
}
