package migrationrehearsal

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/migrations"
)

type Disposition string

const (
	Imported             Disposition = "imported"
	Unchanged            Disposition = "unchanged"
	Rejected             Disposition = "rejected"
	IntentionallySkipped Disposition = "intentionally_skipped"
)
const (
	ReasonPlanned          = "planned_insert"
	ReasonEqual            = "equal_existing"
	ReasonInserted         = "inserted"
	ReasonInvalidEntity    = "entity_not_allowlisted"
	ReasonInvalidPayload   = "invalid_synthetic_contract"
	ReasonMissingReference = "missing_symbolic_reference"
	ReasonEphemeralSkipped = "ephemeral_policy_rejected"
	ReasonUnsupported      = "unsupported_synthetic_entity"
	ReasonConflict         = "different_existing_row"
	ReasonRolledBack       = "rolled_back"
	ReasonRollbackMismatch = "rollback_digest_mismatch"
	ReasonCheckPassed      = "check_passed"
	ReasonCheckDeferred    = "check_deferred"
)

type entitySpec struct {
	table, prefix string
	order         int
	kind          string
	skip          bool
}

var entitySpecs = map[string]entitySpec{
	"user":                        {"users", "usr_", 10, "user", false},
	"assignment":                  {"user_role_assignments", "asn_", 20, "assignment", false},
	"grant":                       {"user_permission_grants", "grt_", 30, "unsupported", false},
	"practicum_session":           {"practicum_sessions", "psn_", 40, "generic", false},
	"practicum_course":            {"practicum_courses", "pco_", 50, "generic", false},
	"practicum_question":          {"practicum_questions", "pqu_", 60, "generic", false},
	"practicum_activation":        {"practicum_activations", "pac_", 70, "generic", false},
	"practicum_attempt":           {"practicum_attempts", "pat_", 80, "generic", false},
	"practicum_grading_job":       {"practicum_grading_jobs", "pgj_", 90, "generic", false},
	"learning_level":              {"learning_levels", "lvl_", 100, "generic", false},
	"learning_module":             {"learning_modules", "mod_", 110, "generic", false},
	"learning_lesson":             {"learning_lessons", "les_", 120, "generic", false},
	"learning_progress":           {"learning_progress", "lpr_", 130, "generic", false},
	"learning_profile":            {"learning_profiles", "lpf_", 140, "generic", false},
	"learning_bug_hunt":           {"learning_bug_hunt", "bug_", 150, "unsupported", false},
	"learning_game_history":       {"learning_game_history", "lgh_", 160, "unsupported", false},
	"academic_class":              {"academic_classes", "cls_", 170, "generic", false},
	"academic_schedule":           {"academic_schedules", "sch_", 180, "generic", false},
	"academic_attendance_session": {"academic_attendance_sessions", "ats_", 190, "generic", false},
	"academic_attendance_record":  {"academic_attendance_records", "atr_", 200, "generic", false},
	"academic_inventory_item":     {"academic_inventory_items", "itm_", 210, "generic", false},
	"academic_inventory_rental":   {"academic_inventory_rentals", "rnt_", 220, "generic", false},
	"audit":                       {"audit_logs", "aud_", 230, "unsupported", false},
	"auth_session":                {"auth_sessions", "ses_", 900, "skip", true},
	"sso_handshake":               {"sso_handshakes", "sso_", 900, "skip", true},
	"auth_rate_limit":             {"auth_rate_limits", "rtl_", 900, "skip", true},
	"practicum_operation":         {"practicum_operations", "pop_", 900, "skip", true},
	"practicum_runner_lease":      {"practicum_runner_leases", "prl_", 900, "skip", true},
	"learning_operation":          {"learning_operations", "lop_", 900, "skip", true},
	"learning_runner_lease":       {"learning_runner_leases", "lrl_", 900, "skip", true},
	"academic_operation":          {"academic_operations", "aop_", 900, "skip", true},
	"academic_notification":       {"academic_notifications", "not_", 900, "skip", true},
}

type Plan struct {
	FormatVersion    string                  `json:"format_version"`
	BatchID          string                  `json:"batch_id"`
	SourceNamespace  string                  `json:"source_namespace"`
	SourceKind       string                  `json:"source_kind"`
	Cutoff           time.Time               `json:"cutoff"`
	PolicyVersion    string                  `json:"policy_version"`
	TargetMigrations []migrations.Definition `json:"target_migrations"`
	Rows             []PlanRow               `json:"rows"`
	Counts           map[string]int          `json:"counts"`
	Digest           string                  `json:"digest"`
}
type PlanRow struct {
	RowToken    string         `json:"row_token"`
	Table       string         `json:"table"`
	Entity      string         `json:"entity"`
	Disposition Disposition    `json:"disposition"`
	Reason      string         `json:"reason"`
	TargetID    string         `json:"target_id,omitempty"`
	Desired     *DesiredRecord `json:"desired,omitempty"`
	order       int
}
type DesiredRecord struct {
	ID              string          `json:"id,omitempty"`
	ParentID        string          `json:"parent_id,omitempty"`
	ParentToken     string          `json:"parent_token,omitempty"`
	OwnerToken      string          `json:"owner_token,omitempty"`
	Scope           json.RawMessage `json:"scope,omitempty"`
	Status          string          `json:"status,omitempty"`
	UniqueKey       string          `json:"unique_key,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
	AuthUserID      string          `json:"auth_user_id,omitempty"`
	NIM             *string         `json:"nim,omitempty"`
	Email           string          `json:"email,omitempty"`
	Name            string          `json:"name,omitempty"`
	IsActive        *bool           `json:"is_active,omitempty"`
	Hidden          *bool           `json:"hidden,omitempty"`
	UserToken       string          `json:"user_token,omitempty"`
	AssignmentToken string          `json:"assignment_token,omitempty"`
	RoleCode        string          `json:"role_code,omitempty"`
	DivisionCode    *string         `json:"division_code,omitempty"`
	PermissionCode  string          `json:"permission_code,omitempty"`
	Action          string          `json:"action,omitempty"`
	App             string          `json:"app,omitempty"`
	SubjectType     string          `json:"subject_type,omitempty"`
	SubjectID       string          `json:"subject_id,omitempty"`
	RequestID       string          `json:"request_id,omitempty"`
	OperationID     string          `json:"operation_id,omitempty"`
	Detail          json.RawMessage `json:"detail,omitempty"`
	CreatedAt       *time.Time      `json:"created_at,omitempty"`
}
type genericPayload struct {
	ParentRef string          `json:"parent_ref,omitempty"`
	OwnerRef  string          `json:"owner_ref,omitempty"`
	Scope     json.RawMessage `json:"scope"`
	Status    string          `json:"status"`
	UniqueKey string          `json:"unique_key,omitempty"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	Data      json.RawMessage `json:"data"`
}
type userPayload struct {
	AuthUserID string  `json:"auth_user_id"`
	NIM        *string `json:"nim"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	IsActive   *bool   `json:"is_active,omitempty"`
	Hidden     *bool   `json:"hidden,omitempty"`
}
type assignmentPayload struct {
	UserRef      string          `json:"user_ref"`
	RoleCode     string          `json:"role_code"`
	DivisionCode *string         `json:"division_code"`
	Scope        json.RawMessage `json:"scope"`
	IsActive     *bool           `json:"is_active,omitempty"`
}

var componentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func validSourceIdentity(r SourceRow) bool {
	return componentPattern.MatchString(r.SourceSystem) && componentPattern.MatchString(r.Entity) && componentPattern.MatchString(r.SourceRef)
}
func qualified(r SourceRow) string { return r.SourceSystem + "/" + r.Entity + "/" + r.SourceRef }
func validQualified(ref string) bool {
	p := strings.Split(ref, "/")
	return len(p) == 3 && componentPattern.MatchString(p[0]) && componentPattern.MatchString(p[1]) && componentPattern.MatchString(p[2])
}

// sourceRef must be qualified system/entity/ref. Namespace and all three source
// identity components participate in the hash; equal local IDs never merge.
func DeterministicID(prefix, namespace, entity, sourceRef string) (string, error) {
	allowed := false
	for _, s := range entitySpecs {
		if s.prefix == prefix {
			allowed = true
		}
	}
	if !allowed || namespace == "" || !validQualified(sourceRef) {
		return "", errors.New("invalid deterministic identity")
	}
	sum := sha256.Sum256([]byte(namespace + "\x00" + entity + "\x00" + sourceRef))
	return prefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:16])), nil
}
func reject(row *PlanRow, reason string) {
	row.Disposition = Rejected
	row.Reason = reason
	row.TargetID = ""
	row.Desired = nil
}
func BuildPlan(m *Manifest, rows []SourceRow) (*Plan, error) {
	if err := validateManifestHeader(m); err != nil {
		return nil, err
	}
	if len(rows) > 100000 {
		return nil, errors.New("too many source rows")
	}
	p := &Plan{FormatVersion: FormatVersion, BatchID: m.BatchID, SourceNamespace: m.SourceNamespace, SourceKind: m.SourceKind, Cutoff: m.Cutoff, PolicyVersion: m.PolicyVersion, TargetMigrations: append([]migrations.Definition(nil), m.TargetMigrations...), Counts: map[string]int{}}
	refs := map[string]PlanRow{}
	var total int
	for _, r := range rows {
		total += len(r.Payload)
		if !validSourceIdentity(r) || len(r.Payload) > MaxRowBytes || total > MaxSourceBytes {
			return nil, errors.New("invalid source row identity or size")
		}
		if _, err := ParseCanonicalJSON(r.Payload); err != nil {
			return nil, errors.New("malformed source payload")
		}
		q := qualified(r)
		if _, ok := refs[q]; ok {
			return nil, errors.New("duplicate qualified source identity")
		}
		s, ok := entitySpecs[r.Entity]
		e := PlanRow{Entity: r.Entity, Disposition: Rejected, Reason: ReasonInvalidEntity, order: 1000}
		if ok {
			e.Table = s.table
			e.order = s.order
			e.RowToken, _ = DeterministicID(s.prefix, m.SourceNamespace, "row:"+r.Entity, q)
			e.TargetID, _ = DeterministicID(s.prefix, m.SourceNamespace, r.Entity, q)
			if s.skip {
				reject(&e, ReasonEphemeralSkipped)
			} else if s.kind == "unsupported" {
				reject(&e, ReasonUnsupported)
			} else {
				e.Disposition = Imported
				e.Reason = ReasonPlanned
			}
		} else {
			sum := sha256.Sum256([]byte(m.SourceNamespace + "\x00" + q))
			e.RowToken = "row_" + hex.EncodeToString(sum[:16])
		}
		refs[q] = e
		p.Rows = append(p.Rows, e)
	}
	auths := map[string]bool{}
	for i, r := range rows {
		e := &p.Rows[i]
		if e.Disposition != Imported {
			continue
		}
		d, reason := transformRow(m, r, *e, entitySpecs[r.Entity], refs)
		if reason != "" {
			reject(e, reason)
			continue
		}
		e.Desired = d
		if r.Entity == "user" {
			if auths[d.AuthUserID] {
				return nil, errors.New("duplicate target auth identity; merging is prohibited")
			}
			auths[d.AuthUserID] = true
		}
	}
	// Rejections propagate through tokens, not stale first-pass source entries.
	remaining := append([]PlanRow(nil), p.Rows...)
	p.Rows = nil
	sort.Slice(remaining, func(i, j int) bool {
		if remaining[i].order != remaining[j].order {
			return remaining[i].order < remaining[j].order
		}
		return remaining[i].RowToken < remaining[j].RowToken
	})
	states := map[string]Disposition{}
	for _, r := range remaining {
		states[r.RowToken] = r.Disposition
	}
	ids := map[string]int64{}
	keys := map[string]string{}
	for len(remaining) > 0 {
		progressed := false
		for i := 0; i < len(remaining); {
			r := remaining[i]
			ready := true
			if r.Disposition == Imported {
				deps, err := dependencies(r)
				if err != nil {
					reject(&r, ReasonInvalidPayload)
				} else {
					for token := range deps {
						if token == r.RowToken {
							continue
						}
						state, ok := states[token]
						if !ok || state == Rejected {
							reject(&r, ReasonMissingReference)
							break
						}
						if _, ok := keys[token]; !ok {
							if _, ok := ids[token]; !ok {
								ready = false
							}
						}
					}
				}
			}
			if r.Disposition == Rejected {
				ready = true
			}
			if !ready {
				i++
				continue
			}
			if r.Disposition == Imported {
				if r.Entity == "user" || r.Entity == "assignment" {
					ids[r.RowToken] = int64(len(ids) + 1)
				} else {
					d, err := bindDomain(r, ids, keys)
					if err != nil {
						reject(&r, ReasonInvalidPayload)
					} else {
						keys[r.RowToken] = d.ID
					}
				}
			}
			states[r.RowToken] = r.Disposition
			p.Rows = append(p.Rows, r)
			remaining = append(remaining[:i], remaining[i+1:]...)
			progressed = true
		}
		if !progressed {
			for i := range remaining {
				reject(&remaining[i], ReasonMissingReference)
				states[remaining[i].RowToken] = Rejected
			}
			p.Rows = append(p.Rows, remaining...)
			break
		}
	}
	reasons := map[string]int{}
	for _, r := range p.Rows {
		p.Counts[string(r.Disposition)]++
		reasons[r.Reason]++
	}
	if !sameCounts(p.Counts, m.Expected.Counts) || !sameCounts(reasons, m.Expected.Reasons) {
		return nil, errors.New("exact expected counts/reasons mismatch (unspecified categories mean zero)")
	}
	p.Digest = digestWithoutPlanDigest(p)
	if err := VerifyPlan(p); err != nil {
		return nil, err
	}
	return p, nil
}
func lookupReference(ref, expected string, refs map[string]PlanRow) (PlanRow, bool) {
	r, ok := refs[ref]
	return r, validQualified(ref) && ok && r.Entity == expected && r.Disposition == Imported
}
func validUser(d *DesiredRecord) bool {
	return d != nil && uuidPattern.MatchString(d.AuthUserID) && strings.HasSuffix(d.Email, ".test") && strings.Contains(d.Email, "@") && d.Email == strings.ToLower(strings.TrimSpace(d.Email)) && len(d.Email) <= 254 && strings.TrimSpace(d.Name) != "" && len(d.Name) <= 200 && d.IsActive != nil && d.Hidden != nil && d.CreatedAt != nil
}
func validAssignment(d *DesiredRecord) bool {
	if d == nil || d.UserToken == "" || d.IsActive == nil || d.CreatedAt == nil || !jsonObject(d.Scope) {
		return false
	}
	// Unrestricted roles only in this deliberately narrow synthetic contract.
	if !jsonEqual(d.Scope, []byte(`{}`)) || d.DivisionCode != nil {
		return false
	}
	return d.RoleCode == "koordinator" || d.RoleCode == "mahasiswa" || d.RoleCode == "peminjam"
}
func transformRow(m *Manifest, r SourceRow, e PlanRow, s entitySpec, refs map[string]PlanRow) (*DesiredRecord, string) {
	active, hidden := true, false
	switch s.kind {
	case "user":
		var p userPayload
		if decodeStrict(r.Payload, &p) != nil {
			return nil, ReasonInvalidPayload
		}
		if p.IsActive == nil {
			p.IsActive = &active
		}
		if p.Hidden == nil {
			p.Hidden = &hidden
		}
		d := &DesiredRecord{AuthUserID: p.AuthUserID, NIM: p.NIM, Email: p.Email, Name: p.Name, IsActive: p.IsActive, Hidden: p.Hidden, CreatedAt: &m.Cutoff}
		if !validUser(d) {
			return nil, ReasonInvalidPayload
		}
		return d, ""
	case "assignment":
		var p assignmentPayload
		if decodeStrict(r.Payload, &p) != nil {
			return nil, ReasonInvalidPayload
		}
		u, ok := lookupReference(p.UserRef, "user", refs)
		if !ok {
			return nil, ReasonMissingReference
		}
		if p.IsActive == nil {
			p.IsActive = &active
		}
		d := &DesiredRecord{UserToken: u.RowToken, RoleCode: p.RoleCode, DivisionCode: p.DivisionCode, Scope: canonicalRaw(p.Scope), IsActive: p.IsActive, CreatedAt: &m.Cutoff}
		if !validAssignment(d) {
			return nil, ReasonInvalidPayload
		}
		return d, ""
	case "generic":
		var p genericPayload
		if decodeStrict(r.Payload, &p) != nil || !jsonObject(p.Data) || !jsonObject(p.Scope) {
			return nil, ReasonInvalidPayload
		}
		d := &DesiredRecord{ID: e.TargetID, Scope: canonicalRaw(p.Scope), Status: p.Status, UniqueKey: p.UniqueKey, ExpiresAt: p.ExpiresAt, Data: canonicalRaw(p.Data)}
		expected := parentEntity(r.Entity)
		if expected != "" {
			parent, ok := lookupReference(p.ParentRef, expected, refs)
			if !ok {
				return nil, ReasonMissingReference
			}
			d.ParentToken = parent.RowToken
		} else if p.ParentRef != "" {
			return nil, ReasonInvalidPayload
		}
		if ownerRequired(r.Entity) {
			owner, ok := lookupReference(p.OwnerRef, "user", refs)
			if !ok {
				return nil, ReasonMissingReference
			}
			d.OwnerToken = owner.RowToken
		} else if p.OwnerRef != "" {
			return nil, ReasonInvalidPayload
		}
		convert := func(token, expected string) (any, error) {
			v, ok := lookupReference(token, expected, refs)
			if !ok {
				return nil, errors.New("reference missing or wrong entity")
			}
			return map[string]any{"$token": v.RowToken}, nil
		}
		var err error
		d.Data, err = rewriteReferences(d.Data, r.Entity, "data", "$ref", convert)
		if err != nil {
			return nil, ReasonMissingReference
		}
		d.Scope, err = rewriteReferences(d.Scope, r.Entity, "scope", "$ref", convert)
		if err != nil {
			return nil, ReasonMissingReference
		}
		return d, ""
	}
	return nil, ReasonInvalidEntity
}
func sameCounts(a, b map[string]int) bool {
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}
func VerifyAccounting(rows []PlanRow, total int) error {
	if len(rows) != total {
		return errors.New("accounting row count mismatch")
	}
	for _, r := range rows {
		if !validDisposition(r.Disposition) {
			return errors.New("invalid disposition")
		}
	}
	return nil
}
func validDisposition(d Disposition) bool {
	return d == Imported || d == Unchanged || d == Rejected || d == IntentionallySkipped
}
func validReason(r string) bool {
	return r != "" && len(r) <= 128 && !strings.ContainsAny(r, " \t\r\n")
}
func jsonObject(raw json.RawMessage) bool {
	v, e := ParseCanonicalJSON(raw)
	return e == nil && len(v) > 0 && v[0] == '{'
}
func canonicalRaw(raw json.RawMessage) json.RawMessage { v, _ := ParseCanonicalJSON(raw); return v }
func digestWithoutPlanDigest(p *Plan) string {
	c := *p
	c.Digest = ""
	b, e := CanonicalJSON(c)
	if e != nil {
		return ""
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func VerifyPlan(p *Plan) error {
	if p == nil {
		return errors.New("plan is required")
	}
	if err := validateManifestHeader(&Manifest{FormatVersion: p.FormatVersion, BatchID: p.BatchID, SourceNamespace: p.SourceNamespace, SourceKind: p.SourceKind, Cutoff: p.Cutoff, PolicyVersion: p.PolicyVersion, TargetMigrations: p.TargetMigrations}); err != nil {
		return err
	}
	if !sha256Pattern.MatchString(p.Digest) || p.Digest != digestWithoutPlanDigest(p) {
		return errors.New("plan digest mismatch")
	}
	if len(p.Rows) > 100000 {
		return errors.New("too many plan rows")
	}
	tokens := map[string]PlanRow{}
	counts := map[string]int{}
	auths := map[string]bool{}
	ids := map[string]int64{}
	keys := map[string]string{}
	for _, r := range p.Rows {
		if !componentPattern.MatchString(r.RowToken) || !componentPattern.MatchString(r.Entity) {
			return errors.New("invalid row token")
		}
		if _, ok := tokens[r.RowToken]; ok {
			return errors.New("duplicate row token")
		}
		s, ok := entitySpecs[r.Entity]
		if (ok && s.table != r.Table) || (!ok && r.Table != "") {
			return errors.New("entity/table mismatch")
		}
		counts[string(r.Disposition)]++
		if r.Disposition == Rejected {
			if (!ok && r.Reason != ReasonInvalidEntity) || (ok && s.skip && r.Reason != ReasonEphemeralSkipped) || (ok && s.kind == "unsupported" && r.Reason != ReasonUnsupported) {
				return errors.New("rejection policy/entity mismatch")
			}
			if r.Desired != nil || r.TargetID != "" || !validRejection(r.Reason) {
				return errors.New("invalid rejected row")
			}
			tokens[r.RowToken] = r
			continue
		}
		if r.Disposition != Imported || !ok || s.skip || s.kind == "unsupported" || r.Reason != ReasonPlanned || r.Desired == nil || !componentPattern.MatchString(r.TargetID) || !strings.HasPrefix(r.TargetID, s.prefix) {
			return errors.New("invalid imported row")
		}
		deps, err := dependencies(r)
		if err != nil {
			return err
		}
		for token, expected := range deps {
			if token == r.RowToken {
				if expected != r.Entity {
					return errors.New("wrong self reference type")
				}
				continue
			}
			prior, ok := tokens[token]
			if !ok || prior.Disposition != Imported || prior.Entity != expected {
				return errors.New("dependency missing, wrong type, rejected, or out of order")
			}
		}
		switch r.Entity {
		case "user":
			if !validUser(r.Desired) || !r.Desired.CreatedAt.Equal(p.Cutoff) || auths[r.Desired.AuthUserID] {
				return errors.New("invalid or duplicate target auth identity")
			}
			auths[r.Desired.AuthUserID] = true
			ids[r.RowToken] = int64(len(ids) + 1)
		case "assignment":
			if !validAssignment(r.Desired) || !r.Desired.CreatedAt.Equal(p.Cutoff) {
				return errors.New("invalid assignment")
			}
			ids[r.RowToken] = int64(len(ids) + 1)
		default:
			if r.Desired.ID != r.TargetID {
				return errors.New("target ID mismatch")
			}
			d, err := bindDomain(r, ids, keys)
			if err != nil {
				return fmt.Errorf("invalid %s typed data: %w", r.Entity, err)
			}
			keys[r.RowToken] = d.ID
		}
		tokens[r.RowToken] = r
	}
	for k, v := range p.Counts {
		if !validDisposition(Disposition(k)) || v < 0 {
			return errors.New("invalid count category")
		}
	}
	if !sameCounts(counts, p.Counts) {
		return errors.New("plan counts mismatch")
	}
	return nil
}
func validRejection(reason string) bool {
	return reason == ReasonInvalidEntity || reason == ReasonInvalidPayload || reason == ReasonMissingReference || reason == ReasonEphemeralSkipped || reason == ReasonUnsupported
}
func WriteJSONAtomic(path string, value any, overwrite bool) error {
	body, err := CanonicalJSON(value)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rehearsal-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(body)
	}
	if ce := tmp.Close(); err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if !overwrite {
		if _, err = os.Lstat(path); err == nil {
			return errors.New("refusing to overwrite existing file")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Link(tmp.Name(), path)
	}
	return os.Rename(tmp.Name(), path)
}
func LoadPlan(path string) (*Plan, error) {
	b, e := readBoundedFile(path, MaxPlanBytes)
	if e != nil {
		return nil, e
	}
	var p Plan
	if e = decodeStrict(b, &p); e != nil {
		return nil, e
	}
	if e = VerifyPlan(&p); e != nil {
		return nil, e
	}
	return &p, nil
}

// Canonical JSON rejects duplicate keys, excessive nesting, and numeric expansion
// attacks. Exact finite decimals are normalized without floating-point rounding.
func CanonicalJSON(v any) ([]byte, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	return ParseCanonicalJSON(b)
}
func ParseCanonicalJSON(raw []byte) ([]byte, error) {
	if len(raw) > MaxPlanBytes {
		return nil, errors.New("JSON exceeds size limit")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, e := parseNode(d, 0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); !errors.Is(e, io.EOF) {
		return nil, errors.New("multiple JSON values")
	}
	var b bytes.Buffer
	if e = writeNode(&b, v); e != nil {
		return nil, e
	}
	if b.Len() > MaxPlanBytes {
		return nil, errors.New("canonical JSON exceeds size limit")
	}
	return b.Bytes(), nil
}

type objectNode map[string]any
type arrayNode []any

func parseNode(d *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("JSON nesting exceeds limit")
	}
	t, e := d.Token()
	if e != nil {
		return nil, e
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			o := objectNode{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok {
					return nil, errors.New("invalid object key")
				}
				if _, ok = o[key]; ok {
					return nil, errors.New("duplicate JSON key")
				}
				c, e := parseNode(d, depth+1)
				if e != nil {
					return nil, e
				}
				o[key] = c
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return nil, errors.New("invalid object")
			}
			return o, nil
		case '[':
			a := arrayNode{}
			for d.More() {
				c, e := parseNode(d, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, c)
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return nil, errors.New("invalid array")
			}
			return a, nil
		default:
			return nil, errors.New("unexpected delimiter")
		}
	case json.Number:
		n, e := normalizeNumber(v.String())
		return json.Number(n), e
	case string, bool, nil:
		return v, nil
	default:
		return nil, errors.New("unsupported JSON token")
	}
}
func writeNode(b *bytes.Buffer, n any) error {
	if b.Len() > MaxPlanBytes {
		return errors.New("canonical JSON exceeds size limit")
	}
	switch v := n.(type) {
	case objectNode:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			s, _ := json.Marshal(k)
			b.Write(s)
			b.WriteByte(':')
			if e := writeNode(b, v[k]); e != nil {
				return e
			}
		}
		b.WriteByte('}')
	case arrayNode:
		b.WriteByte('[')
		for i, c := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if e := writeNode(b, c); e != nil {
				return e
			}
		}
		b.WriteByte(']')
	case json.Number:
		b.WriteString(v.String())
	case string:
		s, _ := json.Marshal(v)
		b.Write(s)
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case nil:
		b.WriteString("null")
	default:
		return errors.New("unsupported canonical node")
	}
	return nil
}
func normalizeNumber(in string) (string, error) {
	if len(in) > 1024 {
		return "", errors.New("JSON number too long")
	}
	f, e := strconv.ParseFloat(in, 64)
	if e != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return "", errors.New("JSON number must be finite and bounded")
	}
	negative := strings.HasPrefix(in, "-")
	if negative {
		in = in[1:]
	}
	mantissa, exp := in, 0
	if i := strings.IndexAny(in, "eE"); i >= 0 {
		mantissa = in[:i]
		exp, e = strconv.Atoi(in[i+1:])
		if e != nil || exp > 308 || exp < -308 {
			return "", errors.New("JSON exponent exceeds limit")
		}
	}
	fraction := 0
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		fraction = len(mantissa) - i - 1
		mantissa = mantissa[:i] + mantissa[i+1:]
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return "0", nil
	}
	shift := exp - fraction
	var result string
	if shift >= 0 {
		result = digits + strings.Repeat("0", shift)
	} else {
		scale := -shift
		if scale >= len(digits) {
			result = "0." + strings.Repeat("0", scale-len(digits)) + digits
		} else {
			result = digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
		}
		result = strings.TrimRight(strings.TrimRight(result, "0"), ".")
	}
	if len(result) > 1024 {
		return "", errors.New("JSON number expansion exceeds limit")
	}
	if negative {
		result = "-" + result
	}
	return result, nil
}
