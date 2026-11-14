package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

var ErrPracticumNotFound = errors.New("practicum record not found")

type PracticumKind string

const (
	PracticumSessions    PracticumKind = "sessions"
	PracticumCourses     PracticumKind = "courses"
	PracticumQuestions   PracticumKind = "questions"
	PracticumActivations PracticumKind = "activations"
	PracticumAttempts    PracticumKind = "attempts"
	PracticumJobs        PracticumKind = "grading_jobs"
	PracticumOperations  PracticumKind = "operations"
	PracticumRuns        PracticumKind = "runner_leases"
)

// Record metadata is indexed separately from its typed JSON payload. JSON is always
// sent as text to pgx (including when QueryExecModeExec is used).
type PracticumRecord struct {
	ID        string
	ParentID  string
	OwnerID   int64
	Scope     entity.Scope
	Status    string
	UniqueKey string
	ExpiresAt *time.Time
	Data      json.RawMessage
}
type PracticumFilter struct {
	ParentID  string
	OwnerID   int64
	Status    string
	UniqueKey string
	AfterID   string
	Limit     int
	Offset    int
	Scopes    []entity.Scope // nil means internal unrestricted; empty means deny all.
	Overlap   bool
	MemberID  int64
	DueBefore *time.Time
}
type PracticumTx interface {
	Get(PracticumKind, string) (PracticumRecord, error)
	Lock(PracticumKind, string) (PracticumRecord, error)
	List(PracticumKind, PracticumFilter) ([]PracticumRecord, error)
	Put(PracticumKind, PracticumRecord) error
	Now() (time.Time, error)
	Audit(entity.AuditEvent) error
}
type PracticumRepository interface {
	Transaction(context.Context, func(PracticumTx) error) error
}

type MemoryPracticumRepository struct {
	mu       sync.Mutex
	records  map[PracticumKind]map[string]PracticumRecord
	audit    []entity.AuditEvent
	clock    func() time.Time
	identity *MemoryIdentityRepository
}

func NewMemoryPracticumRepository(clock ...func() time.Time) *MemoryPracticumRepository {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &MemoryPracticumRepository{records: map[PracticumKind]map[string]PracticumRecord{}, clock: now}
}

// BindIdentity shares local audit storage atomically. Call during application construction.
func (r *MemoryPracticumRepository) BindIdentity(identity *MemoryIdentityRepository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identity = identity
}

type memoryPracticumTx struct {
	ctx     context.Context
	records map[PracticumKind]map[string]PracticumRecord
	audit   []entity.AuditEvent
	clock   func() time.Time
}

func (r *MemoryPracticumRepository) Transaction(ctx context.Context, fn func(PracticumTx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &memoryPracticumTx{ctx: ctx, records: map[PracticumKind]map[string]PracticumRecord{}, clock: r.clock}
	for kind, rows := range r.records {
		tx.records[kind] = map[string]PracticumRecord{}
		for id, row := range rows {
			tx.records[kind][id] = clonePracticumRecord(row)
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Both stores remain locked through the commit; no post-commit AppendAudit call.
	if r.identity != nil {
		r.identity.mu.Lock()
		defer r.identity.mu.Unlock()
		for _, e := range tx.audit {
			e.ID = r.identity.state.nextAuditID
			r.identity.state.nextAuditID++
			r.identity.state.audit = append(r.identity.state.audit, cloneAudit(e))
		}
	}
	r.records = tx.records
	r.audit = append(r.audit, tx.audit...)
	return nil
}
func clonePracticumRecord(r PracticumRecord) PracticumRecord {
	r.Data = append(json.RawMessage(nil), r.Data...)
	if r.ExpiresAt != nil {
		v := *r.ExpiresAt
		r.ExpiresAt = &v
	}
	return r
}
func (tx *memoryPracticumTx) Get(kind PracticumKind, id string) (PracticumRecord, error) {
	r, ok := tx.records[kind][id]
	if !ok {
		return r, ErrPracticumNotFound
	}
	return clonePracticumRecord(r), tx.ctx.Err()
}
func (tx *memoryPracticumTx) Lock(kind PracticumKind, id string) (PracticumRecord, error) {
	return tx.Get(kind, id)
}
func practicumLimit(n int) int {
	if n < 1 {
		return 25
	}
	if n > 1000 {
		return 1000
	}
	return n
}
func practicumMatches(r PracticumRecord, f PracticumFilter) bool {
	if f.ParentID != "" && r.ParentID != f.ParentID || f.OwnerID != 0 && r.OwnerID != f.OwnerID || f.Status != "" && r.Status != f.Status || f.UniqueKey != "" && r.UniqueKey != f.UniqueKey || r.ID <= f.AfterID {
		return false
	}
	if f.DueBefore != nil && (r.ExpiresAt == nil || r.ExpiresAt.After(*f.DueBefore)) {
		return false
	}
	if f.Scopes != nil {
		ok := false
		for _, s := range f.Scopes {
			if s.Contains(r.Scope) {
				ok = true
			}
			if f.Overlap {
				if _, intersects := s.Intersect(r.Scope); intersects {
					ok = true
				}
			}
		}
		if !ok {
			return false
		}
	}
	if f.MemberID != 0 {
		var a entity.PracticumActivationRecord
		if json.Unmarshal(r.Data, &a) != nil {
			return false
		}
		ok := false
		for _, id := range a.Activation.StudentIDs {
			if id == f.MemberID {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func (tx *memoryPracticumTx) List(kind PracticumKind, f PracticumFilter) ([]PracticumRecord, error) {
	out := []PracticumRecord{}
	for _, r := range tx.records[kind] {
		if practicumMatches(r, f) {
			out = append(out, clonePracticumRecord(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	offset := max(0, f.Offset)
	if offset >= len(out) {
		return []PracticumRecord{}, nil
	}
	return out[offset:min(len(out), offset+practicumLimit(f.Limit))], tx.ctx.Err()
}
func (tx *memoryPracticumTx) Put(kind PracticumKind, r PracticumRecord) error {
	if err := tx.ctx.Err(); err != nil {
		return err
	}
	if !validPracticumKind(kind) || r.ID == "" || !json.Valid(r.Data) {
		return ErrConflict
	}
	if tx.records[kind] == nil {
		tx.records[kind] = map[string]PracticumRecord{}
	}
	for id, v := range tx.records[kind] {
		if id != r.ID && r.UniqueKey != "" && v.UniqueKey == r.UniqueKey {
			return ErrConflict
		}
	}
	if err := validatePracticumRecord(kind, r, func(k PracticumKind, id string) bool { _, ok := tx.records[k][id]; return ok }); err != nil {
		return err
	}
	tx.records[kind][r.ID] = clonePracticumRecord(r)
	return nil
}
func (tx *memoryPracticumTx) Now() (time.Time, error) { return tx.clock().UTC(), tx.ctx.Err() }
func (tx *memoryPracticumTx) Audit(e entity.AuditEvent) error {
	tx.audit = append(tx.audit, cloneAudit(e))
	return tx.ctx.Err()
}
func validPracticumKind(k PracticumKind) bool {
	switch k {
	case PracticumSessions, PracticumCourses, PracticumQuestions, PracticumActivations, PracticumAttempts, PracticumJobs, PracticumOperations, PracticumRuns:
		return true
	}
	return false
}
func validatePracticumRecord(k PracticumKind, r PracticumRecord, exists func(PracticumKind, string) bool) error {
	parent := PracticumKind("")
	switch k {
	case PracticumCourses, PracticumActivations:
		parent = PracticumSessions
	case PracticumQuestions:
		parent = PracticumCourses
	case PracticumAttempts:
		parent = PracticumActivations
	case PracticumJobs, PracticumRuns:
		parent = PracticumAttempts
	}
	if parent != "" && !exists(parent, r.ParentID) {
		return ErrConflict
	}
	switch k {
	case PracticumAttempts:
		if r.Status != "in_progress" && r.Status != "submitted" && r.Status != "graded" {
			return ErrConflict
		}
	case PracticumJobs:
		if r.Status != "pending" && r.Status != "running" && r.Status != "needs_manual" && r.Status != "graded" {
			return ErrConflict
		}
	}
	return nil
}
