package usecase_test

import (
	"fmt"
	"testing"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

// Normal authoring must be exercised without SeedLearningFixtures: its fixed
// "level-1" ID otherwise hides whether the Level 1 achievement is reachable.
func TestLearningLevelOneAchievementForNormallyAuthoredCurriculum(t *testing.T) {
	for _, tc := range []struct {
		name              string
		order, extraOrder int
		wantUnlocked      bool
	}{
		{name: "unique order one", order: 1, wantUnlocked: true},
		{name: "no order one", order: 2},
		{name: "ambiguous order one", order: 1, extraOrder: 1},
		{name: "unrelated other level", order: 1, extraOrder: 2, wantUnlocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			identity := repository.NewMemoryIdentityRepository()
			repo := repository.NewMemoryLearningRepository()
			svc := usecase.NewLearningService(usecase.LearningOptions{
				Repository:         repo,
				IdentityRepository: identity,
			})
			principal := func(id int64) *entity.Principal {
				t.Helper()
				user, err := identity.FindUserByID(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				assignments, err := identity.ListAssignments(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				grants, err := identity.ListPermissionGrants(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				return &entity.Principal{User: user, Assignments: assignments, Grants: grants}
			}
			instructor, student := principal(7), principal(1)
			command := func(key string) usecase.LearningCommand {
				return usecase.LearningCommand{Key: key, RequestID: key, OperationID: "learning.achievement.local-regression"}
			}

			level, err := svc.CreateLevel(ctx, instructor, command("create-level-one"), usecase.LearningLevelInput{
				Title:      "Level 1 - Dasar C",
				SortOrder:  tc.order,
				AccessMode: "unlocked",
				Scope:      entity.Scope{Domain: "elearning", Institution: "lab", Division: "pendidikan"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.extraOrder != 0 {
				_, err := svc.CreateLevel(ctx, instructor, command("create-extra-level"), usecase.LearningLevelInput{
					Title: "Other level", SortOrder: tc.extraOrder, AccessMode: "unlocked", Scope: level.Scope,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			module, err := svc.CreateModule(ctx, instructor, command("create-module-one"), usecase.LearningModuleInput{
				LevelID: level.ID, Title: "Dasar C", SortOrder: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			lessons := make([]entity.LearningLesson, 0, 2)
			for order := 1; order <= 2; order++ {
				lesson, err := svc.SaveLesson(ctx, instructor, command(fmt.Sprintf("create-lesson-%d", order)), "", usecase.LearningLessonUpdate{
					LearningLessonInput: usecase.LearningLessonInput{
						ModuleID: module.ID, Title: fmt.Sprintf("Materi dasar %d", order), SortOrder: order,
						Language: "c", Explanation: "Materi teks tanpa latihan kode.", XPReward: 60,
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				lessons = append(lessons, lesson)
			}

			levelAchievement := func() entity.LearningAchievement {
				t.Helper()
				view, err := svc.Achievements(ctx, student)
				if err != nil {
					t.Fatal(err)
				}
				for _, achievement := range view.Achievements {
					if achievement.ID == "level-1-complete" {
						return achievement
					}
				}
				t.Fatal("documented Level 1 achievement is missing")
				return entity.LearningAchievement{}
			}
			for i, lesson := range lessons {
				view, err := svc.SaveProgress(ctx, student, command(fmt.Sprintf("complete-lesson-%d", i+1)), lesson.ID, usecase.LearningProgressInput{
					Language: "c", Complete: true,
				})
				if err != nil {
					t.Fatal(err)
				}
				if view.Progress.Status != "completed" || view.Profile.CompletedLessons != i+1 || view.Profile.XP != (i+1)*60 {
					t.Fatalf("lesson completion did not persist expected progress/XP: %+v", view)
				}
				if i == 0 && levelAchievement().Unlocked {
					t.Fatal("Level 1 achievement unlocked before every lesson was completed")
				}
			}
			levels, err := svc.Levels(ctx, student)
			if err != nil {
				t.Fatal(err)
			}
			var completedLevel *entity.LearningLevel
			for i := range levels.Levels {
				if levels.Levels[i].ID == level.ID {
					completedLevel = &levels.Levels[i]
				}
			}
			if completedLevel == nil || completedLevel.CompletedLessons != 2 || completedLevel.TotalLessons != 2 {
				t.Fatalf("normally authored level is not fully completed: %+v", levels.Levels)
			}
			achievement := levelAchievement()
			if achievement.Unlocked != tc.wantUnlocked {
				t.Fatalf("level %q completion: unlocked=%v, want=%v, target=%q", level.ID, achievement.Unlocked, tc.wantUnlocked, achievement.RequirementValue)
			}
			wantTarget := "unconfigured-level-1"
			if tc.wantUnlocked {
				wantTarget = level.ID
			}
			if achievement.RequirementValue != wantTarget {
				t.Fatalf("achievement target=%q, want=%q", achievement.RequirementValue, wantTarget)
			}
			input := usecase.LearningProgressInput{Language: "c", Complete: true}
			replay, err := svc.SaveProgress(ctx, student, command("complete-lesson-2"), lessons[1].ID, input)
			if err != nil || replay.Profile.XP != 120 {
				t.Fatalf("completion replay: %+v, %v", replay, err)
			}
			input.Revision = replay.Progress.Revision
			again, err := svc.SaveProgress(ctx, student, command("complete-again"), lessons[1].ID, input)
			if err != nil || again.Profile.XP != 120 {
				t.Fatalf("repeated completion: %+v, %v", again, err)
			}
			count := 0
			for _, unlocked := range again.Profile.UnlockedAchievements {
				if unlocked.AchievementID == "level-1-complete" {
					count++
				}
			}
			wantCount := 0
			if tc.wantUnlocked {
				wantCount = 1
			}
			if count != wantCount {
				t.Fatalf("achievement count=%d, want=%d", count, wantCount)
			}
		})
	}
}
