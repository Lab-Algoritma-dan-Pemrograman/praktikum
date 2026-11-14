package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func TestFixtureAuthorizationMatrixUsesCanonicalRolesAndDivisions(t *testing.T) {
	repo := NewMemoryIdentityRepository()
	ctx := context.Background()
	fixtures := []struct {
		authID   string
		division entity.DivisionCode
		grant    string
	}{
		{"fixture-secretary", entity.DivisionSekretaris, "schedule.write"},
		{"fixture-treasurer", entity.DivisionBendahara, "finance.manage"},
		{"fixture-k3", entity.DivisionK3, "inventory.manage"},
		{"fixture-labops", entity.DivisionLabops, "inventory.manage"},
		{"fixture-education", entity.DivisionPendidikan, "learning.content.manage"},
		{"fixture-pdd", entity.DivisionPDD, "learning.content.read"},
	}
	for _, fixture := range fixtures {
		user, err := repo.FindUserByAuthID(ctx, fixture.authID)
		if err != nil {
			t.Fatalf("%s user lookup: %v", fixture.authID, err)
		}
		assignments, err := repo.ListAssignments(ctx, user.ID)
		if err != nil || len(assignments) != 1 || assignments[0].Role != entity.RoleAsisten || assignments[0].Division != fixture.division {
			t.Fatalf("%s assignment is not canonical: %+v, %v", fixture.authID, assignments, err)
		}
		grants, err := repo.ListPermissionGrants(ctx, user.ID)
		if err != nil {
			t.Fatalf("%s grant lookup: %v", fixture.authID, err)
		}
		found := false
		for _, grant := range grants {
			if grant.Permission == fixture.grant {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s missing expected grant %q: %+v", fixture.authID, fixture.grant, grants)
		}
	}

	for _, role := range []entity.RoleCode{entity.RoleKoordinator, entity.RoleAsisten, entity.RoleMahasiswa, entity.RolePeminjam} {
		if role == "admin" || role == "kordas" || role == "praktikan" || role == "user" {
			t.Fatalf("legacy alias leaked into canonical role set: %s", role)
		}
	}
}

func TestMemoryProvisioningPreservesProfileWithoutGrantingRoles(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryIdentityRepository()
	identity := entity.AuthIdentity{AuthUserID: "new-provider-user", Email: "  NEW@EXAMPLE.TEST  ", Name: "Provider Name"}
	user, err := repo.EnsureUser(ctx, identity, time.Now())
	if err != nil || user.Email != "new@example.test" || user.AccessVersion != 1 {
		t.Fatalf("new profile: %+v, %v", user, err)
	}
	access, err := repo.ReadAccess(ctx, user.ID)
	if err != nil || len(access.Assignments) != 0 || len(access.Grants) != 0 {
		t.Fatalf("provisioning granted roles: %+v, %v", access, err)
	}
	user.Name, user.NIM, user.Active = "Manual Name", "12345", false
	if err := repo.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	profile, err := repo.EnsureUser(ctx, identity, time.Now())
	if err != nil || profile.Name != user.Name || profile.Active || profile.NIM != user.NIM {
		t.Fatalf("provider overwrote manual profile: %+v, %v", profile, err)
	}
	identity.AuthUserID = "another-provider-user"
	if _, err := repo.EnsureUser(ctx, identity, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate email accepted: %v", err)
	}
	other, _ := repo.FindUserByID(ctx, 1)
	other.NIM = user.NIM
	if err := repo.SaveUser(ctx, other); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate NIM accepted: %v", err)
	}
}

func TestMemoryTransactionRollsBackAllStateAndDoesNotLeakAliases(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryIdentityRepository()
	user, _ := repo.FindUserByID(ctx, 3)
	originalAccess, _ := repo.ReadAccess(ctx, user.ID)
	now := time.Now().UTC()
	abort := errors.New("abort transaction")
	err := repo.Transaction(ctx, func(tx IdentityRepository) error {
		updated := user
		updated.Name = "Rolled Back"
		if err := tx.SaveUser(ctx, updated); err != nil {
			return err
		}
		if _, err := tx.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: "rolled-back", Email: "rollback@example.test"}, now); err != nil {
			return err
		}
		if err := tx.ReplaceAccess(ctx, user.ID, entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true}}}); err != nil {
			return err
		}
		if err := tx.SaveSession(ctx, testSession(user.ID, "rollback-session", now)); err != nil {
			return err
		}
		if err := tx.SaveHandshake(ctx, testHandshake("rollback-session", "rollback-handshake", now)); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, entity.AuditEvent{ActorID: &user.ID, Detail: map[string]any{"nested": map[string]any{"value": "original"}}}); err != nil {
			return err
		}
		if _, err := tx.TakeRateLimit(ctx, "rollback-rate", now, 1); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	persisted, _ := repo.FindUserByID(ctx, user.ID)
	if persisted.Name != user.Name {
		t.Fatalf("profile rollback failed: %+v", persisted)
	}
	if _, err := repo.FindUserByAuthID(ctx, "rolled-back"); !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("provision rollback failed: %v", err)
	}
	access, _ := repo.ReadAccess(ctx, user.ID)
	if !reflect.DeepEqual(access, originalAccess) {
		t.Fatalf("access rollback failed: %+v", access)
	}
	if _, err := repo.FindSessionByID(ctx, "rollback-session"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("session rollback failed: %v", err)
	}
	if _, err := repo.ConsumeHandshake(ctx, hashRateKey("rollback-handshake"), now); !errors.Is(err, ErrHandshakeInvalid) {
		t.Fatalf("handshake rollback failed: %v", err)
	}
	if audit, _ := repo.ListAudit(ctx, 10); len(audit) != 0 {
		t.Fatalf("audit rollback failed: %+v", audit)
	}
	if allowed, err := repo.TakeRateLimit(ctx, "rollback-rate", now, 1); err != nil || !allowed {
		t.Fatalf("rate rollback failed: %v, %v", allowed, err)
	}
	var escaped IdentityRepository
	if err := repo.Transaction(ctx, func(tx IdentityRepository) error {
		escaped = tx
		user.Name = "Committed"
		return tx.SaveUser(ctx, user)
	}); err != nil {
		t.Fatal(err)
	}
	user.Name = "Escaped transaction"
	if err := escaped.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	persisted, _ = repo.FindUserByID(ctx, user.ID)
	if persisted.Name != "Committed" {
		t.Fatalf("transaction clone leaked after commit: %+v", persisted)
	}
	cancelled, cancel := context.WithCancel(ctx)
	err = repo.Transaction(cancelled, func(tx IdentityRepository) error {
		if err := tx.SaveUser(ctx, user); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled transaction committed: %v", err)
	}
	persisted, _ = repo.FindUserByID(ctx, user.ID)
	if persisted.Name != "Committed" {
		t.Fatal("cancelled transaction changed profile")
	}
	detail := map[string]any{"nested": map[string]any{"value": "original"}}
	if err := repo.AppendAudit(ctx, entity.AuditEvent{ActorID: &user.ID, Detail: detail}); err != nil {
		t.Fatal(err)
	}
	detail["nested"].(map[string]any)["value"] = "mutated input"
	page, _ := repo.ListAuditPage(ctx, 0, 1)
	if page[0].Detail["nested"].(map[string]any)["value"] != "original" {
		t.Fatal("audit retained caller map")
	}
	page[0].Detail["nested"].(map[string]any)["value"] = "mutated output"
	page, _ = repo.ListAuditPage(ctx, 0, 1)
	if page[0].Detail["nested"].(map[string]any)["value"] != "original" {
		t.Fatal("audit read exposed mutable stored map")
	}
}

func TestMemoryAccessUsesActiveAssignmentsAndScopedDefaults(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryIdentityRepository()
	access, _ := repo.ReadAccess(ctx, 11)
	if len(access.Grants) != 1 || access.Grants[0].Permission != "practicum.session.manage" {
		t.Fatalf("raw access is not explicit-only: %+v", access)
	}
	grants, _ := repo.ListPermissionGrants(ctx, 11)
	if len(grants) != 2 || !hasPermission(grants, "admin.overview.read") {
		t.Fatalf("scoped fixture permissions: %+v", grants)
	}
	for _, grant := range grants {
		if grant.Scope != access.Assignments[0].Scope {
			t.Fatalf("grant escaped assignment: %+v", grant)
		}
	}
	repo.state.assignments[11][0].Active = false
	grants, _ = repo.ListPermissionGrants(ctx, 11)
	if len(grants) != 0 {
		t.Fatalf("direct grant outlived assignment revocation: %+v", grants)
	}
	repo.state.assignments[11][0].Active = true
	repo.state.divisions[entity.DivisionPendidikan] = false
	grants, _ = repo.ListPermissionGrants(ctx, 11)
	if len(grants) != 0 {
		t.Fatalf("inactive division retained grants: %+v", grants)
	}
	repo.state.divisions[entity.DivisionPendidikan] = true
	access.Grants[0].Scope = entity.Scope{}
	if err := repo.ReplaceAccess(ctx, 11, access); !errors.Is(err, ErrConflict) {
		t.Fatalf("grant broader than assignment accepted: %v", err)
	}
	raw, _ := repo.ReadAccess(ctx, 11)
	if raw.Version != 1 || raw.Grants[0].Scope == (entity.Scope{}) {
		t.Fatalf("failed access replacement changed state: %+v", raw)
	}
	if err := repo.ReplaceAccess(ctx, 9, entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Scope: entity.Scope{Institution: "one-campus"}, Active: true}}}); err != nil {
		t.Fatal(err)
	}
	grants, _ = repo.ListPermissionGrants(ctx, 9)
	if hasPermission(grants, "identity.access.manage") {
		t.Fatal("scoped coordinator obtained global access management")
	}
	if err := repo.ReplaceAccess(ctx, 9, entity.UserAccess{Version: 1}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale version accepted: %v", err)
	}
}

func assertConcurrentAccessReplacement(t *testing.T, ctx context.Context, repo IdentityRepository, userID, version int64) {
	t.Helper()
	errs := make(chan error, 16)
	for range 16 {
		go func() {
			errs <- repo.Transaction(ctx, func(tx IdentityRepository) error {
				return tx.ReplaceAccess(ctx, userID, entity.UserAccess{Version: version, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true}}})
			})
		}()
	}
	wins := 0
	for range 16 {
		if err := <-errs; err == nil {
			wins++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	access, err := repo.ReadAccess(ctx, userID)
	if wins != 1 || err != nil || access.Version != version+1 {
		t.Fatalf("concurrent access replacement: wins=%d access=%+v err=%v", wins, access, err)
	}
}

func TestMemoryConcurrentAccessVersion(t *testing.T) {
	assertConcurrentAccessReplacement(t, context.Background(), NewMemoryIdentityRepository(), 9, 1)
}

func assertHandshakeIssuerBinding(t *testing.T, ctx context.Context, repo IdentityRepository, userID int64, prefix string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	root := testSession(userID, prefix+"-root", now)
	child := testSession(userID, prefix+"-child", now)
	child.FamilyID = root.ID
	for _, session := range []entity.Session{root, child} {
		if err := repo.SaveSession(ctx, session); err != nil {
			t.Fatal(err)
		}
	}
	handshake := testHandshake(child.ID, prefix, now)
	if err := repo.SaveHandshake(ctx, handshake); err != nil {
		t.Fatalf("active child cannot prepare handshake: %v", err)
	}
	consumed, err := repo.ConsumeHandshake(ctx, handshake.CodeHash, now)
	if err != nil || consumed.SessionID != child.ID {
		t.Fatalf("handshake lost issuing session: %+v, %v", consumed, err)
	}
	if err := repo.RevokeSession(ctx, child.ID, now); err != nil {
		t.Fatal(err)
	}
	handshake.CodeHash = hashRateKey(prefix + "-revoked")
	if err := repo.SaveHandshake(ctx, handshake); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoked issuer prepared handshake: %v", err)
	}
	root, err = repo.FindSessionByID(ctx, root.ID)
	if err != nil || !root.Active(now) {
		t.Fatalf("child revocation changed live root: %+v, %v", root, err)
	}
}

func TestMemoryHandshakeIssuerBinding(t *testing.T) {
	assertHandshakeIssuerBinding(t, context.Background(), NewMemoryIdentityRepository(), 1, "issuer-binding")
}

func TestMemorySessionAndHandshakeConcurrentOneUse(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryIdentityRepository()
	now := time.Now().UTC()
	root := testSession(1, "root", now)
	root.ProviderRefreshTokenSeal = []byte{1, 2, 3}
	if err := repo.SaveSession(ctx, root); err != nil {
		t.Fatal(err)
	}
	root.ProviderRefreshTokenSeal[0] = 99
	stored, _ := repo.FindSessionByID(ctx, root.ID)
	if stored.ProviderRefreshTokenSeal[0] != 1 {
		t.Fatal("session retained mutable caller bytes")
	}
	handshake := testHandshake(root.ID, "one-use", now)
	if err := repo.SaveHandshake(ctx, handshake); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 16)
	for range 16 {
		go func() { _, err := repo.ConsumeHandshake(ctx, handshake.CodeHash, now); errs <- err }()
	}
	wins := 0
	for range 16 {
		if err := <-errs; err == nil {
			wins++
		} else if !errors.Is(err, ErrHandshakeInvalid) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("handshake consumed %d times", wins)
	}
	if err := repo.SaveHandshake(ctx, handshake); !errors.Is(err, ErrConflict) {
		t.Fatalf("consumed handshake overwritten: %v", err)
	}
	for i := range 16 {
		go func() {
			errs <- repo.Transaction(ctx, func(tx IdentityRepository) error {
				session, err := tx.FindSessionByTokenHash(ctx, root.TokenHash)
				if err != nil {
					return err
				}
				session.TokenHash = hashRateKey(fmt.Sprint("rotated", i))
				return tx.SaveSession(ctx, session)
			})
		}()
	}
	wins = 0
	for range 16 {
		if err := <-errs; err == nil {
			wins++
		} else if !errors.Is(err, ErrSessionNotFound) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("session rotated %d times", wins)
	}
	child := testSession(1, "child", now)
	child.FamilyID = root.ID
	if err := repo.SaveSession(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := repo.RevokeSession(ctx, root.ID, now); err != nil {
		t.Fatal(err)
	}
	child, _ = repo.FindSessionByID(ctx, child.ID)
	if child.RevokedAt == nil {
		t.Fatal("root revocation left child live")
	}
	if err := repo.SaveSession(ctx, root); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("revoked session restored: %v", err)
	}
}
