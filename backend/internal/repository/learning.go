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

var ErrLearningNotFound = errors.New("learning record not found")

type LearningKind string

const (
	LearningLevels     LearningKind = "levels"
	LearningModules    LearningKind = "modules"
	LearningLessons    LearningKind = "lessons"
	LearningProgress   LearningKind = "progress"
	LearningProfiles   LearningKind = "profiles"
	LearningBugHunt    LearningKind = "bug_hunt"
	LearningGames      LearningKind = "game_history"
	LearningOperations LearningKind = "operations"
	LearningRuns       LearningKind = "runner_leases"
)

type LearningRecord struct {
	ID        string
	ParentID  string
	OwnerID   int64
	Scope     entity.Scope
	Status    string
	UniqueKey string
	ExpiresAt *time.Time
	Data      json.RawMessage
}

type LearningFilter struct {
	ParentID  string
	OwnerID   int64
	Status    string
	UniqueKey string
	AfterID   string
	SortOrder int
	Limit     int
	Offset    int
	Scopes    []entity.Scope
	Overlap   bool
	DueBefore *time.Time
}

type LearningTx interface {
	Get(LearningKind, string) (LearningRecord, error)
	Lock(LearningKind, string) (LearningRecord, error)
	List(LearningKind, LearningFilter) ([]LearningRecord, error)
	Put(LearningKind, LearningRecord) error
	Now() (time.Time, error)
	Audit(entity.AuditEvent) error
}

type LearningRepository interface {
	Transaction(context.Context, func(LearningTx) error) error
}

type MemoryLearningRepository struct {
	mu       sync.Mutex
	records  map[LearningKind]map[string]LearningRecord
	audit    []entity.AuditEvent
	clock    func() time.Time
	identity *MemoryIdentityRepository
}

func NewMemoryLearningRepository(clock ...func() time.Time) *MemoryLearningRepository {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &MemoryLearningRepository{records: map[LearningKind]map[string]LearningRecord{}, clock: now}
}

func (r *MemoryLearningRepository) BindIdentity(identity *MemoryIdentityRepository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identity = identity
}

type memoryLearningTx struct {
	ctx     context.Context
	records map[LearningKind]map[string]LearningRecord
	audit   []entity.AuditEvent
	clock   func() time.Time
}

func (r *MemoryLearningRepository) Transaction(ctx context.Context, fn func(LearningTx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &memoryLearningTx{ctx: ctx, records: map[LearningKind]map[string]LearningRecord{}, clock: r.clock}
	for kind, rows := range r.records {
		tx.records[kind] = map[string]LearningRecord{}
		for id, row := range rows {
			tx.records[kind][id] = cloneLearningRecord(row)
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
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

func cloneLearningRecord(r LearningRecord) LearningRecord {
	r.Data = append(json.RawMessage(nil), r.Data...)
	if r.ExpiresAt != nil {
		v := *r.ExpiresAt
		r.ExpiresAt = &v
	}
	return r
}

func (tx *memoryLearningTx) Get(kind LearningKind, id string) (LearningRecord, error) {
	r, ok := tx.records[kind][id]
	if !ok {
		return r, ErrLearningNotFound
	}
	return cloneLearningRecord(r), tx.ctx.Err()
}

func (tx *memoryLearningTx) Lock(kind LearningKind, id string) (LearningRecord, error) {
	return tx.Get(kind, id)
}

func learningLimit(n int) int {
	if n < 1 {
		return 25
	}
	if n > 1000 {
		return 1000
	}
	return n
}

func learningMatches(r LearningRecord, f LearningFilter) bool {
	if f.ParentID != "" && r.ParentID != f.ParentID || f.OwnerID != 0 && r.OwnerID != f.OwnerID || f.Status != "" && r.Status != f.Status || f.UniqueKey != "" && r.UniqueKey != f.UniqueKey || r.ID <= f.AfterID {
		return false
	}
	if f.SortOrder != 0 {
		var ordered struct {
			SortOrder int `json:"sort_order"`
		}
		if json.Unmarshal(r.Data, &ordered) != nil || ordered.SortOrder != f.SortOrder {
			return false
		}
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
	return true
}

func (tx *memoryLearningTx) List(kind LearningKind, f LearningFilter) ([]LearningRecord, error) {
	out := []LearningRecord{}
	for _, r := range tx.records[kind] {
		if learningMatches(r, f) {
			out = append(out, cloneLearningRecord(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	offset := max(0, f.Offset)
	if offset >= len(out) {
		return []LearningRecord{}, nil
	}
	return out[offset:min(len(out), offset+learningLimit(f.Limit))], tx.ctx.Err()
}

func (tx *memoryLearningTx) Put(kind LearningKind, r LearningRecord) error {
	if err := tx.ctx.Err(); err != nil {
		return err
	}
	if !validLearningKind(kind) || r.ID == "" || !json.Valid(r.Data) {
		return ErrConflict
	}
	if tx.records[kind] == nil {
		tx.records[kind] = map[string]LearningRecord{}
	}
	for id, v := range tx.records[kind] {
		if id != r.ID && r.UniqueKey != "" && v.UniqueKey == r.UniqueKey {
			return ErrConflict
		}
		if id != r.ID && kind == LearningProgress && v.ParentID == r.ParentID && v.OwnerID == r.OwnerID {
			return ErrConflict
		}
		if id != r.ID && kind == LearningProfiles && v.OwnerID == r.OwnerID {
			return ErrConflict
		}
	}
	if existing, ok := tx.records[kind][r.ID]; ok {
		if kind == LearningGames {
			return ErrConflict
		}
		if kind == LearningProgress {
			if existing.ParentID != r.ParentID || existing.OwnerID != r.OwnerID || (existing.Status == "completed" && r.Status != "completed") {
				return ErrConflict
			}
		}
		if kind == LearningProfiles && existing.OwnerID != r.OwnerID {
			return ErrConflict
		}
	}
	if err := validateLearningRecord(kind, r, func(k LearningKind, id string) bool { _, ok := tx.records[k][id]; return ok }); err != nil {
		return err
	}
	tx.records[kind][r.ID] = cloneLearningRecord(r)
	return nil
}

func (tx *memoryLearningTx) Now() (time.Time, error) { return tx.clock().UTC(), tx.ctx.Err() }
func (tx *memoryLearningTx) Audit(e entity.AuditEvent) error {
	tx.audit = append(tx.audit, cloneAudit(e))
	return tx.ctx.Err()
}

func validLearningKind(k LearningKind) bool {
	switch k {
	case LearningLevels, LearningModules, LearningLessons, LearningProgress, LearningProfiles, LearningBugHunt, LearningGames, LearningOperations, LearningRuns:
		return true
	}
	return false
}

func validateLearningRecord(k LearningKind, r LearningRecord, exists func(LearningKind, string) bool) error {
	parent := LearningKind("")
	switch k {
	case LearningModules:
		parent = LearningLevels
	case LearningLessons:
		parent = LearningModules
	case LearningProgress:
		parent = LearningLessons
	case LearningGames:
		parent = LearningBugHunt
	}
	if parent != "" && !exists(parent, r.ParentID) {
		return ErrConflict
	}
	switch k {
	case LearningProgress:
		if r.Status != "in_progress" && r.Status != "completed" {
			return ErrConflict
		}
	case LearningProfiles:
		if r.Status != "active" {
			return ErrConflict
		}
	case LearningBugHunt:
		if r.Status != "active" && r.Status != "inactive" {
			return ErrConflict
		}
	case LearningGames:
		if r.Status != "completed" {
			return ErrConflict
		}
	case LearningRuns:
		if r.Status != "running" && r.Status != "finished" {
			return ErrConflict
		}
	}
	return nil
}
