package usecase_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type fakeHintGenerator struct {
	response string
	err      error
	calls    int
	system   string
}

func (f *fakeHintGenerator) Complete(_ context.Context, system, user string) (string, error) {
	f.calls++
	f.system = system
	_ = user
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

type learningP1Harness struct {
	t        *testing.T
	ctx      context.Context
	identity *repository.MemoryIdentityRepository
	svc      *usecase.LearningService
	repo     *repository.MemoryLearningRepository
	now      time.Time
	mu       sync.Mutex
}

func (h *learningP1Harness) advance(d time.Duration) {
	h.mu.Lock()
	h.now = h.now.Add(d)
	h.mu.Unlock()
}

func (h *learningP1Harness) clock() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.now
}

func newLearningP1Harness(t *testing.T) *learningP1Harness {
	t.Helper()
	ctx := context.Background()
	h := &learningP1Harness{t: t, ctx: ctx, now: time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)}
	h.identity = repository.NewMemoryIdentityRepository()
	h.repo = repository.NewMemoryLearningRepository(h.clock)
	if err := repository.SeedLearningFixtures(ctx, h.repo); err != nil {
		t.Fatal(err)
	}
	h.svc = usecase.NewLearningService(usecase.LearningOptions{
		Repository:                 h.repo,
		IdentityRepository:         h.identity,
		Runner:                     runner.NewFixture(),
		AllowSimulatedVerification: true,
	})
	return h
}

func (h *learningP1Harness) principal(id int64) *entity.Principal {
	h.t.Helper()
	user, err := h.identity.FindUserByID(h.ctx, id)
	if err != nil {
		h.t.Fatal(err)
	}
	assignments, err := h.identity.ListAssignments(h.ctx, id)
	if err != nil {
		h.t.Fatal(err)
	}
	grants, err := h.identity.ListPermissionGrants(h.ctx, id)
	if err != nil {
		h.t.Fatal(err)
	}
	return &entity.Principal{User: user, Assignments: assignments, Grants: grants}
}

func ptrInt(v int) *int { return &v }

func TestLearningP1WeeklyLeaderboardWIBBoundaryAndSnapshotStability(t *testing.T) {
	h := newLearningP1Harness(t)
	student := h.principal(1)
	command := usecase.LearningCommand{Key: "complete-week-1", RequestID: "req-week", OperationID: "learning.lessons.progress"}
	view, err := h.svc.SaveProgress(h.ctx, student, command, "lesson-1", usecase.LearningProgressInput{
		Language: "c", Complete: true, QuizOption: ptrInt(0),
		DraftCode:         "#include <stdio.h>\nint main(void) {\n  return 0;\n}\n",
		BrowserTestPassed: true, LessonVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Profile.XP != 60 {
		t.Fatalf("expected 60 XP, got %d", view.Profile.XP)
	}

	if _, err = h.svc.ReconcileWeeklyStandings(h.ctx, usecase.LearningCommand{RequestID: "req-cron", OperationID: "learning.internal.standings"}); err != nil {
		t.Fatal(err)
	}
	snapshots, err := h.svc.WeeklyStandings(h.ctx, student, usecase.PracticumPage{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	first := snapshots[0]
	if first.WeekStart != "2026-11-09" {
		t.Fatalf("expected week_start 2026-11-09, got %q", first.WeekStart)
	}
	if len(first.Rows) == 0 || first.Rows[0].UserID != 1 || first.Rows[0].XP != 60 {
		t.Fatalf("expected student 1 top rank with 60 XP, got %+v", first.Rows)
	}

	// Cross Monday 00:00 WIB (Sunday 17:00 UTC) into next week.
	h.advance(6 * 24 * time.Hour)
	if _, err = h.svc.ReconcileWeeklyStandings(h.ctx, usecase.LearningCommand{RequestID: "req-cron-2", OperationID: "learning.internal.standings"}); err != nil {
		t.Fatal(err)
	}
	snapshots, err = h.svc.WeeklyStandings(h.ctx, student, usecase.PracticumPage{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
	if snapshots[1].WeekStart != first.WeekStart || snapshots[1].Rows[0].XP != 60 {
		t.Fatalf("history snapshot mutated: %+v vs %+v", snapshots[1], first)
	}
}

func TestLearningP1AdminXPSetResetAuditedAndIdempotent(t *testing.T) {
	h := newLearningP1Harness(t)
	student := h.principal(1)
	koordinator := h.principal(9)
	command := func(key string) usecase.LearningCommand {
		return usecase.LearningCommand{Key: key, RequestID: key, OperationID: "learning.admin.xp.set"}
	}
	view, err := h.svc.SaveProgress(h.ctx, student, usecase.LearningCommand{Key: "complete-xp", RequestID: "req-xp", OperationID: "learning.lessons.progress"}, "lesson-1", usecase.LearningProgressInput{
		Language: "c", Complete: true, QuizOption: ptrInt(0),
		DraftCode:         "#include <stdio.h>\nint main(void) {\n  return 0;\n}\n",
		BrowserTestPassed: true, LessonVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Profile.XP != 60 {
		t.Fatalf("expected 60 XP, got %d", view.Profile.XP)
	}

	if _, err = h.svc.SetXP(h.ctx, student, command("student-deny"), usecase.LearningXPSetInput{UserID: 1, XP: 10, Reason: "self bump"}); err == nil {
		t.Fatal("student set XP should be denied")
	}

	set, err := h.svc.SetXP(h.ctx, koordinator, command("set-250"), usecase.LearningXPSetInput{UserID: 1, XP: 250, Reason: "koreksi manual"})
	if err != nil {
		t.Fatal(err)
	}
	if set.XP != 250 || set.Level != usecase.CalculateLearningLevel(250) {
		t.Fatalf("unexpected profile after set: %+v", set)
	}
	replay, err := h.svc.SetXP(h.ctx, koordinator, command("set-250"), usecase.LearningXPSetInput{UserID: 1, XP: 250, Reason: "koreksi manual"})
	if err != nil || replay.XP != 250 {
		t.Fatalf("replay set: %+v, %v", replay, err)
	}

	reset, err := h.svc.ResetXP(h.ctx, koordinator, usecase.LearningCommand{Key: "reset-xp", RequestID: "req-reset", OperationID: "learning.admin.xp.reset"}, 1, "reset semester")
	if err != nil {
		t.Fatal(err)
	}
	if reset.XP != 0 {
		t.Fatalf("expected reset XP 0, got %d", reset.XP)
	}

	audits, err := h.identity.ListAudit(h.ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	setFound, resetFound := false, false
	for _, a := range audits {
		if a.Action == "learning.xp.set" && a.SubjectID == "profile-1" {
			setFound = true
		}
		if a.Action == "learning.xp.reset" {
			resetFound = true
		}
	}
	if !setFound || !resetFound {
		t.Fatalf("audit entries missing: set=%v reset=%v", setFound, resetFound)
	}
}

func TestLearningP1AchievementCatalogCRUDAndPermissions(t *testing.T) {
	h := newLearningP1Harness(t)
	koordinator := h.principal(9)
	student := h.principal(1)
	command := func(key string) usecase.LearningCommand {
		return usecase.LearningCommand{Key: key, RequestID: key, OperationID: "learning.admin.achievements"}
	}
	created, err := h.svc.SaveAchievement(h.ctx, koordinator, command("create-ach"), "", usecase.LearningAchievementUpdate{
		LearningAchievementInput: usecase.LearningAchievementInput{
			Title: "Master Pointer", Description: "Menyelesaikan modul pointer.",
			RequirementType: "manual", RequirementValue: "", XPReward: 100, Active: true,
			Scope: entity.Scope{Domain: "elearning", Institution: "lab", Division: "pendidikan"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !created.Active || created.Version != 1 {
		t.Fatalf("unexpected created achievement: %+v", created)
	}

	if _, err = h.svc.SaveAchievement(h.ctx, student, command("student-create"), "", usecase.LearningAchievementUpdate{
		LearningAchievementInput: usecase.LearningAchievementInput{
			Title: "Hack", Description: "x", RequirementType: "manual", Scope: entity.Scope{Domain: "elearning"},
		},
	}); err == nil {
		t.Fatal("student achievement create should be denied")
	}

	updated, err := h.svc.SaveAchievement(h.ctx, koordinator, command("toggle-ach"), created.ID, usecase.LearningAchievementUpdate{
		LearningAchievementInput: usecase.LearningAchievementInput{
			Title: created.Title, Description: created.Description,
			RequirementType: "manual", XPReward: 100, Active: false,
			Scope: created.Scope,
		},
		Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Active || updated.Version != 2 {
		t.Fatalf("toggle failed: %+v", updated)
	}

	list, err := h.svc.AdminAchievements(h.ctx, koordinator)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("unexpected catalog: %+v", list)
	}
}

func TestLearningP1HintAIWithFakeGrader(t *testing.T) {
	h := newLearningP1Harness(t)
	student := h.principal(1)
	gen := &fakeHintGenerator{response: "Perhatikan struktur fungsi main dan format printf."}
	h.svc = usecase.NewLearningService(usecase.LearningOptions{
		Repository: h.repo, IdentityRepository: h.identity, HintGenerator: gen, AllowSimulatedVerification: true,
	})
	out, err := h.svc.RequestAIHint(h.ctx, student, usecase.LearningCommand{Key: "hint-1", RequestID: "req-hint", OperationID: "learning.hint.ai"}, usecase.LearningHintRequest{
		LessonID: "lesson-1", Question: "Bagaimana memulai program C?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gen.calls != 1 {
		t.Fatalf("expected generator call, got %d", gen.calls)
	}
	if !strings.Contains(gen.system, "JANGAN pernah menuliskan jawaban final") || !strings.Contains(gen.system, "kunci kuis") {
		t.Fatalf("system prompt must forbid answers: %q", gen.system)
	}
	if out.Hint == "" || out.Simulated {
		t.Fatalf("unexpected hint response: %+v", out)
	}

	bare := usecase.NewLearningService(usecase.LearningOptions{Repository: h.repo, IdentityRepository: h.identity})
	_, err = bare.RequestAIHint(h.ctx, student, usecase.LearningCommand{Key: "hint-2", RequestID: "req-hint2", OperationID: "learning.hint.ai"}, usecase.LearningHintRequest{LessonID: "lesson-1"})
	if !errors.Is(err, usecase.ErrLearningHintGenerator) {
		t.Fatalf("expected ErrLearningHintGenerator, got %v", err)
	}

	failing := usecase.NewLearningService(usecase.LearningOptions{
		Repository: h.repo, IdentityRepository: h.identity,
		HintGenerator: &fakeHintGenerator{err: errors.New("boom")},
	})
	_, err = failing.RequestAIHint(h.ctx, student, usecase.LearningCommand{Key: "hint-3", RequestID: "req-hint3", OperationID: "learning.hint.ai"}, usecase.LearningHintRequest{LessonID: "lesson-1"})
	if !errors.Is(err, usecase.ErrLearningHintFailed) {
		t.Fatalf("expected ErrLearningHintFailed, got %v", err)
	}
}

func TestLearningP1PrestigeGrantAndHistory(t *testing.T) {
	h := newLearningP1Harness(t)
	koordinator := h.principal(9)
	student := h.principal(1)
	entry, err := h.svc.GrantPrestige(h.ctx, koordinator, usecase.LearningCommand{Key: "grant-1", RequestID: "req-prestige", OperationID: "learning.admin.prestige.grant"}, usecase.LearningPrestigeGrantInput{UserID: 1, Amount: 25, Reason: "kontribusi lab"})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Amount != 25 || entry.Source != "admin_grant" {
		t.Fatalf("unexpected prestige entry: %+v", entry)
	}
	history, err := h.svc.Prestige(h.ctx, student, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != entry.ID {
		t.Fatalf("unexpected prestige history: %+v", history)
	}
	if _, err = h.svc.GrantPrestige(h.ctx, student, usecase.LearningCommand{Key: "grant-2", RequestID: "req-prestige-2", OperationID: "learning.admin.prestige.grant"}, usecase.LearningPrestigeGrantInput{UserID: 1, Amount: 5, Reason: "self"}); err == nil {
		t.Fatal("student prestige grant should be denied")
	}
}
