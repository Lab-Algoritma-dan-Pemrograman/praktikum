package migrationrehearsal

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
)

const (
	FormatVersion    = "1"
	MaxSourceBytes   = 64 << 20
	MaxRowBytes      = 1 << 20
	MaxManifestBytes = 1 << 20
	MaxPlanBytes     = 128 << 20
	PolicyVersion    = "synthetic-v1"
)

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	batchPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
)

type Manifest struct {
	FormatVersion    string                  `json:"format_version"`
	BatchID          string                  `json:"batch_id"`
	SourceNamespace  string                  `json:"source_namespace"`
	SourceKind       string                  `json:"source_kind"`
	Cutoff           time.Time               `json:"cutoff"`
	Files            []SourceFile            `json:"files"`
	TargetMigrations []migrations.Definition `json:"target_migrations"`
	Expected         Expected                `json:"expected"`
	PolicyVersion    string                  `json:"policy_version"`
}

type SourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Rows   int    `json:"rows"`
}

type Expected struct {
	Counts  map[string]int `json:"counts"`
	Reasons map[string]int `json:"reasons"`
}

type SourceRow struct {
	SourceSystem string          `json:"source_system"`
	Entity       string          `json:"entity"`
	SourceRef    string          `json:"source_ref"`
	Payload      json.RawMessage `json:"payload"`
	fileIndex    int
	line         int
}

func LoadManifest(path string) (*Manifest, []SourceRow, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("manifest: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("manifest must be a regular non-symlink file")
	}
	body, err := readBoundedFile(path, MaxManifestBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("manifest: %w", err)
	}
	var manifest Manifest
	if err := decodeStrict(body, &manifest); err != nil {
		return nil, nil, fmt.Errorf("manifest JSON: %w", err)
	}
	if err := validateManifest(&manifest); err != nil {
		return nil, nil, err
	}
	root, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, nil, err
	}
	rows, err := readSourceFiles(root, manifest.Files)
	if err != nil {
		return nil, nil, err
	}
	return &manifest, rows, nil
}

func validateManifestHeader(m *Manifest) error {
	if m == nil {
		return errors.New("manifest is required")
	}
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("unsupported format_version %q", m.FormatVersion)
	}
	if !batchPattern.MatchString(m.BatchID) {
		return errors.New("invalid batch_id")
	}
	if strings.TrimSpace(m.SourceNamespace) == "" || len(m.SourceNamespace) > 256 {
		return errors.New("source_namespace is required and limited to 256 bytes")
	}
	if m.SourceKind != "synthetic" {
		return errors.New("only synthetic source_kind is supported; real adapters are blocked")
	}
	if m.Cutoff.IsZero() || m.Cutoff.Location() != time.UTC {
		return errors.New("cutoff must be a fixed UTC timestamp")
	}
	if m.PolicyVersion != PolicyVersion {
		return errors.New("unsupported policy_version")
	}
	expected := migrations.Catalog()
	if len(m.TargetMigrations) != len(expected) {
		return errors.New("target migration catalog does not match embedded catalog")
	}
	for i := range expected {
		if m.TargetMigrations[i] != expected[i] {
			return errors.New("target migration catalog does not match embedded catalog")
		}
	}
	for key, value := range m.Expected.Counts {
		if !validDisposition(Disposition(key)) || value < 0 {
			return fmt.Errorf("invalid expected count %q", key)
		}
	}
	for key, value := range m.Expected.Reasons {
		if !validReason(key) || value < 0 {
			return fmt.Errorf("invalid expected reason %q", key)
		}
	}
	return nil
}

func validateManifest(m *Manifest) error {
	if err := validateManifestHeader(m); err != nil {
		return err
	}
	if len(m.Files) == 0 || len(m.Files) > 256 {
		return errors.New("files must not be empty")
	}
	seen := make(map[string]bool, len(m.Files))
	var total int64
	for _, file := range m.Files {
		clean := filepath.Clean(filepath.FromSlash(file.Path))
		if !safeSourcePath(file.Path) || filepath.IsAbs(file.Path) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid source file path %q", file.Path)
		}
		key := filepath.ToSlash(clean)
		if seen[key] {
			return fmt.Errorf("duplicate source file %q", file.Path)
		}
		seen[key] = true
		if !sha256Pattern.MatchString(file.SHA256) || file.Bytes < 0 || file.Bytes > MaxSourceBytes || file.Rows < 0 || file.Rows > MaxSourceBytes/2 {
			return fmt.Errorf("invalid metadata for source file %q", file.Path)
		}
		total += file.Bytes
		if total > MaxSourceBytes {
			return errors.New("source files exceed 64 MiB limit")
		}
	}
	return nil
}

func readSourceFiles(root string, files []SourceFile) ([]SourceRow, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve fixture root: %w", err)
	}
	var all []SourceRow
	seenRefs := make(map[string]bool)
	for fileIndex, definition := range files {
		if !safeSourcePath(definition.Path) {
			return nil, errors.New("unsafe source path")
		}
		// A symlink in any source-relative component is excluded, even when it
		// currently resolves inside the fixture directory.
		componentPath := root
		for _, component := range strings.Split(definition.Path, "/") {
			componentPath = filepath.Join(componentPath, component)
			info, err := os.Lstat(componentPath)
			if err != nil {
				return nil, err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, errors.New("symlink source component is prohibited")
			}
		}
		path := filepath.Join(root, filepath.FromSlash(definition.Path))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("source file %q: %w", definition.Path, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("source file %q must be a regular non-symlink file", definition.Path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, fmt.Errorf("resolve source file %q: %w", definition.Path, err)
		}
		rel, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("source file %q escapes fixture root", definition.Path)
		}
		body, err := readBoundedFile(path, definition.Bytes)
		if err != nil {
			return nil, fmt.Errorf("read source file %q: %w", definition.Path, err)
		}
		if int64(len(body)) != definition.Bytes {
			return nil, fmt.Errorf("source file %q byte count mismatch", definition.Path)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != definition.SHA256 {
			return nil, fmt.Errorf("source file %q checksum mismatch", definition.Path)
		}
		rows, err := decodeRows(body, fileIndex)
		if err != nil {
			return nil, fmt.Errorf("source file %q: %w", definition.Path, err)
		}
		if len(rows) != definition.Rows {
			return nil, fmt.Errorf("source file %q row count mismatch", definition.Path)
		}
		for _, row := range rows {
			key := row.SourceSystem + "\x00" + row.Entity + "\x00" + row.SourceRef
			if seenRefs[key] {
				return nil, fmt.Errorf("duplicate source row at file %d line %d", fileIndex+1, row.line)
			}
			seenRefs[key] = true
			all = append(all, row)
		}
	}
	return all, nil
}

func decodeRows(body []byte, fileIndex int) ([]SourceRow, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64<<10), MaxRowBytes+1)
	rows := make([]SourceRow, 0)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, fmt.Errorf("blank NDJSON line %d", len(rows)+1)
		}
		if len(line) > MaxRowBytes {
			return nil, fmt.Errorf("NDJSON row %d exceeds 1 MiB", len(rows)+1)
		}
		var row SourceRow
		if err := decodeStrict(line, &row); err != nil {
			return nil, fmt.Errorf("invalid NDJSON row %d", len(rows)+1)
		}
		row.fileIndex, row.line = fileIndex, len(rows)+1
		if !validSourceIdentity(row) || len(row.Payload) == 0 {
			return nil, fmt.Errorf("invalid required fields in NDJSON row %d", row.line)
		}
		if _, err := ParseCanonicalJSON(row.Payload); err != nil {
			return nil, fmt.Errorf("invalid payload in NDJSON row %d", row.line)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("NDJSON row exceeds 1 MiB or cannot be read")
	}
	return rows, nil
}

func safeSourcePath(path string) bool {
	if len(path) == 0 || len(path) > 512 || strings.ContainsAny(path, "\\\\:\x00") {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "." || component == ".." || !batchPattern.MatchString(component) || strings.HasSuffix(component, ".") {
			return false
		}
		base := strings.ToLower(strings.Split(component, ".")[0])
		if base == "con" || base == "prn" || base == "aux" || base == "nul" || regexp.MustCompile(`^(com|lpt)[1-9]$`).MatchString(base) {
			return false
		}
	}
	return true
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	if limit < 0 || limit > MaxPlanBytes {
		return nil, errors.New("invalid file size limit")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, errors.New("file is not regular or exceeds size limit")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("file exceeds size limit")
	}
	return body, nil
}

func decodeStrict(body []byte, dst any) error {
	if _, err := ParseCanonicalJSON(body); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
