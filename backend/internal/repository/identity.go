package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

var (
	ErrIdentityNotFound = errors.New("identity not found")
	ErrSessionNotFound  = errors.New("session not found")
	ErrHandshakeInvalid = errors.New("invalid or already-used handshake")
	ErrConflict         = errors.New("identity version or unique constraint conflict")
)

type IdentityRepository interface {
	Transaction(context.Context, func(IdentityRepository) error) error
	EnsureUser(context.Context, entity.AuthIdentity, time.Time) (entity.UserProfile, error)
	ListUsers(context.Context, int, int) ([]entity.UserProfile, error)
	ListPermissionDefinitions(context.Context) ([]entity.PermissionDefinition, error)
	ReadAccess(context.Context, int64) (entity.UserAccess, error)
	ReplaceAccess(context.Context, int64, entity.UserAccess) error
	FindUserByID(context.Context, int64) (entity.UserProfile, error)
	FindUserByAuthID(context.Context, string) (entity.UserProfile, error)
	SaveUser(context.Context, entity.UserProfile) error
	ListAssignments(context.Context, int64) ([]entity.RoleAssignment, error)
	ListPermissionGrants(context.Context, int64) ([]entity.PermissionGrant, error)
	SaveSession(context.Context, entity.Session) error
	FindSessionByID(context.Context, string) (entity.Session, error)
	FindSessionByTokenHash(context.Context, string) (entity.Session, error)
	ListSessions(context.Context, int64, time.Time) ([]entity.Session, error)
	RevokeSession(context.Context, string, time.Time) error
	SaveHandshake(context.Context, entity.SSOHandshake) error
	ConsumeHandshake(context.Context, string, time.Time) (entity.SSOHandshake, error)
	TakeRateLimit(context.Context, string, time.Time, int) (bool, error)
	AppendAudit(context.Context, entity.AuditEvent) error
	ListAudit(context.Context, int) ([]entity.AuditEvent, error)
	ListAuditPage(context.Context, int64, int) ([]entity.AuditEvent, error)
}

type assignmentGrant struct {
	AssignmentID int64
	Grant        entity.PermissionGrant
}

type permissionDefault struct {
	Permission string
	Scope      entity.Scope
}

type rateWindow struct {
	Key   string
	Start time.Time
}

type memoryIdentityState struct {
	users            map[int64]entity.UserProfile
	byAuth           map[string]int64
	assignments      map[int64][]entity.RoleAssignment
	grants           map[int64][]assignmentGrant
	sessions         map[string]entity.Session
	handshakes       map[string]entity.SSOHandshake
	divisions        map[entity.DivisionCode]bool
	definitions      map[string]entity.PermissionDefinition
	roleDefaults     map[entity.RoleCode][]permissionDefault
	divisionDefaults map[entity.DivisionCode][]permissionDefault
	rateLimits       map[rateWindow]int
	audit            []entity.AuditEvent
	nextUserID       int64
	nextAssignmentID int64
	nextAuditID      int64
}

type MemoryIdentityRepository struct {
	mu    sync.RWMutex
	state memoryIdentityState
}

var _ IdentityRepository = (*MemoryIdentityRepository)(nil)

func NewMemoryIdentityRepository() *MemoryIdentityRepository {
	r := &MemoryIdentityRepository{state: memoryIdentityState{
		users: make(map[int64]entity.UserProfile), byAuth: make(map[string]int64),
		assignments: make(map[int64][]entity.RoleAssignment), grants: make(map[int64][]assignmentGrant),
		sessions: make(map[string]entity.Session), handshakes: make(map[string]entity.SSOHandshake),
		divisions: make(map[entity.DivisionCode]bool), definitions: make(map[string]entity.PermissionDefinition),
		roleDefaults: make(map[entity.RoleCode][]permissionDefault), divisionDefaults: make(map[entity.DivisionCode][]permissionDefault),
		rateLimits: make(map[rateWindow]int), nextUserID: 12, nextAssignmentID: 12, nextAuditID: 1,
	}}
	r.seedDefinitions()
	now := time.Now().UTC()
	fixtures := []struct {
		id                int64
		auth, email, name string
		role              entity.RoleCode
		division          entity.DivisionCode
	}{
		{1, "fixture-student", "mahasiswa@example.test", "Mahasiswa Fixture", entity.RoleMahasiswa, ""},
		{2, "fixture-assistant", "asisten@example.test", "Asisten Tanpa Divisi", entity.RoleAsisten, ""},
		{3, "fixture-secretary", "sekretaris@example.test", "Asisten Sekretaris", entity.RoleAsisten, entity.DivisionSekretaris},
		{4, "fixture-treasurer", "bendahara@example.test", "Asisten Bendahara", entity.RoleAsisten, entity.DivisionBendahara},
		{5, "fixture-k3", "k3@example.test", "Asisten K3", entity.RoleAsisten, entity.DivisionK3},
		{6, "fixture-labops", "labops@example.test", "Asisten Labops", entity.RoleAsisten, entity.DivisionLabops},
		{7, "fixture-education", "pendidikan@example.test", "Asisten Pendidikan", entity.RoleAsisten, entity.DivisionPendidikan},
		{8, "fixture-pdd", "pdd@example.test", "Asisten PDD", entity.RoleAsisten, entity.DivisionPDD},
		{9, "fixture-coordinator", "koordinator@example.test", "Koordinator Fixture", entity.RoleKoordinator, ""},
		{10, "fixture-borrower", "peminjam@example.test", "Peminjam Fixture", entity.RolePeminjam, ""},
		{11, "fixture-scoped", "scoped@example.test", "Asisten Scoped", entity.RoleAsisten, entity.DivisionPendidikan},
	}
	for _, fixture := range fixtures {
		user := entity.UserProfile{ID: fixture.id, AuthUserID: fixture.auth, Email: fixture.email, Name: fixture.name, Active: true, CreatedAt: now, UpdatedAt: now, AccessVersion: 1}
		r.state.users[user.ID], r.state.byAuth[user.AuthUserID] = user, user.ID
		scope := entity.Scope{Division: string(fixture.division)}
		if fixture.id == 11 {
			scope = entity.Scope{Domain: "praktikum", Division: "pendidikan", ClassID: "class-101", Shift: 1, SessionID: "session-1"}
			r.state.grants[user.ID] = []assignmentGrant{{AssignmentID: fixture.id, Grant: entity.PermissionGrant{Permission: "practicum.session.manage", App: "praktikum", Role: fixture.role, Division: fixture.division, Scope: scope}}}
		}
		r.state.assignments[user.ID] = []entity.RoleAssignment{{ID: fixture.id, UserID: user.ID, Role: fixture.role, Division: fixture.division, Scope: scope, Active: true}}
	}
	return r
}

func (r *MemoryIdentityRepository) seedDefinitions() {
	for _, definition := range []entity.PermissionDefinition{
		{Code: "admin.overview.read", App: "admin", Description: "Melihat ringkasan console admin"},
		{Code: "user.read", App: "all", Description: "Membaca user dalam scope"},
		{Code: "user.write", App: "all", Description: "Mengubah user dalam scope"},
		{Code: "schedule.write", App: "siakad", Description: "Mengelola jadwal"},
		{Code: "schedule.approve", App: "praktikum", Description: "Menyetujui jadwal praktikum"},
		{Code: "class.manage", App: "siakad", Description: "Mengelola kelas"},
		{Code: "attendance.read", App: "siakad", Description: "Membaca absensi"},
		{Code: "inventory.manage", App: "siakad", Description: "Mengelola inventaris"},
		{Code: "rental.approve", App: "siakad", Description: "Menyetujui penyewaan"},
		{Code: "finance.manage", App: "siakad", Description: "Mengelola data keuangan"},
		{Code: "learning.content.read", App: "elearning", Description: "Membaca konten pembelajaran"},
		{Code: "learning.content.manage", App: "elearning", Description: "Mengelola konten pembelajaran"},
		{Code: "learning.monitor", App: "elearning", Description: "Memantau pembelajaran"},
		{Code: "learning.xp.manage", App: "elearning", Description: "Mengelola XP dan prestige mahasiswa"},
		{Code: "practicum.session.manage", App: "praktikum", Description: "Mengelola sesi praktikum"},
		{Code: "question.manage", App: "praktikum", Description: "Mengelola bank soal"},
		{Code: "practicum.grade.write", App: "praktikum", Description: "Mengubah penilaian"},
		{Code: "audit.read", App: "all", Description: "Membaca audit log sesuai scope"},
		{Code: "settings.manage", App: "all", Description: "Mengelola pengaturan"},
		{Code: "identity.access.manage", App: "admin", Description: "Mengelola role, divisi, dan permission user"},
	} {
		r.state.definitions[definition.Code] = definition
	}
	for _, permission := range []string{"admin.overview.read", "user.read", "user.write", "class.manage", "schedule.write", "attendance.read", "inventory.manage", "rental.approve", "finance.manage", "learning.content.read", "learning.content.manage", "learning.monitor", "learning.xp.manage", "practicum.session.manage", "question.manage", "practicum.grade.write", "audit.read", "settings.manage", "identity.access.manage"} {
		scope := entity.Scope{Domain: "*"}
		if permission == "admin.overview.read" {
			scope = entity.Scope{}
		} else if permission == "question.manage" || permission == "practicum.grade.write" {
			scope = entity.Scope{Domain: "praktikum"}
		} else if permission == "learning.content.read" || permission == "learning.content.manage" {
			scope = entity.Scope{Domain: "elearning"}
		} else if permission == "schedule.write" || permission == "attendance.read" || permission == "inventory.manage" || permission == "rental.approve" || permission == "finance.manage" {
			scope = entity.Scope{Domain: "siakad"}
		}
		r.state.roleDefaults[entity.RoleKoordinator] = append(r.state.roleDefaults[entity.RoleKoordinator], permissionDefault{Permission: permission, Scope: scope})
	}
	for _, role := range []entity.RoleCode{entity.RoleAsisten, entity.RoleMahasiswa} {
		r.state.roleDefaults[role] = []permissionDefault{{Permission: "learning.content.read", Scope: entity.Scope{Domain: "elearning"}}}
	}
	for _, division := range []entity.DivisionCode{entity.DivisionSekretaris, entity.DivisionBendahara, entity.DivisionK3, entity.DivisionLabops, entity.DivisionPendidikan, entity.DivisionPDD} {
		r.state.divisions[division] = true
		r.state.divisionDefaults[division] = []permissionDefault{{Permission: "admin.overview.read"}}
	}
	for _, grant := range []struct {
		division   entity.DivisionCode
		permission string
		domain     string
	}{
		{entity.DivisionSekretaris, "schedule.write", "siakad"},
		{entity.DivisionSekretaris, "attendance.read", "siakad"},
		{entity.DivisionSekretaris, "schedule.approve", "praktikum"},
		{entity.DivisionBendahara, "finance.manage", "siakad"},
		{entity.DivisionK3, "inventory.manage", "siakad"},
		{entity.DivisionK3, "rental.approve", "siakad"},
		{entity.DivisionLabops, "inventory.manage", "siakad"},
		{entity.DivisionLabops, "rental.approve", "siakad"},
		{entity.DivisionPendidikan, "learning.content.manage", "elearning"},
		{entity.DivisionPendidikan, "learning.monitor", "elearning"},
	} {
		r.state.divisionDefaults[grant.division] = append(r.state.divisionDefaults[grant.division], permissionDefault{Permission: grant.permission, Scope: entity.Scope{Domain: grant.domain, Division: string(grant.division)}})
	}
}

func (s memoryIdentityState) clone() memoryIdentityState {
	copy := s
	copy.users, copy.byAuth = maps.Clone(s.users), maps.Clone(s.byAuth)
	copy.assignments = make(map[int64][]entity.RoleAssignment, len(s.assignments))
	for id, assignments := range s.assignments {
		copy.assignments[id] = append([]entity.RoleAssignment{}, assignments...)
	}
	copy.grants = make(map[int64][]assignmentGrant, len(s.grants))
	for id, grants := range s.grants {
		copy.grants[id] = append([]assignmentGrant{}, grants...)
	}
	copy.sessions = make(map[string]entity.Session, len(s.sessions))
	for id, session := range s.sessions {
		copy.sessions[id] = cloneSession(session)
	}
	copy.handshakes = make(map[string]entity.SSOHandshake, len(s.handshakes))
	for hash, handshake := range s.handshakes {
		copy.handshakes[hash] = cloneHandshake(handshake)
	}
	copy.audit = make([]entity.AuditEvent, len(s.audit))
	for i, event := range s.audit {
		copy.audit[i] = cloneAudit(event)
	}
	copy.divisions, copy.definitions, copy.rateLimits = maps.Clone(s.divisions), maps.Clone(s.definitions), maps.Clone(s.rateLimits)
	copy.roleDefaults = make(map[entity.RoleCode][]permissionDefault, len(s.roleDefaults))
	for code, defaults := range s.roleDefaults {
		copy.roleDefaults[code] = append([]permissionDefault{}, defaults...)
	}
	copy.divisionDefaults = make(map[entity.DivisionCode][]permissionDefault, len(s.divisionDefaults))
	for code, defaults := range s.divisionDefaults {
		copy.divisionDefaults[code] = append([]permissionDefault{}, defaults...)
	}
	return copy
}

func (r *MemoryIdentityRepository) Transaction(ctx context.Context, callback func(IdentityRepository) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	tx := &MemoryIdentityRepository{state: r.state.clone()}
	if err := callback(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	r.state = tx.state.clone()
	return nil
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

func (r *MemoryIdentityRepository) EnsureUser(_ context.Context, identity entity.AuthIdentity, now time.Time) (entity.UserProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	email := normalizeEmail(identity.Email)
	if identity.AuthUserID == "" || email == "" {
		return entity.UserProfile{}, ErrConflict
	}
	id, exists := r.state.byAuth[identity.AuthUserID]
	for otherID, other := range r.state.users {
		if otherID != id && other.Email == email {
			return entity.UserProfile{}, ErrConflict
		}
	}
	if exists {
		user := r.state.users[id]
		if user.Email != email {
			user.Email, user.UpdatedAt = email, now
			r.state.users[id] = user
		}
		return user, nil
	}
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = email
	}
	user := entity.UserProfile{ID: r.state.nextUserID, AuthUserID: identity.AuthUserID, Email: email, Name: name, Active: true, CreatedAt: now, UpdatedAt: now, AccessVersion: 1}
	r.state.nextUserID++
	r.state.users[user.ID], r.state.byAuth[user.AuthUserID] = user, user.ID
	return user, nil
}

func (r *MemoryIdentityRepository) FindUserByID(_ context.Context, id int64) (entity.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.state.users[id]
	if !ok {
		return entity.UserProfile{}, ErrIdentityNotFound
	}
	return user, nil
}

func (r *MemoryIdentityRepository) FindUserByAuthID(_ context.Context, authID string) (entity.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.state.byAuth[authID]
	if !ok {
		return entity.UserProfile{}, ErrIdentityNotFound
	}
	return r.state.users[id], nil
}

func (r *MemoryIdentityRepository) SaveUser(_ context.Context, user entity.UserProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.state.users[user.ID]
	if !ok || existing.AuthUserID != user.AuthUserID {
		return ErrIdentityNotFound
	}
	user.Email, user.NIM, user.Name = normalizeEmail(user.Email), strings.TrimSpace(user.NIM), strings.TrimSpace(user.Name)
	if user.Email == "" {
		return ErrConflict
	}
	for id, other := range r.state.users {
		if id != user.ID && (other.Email == user.Email || (user.NIM != "" && other.NIM == user.NIM)) {
			return ErrConflict
		}
	}
	user.AccessVersion, user.CreatedAt = existing.AccessVersion, existing.CreatedAt
	r.state.users[user.ID] = user
	return nil
}

func pageLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func (r *MemoryIdentityRepository) ListUsers(_ context.Context, limit, offset int) ([]entity.UserProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make([]entity.UserProfile, 0, len(r.state.users))
	for _, user := range r.state.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	if offset < 0 {
		offset = 0
	}
	if offset >= len(users) {
		return []entity.UserProfile{}, nil
	}
	return users[offset:min(len(users), offset+pageLimit(limit))], nil
}

func (r *MemoryIdentityRepository) ListPermissionDefinitions(_ context.Context) ([]entity.PermissionDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definitions := make([]entity.PermissionDefinition, 0, len(r.state.definitions))
	for _, definition := range r.state.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Code < definitions[j].Code })
	return definitions, nil
}

func (r *MemoryIdentityRepository) ReadAccess(_ context.Context, userID int64) (entity.UserAccess, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.state.users[userID]
	if !ok {
		return entity.UserAccess{}, ErrIdentityNotFound
	}
	access := entity.UserAccess{Version: user.AccessVersion, Assignments: append([]entity.RoleAssignment{}, r.state.assignments[userID]...), Grants: []entity.PermissionGrant{}}
	for _, grant := range r.state.grants[userID] {
		access.Grants = append(access.Grants, grant.Grant)
	}
	return access, nil
}

func validRole(role entity.RoleCode) bool {
	return role == entity.RoleKoordinator || role == entity.RoleAsisten || role == entity.RoleMahasiswa || role == entity.RolePeminjam
}

func matchingAssignment(grant entity.PermissionGrant, assignments []entity.RoleAssignment, divisions map[entity.DivisionCode]bool) (int64, bool) {
	for _, assignment := range assignments {
		if !assignment.Active || assignment.Role != grant.Role || assignment.Division != grant.Division || !assignment.Scope.Contains(grant.Scope) {
			continue
		}
		if assignment.Division != "" && !divisions[assignment.Division] {
			continue
		}
		if grant.Permission == "identity.access.manage" && !globalCoordinator(assignment, grant.Scope) {
			continue
		}
		return assignment.ID, true
	}
	return 0, false
}

func globalCoordinator(assignment entity.RoleAssignment, scope entity.Scope) bool {
	return assignment.Role == entity.RoleKoordinator && assignment.Division == "" && assignment.Scope.Contains(entity.Scope{}) && scope.Contains(entity.Scope{})
}

func (r *MemoryIdentityRepository) ReplaceAccess(_ context.Context, userID int64, access entity.UserAccess) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.state.users[userID]
	if !ok {
		return ErrIdentityNotFound
	}
	if user.AccessVersion != access.Version {
		return ErrConflict
	}
	assignments := append([]entity.RoleAssignment{}, access.Assignments...)
	nextID := r.state.nextAssignmentID
	type assignmentKey struct {
		Role     entity.RoleCode
		Division entity.DivisionCode
		Scope    entity.Scope
	}
	seenAssignments := make(map[assignmentKey]bool)
	for i := range assignments {
		a := &assignments[i]
		if !validRole(a.Role) {
			return ErrConflict
		}
		if a.Division != "" {
			if _, exists := r.state.divisions[a.Division]; !exists || a.Role != entity.RoleAsisten {
				return ErrConflict
			}
		}
		key := assignmentKey{a.Role, a.Division, a.Scope}
		if seenAssignments[key] {
			return ErrConflict
		}
		seenAssignments[key] = true
		a.ID, a.UserID = nextID, userID
		nextID++
	}
	grants := make([]assignmentGrant, 0, len(access.Grants))
	seenGrants := make(map[assignmentGrant]bool)
	for _, grant := range access.Grants {
		definition, exists := r.state.definitions[grant.Permission]
		if !exists {
			return ErrConflict
		}
		assignmentID, matches := matchingAssignment(grant, assignments, r.state.divisions)
		if !matches {
			return ErrConflict
		}
		grant.App = definition.App
		stored := assignmentGrant{AssignmentID: assignmentID, Grant: grant}
		if seenGrants[stored] {
			return ErrConflict
		}
		seenGrants[stored] = true
		grants = append(grants, stored)
	}
	user.AccessVersion++
	user.UpdatedAt = time.Now().UTC()
	r.state.users[userID], r.state.assignments[userID], r.state.grants[userID] = user, assignments, grants
	r.state.nextAssignmentID = nextID
	return nil
}

func (r *MemoryIdentityRepository) ListAssignments(_ context.Context, userID int64) ([]entity.RoleAssignment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []entity.RoleAssignment{}
	if !r.state.users[userID].Active {
		return result, nil
	}
	for _, assignment := range r.state.assignments[userID] {
		if assignment.Active && (assignment.Division == "" || r.state.divisions[assignment.Division]) {
			result = append(result, assignment)
		}
	}
	return result, nil
}

func effectiveGrant(assignment entity.RoleAssignment, grant entity.PermissionGrant) (entity.PermissionGrant, bool) {
	if grant.Permission == "identity.access.manage" && !globalCoordinator(assignment, grant.Scope) {
		return entity.PermissionGrant{}, false
	}
	scope, ok := assignment.Scope.Intersect(grant.Scope)
	if ok && assignment.Division != "" {
		scope, ok = scope.Intersect(entity.Scope{Division: string(assignment.Division)})
	}
	grant.Role, grant.Division, grant.Scope = assignment.Role, assignment.Division, scope
	return grant, ok
}

func uniqueGrants(grants []entity.PermissionGrant) []entity.PermissionGrant {
	result := make([]entity.PermissionGrant, 0, len(grants))
	seen := make(map[entity.PermissionGrant]bool)
	for _, grant := range grants {
		if !seen[grant] {
			seen[grant] = true
			result = append(result, grant)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Permission < result[j].Permission })
	return result
}

func (r *MemoryIdentityRepository) ListPermissionGrants(_ context.Context, userID int64) ([]entity.PermissionGrant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []entity.PermissionGrant{}
	if !r.state.users[userID].Active {
		return result, nil
	}
	for _, assignment := range r.state.assignments[userID] {
		if !assignment.Active || (assignment.Division != "" && !r.state.divisions[assignment.Division]) {
			continue
		}
		defaults := append([]permissionDefault{}, r.state.roleDefaults[assignment.Role]...)
		if assignment.Role == entity.RoleAsisten {
			defaults = append(defaults, r.state.divisionDefaults[assignment.Division]...)
		}
		for _, candidate := range defaults {
			definition := r.state.definitions[candidate.Permission]
			if grant, ok := effectiveGrant(assignment, entity.PermissionGrant{Permission: candidate.Permission, App: definition.App, Scope: candidate.Scope}); ok {
				result = append(result, grant)
			}
		}
		for _, candidate := range r.state.grants[userID] {
			if candidate.AssignmentID == assignment.ID {
				if grant, ok := effectiveGrant(assignment, candidate.Grant); ok {
					result = append(result, grant)
				}
			}
		}
	}
	return uniqueGrants(result), nil
}

func cloneSession(session entity.Session) entity.Session {
	session.ProviderRefreshTokenSeal = append([]byte(nil), session.ProviderRefreshTokenSeal...)
	if session.RevokedAt != nil {
		at := *session.RevokedAt
		session.RevokedAt = &at
	}
	return session
}

func (r *MemoryIdentityRepository) SaveSession(_ context.Context, session entity.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session.FamilyID == "" {
		session.FamilyID = session.ID
	}
	if session.ID == "" || !validHash(session.TokenHash) || !validHash(session.CSRFTokenHash) || !session.ExpiresAt.After(session.CreatedAt) {
		return ErrConflict
	}
	if _, exists := r.state.users[session.UserID]; !exists {
		return ErrConflict
	}
	if session.FamilyID != session.ID {
		root, exists := r.state.sessions[session.FamilyID]
		if !exists || root.FamilyID != root.ID || root.UserID != session.UserID {
			return ErrConflict
		}
	}
	for id, existing := range r.state.sessions {
		if id != session.ID && existing.TokenHash == session.TokenHash {
			return ErrConflict
		}
	}
	if existing, exists := r.state.sessions[session.ID]; exists {
		if existing.RevokedAt != nil || existing.UserID != session.UserID || existing.FamilyID != session.FamilyID || existing.Host != session.Host {
			return ErrSessionNotFound
		}
		session.CreatedAt = existing.CreatedAt
		if session.ExpiresAt.After(existing.ExpiresAt) {
			session.ExpiresAt = existing.ExpiresAt
		}
	}
	r.state.sessions[session.ID] = cloneSession(session)
	return nil
}

func (r *MemoryIdentityRepository) FindSessionByID(_ context.Context, id string) (entity.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.state.sessions[id]
	if !ok {
		return entity.Session{}, ErrSessionNotFound
	}
	return cloneSession(session), nil
}

func (r *MemoryIdentityRepository) FindSessionByTokenHash(_ context.Context, tokenHash string) (entity.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, session := range r.state.sessions {
		if session.TokenHash == tokenHash {
			return cloneSession(session), nil
		}
	}
	return entity.Session{}, ErrSessionNotFound
}

func (r *MemoryIdentityRepository) ListSessions(_ context.Context, userID int64, _ time.Time) ([]entity.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []entity.Session{}
	for _, session := range r.state.sessions {
		if session.UserID == userID {
			result = append(result, cloneSession(session))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (r *MemoryIdentityRepository) RevokeSession(_ context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.state.sessions[id]; !ok {
		return ErrSessionNotFound
	}
	for key, session := range r.state.sessions {
		if (session.ID == id || session.FamilyID == id) && session.RevokedAt == nil {
			revoked := at
			session.RevokedAt = &revoked
			r.state.sessions[key] = session
		}
	}
	return nil
}

func cloneHandshake(handshake entity.SSOHandshake) entity.SSOHandshake {
	if handshake.ConsumedAt != nil {
		at := *handshake.ConsumedAt
		handshake.ConsumedAt = &at
	}
	return handshake
}

func validHash(hash string) bool {
	if len(hash) != 64 || hash != strings.ToLower(hash) {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func validChallenge(challenge string) bool {
	return len(challenge) == 43 && strings.IndexFunc(challenge, func(c rune) bool {
		return !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_')
	}) == -1
}

func (r *MemoryIdentityRepository) SaveHandshake(_ context.Context, handshake entity.SSOHandshake) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !validHash(handshake.CodeHash) || !validChallenge(handshake.CodeChallenge) || strings.TrimSpace(handshake.ReturnTo) == "" {
		return ErrConflict
	}
	if _, exists := r.state.handshakes[handshake.CodeHash]; exists {
		return ErrConflict
	}
	issuer, exists := r.state.sessions[handshake.SessionID]
	root, rootExists := r.state.sessions[issuer.FamilyID]
	if !exists || issuer.RevokedAt != nil || !rootExists || root.FamilyID != root.ID || root.UserID != issuer.UserID || root.RevokedAt != nil {
		return ErrConflict
	}
	r.state.handshakes[handshake.CodeHash] = cloneHandshake(handshake)
	return nil
}

func (r *MemoryIdentityRepository) ConsumeHandshake(_ context.Context, codeHash string, now time.Time) (entity.SSOHandshake, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	handshake, ok := r.state.handshakes[codeHash]
	if !ok || handshake.ConsumedAt != nil || !handshake.ExpiresAt.After(now) {
		return entity.SSOHandshake{}, ErrHandshakeInvalid
	}
	handshake.ConsumedAt = &now
	r.state.handshakes[codeHash] = handshake
	return cloneHandshake(handshake), nil
}

func hashRateKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (r *MemoryIdentityRepository) TakeRateLimit(_ context.Context, key string, windowStart time.Time, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	if key == "" || windowStart.IsZero() || limit > 2147483647 {
		return false, ErrConflict
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	windowStart = windowStart.UTC().Truncate(time.Microsecond)
	pruned := 0
	for window := range r.state.rateLimits {
		if window.Start.Before(windowStart.Add(-24 * time.Hour)) {
			delete(r.state.rateLimits, window)
			pruned++
			if pruned == 128 {
				break
			}
		}
	}
	window := rateWindow{Key: hashRateKey(key), Start: windowStart}
	if r.state.rateLimits[window] >= limit {
		return false, nil
	}
	r.state.rateLimits[window]++
	return true, nil
}

func cloneAudit(event entity.AuditEvent) entity.AuditEvent {
	if event.ActorID != nil {
		id := *event.ActorID
		event.ActorID = &id
	}
	if event.Detail != nil {
		data, _ := json.Marshal(event.Detail)
		event.Detail = nil
		_ = json.Unmarshal(data, &event.Detail)
	}
	return event
}

func (r *MemoryIdentityRepository) AppendAudit(_ context.Context, event entity.AuditEvent) error {
	if _, err := json.Marshal(event.Detail); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ActorID != nil {
		if _, exists := r.state.users[*event.ActorID]; !exists {
			return ErrConflict
		}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	event.ID = r.state.nextAuditID
	r.state.nextAuditID++
	r.state.audit = append(r.state.audit, cloneAudit(event))
	return nil
}

func (r *MemoryIdentityRepository) ListAudit(ctx context.Context, limit int) ([]entity.AuditEvent, error) {
	return r.ListAuditPage(ctx, 0, limit)
}

func (r *MemoryIdentityRepository) ListAuditPage(_ context.Context, beforeID int64, limit int) ([]entity.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []entity.AuditEvent{}
	for i := len(r.state.audit) - 1; i >= 0 && len(result) < pageLimit(limit); i-- {
		if beforeID <= 0 || r.state.audit[i].ID < beforeID {
			result = append(result, cloneAudit(r.state.audit[i]))
		}
	}
	return result, nil
}
