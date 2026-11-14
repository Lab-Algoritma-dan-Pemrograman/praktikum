package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnapshotColumnAllowlistExcludesCredentials(t *testing.T) {
	for table, cols := range snapshotColumns {
		for _, col := range strings.Fields(cols) {
			for _, bad := range []string{"password", "token", "salt", "secret", "supabase_user_id", "phone", "foto", "medsos"} {
				if strings.Contains(col, bad) {
					t.Fatalf("credential/PII column %s.%s selected", table, col)
				}
			}
		}
	}
}
func TestMaskRowsPreservesJoinsAndRemovesPrivateText(t *testing.T) {
	raw := map[string][]map[string]any{
		"users":             {{"id": float64(1), "nim": "NIM001", "nama": "Ayu Example", "email": "ayu@original.test"}, {"id": float64(2), "nim": "NIM002", "nama": "Bima Example", "email": "bima@original.test"}},
		"student_progress":  {{"id": "progress1", "nim": "NIM001", "lesson_id": "lesson1", "completed": true}},
		"jawaban_mahasiswa": {{"id": float64(3), "mahasiswa_id": float64(1), "jawaban_teks": "PRIVATE MEDICAL STORY", "feedback": "PRIVATE FEEDBACK", "nilai": float64(85)}},
		"lessons":           {{"id": "lesson1", "explanation": "AYU EXAMPLE email ayu@original.test contact 081234567890", "quiz": "{\"question\":\"NIM001\"}"}},
	}
	if err := maskRows(raw); err != nil {
		t.Fatal(err)
	}
	uid := raw["users"][0]["id"].(int64)
	if uid < 1000000000 || uid == raw["users"][1]["id"].(int64) {
		t.Fatal("opaque IDs missing")
	}
	if raw["jawaban_mahasiswa"][0]["mahasiswa_id"] != uid || raw["student_progress"][0]["nim"] != raw["users"][0]["nim"] {
		t.Fatal("masked joins broken")
	}
	if raw["jawaban_mahasiswa"][0]["nilai"] != float64(85) {
		t.Fatal("academic numeric value changed")
	}
	body, _ := json.Marshal(raw)
	for _, private := range []string{"NIM001", "NIM002", "Ayu Example", "AYU EXAMPLE", "Bima Example", "original.test", "081234567890", "PRIVATE MEDICAL STORY", "PRIVATE FEEDBACK"} {
		if strings.Contains(string(body), private) {
			t.Fatalf("private text remained: %s", private)
		}
	}
}
func TestMaskRowsRejectsUnresolvedUser(t *testing.T) {
	raw := map[string][]map[string]any{"users": {{"id": float64(1), "nim": "TEST1", "nama": "Synthetic User", "email": "s@test.invalid"}}, "jawaban_mahasiswa": {{"mahasiswa_id": float64(2)}}}
	if err := maskRows(raw); err == nil {
		t.Fatal("orphan accepted")
	}
}
