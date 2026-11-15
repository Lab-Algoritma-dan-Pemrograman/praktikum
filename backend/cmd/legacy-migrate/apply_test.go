package main

import (
	"strings"
	"testing"
)

// TestPracticumMappingPreservesActivationScopeShift guards a regression that
// silently locked every student out of their practicum data: the activation row
// `scope` column carried the shift, but the embedded
// `data->'activation'->'scope'` JSON did not. practicumEligible compares the
// student's role scope (which includes shift) against that JSON copy, so the
// mismatch made Scope.Contains fail for all students. The generator must keep
// building both the row scope and the JSON scope from the same shift source.
func TestPracticumMappingUsesParentActivationMetadata(t *testing.T) {
	body, err := mappingFiles.ReadFile("sql/030_practicum.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	if !strings.Contains(sql, "'session_id', act.parent_id") || !strings.Contains(sql, "'scope', act.scope") || !strings.Contains(sql, "select act2.data, act2.scope, act2.parent_id") {
		t.Fatal("attempt metadata must come from its persisted parent activation")
	}
	if strings.Contains(sql, "km.row_data") || strings.Contains(sql, "'session_id', 'legacy-session-' || (ac.row_data->>'aktivasi_sesi_id')") {
		t.Fatal("mapping regressed to latest membership or activation-session IDs")
	}
}

func TestPracticumMappingPreservesActivationScopeShift(t *testing.T) {
	body, err := mappingFiles.ReadFile("sql/030_practicum.sql")
	if err != nil {
		t.Fatalf("read practicum mapping: %v", err)
	}
	sql := string(body)

	// The JSON scope builder must contain the shift field, not just class_id.
	jsonScope := "'scope', jsonb_strip_nulls(jsonb_build_object("
	if !strings.Contains(sql, jsonScope) {
		t.Fatalf("activation JSON scope builder missing; want %q", jsonScope)
	}

	// Both the row scope and the embedded JSON scope must pull shift from the
	// activation session row. Count occurrences of the shift expression.
	shiftExpr := "(s.row_data->>'shift')::int"
	if got := strings.Count(sql, shiftExpr); got < 2 {
		t.Fatalf("expected shift wired into both row and JSON scope (>=2 uses), got %d", got)
	}
}
