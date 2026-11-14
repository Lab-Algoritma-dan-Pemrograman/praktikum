package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/application"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/migrationrehearsal"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPhase6LiveImportedFlows is a local acceptance test for a disposable copy
// restored after the live, masked import. It does not migrate, seed, clean, or
// contact Supabase/a browser. The test intentionally mutates its restored copy.
func TestPhase6LiveImportedFlows(t *testing.T) {
	if os.Getenv("ALGOHUB_PHASE6_LIVE_FLOWS") != "true" {
		t.Skip("requires ALGOHUB_PHASE6_LIVE_FLOWS=true and a disposable restored live-import test database")
	}

	databaseURL := os.Getenv("ALGOHUB_TEST_DATABASE_URL")
	acceptanceEmail := strings.ToLower(strings.TrimSpace(os.Getenv("PHASE6_ACCEPTANCE_EMAIL")))
	acceptancePassword := os.Getenv("PHASE6_ACCEPTANCE_PASSWORD")
	if acceptanceEmail == "" || len(acceptancePassword) < 16 {
		t.Fatal("PHASE6_ACCEPTANCE_EMAIL and a private acceptance password of at least 16 characters are required")
	}
	if !strings.HasPrefix(acceptanceEmail, "migration.") || !strings.HasSuffix(acceptanceEmail, "@example.test") {
		t.Fatal("PHASE6_ACCEPTANCE_EMAIL must identify a masked migration acceptance user")
	}

	poolConfig := livePhase6PoolConfig(t, databaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var target migrationrehearsal.LocalTarget
	file, err := os.Open(os.Getenv("ALGOHUB_PHASE6_TARGET_FILE"))
	if err != nil {
		t.Fatal("owned recovery target descriptor required")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 16<<10))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&target)
	file.Close()
	if err != nil {
		t.Fatal("invalid owned recovery target descriptor")
	}
	ctx = migrationrehearsal.WithLocalTarget(ctx, target)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal("could not initialize the local live-import acceptance pool")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal("could not connect to the restored local live-import acceptance database")
	}

	if err = migrationrehearsal.VerifyLocalTarget(ctx, pool, target); err != nil {
		t.Fatal("recovery database ownership mismatch")
	}
	contract := livePhase6VerifyRestoredContract(t, ctx, pool)
	baselineUsers := livePhase6UserSnapshot(t, ctx, pool)
	baselineUnrelatedUsers := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users where email not like 'migration.%@example.test'`)
	baselineUserCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users`)
	baselineAssignmentCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.user_role_assignments`)
	baselineAuthUserCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from auth.users`)
	baselineRentalCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_inventory_rentals`)

	identity := repository.NewPostgresIdentityRepository(pool)
	var acceptanceUserID int64
	var acceptanceAuthID string
	if err = pool.QueryRow(ctx, `
		select u.id, u.auth_user_id::text
		from public.users u
		join public.user_role_assignments a on a.user_id=u.id and a.role_code='mahasiswa' and a.is_active
		where lower(u.email)=$1 and u.is_active and not u.hidden`, acceptanceEmail).Scan(&acceptanceUserID, &acceptanceAuthID); err != nil {
		t.Fatal("the configured acceptance email is not one active imported mahasiswa identity")
	}
	var coordinatorUserID int64
	if err = pool.QueryRow(ctx, `
		select u.id
		from public.users u
		join public.user_role_assignments a on a.user_id=u.id and a.role_code='koordinator' and a.is_active
		where u.email like 'migration.%@example.test' and u.is_active and not u.hidden
		order by u.id limit 1`).Scan(&coordinatorUserID); err != nil {
		t.Fatal("no active imported koordinator identity is available")
	}
	student := livePhase6Principal(t, ctx, identity, acceptanceUserID, entity.RoleMahasiswa)
	coordinator := livePhase6Principal(t, ctx, identity, coordinatorUserID, entity.RoleKoordinator)

	learning := usecase.NewLearningService(usecase.LearningOptions{
		Repository:         repository.NewPostgresLearningRepository(pool),
		IdentityRepository: identity,
	})
	practicum := usecase.NewPracticumService(usecase.PracticumOptions{
		Repository:         repository.NewPostgresPracticumRepository(pool),
		IdentityRepository: identity,
	})
	academic := usecase.NewAcademicService(usecase.AcademicOptions{
		Repository:         repository.NewPostgresAcademicRepository(pool),
		IdentityRepository: identity,
	})
	page := usecase.PracticumPage{Limit: 100}
	baselineReport, err := academic.Reports(ctx, coordinator)
	if err != nil {
		t.Fatalf("cannot read the baseline scoped academic report: %v", err)
	}

	var importedClassID, importedScheduleID, localAttendanceSessionID string
	var importedItemID, localRentalID string
	var localRentalFee int

	t.Run("imported_identity_and_access_reads", func(t *testing.T) {
		if student.User.ID != acceptanceUserID || student.User.AuthUserID != acceptanceAuthID || student.User.Email != acceptanceEmail {
			t.Fatal("mahasiswa repository read did not preserve the original imported identity")
		}
		if coordinator.User.ID != coordinatorUserID || !strings.HasPrefix(coordinator.User.Email, "migration.") || !strings.HasSuffix(coordinator.User.Email, "@example.test") {
			t.Fatal("koordinator repository read did not preserve the original imported identity")
		}
		for _, check := range []struct {
			principal  *entity.Principal
			permission string
		}{
			{student, "learning.content.read"},
			{coordinator, "practicum.grade.write"},
			{coordinator, "schedule.write"},
			{coordinator, "rental.approve"},
			{coordinator, "finance.manage"},
		} {
			if !livePhase6HasGrant(check.principal, check.permission) {
				t.Fatalf("imported identity is missing expected access grant %s", check.permission)
			}
		}
	})

	t.Run("local_fake_provider_acceptance_login_and_cross_instance_sso_not_real_supabase_or_browser", func(t *testing.T) {
		var passwordUsers int
		if err := pool.QueryRow(ctx, `
			select count(*)
			from auth.users a join public.users u on u.auth_user_id=a.id
			where u.email like 'migration.%@example.test'
			  and coalesce(a.encrypted_password,'')<>''`).Scan(&passwordUsers); err != nil || passwordUsers != 1 {
			t.Fatalf("expected exactly one imported auth identity with an acceptance password: count=%d err=%v", passwordUsers, err)
		}
		var storedAuthID string
		var passwordMatches bool
		if err := pool.QueryRow(ctx, `
			select a.id::text,
			       a.encrypted_password=crypt($2,a.encrypted_password)
			from auth.users a join public.users u on u.auth_user_id=a.id
			where lower(u.email)=$1 and coalesce(a.encrypted_password,'')<>''`, acceptanceEmail, acceptancePassword).Scan(&storedAuthID, &passwordMatches); err != nil || !passwordMatches || storedAuthID != acceptanceAuthID {
			t.Fatal("the harness-supplied acceptance password does not match the single imported auth identity")
		}
		livePhase6FakeProviderLoginAndSSO(t, ctx, databaseURL, acceptanceEmail, acceptancePassword, student)
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users`); got != baselineUserCount {
			t.Fatalf("local acceptance login created a replacement public identity: before=%d after=%d", baselineUserCount, got)
		}
		got, err := identity.FindUserByAuthID(ctx, acceptanceAuthID)
		if err != nil || got.ID != acceptanceUserID {
			t.Fatalf("local acceptance login lost the imported identity: %v", err)
		}
	})

	t.Run("imported_completed_learning_replay_preserves_opening_balance_xp", func(t *testing.T) {
		var progressID, profileID, lessonID string
		var progressRaw, profileRaw, lessonRaw []byte
		if err := pool.QueryRow(ctx, `
			select p.id,p.data,pr.id,pr.data,l.id,l.data
			from public.learning_progress p
			join public.learning_lessons l on l.id=p.parent_id
			join public.learning_profiles pr on pr.owner_id=p.owner_id
			join public.users u on u.id=p.owner_id
			where u.email like 'migration.%@example.test'
			  and p.status='completed' and (p.data->>'xp_awarded')::int=0
			order by (p.owner_id=$1) desc,p.id limit 1`, acceptanceUserID).Scan(&progressID, &progressRaw, &profileID, &profileRaw, &lessonID, &lessonRaw); err != nil {
			t.Fatalf("cannot select an imported completed learning record: %v", err)
		}
		var progress entity.LearningProgress
		var profile entity.LearningProfile
		var lesson entity.LearningLesson
		livePhase6Decode(t, progressRaw, &progress)
		livePhase6Decode(t, profileRaw, &profile)
		livePhase6Decode(t, lessonRaw, &lesson)
		if !strings.HasPrefix(lessonID, "mig-les-") || !strings.HasPrefix(lesson.ModuleID, "mig-mod-") || !strings.HasPrefix(lesson.LevelID, "mig-lvl-") || lesson.ID != lessonID || progressID != fmt.Sprintf("progress-%s-%d", lessonID, progress.UserID) || profileID != fmt.Sprintf("profile-%d", progress.UserID) {
			t.Fatal("learning selection did not retain the original imported IDs")
		}
		if progress.Status != "completed" || progress.XPAwarded != 0 || progress.CompletedAt == nil || profile.UserID != progress.UserID {
			t.Fatal("selected learning state does not implement the imported opening-balance policy")
		}
		learner := livePhase6Principal(t, ctx, identity, progress.UserID, entity.RoleMahasiswa)
		before, err := learning.Lesson(ctx, learner, lessonID)
		if err != nil || before.Lesson.ID != lessonID || before.Progress.Revision != progress.Revision || before.Profile.UserID != profile.UserID {
			t.Fatalf("usecase did not read the imported lesson/progress/profile: %v", err)
		}

		draft := progress.DraftCode
		codeRequired := lesson.StarterCode != "" || len(lesson.TestCases) > 0
		if codeRequired && strings.TrimSpace(draft) == "" {
			for _, candidate := range []string{lesson.StarterCode, lesson.CodeExample, lesson.Solution} {
				if strings.TrimSpace(candidate) != "" {
					draft = candidate
					break
				}
			}
			if strings.TrimSpace(draft) == "" {
				t.Fatal("imported code lesson has no imported code detail that can be replayed")
			}
		}
		var quizOption *int
		if lesson.Quiz != nil {
			if lesson.Quiz.CorrectOption == nil {
				t.Fatal("imported quiz omitted its accepted option")
			}
			option := *lesson.Quiz.CorrectOption
			quizOption = &option
		}
		input := usecase.LearningProgressInput{
			Revision:          progress.Revision,
			DraftCode:         draft,
			Language:          lesson.Language,
			QuizOption:        quizOption,
			Complete:          true,
			BrowserTestPassed: codeRequired,
		}
		if codeRequired {
			// This is the accepted browser assertion/version contract. No browser or
			// remote runner is invoked by this local acceptance test.
			input.LessonVersion = lesson.Version
		}
		command := usecase.LearningCommand{Key: "phase6-live-learning-complete", RequestID: "phase6-live-learning", OperationID: "phase6-live-learning"}
		first, err := learning.SaveProgress(ctx, learner, command, lessonID, input)
		if err != nil {
			t.Fatalf("replaying imported completion failed: %v", err)
		}
		replay, err := learning.SaveProgress(ctx, learner, command, lessonID, input)
		if err != nil {
			t.Fatalf("idempotent imported completion replay failed: %v", err)
		}
		for _, got := range []entity.LearningLessonView{first, replay} {
			if got.Lesson.ID != lessonID || got.Progress.Status != "completed" || got.Progress.Revision != progress.Revision+1 || got.Progress.XPAwarded != 0 || got.Profile.XP != profile.XP || got.Profile.CompletedLessons != profile.CompletedLessons {
				t.Fatal("completion replay changed opening-balance XP or duplicated completion")
			}
		}
		storedProgress := livePhase6ReadData[entity.LearningProgress](t, ctx, pool, "learning_progress", progressID)
		storedProfileRaw := livePhase6ReadDataRaw(t, ctx, pool, "learning_profiles", profileID)
		if storedProgress.LessonID != lessonID || storedProgress.Revision != progress.Revision+1 || storedProgress.XPAwarded != 0 || storedProgress.CompletedAt == nil || !storedProgress.CompletedAt.Equal(*progress.CompletedAt) || string(storedProfileRaw) != string(profileRaw) {
			t.Fatal("stored learning replay duplicated XP, replaced an imported row, or changed the opening-balance profile")
		}
	})

	t.Run("imported_practicum_manual_grade_publish_and_replay", func(t *testing.T) {
		var attemptID, jobID string
		var attemptRaw, jobRaw []byte
		if err := pool.QueryRow(ctx, `
			select a.id,a.data,j.id,j.data
			from public.practicum_attempts a
			join public.practicum_grading_jobs j on j.parent_id=a.id
			join public.users u on u.id=a.owner_id
			where u.email like 'migration.%@example.test'
			  and a.status='submitted' and a.data->'attempt'->>'grading_status'='needs_manual'
			  and j.status='needs_manual'
			order by a.id limit 1`).Scan(&attemptID, &attemptRaw, &jobID, &jobRaw); err != nil {
			t.Fatalf("cannot select an imported submitted practicum attempt/job: %v", err)
		}
		var attempt entity.PracticumAttemptRecord
		var job entity.PracticumGradingJob
		livePhase6Decode(t, attemptRaw, &attempt)
		livePhase6Decode(t, jobRaw, &job)
		if !strings.HasPrefix(attemptID, "mig-pat-") || attempt.Attempt.AttemptID != attemptID || jobID != attemptID || job.ID != jobID || job.AttemptID != attemptID || !strings.HasPrefix(attempt.Attempt.ActivationID, "mig-pac-") || !strings.HasPrefix(attempt.Attempt.SessionID, "mig-psn-") || !strings.HasPrefix(attempt.Attempt.CourseID, "mig-pco-") {
			t.Fatal("practicum selection did not preserve the original imported attempt/job/activation IDs")
		}
		if attempt.Attempt.Status != "submitted" || attempt.Attempt.GradingStatus != "needs_manual" || len(attempt.SubmittedAnswers) == 0 || len(attempt.Attempt.Questions) == 0 {
			t.Fatal("selected imported practicum attempt is not the required manual-review state")
		}
		learner := livePhase6Principal(t, ctx, identity, attempt.Attempt.UserID, entity.RoleMahasiswa)
		review, err := practicum.Review(ctx, coordinator, attemptID, livePhase6PracticumCommand("review"))
		if err != nil || review.Attempt.AttemptID != attemptID || review.GradingJob == nil || review.GradingJob.ID != jobID {
			t.Fatalf("usecase cannot read the imported practicum attempt/job: %v", err)
		}
		grades := make([]entity.PracticumManualGrade, 0, len(attempt.Attempt.Questions))
		for _, question := range attempt.Attempt.Questions {
			if !strings.HasPrefix(question.ID, "mig-pqu-") || question.Points < 1 {
				t.Fatal("imported attempt contains an invalid original question ID or score")
			}
			grades = append(grades, entity.PracticumManualGrade{QuestionID: question.ID, Score: question.Points, Feedback: "Local masked-import acceptance review"})
		}
		input := usecase.PracticumManualInput{GradeRevision: attempt.Attempt.GradeRevision, Grades: grades, Reason: "Local masked-import acceptance"}
		draft, err := practicum.ManualGrade(ctx, coordinator, livePhase6PracticumCommand("grade-draft"), attemptID, input)
		if err != nil || draft.Attempt.AttemptID != attemptID || draft.Attempt.Status != "submitted" || draft.Attempt.Score != nil || draft.Attempt.GradeRevision != input.GradeRevision+1 {
			t.Fatalf("manual grade draft failed or published early: %v", err)
		}
		input.GradeRevision = draft.Attempt.GradeRevision
		input.Publish = true
		var published entity.PracticumAttemptReview
		for range 2 {
			published, err = practicum.ManualGrade(ctx, coordinator, livePhase6PracticumCommand("grade-publish"), attemptID, input)
			if err != nil {
				t.Fatalf("manual grade publication/replay failed: %v", err)
			}
			if published.Attempt.AttemptID != attemptID || published.Attempt.Status != "graded" || published.Attempt.Score == nil || *published.Attempt.Score != published.Attempt.MaxScore || published.GradingJob == nil || published.GradingJob.ID != jobID || published.GradingJob.Status != "graded" {
				t.Fatal("manual grade publication/replay did not retain the imported attempt and job")
			}
		}
		summary, err := livePhase6FindGrade(ctx, practicum, learner, attemptID)
		if err != nil || summary.AttemptID != attemptID || summary.Score == nil || *summary.Score != summary.MaxScore {
			t.Fatalf("published imported grade is not visible to its original learner: %v", err)
		}
		storedAttempt := livePhase6ReadData[entity.PracticumAttemptRecord](t, ctx, pool, "practicum_attempts", attemptID)
		storedJob := livePhase6ReadData[entity.PracticumGradingJob](t, ctx, pool, "practicum_grading_jobs", jobID)
		if storedAttempt.Attempt.AttemptID != attemptID || storedJob.ID != jobID || storedJob.Status != "graded" || !reflect.DeepEqual(storedAttempt.SubmittedAnswers, attempt.SubmittedAnswers) {
			t.Fatal("manual grading changed frozen submitted answers or replaced imported attempt/job IDs")
		}
	})

	t.Run("imported_class_schedule_local_attendance_duplicate_and_close", func(t *testing.T) {
		var classRaw, scheduleRaw []byte
		var learnerID int64
		if err := pool.QueryRow(ctx, `
			select c.id,c.data,s.id,s.data,u.id
			from public.academic_classes c
			join public.academic_schedules s on s.parent_id=c.id
			cross join lateral jsonb_array_elements_text(c.data->'student_ids') member(user_id)
			join public.users u on u.id=member.user_id::bigint and u.is_active and not u.hidden
			join public.user_role_assignments a on a.user_id=u.id and a.role_code='mahasiswa' and a.is_active
			where c.id like 'mig-cls-%' and s.id like 'mig-sch-%'
			order by c.id,s.id,u.id limit 1`).Scan(&importedClassID, &classRaw, &importedScheduleID, &scheduleRaw, &learnerID); err != nil {
			t.Fatalf("cannot select an imported class/schedule with an active learner: %v", err)
		}
		var class entity.AcademicClass
		var schedule entity.AcademicSchedule
		livePhase6Decode(t, classRaw, &class)
		livePhase6Decode(t, scheduleRaw, &schedule)
		if class.ID != importedClassID || schedule.ID != importedScheduleID || schedule.ClassID != importedClassID || !livePhase6ContainsInt64(class.StudentIDs, learnerID) {
			t.Fatal("academic selection did not preserve original imported class/schedule/member IDs")
		}
		learner := livePhase6Principal(t, ctx, identity, learnerID, entity.RoleMahasiswa)
		classes, err := academic.Classes(ctx, learner, false, page)
		if err != nil || !livePhase6ContainsClass(classes, importedClassID) {
			t.Fatalf("learner cannot read the imported class: %v", err)
		}
		schedules, err := academic.Schedules(ctx, learner, false, page)
		if err != nil || !livePhase6ContainsSchedule(schedules, importedScheduleID) {
			t.Fatalf("learner cannot read the imported schedule: %v", err)
		}
		opened, err := academic.OpenAttendanceSession(ctx, coordinator, livePhase6AcademicCommand("attendance-open"), usecase.AcademicAttendanceOpenInput{ScheduleID: importedScheduleID, Title: "Local masked-import acceptance", DurationMinutes: 30})
		if err != nil || opened.ID == "" || opened.ID == importedScheduleID || opened.ScheduleID != importedScheduleID || opened.ClassID != importedClassID || opened.QRToken == "" {
			t.Fatalf("cannot open a local attendance session on the imported schedule: %v", err)
		}
		localAttendanceSessionID = opened.ID
		checkin := usecase.AcademicAttendanceCheckinInput{SessionID: opened.ID, QRToken: opened.QRToken}
		first, err := academic.CheckInAttendance(ctx, learner, livePhase6AcademicCommand("attendance-checkin"), checkin)
		if err != nil {
			t.Fatalf("local attendance check-in failed: %v", err)
		}
		duplicate, err := academic.CheckInAttendance(ctx, learner, livePhase6AcademicCommand("attendance-duplicate"), checkin)
		if err != nil || !reflect.DeepEqual(first, duplicate) || first.SessionID != opened.ID || first.ScheduleID != importedScheduleID || first.ClassID != importedClassID || !first.VerifiedByQR {
			t.Fatalf("duplicate attendance check-in did not return the original local record: %v", err)
		}
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_attendance_records where parent_id=$1`, opened.ID); got != 1 {
			t.Fatalf("duplicate check-in created %d local attendance records", got)
		}
		closed, err := academic.CloseAttendanceSession(ctx, coordinator, livePhase6AcademicCommand("attendance-close"), opened.ID)
		if err != nil || closed.IsOpen || closed.ID != opened.ID || closed.ScheduleID != importedScheduleID || closed.ClosedAt == nil {
			t.Fatalf("local attendance close failed: %v", err)
		}
		if string(livePhase6ReadDataRaw(t, ctx, pool, "academic_classes", importedClassID)) != string(classRaw) || string(livePhase6ReadDataRaw(t, ctx, pool, "academic_schedules", importedScheduleID)) != string(scheduleRaw) {
			t.Fatal("local attendance flow changed or replaced imported class/schedule definitions")
		}
	})

	t.Run("imported_inventory_local_rental_lifecycle_replays_without_stock_duplication", func(t *testing.T) {
		var itemRaw []byte
		if err := pool.QueryRow(ctx, `
			select id,data from public.academic_inventory_items
			where id like 'mig-itm-%' and status='available'
			  and (data->>'available_quantity')::int>0
			order by id limit 1`).Scan(&importedItemID, &itemRaw); err != nil {
			t.Fatalf("no available imported inventory item can be used for the lifecycle: %v", err)
		}
		var item entity.InventoryItem
		livePhase6Decode(t, itemRaw, &item)
		if item.ID != importedItemID || !strings.HasPrefix(item.ID, "mig-itm-") || item.AvailableQuantity < 1 || item.TotalQuantity < item.AvailableQuantity || item.RentalFee < 0 {
			t.Fatal("selected inventory row is not a usable original imported item")
		}
		visible := false
		for offset := 0; offset < baselineReport.TotalInventoryItems+100; offset += 100 {
			items, err := academic.InventoryItems(ctx, student, usecase.PracticumPage{Limit: 100, Offset: offset})
			if err != nil {
				t.Fatalf("learner cannot page imported inventory: %v", err)
			}
			if livePhase6ContainsItem(items, importedItemID) {
				visible = true
				break
			}
			if len(items) < 100 {
				break
			}
		}
		if !visible {
			t.Fatal("learner cannot read imported inventory definition across all pages")
		}
		create := usecase.InventoryRentalCreateInput{ItemID: importedItemID, Quantity: 1, Purpose: "Local masked-import acceptance", DueAt: time.Now().UTC().Add(24 * time.Hour)}
		requested, err := academic.RequestRental(ctx, student, livePhase6AcademicCommand("rental-request"), create)
		if err != nil {
			t.Fatalf("local rental request failed: %v", err)
		}
		replayedRequest, err := academic.RequestRental(ctx, student, livePhase6AcademicCommand("rental-request"), create)
		if err != nil || !reflect.DeepEqual(requested, replayedRequest) || requested.ItemID != importedItemID || requested.BorrowerID != student.User.ID || requested.Status != "requested" {
			t.Fatalf("local rental request replay changed the request: %v", err)
		}
		localRentalID = requested.ID
		localRentalFee = requested.TotalFee
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_inventory_rentals`); got != baselineRentalCount+1 {
			t.Fatalf("rental request replay changed rental count incorrectly: baseline=%d observed=%d", baselineRentalCount, got)
		}
		if got := livePhase6ReadData[entity.InventoryItem](t, ctx, pool, "academic_inventory_items", importedItemID); got.AvailableQuantity != item.AvailableQuantity {
			t.Fatal("request/replay reserved stock before approval")
		}

		version := requested.Version
		// A zero-fee item is payment-verified at request time, and a repeat
		// verify_payment on an already verified rental is a domain no-op.
		paid := requested.PaymentVerified
		for _, step := range []struct {
			action     string
			wantStatus string
			wantStock  int
			wantPaid   bool
		}{
			{action: "approve", wantStatus: "approved", wantStock: item.AvailableQuantity - 1, wantPaid: paid},
			{action: "verify_payment", wantStatus: "approved", wantStock: item.AvailableQuantity - 1, wantPaid: true},
			{action: "return", wantStatus: "returned", wantStock: item.AvailableQuantity, wantPaid: true},
		} {
			input := usecase.InventoryRentalDecideInput{Version: version, Action: step.action}
			wantVersion := version + 1
			if step.action == "verify_payment" && paid {
				wantVersion = version
			}
			var decided entity.InventoryRental
			for range 2 {
				decided, err = academic.DecideRental(ctx, coordinator, livePhase6AcademicCommand("rental-"+step.action), requested.ID, input)
				if err != nil {
					t.Fatalf("rental %s/replay failed: %v", step.action, err)
				}
				if decided.ID != requested.ID || decided.ItemID != importedItemID || decided.Status != step.wantStatus || decided.Version != wantVersion || decided.PaymentVerified != step.wantPaid {
					t.Fatalf("rental %s/replay returned an unexpected lifecycle state", step.action)
				}
				storedItem := livePhase6ReadData[entity.InventoryItem](t, ctx, pool, "academic_inventory_items", importedItemID)
				if storedItem.ID != importedItemID || storedItem.TotalQuantity != item.TotalQuantity || storedItem.AvailableQuantity != step.wantStock {
					t.Fatalf("rental %s/replay duplicated stock movement", step.action)
				}
			}
			version = wantVersion
			paid = step.wantPaid
		}
		storedRental := livePhase6ReadData[entity.InventoryRental](t, ctx, pool, "academic_inventory_rentals", requested.ID)
		storedItem := livePhase6ReadData[entity.InventoryItem](t, ctx, pool, "academic_inventory_items", importedItemID)
		if storedRental.ID != requested.ID || storedRental.ItemID != importedItemID || storedRental.Status != "returned" || storedRental.ReturnedAt == nil || !storedRental.PaymentVerified || storedItem.ID != importedItemID || storedItem.AvailableQuantity != item.AvailableQuantity || storedItem.TotalQuantity != item.TotalQuantity || storedItem.Version != item.Version+2 {
			t.Fatal("rental final state did not preserve the imported item or restore stock exactly once")
		}
	})

	t.Run("scoped_report_reads_imported_definitions_and_new_local_flows", func(t *testing.T) {
		report, err := academic.Reports(ctx, coordinator)
		if err != nil {
			t.Fatalf("cannot read the post-flow scoped academic report: %v", err)
		}
		if report.ClassesCount < contract.importedClasses || report.SchedulesCount < contract.importedSchedules || report.TotalInventoryItems < contract.importedItems {
			t.Fatalf("scoped report omitted imported definitions: %+v", report)
		}
		if report.ClassesCount != baselineReport.ClassesCount || report.SchedulesCount != baselineReport.SchedulesCount || report.TotalInventoryItems != baselineReport.TotalInventoryItems || report.AvailableInventoryUnit != baselineReport.AvailableInventoryUnit {
			t.Fatal("local flows replaced definitions or left imported inventory stock changed")
		}
		if report.OpenAttendanceSessions != baselineReport.OpenAttendanceSessions || report.AttendanceByStatus["hadir"] != baselineReport.AttendanceByStatus["hadir"]+1 || report.ActiveRentalsCount != baselineReport.ActiveRentalsCount || report.TotalVerifiedRevenue != baselineReport.TotalVerifiedRevenue+localRentalFee {
			t.Fatalf("scoped report did not include exactly the new closed attendance and returned paid rental flows: %+v", report)
		}
		if !strings.Contains(report.CSVExport, fmt.Sprintf("total_verified_revenue,%d\n", report.TotalVerifiedRevenue)) {
			t.Fatal("scoped CSV report disagrees with the verified rental revenue")
		}
		if importedClassID == "" || importedScheduleID == "" || importedItemID == "" || localAttendanceSessionID == "" || localRentalID == "" {
			t.Fatal("required imported definitions or local flow IDs were not retained")
		}
		if got := livePhase6ReadData[entity.AcademicClass](t, ctx, pool, "academic_classes", importedClassID); got.ID != importedClassID {
			t.Fatal("imported class definition disappeared after local flows")
		}
		if got := livePhase6ReadData[entity.AcademicSchedule](t, ctx, pool, "academic_schedules", importedScheduleID); got.ID != importedScheduleID || got.ClassID != importedClassID {
			t.Fatal("imported schedule definition disappeared after local flows")
		}
		if got := livePhase6ReadData[entity.AcademicAttendanceSessionRecord](t, ctx, pool, "academic_attendance_sessions", localAttendanceSessionID); got.Session.ID != localAttendanceSessionID || got.Session.ScheduleID != importedScheduleID || got.Session.IsOpen {
			t.Fatal("scoped local attendance flow is not stored against the imported schedule")
		}
		if got := livePhase6ReadData[entity.InventoryRental](t, ctx, pool, "academic_inventory_rentals", localRentalID); got.ID != localRentalID || got.ItemID != importedItemID || got.Status != "returned" {
			t.Fatal("scoped local rental flow is not stored against the imported item")
		}
	})

	t.Run("existing_identities_remain_unchanged", func(t *testing.T) {
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users where email not like 'migration.%@example.test'`); got != baselineUnrelatedUsers {
			t.Fatalf("unrelated existing user count changed: before=%d after=%d", baselineUnrelatedUsers, got)
		}
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users`); got != baselineUserCount {
			t.Fatalf("public user count changed: before=%d after=%d", baselineUserCount, got)
		}
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.user_role_assignments`); got != baselineAssignmentCount {
			t.Fatalf("role assignment count changed: before=%d after=%d", baselineAssignmentCount, got)
		}
		if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from auth.users`); got != baselineAuthUserCount {
			t.Fatalf("auth user count changed: before=%d after=%d", baselineAuthUserCount, got)
		}
		if after := livePhase6UserSnapshot(t, ctx, pool); !reflect.DeepEqual(after, baselineUsers) {
			t.Fatal("an existing public user row changed during imported-data acceptance flows")
		}
	})
}

type livePhase6Contract struct {
	importedClasses   int
	importedSchedules int
	importedItems     int
}

func livePhase6PoolConfig(t *testing.T, databaseURL string) *pgxpool.Config {
	t.Helper()
	u, err := url.Parse(databaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() != "127.0.0.1" || u.Fragment != "" {
		t.Fatal("ALGOHUB_TEST_DATABASE_URL must use PostgreSQL at literal 127.0.0.1")
	}
	for key := range u.Query() {
		if key != "sslmode" {
			t.Fatal("the local acceptance database URL only permits the sslmode query parameter")
		}
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil || config.ConnConfig.Host != "127.0.0.1" || !strings.HasSuffix(config.ConnConfig.Database, "_test") || len(config.ConnConfig.Database) <= len("_test") {
		t.Fatal("ALGOHUB_TEST_DATABASE_URL must name a literal 127.0.0.1 database ending in _test")
	}
	config.ConnConfig.Fallbacks = nil
	config.ConnConfig.ConnectTimeout = 3 * time.Second
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	config.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	config.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "5000"
	config.ConnConfig.RuntimeParams["timezone"] = "UTC"
	// pgcrypto lives in `public` on a native disposable cluster and in
	// `extensions` on Supabase; the acceptance contract must work on both.
	config.ConnConfig.RuntimeParams["search_path"] = "public,extensions"
	config.MaxConns = 4
	config.MinConns = 0
	return config
}

func livePhase6VerifyRestoredContract(t *testing.T, ctx context.Context, pool *pgxpool.Pool) livePhase6Contract {
	t.Helper()
	migratedUsers := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.users where email like 'migration.%@example.test'`)
	if migratedUsers < 2 {
		t.Fatalf("restored database has only %d masked migrated users", migratedUsers)
	}
	if got := livePhase6CountQuery(t, ctx, pool, `
		select count(*) from public.user_role_assignments a join public.users u on u.id=a.user_id
		where u.email like 'migration.%@example.test' and a.role_code='mahasiswa' and a.is_active`); got < 1 {
		t.Fatal("restored database has no active imported mahasiswa")
	}
	if got := livePhase6CountQuery(t, ctx, pool, `
		select count(*) from public.user_role_assignments a join public.users u on u.id=a.user_id
		where u.email like 'migration.%@example.test' and a.role_code='koordinator' and a.is_active`); got < 1 {
		t.Fatal("restored database has no active imported koordinator")
	}

	progressCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.learning_progress where parent_id like 'mig-les-%'`)
	if progressCount < 1 {
		t.Fatal("restored database has no imported learning progress")
	}
	if got := livePhase6CountQuery(t, ctx, pool, `
		select count(*) from public.learning_progress
		where parent_id like 'mig-les-%' and
		 (status<>'completed' or data->>'status'<>'completed' or (data->>'xp_awarded')::int<>0 or data->'completed_at'='null'::jsonb)`); got != 0 {
		t.Fatalf("%d imported learning progress rows violate completed zero-award opening-balance policy", got)
	}
	if got := livePhase6CountQuery(t, ctx, pool, `
		select count(*) from public.learning_profiles p join public.users u on u.id=p.owner_id
		where u.email like 'migration.%@example.test' and
		 ((p.data->>'user_id')::bigint<>p.owner_id or
		  (p.data->>'completed_lessons')::int<>(select count(*) from public.learning_progress x where x.owner_id=p.owner_id and x.status='completed'))`); got != 0 {
		t.Fatalf("%d imported learning profiles disagree with the opening-balance progress set", got)
	}

	attemptCount := livePhase6CountQuery(t, ctx, pool, `select count(*) from public.practicum_attempts where id like 'mig-pat-%'`)
	if attemptCount < 1 {
		t.Fatal("restored database has no imported submitted practicum attempt")
	}
	if got := livePhase6CountQuery(t, ctx, pool, `
		select count(*) from public.practicum_attempts a
		left join public.practicum_grading_jobs j on j.id=a.id and j.parent_id=a.id
		where a.id like 'mig-pat-%' and
		 (a.status<>'submitted' or a.data->'attempt'->>'grading_status'<>'needs_manual' or j.id is null or j.status<>'needs_manual')`); got != 0 {
		t.Fatalf("%d imported practicum attempts/jobs are not in submitted needs_manual state", got)
	}

	contract := livePhase6Contract{
		importedClasses:   livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_classes where id like 'mig-cls-%'`),
		importedSchedules: livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_schedules where id like 'mig-sch-%'`),
		importedItems:     livePhase6CountQuery(t, ctx, pool, `select count(*) from public.academic_inventory_items where id like 'mig-itm-%'`),
	}
	if contract.importedClasses < 1 || contract.importedSchedules < 1 {
		t.Fatal("restored database is missing imported academic class/schedule definitions")
	}
	if contract.importedItems != 3 {
		t.Fatalf("restored database has %d imported inventory items; the approved plan requires 3", contract.importedItems)
	}
	// Retained Phase1-5 attendance/rentals are allowed on a restored staging
	// copy. There must be no fabricated *historical imported* rows; the test
	// checks per-flow deltas and preserves all unrelated identities/definitions.
	if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from academic_attendance_sessions where id like 'mig-%'`); got != 0 {
		t.Fatal("unexpected imported historical attendance session")
	}
	if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from academic_attendance_records where id like 'mig-%'`); got != 0 {
		t.Fatal("unexpected imported historical attendance record")
	}
	if got := livePhase6CountQuery(t, ctx, pool, `select count(*) from academic_inventory_rentals where id like 'mig-%'`); got != 0 {
		t.Fatal("unexpected imported historical rental")
	}
	return contract
}

func livePhase6Principal(t *testing.T, ctx context.Context, identity repository.IdentityRepository, userID int64, role entity.RoleCode) *entity.Principal {
	t.Helper()
	user, err := identity.FindUserByID(ctx, userID)
	if err != nil || !user.Active || user.Hidden || !strings.HasPrefix(user.Email, "migration.") || !strings.HasSuffix(user.Email, "@example.test") {
		t.Fatalf("cannot read active masked imported %s identity: %v", role, err)
	}
	assignments, err := identity.ListAssignments(ctx, userID)
	if err != nil {
		t.Fatalf("cannot read imported %s assignments: %v", role, err)
	}
	foundRole := false
	for _, assignment := range assignments {
		if assignment.UserID == userID && assignment.Role == role && assignment.Active {
			foundRole = true
		}
	}
	if !foundRole {
		t.Fatalf("imported identity %d does not retain active role %s", userID, role)
	}
	grants, err := identity.ListPermissionGrants(ctx, userID)
	if err != nil || len(grants) == 0 {
		t.Fatalf("cannot read imported %s permission grants: %v", role, err)
	}
	// Access snapshots must preserve the imported version and assignment set.
	// Explicit per-user grants are optional: this batch imports role/division
	// assignments only, and permissions derive from the canonical catalogs.
	access, err := identity.ReadAccess(ctx, userID)
	if err != nil || access.Version != user.AccessVersion || len(access.Assignments) == 0 {
		t.Fatalf("cannot read imported %s access snapshot: %v", role, err)
	}
	return &entity.Principal{User: user, Assignments: assignments, Grants: grants}
}

func livePhase6HasGrant(principal *entity.Principal, permission string) bool {
	for _, grant := range principal.Grants {
		if grant.Permission == permission {
			return true
		}
	}
	return false
}

func livePhase6CountQuery(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("acceptance count query failed: %v", err)
	}
	return count
}

func livePhase6UserSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) map[int64]string {
	t.Helper()
	rows, err := pool.Query(ctx, `select id,to_jsonb(u)::text from public.users u order by id`)
	if err != nil {
		t.Fatalf("cannot capture existing user snapshot: %v", err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var data string
		if err = rows.Scan(&id, &data); err != nil {
			t.Fatalf("cannot scan existing user snapshot: %v", err)
		}
		out[id] = data
	}
	if err = rows.Err(); err != nil {
		t.Fatalf("cannot finish existing user snapshot: %v", err)
	}
	return out
}

func livePhase6Decode(t *testing.T, raw []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("invalid imported typed payload: %v", err)
	}
}

func livePhase6ReadDataRaw(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, id string) []byte {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(ctx, "select data from "+pgx.Identifier{"public", table}.Sanitize()+" where id=$1", id).Scan(&raw); err != nil {
		t.Fatalf("cannot reread retained row %s: %v", id, err)
	}
	return raw
}

func livePhase6ReadData[T any](t *testing.T, ctx context.Context, pool *pgxpool.Pool, table, id string) T {
	t.Helper()
	var out T
	livePhase6Decode(t, livePhase6ReadDataRaw(t, ctx, pool, table, id), &out)
	return out
}

func livePhase6PracticumCommand(key string) usecase.PracticumCommand {
	return usecase.PracticumCommand{Key: "phase6-live-" + key, RequestID: "phase6-live-practicum", OperationID: "phase6-live-practicum"}
}

func livePhase6AcademicCommand(key string) usecase.AcademicCommand {
	return usecase.AcademicCommand{Key: "phase6-live-" + key, RequestID: "phase6-live-academic", OperationID: "phase6-live-academic"}
}

func livePhase6FindGrade(ctx context.Context, service *usecase.PracticumService, principal *entity.Principal, attemptID string) (entity.PracticumGradeSummary, error) {
	for offset := 0; offset <= 1000000; offset += 100 {
		rows, err := service.Grades(ctx, principal, usecase.PracticumPage{Limit: 100, Offset: offset})
		if err != nil {
			return entity.PracticumGradeSummary{}, err
		}
		for _, row := range rows {
			if row.AttemptID == attemptID {
				return row, nil
			}
		}
		if len(rows) < 100 {
			break
		}
	}
	return entity.PracticumGradeSummary{}, fmt.Errorf("published attempt %s not found", attemptID)
}

func livePhase6ContainsClass(rows []entity.AcademicClass, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

func livePhase6ContainsInt64(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func livePhase6ContainsSchedule(rows []entity.AcademicScheduleView, id string) bool {
	for _, row := range rows {
		if row.Schedule.ID == id {
			return true
		}
	}
	return false
}

func livePhase6ContainsItem(rows []entity.InventoryItem, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return true
		}
	}
	return false
}

// This provider is an in-process acceptance fake. It proves application HTTP
// login/session/SSO behavior against PostgreSQL, not real Supabase or a browser.
func livePhase6FakeProviderLoginAndSSO(t *testing.T, ctx context.Context, databaseURL, email, password string, principal *entity.Principal) {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		decoder := json.NewDecoder(io.LimitReader(r.Body, 16<<10))
		if r.URL.Path != "/auth/v1/token" || r.Method != http.MethodPost || decoder.Decode(&input) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.ToLower(strings.TrimSpace(input.Email)) != email || input.Password != password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]any{
				"id":    principal.User.AuthUserID,
				"email": principal.User.Email,
				"user_metadata": map[string]string{
					"name": principal.User.Name,
					"role": "koordinator",
				},
			},
		})
	}))
	defer provider.Close()

	origins := []string{"https://auth.phase6-live.example.test", "https://admin.phase6-live.example.test"}
	cfg := config.Config{
		Addr:              "127.0.0.1:8080",
		Environment:       "local",
		RequestTimeout:    10 * time.Second,
		AllowedOrigins:    origins,
		SSOAllowedOrigins: origins,
		CookieSecure:      true,
		SessionTTL:        time.Hour,
		SupabaseURL:       provider.URL,
		SupabaseAnonKey:   "local-fake-provider-not-a-secret",
		DatabaseURL:       databaseURL,
		DBMaxConns:        2,
		AuthRateLimit:     200,
		UserRateLimit:     1000,
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	first, err := application.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("cannot create first local acceptance application instance: %v", err)
	}
	defer first.Close()
	second, err := application.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("cannot create second local acceptance application instance: %v", err)
	}
	defer second.Close()

	call := func(handler http.Handler, method, path string, body []byte, origin string, cookies []*http.Cookie, wantStatus int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(string(body))).WithContext(ctx)
		r.Host = "api.phase6-live.example.test"
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			r.AddCookie(cookie)
			if cookie.Name == "algohub_csrf" {
				r.Header.Set("X-CSRF-Token", cookie.Value)
			}
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != wantStatus {
			t.Fatalf("local acceptance %s %s: expected status %d, got %d", method, path, wantStatus, w.Code)
		}
		return w
	}
	assertMe := func(handler http.Handler, origin string, cookies []*http.Cookie) {
		t.Helper()
		response := call(handler, http.MethodGet, "/v1/auth/me", nil, origin, cookies, http.StatusOK)
		var envelope struct {
			Data entity.Principal `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Data.User.ID != principal.User.ID || envelope.Data.User.AuthUserID != principal.User.AuthUserID || !livePhase6PrincipalHasRole(&envelope.Data, entity.RoleMahasiswa) {
			t.Fatalf("cross-instance local session did not retain imported mahasiswa identity/access: %v", err)
		}
	}

	loginBody, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		t.Fatal("cannot encode local acceptance login request")
	}
	central := call(first.Handler, http.MethodPost, "/v1/auth/password", loginBody, origins[0], nil, http.StatusOK).Result().Cookies()
	assertMe(second.Handler, origins[0], central)
	call(second.Handler, http.MethodGet, "/v1/admin/access", nil, origins[0], central, http.StatusForbidden)

	verifier := strings.Repeat("p", 43)
	sum := sha256.Sum256([]byte(verifier))
	prepareBody, err := json.Marshal(map[string]string{"return_to": origins[1] + "/", "code_challenge": base64.RawURLEncoding.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal("cannot encode local acceptance SSO preparation")
	}
	prepared := call(first.Handler, http.MethodPost, "/v1/auth/sso/prepare", prepareBody, origins[0], central, http.StatusOK)
	var handshake struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(prepared.Body.Bytes(), &handshake); err != nil || handshake.Data.Code == "" {
		t.Fatalf("local acceptance SSO preparation did not issue a code: %v", err)
	}
	exchangeBody, err := json.Marshal(map[string]string{"code": handshake.Data.Code, "return_to": origins[1] + "/", "code_verifier": verifier})
	if err != nil {
		t.Fatal("cannot encode local acceptance SSO exchange")
	}
	child := call(second.Handler, http.MethodPost, "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, http.StatusOK).Result().Cookies()
	assertMe(first.Handler, origins[1], child)
	call(first.Handler, http.MethodPost, "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, http.StatusUnauthorized)
	call(second.Handler, http.MethodPost, "/v1/auth/logout", nil, origins[1], child, http.StatusOK)
	call(first.Handler, http.MethodGet, "/v1/auth/me", nil, origins[0], central, http.StatusUnauthorized)
}

func livePhase6PrincipalHasRole(principal *entity.Principal, role entity.RoleCode) bool {
	for _, assignment := range principal.Assignments {
		if assignment.Active && assignment.Role == role {
			return true
		}
	}
	return false
}
