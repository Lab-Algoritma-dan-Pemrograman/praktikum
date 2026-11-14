package migrationrehearsal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestClosedAttendanceUsesOnlyInertHash(t *testing.T) {
	sum := sha256.Sum256([]byte("phase6-inert-closed-attendance"))
	if hex.EncodeToString(sum[:]) != syntheticClosedAttendanceHash {
		t.Fatal("inert attendance hash changed")
	}
	plan := fixturePlan(t)
	var payload struct {
		TokenHash string `json:"token_hash"`
	}
	if err := json.Unmarshal(rowByEntity(t, plan, "academic_attendance_session").Desired.Data, &payload); err != nil || payload.TokenHash != syntheticClosedAttendanceHash {
		t.Fatal("attendance storage hash missing")
	}
	for _, bad := range []string{"", strings.Repeat("0", 64)} {
		m, rows := SyntheticFixture()
		changePayload(t, rows, "attendance-session", func(p map[string]any) { p["data"].(map[string]any)["token_hash"] = bad })
		expectRejected(m, ReasonInvalidPayload, 1)
		expectRejected(m, ReasonMissingReference, 1)
		if _, err := BuildPlan(m, rows); err != nil {
			t.Fatal(err)
		}
	}
}

func fixturePlan(t *testing.T) *Plan {
	t.Helper()
	m, r := SyntheticFixture()
	p, e := BuildPlan(m, r)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func changePayload(t *testing.T, rows []SourceRow, label string, change func(map[string]any)) {
	t.Helper()
	for i := range rows {
		if rows[i].SourceRef == label {
			var p map[string]any
			if e := json.Unmarshal(rows[i].Payload, &p); e != nil {
				t.Fatal(e)
			}
			change(p)
			b, e := CanonicalJSON(p)
			if e != nil {
				t.Fatal(e)
			}
			rows[i].Payload = b
			return
		}
	}
	t.Fatalf("missing fixture row %s", label)
}
func expectRejected(m *Manifest, reason string, n int) {
	m.Expected.Counts[string(Imported)] -= n
	m.Expected.Counts[string(Rejected)] += n
	m.Expected.Reasons[ReasonPlanned] -= n
	m.Expected.Reasons[reason] += n
}
func rowByEntity(t *testing.T, p *Plan, kind string) PlanRow {
	t.Helper()
	for _, r := range p.Rows {
		if r.Entity == kind {
			return r
		}
	}
	t.Fatalf("missing %s", kind)
	return PlanRow{}
}

func TestSyntheticFixtureBinding(t *testing.T) {
	p := fixturePlan(t)
	if p.Counts[string(Imported)] != 21 || p.Counts[string(Rejected)] != 2 || len(p.Rows) != 23 {
		t.Fatalf("accounting %+v", p.Counts)
	}
	before, _ := CanonicalJSON(p)
	ids := map[string]int64{}
	keys := map[string]string{}
	bound := map[string]*DesiredRecord{}
	for _, r := range p.Rows {
		if r.Disposition != Imported {
			continue
		}
		if r.Entity == "user" || r.Entity == "assignment" {
			ids[r.RowToken] = int64(101 + len(ids))
			continue
		}
		d, e := bindDomain(r, ids, keys)
		if e != nil {
			t.Fatalf("%s: %v", r.Entity, e)
		}
		keys[r.RowToken] = d.ID
		bound[r.Entity] = d
		if bytes.Contains(d.Data, []byte(`"$token"`)) || bytes.Contains(d.Scope, []byte(`"$token"`)) {
			t.Fatalf("unresolved %s", r.Entity)
		}
	}
	after, _ := CanonicalJSON(p)
	if !bytes.Equal(before, after) {
		t.Fatal("binding mutated the plan")
	}
	progress := bound["learning_progress"]
	profile := bound["learning_profile"]
	attendance := bound["academic_attendance_record"]
	if !strings.HasPrefix(progress.ID, "progress-"+progress.ParentID+"-") || !strings.HasPrefix(profile.ID, "profile-") || profile.UniqueKey != profile.ID || !strings.HasPrefix(attendance.ID, "att-rec-"+attendance.ParentID+"-") {
		t.Fatal("application-composed IDs missing")
	}
	job := bound["practicum_grading_job"]
	attempt := bound["practicum_attempt"]
	if job.ID != attempt.ID || job.UniqueKey != attempt.ID || job.ParentID != attempt.ID {
		t.Fatal("job must use attempt ID")
	}
	if bound["academic_inventory_rental"].ExpiresAt == nil || bound["practicum_activation"].ExpiresAt == nil {
		t.Fatal("derived expiry missing")
	}
}
func TestDeterministicReorder(t *testing.T) {
	m, r := SyntheticFixture()
	a, e := BuildPlan(m, r)
	if e != nil {
		t.Fatal(e)
	}
	sort.Slice(r, func(i, j int) bool { return r[i].SourceRef > r[j].SourceRef })
	b, e := BuildPlan(m, r)
	if e != nil {
		t.Fatal(e)
	}
	aa, _ := CanonicalJSON(a)
	bb, _ := CanonicalJSON(b)
	if !bytes.Equal(aa, bb) {
		t.Fatal("input ordering changed plan")
	}
}
func TestQualifiedSourceSystemSeparation(t *testing.T) {
	m, rows := SyntheticFixture()
	var level SourceRow
	for _, r := range rows {
		if r.Entity == "learning_level" {
			level = r
		}
	}
	other := level
	other.SourceSystem = "other"
	other.Payload = bytes.ReplaceAll(other.Payload, []byte("fixture/learning_level/level"), []byte("other/learning_level/level"))
	m.Expected = Expected{Counts: map[string]int{"imported": 2}, Reasons: map[string]int{ReasonPlanned: 2}}
	p, e := BuildPlan(m, []SourceRow{level, other})
	if e != nil {
		t.Fatal(e)
	}
	if p.Rows[0].TargetID == p.Rows[1].TargetID || p.Rows[0].RowToken == p.Rows[1].RowToken {
		t.Fatal("source system collision")
	}
}
func TestRejectedParentTransitive(t *testing.T) {
	m, r := SyntheticFixture()
	changePayload(t, r, "level", func(p map[string]any) { p["data"].(map[string]any)["title"] = "" })
	expectRejected(m, ReasonInvalidPayload, 1)
	expectRejected(m, ReasonMissingReference, 3)
	p, e := BuildPlan(m, r)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"learning_level", "learning_module", "learning_lesson", "learning_progress"} {
		if rowByEntity(t, p, kind).Disposition != Rejected {
			t.Fatal("accepted rejected ancestor", kind)
		}
	}
}
func TestWrongAndMalformedReferences(t *testing.T) {
	tests := map[string]any{
		"wrong_entity":         map[string]any{"$ref": "fixture/user/student"},
		"unqualified":          map[string]any{"$ref": "lesson"},
		"extra_component":      map[string]any{"$ref": "fixture/learning_lesson/lesson/extra"},
		"empty_component":      map[string]any{"$ref": "fixture//lesson"},
		"extra_key":            map[string]any{"$ref": "fixture/learning_lesson/lesson", "extra": true},
		"wrong_type":           map[string]any{"$ref": 123},
		"injected_token":       map[string]any{"$token": "anything"},
		"literal_target_id":    "les_arbitrary",
		"other_system_missing": map[string]any{"$ref": "other/learning_lesson/lesson"},
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			m, r := SyntheticFixture()
			changePayload(t, r, "progress", func(p map[string]any) { p["data"].(map[string]any)["lesson_id"] = value })
			expectRejected(m, ReasonMissingReference, 1)
			p, e := BuildPlan(m, r)
			if e != nil {
				t.Fatal(e)
			}
			if rowByEntity(t, p, "learning_progress").Disposition != Rejected {
				t.Fatal("bad reference accepted")
			}
		})
	}
}
func TestMetadataAndStaticValidation(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"mismatched_status": func(p map[string]any) { p["status"] = "completed" },
		"mismatched_owner": func(p map[string]any) {
			p["data"].(map[string]any)["user_id"] = map[string]any{"$ref": "fixture/user/teacher"}
		},
		"unknown_data_field": func(p map[string]any) { p["data"].(map[string]any)["legacy_state"] = "unknown" },
		"xp_inference":       func(p map[string]any) { p["data"].(map[string]any)["xp_awarded"] = 40 },
		"invalid_unique_key": func(p map[string]any) { p["unique_key"] = "wrong" },
	} {
		t.Run(name, func(t *testing.T) {
			m, r := SyntheticFixture()
			changePayload(t, r, "progress", mutate)
			expectRejected(m, ReasonInvalidPayload, 1)
			p, e := BuildPlan(m, r)
			if e != nil {
				t.Fatal(e)
			}
			if rowByEntity(t, p, "learning_progress").Disposition != Rejected {
				t.Fatal("invalid static data accepted")
			}
		})
	}
}
func TestExcludedEntitiesAreRejected(t *testing.T) {
	m, _ := SyntheticFixture()
	rows := []SourceRow{}
	reasons := map[string]int{}
	for kind, s := range entitySpecs {
		if s.skip || s.kind == "unsupported" {
			rows = append(rows, SourceRow{SourceSystem: "fixture", Entity: kind, SourceRef: "excluded", Payload: json.RawMessage(`{}`)})
			if s.skip {
				reasons[ReasonEphemeralSkipped]++
			} else {
				reasons[ReasonUnsupported]++
			}
		}
	}
	m.Expected = Expected{Counts: map[string]int{"rejected": len(rows)}, Reasons: reasons}
	p, e := BuildPlan(m, rows)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range p.Rows {
		if r.Disposition != Rejected || r.Desired != nil {
			t.Fatal("excluded entity silently skipped")
		}
	}
}
func TestMalformedInputsAndExactExpectations(t *testing.T) {
	for _, name := range []string{"nil_manifest", "real_adapter", "unknown_policy", "malformed_json", "duplicate_key", "bad_identity", "duplicate_identity", "duplicate_auth", "unexpected_reason", "unexpected_count"} {
		t.Run(name, func(t *testing.T) {
			m, r := SyntheticFixture()
			switch name {
			case "nil_manifest":
				m = nil
			case "real_adapter":
				m.SourceKind = "sanitized"
			case "unknown_policy":
				m.PolicyVersion = "other"
			case "malformed_json":
				r[0].Payload = []byte(`{"broken"`)
			case "duplicate_key":
				r[0].Payload = []byte(`{"x":1,"x":2}`)
			case "bad_identity":
				r[0].SourceRef = "../student"
			case "duplicate_identity":
				r = append(r, r[0])
			case "duplicate_auth":
				changePayload(t, r, "teacher", func(p map[string]any) { p["auth_user_id"] = SyntheticStudentAuthID })
			case "unexpected_reason":
				delete(m.Expected.Reasons, ReasonEphemeralSkipped)
			case "unexpected_count":
				delete(m.Expected.Counts, "rejected")
			}
			if _, e := BuildPlan(m, r); e == nil {
				t.Fatal("malformed contract accepted")
			}
		})
	}
}
func TestVerifyPlanStructuralHardening(t *testing.T) {
	for _, name := range []string{"format", "policy", "catalog", "table", "counts", "token", "closure", "rejected_parent", "duplicate_auth", "skipped", "desired_id", "bound_status"} {
		t.Run(name, func(t *testing.T) {
			p := fixturePlan(t)
			switch name {
			case "format":
				p.FormatVersion = "bad"
			case "policy":
				p.PolicyVersion = "bad"
			case "catalog":
				p.TargetMigrations = p.TargetMigrations[:1]
			case "table":
				p.Rows[0].Table = "auth.users"
			case "counts":
				p.Counts["imported"]++
			case "token":
				p.Rows[1].RowToken = p.Rows[0].RowToken
			case "closure":
				for i := range p.Rows {
					if p.Rows[i].Entity == "learning_progress" {
						p.Rows[i].Desired.ParentToken = "missing"
					}
				}
			case "rejected_parent":
				for i := range p.Rows {
					if p.Rows[i].Entity == "learning_level" {
						reject(&p.Rows[i], ReasonInvalidPayload)
						p.Counts["imported"]--
						p.Counts["rejected"]++
					}
				}
			case "duplicate_auth":
				p.Rows[1].Desired.AuthUserID = p.Rows[0].Desired.AuthUserID
			case "skipped":
				p.Rows[0].Disposition = IntentionallySkipped
			case "desired_id":
				for i := range p.Rows {
					if p.Rows[i].Entity == "learning_lesson" {
						p.Rows[i].Desired.ID = "other"
					}
				}
			case "bound_status":
				for i := range p.Rows {
					if p.Rows[i].Entity == "learning_progress" {
						p.Rows[i].Desired.Status = "completed"
					}
				}
			}
			p.Digest = digestWithoutPlanDigest(p)
			if e := VerifyPlan(p); e == nil {
				t.Fatal("tampered structure passed with recomputed digest")
			}
		})
	}
}
func TestCanonicalJSONExtremeDecimalRoundTrip(t *testing.T) {
	for _, input := range []string{"1e308", "1e-308", "-1e308", "123456789012345678901234567890.123456789"} {
		a, err := ParseCanonicalJSON([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseCanonicalJSON(a)
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("canonical number is not idempotent: %s (%v)", input, err)
		}
	}
}

func TestLiteralReferenceTextIsNotRewritten(t *testing.T) {
	m, rows := SyntheticFixture()
	changePayload(t, rows, "lesson", func(p map[string]any) {
		p["data"].(map[string]any)["explanation"] = `Literal {"$ref":"fixture/user/student"} is lesson text.`
	})
	plan, err := BuildPlan(m, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(rowByEntity(t, plan, "learning_lesson").Desired.Data, []byte("fixture/user/student")) {
		t.Fatal("free text was rewritten")
	}
}

func TestCanonicalJSONNumbersAndKeys(t *testing.T) {
	for in, want := range map[string]string{`{"b":1e2,"a":1.2300}`: `{"a":1.23,"b":100}`, `[-0,0.000,12.50e-1,1e-3]`: `[0,0,1.25,0.001]`, `9007199254740993.000`: `9007199254740993`, `[3,1,2]`: `[3,1,2]`} {
		got, e := ParseCanonicalJSON([]byte(in))
		if e != nil || string(got) != want {
			t.Fatalf("%s => %s (%v)", in, got, e)
		}
	}
	for _, in := range []string{`{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `1 2`, `NaN`, `1e309`, `1e100000`, `1e-100000`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		if _, e := ParseCanonicalJSON([]byte(in)); e == nil {
			t.Fatalf("accepted %s", in)
		}
	}
}
func writeFixtureFiles(t *testing.T) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	m, rows := SyntheticFixture()
	var body bytes.Buffer
	for _, r := range rows {
		b, e := CanonicalJSON(r)
		if e != nil {
			t.Fatal(e)
		}
		body.Write(b)
		body.WriteByte('\n')
	}
	sum := sha256.Sum256(body.Bytes())
	m.Files = []SourceFile{{Path: "rows.ndjson", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(body.Len()), Rows: len(rows)}}
	if e := os.WriteFile(filepath.Join(dir, "rows.ndjson"), body.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	return dir, m
}
func TestManifestFilesAndRoundTrip(t *testing.T) {
	dir, m := writeFixtureFiles(t)
	path := filepath.Join(dir, "manifest.json")
	if e := WriteJSONAtomic(path, m, false); e != nil {
		t.Fatal(e)
	}
	loaded, rows, e := LoadManifest(path)
	if e != nil {
		t.Fatal(e)
	}
	p, e := BuildPlan(loaded, rows)
	if e != nil {
		t.Fatal(e)
	}
	planPath := filepath.Join(dir, "plan.json")
	if e = WriteJSONAtomic(planPath, p, false); e != nil {
		t.Fatal(e)
	}
	if _, e = LoadPlan(planPath); e != nil {
		t.Fatal(e)
	}
	if e = WriteJSONAtomic(planPath, p, false); e == nil {
		t.Fatal("overwrote output")
	}
}
func TestSafeFilePathsAndLimits(t *testing.T) {
	for _, path := range []string{"../outside.ndjson", "a/../../outside", "/abs.ndjson", "C:/abs.ndjson", "a\\b.ndjson", "a//b.ndjson", "a/./b.ndjson", "nul", "con.json", "com1.txt", "trailing.", "file:stream"} {
		t.Run(path, func(t *testing.T) {
			m, _ := SyntheticFixture()
			m.Files[0].Path = path
			if e := validateManifest(m); e == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	m, _ := SyntheticFixture()
	m.Files[0].Bytes = MaxSourceBytes + 1
	if e := validateManifest(m); e == nil {
		t.Fatal("oversize file metadata accepted")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "large")
	if e := os.WriteFile(path, []byte("12345"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := readBoundedFile(path, 4); e == nil {
		t.Fatal("unbounded read")
	}
	for _, body := range [][]byte{[]byte("\n"), []byte(`{"source_system":"fixture","entity":"user","source_ref":"x","payload":{},"unknown":true}`), bytes.Repeat([]byte("x"), MaxRowBytes+1)} {
		if _, e := decodeRows(body, 0); e == nil {
			t.Fatal("malformed NDJSON accepted")
		}
	}
}
func TestManifestChecksumAndSizeMismatch(t *testing.T) {
	for _, name := range []string{"checksum", "bytes", "rows"} {
		t.Run(name, func(t *testing.T) {
			dir, m := writeFixtureFiles(t)
			switch name {
			case "checksum":
				m.Files[0].SHA256 = strings.Repeat("0", 64)
			case "bytes":
				m.Files[0].Bytes--
			case "rows":
				m.Files[0].Rows--
			}
			path := filepath.Join(dir, "manifest.json")
			if e := WriteJSONAtomic(path, m, false); e != nil {
				t.Fatal(e)
			}
			if _, _, e := LoadManifest(path); e == nil {
				t.Fatal("inconsistent file accepted")
			}
		})
	}
}
