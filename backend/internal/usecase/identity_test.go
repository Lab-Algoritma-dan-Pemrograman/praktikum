package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type newIdentityProvider struct{}

func (newIdentityProvider) PasswordLogin(context.Context, string, string) (auth.Identity, error) {
	return auth.Identity{AuthUserID: "new-supabase-user", Email: "new@example.test", Name: "New User"}, nil
}
func (newIdentityProvider) ExchangeCode(context.Context, string, string) (auth.Identity, error) {
	return auth.Identity{}, auth.ErrInvalidAuthCode
}

func TestNewIdentityGetsProfileButNoPrivileges(t *testing.T) {
	repo := repository.NewMemoryIdentityRepository()
	service := NewIdentityService(IdentityOptions{Repository: repo, Provider: newIdentityProvider{}, Allowlist: returnto.Default()})
	cookies, principal, err := service.PasswordLogin(t.Context(), "new@example.test", "password", "auth.algohub.web.id", "req_new")
	if err != nil {
		t.Fatal(err)
	}
	if principal.User.ID == 0 || len(principal.Assignments) != 0 || len(principal.Grants) != 0 {
		t.Fatal("new identity received privileges or no profile")
	}
	if service.CanOpenAdmin(principal, "/") == nil {
		t.Fatal("unassigned new user opened admin")
	}
	_, _, err = service.Authenticate(t.Context(), cookies.Session, "auth.algohub.web.id")
	if err != nil {
		t.Fatal(err)
	}
	_, next, err := service.PasswordLogin(t.Context(), "new@example.test", "password", "auth.algohub.web.id", "req_again")
	if err != nil || next.User.ID != principal.User.ID {
		t.Fatal("repeat login duplicated profile")
	}
}

type auditFailure struct{ repository.IdentityRepository }

func (r auditFailure) AppendAudit(context.Context, entity.AuditEvent) error {
	return errors.New("audit unavailable")
}
func (r auditFailure) Transaction(ctx context.Context, callback func(repository.IdentityRepository) error) error {
	return r.IdentityRepository.Transaction(ctx, func(tx repository.IdentityRepository) error { return callback(auditFailure{tx}) })
}

func TestLoginRollsBackWhenRequiredAuditFails(t *testing.T) {
	repo := repository.NewMemoryIdentityRepository()
	service := NewIdentityService(IdentityOptions{Repository: auditFailure{repo}, Provider: auth.NewFixtureProvider()})
	_, _, err := service.PasswordLogin(t.Context(), "koordinator@example.test", "fixture-password", "auth.algohub.web.id", "req_rollback")
	if err == nil {
		t.Fatal("audit failure was ignored")
	}
	sessions, err := repo.ListSessions(t.Context(), 9, time.Now())
	if err != nil || len(sessions) != 0 {
		t.Fatal("failed login persisted a session")
	}
}

func TestAccessValidationRejectsAliasesAndScopeExpansion(t *testing.T) {
	catalog := []entity.PermissionDefinition{{Code: "audit.read", App: "all"}}
	for _, value := range []entity.UserAccess{
		{Version: 1, Assignments: []entity.RoleAssignment{{Role: "admin"}}},
		{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Division: entity.DivisionK3}}},
		{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleAsisten, Scope: entity.Scope{ClassID: "101"}}}, Grants: []entity.PermissionGrant{{Permission: "audit.read", Role: entity.RoleAsisten, Scope: entity.Scope{}}}},
	} {
		if validateAccess(&value, catalog) == nil {
			t.Fatal("unsafe access assignment accepted")
		}
	}
}
