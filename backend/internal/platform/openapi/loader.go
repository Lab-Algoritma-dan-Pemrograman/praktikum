package openapi

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed spec.yaml
var bundledYAML []byte

//go:embed spec.json
var bundledJSON []byte

type Bundle struct {
	YAML []byte
	JSON []byte
}

func Load(configuredPath string) Bundle {
	for _, candidate := range []string{configuredPath, "docs/openapi/openapi.yaml", filepath.Join("..", "docs", "openapi", "openapi.yaml")} {
		if candidate == "" {
			continue
		}
		yaml, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		json, err := os.ReadFile(strings.TrimSuffix(candidate, filepath.Ext(candidate)) + ".json")
		if err == nil {
			return Bundle{YAML: yaml, JSON: json}
		}
	}
	return Bundle{YAML: bundledYAML, JSON: bundledJSON}
}
