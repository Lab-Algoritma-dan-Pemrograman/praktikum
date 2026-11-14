package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func postgresTestDatabase(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("ALGOHUB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ALGOHUB_TEST_DATABASE_URL is not configured")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid ALGOHUB_TEST_DATABASE_URL")
	}
	hosts := []string{config.ConnConfig.Host}
	for _, fallback := range config.ConnConfig.Fallbacks {
		hosts = append(hosts, fallback.Host)
	}
	for _, host := range hosts {
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			t.Fatal("integration tests require an isolated loopback PostgreSQL database")
		}
	}
	if !strings.HasSuffix(config.ConnConfig.Database, "_test") {
		t.Fatal("integration database name must end in _test")
	}
	config.MaxConns, config.MinConns = 8, 0
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
		do $roles$ begin
		  if not exists (select from pg_roles where rolname = 'anon') then create role anon nologin; end if;
		  if not exists (select from pg_roles where rolname = 'authenticated') then create role authenticated nologin; end if;
		end $roles$;
		create schema if not exists auth;
		create table if not exists auth.users (id uuid primary key);
		create or replace function auth.uid() returns uuid language sql stable as $uid$
		  select nullif(current_setting('request.jwt.claim.sub', true), '')::uuid
		$uid$;
		grant usage on schema auth, public to anon, authenticated;
		grant execute on function auth.uid() to anon, authenticated;
	`)
	if err != nil {
		t.Fatal(err)
	}
	var bypass bool
	if err := pool.QueryRow(ctx, `select bool_or(rolsuper or rolbypassrls) from pg_roles where rolname in ('anon', 'authenticated')`).Scan(&bypass); err != nil || bypass {
		t.Fatalf("browser fixture roles must not bypass RLS: %v, %v", bypass, err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}

func postgresTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) entity.UserProfile {
	t.Helper()
	var authID string
	if err := pool.QueryRow(ctx, `insert into auth.users (id) values (gen_random_uuid()) returning id::text`).Scan(&authID); err != nil {
		t.Fatal(err)
	}
	user, err := NewPostgresIdentityRepository(pool).EnsureUser(ctx, entity.AuthIdentity{AuthUserID: authID, Email: "  " + strings.ToUpper(authID) + "@EXAMPLE.TEST  ", Name: "Integration User"}, time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, `delete from public.audit_logs where actor_id = $1`, user.ID); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(cleanup, `delete from auth.users where id = $1`, authID); err != nil {
			t.Error(err)
		}
	})
	return user
}

func testSession(userID int64, id string, now time.Time) entity.Session {
	return entity.Session{ID: id, FamilyID: id, UserID: userID, Host: "auth.example.test", TokenHash: hashRateKey(id + "-token"), CSRFTokenHash: hashRateKey(id + "-csrf"), CreatedAt: now, ExpiresAt: now.Add(12 * time.Hour)}
}

func testHandshake(sessionID, label string, now time.Time) entity.SSOHandshake {
	return entity.SSOHandshake{CodeHash: hashRateKey(label), SessionID: sessionID, CodeChallenge: strings.Repeat("A", 43), ReturnTo: "https://admin.example.test/", ExpiresAt: now.Add(2 * time.Minute)}
}

func hasPermission(grants []entity.PermissionGrant, permission string) bool {
	for _, grant := range grants {
		if grant.Permission == permission {
			return true
		}
	}
	return false
}

func grantSet(grants []entity.PermissionGrant) map[entity.PermissionGrant]bool {
	result := make(map[entity.PermissionGrant]bool)
	for _, grant := range grants {
		result[grant] = true
	}
	return result
}

func TestPostgresMigrationRepeatChecksumAndConcurrentApply(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	type history struct {
		Name, Checksum string
		AppliedAt      time.Time
	}
	readHistory := func() []history {
		rows, err := pool.Query(ctx, `select name, checksum, applied_at from public.schema_migrations order by name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[history])
		if err != nil {
			t.Fatal(err)
		}
		return items
	}
	before := readHistory()
	if len(before) != 6 || before[0].Name != "0002_identity.sql" || before[2].Name != "0004_identity_hardening.sql" || before[3].Name != "0005_practicum_core.sql" || before[4].Name != "0006_elearning_core.sql" || before[5].Name != "0007_siakad_inventory_core.sql" {
		t.Fatalf("unexpected ordered history: %+v", before)
	}
	errs := make(chan error, 4)
	for range 4 {
		go func() { errs <- migrations.Apply(ctx, pool) }()
	}
	for range 4 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if after := readHistory(); !reflect.DeepEqual(before, after) {
		t.Fatalf("repeat migrations changed history: %+v", after)
	}
	if _, err := pool.Exec(ctx, `update public.schema_migrations set checksum = $1 where name = $2`, strings.Repeat("0", 64), before[0].Name); err != nil {
		t.Fatal(err)
	}
	err := migrations.Apply(ctx, pool)
	if _, restoreErr := pool.Exec(ctx, `update public.schema_migrations set checksum = $1 where name = $2`, before[0].Checksum, before[0].Name); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if err == nil || !strings.Contains(err.Error(), "checksum drift") {
		t.Fatalf("checksum drift was accepted: %v", err)
	}
}

func TestPostgresEnsureUserPersistenceAndUniqueness(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	if user.AccessVersion != 1 || user.Email != strings.ToLower(user.AuthUserID)+"@example.test" {
		t.Fatalf("profile was not normalized/versioned: %+v", user)
	}
	access, err := repo.ReadAccess(ctx, user.ID)
	if err != nil || len(access.Assignments) != 0 || len(access.Grants) != 0 {
		t.Fatalf("provisioning granted access: %+v, %v", access, err)
	}
	user.Name, user.NIM, user.Active, user.Hidden = "Manual Name", user.AuthUserID, false, true
	if err := repo.SaveUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 12)
	for range 12 {
		go func() {
			profile, err := repo.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: user.AuthUserID, Email: " " + strings.ToUpper(user.Email) + " ", Name: "Provider Name"}, time.Now())
			if err == nil && (profile.ID != user.ID || profile.Name != "Manual Name" || profile.Active || !profile.Hidden || profile.NIM != user.NIM) {
				err = fmt.Errorf("provider overwrote manual profile: %+v", profile)
			}
			errs <- err
		}()
	}
	for range 12 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	secondPool, err := pgxpool.NewWithConfig(ctx, pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer secondPool.Close()
	persisted, err := NewPostgresIdentityRepository(secondPool).FindUserByAuthID(ctx, user.AuthUserID)
	if err != nil || persisted.Name != "Manual Name" || persisted.Active {
		t.Fatalf("second repository lost profile: %+v, %v", persisted, err)
	}
	other := postgresTestUser(t, ctx, pool)
	if _, err := repo.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: other.AuthUserID, Email: user.Email}, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate email: %v", err)
	}
	other.NIM = user.NIM
	if err := repo.SaveUser(ctx, other); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate NIM: %v", err)
	}
	other.NIM = ""
	other.Email = user.Email
	if err := repo.SaveUser(ctx, other); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate profile email: %v", err)
	}
}

func TestPostgresSeedParityAndAccessReplacement(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	memory := NewMemoryIdentityRepository()
	definitions, err := repo.ListPermissionDefinitions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fixtureDefinitions, _ := memory.ListPermissionDefinitions(ctx)
	if !reflect.DeepEqual(definitions, fixtureDefinitions) {
		t.Fatalf("memory permission definitions diverged from SQL: %+v", definitions)
	}
	for id := int64(1); id <= 11; id++ {
		user := postgresTestUser(t, ctx, pool)
		access, _ := memory.ReadAccess(ctx, id)
		if err := repo.ReplaceAccess(ctx, user.ID, access); err != nil {
			t.Fatalf("fixture %d: %v", id, err)
		}
		grants, err := repo.ListPermissionGrants(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		fixtureGrants, _ := memory.ListPermissionGrants(ctx, id)
		if !reflect.DeepEqual(grantSet(grants), grantSet(fixtureGrants)) {
			t.Fatalf("fixture %d differs: database=%+v memory=%+v", id, grants, fixtureGrants)
		}
		if err := repo.ReplaceAccess(ctx, user.ID, access); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale access replacement: %v", err)
		}
	}
	user := postgresTestUser(t, ctx, pool)
	scope := entity.Scope{Domain: "praktikum", Division: "pendidikan", ClassID: "class-101"}
	access := entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleAsisten, Division: entity.DivisionPendidikan, Scope: scope, Active: true}}, Grants: []entity.PermissionGrant{{Permission: "question.manage", Role: entity.RoleAsisten, Division: entity.DivisionPendidikan, Scope: scope}}}
	if err := repo.ReplaceAccess(ctx, user.ID, access); err != nil {
		t.Fatal(err)
	}
	raw, err := repo.ReadAccess(ctx, user.ID)
	if err != nil || raw.Version != 2 || len(raw.Grants) != 1 || raw.Grants[0].Permission != "question.manage" {
		t.Fatalf("raw access included defaults or wrong version: %+v, %v", raw, err)
	}
	grants, err := repo.ListPermissionGrants(ctx, user.ID)
	if err != nil || !hasPermission(grants, "question.manage") || !hasPermission(grants, "admin.overview.read") || hasPermission(grants, "learning.content.manage") {
		t.Fatalf("scope intersection is incorrect: %+v, %v", grants, err)
	}
	for _, grant := range grants {
		if grant.Scope != scope {
			t.Fatalf("grant escaped assignment scope: %+v", grant)
		}
	}
	if _, err := pool.Exec(ctx, `update public.user_role_assignments set is_active = false where user_id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	grants, err = repo.ListPermissionGrants(ctx, user.ID)
	if err != nil || len(grants) != 0 {
		t.Fatalf("revoked assignment retained grants: %+v, %v", grants, err)
	}
	if _, err := pool.Exec(ctx, `update public.user_role_assignments set is_active = true where user_id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `update public.divisions set is_active = false where code = 'pendidikan'`); err != nil {
		t.Fatal(err)
	}
	grants, err = repo.ListPermissionGrants(ctx, user.ID)
	_, restoreErr := pool.Exec(ctx, `update public.divisions set is_active = true where code = 'pendidikan'`)
	if err != nil || restoreErr != nil || len(grants) != 0 {
		t.Fatalf("inactive division retained grants: %+v, %v, %v", grants, err, restoreErr)
	}
	other := postgresTestUser(t, ctx, pool)
	_, err = pool.Exec(ctx, `insert into public.user_permission_grants (user_id, assignment_id, permission_code) values ($1, $2, 'question.manage')`, other.ID, raw.Assignments[0].ID)
	assertPostgresCode(t, err, "23503")
	access = entity.UserAccess{Version: raw.Version, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Scope: entity.Scope{Institution: "one-campus"}, Active: true}}}
	if err := repo.ReplaceAccess(ctx, user.ID, access); err != nil {
		t.Fatal(err)
	}
	grants, err = repo.ListPermissionGrants(ctx, user.ID)
	if err != nil || hasPermission(grants, "identity.access.manage") || !hasPermission(grants, "question.manage") || !hasPermission(grants, "practicum.grade.write") {
		t.Fatalf("scoped coordinator defaults are incorrect: %+v, %v", grants, err)
	}
	// Phase 3 adds these coordinator defaults; they must still intersect the
	// assignment and must not conceal a retained direct grant from the old role.
	for _, grant := range grants {
		if grant.Permission == "question.manage" || grant.Permission == "practicum.grade.write" {
			if grant.Role != entity.RoleKoordinator || grant.Scope != (entity.Scope{Domain: "praktikum", Institution: "one-campus"}) {
				t.Fatalf("practicum default escaped coordinator scope: %+v", grant)
			}
		}
	}
	raw, err = repo.ReadAccess(ctx, user.ID)
	if err != nil || len(raw.Grants) != 0 {
		t.Fatalf("replacement retained old direct grants: %+v, %v", raw.Grants, err)
	}
}

func TestPostgresConcurrentSessionRotationAndFamilies(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	root := testSession(user.ID, "root-"+user.AuthUserID, now)
	if err := repo.SaveSession(ctx, root); err != nil {
		t.Fatal(err)
	}
	child := testSession(user.ID, "child-"+user.AuthUserID, now)
	child.FamilyID, child.Host = root.ID, "admin.example.test"
	if err := repo.SaveSession(ctx, child); err != nil {
		t.Fatal(err)
	}
	bad := testSession(user.ID, "bad-"+user.AuthUserID, now)
	bad.FamilyID = child.ID
	if err := repo.SaveSession(ctx, bad); !errors.Is(err, ErrConflict) {
		t.Fatalf("non-root family accepted: %v", err)
	}
	other := postgresTestUser(t, ctx, pool)
	bad.UserID, bad.FamilyID = other.ID, root.ID
	if err := repo.SaveSession(ctx, bad); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-user family accepted: %v", err)
	}
	errs := make(chan error, 16)
	for i := range 16 {
		go func() {
			errs <- repo.Transaction(ctx, func(tx IdentityRepository) error {
				session, err := tx.FindSessionByTokenHash(ctx, root.TokenHash)
				if err != nil {
					return err
				}
				session.TokenHash, session.CSRFTokenHash = hashRateKey(fmt.Sprint(root.ID, i)), hashRateKey(fmt.Sprint(root.ID, "csrf", i))
				session.ExpiresAt = session.ExpiresAt.Add(time.Hour)
				return tx.SaveSession(ctx, session)
			})
		}()
	}
	wins := 0
	for range 16 {
		err := <-errs
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrSessionNotFound) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("rotated one token %d times", wins)
	}
	if _, err := repo.FindSessionByTokenHash(ctx, root.TokenHash); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old token remains valid: %v", err)
	}
	persisted, err := NewPostgresIdentityRepository(pool).FindSessionByID(ctx, root.ID)
	if err != nil || !persisted.ExpiresAt.Equal(root.ExpiresAt) || persisted.TokenHash == root.TokenHash {
		t.Fatalf("rotation lost persistence or extended lifetime: %+v, %v", persisted, err)
	}
	for _, byToken := range []bool{false, true} {
		ready, release, done := make(chan error, 1), make(chan struct{}), make(chan error, 1)
		go func() {
			done <- repo.Transaction(ctx, func(tx IdentityRepository) error {
				var err error
				if byToken {
					_, err = tx.FindSessionByTokenHash(ctx, persisted.TokenHash)
				} else {
					_, err = tx.FindSessionByID(ctx, root.ID)
				}
				ready <- err
				<-release
				return err
			})
		}()
		select {
		case err := <-ready:
			if err != nil {
				close(release)
				t.Fatal(err)
			}
		case err := <-done:
			close(release)
			t.Fatalf("transaction failed before row lock: %v", err)
		}
		blocked, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		_, err := pool.Exec(blocked, `update public.auth_sessions set csrf_token_hash = csrf_token_hash where id = $1`, root.ID)
		cancel()
		close(release)
		if txErr := <-done; txErr != nil {
			t.Fatal(txErr)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("session lookup did not lock row (token=%v): %v", byToken, err)
		}
	}
	if err := repo.RevokeSession(ctx, root.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSession(ctx, persisted); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("revoked session restored: %v", err)
	}
	child, err = repo.FindSessionByID(ctx, child.ID)
	if err != nil || child.RevokedAt == nil {
		t.Fatalf("family child not revoked: %+v, %v", child, err)
	}
}

func TestPostgresHandshakeConcurrentOneUseExpiryAndRollback(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	root := testSession(user.ID, user.AuthUserID, now)
	if err := repo.SaveSession(ctx, root); err != nil {
		t.Fatal(err)
	}
	handshake := testHandshake(root.ID, user.AuthUserID, now)
	if err := repo.SaveHandshake(ctx, handshake); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 16)
	for range 16 {
		go func() {
			_, err := repo.ConsumeHandshake(ctx, handshake.CodeHash, now)
			errs <- err
		}()
	}
	wins := 0
	for range 16 {
		err := <-errs
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrHandshakeInvalid) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("consumed handshake %d times", wins)
	}
	if err := repo.SaveHandshake(ctx, handshake); !errors.Is(err, ErrConflict) {
		t.Fatalf("consumed handshake overwritten: %v", err)
	}
	expired := testHandshake(root.ID, user.AuthUserID+"expired", now)
	expired.ExpiresAt = now
	if err := repo.SaveHandshake(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ConsumeHandshake(ctx, expired.CodeHash, now); !errors.Is(err, ErrHandshakeInvalid) {
		t.Fatalf("expired handshake accepted: %v", err)
	}
	rollback := testHandshake(root.ID, user.AuthUserID+"rollback", now)
	if err := repo.SaveHandshake(ctx, rollback); err != nil {
		t.Fatal(err)
	}
	abort := errors.New("abort exchange")
	err := repo.Transaction(ctx, func(tx IdentityRepository) error {
		if _, err := tx.ConsumeHandshake(ctx, rollback.CodeHash, now); err != nil {
			return err
		}
		child := testSession(user.ID, "rollback-"+root.ID, now)
		child.FamilyID = root.ID
		if err := tx.SaveSession(ctx, child); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if _, err := repo.FindSessionByID(ctx, "rollback-"+root.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("rolled-back exchange session persisted: %v", err)
	}
	if _, err := repo.ConsumeHandshake(ctx, rollback.CodeHash, now); err != nil {
		t.Fatalf("rolled-back exchange spent handshake: %v", err)
	}
}

func TestPostgresHandshakeIssuerBinding(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	user := postgresTestUser(t, ctx, pool)
	assertHandshakeIssuerBinding(t, ctx, NewPostgresIdentityRepository(pool), user.ID, user.AuthUserID)
}

func TestPostgresTransactionRollbackAndAuditPagination(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	abort := errors.New("rollback all identity writes")
	err := repo.Transaction(ctx, func(tx IdentityRepository) error {
		updated := user
		updated.Name = "Do Not Persist"
		if err := tx.SaveUser(ctx, updated); err != nil {
			return err
		}
		if err := tx.ReplaceAccess(ctx, user.ID, entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true}}}); err != nil {
			return err
		}
		if err := tx.SaveSession(ctx, testSession(user.ID, user.AuthUserID, now)); err != nil {
			return err
		}
		if err := tx.AppendAudit(ctx, entity.AuditEvent{ActorID: &user.ID, Action: "rollback", SubjectID: user.AuthUserID, CreatedAt: now}); err != nil {
			return err
		}
		if _, err := tx.TakeRateLimit(ctx, user.AuthUserID, now, 1); err != nil {
			return err
		}
		return abort
	})
	if !errors.Is(err, abort) {
		t.Fatal(err)
	}
	persisted, err := repo.FindUserByID(ctx, user.ID)
	if err != nil || persisted.Name != user.Name || persisted.AccessVersion != 1 {
		t.Fatalf("profile rollback failed: %+v, %v", persisted, err)
	}
	access, _ := repo.ReadAccess(ctx, user.ID)
	if len(access.Assignments) != 0 {
		t.Fatalf("access rollback failed: %+v", access)
	}
	if _, err := repo.FindSessionByID(ctx, user.AuthUserID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("session rollback failed: %v", err)
	}
	allowed, err := repo.TakeRateLimit(ctx, user.AuthUserID, now, 1)
	if err != nil || !allowed {
		t.Fatalf("rate rollback failed: %v, %v", allowed, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `select count(*) from public.audit_logs where actor_id = $1`, user.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("audit rollback failed: %d, %v", count, err)
	}
	for i := range 3 {
		if err := repo.AppendAudit(ctx, entity.AuditEvent{ActorID: &user.ID, Action: fmt.Sprint("event-", i), SubjectID: user.AuthUserID, Detail: map[string]any{"scope": "test"}, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := repo.ListAuditPage(ctx, 0, 2)
	if err != nil || len(page) != 2 || page[0].ID <= page[1].ID {
		t.Fatalf("audit page is not newest-first: %+v, %v", page, err)
	}
	next, err := repo.ListAuditPage(ctx, page[1].ID, 2)
	if err != nil || len(next) == 0 || next[0].ID >= page[1].ID {
		t.Fatalf("audit cursor overlaps: %+v, %v", next, err)
	}
}

func TestPostgresRateLimitConcurrencyAndBoundedPruning(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	key := t.Name() + time.Now().String()
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-48 * time.Hour)
	if _, err := pool.Exec(ctx, `insert into public.auth_rate_limits (key_hash, window_start, hits) select lpad(to_hex(n), 64, '0'), $1, 1 from generate_series(1, 300) n`, old); err != nil {
		t.Fatal(err)
	}
	var before, after int
	if err := pool.QueryRow(ctx, `select count(*) from public.auth_rate_limits where window_start < $1`, now.Add(-24*time.Hour)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.TakeRateLimit(ctx, key+"prune", now, 1); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `select count(*) from public.auth_rate_limits where window_start < $1`, now.Add(-24*time.Hour)).Scan(&after); err != nil || before-after != 128 {
		t.Fatalf("pruning was not bounded: before=%d after=%d err=%v", before, after, err)
	}
	type result struct {
		Allowed bool
		Err     error
	}
	results := make(chan result, 24)
	var start sync.WaitGroup
	start.Add(1)
	for range 24 {
		go func() {
			start.Wait()
			allowed, err := NewPostgresIdentityRepository(pool).TakeRateLimit(ctx, key, now, 7)
			results <- result{allowed, err}
		}()
	}
	start.Done()
	accepted := 0
	for range 24 {
		result := <-results
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if result.Allowed {
			accepted++
		}
	}
	if accepted != 7 {
		t.Fatalf("rate limiter accepted %d requests, wanted 7", accepted)
	}
	if allowed, err := repo.TakeRateLimit(ctx, key, now.Add(time.Minute), 7); err != nil || !allowed {
		t.Fatalf("new window was denied: %v, %v", allowed, err)
	}
	if _, err := pool.Exec(ctx, `delete from public.auth_rate_limits where window_start = $1 or key_hash = any($2::text[])`, old, []string{hashRateKey(key), hashRateKey(key + "prune")}); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresConcurrentAccessAndActorRecheck(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	assertConcurrentAccessReplacement(t, ctx, repo, user.ID, 1)
	actor := postgresTestUser(t, ctx, pool)
	if err := repo.ReplaceAccess(ctx, actor.ID, entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Active: true}}}); err != nil {
		t.Fatal(err)
	}
	target := postgresTestUser(t, ctx, pool)
	ready, release, firstDone := make(chan error, 1), make(chan struct{}), make(chan error, 1)
	go func() {
		firstDone <- repo.Transaction(ctx, func(tx IdentityRepository) error {
			err := tx.ReplaceAccess(ctx, actor.ID, entity.UserAccess{Version: 2, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true}}})
			ready <- err
			<-release
			return err
		})
	}()
	select {
	case err := <-ready:
		if err != nil {
			close(release)
			t.Fatal(err)
		}
	case err := <-firstDone:
		close(release)
		t.Fatalf("access transaction failed before change: %v", err)
	}
	entered, secondDone := make(chan struct{}, 1), make(chan error, 1)
	denied := errors.New("actor no longer manages access")
	go func() {
		secondDone <- repo.Transaction(ctx, func(tx IdentityRepository) error {
			entered <- struct{}{}
			grants, err := tx.ListPermissionGrants(ctx, actor.ID)
			if err != nil {
				return err
			}
			if !hasPermission(grants, "identity.access.manage") {
				return denied
			}
			return tx.ReplaceAccess(ctx, target.ID, entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Active: true}}})
		})
	}()
	early := false
	select {
	case <-entered:
		early = true
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; !errors.Is(err, denied) || early {
		t.Fatalf("actor recheck was not serialized before change: early=%v err=%v", early, err)
	}
	access, err := repo.ReadAccess(ctx, target.ID)
	if err != nil || access.Version != 1 || len(access.Assignments) != 0 {
		t.Fatalf("revoked actor changed target access: %+v, %v", access, err)
	}
}

func TestPostgresIdentityConstraints(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	user := postgresTestUser(t, ctx, pool)
	_, err := pool.Exec(ctx, `insert into public.user_role_assignments (user_id, role_code, division_code) values ($1, 'mahasiswa', 'pendidikan')`, user.ID)
	assertPostgresCode(t, err, "23514")
	_, err = pool.Exec(ctx, `update public.users set email = upper(email) where id = $1`, user.ID)
	assertPostgresCode(t, err, "23514")
	_, err = pool.Exec(ctx, `insert into public.user_permission_grants (user_id, permission_code) values ($1, 'question.manage')`, user.ID)
	assertPostgresCode(t, err, "23502")
	now := time.Now().UTC().Truncate(time.Microsecond)
	root := testSession(user.ID, user.AuthUserID, now)
	invalid := root
	invalid.TokenHash = "plaintext-session-token"
	if err := repo.SaveSession(ctx, invalid); !errors.Is(err, ErrConflict) {
		t.Fatalf("plaintext token persisted: %v", err)
	}
	if err := repo.SaveSession(ctx, root); err != nil {
		t.Fatal(err)
	}
	duplicate := root
	duplicate.ID, duplicate.FamilyID = "duplicate-"+root.ID, "duplicate-"+root.ID
	if err := repo.SaveSession(ctx, duplicate); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate session token accepted: %v", err)
	}
	handshake := testHandshake(root.ID, root.ID, now)
	handshake.CodeHash = "plaintext-handshake-code"
	if err := repo.SaveHandshake(ctx, handshake); !errors.Is(err, ErrConflict) {
		t.Fatalf("plaintext handshake persisted: %v", err)
	}
	handshake.CodeHash, handshake.CodeChallenge = hashRateKey(root.ID), "invalid-challenge"
	if err := repo.SaveHandshake(ctx, handshake); !errors.Is(err, ErrConflict) {
		t.Fatalf("invalid PKCE challenge accepted: %v", err)
	}
	if err := repo.RevokeSession(ctx, root.ID, now); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `update public.auth_sessions set revoked_at = null where id = $1`, root.ID)
	assertPostgresCode(t, err, "23514")
}

func assertPostgresCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("wanted SQLSTATE %s, got %v", code, err)
	}
}

func TestPostgresBrowserRLSAndAdministrativeWrites(t *testing.T) {
	ctx, pool := postgresTestDatabase(t)
	repo := NewPostgresIdentityRepository(pool)
	own, other := postgresTestUser(t, ctx, pool), postgresTestUser(t, ctx, pool)
	for _, user := range []entity.UserProfile{own, other} {
		access := entity.UserAccess{Version: 1, Assignments: []entity.RoleAssignment{{Role: entity.RoleAsisten, Active: true}}, Grants: []entity.PermissionGrant{{Role: entity.RoleAsisten, Permission: "question.manage", Scope: entity.Scope{Domain: "praktikum"}}}}
		if err := repo.ReplaceAccess(ctx, user.ID, access); err != nil {
			t.Fatal(err)
		}
		root := testSession(user.ID, user.AuthUserID, time.Now().UTC().Truncate(time.Microsecond))
		if err := repo.SaveSession(ctx, root); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveHandshake(ctx, testHandshake(root.ID, root.ID, time.Now())); err != nil {
			t.Fatal(err)
		}
	}
	browser := func(role, extraGrant string, operation func(pgx.Tx)) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tx.Rollback(cleanup)
		}()
		if extraGrant != "" {
			if _, err := tx.Exec(ctx, extraGrant); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := tx.Exec(ctx, `select set_config('request.jwt.claim.sub', $1, true)`, own.AuthUserID); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `set local role `+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		operation(tx)
	}
	browser("authenticated", "", func(tx pgx.Tx) {
		var ids []int64
		if err := tx.QueryRow(ctx, `select array_agg(id) from public.users`).Scan(&ids); err != nil || !reflect.DeepEqual(ids, []int64{own.ID}) {
			t.Fatalf("cross-user profiles exposed: %v, %v", ids, err)
		}
		for _, table := range []string{"user_role_assignments", "user_permission_grants"} {
			if err := tx.QueryRow(ctx, `select array_agg(distinct user_id) from public.`+pgx.Identifier{table}.Sanitize()).Scan(&ids); err != nil || !reflect.DeepEqual(ids, []int64{own.ID}) {
				t.Fatalf("cross-user %s exposed: %v, %v", table, ids, err)
			}
		}
	})
	for _, query := range []string{
		`select token_hash, csrf_token_hash, provider_refresh_token_seal from public.auth_sessions`,
		`select * from public.sso_handshakes`,
		`select * from public.auth_rate_limits`,
		`select * from public.audit_logs`,
		`update public.users set is_active = true, hidden = false, access_version = 100`,
		`update public.user_role_assignments set role_code = 'koordinator'`,
		`delete from public.user_permission_grants`,
		`insert into public.users (auth_user_id, email, name) values (auth.uid(), 'browser@example.test', 'browser')`,
	} {
		for _, role := range []string{"authenticated", "anon"} {
			browser(role, "", func(tx pgx.Tx) {
				_, err := tx.Exec(ctx, query)
				assertPostgresCode(t, err, "42501")
			})
		}
	}
	browser("anon", "", func(tx pgx.Tx) {
		_, err := tx.Exec(ctx, `select * from public.users`)
		assertPostgresCode(t, err, "42501")
	})
	browser("authenticated", `grant select on public.auth_sessions, public.sso_handshakes, public.auth_rate_limits, public.audit_logs to authenticated`, func(tx pgx.Tx) {
		for _, table := range []string{"auth_sessions", "sso_handshakes", "auth_rate_limits", "audit_logs"} {
			var count int
			if err := tx.QueryRow(ctx, `select count(*) from public.`+pgx.Identifier{table}.Sanitize()).Scan(&count); err != nil || count != 0 {
				t.Fatalf("RLS did not hide %s despite accidental SELECT privilege: %d, %v", table, count, err)
			}
		}
	})
	browser("authenticated", `grant update on public.users to authenticated`, func(tx pgx.Tx) {
		tag, err := tx.Exec(ctx, `update public.users set is_active = false where id = $1`, own.ID)
		if err != nil || tag.RowsAffected() != 0 {
			t.Fatalf("RLS permitted administrative self-update: %d, %v", tag.RowsAffected(), err)
		}
	})
}
