package entity

import "time"

type RoleCode string

const (
	RoleKoordinator RoleCode = "koordinator"
	RoleAsisten     RoleCode = "asisten"
	RoleMahasiswa   RoleCode = "mahasiswa"
	RolePeminjam    RoleCode = "peminjam"
)

type DivisionCode string

const (
	DivisionSekretaris DivisionCode = "sekretaris"
	DivisionBendahara  DivisionCode = "bendahara"
	DivisionK3         DivisionCode = "k3"
	DivisionLabops     DivisionCode = "labops"
	DivisionPendidikan DivisionCode = "pendidikan"
	DivisionPDD        DivisionCode = "pdd"
)

type Scope struct {
	Domain      string `json:"domain,omitempty"`
	Institution string `json:"institution,omitempty"`
	Division    string `json:"division,omitempty"`
	ClassID     string `json:"class_id,omitempty"`
	Shift       int    `json:"shift,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
}

func (s Scope) Allows(required Scope) bool {
	return allowsString(s.Domain, required.Domain) &&
		allowsString(s.Institution, required.Institution) &&
		allowsString(s.Division, required.Division) &&
		allowsString(s.ClassID, required.ClassID) &&
		(s.Shift == 0 || required.Shift == 0 || s.Shift == required.Shift) &&
		allowsString(s.SessionID, required.SessionID)
}

func allowsString(granted, required string) bool {
	return required == "" || granted == "" || granted == "*" || granted == required
}

// Contains requires every restriction of the grant to be present in the resource scope.
func (s Scope) Contains(required Scope) bool {
	contains := func(granted, value string) bool {
		return granted == "" || granted == "*" || granted == value
	}
	return contains(s.Domain, required.Domain) &&
		contains(s.Institution, required.Institution) &&
		contains(s.Division, required.Division) &&
		contains(s.ClassID, required.ClassID) &&
		(s.Shift == 0 || s.Shift == required.Shift) &&
		contains(s.SessionID, required.SessionID)
}

func (s Scope) Intersect(other Scope) (Scope, bool) {
	result := Scope{}
	intersect := func(a, b string) (string, bool) {
		if a == "" || a == "*" {
			return b, true
		}
		if b == "" || b == "*" {
			return a, true
		}
		return a, a == b
	}
	for _, field := range []struct {
		a, b string
		out  *string
	}{
		{s.Domain, other.Domain, &result.Domain},
		{s.Institution, other.Institution, &result.Institution},
		{s.Division, other.Division, &result.Division},
		{s.ClassID, other.ClassID, &result.ClassID},
		{s.SessionID, other.SessionID, &result.SessionID},
	} {
		value, ok := intersect(field.a, field.b)
		if !ok {
			return Scope{}, false
		}
		*field.out = value
	}
	if s.Shift != 0 && other.Shift != 0 && s.Shift != other.Shift {
		return Scope{}, false
	}
	result.Shift = s.Shift
	if result.Shift == 0 {
		result.Shift = other.Shift
	}
	return result, true
}

type UserProfile struct {
	ID            int64     `json:"id"`
	AuthUserID    string    `json:"auth_user_id"`
	NIM           string    `json:"nim,omitempty"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	Active        bool      `json:"is_active"`
	Hidden        bool      `json:"hidden"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	AccessVersion int64     `json:"access_version"`
}

type RoleAssignment struct {
	ID       int64        `json:"id"`
	UserID   int64        `json:"user_id"`
	Role     RoleCode     `json:"role"`
	Division DivisionCode `json:"division,omitempty"`
	Scope    Scope        `json:"scope"`
	Active   bool         `json:"active"`
}

type PermissionGrant struct {
	Permission string       `json:"permission"`
	App        string       `json:"app"`
	Role       RoleCode     `json:"role"`
	Division   DivisionCode `json:"division,omitempty"`
	Scope      Scope        `json:"scope"`
}

type Principal struct {
	User        UserProfile       `json:"user"`
	Assignments []RoleAssignment  `json:"assignments"`
	Grants      []PermissionGrant `json:"grants"`
	SessionID   string            `json:"-"`
	Host        string            `json:"-"`
}

type Session struct {
	ID                       string
	FamilyID                 string
	UserID                   int64
	Host                     string
	TokenHash                string
	CSRFTokenHash            string
	ProviderRefreshTokenSeal []byte
	ExpiresAt                time.Time
	CreatedAt                time.Time
	RevokedAt                *time.Time
}

func (s Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(now)
}

type AuthIdentity struct {
	AuthUserID      string
	Email           string
	Name            string
	ProviderRefresh string
}

type SSOHandshake struct {
	CodeHash      string
	SessionID     string
	CodeChallenge string
	ReturnTo      string
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
}

type PermissionDefinition struct {
	Code        string `json:"code"`
	App         string `json:"app"`
	Description string `json:"description"`
}

type UserAccess struct {
	Version     int64             `json:"version"`
	Assignments []RoleAssignment  `json:"assignments"`
	Grants      []PermissionGrant `json:"grants"`
}

type AuditEvent struct {
	ID          int64          `json:"id"`
	ActorID     *int64         `json:"actor_id,omitempty"`
	Action      string         `json:"action"`
	App         string         `json:"app"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	Scope       Scope          `json:"scope"`
	RequestID   string         `json:"request_id"`
	OperationID string         `json:"operation_id"`
	Detail      map[string]any `json:"detail,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}
