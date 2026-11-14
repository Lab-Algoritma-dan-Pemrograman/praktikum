package main

import (
	"encoding/json"
	"time"
)

type Plan struct {
	FormatVersion string            `json:"format_version"`
	BatchID       string            `json:"batch_id"`
	Cutoff        time.Time         `json:"cutoff"`
	Source        map[string]any    `json:"source"`
	Policy        map[string]string `json:"policy"`
	AuthUsers     []AuthUser        `json:"auth_users"`
	Users         []User            `json:"users"`
	Assignments   []Assignment      `json:"assignments"`
	DomainRows    []DomainRow       `json:"domain_rows"`
	Dispositions  []Disposition     `json:"dispositions"`
	Counts        map[string]int    `json:"counts"`
	ReasonCounts  map[string]int    `json:"reason_counts"`
	Digest        string            `json:"digest"`
}
type AuthUser struct {
	Token           string    `json:"token"`
	AuthUserID      string    `json:"auth_user_id"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"created_at"`
	AcceptanceLogin bool      `json:"acceptance_login"`
}
type User struct {
	Token      string    `json:"token"`
	AuthUserID string    `json:"auth_user_id"`
	NIM        *string   `json:"nim"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	IsActive   bool      `json:"is_active"`
	Hidden     bool      `json:"hidden"`
	CreatedAt  time.Time `json:"created_at"`
}
type Assignment struct {
	Token        string         `json:"token"`
	UserToken    string         `json:"user_token"`
	RoleCode     string         `json:"role_code"`
	DivisionCode *string        `json:"division_code"`
	Scope        map[string]any `json:"scope"`
	IsActive     bool           `json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
}
type DomainRow struct {
	Token      string         `json:"token"`
	Table      string         `json:"table"`
	ID         string         `json:"id"`
	IDTemplate string         `json:"id_template"`
	ParentID   string         `json:"parent_id"`
	OwnerToken string         `json:"owner_token"`
	Scope      map[string]any `json:"scope"`
	Status     string         `json:"status"`
	UniqueKey  string         `json:"unique_key"`
	ExpiresAt  *time.Time     `json:"expires_at"`
	Data       map[string]any `json:"data"`
}
type Disposition struct {
	SourceSystem string `json:"source_system"`
	Entity       string `json:"entity"`
	SourceRef    string `json:"source_ref"`
	Disposition  string `json:"disposition"`
	Reason       string `json:"reason"`
	TargetToken  string `json:"target_token"`
	Rows         int    `json:"rows"`
}
type Receipt struct {
	FormatVersion     string         `json:"format_version"`
	BatchID           string         `json:"batch_id"`
	PlanFileSHA256    string         `json:"plan_file_sha256"`
	TargetFingerprint string         `json:"target_fingerprint"`
	Rows              []ReceiptRow   `json:"rows"`
	Counts            map[string]int `json:"counts"`
	StartedAt         time.Time      `json:"started_at"`
	FinishedAt        time.Time      `json:"finished_at"`
	DurationMS        int64          `json:"duration_ms"`
	Digest            string         `json:"digest"`
}
type ReceiptRow struct {
	Token  string `json:"token"`
	Kind   string `json:"kind"`
	Table  string `json:"table"`
	Key    string `json:"key"`
	Action string `json:"action"`
	Digest string `json:"digest"`
}
type ReconcileReport struct {
	FormatVersion     string         `json:"format_version"`
	BatchID           string         `json:"batch_id"`
	PlanFileSHA256    string         `json:"plan_file_sha256"`
	TargetFingerprint string         `json:"target_fingerprint"`
	Checks            []Check        `json:"checks"`
	Counts            map[string]int `json:"counts"`
	Digest            string         `json:"digest"`
}
type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Observed int    `json:"observed,omitempty"`
	Expected int    `json:"expected,omitempty"`
}

var allowedTables = map[string]bool{"academic_classes": true, "academic_schedules": true, "academic_inventory_items": true, "learning_levels": true, "learning_modules": true, "learning_lessons": true, "learning_progress": true, "learning_profiles": true, "practicum_sessions": true, "practicum_courses": true, "practicum_questions": true, "practicum_activations": true, "practicum_attempts": true, "practicum_grading_jobs": true}

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
