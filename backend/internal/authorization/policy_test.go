package authorization

import (
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

func principalFor(role entity.RoleCode, grants []entity.PermissionGrant) *entity.Principal {
	return &entity.Principal{User: entity.UserProfile{ID: 1, Active: true}, Assignments: []entity.RoleAssignment{{Role: role, Active: true}}, Grants: grants}
}

func TestAuthorizationChainRequiresPermissionAndScope(t *testing.T) {
	policy := Service{}
	grant := entity.PermissionGrant{Permission: "practicum.session.manage", Role: entity.RoleAsisten, Scope: entity.Scope{Domain: "praktikum", ClassID: "class-101", Shift: 1}}
	principal := principalFor(entity.RoleAsisten, []entity.PermissionGrant{grant})

	if err := policy.Check(principal, Requirement{Permission: "practicum.session.manage", Scope: entity.Scope{Domain: "praktikum", ClassID: "class-101", Shift: 1}, ResourceExists: true}); err != nil {
		t.Fatalf("expected scoped permission to pass: %v", err)
	}
	if err := policy.Check(principal, Requirement{Permission: "practicum.session.manage", Scope: entity.Scope{Domain: "praktikum", ClassID: "class-202", Shift: 1}, ResourceExists: true, HideNotFound: true}); !Is(err, "NOT_FOUND") {
		t.Fatalf("expected non-enumerable 404, got %v", err)
	}
	if err := policy.Check(principal, Requirement{Permission: "practicum.session.manage", Scope: entity.Scope{Domain: "praktikum"}, ResourceExists: true}); !Is(err, "FORBIDDEN") {
		t.Fatalf("omitting resource scope must not bypass restrictions: %v", err)
	}
	principal.Assignments[0].Active = false
	if err := policy.Check(principal, Requirement{Permission: "practicum.session.manage", Scope: grant.Scope, ResourceExists: true}); !Is(err, "FORBIDDEN") {
		t.Fatalf("stale grant survived assignment revocation: %v", err)
	}
}

func TestAuthorizationDistinguishesAnonymousAndForbidden(t *testing.T) {
	policy := Service{}
	if err := policy.Check(nil, Requirement{Permission: "admin.overview.read"}); !Is(err, "UNAUTHENTICATED") {
		t.Fatalf("expected 401 policy error, got %v", err)
	}
	principal := principalFor(entity.RoleMahasiswa, nil)
	if err := policy.Check(principal, Requirement{Permission: "admin.overview.read", ResourceExists: true}); !Is(err, "FORBIDDEN") {
		t.Fatalf("expected 403 policy error, got %v", err)
	}
}
