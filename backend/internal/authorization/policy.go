package authorization

import (
	"errors"
	"fmt"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

var (
	ErrUnauthenticated = errors.New("UNAUTHENTICATED")
	ErrForbidden       = errors.New("FORBIDDEN")
	ErrNotFound        = errors.New("NOT_FOUND")
)

type Requirement struct {
	Permission     string
	Scope          entity.Scope
	ResourceOwner  *int64
	HideNotFound   bool
	ResourceExists bool
	AnyScope       bool
}

type PolicyError struct {
	Code       string
	Status     int
	Permission string
	Message    string
}

func (e *PolicyError) Error() string { return e.Code + ": " + e.Message }

type Service struct{}

func (Service) Check(principal *entity.Principal, requirement Requirement) error {
	if principal == nil {
		return &PolicyError{Code: "UNAUTHENTICATED", Status: 401, Message: "Sesi login diperlukan."}
	}
	if !principal.User.Active {
		return &PolicyError{Code: "FORBIDDEN", Status: 403, Message: "Akun tidak aktif."}
	}
	if requirement.ResourceOwner != nil && *requirement.ResourceOwner != principal.User.ID {
		return policyDenied(requirement, "Resource bukan milik user ini.")
	}
	if !requirement.ResourceExists {
		return policyDenied(requirement, "Resource tidak ditemukan.")
	}
	for _, grant := range principal.Grants {
		if grant.Permission != requirement.Permission {
			continue
		}
		active := false
		for _, assignment := range principal.Assignments {
			if assignment.Active && assignment.Role == grant.Role && assignment.Division == grant.Division && assignment.Scope.Contains(grant.Scope) {
				active = true
				break
			}
		}
		if !active {
			continue
		}
		if grant.Scope.Contains(requirement.Scope) || (requirement.AnyScope && grant.Scope.Allows(requirement.Scope)) {
			return nil
		}
	}
	if requirement.HideNotFound {
		return &PolicyError{Code: "NOT_FOUND", Status: 404, Permission: requirement.Permission, Message: "Resource tidak ditemukan."}
	}
	return &PolicyError{Code: "FORBIDDEN", Status: 403, Permission: requirement.Permission, Message: fmt.Sprintf("Permission %q tidak tersedia untuk scope tersebut.", requirement.Permission)}
}

func policyDenied(requirement Requirement, message string) error {
	if requirement.HideNotFound {
		return &PolicyError{Code: "NOT_FOUND", Status: 404, Message: message}
	}
	return &PolicyError{Code: "FORBIDDEN", Status: 403, Message: message}
}

func Is(err error, code string) bool {
	var policyErr *PolicyError
	return errors.As(err, &policyErr) && policyErr.Code == code
}
