package repository_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPostgresPracticumLegacyActivationCanCloseWithoutSnapshotLoss(t *testing.T) {
	h := newPGPracticum(t)
	a := h.start(t, 0)
	tx, err := h.pool.Begin(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(h.ctx, `alter table public.practicum_activations disable trigger practicum_activation_frozen`); err != nil {
		t.Fatal(err)
	}
	start := strings.TrimSuffix(a.StartedAt.UTC().Format(time.RFC3339Nano), "Z") + "+00:00"
	deadline := strings.TrimSuffix(a.DeadlineAt.UTC().Format(time.RFC3339Nano), "Z") + "+00:00"
	if _, err = tx.Exec(h.ctx, `update public.practicum_activations set data=
 jsonb_set(jsonb_set(jsonb_set(jsonb_set(jsonb_set(data #- '{activation,entry_approved_ids}' #- '{activation,question_ids}',
 '{course,legacy}','{"source_id":10}'),'{questions,0,legacy}','{"source_id":99}'),
 '{activation,started_at}',to_jsonb($2::text)), '{activation,deadline_at}',to_jsonb($3::text)),
 '{legacy}','{"aktivasi_course":{"id":88}}') where id=$1`, h.activation.ID, start, deadline); err != nil {
		t.Fatal(err)
	}
	legacyKey := "legacy:aktivasi_course:" + h.activation.ID
	if _, err = tx.Exec(h.ctx, `update public.practicum_activations set unique_key=$2,status='open' where id=$1`, h.activation.ID, legacyKey); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(h.ctx, `alter table public.practicum_activations enable trigger practicum_activation_frozen`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(h.ctx); err != nil {
		t.Fatal(err)
	}
	var course, questions string
	if err = h.pool.QueryRow(h.ctx, `select (data->'course')::text,(data->'questions')::text from public.practicum_activations where id=$1`, h.activation.ID).Scan(&course, &questions); err != nil {
		t.Fatal(err)
	}
	closed, err := h.service.Close(h.ctx, h.admin, pgPracticumCommand("legacy-close"), h.session.ID, h.course.ID, h.activation.ID)
	if err != nil || closed.IsOpen {
		t.Fatalf("legacy activation could not close: %+v %v", closed, err)
	}
	var finalCourse, finalQuestions, finalStart, finalDeadline string
	var legacy, absentRosterFields bool
	if err = h.pool.QueryRow(h.ctx, `select (data->'course')::text,(data->'questions')::text,
 data->'activation'->>'started_at',data->'activation'->>'deadline_at',
 data->'legacy'->'aktivasi_course'->>'id'='88',
 not (data->'activation' ? 'entry_approved_ids') and not (data->'activation' ? 'question_ids')
 from public.practicum_activations where id=$1`, h.activation.ID).Scan(&finalCourse, &finalQuestions, &finalStart, &finalDeadline, &legacy, &absentRosterFields); err != nil {
		t.Fatal(err)
	}
	if course != finalCourse || questions != finalQuestions || start != finalStart || deadline != finalDeadline || !legacy || !absentRosterFields {
		t.Fatal("closing activation changed immutable snapshot or legacy JSON shape")
	}
	var storedKey, indexedStatus string
	if err = h.pool.QueryRow(h.ctx, `select unique_key,status from public.practicum_activations where id=$1`, h.activation.ID).Scan(&storedKey, &indexedStatus); err != nil || storedKey != legacyKey || indexedStatus != "closed" {
		t.Fatalf("legacy operation identity or indexed status changed incorrectly: %s %s %v", storedKey, indexedStatus, err)
	}
	submitted, err := h.service.Attempt(h.ctx, h.students[0], a.AttemptID, pgPracticumCommand("closed-legacy-attempt"))
	if err != nil || submitted.SubmissionReason != "closed" {
		t.Fatalf("legacy activation close did not atomically collect work: %+v %v", submitted, err)
	}
}
