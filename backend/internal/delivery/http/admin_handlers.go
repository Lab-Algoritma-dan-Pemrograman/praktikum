package httpdelivery

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func (h authHandlers) adminSession(w http.ResponseWriter, r *http.Request, path string, mutation bool) (*entity.Principal, bool) {
	principal, _, ok := h.session(w, r, mutation)
	if !ok {
		return nil, false
	}
	if err := h.identity.CanOpenAdmin(principal, path); err != nil {
		h.deny(w, r, principal, err)
		return nil, false
	}
	return principal, true
}

func (h authHandlers) adminAccess(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	principal, ok := h.adminSession(w, r, path, false)
	if ok {
		writeSuccess(w, http.StatusOK, principal)
	}
}

func (h authHandlers) adminOverview(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.adminSession(w, r, "/", false)
	if !ok {
		return
	}
	domains := map[string]bool{}
	for _, domain := range []string{"praktikum", "elearning", "siakad"} {
		if h.identity.CanOpenAdmin(principal, "/"+domain) == nil {
			domains[domain] = true
		}
	}
	writeSuccess(w, http.StatusOK, map[string]any{"user": principal.User, "domains": domains})
}

func (h authHandlers) permissions(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.adminSession(w, r, "/", false)
	if ok {
		writeSuccess(w, http.StatusOK, map[string]any{"assignments": principal.Assignments, "grants": principal.Grants})
	}
}

func (h authHandlers) auditLog(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.adminSession(w, r, "/auditlog", false)
	if !ok {
		return
	}
	limit, err := boundedQuery(r, "limit", 50, 1, 100)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	before := int64(0)
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			writeAuthError(w, usecase.ErrValidation)
			return
		}
	}
	items, next, err := h.identity.AuditPage(r.Context(), principal, before, limit)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "meta": map[string]any{"request_id": requestID(r), "operation_id": "admin.auditlog", "next_before": next}})
}

func (h authHandlers) permissionCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminSession(w, r, "/pengaturan", false); !ok {
		return
	}
	items, err := h.repository.ListPermissionDefinitions(r.Context())
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, items)
}

func (h authHandlers) users(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminSession(w, r, "/pengaturan", false); !ok {
		return
	}
	limit, err := boundedQuery(r, "limit", 25, 1, 100)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	offset, err := boundedQuery(r, "offset", 0, 0, 1000000)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	items, err := h.repository.ListUsers(r.Context(), limit, offset)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, items)
}

func (h authHandlers) userAccess(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.adminSession(w, r, "/pengaturan", r.Method == http.MethodPut)
	if !ok {
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/admin/users/"), "/")
	if len(parts) != 2 || parts[1] != "access" {
		writeAuthError(w, authorization.ErrNotFound)
		return
	}
	userID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || userID < 1 {
		writeAuthError(w, authorization.ErrNotFound)
		return
	}
	if r.Method == http.MethodGet {
		access, err := h.repository.ReadAccess(r.Context(), userID)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		writeSuccess(w, http.StatusOK, access)
		return
	}
	var input struct {
		Version     int64 `json:"version"`
		Assignments []struct {
			Role     entity.RoleCode     `json:"role"`
			Division entity.DivisionCode `json:"division"`
			Scope    *entity.Scope       `json:"scope"`
		} `json:"assignments"`
		Grants []struct {
			Permission string              `json:"permission"`
			Role       entity.RoleCode     `json:"role"`
			Division   entity.DivisionCode `json:"division"`
			Scope      *entity.Scope       `json:"scope"`
		} `json:"grants"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeAuthError(w, err)
		return
	}
	if input.Assignments == nil || input.Grants == nil {
		writeAuthError(w, usecase.ErrValidation)
		return
	}
	access := entity.UserAccess{Version: input.Version, Assignments: []entity.RoleAssignment{}, Grants: []entity.PermissionGrant{}}
	for _, assignment := range input.Assignments {
		if assignment.Scope == nil {
			writeAuthError(w, usecase.ErrValidation)
			return
		}
		access.Assignments = append(access.Assignments, entity.RoleAssignment{Role: assignment.Role, Division: assignment.Division, Scope: *assignment.Scope, Active: true})
	}
	for _, grant := range input.Grants {
		if grant.Scope == nil {
			writeAuthError(w, usecase.ErrValidation)
			return
		}
		access.Grants = append(access.Grants, entity.PermissionGrant{Permission: grant.Permission, Role: grant.Role, Division: grant.Division, Scope: *grant.Scope})
	}
	updated, err := h.identity.ChangeAccess(r.Context(), principal, userID, access, requestID(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeSuccess(w, http.StatusOK, updated)
}

func boundedQuery(r *http.Request, key string, fallback, min, max int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < min || value > max {
		return 0, usecase.ErrValidation
	}
	return value, nil
}
