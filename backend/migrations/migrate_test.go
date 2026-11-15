package migrations

import (
	"strings"
	"testing"
)

// TestActivationScopeShiftMigrationEmbedded ensures the backfill that restores
// `shift` into the embedded activation JSON scope stays part of the catalog.
// Without it, legacy-migrated activations keep a class-only scope and the
// practicum eligibility check rejects every student.
func TestPracticumAttemptMetadataRepairEmbedded(t *testing.T) {
	for _, def := range Catalog() {
		if def.Name == "0011_practicum_attempt_metadata_repair.sql" {
			return
		}
	}
	t.Fatal("forward attempt metadata repair missing from migration catalog")
}

func TestActivationScopeShiftMigrationEmbedded(t *testing.T) {
	var found bool
	for _, def := range Catalog() {
		if def.Name != "0009_activation_scope_shift.sql" {
			continue
		}
		found = true
		body, err := files.ReadFile(def.Name)
		if err != nil {
			t.Fatalf("read %s: %v", def.Name, err)
		}
		sql := string(body)
		if !strings.Contains(sql, "practicum_activations") ||
			!strings.Contains(sql, "'{activation,scope}'") ||
			!strings.Contains(sql, "'shift'") {
			t.Fatalf("%s does not backfill activation scope shift", def.Name)
		}
	}
	if !found {
		t.Fatal("0009_activation_scope_shift.sql missing from migration catalog")
	}
}

// TestActivationRosterBackfillEmbedded ensures the roster backfill (add attempt
// owners to student_ids) stays in the catalog and re-enables the freeze trigger
// it must briefly disable. Without it, students whose legacy shift changed keep
// their own attempts hidden.
func TestActivationRosterBackfillEmbedded(t *testing.T) {
	var found bool
	for _, def := range Catalog() {
		if def.Name != "0010_activation_roster_backfill.sql" {
			continue
		}
		found = true
		body, err := files.ReadFile(def.Name)
		if err != nil {
			t.Fatalf("read %s: %v", def.Name, err)
		}
		sql := string(body)
		if !strings.Contains(sql, "'{activation,student_ids}'") ||
			!strings.Contains(sql, "disable trigger practicum_activation_frozen") ||
			!strings.Contains(sql, "enable trigger practicum_activation_frozen") {
			t.Fatalf("%s must backfill student_ids and toggle the freeze trigger", def.Name)
		}
	}
	if !found {
		t.Fatal("0010_activation_roster_backfill.sql missing from migration catalog")
	}
}
