package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/storage"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

var (
	ErrLaporanStorage = errors.New("laporan besar storage unavailable")
	ErrLaporanWindow  = errors.New("laporan besar submission window closed")
)

// TTL signed URLs: upload 30 minutes (large files), read 5 minutes.
const (
	laporanTTLUpload = 30 * time.Minute
	laporanTTLRead   = 5 * time.Minute
)

const laporanManage = "laporan.manage"

type LaporanOptions struct {
	Repository         repository.AcademicRepository
	IdentityRepository repository.IdentityRepository
	Storage            storage.SignedStore
}

type LaporanService struct{ options LaporanOptions }

func NewLaporanService(o LaporanOptions) *LaporanService {
	if r, ok := o.Repository.(*repository.MemoryAcademicRepository); ok {
		if identity, ok := o.IdentityRepository.(*repository.MemoryIdentityRepository); ok {
			r.BindIdentity(identity)
		}
	}
	return &LaporanService{options: o}
}

type LaporanCommand = AcademicCommand

type LaporanTargetInput struct {
	ClassID    string     `json:"class_id"`
	Shift      int        `json:"shift"`
	BukaAt     *time.Time `json:"buka_at"`
	DeadlineAt time.Time  `json:"deadline_at"`
	LateUntil  *time.Time `json:"late_until"`
}

type LaporanCreateInput struct {
	Judul        string               `json:"judul"`
	Deskripsi    string               `json:"deskripsi"`
	ButuhLaporan bool                 `json:"butuh_laporan"`
	ButuhAnalisa bool                 `json:"butuh_analisa"`
	Active       bool                 `json:"active"`
	Targets      []LaporanTargetInput `json:"targets"`
}

type LaporanUpdateInput struct {
	Version      int64                `json:"version"`
	Judul        string               `json:"judul"`
	Deskripsi    string               `json:"deskripsi"`
	ButuhLaporan bool                 `json:"butuh_laporan"`
	ButuhAnalisa bool                 `json:"butuh_analisa"`
	Active       bool                 `json:"active"`
	Targets      []LaporanTargetInput `json:"targets"`
}

type LaporanUploadURLRequest struct {
	Jenis string `json:"jenis"` // "laporan" (pdf) | "analisa" (docx)
	Ext   string `json:"ext"`
}

type LaporanKumpulInput struct {
	LaporanPath string `json:"laporan_path"`
	AnalisaPath string `json:"analisa_path"`
}

type LaporanMarkInput struct {
	Version int64  `json:"version"`
	Status  string `json:"status"` // "terkumpul" | "belum"
	Catatan string `json:"catatan"`
}

type LaporanOverrideInput struct {
	Version int64  `json:"version"`
	Alasan  string `json:"alasan"`
}

type laporanOperation struct {
	PayloadHash string `json:"payload_hash"`
	TargetID    string `json:"target_id"`
}

func laporanID(prefix string) string {
	v, err := security.RandomToken("")
	if err != nil {
		panic(err)
	}
	return prefix + "-" + v
}

func laporanHash(v any) string { data, _ := json.Marshal(v); return security.Hash(string(data)) }

func laporanRead[T any](tx repository.AcademicTx, k repository.AcademicKind, id string, lock bool) (T, error) {
	var out T
	var row repository.AcademicRecord
	var err error
	if lock {
		row, err = tx.Lock(k, id)
	} else {
		row, err = tx.Get(k, id)
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(row.Data, &out)
	return out, err
}

func laporanPut(tx repository.AcademicTx, k repository.AcademicKind, id, parent string, owner int64, scope entity.Scope, status, unique string, expires *time.Time, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return tx.Put(k, repository.AcademicRecord{ID: id, ParentID: parent, OwnerID: owner, Scope: scope, Status: status, UniqueKey: unique, ExpiresAt: expires, Data: raw})
}

func laporanAudit(tx repository.AcademicTx, p *entity.Principal, c LaporanCommand, action, id string, scope entity.Scope) error {
	now, err := tx.Now()
	if err != nil {
		return err
	}
	var actor *int64
	if p != nil {
		actor = &p.User.ID
	}
	if scope.Domain == "" {
		scope.Domain = "siakad"
	}
	return tx.Audit(entity.AuditEvent{ActorID: actor, Action: action, App: "siakad", SubjectType: "laporan_besar", SubjectID: id, Scope: scope, RequestID: c.RequestID, OperationID: c.OperationID, CreatedAt: now, Detail: map[string]any{}})
}

func laporanMutate[T any](s *LaporanService, ctx context.Context, p *entity.Principal, c LaporanCommand, action, resource string, input any, fn func(repository.AcademicTx, string) (T, string, entity.Scope, error)) (T, error) {
	var out T
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !practicumKeyPattern.MatchString(c.Key) {
		return out, ErrValidation
	}
	opID := laporanHash([]any{p.User.ID, action, resource, c.Key})
	hash := laporanHash(input)
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		replay := ""
		op, err := laporanRead[laporanOperation](tx, repository.AcademicOperations, opID, false)
		if err == nil {
			if op.PayloadHash != hash {
				return repository.ErrAcademicConflict
			}
			replay = op.TargetID
		} else if !errors.Is(err, repository.ErrAcademicNotFound) {
			return err
		}
		var target string
		var scope entity.Scope
		out, target, scope, err = fn(tx, replay)
		if err != nil {
			return err
		}
		if replay != "" {
			return nil
		}
		if scope.Domain == "" {
			scope.Domain = "siakad"
		}
		if err = laporanPut(tx, repository.AcademicOperations, opID, "", p.User.ID, scope, "", opID, nil, laporanOperation{PayloadHash: hash, TargetID: target}); err != nil {
			return err
		}
		return laporanAudit(tx, p, c, action, target, scope)
	})
	return out, err
}

// laporanActive finds the single active laporan besar. Legacy parity: FindAktif.
func laporanActive(tx repository.AcademicTx) (*entity.LaporanBesar, error) {
	rows, err := tx.List(repository.AcademicLaporanBesar, repository.AcademicFilter{Status: "active", Limit: 100})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, repository.ErrAcademicNotFound
	}
	// Deterministic: latest created wins when more than one is active.
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })
	out, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, rows[0].ID, false)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// laporanTarget picks the most specific target: exact shift match, then shift-0 wildcard.
func laporanTarget(l *entity.LaporanBesar, classID string, shift int) (*entity.LaporanBesarTarget, error) {
	var wildcard *entity.LaporanBesarTarget
	for i := range l.Targets {
		t := &l.Targets[i]
		if t.ClassID != classID {
			continue
		}
		if t.Shift == shift {
			return t, nil
		}
		if t.Shift == 0 && wildcard == nil {
			wildcard = t
		}
	}
	if wildcard == nil {
		return nil, repository.ErrAcademicNotFound
	}
	return wildcard, nil
}

// laporanFase computes the submission window phase server-side.
func laporanFase(t *entity.LaporanBesarTarget, now time.Time) string {
	if t.BukaAt != nil && now.Before(*t.BukaAt) {
		return entity.LaporanBelumDibuka
	}
	if now.Before(t.DeadlineAt) {
		return entity.LaporanDibuka
	}
	if t.LateUntil != nil && !now.After(*t.LateUntil) {
		return entity.LaporanTelat
	}
	return entity.LaporanDitutup
}

func laporanBatas(t *entity.LaporanBesarTarget, fase string) *time.Time {
	switch fase {
	case entity.LaporanBelumDibuka:
		return t.BukaAt
	case entity.LaporanDibuka:
		d := t.DeadlineAt
		return &d
	case entity.LaporanTelat:
		return t.LateUntil
	}
	return nil
}

func laporanSisaDetik(batas *time.Time, now time.Time) int64 {
	if batas == nil {
		return 0
	}
	return int64(batas.Sub(now).Seconds())
}

// laporanStudentScope returns the student's class scope, or an error when unset.
func laporanStudentScope(p *entity.Principal) (entity.Scope, error) {
	for _, a := range p.Assignments {
		if a.Active && a.Role == entity.RoleMahasiswa && a.Scope.ClassID != "" {
			return a.Scope, nil
		}
	}
	return entity.Scope{}, fmt.Errorf("%w: kelas mahasiswa belum diatur", ErrValidation)
}

// laporanRequire grants access to any staff holding laporan.manage. The report
// spans several classes, so permission is checked at grant level instead of
// against one resource scope.
func laporanRequire(p *entity.Principal) error {
	if err := practicumAuthenticated(p); err != nil {
		return err
	}
	for _, g := range p.Grants {
		if g.Permission == laporanManage && practicumAllowed(p, laporanManage, g.Scope) {
			return nil
		}
	}
	return authorization.ErrForbidden
}

// laporanScope mirrors the requester's institution on the stored record so the
// policy check has an exact scope to match against.
func laporanScope(p *entity.Principal) entity.Scope {
	scope := entity.Scope{Domain: "siakad"}
	for _, s := range laporanScopes(p) {
		if s.Institution != "" {
			scope.Institution = s.Institution
			break
		}
	}
	return scope
}

func laporanStore(s *LaporanService) storage.SignedStore {
	if s.options.Storage == nil {
		return storage.Unconfigured{}
	}
	return s.options.Storage
}

func validLaporanJudul(s string) bool {
	return len(s) <= 200 && !strings.ContainsAny(s, "\x00\r\n")
}

// laporanTargetInput validates per-class window ordering rules.
func validLaporanTargets(targets []LaporanTargetInput) bool {
	if len(targets) > 100 {
		return false
	}
	for _, t := range targets {
		if t.ClassID == "" || len(t.ClassID) > 128 || t.Shift < 0 || t.Shift > 2 {
			return false
		}
		if t.LateUntil != nil && t.LateUntil.Before(t.DeadlineAt) {
			return false
		}
		if t.BukaAt != nil && t.BukaAt.After(t.DeadlineAt) {
			return false
		}
	}
	return true
}

// laporanScopes returns the siakad class scopes the requester may manage.
func laporanScopes(p *entity.Principal) []entity.Scope {
	out := []entity.Scope{}
	if practicumAuthenticated(p) != nil {
		return out
	}
	for _, g := range p.Grants {
		if g.Permission != laporanManage {
			continue
		}
		s := g.Scope
		s.Division = ""
		out = append(out, s)
	}
	return out
}

// laporanScopeContains reports whether one class target falls inside the
// requester's manage scopes (shift wildcard 0 covers both shifts).
func laporanScopeContains(p *entity.Principal, classID string, shift int) bool {
	for _, s := range laporanScopes(p) {
		if s.ClassID == classID && (s.Shift == shift || s.Shift == 0) {
			return true
		}
	}
	return false
}

func (s *LaporanService) Create(ctx context.Context, p *entity.Principal, c LaporanCommand, in LaporanCreateInput) (entity.LaporanBesar, error) {
	var out entity.LaporanBesar
	if in.Judul == "" || !validLaporanJudul(in.Judul) || !validLaporanJudul(in.Deskripsi) || len(in.Deskripsi) > 4000 || !validLaporanTargets(in.Targets) {
		return out, ErrValidation
	}
	for _, t := range in.Targets {
		if !laporanScopeContains(p, t.ClassID, t.Shift) {
			return out, authorization.ErrForbidden
		}
	}
	return laporanMutate(s, ctx, p, c, "laporan.besar.create", "", in, func(tx repository.AcademicTx, replay string) (entity.LaporanBesar, string, entity.Scope, error) {
		if replay != "" {
			out, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, replay, false)
			if err == nil {
				err = laporanRequire(p)
			}
			return out, replay, out.Scope, err
		}
		if err := laporanRequire(p); err != nil {
			return out, "", entity.Scope{}, err
		}
		now, err := tx.Now()
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		id := laporanID("laporan")
		scope := laporanScope(p)
		targets := make([]entity.LaporanBesarTarget, 0, len(in.Targets))
		for _, t := range in.Targets {
			targets = append(targets, entity.LaporanBesarTarget{ClassID: t.ClassID, Shift: t.Shift, BukaAt: t.BukaAt, DeadlineAt: t.DeadlineAt, LateUntil: t.LateUntil})
		}
		out = entity.LaporanBesar{
			ID: id, Judul: strings.TrimSpace(in.Judul), Deskripsi: in.Deskripsi,
			ButuhLaporan: in.ButuhLaporan, ButuhAnalisa: in.ButuhAnalisa, Active: in.Active,
			Targets: targets, CreatedBy: p.User.ID, CreatedAt: now, UpdatedAt: &now, Scope: scope, Version: 1,
		}
		err = laporanPut(tx, repository.AcademicLaporanBesar, id, "", 0, scope, laporanStatusOf(out.Active), "", nil, out)
		return out, id, scope, err
	})
}

func laporanStatusOf(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func (s *LaporanService) Update(ctx context.Context, p *entity.Principal, c LaporanCommand, id string, in LaporanUpdateInput) (entity.LaporanBesar, error) {
	var out entity.LaporanBesar
	if in.Judul == "" || !validLaporanJudul(in.Judul) || !validLaporanJudul(in.Deskripsi) || len(in.Deskripsi) > 4000 || in.Version < 1 || !validLaporanTargets(in.Targets) {
		return out, ErrValidation
	}
	for _, t := range in.Targets {
		if !laporanScopeContains(p, t.ClassID, t.Shift) {
			return out, authorization.ErrForbidden
		}
	}
	return laporanMutate(s, ctx, p, c, "laporan.besar.update", id, in, func(tx repository.AcademicTx, replay string) (entity.LaporanBesar, string, entity.Scope, error) {
		out, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, id, true)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if err := laporanRequire(p); err != nil {
			return out, "", entity.Scope{}, err
		}
		if replay != "" {
			return out, replay, out.Scope, nil
		}
		if out.Version != in.Version {
			return out, "", entity.Scope{}, repository.ErrAcademicConflict
		}
		now, err := tx.Now()
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		out.Judul = strings.TrimSpace(in.Judul)
		out.Deskripsi = in.Deskripsi
		out.ButuhLaporan = in.ButuhLaporan
		out.ButuhAnalisa = in.ButuhAnalisa
		out.Active = in.Active
		targets := make([]entity.LaporanBesarTarget, 0, len(in.Targets))
		for _, t := range in.Targets {
			targets = append(targets, entity.LaporanBesarTarget{ClassID: t.ClassID, Shift: t.Shift, BukaAt: t.BukaAt, DeadlineAt: t.DeadlineAt, LateUntil: t.LateUntil})
		}
		out.Targets = targets
		out.Version++
		out.UpdatedAt = &now
		if err := laporanPut(tx, repository.AcademicLaporanBesar, id, "", 0, out.Scope, laporanStatusOf(out.Active), "", nil, out); err != nil {
			return out, "", entity.Scope{}, err
		}
		return out, id, out.Scope, nil
	})
}

// List returns every laporan besar visible to the requester's manage scopes.
func (s *LaporanService) List(ctx context.Context, p *entity.Principal) ([]entity.LaporanBesar, error) {
	out := []entity.LaporanBesar{}
	if err := laporanRequire(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicLaporanBesar, repository.AcademicFilter{Limit: 200})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var l entity.LaporanBesar
			if err = json.Unmarshal(r.Data, &l); err != nil {
				return err
			}
			if !laporanTargetsVisible(l, laporanScopes(p)) {
				continue
			}
			out = append(out, l)
		}
		return nil
	})
	return out, err
}

// laporanTargetsVisible keeps only classes inside the requester's scopes.
func laporanTargetsVisible(l entity.LaporanBesar, scopes []entity.Scope) bool {
	for _, t := range l.Targets {
		for _, s := range scopes {
			if s.ClassID == t.ClassID && (s.Shift == t.Shift || s.Shift == 0) {
				return true
			}
		}
	}
	return false
}

// StatusSaya returns the active laporan besar state for one student, computed
// from the server clock. Paths and override identity never reach the response.
func (s *LaporanService) StatusSaya(ctx context.Context, p *entity.Principal) (entity.LaporanStatus, error) {
	var out entity.LaporanStatus
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	scope, err := laporanStudentScope(p)
	if err != nil {
		return out, err
	}
	err = s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		l, err := laporanActive(tx)
		if err != nil {
			return err
		}
		target, err := laporanTarget(l, scope.ClassID, scope.Shift)
		if err != nil {
			return authorization.ErrNotFound
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		fase := laporanFase(target, now)
		out = entity.LaporanStatus{
			LaporanID: l.ID, Judul: l.Judul, Deskripsi: l.Deskripsi,
			ButuhLaporan: l.ButuhLaporan, ButuhAnalisa: l.ButuhAnalisa,
			ClassID: scope.ClassID, Shift: scope.Shift,
			BukaAt: target.BukaAt, DeadlineAt: &target.DeadlineAt, LateUntil: target.LateUntil,
			Fase: fase, SisaDetik: laporanSisaDetik(laporanBatas(target, fase), now),
			BisaKumpul: fase == entity.LaporanDibuka || fase == entity.LaporanTelat,
			Status:     "belum",
		}
		rows, err := tx.List(repository.AcademicLaporanPengumpulan, repository.AcademicFilter{ParentID: l.ID, OwnerID: p.User.ID, Limit: 1})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		var rec entity.LaporanPengumpulan
		if err = json.Unmarshal(rows[0].Data, &rec); err != nil {
			return err
		}
		out.Status = rec.Status
		out.Sumber = rec.Sumber
		out.Terlambat = rec.IsLate
		out.SubmittedAt = rec.SubmittedAt
		out.HasLaporan = rec.LaporanPath != ""
		out.HasAnalisa = rec.AnalisaPath != ""
		return nil
	})
	return out, err
}

func laporanUploadExt(jenis, ext string) (string, bool) {
	ext = strings.ToLower(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ext)), "."))
	if jenis == "laporan" {
		return ext, ext == "pdf"
	}
	if jenis == "analisa" {
		return ext, ext == "docx"
	}
	return "", false
}

// UploadURL issues a time-limited direct upload target for one report file.
func (s *LaporanService) UploadURL(ctx context.Context, p *entity.Principal, c LaporanCommand, in LaporanUploadURLRequest) (LaporanUploadURLResult, error) {
	var out LaporanUploadURLResult
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	jenis := strings.ToLower(strings.TrimSpace(in.Jenis))
	ext, ok := laporanUploadExt(jenis, in.Ext)
	if !ok {
		return out, ErrValidation
	}
	if store := laporanStore(s); !store.Enabled() {
		return out, ErrLaporanStorage
	}
	scope, err := laporanStudentScope(p)
	if err != nil {
		return out, err
	}
	err = s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		l, err := laporanActive(tx)
		if err != nil {
			return err
		}
		target, err := laporanTarget(l, scope.ClassID, scope.Shift)
		if err != nil {
			return authorization.ErrNotFound
		}
		now, err := tx.Now()
		if err != nil {
			return err
		}
		switch laporanFase(target, now) {
		case entity.LaporanBelumDibuka:
			return fmt.Errorf("%w: laporan besar belum dibuka", ErrLaporanWindow)
		case entity.LaporanDitutup:
			return fmt.Errorf("%w: waktu pengumpulan laporan besar telah ditutup", ErrLaporanWindow)
		}
		nowReal := time.Now()
		path := fmt.Sprintf("%s/%d/%s-%d.%s", l.ID, p.User.ID, jenis, nowReal.UnixNano(), ext)
		url, err := laporanStore(s).SignedUploadURL(ctx, path, laporanTTLUpload)
		if err != nil {
			return err
		}
		out = LaporanUploadURLResult{UploadURL: url, Path: path}
		return nil
	})
	return out, err
}

// Kumpul records the submission after re-validating the window server-side.
func (s *LaporanService) Kumpul(ctx context.Context, p *entity.Principal, c LaporanCommand, in LaporanKumpulInput) (entity.LaporanPengumpulan, error) {
	var out entity.LaporanPengumpulan
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	in.LaporanPath = strings.TrimSpace(in.LaporanPath)
	in.AnalisaPath = strings.TrimSpace(in.AnalisaPath)
	if len(in.LaporanPath) > 512 || len(in.AnalisaPath) > 512 {
		return out, ErrValidation
	}
	scope, err := laporanStudentScope(p)
	if err != nil {
		return out, err
	}
	return laporanMutate(s, ctx, p, c, "laporan.kumpul", scope.ClassID, in, func(tx repository.AcademicTx, replay string) (entity.LaporanPengumpulan, string, entity.Scope, error) {
		l, err := laporanActive(tx)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		target, err := laporanTarget(l, scope.ClassID, scope.Shift)
		if err != nil {
			return out, "", entity.Scope{}, authorization.ErrNotFound
		}
		now, err := tx.Now()
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		fase := laporanFase(target, now)
		if fase == entity.LaporanBelumDibuka {
			return out, "", entity.Scope{}, fmt.Errorf("%w: laporan besar belum dibuka", ErrLaporanWindow)
		}
		if fase == entity.LaporanDitutup {
			return out, "", entity.Scope{}, fmt.Errorf("%w: waktu pengumpulan laporan besar telah ditutup", ErrLaporanWindow)
		}
		if l.ButuhLaporan && in.LaporanPath == "" {
			return out, "", entity.Scope{}, fmt.Errorf("%w: berkas laporan wajib diunggah", ErrValidation)
		}
		if l.ButuhAnalisa && in.AnalisaPath == "" {
			return out, "", entity.Scope{}, fmt.Errorf("%w: berkas analisa wajib diunggah", ErrValidation)
		}
		if in.LaporanPath == "" && in.AnalisaPath == "" {
			return out, "", entity.Scope{}, ErrValidation
		}
		id := fmt.Sprintf("lap-pg-%s-%d", l.ID, p.User.ID)
		targetID := id
		existing, err := laporanRead[entity.LaporanPengumpulan](tx, repository.AcademicLaporanPengumpulan, id, true)
		if err == nil {
			// Preserve earlier object paths plus any manual or override trail.
			if in.LaporanPath == "" {
				in.LaporanPath = existing.LaporanPath
			}
			if in.AnalisaPath == "" {
				in.AnalisaPath = existing.AnalisaPath
			}
			existing.Status = "terkumpul"
			existing.Sumber = "web"
			existing.LaporanPath = in.LaporanPath
			existing.AnalisaPath = in.AnalisaPath
			existing.SubmittedAt = &now
			existing.IsLate = fase == entity.LaporanTelat
			out, err = existing, laporanPut(tx, repository.AcademicLaporanPengumpulan, id, l.ID, p.User.ID, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, "terkumpul", fmt.Sprintf("%s:%d", l.ID, p.User.ID), nil, existing)
			return out, targetID, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, err
		} else if !errors.Is(err, repository.ErrAcademicNotFound) {
			return out, "", entity.Scope{}, err
		}
		out = entity.LaporanPengumpulan{
			ID: id, LaporanID: l.ID, UserID: p.User.ID, Status: "terkumpul", Sumber: "web",
			LaporanPath: in.LaporanPath, AnalisaPath: in.AnalisaPath, SubmittedAt: &now,
			IsLate: fase == entity.LaporanTelat, ClassID: scope.ClassID, Shift: scope.Shift, Version: 1,
		}
		return out, targetID, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, laporanPut(tx, repository.AcademicLaporanPengumpulan, id, l.ID, p.User.ID, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, "terkumpul", fmt.Sprintf("%s:%d", l.ID, p.User.ID), nil, out)
	})
}

// BatalkanKumpul resets the student's submission while no manual mark or
// override exists and the window is still open.
func (s *LaporanService) BatalkanKumpul(ctx context.Context, p *entity.Principal, c LaporanCommand) (entity.LaporanPengumpulan, error) {
	var out entity.LaporanPengumpulan
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	scope, err := laporanStudentScope(p)
	if err != nil {
		return out, err
	}
	return laporanMutate(s, ctx, p, c, "laporan.batalkan", scope.ClassID, struct{}{}, func(tx repository.AcademicTx, replay string) (entity.LaporanPengumpulan, string, entity.Scope, error) {
		l, err := laporanActive(tx)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		id := fmt.Sprintf("lap-pg-%s-%d", l.ID, p.User.ID)
		rec, err := laporanRead[entity.LaporanPengumpulan](tx, repository.AcademicLaporanPengumpulan, id, true)
		if err != nil {
			return out, "", entity.Scope{}, repository.ErrAcademicNotFound
		}
		if rec.ManualOleh != nil || rec.OverridePada != nil {
			return out, "", entity.Scope{}, authorization.ErrForbidden
		}
		if target, e := laporanTarget(l, scope.ClassID, scope.Shift); e == nil {
			if now, e := tx.Now(); e == nil && laporanFase(target, now) == entity.LaporanDitutup {
				return out, "", entity.Scope{}, fmt.Errorf("%w: waktu pengumpulan telah ditutup", ErrLaporanWindow)
			}
		}
		rec.Status = "belum"
		rec.Sumber = ""
		rec.SubmittedAt = nil
		rec.IsLate = false
		rec.Version++
		return rec, id, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, laporanPut(tx, repository.AcademicLaporanPengumpulan, id, l.ID, p.User.ID, entity.Scope{Domain: "siakad", ClassID: scope.ClassID, Shift: scope.Shift}, "belum", fmt.Sprintf("%s:%d", l.ID, p.User.ID), nil, rec)
	})
}

// Pengumpulan lists one laporan's submissions for the requesting staff.
func (s *LaporanService) Pengumpulan(ctx context.Context, p *entity.Principal, laporanID string, page PracticumPage) ([]entity.LaporanPengumpulanRow, error) {
	out := []entity.LaporanPengumpulanRow{}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !page.valid() {
		return out, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		l, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, laporanID, false)
		if err != nil {
			return err
		}
		if !laporanTargetsVisible(l, laporanScopes(p)) {
			return authorization.ErrForbidden
		}
		rows, err := tx.List(repository.AcademicLaporanPengumpulan, repository.AcademicFilter{ParentID: laporanID, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.LaporanPengumpulan
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			name := ""
			if s.options.IdentityRepository != nil {
				if u, e := s.options.IdentityRepository.FindUserByID(ctx, rec.UserID); e == nil {
					name = u.Name
				}
			}
			row := entity.LaporanPengumpulanRow{
				UserID: rec.UserID, UserName: name, ClassID: rec.ClassID, Shift: rec.Shift,
				Status: rec.Status, Sumber: rec.Sumber, LaporanPath: rec.LaporanPath, AnalisaPath: rec.AnalisaPath,
				SubmittedAt: rec.SubmittedAt, IsLate: rec.IsLate,
				ManualOleh: rec.ManualOleh, ManualPada: rec.ManualPada, ManualCatatan: rec.ManualCatatan,
				OverrideOleh: rec.OverrideOleh, OverrideAlasan: rec.OverrideAlasan, OverridePada: rec.OverridePada,
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

// laporanPengumpulanLocked loads or lazily creates one submission row.
func laporanPengumpulanLocked(tx repository.AcademicTx, laporanID string, userID int64, scope entity.Scope) (entity.LaporanPengumpulan, string, error) {
	id := fmt.Sprintf("lap-pg-%s-%d", laporanID, userID)
	rec, err := laporanRead[entity.LaporanPengumpulan](tx, repository.AcademicLaporanPengumpulan, id, true)
	if errors.Is(err, repository.ErrAcademicNotFound) {
		return entity.LaporanPengumpulan{ID: id, LaporanID: laporanID, UserID: userID, Status: "belum", ClassID: scope.ClassID, Shift: scope.Shift, Version: 1}, id, nil
	}
	return rec, id, err
}

// Mark records an assistant's manual check, or returns the row to belum.
func (s *LaporanService) Mark(ctx context.Context, p *entity.Principal, c LaporanCommand, laporanID, userID string, in LaporanMarkInput) (entity.LaporanPengumpulanRow, error) {
	var out entity.LaporanPengumpulanRow
	targetUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || targetUserID < 1 {
		return out, ErrValidation
	}
	if in.Version < 1 || (in.Status != "terkumpul" && in.Status != "belum") || len(in.Catatan) > 500 {
		return out, ErrValidation
	}
	return laporanMutate(s, ctx, p, c, "laporan.mark", laporanID, in, func(tx repository.AcademicTx, replay string) (entity.LaporanPengumpulanRow, string, entity.Scope, error) {
		l, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, laporanID, false)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if err := laporanRequire(p); err != nil {
			return out, "", entity.Scope{}, err
		}
		rec, id, err := laporanPengumpulanLocked(tx, laporanID, targetUserID, l.Scope)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if replay == "" {
			if rec.Version != in.Version {
				return out, "", entity.Scope{}, repository.ErrAcademicConflict
			}
			now, e := tx.Now()
			if e != nil {
				return out, "", entity.Scope{}, e
			}
			if in.Status == "terkumpul" {
				rec.Status = "terkumpul"
				rec.Sumber = "manual"
				rec.ManualOleh = &p.User.ID
				rec.ManualPada = &now
				rec.ManualCatatan = strings.TrimSpace(in.Catatan)
				if rec.SubmittedAt == nil {
					rec.SubmittedAt = &now
				}
			} else {
				rec.Status = "belum"
				rec.Sumber = ""
				rec.ManualOleh, rec.ManualPada, rec.ManualCatatan = nil, nil, ""
			}
			rec.Version++
			if err = laporanPut(tx, repository.AcademicLaporanPengumpulan, id, laporanID, targetUserID, l.Scope, rec.Status, fmt.Sprintf("%s:%d", laporanID, targetUserID), nil, rec); err != nil {
				return out, "", entity.Scope{}, err
			}
		}
		return laporanRow(rec), id, l.Scope, nil
	})
}

// SetOverride records a reasoned exam-gate override without changing status.
func (s *LaporanService) SetOverride(ctx context.Context, p *entity.Principal, c LaporanCommand, laporanID, userID string, in LaporanOverrideInput) (entity.LaporanPengumpulanRow, error) {
	var out entity.LaporanPengumpulanRow
	targetUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || targetUserID < 1 {
		return out, ErrValidation
	}
	if in.Version < 1 || len(strings.TrimSpace(in.Alasan)) < 3 || len(in.Alasan) > 500 {
		return out, ErrValidation
	}
	return laporanMutate(s, ctx, p, c, "laporan.override.set", laporanID, in, func(tx repository.AcademicTx, replay string) (entity.LaporanPengumpulanRow, string, entity.Scope, error) {
		l, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, laporanID, false)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if err := laporanRequire(p); err != nil {
			return out, "", entity.Scope{}, err
		}
		rec, id, err := laporanPengumpulanLocked(tx, laporanID, targetUserID, l.Scope)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if replay == "" {
			if rec.Version != in.Version {
				return out, "", entity.Scope{}, repository.ErrAcademicConflict
			}
			now, e := tx.Now()
			if e != nil {
				return out, "", entity.Scope{}, e
			}
			rec.OverrideOleh = &p.User.ID
			rec.OverrideAlasan = strings.TrimSpace(in.Alasan)
			rec.OverridePada = &now
			rec.Version++
			if err = laporanPut(tx, repository.AcademicLaporanPengumpulan, id, laporanID, targetUserID, l.Scope, rec.Status, fmt.Sprintf("%s:%d", laporanID, targetUserID), nil, rec); err != nil {
				return out, "", entity.Scope{}, err
			}
		}
		return laporanRow(rec), id, l.Scope, nil
	})
}

// RevokeOverride clears a previously recorded override.
func (s *LaporanService) RevokeOverride(ctx context.Context, p *entity.Principal, c LaporanCommand, laporanID, userID string, in LaporanOverrideInput) (entity.LaporanPengumpulanRow, error) {
	var out entity.LaporanPengumpulanRow
	targetUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || targetUserID < 1 {
		return out, ErrValidation
	}
	return laporanMutate(s, ctx, p, c, "laporan.override.revoke", laporanID, in, func(tx repository.AcademicTx, replay string) (entity.LaporanPengumpulanRow, string, entity.Scope, error) {
		l, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, laporanID, false)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if err := laporanRequire(p); err != nil {
			return out, "", entity.Scope{}, err
		}
		rec, id, err := laporanPengumpulanLocked(tx, laporanID, targetUserID, l.Scope)
		if err != nil {
			return out, "", entity.Scope{}, err
		}
		if rec.OverridePada == nil {
			return out, "", entity.Scope{}, repository.ErrAcademicNotFound
		}
		if replay == "" {
			if rec.Version != in.Version {
				return out, "", entity.Scope{}, repository.ErrAcademicConflict
			}
			rec.OverrideOleh, rec.OverridePada = nil, nil
			rec.OverrideAlasan = ""
			rec.Version++
			if err = laporanPut(tx, repository.AcademicLaporanPengumpulan, id, laporanID, targetUserID, l.Scope, rec.Status, fmt.Sprintf("%s:%d", laporanID, targetUserID), nil, rec); err != nil {
				return out, "", entity.Scope{}, err
			}
		}
		return laporanRow(rec), id, l.Scope, nil
	})
}

// laporanRow strips nothing: admin recap carries paths and audit trails.
func laporanRow(rec entity.LaporanPengumpulan) entity.LaporanPengumpulanRow {
	return entity.LaporanPengumpulanRow{
		UserID: rec.UserID, ClassID: rec.ClassID, Shift: rec.Shift,
		Status: rec.Status, Sumber: rec.Sumber, LaporanPath: rec.LaporanPath, AnalisaPath: rec.AnalisaPath,
		SubmittedAt: rec.SubmittedAt, IsLate: rec.IsLate,
		ManualOleh: rec.ManualOleh, ManualPada: rec.ManualPada, ManualCatatan: rec.ManualCatatan,
		OverrideOleh: rec.OverrideOleh, OverrideAlasan: rec.OverrideAlasan, OverridePada: rec.OverridePada,
		Version: rec.Version,
	}
}

// SignedBerkas returns a short-lived read URL for one stored report file.
// Students may only reach their own objects; staff need laporan.manage.
func (s *LaporanService) SignedBerkas(ctx context.Context, p *entity.Principal, laporanID, userID, jenis string, forDownload bool) (LaporanBerkasResult, error) {
	var out LaporanBerkasResult
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	jenis = strings.ToLower(strings.TrimSpace(jenis))
	if jenis != "laporan" && jenis != "analisa" {
		return out, ErrValidation
	}
	targetUserID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || targetUserID < 1 {
		return out, ErrValidation
	}
	isStaff := laporanRequire(p) == nil
	if !isStaff && p.User.ID != targetUserID {
		return out, authorization.ErrForbidden
	}
	err = s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		l, err := laporanRead[entity.LaporanBesar](tx, repository.AcademicLaporanBesar, laporanID, false)
		if err != nil {
			return err
		}
		if isStaff && !laporanTargetsVisible(l, laporanScopes(p)) {
			return authorization.ErrForbidden
		}
		rows, err := tx.List(repository.AcademicLaporanPengumpulan, repository.AcademicFilter{ParentID: laporanID, OwnerID: targetUserID, Limit: 1})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return repository.ErrAcademicNotFound
		}
		var rec entity.LaporanPengumpulan
		if err = json.Unmarshal(rows[0].Data, &rec); err != nil {
			return err
		}
		path := rec.LaporanPath
		if jenis == "analisa" {
			path = rec.AnalisaPath
		}
		if path == "" {
			return repository.ErrAcademicNotFound
		}
		ext := path[strings.LastIndex(path, "."):]
		if ext == "" || strings.ContainsAny(ext, "/") {
			ext = ".bin"
		}
		name := fmt.Sprintf("%d-%s%s", targetUserID, jenis, ext)
		url, err := laporanStore(s).SignedDownloadURL(ctx, path, laporanTTLRead, name)
		if err != nil {
			return err
		}
		out = LaporanBerkasResult{URL: url, Filename: name}
		return nil
	})
	return out, err
}

type LaporanUploadURLResult struct {
	UploadURL string `json:"upload_url"`
	Path      string `json:"path"`
}

type LaporanBerkasResult struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}
