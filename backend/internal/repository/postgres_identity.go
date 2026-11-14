package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type identityDatabase interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type PostgresIdentityRepository struct {
	db   identityDatabase
	inTx bool
}

var _ IdentityRepository = (*PostgresIdentityRepository)(nil)

func NewPostgresIdentityRepository(pool *pgxpool.Pool) *PostgresIdentityRepository {
	return &PostgresIdentityRepository{db: pool}
}

func databaseContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}

func databaseError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503", "23514", "23502":
			return ErrConflict
		}
	}
	return err
}

func (r *PostgresIdentityRepository) Transaction(ctx context.Context, callback func(IdentityRepository) error) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := databaseContext(context.WithoutCancel(ctx))
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var coordinator string
	if err := tx.QueryRow(ctx, `select code from public.roles where code = 'koordinator' for update`).Scan(&coordinator); err != nil {
		return err
	}
	if err := callback(&PostgresIdentityRepository{db: tx, inTx: true}); err != nil {
		return err
	}
	return databaseError(tx.Commit(ctx))
}

const userColumns = `id, auth_user_id::text, coalesce(nim, ''), email, name, is_active, hidden, created_at, updated_at, access_version`

func scanUser(row pgx.Row) (entity.UserProfile, error) {
	var user entity.UserProfile
	err := row.Scan(&user.ID, &user.AuthUserID, &user.NIM, &user.Email, &user.Name, &user.Active, &user.Hidden, &user.CreatedAt, &user.UpdatedAt, &user.AccessVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.UserProfile{}, ErrIdentityNotFound
	}
	return user, databaseError(err)
}

func (r *PostgresIdentityRepository) EnsureUser(ctx context.Context, identity entity.AuthIdentity, now time.Time) (entity.UserProfile, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	email := normalizeEmail(identity.Email)
	if identity.AuthUserID == "" || email == "" {
		return entity.UserProfile{}, ErrConflict
	}
	name := strings.TrimSpace(identity.Name)
	if name == "" {
		name = email
	}
	return scanUser(r.db.QueryRow(ctx, `
		insert into public.users (auth_user_id, email, name, created_at, updated_at)
		values ($1, $2, $3, $4, $4)
		on conflict (auth_user_id) do update set
		  email = excluded.email,
		  updated_at = case when users.email is distinct from excluded.email then excluded.updated_at else users.updated_at end
		returning `+userColumns, identity.AuthUserID, email, name, now))
}

func (r *PostgresIdentityRepository) FindUserByID(ctx context.Context, id int64) (entity.UserProfile, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	return scanUser(r.db.QueryRow(ctx, `select `+userColumns+` from public.users where id = $1`, id))
}

func (r *PostgresIdentityRepository) FindUserByAuthID(ctx context.Context, authID string) (entity.UserProfile, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	return scanUser(r.db.QueryRow(ctx, `select `+userColumns+` from public.users where auth_user_id = $1`, authID))
}

func (r *PostgresIdentityRepository) SaveUser(ctx context.Context, user entity.UserProfile) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	tag, err := r.db.Exec(ctx, `
		update public.users set nim = nullif($3, ''), email = $4, name = $5,
		  is_active = $6, hidden = $7, updated_at = $8
		where id = $1 and auth_user_id = $2`, user.ID, user.AuthUserID, strings.TrimSpace(user.NIM), normalizeEmail(user.Email), strings.TrimSpace(user.Name), user.Active, user.Hidden, user.UpdatedAt)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrIdentityNotFound
	}
	return nil
}

func (r *PostgresIdentityRepository) ListUsers(ctx context.Context, limit, offset int) ([]entity.UserProfile, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `select `+userColumns+` from public.users order by id limit $1 offset $2`, pageLimit(limit), max(0, offset))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []entity.UserProfile{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresIdentityRepository) ListPermissionDefinitions(ctx context.Context) ([]entity.PermissionDefinition, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `select code, app, description from public.permissions order by code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := []entity.PermissionDefinition{}
	for rows.Next() {
		var definition entity.PermissionDefinition
		if err := rows.Scan(&definition.Code, &definition.App, &definition.Description); err != nil {
			return nil, err
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

func (r *PostgresIdentityRepository) ReadAccess(ctx context.Context, userID int64) (entity.UserAccess, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	var access entity.UserAccess
	err := r.db.QueryRow(ctx, `
		select u.access_version,
		  coalesce((select jsonb_agg(jsonb_build_object(
		    'id', a.id, 'user_id', a.user_id, 'role', a.role_code,
		    'division', coalesce(a.division_code, ''), 'scope', a.scope, 'active', a.is_active
		  ) order by a.id) from public.user_role_assignments a where a.user_id = u.id), '[]'::jsonb),
		  coalesce((select jsonb_agg(jsonb_build_object(
		    'permission', p.code, 'app', p.app, 'role', a.role_code,
		    'division', coalesce(a.division_code, ''), 'scope', g.scope
		  ) order by g.id) from public.user_permission_grants g
		    join public.user_role_assignments a on a.id = g.assignment_id and a.user_id = g.user_id
		    join public.permissions p on p.code = g.permission_code
		    where g.user_id = u.id and g.is_active), '[]'::jsonb)
		from public.users u where u.id = $1`, userID).Scan(&access.Version, &access.Assignments, &access.Grants)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.UserAccess{}, ErrIdentityNotFound
	}
	return access, err
}

func (r *PostgresIdentityRepository) ReplaceAccess(ctx context.Context, userID int64, access entity.UserAccess) error {
	if !r.inTx {
		return r.Transaction(ctx, func(tx IdentityRepository) error { return tx.ReplaceAccess(ctx, userID, access) })
	}
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	tag, err := r.db.Exec(ctx, `update public.users set access_version = access_version + 1, updated_at = now() where id = $1 and access_version = $2`, userID, access.Version)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.FindUserByID(ctx, userID); err != nil {
			return err
		}
		return ErrConflict
	}
	if _, err := r.db.Exec(ctx, `delete from public.user_role_assignments where user_id = $1`, userID); err != nil {
		return databaseError(err)
	}
	assignments := append([]entity.RoleAssignment{}, access.Assignments...)
	for i := range assignments {
		assignment := &assignments[i]
		assignment.UserID = userID
		scope, err := jsonText(assignment.Scope)
		if err != nil {
			return err
		}
		err = r.db.QueryRow(ctx, `
			insert into public.user_role_assignments (user_id, role_code, division_code, scope, is_active)
			values ($1, $2, nullif($3, ''), $4, $5) returning id`, userID, assignment.Role, assignment.Division, scope, assignment.Active).Scan(&assignment.ID)
		if err != nil {
			return databaseError(err)
		}
	}
	divisions, err := r.activeDivisions(ctx)
	if err != nil {
		return err
	}
	for _, grant := range access.Grants {
		assignmentID, matches := matchingAssignment(grant, assignments, divisions)
		if !matches {
			return ErrConflict
		}
		scope, err := jsonText(grant.Scope)
		if err != nil {
			return err
		}
		_, err = r.db.Exec(ctx, `
			insert into public.user_permission_grants (user_id, assignment_id, permission_code, scope)
			values ($1, $2, $3, $4)`, userID, assignmentID, grant.Permission, scope)
		if err != nil {
			return databaseError(err)
		}
	}
	return nil
}

func (r *PostgresIdentityRepository) activeDivisions(ctx context.Context) (map[entity.DivisionCode]bool, error) {
	rows, err := r.db.Query(ctx, `select code from public.divisions where is_active`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	divisions := make(map[entity.DivisionCode]bool)
	for rows.Next() {
		var code entity.DivisionCode
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		divisions[code] = true
	}
	return divisions, rows.Err()
}

func (r *PostgresIdentityRepository) ListAssignments(ctx context.Context, userID int64) ([]entity.RoleAssignment, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `
		select a.id, a.user_id, a.role_code, coalesce(a.division_code, ''), a.scope, a.is_active
		from public.user_role_assignments a join public.users u on u.id = a.user_id
		left join public.divisions d on d.code = a.division_code
		where a.user_id = $1 and u.is_active and a.is_active and (a.division_code is null or d.is_active)
		order by a.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assignments := []entity.RoleAssignment{}
	for rows.Next() {
		var assignment entity.RoleAssignment
		if err := rows.Scan(&assignment.ID, &assignment.UserID, &assignment.Role, &assignment.Division, &assignment.Scope, &assignment.Active); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

func (r *PostgresIdentityRepository) ListPermissionGrants(ctx context.Context, userID int64) ([]entity.PermissionGrant, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `
		select a.role_code, coalesce(a.division_code, ''), a.scope, candidate.code, candidate.app, candidate.scope
		from public.user_role_assignments a
		join public.users u on u.id = a.user_id
		left join public.divisions d on d.code = a.division_code
		cross join lateral (
		  select p.code, p.app, rp.default_scope as scope
		  from public.role_permissions rp join public.permissions p on p.code = rp.permission_code
		  where rp.role_code = a.role_code
		  union all
		  select p.code, p.app, dp.default_scope
		  from public.division_permissions dp join public.permissions p on p.code = dp.permission_code
		  where a.role_code = 'asisten' and dp.division_code = a.division_code
		  union all
		  select p.code, p.app, g.scope
		  from public.user_permission_grants g join public.permissions p on p.code = g.permission_code
		  where g.assignment_id = a.id and g.user_id = a.user_id and g.is_active
		) candidate
		where a.user_id = $1 and u.is_active and a.is_active and (a.division_code is null or d.is_active)
		order by a.id, candidate.code`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []entity.PermissionGrant{}
	for rows.Next() {
		var assignment entity.RoleAssignment
		var candidate entity.PermissionGrant
		if err := rows.Scan(&assignment.Role, &assignment.Division, &assignment.Scope, &candidate.Permission, &candidate.App, &candidate.Scope); err != nil {
			return nil, err
		}
		if grant, ok := effectiveGrant(assignment, candidate); ok {
			grants = append(grants, grant)
		}
	}
	return uniqueGrants(grants), rows.Err()
}

const sessionColumns = `id, family_id, user_id, host, token_hash, csrf_token_hash, provider_refresh_token_seal, created_at, expires_at, revoked_at`

func scanSession(row pgx.Row) (entity.Session, error) {
	var session entity.Session
	err := row.Scan(&session.ID, &session.FamilyID, &session.UserID, &session.Host, &session.TokenHash, &session.CSRFTokenHash, &session.ProviderRefreshTokenSeal, &session.CreatedAt, &session.ExpiresAt, &session.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Session{}, ErrSessionNotFound
	}
	return session, databaseError(err)
}

func (r *PostgresIdentityRepository) SaveSession(ctx context.Context, session entity.Session) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if session.FamilyID == "" {
		session.FamilyID = session.ID
	}
	var id string
	err := r.db.QueryRow(ctx, `
		insert into public.auth_sessions
		  (id, family_id, user_id, host, token_hash, csrf_token_hash, provider_refresh_token_seal, created_at, expires_at, revoked_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		on conflict (id) do update set
		  token_hash = excluded.token_hash, csrf_token_hash = excluded.csrf_token_hash,
		  provider_refresh_token_seal = excluded.provider_refresh_token_seal,
		  expires_at = least(auth_sessions.expires_at, excluded.expires_at), revoked_at = excluded.revoked_at
		where auth_sessions.revoked_at is null and auth_sessions.user_id = excluded.user_id
		  and auth_sessions.family_id = excluded.family_id and auth_sessions.host = excluded.host
		returning id`, session.ID, session.FamilyID, session.UserID, session.Host, session.TokenHash, session.CSRFTokenHash, session.ProviderRefreshTokenSeal, session.CreatedAt, session.ExpiresAt, session.RevokedAt).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	return databaseError(err)
}

func (r *PostgresIdentityRepository) FindSessionByID(ctx context.Context, id string) (entity.Session, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	query := `select ` + sessionColumns + ` from public.auth_sessions where id = $1`
	if r.inTx {
		query += ` for update`
	}
	return scanSession(r.db.QueryRow(ctx, query, id))
}

func (r *PostgresIdentityRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (entity.Session, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	query := `select ` + sessionColumns + ` from public.auth_sessions where token_hash = $1`
	if r.inTx {
		query += ` for update`
	}
	return scanSession(r.db.QueryRow(ctx, query, tokenHash))
}

func (r *PostgresIdentityRepository) ListSessions(ctx context.Context, userID int64, _ time.Time) ([]entity.Session, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `select `+sessionColumns+` from public.auth_sessions where user_id = $1 order by created_at desc, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []entity.Session{}
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *PostgresIdentityRepository) RevokeSession(ctx context.Context, id string, at time.Time) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	tag, err := r.db.Exec(ctx, `update public.auth_sessions set revoked_at = coalesce(revoked_at, $2) where id = $1 or family_id = $1`, id, at)
	if err != nil {
		return databaseError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (r *PostgresIdentityRepository) SaveHandshake(ctx context.Context, handshake entity.SSOHandshake) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	var hash string
	err := r.db.QueryRow(ctx, `
		insert into public.sso_handshakes (code_hash, session_id, code_challenge, return_to, expires_at, consumed_at)
		select $1, issuer.id, $3, $4, $5, $6 from public.auth_sessions issuer
		join public.auth_sessions root on root.id = issuer.family_id and root.user_id = issuer.user_id
		where issuer.id = $2 and issuer.revoked_at is null and root.revoked_at is null and root.family_id = root.id
		returning code_hash`, handshake.CodeHash, handshake.SessionID, handshake.CodeChallenge, handshake.ReturnTo, handshake.ExpiresAt, handshake.ConsumedAt).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrConflict
	}
	return databaseError(err)
}

func (r *PostgresIdentityRepository) ConsumeHandshake(ctx context.Context, codeHash string, now time.Time) (entity.SSOHandshake, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	var handshake entity.SSOHandshake
	err := r.db.QueryRow(ctx, `
		update public.sso_handshakes set consumed_at = $2
		where code_hash = $1 and consumed_at is null and expires_at > $2
		returning code_hash, session_id, code_challenge, return_to, expires_at, consumed_at`, codeHash, now).Scan(&handshake.CodeHash, &handshake.SessionID, &handshake.CodeChallenge, &handshake.ReturnTo, &handshake.ExpiresAt, &handshake.ConsumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.SSOHandshake{}, ErrHandshakeInvalid
	}
	return handshake, databaseError(err)
}

func (r *PostgresIdentityRepository) TakeRateLimit(ctx context.Context, key string, windowStart time.Time, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	if key == "" || windowStart.IsZero() || limit > 2147483647 {
		return false, ErrConflict
	}
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	windowStart = windowStart.UTC().Truncate(time.Microsecond)
	_, err := r.db.Exec(ctx, `
		delete from public.auth_rate_limits where (key_hash, window_start) in (
		  select key_hash, window_start from public.auth_rate_limits where window_start < $1
		  order by window_start limit 128 for update skip locked
		)`, windowStart.Add(-24*time.Hour))
	if err != nil {
		return false, err
	}
	var hits int
	err = r.db.QueryRow(ctx, `
		insert into public.auth_rate_limits (key_hash, window_start, hits) values ($1, $2, 1)
		on conflict (key_hash, window_start) do update set hits = auth_rate_limits.hits + 1
		where auth_rate_limits.hits < $3
		returning hits`, hashRateKey(key), windowStart, int32(limit)).Scan(&hits)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func jsonText(value any) (string, error) {
	data, err := json.Marshal(value)
	return string(data), err
}

func (r *PostgresIdentityRepository) AppendAudit(ctx context.Context, event entity.AuditEvent) error {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	if event.Detail == nil {
		event.Detail = map[string]any{}
	}
	detail, err := jsonText(event.Detail)
	if err != nil {
		return err
	}
	scope, err := jsonText(event.Scope)
	if err != nil {
		return err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err = r.db.Exec(ctx, `
		insert into public.audit_logs
		  (actor_id, action, app, subject_type, subject_id, scope, request_id, operation_id, detail, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, event.ActorID, event.Action, event.App, event.SubjectType, event.SubjectID, scope, event.RequestID, event.OperationID, detail, event.CreatedAt)
	return databaseError(err)
}

func (r *PostgresIdentityRepository) ListAudit(ctx context.Context, limit int) ([]entity.AuditEvent, error) {
	return r.ListAuditPage(ctx, 0, limit)
}

func (r *PostgresIdentityRepository) ListAuditPage(ctx context.Context, beforeID int64, limit int) ([]entity.AuditEvent, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()
	rows, err := r.db.Query(ctx, `
		select id, actor_id, action, app, subject_type, subject_id, scope, request_id, operation_id, detail, created_at
		from public.audit_logs where ($1::bigint <= 0 or id < $1) order by id desc limit $2`, beforeID, pageLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []entity.AuditEvent{}
	for rows.Next() {
		var event entity.AuditEvent
		if err := rows.Scan(&event.ID, &event.ActorID, &event.Action, &event.App, &event.SubjectType, &event.SubjectID, &event.Scope, &event.RequestID, &event.OperationID, &event.Detail, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
