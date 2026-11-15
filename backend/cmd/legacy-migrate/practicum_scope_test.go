package main

import (
	"strings"
	"testing"
)

func TestPracticumMappingCanonicalLegacyContext(t *testing.T) {
	body, err := mappingFiles.ReadFile("sql/030_practicum.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"jsonb_build_object('domain','praktikum','session_id','legacy-session-' || s.legacy_id)",
		"select 'legacy-course-' || c.legacy_id, parent.id, 0, parent.scope",
		"select 'legacy-question-' || q.legacy_id, parent.id, 0, parent.scope",
		"join public.practicum_sessions parent on parent.id = 'legacy-session-' || (c.row_data->>'sesi_praktikum_id')",
		"join public.practicum_courses parent on parent.id = 'legacy-course-' || (q.row_data->>'course_id')",
		"'session_id', act.parent_id", "'scope', act.scope",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("missing canonical ancestry expression %q", fragment)
		}
	}
	if strings.Count(sql, "'domain', 'praktikum'") != 2 || strings.Count(sql, "'session_id', 'legacy-session-' || (s.row_data->>'sesi_praktikum_id')") != 3 {
		t.Fatal("both activation scopes must carry domain and real session ancestry")
	}
	definitions := strings.Split(strings.Split(sql, "insert into public.practicum_courses")[1], "insert into public.practicum_activations")[0]
	if strings.Contains(definitions, "'scope',") {
		t.Fatal("course/question JSON definitions must not gain invented scope fields")
	}
	for _, invented := range []string{"'institution'", "'division'", "'legacy-class-' || coalesce(s.row_data->>'kelas_id', '')"} {
		if strings.Contains(sql, invented) {
			t.Errorf("generator must not invent restrictions: %s", invented)
		}
	}
	verification, err := mappingFiles.ReadFile("sql/090_verify_domain.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"session", "course", "question", "activation"} {
		if !strings.Contains(string(verification), kind+" context mismatch") {
			t.Errorf("missing %s context verification", kind)
		}
	}
	if !strings.Contains(string(verification), "left join public.practicum_activations ac on ac.id=at.parent_id") {
		t.Fatal("orphan attempts must not disappear from verification")
	}
}
