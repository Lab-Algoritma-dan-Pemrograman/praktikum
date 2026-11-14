package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

func TestLearningProgressionIdempotencyStreakAndQuotas(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	identity := repository.NewMemoryIdentityRepository()
	repo := repository.NewMemoryLearningRepository(clock)
	if err := repository.SeedLearningFixtures(ctx, repo); err != nil {
		t.Fatal(err)
	}
	svc := usecase.NewLearningService(usecase.LearningOptions{
		Repository:                 repo,
		IdentityRepository:         identity,
		Runner:                     runner.NewFixture(),
		AllowSimulatedVerification: true,
	})

	studentUser, _ := identity.FindUserByID(ctx, 1)
	studentGrants, _ := identity.ListPermissionGrants(ctx, 1)
	studentAssignments, _ := identity.ListAssignments(ctx, 1)
	student := &entity.Principal{User: studentUser, Assignments: studentAssignments, Grants: studentGrants}

	borrowerUser, _ := identity.FindUserByID(ctx, 10)
	borrowerAssignments, _ := identity.ListAssignments(ctx, 10)
	borrower := &entity.Principal{User: borrowerUser, Assignments: borrowerAssignments}

	if _, err := svc.Levels(ctx, borrower); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected forbidden for peminjam without learning.content.read, got %v", err)
	}

	// Level 2 requires 60 XP; initially locked for student.
	if _, err := svc.Lesson(ctx, student, "lesson-2"); !errors.Is(err, usecase.ErrLearningLocked) {
		t.Fatalf("expected locked lesson-2, got %v", err)
	}

	optZero := 0
	lesson1, err := svc.Lesson(ctx, student, "lesson-1")
	if err != nil {
		t.Fatal(err)
	}
	// Concurrent completion attempts on lesson-1 must award 60 XP at most once.
	errs := make(chan error, 10)
	for i := range 10 {
		go func() {
			_, err := svc.SaveProgress(ctx, student, usecase.LearningCommand{Key: fmt.Sprintf("complete-%d", i), RequestID: "req-1", OperationID: "learning.lessons.progress"}, "lesson-1", usecase.LearningProgressInput{
				Revision:          0,
				DraftCode:         "#include <stdio.h>\nint main(void) { return 0; }",
				Language:          "c",
				QuizOption:        &optZero,
				Complete:          true,
				BrowserTestPassed: true,
				LessonVersion:     lesson1.Lesson.Version,
			})
			errs <- err
		}()
	}
	wins := 0
	for range 10 {
		err := <-errs
		if err == nil {
			wins++
		} else if !errors.Is(err, repository.ErrConflict) {
			t.Fatalf("unexpected concurrent completion error: %v", err)
		}
	}
	if wins != 1 {
		t.Fatalf("expected 1 revision winner, got %d", wins)
	}

	// Re-completing lesson-1 on revision 1 must be idempotent with 0 extra XP.
	view, err := svc.SaveProgress(ctx, student, usecase.LearningCommand{Key: "complete-again", RequestID: "req-2", OperationID: "learning.lessons.progress"}, "lesson-1", usecase.LearningProgressInput{
		Revision:   1,
		DraftCode:  "#include <stdio.h>\nint main(void) { return 0; }",
		Language:   "c",
		QuizOption: &optZero,
		Complete:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Profile.XP != 60 || view.Profile.CompletedLessons != 1 || view.Profile.Streak != 1 {
		t.Fatalf("duplicate completion inflated XP or count: %+v", view.Profile)
	}

	// Advance 24 hours (next Jakarta calendar day) and complete lesson-2 (now unlocked at 60 XP).
	advance(24 * time.Hour)
	optOne := 1
	lesson2, err := svc.Lesson(ctx, student, "lesson-2")
	if err != nil {
		t.Fatal(err)
	}
	view2, err := svc.SaveProgress(ctx, student, usecase.LearningCommand{Key: "complete-lesson-2", RequestID: "req-3", OperationID: "learning.lessons.progress"}, "lesson-2", usecase.LearningProgressInput{
		Revision:          0,
		DraftCode:         "nilai = 80\nif nilai >= 75:\n    print('Lulus')",
		Language:          "python",
		QuizOption:        &optOne,
		Complete:          true,
		BrowserTestPassed: true,
		LessonVersion:     lesson2.Lesson.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view2.Profile.XP != 140 || view2.Profile.CompletedLessons != 2 || view2.Profile.Streak != 2 {
		t.Fatalf("expected 140 XP, 2 completed lessons, 2-day streak; got %+v", view2.Profile)
	}

	// Advance another 24 hours and solve Bug Hunt challenge to reach 3-day streak and unlock streak-3.
	advance(24 * time.Hour)
	bh, err := svc.SubmitBugHunt(ctx, student, usecase.LearningCommand{Key: "bh-1", RequestID: "req-4", OperationID: "learning.bughunt.attempt"}, usecase.LearningBugHuntAttemptInput{
		QuestionID:   "bughunt-c-1",
		SelectedLine: 4,
		Explanation:  "Assignment = dipakai pada kondisi if",
	})
	if err != nil {
		t.Fatal(err)
	}
	if bh.Profile.Streak != 3 || bh.Profile.XP != 190 {
		t.Fatalf("expected 3-day streak and 190 XP, got %+v", bh.Profile)
	}
	ach, err := svc.Achievements(ctx, student)
	if err != nil {
		t.Fatal(err)
	}
	unlockedCount := 0
	for _, a := range ach.Achievements {
		if a.Unlocked {
			unlockedCount++
		}
	}
	if unlockedCount != 5 {
		t.Fatalf("expected all 5 achievements unlocked, got %d (%+v)", unlockedCount, ach.Achievements)
	}
}
