package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/storage"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

// laporanDeny is the authorization failure surfaced to a non-staff student.
var laporanDeny = errors.New("laporan: denied")

type laporanHarness struct {
	s        *LaporanService
	repo     *repository.MemoryAcademicRepository
	clock    *practicumTestClock
	admin    *entity.Principal
	student  *entity.Principal
	identity *practicumTestIdentity
	store    laporanFakeStore
}

func laporanCmd(key string) LaporanCommand {
	return LaporanCommand{Key: key, RequestID: "req-laporan", OperationID: "laporan.test"}
}

func laporanStaff(scope entity.Scope, permissions ...string) *entity.Principal {
	p := &entity.Principal{User: entity.UserProfile{ID: 9, Active: true, Name: "Koordinator"}, Assignments: []entity.RoleAssignment{{Role: entity.RoleKoordinator, Active: true, Scope: scope}}}
	for _, permission := range permissions {
		p.Grants = append(p.Grants, entity.PermissionGrant{Permission: permission, Role: entity.RoleKoordinator, Scope: scope})
	}
	return p
}

type laporanFakeStore struct{}

func (laporanFakeStore) Enabled() bool { return true }
func (laporanFakeStore) SignedUploadURL(context.Context, string, time.Duration) (string, error) {
	return "https://bucket.example.test/upload-signed", nil
}
func (laporanFakeStore) SignedDownloadURL(context.Context, string, time.Duration, string) (string, error) {
	return "https://bucket.example.test/download-signed", nil
}

func newLaporanHarness(t *testing.T, now time.Time) *laporanHarness {
	t.Helper()
	h := &laporanHarness{clock: &practicumTestClock{now: now}}
	h.repo = repository.NewMemoryAcademicRepository(h.clock.Now)
	h.identity = &practicumTestIdentity{students: map[int64]*entity.Principal{}}
	h.repo.BindIdentity(repository.NewMemoryIdentityRepository())
	institution := "lab"
	scope := entity.Scope{Domain: "siakad", Institution: institution, ClassID: "class-a", Shift: 0}
	h.admin = laporanStaff(scope, laporanManage)
	h.student = &entity.Principal{
		User:        entity.UserProfile{ID: 1, Active: true, Name: "Synthetic 1"},
		Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true, Scope: entity.Scope{Domain: "siakad", Institution: institution, ClassID: "class-a", Shift: 1}}},
	}
	h.identity.students[1] = h.student
	h.store = laporanFakeStore{}
	h.s = NewLaporanService(LaporanOptions{Repository: h.repo, IdentityRepository: h.identity, Storage: h.store})
	return h
}

func laporanWindow(buka, deadline, lateUntil time.Time) LaporanCreateInput {
	b := buka
	l := lateUntil
	return LaporanCreateInput{
		Judul: "Laporan Besar", Deskripsi: "desc", ButuhLaporan: true, ButuhAnalisa: true, Active: true,
		Targets: []LaporanTargetInput{{ClassID: "class-a", Shift: 0, BukaAt: &b, DeadlineAt: deadline, LateUntil: &l}},
	}
}

func TestLaporanFaseWindow(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	created, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	check := func(fase string) {
		t.Helper()
		status, err := h.s.StatusSaya(t.Context(), h.student)
		if err != nil {
			t.Fatal(err)
		}
		if status.Fase != fase {
			t.Fatalf("expected fase %s, got %s", fase, status.Fase)
		}
	}
	check(entity.LaporanDibuka)
	h.clock.Set(base.Add(90 * time.Minute))
	check(entity.LaporanTelat)
	status, err := h.s.StatusSaya(t.Context(), h.student)
	if err != nil {
		t.Fatal(err)
	}
	if !status.BisaKumpul || status.SisaDetik <= 0 {
		t.Fatalf("late window should allow submission: %+v", status)
	}
	h.clock.Set(base.Add(3 * time.Hour))
	check(entity.LaporanDitutup)
	if _, err = h.s.UploadURL(t.Context(), h.student, laporanCmd("up"), LaporanUploadURLRequest{Jenis: "laporan", Ext: "pdf"}); err == nil {
		t.Fatal("closed window must reject upload URL")
	}
	_ = created
}

func TestLaporanStudentWithoutClass(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	if _, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	orphan := &entity.Principal{User: entity.UserProfile{ID: 7, Active: true, Name: "No Class"}, Assignments: []entity.RoleAssignment{{Role: entity.RoleMahasiswa, Active: true, Scope: entity.Scope{Domain: "siakad", Institution: "lab"}}}}
	h.identity.students[7] = orphan
	if _, err := h.s.StatusSaya(t.Context(), orphan); err == nil {
		t.Fatal("student without class must fail")
	}
}

func TestLaporanShiftSpecificTargetWins(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	in := laporanWindow(base.Add(-time.Hour), base.Add(10*time.Hour), base.Add(20*time.Hour))
	in.ButuhAnalisa = false
	late := base.Add(time.Hour)
	in.Targets = append(in.Targets, LaporanTargetInput{ClassID: "class-a", Shift: 1, BukaAt: &late, DeadlineAt: base.Add(2 * time.Hour), LateUntil: nil})
	created, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), in)
	if err != nil {
		t.Fatal(err)
	}
	status, err := h.s.StatusSaya(t.Context(), h.student)
	if err != nil {
		t.Fatal(err)
	}
	if status.DeadlineAt == nil || !status.DeadlineAt.Equal(created.Targets[1].DeadlineAt) {
		t.Fatalf("shift-specific target must win: %+v", status.DeadlineAt)
	}
}

func TestLaporanKumpulAndIdempotency(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	if _, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	in := LaporanKumpulInput{LaporanPath: "laporan-id/1/laporan-1.pdf", AnalisaPath: "laporan-id/1/analisa-1.docx"}
	first, err := h.s.Kumpul(t.Context(), h.student, laporanCmd("kumpul"), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "terkumpul" || first.IsLate || first.LaporanPath != in.LaporanPath {
		t.Fatalf("unexpected submission: %+v", first)
	}
	again, err := h.s.Kumpul(t.Context(), h.student, laporanCmd("kumpul"), in)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("idempotent submission must reuse row: %s vs %s", first.ID, again.ID)
	}
	// is_late in the late phase.
	h.clock.Set(base.Add(90 * time.Minute))
	late, err := h.s.Kumpul(t.Context(), h.student, laporanCmd("kumpul-late"), in)
	if err != nil {
		t.Fatal(err)
	}
	if !late.IsLate {
		t.Fatal("submission in late phase must be flagged late")
	}
	// butuh_laporan enforced.
	h2 := newLaporanHarness(t, base)
	created, err := h2.s.Create(t.Context(), h2.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	created.ButuhLaporan = true
	if _, err = h2.s.Kumpul(t.Context(), h2.student, laporanCmd("kumpul"), LaporanKumpulInput{AnalisaPath: "a.docx"}); err == nil {
		t.Fatal("missing laporan must be rejected when butuh_laporan")
	}
}

func TestLaporanBatalkanAfterManual(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	if _, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	submitted, err := h.s.Kumpul(t.Context(), h.student, laporanCmd("kumpul"), LaporanKumpulInput{LaporanPath: "p.pdf", AnalisaPath: "a.docx"})
	if err != nil {
		t.Fatal(err)
	}
	marked, err := h.s.Mark(t.Context(), h.admin, laporanCmd("mark"), submitted.LaporanID, "1", LaporanMarkInput{Version: submitted.Version, Status: "terkumpul", Catatan: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if marked.Sumber != "manual" || marked.ManualOleh == nil {
		t.Fatalf("manual mark trail missing: %+v", marked)
	}
	if _, err := h.s.BatalkanKumpul(t.Context(), h.student, laporanCmd("batalkan")); err == nil {
		t.Fatal("cancel after manual mark must be rejected")
	}
}

func TestLaporanUploadURLValidation(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	if _, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.UploadURL(t.Context(), h.student, laporanCmd("up"), LaporanUploadURLRequest{Jenis: "laporan", Ext: "docx"}); err == nil {
		t.Fatal("laporan must require pdf")
	}
	if _, err := h.s.UploadURL(t.Context(), h.student, laporanCmd("up"), LaporanUploadURLRequest{Jenis: "analisa", Ext: "pdf"}); err == nil {
		t.Fatal("analisa must require docx")
	}
	res, err := h.s.UploadURL(t.Context(), h.student, laporanCmd("up"), LaporanUploadURLRequest{Jenis: "analisa", Ext: "docx"})
	if err != nil {
		t.Fatal(err)
	}
	if res.UploadURL == "" || res.Path == "" {
		t.Fatalf("signed upload target missing: %+v", res)
	}
	h2 := newLaporanHarness(t, base)
	h2.s.options.Storage = storage.Unconfigured{}
	if _, err := h2.s.Create(t.Context(), h2.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := h2.s.UploadURL(t.Context(), h2.student, laporanCmd("up"), LaporanUploadURLRequest{Jenis: "laporan", Ext: "pdf"}); err == nil {
		t.Fatal("unconfigured storage must fail upload URL")
	}
}

func TestLaporanStudentCannotUseAdminRoutes(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	if _, err := h.s.List(t.Context(), h.student); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("student cannot list admin laporan, got %v", err)
	}
}

func TestLaporanSignedBerkasOwnOnly(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	created, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.s.Kumpul(t.Context(), h.student, laporanCmd("kumpul"), LaporanKumpulInput{LaporanPath: created.ID + "/1/laporan-1.pdf", AnalisaPath: created.ID + "/1/analisa-1.docx"}); err != nil {
		t.Fatal(err)
	}
	if _, err = h.s.SignedBerkas(t.Context(), h.student, created.ID, "2", "laporan", false); err == nil {
		t.Fatal("student cannot read another student's berkas")
	}
	own, err := h.s.SignedBerkas(t.Context(), h.student, created.ID, "1", "laporan", true)
	if err != nil {
		t.Fatal(err)
	}
	if own.Filename != "1-laporan.pdf" {
		t.Fatalf("unexpected download name: %+v", own)
	}
	if _, err = h.s.SignedBerkas(t.Context(), h.student, created.ID, "1", "analisa", false); err != nil {
		t.Fatal(err)
	}
	if _, err = h.s.SignedBerkas(t.Context(), h.student, created.ID, "1", "bogus", false); err == nil {
		t.Fatal("unknown jenis must be rejected")
	}
}

func TestLaporanOverrideRequiresReason(t *testing.T) {
	base := time.Date(2026, 6, 1, 2, 0, 0, 0, time.UTC)
	h := newLaporanHarness(t, base)
	created, err := h.s.Create(t.Context(), h.admin, laporanCmd("create"), laporanWindow(base.Add(-time.Hour), base.Add(time.Hour), base.Add(2*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.s.SetOverride(t.Context(), h.admin, laporanCmd("ov"), created.ID, "1", LaporanOverrideInput{Version: 1, Alasan: "  "}); err == nil {
		t.Fatal("empty reason must be rejected")
	}
	row, err := h.s.SetOverride(t.Context(), h.admin, laporanCmd("ov"), created.ID, "1", LaporanOverrideInput{Version: 1, Alasan: "gerbang dibuka manual"})
	if err != nil {
		t.Fatal(err)
	}
	if row.OverrideAlasan != "gerbang dibuka manual" || row.OverridePada == nil {
		t.Fatalf("override trail missing: %+v", row)
	}
	revoked, err := h.s.RevokeOverride(t.Context(), h.admin, laporanCmd("rev"), created.ID, "1", LaporanOverrideInput{Version: row.Version})
	if err != nil {
		t.Fatal(err)
	}
	if revoked.OverridePada != nil {
		t.Fatal("override must be cleared")
	}
}
