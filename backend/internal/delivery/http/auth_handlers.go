package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/middleware"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/observability"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type authHandlers struct {
	identity   usecase.IdentityService
	config     config.Config
	repository repository.IdentityRepository
}

func (h authHandlers) endpoint(method, operation string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Operation-ID", operation)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Pragma", "no-cache")
		r = r.WithContext(observability.WithOperation(r.Context(), operation))
		allowed := false
		for _, candidate := range strings.Split(method, ",") {
			if r.Method == strings.TrimSpace(candidate) {
				allowed = true
			}
		}
		if !allowed {
			w.Header().Set("Allow", method)
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method tidak didukung.")
			return
		}
		if h.repository == nil {
			writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity belum dikonfigurasi.")
			return
		}
		if r.Method != http.MethodGet && h.config.Environment != "local" && r.Header.Get("Origin") == "" {
			writeError(w, http.StatusForbidden, "FORBIDDEN_ORIGIN", "Origin wajib diisi.")
			return
		}
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		limit := h.config.UserRateLimit
		if strings.HasPrefix(operation, "auth.") {
			limit = h.config.AuthRateLimit
		}
		if !h.rateLimit(w, r, "ip:"+ip+":"+operation, limit) {
			return
		}
		handler(w, r)
	}
}

func (h authHandlers) rateLimit(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	if limit <= 0 {
		limit = 120
	}
	now := time.Now().UTC()
	allowed, err := h.repository.TakeRateLimit(r.Context(), security.Hash(key), now.Truncate(time.Minute), limit)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Layanan identity sedang tidak tersedia.")
		return false
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(60-now.Second()))
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan. Coba lagi nanti.")
		return false
	}
	return true
}

func requestHost(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		if parsed, err := url.Parse(origin); err == nil {
			return parsed.Host
		}
		return ""
	}
	return r.Host
}

func (h authHandlers) password(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	cookies, principal, err := h.identity.PasswordLogin(r.Context(), input.Email, input.Password, requestHost(r), requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookies(w, cookies, h.config.CookieSecure)
	writeSuccess(w, http.StatusOK, map[string]any{"user": principal.User})
}

func (h authHandlers) callback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code     string `json:"code"`
		Verifier string `json:"code_verifier"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	cookies, principal, err := h.identity.Callback(r.Context(), input.Code, input.Verifier, requestHost(r), requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookies(w, cookies, h.config.CookieSecure)
	writeSuccess(w, http.StatusOK, map[string]any{"user": principal.User})
}

func (h authHandlers) prepare(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := h.session(w, r, true)
	if !ok {
		return
	}
	var input struct {
		ReturnTo  string `json:"return_to"`
		Challenge string `json:"code_challenge"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	code, err := h.identity.PrepareHandshake(r.Context(), principal, input.ReturnTo, input.Challenge, requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, map[string]any{"code": code, "expires_in": 120})
}

func (h authHandlers) exchange(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code     string `json:"code"`
		ReturnTo string `json:"return_to"`
		Verifier string `json:"code_verifier"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	cookies, principal, err := h.identity.ExchangeHandshake(r.Context(), input.Code, input.ReturnTo, input.Verifier, requestHost(r), requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookies(w, cookies, h.config.CookieSecure)
	writeSuccess(w, http.StatusOK, map[string]any{"user": principal.User, "return_to": input.ReturnTo})
}

func (h authHandlers) me(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := h.session(w, r, false)
	if ok {
		writeSuccess(w, http.StatusOK, principal)
	}
}

func (h authHandlers) refresh(w http.ResponseWriter, r *http.Request) {
	principal, session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	cookies, err := h.identity.Refresh(r.Context(), principal, session, requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookies(w, cookies, h.config.CookieSecure)
	writeSuccess(w, http.StatusOK, map[string]any{"user": principal.User})
}

func (h authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	principal, session, ok := h.session(w, r, true)
	if !ok {
		return
	}
	if err := h.identity.Logout(r.Context(), principal, session, requestID(r)); err != nil {
		writeAuthError(w, err)
		return
	}
	clearSessionCookies(w, h.config.CookieSecure)
	writeSuccess(w, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h authHandlers) sessions(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := h.session(w, r, false)
	if !ok {
		return
	}
	items, err := h.identity.Sessions(r.Context(), principal)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, items)
}

func (h authHandlers) revokeSession(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := h.session(w, r, true)
	if !ok {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/auth/sessions/")
	if id == "" || len(id) > 100 || strings.ContainsAny(id, "/?#") {
		writeAuthError(w, authorization.ErrNotFound)
		return
	}
	if err := h.identity.Revoke(r.Context(), principal, id, requestID(r)); err != nil {
		writeAuthError(w, err)
		return
	}
	if id == principal.SessionID {
		clearSessionCookies(w, h.config.CookieSecure)
	}
	writeSuccess(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (h authHandlers) profile(w http.ResponseWriter, r *http.Request) {
	principal, _, ok := h.session(w, r, r.Method == http.MethodPatch)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		writeSuccess(w, http.StatusOK, principal.User)
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	updated, err := h.identity.UpdateProfile(r.Context(), principal, input.Name, requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, updated.User)
}

func (h authHandlers) session(w http.ResponseWriter, r *http.Request, mutation bool) (*entity.Principal, entity.Session, bool) {
	cookie, err := r.Cookie(usecase.SessionCookieName)
	if err != nil {
		writeAuthError(w, usecase.ErrInvalidSession)
		return nil, entity.Session{}, false
	}
	principal, session, err := h.identity.Authenticate(r.Context(), cookie.Value, requestHost(r))
	if err != nil {
		writeAuthError(w, err)
		return nil, entity.Session{}, false
	}
	if mutation {
		if err := h.identity.CheckCSRF(session, r.Header.Get("X-CSRF-Token"), csrfCookie(r)); err != nil {
			writeAuthError(w, err)
			return nil, entity.Session{}, false
		}
	}
	if !h.rateLimit(w, r, "user:"+strconv.FormatInt(principal.User.ID, 10), h.config.UserRateLimit) {
		return nil, entity.Session{}, false
	}
	return principal, session, true
}

func (h authHandlers) deny(w http.ResponseWriter, r *http.Request, principal *entity.Principal, err error) {
	if err == nil {
		return
	}
	var actor *int64
	if principal != nil {
		actor = &principal.User.ID
	}
	op := observability.Operation(r.Context())
	if auditErr := h.repository.AppendAudit(r.Context(), entity.AuditEvent{ActorID: actor, Action: "authorization.denied", App: "admin", SubjectType: "route", SubjectID: r.URL.Path, RequestID: requestID(r), OperationID: op, CreatedAt: time.Now().UTC()}); auditErr != nil {
		writeAuthError(w, auditErr)
		return
	}
	writeAuthError(w, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return usecase.ErrValidation
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return jsonError(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return jsonError(err)
	}
	return nil
}

func jsonError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return err
	}
	return usecase.ErrValidation
}

func setSessionCookies(w http.ResponseWriter, cookies usecase.SessionCookies, secure bool) {
	http.SetCookie(w, usecase.Cookie(usecase.SessionCookieName, cookies.Session, cookies.MaxAge, secure))
	http.SetCookie(w, usecase.Cookie(usecase.CSRFCookieName, cookies.CSRF, cookies.MaxAge, secure))
}

func clearSessionCookies(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, usecase.Cookie(usecase.SessionCookieName, "", -1, secure))
	http.SetCookie(w, usecase.Cookie(usecase.CSRFCookieName, "", -1, secure))
}

func csrfCookie(r *http.Request) string {
	cookie, err := r.Cookie(usecase.CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func requestID(r *http.Request) string { return middleware.GetRequestID(r.Context()) }

func writeAuthError(w http.ResponseWriter, err error) {
	var policy *authorization.PolicyError
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &policy):
		writeError(w, policy.Status, policy.Code, policy.Message)
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "Payload terlalu besar.")
	case errors.Is(err, authorization.ErrUnauthenticated), errors.Is(err, usecase.ErrInvalidSession), errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidAuthCode), errors.Is(err, repository.ErrHandshakeInvalid):
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sesi atau credential tidak valid.")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Anda tidak memiliki akses.")
	case errors.Is(err, authorization.ErrNotFound), errors.Is(err, repository.ErrIdentityNotFound), errors.Is(err, repository.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Resource tidak ditemukan.")
	case errors.Is(err, repository.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", "Data berubah atau identity sudah digunakan. Muat ulang sebelum mencoba lagi.")
	case errors.Is(err, usecase.ErrCSRF):
		writeError(w, http.StatusForbidden, "CSRF_INVALID", "CSRF token tidak valid.")
	case errors.Is(err, returnto.ErrNotAllowed), errors.Is(err, usecase.ErrValidation):
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Payload atau tujuan tidak valid.")
	case errors.Is(err, usecase.ErrSessionLimit):
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "SESSION_LIMIT", "Terlalu banyak sesi aktif. Cabut sesi lama dahulu.")
	case errors.Is(err, auth.ErrProviderNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "AUTH_PROVIDER_NOT_CONFIGURED", "Identity provider belum dikonfigurasi.")
	case errors.Is(err, auth.ErrProviderUnavailable), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Identity provider sedang tidak tersedia.")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Permintaan identity gagal.")
	}
}
