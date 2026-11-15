package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresPracticumActivationWithOnePoolConnection(t *testing.T) {
	h := newPGPracticum(t)
	cfg := h.pool.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(h.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(pool), IdentityRepository: repository.NewPostgresIdentityRepository(pool)})
	ctx, cancel := context.WithTimeout(h.ctx, 3*time.Second)
	defer cancel()
	got, err := service.Activate(ctx, h.admin, pgPracticumCommand("one-connection"), h.session.ID, usecase.PracticumActivationInput{CourseID: h.course.ID, Scope: h.activation.Scope, TimerMode: "individual", StudentIDs: []int64{h.students[0].User.ID}, QuestionIDs: []string{h.questions[0].ID}})
	if err != nil || got.ID == "" {
		t.Fatalf("activation starved its own connection: %+v %v", got, err)
	}
}

func TestPostgresPracticumMetadataRepairPreservesWorkAndRestoresTriggers(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	_, err := h.service.SaveAnswers(h.ctx, h.students[0], pgPracticumCommand("metadata-answer"), a.AttemptID, usecase.PracticumSaveInput{Answers: []usecase.PracticumAnswerInput{{QuestionID: h.questions[0].ID, Value: "SYNTHETIC_PRESERVED_ANSWER"}}})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../../migrations/0011_practicum_attempt_metadata_repair.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// Simulate the pre-0011 schema only inside this rollback-only transaction.
	if _, err = tx.Exec(h.ctx, `drop trigger practicum_attempt_metadata on public.practicum_attempts;
 drop function public.practicum_check_attempt_metadata();
 alter table public.practicum_attempts disable trigger practicum_attempt_frozen;`); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = tx.QueryRow(h.ctx, `select data from public.practicum_attempts where id=$1`, a.AttemptID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err = json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	attempt := record["attempt"].(map[string]any)
	legacyID := "legacy-attempt-" + a.AttemptID
	attempt["attempt_id"] = legacyID
	attempt["session_id"] = "legacy-session-wrong"
	attempt["status"] = "submitted"
	record["submitted_answers"] = attempt["answers"]
	record["legacy"] = map[string]any{"pengerjaan_course": map[string]any{"id": 12}}
	wrong := h.activation.Scope
	wrong.Shift = 2
	wrongRaw, _ := json.Marshal(wrong)
	record["scope"] = wrong
	before, _ := json.Marshal(record)
	if _, err = tx.Exec(h.ctx, `update public.practicum_attempts set id=$1,scope=$2::jsonb,status='submitted',data=$3::jsonb where id=$4`, legacyID, string(wrongRaw), string(before), a.AttemptID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(h.ctx, `alter table public.practicum_attempts enable trigger practicum_attempt_frozen`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(h.ctx, string(body)); err != nil {
		t.Fatal(err)
	}
	var afterRaw, scopeRaw []byte
	if err = tx.QueryRow(h.ctx, `select data,scope from public.practicum_attempts where id=$1`, legacyID).Scan(&afterRaw, &scopeRaw); err != nil {
		t.Fatal(err)
	}
	var after map[string]any
	if err = json.Unmarshal(afterRaw, &after); err != nil {
		t.Fatal(err)
	}
	afterAttempt := after["attempt"].(map[string]any)
	if afterAttempt["session_id"] != h.session.ID {
		t.Fatal("session ancestry not repaired")
	}
	var repairedScope entity.Scope
	if err = json.Unmarshal(scopeRaw, &repairedScope); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repairedScope, h.activation.Scope) {
		t.Fatal("grader scope not repaired")
	}
	// Remove exactly the approved metadata differences and compare all work bytes.
	delete(record, "scope")
	delete(after, "scope")
	delete(attempt, "session_id")
	delete(afterAttempt, "session_id")
	wantWork, _ := json.Marshal(record)
	gotWork, _ := json.Marshal(after)
	if string(wantWork) != string(gotWork) {
		t.Fatal("repair changed answers, snapshots, grades, revisions or timers")
	}
	var enabled string
	if err = tx.QueryRow(h.ctx, `select tgenabled::text from pg_trigger where tgrelid='public.practicum_attempts'::regclass and tgname='practicum_attempt_frozen'`).Scan(&enabled); err != nil || enabled != "O" {
		t.Fatalf("freeze trigger not restored: %s %v", enabled, err)
	}
	if _, err = tx.Exec(h.ctx, "savepoint reject_metadata"); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(h.ctx, `update public.practicum_attempts set data=jsonb_set(data,'{attempt,session_id}','"forbidden-session"') where id=$1`, legacyID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("new metadata drift allowed: %v", err)
	}
	if _, err = tx.Exec(h.ctx, "rollback to savepoint reject_metadata"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(h.ctx); err != nil {
		t.Fatal(err)
	}
	if err = h.pool.QueryRow(h.ctx, `select tgenabled::text from pg_trigger where tgrelid='public.practicum_attempts'::regclass and tgname='practicum_attempt_frozen'`).Scan(&enabled); err != nil || enabled != "O" {
		t.Fatalf("rollback left trigger disabled: %s %v", enabled, err)
	}
}
