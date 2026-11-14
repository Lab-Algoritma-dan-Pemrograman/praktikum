package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// SeedPracticumFixtures is opt-in local/test data, never a migration or normal
// constructor side effect. Identity user 1 must already exist in PostgreSQL.
func SeedPracticumFixtures(ctx context.Context, repo PracticumRepository) error {
	return repo.Transaction(ctx, func(tx PracticumTx) error {
		if _, err := tx.Get(PracticumSessions, "session-1"); err == nil {
			return nil
		} else if !errors.Is(err, ErrPracticumNotFound) {
			return err
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		scope := entity.Scope{Domain: "praktikum", Institution: "lab", Division: "pendidikan", SessionID: "session-1"}
		session := entity.PracticumSession{ID: "session-1", Title: "Sesi Praktikum Fixture", Sequence: 1, Scope: scope, Active: true, Version: 1, CreatedAt: now, UpdatedAt: now}
		put := func(kind PracticumKind, id, parent, unique string, value any, recordScope entity.Scope) error {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return tx.Put(kind, PracticumRecord{ID: id, ParentID: parent, UniqueKey: unique, Scope: recordScope, Data: raw})
		}
		if err = put(PracticumSessions, session.ID, "", "", session, scope); err != nil {
			return err
		}
		var course entity.PracticumCourse
		for _, kind := range []string{"pretest", "posttest", "keterampilan", "ujian_praktik"} {
			c := entity.PracticumCourse{ID: "course-" + kind, SessionID: session.ID, Title: "Fixture " + kind, Kind: kind, DurationMinutes: 30, Version: 1}
			if kind == "pretest" {
				course = c
			}
			if err = put(PracticumCourses, c.ID, session.ID, session.ID+":"+kind, c, scope); err != nil {
				return err
			}
		}
		questions := []entity.PracticumQuestion{
			{ID: "question-essay", CourseID: course.ID, Type: "essay", Difficulty: "easy", Category: "module", Prompt: "Jelaskan langkah algoritma untuk menjumlahkan dua bilangan.", Points: 10, ReferenceAnswer: "Baca dua bilangan, jumlahkan, tampilkan hasil.", Rubric: []entity.Criterion{{ID: "logic", Description: "Urutan algoritma tepat", MaxScore: 10}}, Version: 1},
			{ID: "question-coding", CourseID: course.ID, Type: "coding", Difficulty: "easy", Category: "module", Prompt: "Tulis program yang menampilkan jumlah dua bilangan.", Points: 10, Language: "c", StarterCode: "#include <stdio.h>\nint main(void) {\n  return 0;\n}\n", ReferenceAnswer: "Use an addition expression and print the result.", Rubric: []entity.Criterion{{ID: "implementation", Description: "Implementasi penjumlahan tepat", MaxScore: 10}}, Version: 1},
		}
		for _, q := range questions {
			if err = put(PracticumQuestions, q.ID, course.ID, "", q, scope); err != nil {
				return err
			}
		}
		activationScope := scope
		activationScope.ClassID = "class-101"
		activationScope.Shift = 1
		a := entity.PracticumActivationRecord{Activation: entity.PracticumActivation{ID: "activation-1", SessionID: session.ID, CourseID: course.ID, Scope: activationScope, TimerMode: "shared", IsOpen: true, QuestionCount: 2, Version: 1, StudentIDs: []int64{1}, EntryApprovedIDs: []int64{}, QuestionIDs: []string{"question-essay", "question-coding"}}, Course: course, Questions: questions}
		return put(PracticumActivations, a.Activation.ID, session.ID, "", a, activationScope)
	})
}
