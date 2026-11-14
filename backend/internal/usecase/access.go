package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

var scopeID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,100}$`)

func (s IdentityService) CanOpenAdmin(principal *entity.Principal, path string) error {
	if principal == nil {
		return authorization.ErrUnauthenticated
	}
	adminRole := false
	for _, assignment := range principal.Assignments {
		if assignment.Active && (assignment.Role == entity.RoleAsisten || assignment.Role == entity.RoleKoordinator) {
			adminRole = true
		}
	}
	if !adminRole {
		return authorization.ErrForbidden
	}
	if err := s.authorizer.Check(principal, authorization.Requirement{Permission: "admin.overview.read", ResourceExists: true, AnyScope: true}); err != nil {
		return err
	}
	var permissions []string
	domain := ""
	switch path {
	case "/", "/profil":
		return nil
	case "/pengaturan":
		return s.CanManageAccess(principal)
	case "/auditlog":
		permissions = []string{"audit.read"}
	case "/praktikum":
		domain, permissions = "praktikum", []string{"practicum.session.manage", "question.manage", "practicum.grade.write", "schedule.approve"}
	case "/elearning":
		domain, permissions = "elearning", []string{"learning.content.manage", "learning.monitor"}
	case "/siakad":
		domain, permissions = "siakad", []string{"class.manage", "schedule.write", "attendance.read", "inventory.manage", "rental.approve", "finance.manage"}
	default:
		return authorization.ErrNotFound
	}
	for _, permission := range permissions {
		if s.authorizer.Check(principal, authorization.Requirement{Permission: permission, Scope: entity.Scope{Domain: domain}, ResourceExists: true, AnyScope: true}) == nil {
			return nil
		}
	}
	return authorization.ErrForbidden
}

func (s IdentityService) CanManageAccess(principal *entity.Principal) error {
	if principal == nil {
		return authorization.ErrUnauthenticated
	}
	coordinator := false
	for _, assignment := range principal.Assignments {
		if assignment.Active && assignment.Role == entity.RoleKoordinator && assignment.Scope.Contains(entity.Scope{}) {
			coordinator = true
		}
	}
	if !coordinator {
		return authorization.ErrForbidden
	}
	return s.authorizer.Check(principal, authorization.Requirement{Permission: "identity.access.manage", ResourceExists: true})
}

func (s IdentityService) ChangeAccess(ctx context.Context, principal *entity.Principal, userID int64, access entity.UserAccess, requestID string) (updated entity.UserAccess, err error) {
	if principal == nil {
		return updated, authorization.ErrUnauthenticated
	}
	if userID == principal.User.ID {
		return updated, ErrValidation
	}
	err = s.transaction(ctx, func(tx IdentityService) error {
		if _, e := tx.activeSession(ctx, principal.SessionID, principal.User.ID); e != nil {
			return e
		}
		actor, e := tx.repository.FindUserByID(ctx, principal.User.ID)
		if e != nil {
			return e
		}
		fresh, e := tx.principal(ctx, actor, principal.SessionID, principal.Host)
		if e != nil {
			return e
		}
		if e = tx.CanManageAccess(&fresh); e != nil {
			return e
		}
		catalog, e := tx.repository.ListPermissionDefinitions(ctx)
		if e != nil {
			return e
		}
		if e = validateAccess(&access, catalog); e != nil {
			return e
		}
		before, e := tx.repository.ReadAccess(ctx, userID)
		if e != nil {
			return e
		}
		if e = tx.repository.ReplaceAccess(ctx, userID, access); e != nil {
			return e
		}
		updated, e = tx.repository.ReadAccess(ctx, userID)
		if e != nil {
			return e
		}
		return tx.audit(ctx, &actor.ID, "identity.access.update", "identity", "user", fmt.Sprint(userID), entity.Scope{}, requestID, map[string]any{"before": before, "after": updated})
	})
	return
}

func validateAccess(access *entity.UserAccess, catalog []entity.PermissionDefinition) error {
	if access.Version < 1 || len(access.Assignments) > 16 || len(access.Grants) > 64 {
		return ErrValidation
	}
	permissions := make(map[string]string, len(catalog))
	for _, permission := range catalog {
		permissions[permission.Code] = permission.App
	}
	seen := make(map[string]bool)
	for i := range access.Assignments {
		assignment := &access.Assignments[i]
		if !validRole(assignment.Role) || !validDivision(assignment.Division) || (assignment.Role != entity.RoleAsisten && assignment.Division != "") || !validScope(assignment.Scope) {
			return ErrValidation
		}
		if assignment.Division != "" {
			if assignment.Scope.Division != "" && assignment.Scope.Division != string(assignment.Division) {
				return ErrValidation
			}
			assignment.Scope.Division = string(assignment.Division)
		}
		assignment.ID, assignment.UserID, assignment.Active = 0, 0, true
		key, _ := json.Marshal(assignment)
		if seen[string(key)] {
			return ErrValidation
		}
		seen[string(key)] = true
	}
	seen = make(map[string]bool)
	for i := range access.Grants {
		grant := &access.Grants[i]
		app, exists := permissions[grant.Permission]
		if !exists || !validScope(grant.Scope) || !validRole(grant.Role) || !validDivision(grant.Division) {
			return ErrValidation
		}
		grant.App = app
		if grant.Division != "" {
			if grant.Scope.Division != "" && grant.Scope.Division != string(grant.Division) {
				return ErrValidation
			}
			grant.Scope.Division = string(grant.Division)
		}
		matched := false
		for _, assignment := range access.Assignments {
			if assignment.Role == grant.Role && assignment.Division == grant.Division && assignment.Scope.Contains(grant.Scope) {
				matched = true
			}
		}
		if !matched {
			return ErrValidation
		}
		if grant.Permission == "identity.access.manage" && (grant.Role != entity.RoleKoordinator || !grant.Scope.Contains(entity.Scope{})) {
			return ErrValidation
		}
		key, _ := json.Marshal(grant)
		if seen[string(key)] {
			return ErrValidation
		}
		seen[string(key)] = true
	}
	return nil
}

func validRole(role entity.RoleCode) bool {
	return role == entity.RoleKoordinator || role == entity.RoleAsisten || role == entity.RoleMahasiswa || role == entity.RolePeminjam
}

func validDivision(division entity.DivisionCode) bool {
	switch division {
	case "", entity.DivisionSekretaris, entity.DivisionBendahara, entity.DivisionK3, entity.DivisionLabops, entity.DivisionPendidikan, entity.DivisionPDD:
		return true
	}
	return false
}

func validScope(scope entity.Scope) bool {
	if scope.Shift < 0 || scope.Shift > 100 {
		return false
	}
	switch scope.Domain {
	case "", "*", "portal", "admin", "auth", "praktikum", "elearning", "siakad":
	default:
		return false
	}
	if scope.Division != "*" && !validDivision(entity.DivisionCode(scope.Division)) {
		return false
	}
	for _, id := range []string{scope.Institution, scope.ClassID, scope.SessionID} {
		if id != "" && id != "*" && (!scopeID.MatchString(id) || strings.TrimSpace(id) != id) {
			return false
		}
	}
	return true
}

func (s IdentityService) AuditPage(ctx context.Context, principal *entity.Principal, before int64, limit int) ([]entity.AuditEvent, int64, error) {
	if err := s.CanOpenAdmin(principal, "/auditlog"); err != nil {
		return nil, 0, err
	}
	rows, err := s.repository.ListAuditPage(ctx, before, limit)
	if err != nil {
		return nil, 0, err
	}
	items := make([]entity.AuditEvent, 0, len(rows))
	for _, row := range rows {
		if s.authorizer.Check(principal, authorization.Requirement{Permission: "audit.read", Scope: row.Scope, ResourceExists: true}) == nil {
			items = append(items, row)
		}
	}
	next := int64(0)
	if len(rows) == limit {
		next = rows[len(rows)-1].ID
	}
	return items, next, nil
}
