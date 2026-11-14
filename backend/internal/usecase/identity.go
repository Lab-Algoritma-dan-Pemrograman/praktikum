package usecase

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/observability"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

const (
	SessionCookieName = "algohub_session"
	CSRFCookieName    = "algohub_csrf"
)

var (
	ErrCSRF           = errors.New("CSRF_INVALID")
	ErrInvalidSession = errors.New("UNAUTHENTICATED")
	ErrValidation     = errors.New("VALIDATION_ERROR")
	ErrSessionLimit   = errors.New("SESSION_LIMIT")
	pkceVerifier      = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
)

type SessionCookies struct {
	Session string
	CSRF    string
	MaxAge  int
}

type SessionView struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

type IdentityService struct {
	repository repository.IdentityRepository
	provider   auth.IdentityProvider
	allowlist  returnto.Allowlist
	authorizer authorization.Service
	now        func() time.Time
	sessionTTL time.Duration
}

type IdentityOptions struct {
	Repository repository.IdentityRepository
	Provider   auth.IdentityProvider
	Allowlist  returnto.Allowlist
	SessionTTL time.Duration
	Now        func() time.Time
}

func NewIdentityService(options IdentityOptions) IdentityService {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.SessionTTL <= 0 {
		options.SessionTTL = 12 * time.Hour
	}
	if options.Provider == nil {
		options.Provider = auth.UnconfiguredProvider{}
	}
	return IdentityService{repository: options.Repository, provider: options.Provider, allowlist: options.Allowlist, now: options.Now, sessionTTL: options.SessionTTL}
}

func (s IdentityService) transaction(ctx context.Context, fn func(IdentityService) error) error {
	return s.repository.Transaction(ctx, func(repo repository.IdentityRepository) error {
		s.repository = repo
		return fn(s)
	})
}

func (s IdentityService) Callback(ctx context.Context, code, verifier, host, requestID string) (SessionCookies, *entity.Principal, error) {
	if code == "" || len(code) > 4096 || !pkceVerifier.MatchString(verifier) {
		return SessionCookies{}, nil, ErrValidation
	}
	identity, err := s.provider.ExchangeCode(ctx, code, verifier)
	if err != nil {
		return SessionCookies{}, nil, err
	}
	return s.createSession(ctx, identity, host, requestID)
}

func (s IdentityService) PasswordLogin(ctx context.Context, email, password, host, requestID string) (SessionCookies, *entity.Principal, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 320 || !strings.Contains(email, "@") || len(password) == 0 || len(password) > 1024 {
		return SessionCookies{}, nil, ErrValidation
	}
	identity, err := s.provider.PasswordLogin(ctx, email, password)
	if err != nil {
		return SessionCookies{}, nil, err
	}
	return s.createSession(ctx, identity, host, requestID)
}

func (s IdentityService) createSession(ctx context.Context, identity auth.Identity, host, requestID string) (cookies SessionCookies, principal *entity.Principal, err error) {
	if identity.AuthUserID == "" || identity.Email == "" || host == "" {
		return cookies, nil, ErrInvalidSession
	}
	identity.Name = cleanName(identity.Name)
	if identity.Name == "" {
		identity.Name = strings.Split(identity.Email, "@")[0]
	}
	err = s.transaction(ctx, func(tx IdentityService) error {
		user, e := tx.repository.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: identity.AuthUserID, Email: strings.ToLower(identity.Email), Name: identity.Name}, tx.now())
		if e != nil {
			return e
		}
		if !user.Active {
			return authorization.ErrForbidden
		}
		cookies, principal, e = tx.issueSession(ctx, user, host, "", tx.now().Add(tx.sessionTTL), "login", requestID)
		return e
	})
	return
}

func (s IdentityService) issueSession(ctx context.Context, user entity.UserProfile, host, familyID string, expires time.Time, action, requestID string) (SessionCookies, *entity.Principal, error) {
	sessions, err := s.repository.ListSessions(ctx, user.ID, s.now())
	if err != nil {
		return SessionCookies{}, nil, err
	}
	active := 0
	for _, session := range sessions {
		if session.Active(s.now()) {
			active++
		}
	}
	if active >= 100 {
		return SessionCookies{}, nil, ErrSessionLimit
	}
	id, err := security.RandomToken("sid_")
	if err != nil {
		return SessionCookies{}, nil, err
	}
	if familyID == "" {
		familyID = id
	}
	cookies, err := freshCookies(expires, s.now())
	if err != nil {
		return SessionCookies{}, nil, err
	}
	session := entity.Session{ID: id, FamilyID: familyID, UserID: user.ID, Host: host, TokenHash: security.Hash(cookies.Session), CSRFTokenHash: security.Hash(cookies.CSRF), CreatedAt: s.now(), ExpiresAt: expires}
	if err = s.repository.SaveSession(ctx, session); err != nil {
		return SessionCookies{}, nil, err
	}
	principal, err := s.principal(ctx, user, session.ID, host)
	if err != nil {
		return SessionCookies{}, nil, err
	}
	if err = s.audit(ctx, &user.ID, action, "auth", "session", session.ID, entity.Scope{}, requestID, nil); err != nil {
		return SessionCookies{}, nil, err
	}
	return cookies, &principal, nil
}

func freshCookies(expires, now time.Time) (SessionCookies, error) {
	token, err := security.RandomToken("sess_")
	if err != nil {
		return SessionCookies{}, err
	}
	csrf, err := security.RandomToken("csrf_")
	if err != nil {
		return SessionCookies{}, err
	}
	maxAge := int(expires.Sub(now).Seconds())
	if maxAge < 1 {
		return SessionCookies{}, ErrInvalidSession
	}
	return SessionCookies{Session: token, CSRF: csrf, MaxAge: maxAge}, nil
}

func (s IdentityService) PrepareHandshake(ctx context.Context, principal *entity.Principal, returnTo, challenge, requestID string) (code string, err error) {
	if principal == nil {
		return "", ErrInvalidSession
	}
	if _, err = s.allowlist.Validate(returnTo); err != nil {
		return "", err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil || len(decoded) != sha256.Size || len(challenge) != 43 {
		return "", ErrValidation
	}
	code, err = security.RandomToken("sso_")
	if err != nil {
		return "", err
	}
	err = s.transaction(ctx, func(tx IdentityService) error {
		session, e := tx.activeSession(ctx, principal.SessionID, principal.User.ID)
		if e != nil {
			return e
		}
		return tx.repository.SaveHandshake(ctx, entity.SSOHandshake{CodeHash: security.Hash(code), SessionID: session.ID, CodeChallenge: challenge, ReturnTo: returnTo, ExpiresAt: tx.now().Add(2 * time.Minute)})
	})
	return
}

func (s IdentityService) ExchangeHandshake(ctx context.Context, code, returnTo, verifier, host, requestID string) (cookies SessionCookies, principal *entity.Principal, err error) {
	target, err := s.allowlist.Validate(returnTo)
	if err != nil {
		return cookies, nil, err
	}
	if target.Host != host || !pkceVerifier.MatchString(verifier) || len(code) > 256 || !strings.HasPrefix(code, "sso_") {
		return cookies, nil, repository.ErrHandshakeInvalid
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	err = s.transaction(ctx, func(tx IdentityService) error {
		handshake, e := tx.repository.ConsumeHandshake(ctx, security.Hash(code), tx.now())
		if e != nil {
			return e
		}
		if handshake.ReturnTo != returnTo || subtle.ConstantTimeCompare([]byte(handshake.CodeChallenge), []byte(challenge)) != 1 {
			return repository.ErrHandshakeInvalid
		}
		issuer, e := tx.repository.FindSessionByID(ctx, handshake.SessionID)
		if e != nil {
			return e
		}
		issuer, e = tx.activeSession(ctx, issuer.ID, issuer.UserID)
		if e != nil {
			return e
		}
		user, e := tx.repository.FindUserByID(ctx, issuer.UserID)
		if e != nil {
			return e
		}
		if !user.Active {
			return authorization.ErrForbidden
		}
		cookies, principal, e = tx.issueSession(ctx, user, host, issuer.FamilyID, issuer.ExpiresAt, "sso.exchange", requestID)
		return e
	})
	return
}

func (s IdentityService) activeSession(ctx context.Context, id string, userID int64) (entity.Session, error) {
	session, err := s.repository.FindSessionByID(ctx, id)
	if err != nil {
		return entity.Session{}, sessionError(err)
	}
	if session.UserID != userID || !session.Active(s.now()) || session.FamilyID == "" {
		return entity.Session{}, ErrInvalidSession
	}
	if session.FamilyID != session.ID {
		root, err := s.repository.FindSessionByID(ctx, session.FamilyID)
		if err != nil {
			return entity.Session{}, sessionError(err)
		}
		if root.UserID != session.UserID || !root.Active(s.now()) || root.FamilyID != root.ID {
			return entity.Session{}, ErrInvalidSession
		}
	}
	return session, nil
}

func sessionError(err error) error {
	if errors.Is(err, repository.ErrSessionNotFound) || errors.Is(err, repository.ErrIdentityNotFound) {
		return ErrInvalidSession
	}
	return err
}

func (s IdentityService) Authenticate(ctx context.Context, rawToken, host string) (*entity.Principal, entity.Session, error) {
	if rawToken == "" || len(rawToken) > 256 || host == "" {
		return nil, entity.Session{}, ErrInvalidSession
	}
	session, err := s.repository.FindSessionByTokenHash(ctx, security.Hash(rawToken))
	if err != nil {
		return nil, entity.Session{}, sessionError(err)
	}
	if session.Host != host {
		return nil, entity.Session{}, ErrInvalidSession
	}
	session, err = s.activeSession(ctx, session.ID, session.UserID)
	if err != nil {
		return nil, entity.Session{}, err
	}
	user, err := s.repository.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, entity.Session{}, sessionError(err)
	}
	if !user.Active {
		return nil, entity.Session{}, authorization.ErrForbidden
	}
	principal, err := s.principal(ctx, user, session.ID, session.Host)
	if err != nil {
		return nil, entity.Session{}, err
	}
	return &principal, session, nil
}

func (s IdentityService) CheckCSRF(session entity.Session, headerToken, cookieToken string) error {
	if headerToken == "" || cookieToken == "" || len(headerToken) > 256 || len(cookieToken) > 256 || subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) != 1 || security.Hash(cookieToken) != session.CSRFTokenHash {
		return ErrCSRF
	}
	return nil
}

func (s IdentityService) Refresh(ctx context.Context, principal *entity.Principal, previous entity.Session, requestID string) (cookies SessionCookies, err error) {
	if principal == nil {
		return cookies, ErrInvalidSession
	}
	err = s.transaction(ctx, func(tx IdentityService) error {
		session, e := tx.repository.FindSessionByTokenHash(ctx, previous.TokenHash)
		if e != nil {
			return sessionError(e)
		}
		if session.ID != principal.SessionID {
			return ErrInvalidSession
		}
		session, e = tx.activeSession(ctx, session.ID, principal.User.ID)
		if e != nil {
			return e
		}
		cookies, e = freshCookies(session.ExpiresAt, tx.now())
		if e != nil {
			return e
		}
		session.TokenHash, session.CSRFTokenHash = security.Hash(cookies.Session), security.Hash(cookies.CSRF)
		if e = tx.repository.SaveSession(ctx, session); e != nil {
			return e
		}
		return tx.audit(ctx, &principal.User.ID, "session.refresh", "auth", "session", session.ID, entity.Scope{}, requestID, nil)
	})
	return
}

func (s IdentityService) Logout(ctx context.Context, principal *entity.Principal, session entity.Session, requestID string) error {
	if principal == nil {
		return ErrInvalidSession
	}
	return s.transaction(ctx, func(tx IdentityService) error {
		current, err := tx.activeSession(ctx, session.ID, principal.User.ID)
		if err != nil {
			return err
		}
		if err = tx.repository.RevokeSession(ctx, current.FamilyID, tx.now()); err != nil {
			return err
		}
		return tx.audit(ctx, &principal.User.ID, "logout", "auth", "session", current.FamilyID, entity.Scope{}, requestID, nil)
	})
}

func (s IdentityService) Revoke(ctx context.Context, principal *entity.Principal, sessionID, requestID string) error {
	if principal == nil {
		return ErrInvalidSession
	}
	return s.transaction(ctx, func(tx IdentityService) error {
		if _, err := tx.activeSession(ctx, principal.SessionID, principal.User.ID); err != nil {
			return err
		}
		target, err := tx.repository.FindSessionByID(ctx, sessionID)
		if errors.Is(err, repository.ErrSessionNotFound) || (err == nil && target.UserID != principal.User.ID) {
			return authorization.ErrNotFound
		}
		if err != nil {
			return err
		}
		if target.RevokedAt != nil {
			return nil
		}
		if err = tx.repository.RevokeSession(ctx, target.ID, tx.now()); err != nil {
			return err
		}
		return tx.audit(ctx, &principal.User.ID, "session.revoke", "auth", "session", target.ID, entity.Scope{}, requestID, nil)
	})
}

func (s IdentityService) Sessions(ctx context.Context, principal *entity.Principal) ([]SessionView, error) {
	if principal == nil {
		return nil, ErrInvalidSession
	}
	sessions, err := s.repository.ListSessions(ctx, principal.User.ID, s.now())
	if err != nil {
		return nil, err
	}
	roots := make(map[string]bool)
	for _, session := range sessions {
		if session.FamilyID == session.ID {
			roots[session.ID] = session.Active(s.now())
		}
	}
	result := make([]SessionView, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, SessionView{ID: session.ID, Host: session.Host, CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt, Revoked: session.RevokedAt != nil || !roots[session.FamilyID]})
	}
	return result, nil
}

func cleanName(name string) string {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 160 {
		return ""
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return name
}

func (s IdentityService) UpdateProfile(ctx context.Context, principal *entity.Principal, name, requestID string) (*entity.Principal, error) {
	if principal == nil {
		return nil, ErrInvalidSession
	}
	name = cleanName(name)
	if name == "" {
		return nil, ErrValidation
	}
	var updated entity.Principal
	err := s.transaction(ctx, func(tx IdentityService) error {
		if _, e := tx.activeSession(ctx, principal.SessionID, principal.User.ID); e != nil {
			return e
		}
		user, e := tx.repository.FindUserByID(ctx, principal.User.ID)
		if e != nil {
			return e
		}
		if !user.Active {
			return authorization.ErrForbidden
		}
		before := user.Name
		user.Name, user.UpdatedAt = name, tx.now()
		if e = tx.repository.SaveUser(ctx, user); e != nil {
			return e
		}
		if e = tx.audit(ctx, &user.ID, "profile.update", "identity", "user", fmt.Sprint(user.ID), entity.Scope{}, requestID, map[string]any{"before": map[string]string{"name": before}, "after": map[string]string{"name": name}}); e != nil {
			return e
		}
		updated, e = tx.principal(ctx, user, principal.SessionID, principal.Host)
		return e
	})
	return &updated, err
}

func (s IdentityService) principal(ctx context.Context, user entity.UserProfile, sessionID, host string) (entity.Principal, error) {
	assignments, err := s.repository.ListAssignments(ctx, user.ID)
	if err != nil {
		return entity.Principal{}, err
	}
	grants, err := s.repository.ListPermissionGrants(ctx, user.ID)
	if err != nil {
		return entity.Principal{}, err
	}
	if assignments == nil {
		assignments = []entity.RoleAssignment{}
	}
	if grants == nil {
		grants = []entity.PermissionGrant{}
	}
	return entity.Principal{User: user, Assignments: assignments, Grants: grants, SessionID: sessionID, Host: host}, nil
}

func (s IdentityService) audit(ctx context.Context, actor *int64, action, app, subjectType, subjectID string, scope entity.Scope, requestID string, detail map[string]any) error {
	operation := observability.Operation(ctx)
	if operation == "" {
		operation = action
	}
	return s.repository.AppendAudit(ctx, entity.AuditEvent{ActorID: actor, Action: action, App: app, SubjectType: subjectType, SubjectID: subjectID, Scope: scope, RequestID: requestID, OperationID: operation, Detail: detail, CreatedAt: s.now()})
}

func Cookie(name, value string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, Secure: secure, HttpOnly: name == SessionCookieName, SameSite: http.SameSiteLaxMode}
}
