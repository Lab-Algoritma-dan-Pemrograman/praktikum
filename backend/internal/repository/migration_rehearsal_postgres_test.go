package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/application"
	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/config"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/migrationrehearsal"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrationRehearsalImportedFlows consumes the harness's restored synthetic
// database, not newly created repository fixtures. It deliberately never invokes
// OpenTestDatabase, migrations, fixture seeders, or cleanup SQL. The harness owns
// this disposable database and must restore it again before another acceptance run.
func TestMigrationRehearsalImportedFlows(t *testing.T) {
	if os.Getenv("ALGOHUB_PHASE6_IMPORTED_FLOWS") != "true" {
		t.Skip("requires ALGOHUB_PHASE6_IMPORTED_FLOWS=true and a restored synthetic acceptance database")
	}
	// This generated, non-secret descriptor binds acceptance to the exact cluster
	// and database owned by the harness, not merely another loopback _test DB.
	targetPath := os.Getenv("ALGOHUB_PHASE6_TARGET_FILE")
	if targetPath == "" {
		t.Fatal("ALGOHUB_PHASE6_TARGET_FILE must name the harness-generated acceptance target")
	}
	targetFile, err := os.Open(targetPath)
	if err != nil {
		t.Fatal("could not open harness acceptance target descriptor")
	}
	defer targetFile.Close()
	info, err := targetFile.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		t.Fatal("acceptance target descriptor must be a regular file of at most 16 KiB")
	}
	targetData, err := io.ReadAll(io.LimitReader(targetFile, (16<<10)+1))
	if err != nil || len(targetData) > 16<<10 {
		t.Fatal("could not read bounded acceptance target descriptor")
	}
	var target migrationrehearsal.LocalTarget
	decoder := json.NewDecoder(strings.NewReader(string(targetData)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&target); err != nil {
		t.Fatal("invalid acceptance target descriptor")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("acceptance target descriptor must contain exactly one JSON value")
	}

	databaseURL := os.Getenv("ALGOHUB_TEST_DATABASE_URL")
	u, err := url.Parse(databaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("ALGOHUB_TEST_DATABASE_URL must explicitly identify a local PostgreSQL test database")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || !strings.HasSuffix(u.Path, "_test") || len(strings.TrimPrefix(u.Path, "/")) <= len("_test") {
		t.Fatal("acceptance database requires a literal loopback IP and a database name ending in _test")
	}
	// Do not let query parameters override the already-validated endpoint/name.
	for key := range u.Query() {
		if key != "sslmode" {
			t.Fatal("acceptance database URL only permits the sslmode query parameter")
		}
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("invalid acceptance database configuration")
	}
	parsedIP := net.ParseIP(poolConfig.ConnConfig.Host)
	if parsedIP == nil || !parsedIP.IsLoopback() || !strings.HasSuffix(poolConfig.ConnConfig.Database, "_test") {
		t.Fatal("resolved acceptance database configuration is not local/test-only")
	}
	poolConfig.ConnConfig.Fallbacks = nil
	poolConfig.ConnConfig.ConnectTimeout = 3 * time.Second
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "5000"
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolConfig.MaxConns, poolConfig.MinConns = 3, 0
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = migrationrehearsal.WithLocalTarget(ctx, target)
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal("could not initialize acceptance database pool")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal("could not connect to restored acceptance database")
	}
	// Inspect is read-only: verifies physical data directory, system identifier,
	// database ownership comment and pinned schema without applying migrations.
	if _, err := migrationrehearsal.Inspect(ctx, pool); err != nil {
		t.Fatalf("restored acceptance target failed ownership/schema verification: %v", err)
	}

	// A single existing row per domain table is the synthetic fixture contract.
	// Capture SQL IDs before any action and require every public read/mutation to
	// retain those IDs. An empty or newly seeded substitute database cannot pass.
	levelID, level := phase6ImportedRow[entity.LearningLevel](t, ctx, pool, "learning_levels")
	moduleID, module := phase6ImportedRow[entity.LearningModule](t, ctx, pool, "learning_modules")
	lessonID, lesson := phase6ImportedRow[entity.LearningLesson](t, ctx, pool, "learning_lessons")
	progressID, progress := phase6ImportedRow[entity.LearningProgress](t, ctx, pool, "learning_progress")
	profileID, profile := phase6ImportedRow[entity.LearningProfile](t, ctx, pool, "learning_profiles")
	sessionID, session := phase6ImportedRow[entity.PracticumSession](t, ctx, pool, "practicum_sessions")
	courseID, course := phase6ImportedRow[entity.PracticumCourse](t, ctx, pool, "practicum_courses")
	questionID, question := phase6ImportedRow[entity.PracticumQuestion](t, ctx, pool, "practicum_questions")
	activationID, activation := phase6ImportedRow[entity.PracticumActivationRecord](t, ctx, pool, "practicum_activations")
	attemptID, attempt := phase6ImportedRow[entity.PracticumAttemptRecord](t, ctx, pool, "practicum_attempts")
	jobID, job := phase6ImportedRow[entity.PracticumGradingJob](t, ctx, pool, "practicum_grading_jobs")
	classID, class := phase6ImportedRow[entity.AcademicClass](t, ctx, pool, "academic_classes")
	scheduleID, schedule := phase6ImportedRow[entity.AcademicSchedule](t, ctx, pool, "academic_schedules")
	attendanceSessionID, attendanceSession := phase6ImportedRow[entity.AcademicAttendanceSessionRecord](t, ctx, pool, "academic_attendance_sessions")
	attendanceRecordID, attendanceRecord := phase6ImportedRow[entity.AcademicAttendanceRecord](t, ctx, pool, "academic_attendance_records")
	itemID, item := phase6ImportedRow[entity.InventoryItem](t, ctx, pool, "academic_inventory_items")
	rentalID, rental := phase6ImportedRow[entity.InventoryRental](t, ctx, pool, "academic_inventory_rentals")
	phase6Count(t, ctx, pool, "users", 3)
	phase6Count(t, ctx, pool, "user_role_assignments", 2)

	// The initializer owns this unrelated baseline identity. Capture every SQL
	// column, not just its display name, so imported-user flows cannot modify it.
	sentinelSnapshot := func(t *testing.T) string {
		t.Helper()
		var snapshot string
		if err := pool.QueryRow(ctx, `select to_jsonb(u)::text from public.users u
			where u.auth_user_id='00000000-0000-4000-8000-000000000099'
			and u.email='phase6-baseline@example.test' and u.name='Unrelated baseline sentinel'
			and u.is_active and not u.hidden
			and u.created_at='2026-11-01T00:00:00Z' and u.updated_at='2026-11-01T00:00:00Z'
			and not exists(select 1 from public.user_role_assignments a where a.user_id=u.id)`).Scan(&snapshot); err != nil {
			t.Fatalf("unrelated baseline sentinel is missing or changed: %v", err)
		}
		return snapshot
	}
	baselineSentinel := sentinelSnapshot(t)

	identity := repository.NewPostgresIdentityRepository(pool)
	student := phase6ImportedPrincipal(t, ctx, pool, identity, migrationrehearsal.SyntheticStudentAuthID, entity.RoleMahasiswa)
	teacher := phase6ImportedPrincipal(t, ctx, pool, identity, migrationrehearsal.SyntheticTeacherAuthID, entity.RoleKoordinator)
	learning := usecase.NewLearningService(usecase.LearningOptions{Repository: repository.NewPostgresLearningRepository(pool), IdentityRepository: identity})
	practicum := usecase.NewPracticumService(usecase.PracticumOptions{Repository: repository.NewPostgresPracticumRepository(pool), IdentityRepository: identity})
	academic := usecase.NewAcademicService(usecase.AcademicOptions{Repository: repository.NewPostgresAcademicRepository(pool), IdentityRepository: identity})
	page := usecase.PracticumPage{Limit: 25}

	t.Run("imported_identity_and_permissions", func(t *testing.T) {
		policy := authorization.Service{}
		for _, check := range []struct {
			principal  *entity.Principal
			permission string
			scope      entity.Scope
		}{
			{student, "learning.content.read", level.Scope},
			{teacher, "practicum.grade.write", attempt.Scope},
			{teacher, "schedule.write", schedule.Scope},
			{teacher, "attendance.read", attendanceRecord.Scope},
			{teacher, "rental.approve", rental.Scope},
			{teacher, "finance.manage", rental.Scope},
		} {
			if err := policy.Check(check.principal, authorization.Requirement{Permission: check.permission, Scope: check.scope, ResourceExists: true}); err != nil {
				t.Fatalf("imported permission %s: %v", check.permission, err)
			}
		}
		if err := policy.Check(student, authorization.Requirement{Permission: "practicum.grade.write", Scope: attempt.Scope, ResourceExists: true}); !authorization.Is(err, "FORBIDDEN") {
			t.Fatalf("student unexpectedly has grading permission: %v", err)
		}
	})

	t.Run("fake_provider_http_login_and_sso", func(t *testing.T) {
		phase6FakeProviderHTTP(t, ctx, databaseURL, student, teacher)
		// Password login must find imported users, not create replacement identities.
		phase6Count(t, ctx, pool, "users", 3)
		phase6Count(t, ctx, pool, "user_role_assignments", 2)
		for _, principal := range []*entity.Principal{student, teacher} {
			got, err := identity.FindUserByAuthID(ctx, principal.User.AuthUserID)
			if err != nil || got.ID != principal.User.ID {
				t.Fatalf("login did not retain imported identity: %v", err)
			}
		}
	})

	t.Run("learning_imported_progress_complete_and_retry", func(t *testing.T) {
		if level.ID != levelID || module.ID != moduleID || module.LevelID != levelID || lesson.ID != lessonID || lesson.ModuleID != moduleID || lesson.LevelID != levelID {
			t.Fatal("imported learning IDs/relationships do not match SQL rows")
		}
		if lesson.Language != "c" || lesson.XPReward != 40 || lesson.Quiz != nil || len(lesson.TestCases) != 0 || lesson.StarterCode != "" || lesson.Solution != "" {
			t.Fatal("expected the imported text-only C lesson worth 40 XP")
		}
		if progressID != fmt.Sprintf("progress-%s-%d", lessonID, student.User.ID) || profileID != fmt.Sprintf("profile-%d", student.User.ID) || progress.UserID != student.User.ID || profile.UserID != student.User.ID || progress.LessonID != lessonID || progress.Revision != 1 || progress.Status != "in_progress" || progress.XPAwarded != 0 || profile.XP != 0 {
			t.Fatal("imported learning state or natural IDs do not match the acceptance contract")
		}
		before, err := learning.Lesson(ctx, student, lessonID)
		if err != nil {
			t.Fatal(err)
		}
		if before.Lesson.ID != lessonID || !reflect.DeepEqual(before.Progress, progress) || before.Profile.UserID != profile.UserID || before.Profile.XP != profile.XP || !before.Profile.UpdatedAt.Equal(profile.UpdatedAt) {
			t.Fatal("learning read did not return the existing imported progress/profile")
		}
		input := usecase.LearningProgressInput{Revision: progress.Revision, DraftCode: progress.DraftCode, Language: "c", Complete: true}
		command := usecase.LearningCommand{Key: "phase6-complete", RequestID: "phase6-learning", OperationID: "phase6-learning"}
		first, err := learning.SaveProgress(ctx, student, command, lessonID, input)
		if err != nil {
			t.Fatal(err)
		}
		retry, err := learning.SaveProgress(ctx, student, command, lessonID, input)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range []entity.LearningLessonView{first, retry} {
			if got.Lesson.ID != lessonID || got.Progress.Status != "completed" || got.Progress.Revision != 2 || got.Progress.XPAwarded != 40 || got.Profile.XP != 40 || got.Profile.CompletedLessons != 1 || got.Progress.CompletedAt == nil {
				t.Fatalf("completion/replay must award 40 XP exactly once: %+v", got.Progress)
			}
		}
		storedProgressID, storedProgress := phase6ImportedRow[entity.LearningProgress](t, ctx, pool, "learning_progress")
		storedProfileID, storedProfile := phase6ImportedRow[entity.LearningProfile](t, ctx, pool, "learning_profiles")
		if storedProgressID != progressID || storedProfileID != profileID || storedProgress.XPAwarded != 40 || storedProgress.Revision != 2 || storedProfile.XP != 40 || storedProfile.CompletedLessons != 1 {
			t.Fatal("completion duplicated or replaced imported learning state")
		}
	})

	t.Run("practicum_manual_grade_publish_and_replay", func(t *testing.T) {
		if session.ID != sessionID || course.ID != courseID || course.SessionID != sessionID || question.ID != questionID || question.CourseID != courseID || question.Type != "essay" || question.Points != 100 {
			t.Fatal("imported practicum definitions do not match SQL rows")
		}
		if activation.Activation.ID != activationID || activation.Activation.IsOpen || activation.Course.ID != courseID || len(activation.Questions) != 1 || activation.Questions[0].ID != questionID || activation.Questions[0].Points != 100 {
			t.Fatal("expected a closed imported activation with the frozen essay question")
		}
		if attempt.Attempt.AttemptID != attemptID || attempt.Attempt.ActivationID != activationID || attempt.Attempt.UserID != student.User.ID || attempt.Attempt.Status != "submitted" || attempt.Attempt.GradingStatus != "needs_manual" || attempt.Attempt.Score != nil || len(attempt.SubmittedAnswers) != 1 || jobID != attemptID || job.ID != jobID || job.AttemptID != attemptID {
			t.Fatal("expected imported submitted/ungraded attempt and its existing grading job")
		}
		review, err := practicum.Review(ctx, teacher, attemptID, phase6PracticumCommand("review"))
		if err != nil || review.Attempt.AttemptID != attemptID || review.GradingJob == nil || review.GradingJob.ID != jobID {
			t.Fatalf("cannot review imported attempt/job: %v", err)
		}
		input := usecase.PracticumManualInput{GradeRevision: attempt.Attempt.GradeRevision, Grades: []entity.PracticumManualGrade{{QuestionID: questionID, Score: 85, Feedback: "Synthetic manual review"}}, Reason: "Local synthetic migration acceptance"}
		draft, err := practicum.ManualGrade(ctx, teacher, phase6PracticumCommand("grade-draft"), attemptID, input)
		if err != nil || draft.Attempt.GradeRevision != input.GradeRevision+1 || draft.Attempt.Score != nil || draft.Attempt.Status != "submitted" {
			t.Fatalf("manual grade draft failed or published prematurely: %v", err)
		}
		grades, err := practicum.Grades(ctx, student, page)
		if err != nil || len(grades) != 1 || grades[0].AttemptID != attemptID || grades[0].Score != nil {
			t.Fatalf("unpublished grade leaked or imported attempt disappeared: %v", err)
		}
		input.GradeRevision, input.Publish = draft.Attempt.GradeRevision, true
		for range 2 { // The second call replays the same publication command/input.
			published, err := practicum.ManualGrade(ctx, teacher, phase6PracticumCommand("grade-publish"), attemptID, input)
			if err != nil {
				t.Fatal(err)
			}
			if published.Attempt.AttemptID != attemptID || published.Attempt.Status != "graded" || published.Attempt.GradeRevision != input.GradeRevision+1 || published.Attempt.Score == nil || *published.Attempt.Score != 85 || published.GradingJob == nil || published.GradingJob.ID != jobID || published.GradingJob.Status != "graded" {
				t.Fatal("publication/replay did not retain one final grade and imported job")
			}
		}
		grades, err = practicum.Grades(ctx, student, page)
		if err != nil || len(grades) != 1 || grades[0].AttemptID != attemptID || grades[0].Score == nil || *grades[0].Score != 85 || grades[0].MaxScore != 100 || grades[0].Percentage == nil || *grades[0].Percentage != 85 {
			t.Fatalf("student did not receive the published imported grade: %v", err)
		}
		storedID, stored := phase6ImportedRow[entity.PracticumAttemptRecord](t, ctx, pool, "practicum_attempts")
		storedJobID, storedJob := phase6ImportedRow[entity.PracticumGradingJob](t, ctx, pool, "practicum_grading_jobs")
		if storedID != attemptID || storedJobID != jobID || storedJob.GradeRevision != input.GradeRevision+1 || !reflect.DeepEqual(stored.SubmittedAnswers, attempt.SubmittedAnswers) {
			t.Fatal("manual grading replaced imported records or changed frozen submitted answers")
		}
	})

	t.Run("academic_historical_attendance_and_local_qr_duplicate", func(t *testing.T) {
		if class.ID != classID || len(class.StudentIDs) != 1 || class.StudentIDs[0] != student.User.ID || schedule.ID != scheduleID || schedule.ClassID != classID || schedule.DayOfWeek != "Senin" || schedule.Shift != 1 {
			t.Fatal("imported roster/schedule does not match the acceptance contract")
		}
		if attendanceSession.Session.ID != attendanceSessionID || attendanceSession.Session.IsOpen || attendanceSession.Session.ScheduleID != scheduleID || attendanceRecord.ID != attendanceRecordID || attendanceRecord.SessionID != attendanceSessionID || attendanceRecord.UserID != student.User.ID || attendanceRecord.Status != "hadir" {
			t.Fatal("expected historical closed session and imported hadir record")
		}
		classes, err := academic.Classes(ctx, student, false, page)
		if err != nil || len(classes) != 1 || classes[0].ID != classID {
			t.Fatalf("student cannot read imported class: %v", err)
		}
		schedules, err := academic.Schedules(ctx, student, false, page)
		if err != nil || len(schedules) != 1 || schedules[0].Schedule.ID != scheduleID || schedules[0].OpenSession != nil {
			t.Fatalf("student cannot read imported schedule/history: %v", err)
		}
		for _, principal := range []*entity.Principal{student, teacher} {
			records, err := academic.AttendanceRecords(ctx, principal, principal == teacher, attendanceSessionID, page)
			if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0], attendanceRecord) {
				t.Fatalf("historical imported attendance read failed: %v", err)
			}
		}
		opened, err := academic.OpenAttendanceSession(ctx, teacher, phase6AcademicCommand("attendance-open"), usecase.AcademicAttendanceOpenInput{ScheduleID: scheduleID, Title: "Local synthetic acceptance QR", DurationMinutes: 30})
		if err != nil || opened.ID == attendanceSessionID || opened.QRToken == "" {
			t.Fatalf("new local QR session on imported schedule failed: %v", err)
		}
		checkin := usecase.AcademicAttendanceCheckinInput{SessionID: opened.ID, QRToken: opened.QRToken}
		first, err := academic.CheckInAttendance(ctx, student, phase6AcademicCommand("attendance-checkin"), checkin)
		if err != nil {
			t.Fatal(err)
		}
		// A different command key exercises record-level duplicate protection too.
		duplicate, err := academic.CheckInAttendance(ctx, student, phase6AcademicCommand("attendance-duplicate"), checkin)
		if err != nil || !reflect.DeepEqual(first, duplicate) || first.UserID != student.User.ID || !first.VerifiedByQR || first.Status != "hadir" {
			t.Fatalf("duplicate QR check-in changed or duplicated attendance: %v", err)
		}
		records, err := academic.AttendanceRecords(ctx, teacher, true, opened.ID, page)
		if err != nil || len(records) != 1 || records[0].ID != first.ID {
			t.Fatalf("expected one new QR attendance record: %v", err)
		}
		if _, err := academic.CloseAttendanceSession(ctx, teacher, phase6AcademicCommand("attendance-close"), opened.ID); err != nil {
			t.Fatal(err)
		}
		historical, err := academic.AttendanceRecords(ctx, student, false, attendanceSessionID, page)
		if err != nil || len(historical) != 1 || !reflect.DeepEqual(historical[0], attendanceRecord) {
			t.Fatalf("new session changed imported attendance history: %v", err)
		}
		phase6Count(t, ctx, pool, "academic_attendance_sessions", 2)
		phase6Count(t, ctx, pool, "academic_attendance_records", 2)
	})

	t.Run("inventory_imported_rental_approve_pay_return_replays", func(t *testing.T) {
		if item.ID != itemID || item.TotalQuantity != 3 || item.AvailableQuantity != 3 || item.RentalFee != 1000 || rental.ID != rentalID || rental.ItemID != itemID || rental.BorrowerID != student.User.ID || rental.Status != "requested" || rental.Quantity != 1 || rental.TotalFee != 1000 || rental.PaymentVerified || rental.DueAt.UTC().Format("2006-01-02") != "2026-12-01" {
			t.Fatal("expected imported three-unit inventory and requested one-unit rental")
		}
		rentals, err := academic.Rentals(ctx, student, false, page)
		if err != nil || len(rentals) != 1 || !reflect.DeepEqual(rentals[0], rental) {
			t.Fatalf("student did not read imported rental: %v", err)
		}
		version := rental.Version
		for _, step := range []struct {
			action, status string
			stock          int
			paid           bool
		}{
			{"approve", "approved", 2, false},
			{"verify_payment", "approved", 2, true},
			{"return", "returned", 3, true},
		} {
			input := usecase.InventoryRentalDecideInput{Version: version, Action: step.action}
			for range 2 {
				got, err := academic.DecideRental(ctx, teacher, phase6AcademicCommand("rental-"+step.action), rentalID, input)
				if err != nil || got.ID != rentalID || got.Status != step.status || got.Version != version+1 || got.PaymentVerified != step.paid || got.TotalFee != 1000 {
					t.Fatalf("rental %s/replay failed: %v", step.action, err)
				}
				storedID, stored := phase6ImportedRow[entity.InventoryItem](t, ctx, pool, "academic_inventory_items")
				if storedID != itemID || stored.AvailableQuantity != step.stock || stored.TotalQuantity != 3 {
					t.Fatalf("rental %s/replay changed stock more than once", step.action)
				}
			}
			version++
		}
		storedID, stored := phase6ImportedRow[entity.InventoryRental](t, ctx, pool, "academic_inventory_rentals")
		if storedID != rentalID || stored.Status != "returned" || stored.ReturnedAt == nil || stored.DecidedBy == nil || *stored.DecidedBy != teacher.User.ID || !stored.PaymentVerified {
			t.Fatal("rental finalization did not persist on the imported rental")
		}
	})

	t.Run("academic_scoped_report_totals", func(t *testing.T) {
		report, err := academic.Reports(ctx, teacher)
		if err != nil {
			t.Fatal(err)
		}
		if report.ClassesCount != 1 || report.SchedulesCount != 1 || report.OpenAttendanceSessions != 0 || report.AttendanceByStatus["hadir"] != 2 || report.AttendanceByStatus["izin"] != 0 || report.AttendanceByStatus["alpha"] != 0 || report.AttendanceByStatus["terlambat"] != 0 || report.TotalInventoryItems != 1 || report.AvailableInventoryUnit != 3 || report.ActiveRentalsCount != 0 || report.TotalVerifiedRevenue != 1000 || !strings.Contains(report.CSVExport, "total_verified_revenue,1000\n") {
			t.Fatalf("unexpected scoped post-flow report: %+v", report)
		}
		studentReport, err := academic.Reports(ctx, student)
		if err != nil || studentReport.ClassesCount != 0 || studentReport.TotalInventoryItems != 0 || studentReport.TotalVerifiedRevenue != 0 {
			t.Fatalf("student received staff/finance report totals: %v", err)
		}
	})

	t.Run("unrelated_baseline_sentinel_unchanged", func(t *testing.T) {
		phase6Count(t, ctx, pool, "users", 3)
		phase6Count(t, ctx, pool, "user_role_assignments", 2)
		if sentinelSnapshot(t) != baselineSentinel {
			t.Fatal("acceptance flows changed the unrelated baseline user")
		}
	})
}

func phase6Count(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, want int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, "select count(*) from "+pgx.Identifier{"public", table}.Sanitize()).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("%s: expected %d existing rows, got %d; restore the synthetic acceptance copy before running", table, want, count)
	}
}

func phase6ImportedRow[T any](t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) (string, T) {
	t.Helper()
	phase6Count(t, ctx, pool, table, 1)
	var id string
	var data []byte
	var value T
	if err := pool.QueryRow(ctx, "select id, data from "+pgx.Identifier{"public", table}.Sanitize()).Scan(&id, &data); err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatalf("%s: empty imported ID", table)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s: invalid imported typed payload: %v", table, err)
	}
	return id, value
}

func phase6ImportedPrincipal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo repository.IdentityRepository, authID string, role entity.RoleCode) *entity.Principal {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, "select id from public.users where auth_user_id=$1", authID).Scan(&id); err != nil {
		t.Fatalf("missing imported %s identity: %v", role, err)
	}
	user, err := repo.FindUserByAuthID(ctx, authID)
	if err != nil || user.ID != id || !user.Active {
		t.Fatalf("imported %s identity read failed: %v", role, err)
	}
	assignments, err := repo.ListAssignments(ctx, id)
	if err != nil || len(assignments) != 1 || assignments[0].Role != role || !assignments[0].Active || assignments[0].UserID != id {
		t.Fatalf("imported %s assignment read failed: %v", role, err)
	}
	grants, err := repo.ListPermissionGrants(ctx, id)
	if err != nil || len(grants) == 0 {
		t.Fatalf("imported %s permission retrieval failed: %v", role, err)
	}
	return &entity.Principal{User: user, Assignments: assignments, Grants: grants}
}

func phase6PracticumCommand(key string) usecase.PracticumCommand {
	return usecase.PracticumCommand{Key: "phase6-" + key, RequestID: "phase6-practicum", OperationID: "phase6-practicum"}
}

func phase6AcademicCommand(key string) usecase.AcademicCommand {
	return usecase.AcademicCommand{Key: "phase6-" + key, RequestID: "phase6-academic", OperationID: "phase6-academic"}
}

// This is explicitly an in-process fake provider, NOT real Supabase or a browser
// E2E test. The application and session/permission repositories use PostgreSQL.
func phase6FakeProviderHTTP(t *testing.T, ctx context.Context, databaseURL string, student, teacher *entity.Principal) {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct{ Email, Password string }
		if r.URL.Path != "/auth/v1/token" || r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&input) != nil || input.Password != "synthetic-local-password" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var user *entity.UserProfile
		for _, principal := range []*entity.Principal{student, teacher} {
			if input.Email == principal.User.Email {
				copy := principal.User
				user = &copy
			}
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Hostile metadata must not override the student's imported database role.
		_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": user.AuthUserID, "email": user.Email, "user_metadata": map[string]string{"name": user.Name, "role": "koordinator"}}})
	}))
	defer provider.Close()
	origins := []string{"https://auth.phase6.example.test", "https://admin.phase6.example.test"}
	cfg := config.Config{Addr: "127.0.0.1:8080", Environment: "local", RequestTimeout: 10 * time.Second, AllowedOrigins: origins, SSOAllowedOrigins: origins, CookieSecure: true, SessionTTL: time.Hour, SupabaseURL: provider.URL, SupabaseAnonKey: "synthetic-not-a-secret", DatabaseURL: databaseURL, DBMaxConns: 2, AuthRateLimit: 200, UserRateLimit: 1000}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	first, err := application.New(ctx, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := application.New(ctx, cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	call := func(handler http.Handler, method, path, body, origin string, cookies []*http.Cookie, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		r.Host = "api.phase6.example.test"
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
		if w.Code != status {
			t.Fatalf("%s %s: expected %d, got %d: %s", method, path, status, w.Code, w.Body.String())
		}
		return w
	}
	assertMe := func(handler http.Handler, origin string, cookies []*http.Cookie, want *entity.Principal) {
		t.Helper()
		response := call(handler, "GET", "/v1/auth/me", "", origin, cookies, 200)
		var envelope struct {
			Data entity.Principal `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || envelope.Data.User.ID != want.User.ID || envelope.Data.User.AuthUserID != want.User.AuthUserID || len(envelope.Data.Assignments) != 1 || envelope.Data.Assignments[0].Role != want.Assignments[0].Role {
			t.Fatalf("HTTP session did not preserve the imported identity/role: %v", err)
		}
	}
	for _, principal := range []*entity.Principal{student, teacher} {
		body, err := json.Marshal(map[string]string{"email": principal.User.Email, "password": "synthetic-local-password"})
		if err != nil {
			t.Fatal(err)
		}
		central := call(first.Handler, "POST", "/v1/auth/password", string(body), origins[0], nil, 200).Result().Cookies()
		assertMe(second.Handler, origins[0], central, principal)
		accessStatus := http.StatusForbidden
		if principal == teacher {
			accessStatus = http.StatusOK
		}
		call(second.Handler, "GET", "/v1/admin/access", "", origins[0], central, accessStatus)
		verifier := strings.Repeat("p", 43)
		sum := sha256.Sum256([]byte(verifier))
		prepared := call(first.Handler, "POST", "/v1/auth/sso/prepare", fmt.Sprintf(`{"return_to":"%s/","code_challenge":"%s"}`, origins[1], base64.RawURLEncoding.EncodeToString(sum[:])), origins[0], central, 200)
		var handshake struct {
			Data struct {
				Code string `json:"code"`
			} `json:"data"`
		}
		if err := json.Unmarshal(prepared.Body.Bytes(), &handshake); err != nil || handshake.Data.Code == "" {
			t.Fatalf("SSO preparation did not issue a code: %v", err)
		}
		exchangeBody := fmt.Sprintf(`{"code":"%s","return_to":"%s/","code_verifier":"%s"}`, handshake.Data.Code, origins[1], verifier)
		child := call(second.Handler, "POST", "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, 200).Result().Cookies()
		assertMe(first.Handler, origins[1], child, principal)
		call(first.Handler, "POST", "/v1/auth/sso/exchange", exchangeBody, origins[1], nil, 401)
		call(second.Handler, "POST", "/v1/auth/logout", "", origins[1], child, 200)
		call(first.Handler, "GET", "/v1/auth/me", "", origins[0], central, 401)
	}
}
