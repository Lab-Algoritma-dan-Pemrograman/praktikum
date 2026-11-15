package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresPracticumBatchRosterFiveHundredStudents(t *testing.T) {
	h := newPGPracticum(t)
	scopeJSON, _ := json.Marshal(h.session.Scope)
	rows, err := h.pool.Query(h.ctx, `with new_auth as (
 insert into auth.users(id) select gen_random_uuid() from generate_series(1,500) returning id
 ), new_users as (
 insert into public.users(auth_user_id,email,name)
 select id,id::text||'@example.test','Synthetic roster user' from new_auth returning id,auth_user_id
 ), assignments as (
 insert into public.user_role_assignments(user_id,role_code,scope)
 select id,'mahasiswa',$1::jsonb from new_users returning user_id
 ) select u.id,u.auth_user_id::text from new_users u join assignments a on a.user_id=u.id`, string(scopeJSON))
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	authIDs := []string{}
	for rows.Next() {
		var id int64
		var authID string
		if err = rows.Scan(&id, &authID); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		authIDs = append(authIDs, authID)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := h.pool.Exec(ctx, `delete from auth.users where id=any($1::uuid[])`, authIDs); err != nil {
			t.Error(err)
		}
	})
	cfg := h.pool.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(h.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(pool), IdentityRepository: repository.NewPostgresIdentityRepository(pool)})
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()
	activation, err := service.Activate(ctx, h.admin, pgPracticumCommand("five-hundred-roster"), h.session.ID, usecase.PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "individual", StudentIDs: ids, QuestionIDs: []string{h.questions[0].ID}})
	if err != nil || len(activation.StudentIDs) != 500 {
		t.Fatalf("batch roster failed: %d %v", len(activation.StudentIDs), err)
	}
}
