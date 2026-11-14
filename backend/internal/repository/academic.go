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

var ErrAcademicNotFound = errors.New("academic record not found")

type AcademicKind string

const (
	AcademicClasses            AcademicKind = "classes"
	AcademicSchedules          AcademicKind = "schedules"
	AcademicAttendanceSessions AcademicKind = "attendance_sessions"
	AcademicAttendanceRecords  AcademicKind = "attendance_records"
	AcademicInventoryItems     AcademicKind = "inventory_items"
	AcademicInventoryRentals   AcademicKind = "inventory_rentals"
	AcademicNotifications      AcademicKind = "notifications"
	AcademicOperations         AcademicKind = "operations"
)

type AcademicRecord struct {
	ID        string
	ParentID  string
	OwnerID   int64
	Scope     entity.Scope
	Status    string
	UniqueKey string
	ExpiresAt *time.Time
	Data      json.RawMessage
}

type AcademicFilter struct {
	ParentID  string
	OwnerID   int64
	Status    string
	UniqueKey string
	AfterID   string
	Limit     int
	Offset    int
	Scopes    []entity.Scope
	Overlap   bool
	MemberID  int64
	DueBefore *time.Time
}

type AcademicTx interface {
	Get(AcademicKind, string) (AcademicRecord, error)
	Lock(AcademicKind, string) (AcademicRecord, error)
	List(AcademicKind, AcademicFilter) ([]AcademicRecord, error)
	Put(AcademicKind, AcademicRecord) error
	Now() (time.Time, error)
	Audit(entity.AuditEvent) error
}

type AcademicRepository interface {
	Transaction(context.Context, func(AcademicTx) error) error
}

type MemoryAcademicRepository struct {
	mu       sync.Mutex
	records  map[AcademicKind]map[string]AcademicRecord
	audit    []entity.AuditEvent
	clock    func() time.Time
	identity *MemoryIdentityRepository
}

func NewMemoryAcademicRepository(clock ...func() time.Time) *MemoryAcademicRepository {
	now := time.Now
	if len(clock) > 0 && clock[0] != nil {
		now = clock[0]
	}
	return &MemoryAcademicRepository{records: map[AcademicKind]map[string]AcademicRecord{}, clock: now}
}

func (r *MemoryAcademicRepository) BindIdentity(identity *MemoryIdentityRepository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identity = identity
}

type memoryAcademicTx struct {
	ctx     context.Context
	records map[AcademicKind]map[string]AcademicRecord
	audit   []entity.AuditEvent
	clock   func() time.Time
}

func (r *MemoryAcademicRepository) Transaction(ctx context.Context, fn func(AcademicTx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &memoryAcademicTx{ctx: ctx, records: map[AcademicKind]map[string]AcademicRecord{}, clock: r.clock}
	for kind, rows := range r.records {
		tx.records[kind] = map[string]AcademicRecord{}
		for id, row := range rows {
			tx.records[kind][id] = cloneAcademicRecord(row)
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

func cloneAcademicRecord(r AcademicRecord) AcademicRecord {
	r.Data = append(json.RawMessage(nil), r.Data...)
	if r.ExpiresAt != nil {
		v := *r.ExpiresAt
		r.ExpiresAt = &v
	}
	return r
}

func (tx *memoryAcademicTx) Get(kind AcademicKind, id string) (AcademicRecord, error) {
	r, ok := tx.records[kind][id]
	if !ok {
		return r, ErrAcademicNotFound
	}
	return cloneAcademicRecord(r), tx.ctx.Err()
}

func (tx *memoryAcademicTx) Lock(kind AcademicKind, id string) (AcademicRecord, error) {
	return tx.Get(kind, id)
}

func academicMatches(r AcademicRecord, f AcademicFilter) bool {
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
	return true
}

func academicMember(r AcademicRecord, memberID int64) bool {
	var c entity.AcademicClass
	if json.Unmarshal(r.Data, &c) != nil || !c.Active {
		return false
	}
	for _, ids := range [][]int64{c.StudentIDs, c.AssistantIDs} {
		for _, id := range ids {
			if id == memberID {
				return true
			}
		}
	}
	return false
}

func (tx *memoryAcademicTx) List(kind AcademicKind, f AcademicFilter) ([]AcademicRecord, error) {
	out := []AcademicRecord{}
	for _, r := range tx.records[kind] {
		if f.MemberID != 0 {
			cls := r
			if kind == AcademicSchedules {
				cls = tx.records[AcademicClasses][r.ParentID]
			}
			if !academicMember(cls, f.MemberID) {
				continue
			}
		}
		if academicMatches(r, f) {
			out = append(out, cloneAcademicRecord(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	offset := max(0, f.Offset)
	if offset >= len(out) {
		return []AcademicRecord{}, nil
	}
	return out[offset:min(len(out), offset+learningLimit(f.Limit))], tx.ctx.Err()
}

func (tx *memoryAcademicTx) Put(kind AcademicKind, r AcademicRecord) error {
	if err := tx.ctx.Err(); err != nil {
		return err
	}
	if !validAcademicKind(kind) || r.ID == "" || !json.Valid(r.Data) {
		return ErrConflict
	}
	if tx.records[kind] == nil {
		tx.records[kind] = map[string]AcademicRecord{}
	}
	for id, v := range tx.records[kind] {
		if id != r.ID && r.UniqueKey != "" && v.UniqueKey == r.UniqueKey {
			return ErrConflict
		}
		if id != r.ID && kind == AcademicAttendanceRecords && v.ParentID == r.ParentID && v.OwnerID == r.OwnerID {
			return ErrConflict
		}
	}
	if existing, ok := tx.records[kind][r.ID]; ok {
		if kind == AcademicAttendanceSessions && existing.Status == "closed" && r.Status != "closed" {
			return ErrConflict
		}
		if kind == AcademicInventoryRentals {
			if (existing.Status == "rejected" && r.Status != "rejected") ||
				(existing.Status == "returned" && r.Status != "returned") ||
				(existing.Status == "approved" && (r.Status == "requested" || r.Status == "rejected")) {
				return ErrConflict
			}
		}
	}
	if err := validateAcademicRecord(kind, r, func(k AcademicKind, id string) bool { _, ok := tx.records[k][id]; return ok }); err != nil {
		return err
	}
	tx.records[kind][r.ID] = cloneAcademicRecord(r)
	return nil
}

func (tx *memoryAcademicTx) Now() (time.Time, error) { return tx.clock().UTC(), tx.ctx.Err() }
func (tx *memoryAcademicTx) Audit(e entity.AuditEvent) error {
	tx.audit = append(tx.audit, cloneAudit(e))
	return tx.ctx.Err()
}

func validAcademicKind(k AcademicKind) bool {
	switch k {
	case AcademicClasses, AcademicSchedules, AcademicAttendanceSessions, AcademicAttendanceRecords, AcademicInventoryItems, AcademicInventoryRentals, AcademicNotifications, AcademicOperations:
		return true
	}
	return false
}

func validateAcademicRecord(k AcademicKind, r AcademicRecord, exists func(AcademicKind, string) bool) error {
	parent := AcademicKind("")
	switch k {
	case AcademicSchedules:
		parent = AcademicClasses
	case AcademicAttendanceSessions:
		parent = AcademicSchedules
	case AcademicAttendanceRecords:
		parent = AcademicAttendanceSessions
	case AcademicInventoryRentals:
		parent = AcademicInventoryItems
	}
	if parent != "" && !exists(parent, r.ParentID) {
		return ErrConflict
	}
	switch k {
	case AcademicAttendanceSessions:
		if r.Status != "open" && r.Status != "closed" {
			return ErrConflict
		}
	case AcademicAttendanceRecords:
		if r.Status != "hadir" && r.Status != "izin" && r.Status != "alpha" && r.Status != "terlambat" {
			return ErrConflict
		}
	case AcademicInventoryItems:
		if r.Status != "available" && r.Status != "maintenance" && r.Status != "retired" {
			return ErrConflict
		}
	case AcademicInventoryRentals:
		if r.Status != "requested" && r.Status != "approved" && r.Status != "rejected" && r.Status != "returned" {
			return ErrConflict
		}
	case AcademicNotifications:
		if r.Status != "unread" && r.Status != "read" {
			return ErrConflict
		}
	}
	return nil
}
