package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgLearningHarness struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	repo     *repository.PostgresLearningRepository
	service  *usecase.LearningService
	admin    *entity.Principal
	students []*entity.Principal
	level    entity.LearningLevel
	module   entity.LearningModule
	lesson   entity.LearningLesson
	bugHunt  entity.LearningBugHuntQuestion
}

func pgLearningCommand(key string) usecase.LearningCommand {
	return usecase.LearningCommand{Key: key, RequestID: "req-pg-learning", OperationID: "op-pg-learning"}
}

func newPGLearning(t *testing.T) *pgLearningHarness {
	t.Helper()
	ctx, pool := repository.OpenTestDatabase(t)
	h := &pgLearningHarness{ctx: ctx, pool: pool, repo: repository.NewPostgresLearningRepository(pool)}
	identity := repository.NewPostgresIdentityRepository(pool)
	scope := entity.Scope{Domain: "elearning", Institution: "synthetic-elearning", Division: "pendidikan"}
	var authIDs []string
	var userIDs []int64
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, `delete from public.audit_logs where actor_id=any($1::bigint[]) or scope->>'institution'='synthetic-elearning'`, userIDs); err != nil {
			t.Error(err)
		}
		for _, table := range []string{"game_history", "runner_leases", "operations", "progress", "profiles", "bug_hunt", "lessons", "modules", "levels"} {
			if _, err := pool.Exec(cleanup, `delete from public.learning_`+table+` where scope->>'institution'='synthetic-elearning' or owner_id=any($1::bigint[])`, userIDs); err != nil {
				t.Error(err)
			}
		}
		if _, err := pool.Exec(cleanup, `delete from auth.users where id=any($1::uuid[])`, authIDs); err != nil {
			t.Error(err)
		}
	})
	for i := range 3 {
		var authID string
		if err := pool.QueryRow(ctx, `insert into auth.users(id) values(gen_random_uuid()) returning id::text`).Scan(&authID); err != nil {
			t.Fatal(err)
		}
		authIDs = append(authIDs, authID)
		user, err := identity.EnsureUser(ctx, entity.AuthIdentity{AuthUserID: authID, Email: authID + "@example.test", Name: "Synthetic learning user"}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, user.ID)
		role := entity.RoleMahasiswa
		if i == 0 {
			role = entity.RoleAsisten
		}
		assignment := entity.RoleAssignment{Role: role, Division: entity.DivisionPendidikan, Scope: scope, Active: true}
		if role == entity.RoleMahasiswa {
			assignment.Division = ""
		}
		if err := identity.ReplaceAccess(ctx, user.ID, entity.UserAccess{Version: user.AccessVersion, Assignments: []entity.RoleAssignment{assignment}}); err != nil {
			t.Fatal(err)
		}
		p := &entity.Principal{User: user, Assignments: []entity.RoleAssignment{assignment}}
		if i == 0 {
			for _, perm := range []string{"learning.content.read", "learning.content.manage", "learning.monitor"} {
				p.Grants = append(p.Grants, entity.PermissionGrant{Permission: perm, Role: role, Division: entity.DivisionPendidikan, Scope: scope})
			}
			h.admin = p
		} else {
			p.Grants = []entity.PermissionGrant{{Permission: "learning.content.read", Role: role, Scope: scope}}
			h.students = append(h.students, p)
		}
	}
	h.service = usecase.NewLearningService(usecase.LearningOptions{
		Repository:                 h.repo,
		IdentityRepository:         identity,
		Runner:                     runner.NewFixture(),
		AllowSimulatedVerification: true,
	})
	var err error
	h.level, err = h.service.CreateLevel(ctx, h.admin, pgLearningCommand("level"), usecase.LearningLevelInput{
		Title: "Synthetic Level 1", Description: "Database test level", SortOrder: 1, AccessMode: "unlocked", RequiredXP: 0, Scope: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.module, err = h.service.CreateModule(ctx, h.admin, pgLearningCommand("module"), usecase.LearningModuleInput{
		LevelID: h.level.ID, Title: "Synthetic Module 1", Description: "Database test module", SortOrder: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	optZero := 0
	h.lesson, err = h.service.SaveLesson(ctx, h.admin, pgLearningCommand("lesson"), "", usecase.LearningLessonUpdate{
		LearningLessonInput: usecase.LearningLessonInput{
			ModuleID:    h.module.ID,
			Title:       "Synthetic Lesson 1",
			SortOrder:   1,
			Language:    "c",
			Explanation: "Penjelasan C",
			CodeExample: "int main(void){return 0;}",
			StarterCode: "// starter",
			Solution:    "PRIVATE_DB_LESSON_SOLUTION",
			Hint:        "Hint C",
			XPReward:    60,
			Quiz:        &entity.LearningQuiz{Question: "Fungsi utama?", Options: []string{"main", "init"}, CorrectOption: &optZero},
			TestCases:   []entity.LearningTestCase{{Description: "Uji C", Input: "", ExpectedOutput: "SIMULATED fixture: success scenario; source was not executed."}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.bugHunt, err = h.service.SaveBugHunt(ctx, h.admin, pgLearningCommand("bughunt"), "", usecase.LearningBugHuntUpdate{
		LearningBugHuntInput: usecase.LearningBugHuntInput{
			Language:    "c",
			Difficulty:  "easy",
			Title:       "Synthetic Bug Hunt",
			BuggyCode:   "int main(void){\n  if (x = 1) return 1;\n  return 0;\n}",
			BugLine:     2,
			Explanation: "PRIVATE_DB_BUGHUNT_EXPLANATION",
			XPReward:    50,
			Active:      true,
			Scope:       scope,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPostgresLearningConcurrentCompletionImmutabilityRollbackAndRLS(t *testing.T) {
	h := newPGLearning(t)
	pool2, err := pgxpool.NewWithConfig(h.ctx, h.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer pool2.Close()
	other := usecase.NewLearningService(usecase.LearningOptions{
		Repository:                 repository.NewPostgresLearningRepository(pool2),
		Runner:                     runner.NewFixture(),
		AllowSimulatedVerification: true,
	})

	optZero := 0
	input := usecase.LearningProgressInput{
		Revision:          0,
		DraftCode:         "#include <stdio.h>\nint main(void) { return 0; }",
		Language:          "c",
		QuizOption:        &optZero,
		Complete:          true,
		BrowserTestPassed: true,
		LessonVersion:     h.lesson.Version,
	}
	gate := make(chan struct{})
	errs := make(chan error, 10)
	for i := range 10 {
		go func() {
			<-gate
			s := h.service
			if i%2 == 0 {
				s = other
			}
			_, err := s.SaveProgress(h.ctx, h.students[0], pgLearningCommand("same-complete"), h.lesson.ID, input)
			errs <- err
		}()
	}
	close(gate)
	for range 10 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	view, err := h.service.Lesson(h.ctx, h.students[0], h.lesson.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Progress.Status != "completed" || view.Progress.Revision != 1 || view.Progress.XPAwarded != 60 || view.Profile.XP != 60 || view.Profile.CompletedLessons != 1 {
		t.Fatalf("unexpected progress/profile after concurrent completion: %+v %+v", view.Progress, view.Profile)
	}

	// Second completion with new key and revision 1 must not award extra XP.
	input.Revision = 1
	view2, err := other.SaveProgress(h.ctx, h.students[0], pgLearningCommand("second-complete"), h.lesson.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if view2.Profile.XP != 60 || view2.Profile.CompletedLessons != 1 {
		t.Fatalf("duplicate completion awarded extra XP: %+v", view2.Profile)
	}

	// Submit Bug Hunt to create a game_history row.
	bh, err := h.service.SubmitBugHunt(h.ctx, h.students[0], pgLearningCommand("bh-1"), usecase.LearningBugHuntAttemptInput{
		QuestionID:   h.bugHunt.ID,
		SelectedLine: 2,
		Explanation:  "Operator assignment pada kondisi if",
	})
	if err != nil || !bh.Attempt.Correct || bh.Profile.XP != 110 {
		t.Fatalf("bug hunt submission failed: %+v %v", bh, err)
	}

	progressID := fmt.Sprintf("progress-%s-%d", h.lesson.ID, h.students[0].User.ID)
	profileID := fmt.Sprintf("profile-%d", h.students[0].User.ID)
	tests := []struct {
		name, sql string
		args      []any
	}{
		{"foreign-module-level", `insert into public.learning_modules(id,parent_id,scope,data) values('bad-mod','missing-level','{}','{"sort_order":1,"version":1}')`, nil},
		{"invalid-access-mode", `update public.learning_levels set data=jsonb_set(data,'{access_mode}','"invalid"') where id=$1`, []any{h.level.ID}},
		{"invalid-lesson-xp", `update public.learning_lessons set data=jsonb_set(data,'{xp_reward}','501') where id=$1`, []any{h.lesson.ID}},
		{"reopen-completed-progress", `update public.learning_progress set status='in_progress' where id=$1`, []any{progressID}},
		{"tamper-completed-xp", `update public.learning_progress set data=jsonb_set(data,'{xp_awarded}','500') where id=$1`, []any{progressID}},
		{"decrease-progress-revision", `update public.learning_progress set data=jsonb_set(data,'{revision}','0') where id=$1`, []any{progressID}},
		{"duplicate-progress-row", `insert into public.learning_progress(id,parent_id,owner_id,scope,status,data) select id||'-dup',parent_id,owner_id,scope,status,data from public.learning_progress where id=$1`, []any{progressID}},
		{"decrease-profile-xp", `update public.learning_profiles set data=jsonb_set(data,'{xp}','10') where id=$1`, []any{profileID}},
		{"change-profile-owner", `update public.learning_profiles set owner_id=$2 where id=$1`, []any{profileID, h.students[1].User.ID}},
		{"mutate-game-history", `update public.learning_game_history set data=jsonb_set(data,'{xp_earned}','300') where id=$1`, []any{bh.Attempt.AttemptID}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.pool.Begin(h.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(context.Background())
			if _, err = tx.Exec(h.ctx, tc.sql, tc.args...); err == nil {
				t.Fatal("database accepted invalid state")
			}
		})
	}

	sentinel := errors.New("synthetic learning rollback")
	err = h.repo.Transaction(h.ctx, func(tx repository.LearningTx) error {
		raw, _ := json.Marshal(h.level)
		if err := tx.Put(repository.LearningLevels, repository.LearningRecord{ID: h.level.ID + "-rollback", Scope: h.level.Scope, Data: raw}); err != nil {
			return err
		}
		if err := tx.Audit(entity.AuditEvent{ActorID: &h.admin.User.ID, Action: "learning.rollback.probe", App: "elearning", SubjectID: h.level.ID, Scope: h.level.Scope, CreatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal("repository transaction did not return sentinel")
	}
	var audits int
	if err := h.pool.QueryRow(h.ctx, `select count(*) from public.audit_logs where action='learning.rollback.probe' and subject_id=$1`, h.level.ID).Scan(&audits); err != nil || audits != 0 {
		t.Fatal("audit escaped rolled-back learning transaction")
	}

	for _, role := range []string{"anon", "authenticated"} {
		for _, table := range []string{"levels", "modules", "lessons", "progress", "profiles", "bug_hunt", "game_history", "operations", "runner_leases"} {
			t.Run(role+"/"+table, func(t *testing.T) {
				var rls, canSelect bool
				if err := h.pool.QueryRow(h.ctx, `select c.relrowsecurity,has_table_privilege($1,c.oid,'SELECT') from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='public' and c.relname=$2`, role, "learning_"+table).Scan(&rls, &canSelect); err != nil || !rls || canSelect {
					t.Fatalf("missing RLS/grant restriction: rls=%v select=%v %v", rls, canSelect, err)
				}
				tx, err := h.pool.Begin(h.ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(context.Background())
				if _, err := tx.Exec(h.ctx, "set local role "+role); err != nil {
					t.Fatal(err)
				}
				_, err = tx.Exec(h.ctx, "select * from public.learning_"+table)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
					t.Fatalf("browser role directly read table: %v", err)
				}
			})
		}
	}

	var auditText string
	if err := h.pool.QueryRow(h.ctx, `select coalesce(jsonb_agg(to_jsonb(a))::text,'[]') from public.audit_logs a where scope->>'institution'='synthetic-elearning'`).Scan(&auditText); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_DB_LESSON_SOLUTION", "PRIVATE_DB_BUGHUNT_EXPLANATION"} {
		if strings.Contains(auditText, secret) {
			t.Fatal("transactional audit leaked private E-learning content")
		}
	}
}
