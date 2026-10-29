package usecase

import (
	"fmt"
	"path"
	"strings"
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
	"lab-ap/pkg/supabase"
)

// Fase window pengumpulan laporan besar (server-authoritative, waktu dari server).
const (
	FaseBelumDibuka = "belum_dibuka"
	FaseDibuka      = "dibuka"
	FaseTelat       = "telat"
	FaseDitutup     = "ditutup"
)

// TTL signed URL: unggah 30 menit (cukup untuk berkas besar), baca 5 menit.
const (
	ttlUploadLaporan = 30 * time.Minute
	ttlBacaLaporan   = 5 * time.Minute
)

type LaporanBesarUsecase struct {
	repo    repository.LaporanBesarRepository
	users   repository.UserRepository
	sb      *supabase.Client // bucket privat laporan besar
	ttlRead time.Duration
	ttlUp   time.Duration
}

func NewLaporanBesarUsecase(
	r repository.LaporanBesarRepository,
	u repository.UserRepository,
	sb *supabase.Client,
) *LaporanBesarUsecase {
	return &LaporanBesarUsecase{repo: r, users: u, sb: sb, ttlRead: ttlBacaLaporan, ttlUp: ttlUploadLaporan}
}

// ---- Helper waktu ----

// faseWindow menghitung fase window untuk satu target pada waktu now.
// now < buka_at => belum_dibuka; buka_at<=now<deadline_at => dibuka;
// deadline_at<=now<=late_until => telat; selain itu => ditutup.
func faseWindow(t *entity.LaporanBesarTarget, now time.Time) string {
	if t.BukaAt != nil && now.Before(*t.BukaAt) {
		return FaseBelumDibuka
	}
	if now.Before(t.DeadlineAt) {
		return FaseDibuka
	}
	if t.LateUntil != nil && !now.After(*t.LateUntil) {
		return FaseTelat
	}
	return FaseDitutup
}

// batasFase mengembalikan waktu batas fase aktif (untuk hitung sisa waktu).
func batasFase(t *entity.LaporanBesarTarget, fase string) *time.Time {
	switch fase {
	case FaseBelumDibuka:
		return t.BukaAt
	case FaseDibuka:
		d := t.DeadlineAt
		return &d
	case FaseTelat:
		return t.LateUntil
	default:
		return nil
	}
}

func sisaDetik(batas *time.Time, now time.Time) int64 {
	if batas == nil {
		return 0
	}
	return int64(batas.Sub(now).Seconds())
}

// ---- Laporan induk (admin) ----

func (uc *LaporanBesarUsecase) List() ([]entity.LaporanBesar, error) { return uc.repo.List() }

func (uc *LaporanBesarUsecase) Create(creatorID int, req dto.LaporanBesarRequest) (*entity.LaporanBesar, error) {
	judul := strings.TrimSpace(req.Judul)
	if judul == "" {
		judul = "Laporan Besar"
	}
	l := &entity.LaporanBesar{
		Judul:        judul,
		Deskripsi:    req.Deskripsi,
		IsAktif:      boolOr(req.IsAktif, true),
		ButuhLaporan: boolOr(req.ButuhLaporan, true),
		ButuhAnalisa: boolOr(req.ButuhAnalisa, true),
		CreatedBy:    &creatorID,
		CreatedAt:    time.Now(),
	}
	if err := uc.repo.Create(l); err != nil {
		return nil, err
	}
	return l, nil
}

func (uc *LaporanBesarUsecase) Update(id int, req dto.LaporanBesarRequest) (*entity.LaporanBesar, error) {
	l, err := uc.repo.FindByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if j := strings.TrimSpace(req.Judul); j != "" {
		l.Judul = j
	}
	l.Deskripsi = req.Deskripsi
	if req.IsAktif != nil {
		l.IsAktif = *req.IsAktif
	}
	if req.ButuhLaporan != nil {
		l.ButuhLaporan = *req.ButuhLaporan
	}
	if req.ButuhAnalisa != nil {
		l.ButuhAnalisa = *req.ButuhAnalisa
	}
	now := time.Now()
	l.UpdatedAt = &now
	if err := uc.repo.Update(l); err != nil {
		return nil, err
	}
	return l, nil
}

func (uc *LaporanBesarUsecase) Delete(id int) error {
	if _, err := uc.repo.FindByID(id); err != nil {
		return ErrNotFound
	}
	return uc.repo.Delete(id)
}

// GetDetail: detail laporan + seluruh target kelas.
func (uc *LaporanBesarUsecase) GetDetail(id int) (*entity.LaporanBesar, []entity.LaporanBesarTarget, error) {
	l, err := uc.repo.FindByID(id)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	ts, err := uc.repo.ListTarget(id)
	if err != nil {
		return nil, nil, err
	}
	return l, ts, nil
}

// SetTargets mengganti (replace) seluruh window kelas untuk laporan ini.
func (uc *LaporanBesarUsecase) SetTargets(laporanID int, req dto.SetTargetRequest) ([]entity.LaporanBesarTarget, error) {
	if _, err := uc.repo.FindByID(laporanID); err != nil {
		return nil, ErrNotFound
	}
	for _, t := range req.Targets {
		if t.LateUntil != nil && t.LateUntil.Before(t.DeadlineAt) {
			return nil, fmt.Errorf("%w: late_until tidak boleh sebelum deadline_at", ErrBadRequest)
		}
		if t.BukaAt != nil && t.BukaAt.After(t.DeadlineAt) {
			return nil, fmt.Errorf("%w: buka_at tidak boleh setelah deadline_at", ErrBadRequest)
		}
	}
	for _, t := range req.Targets {
		target := &entity.LaporanBesarTarget{
			LaporanBesarID: laporanID,
			KelasID:        t.KelasID,
			BukaAt:         t.BukaAt,
			DeadlineAt:     t.DeadlineAt,
			LateUntil:      t.LateUntil,
		}
		if err := uc.repo.UpsertTarget(target); err != nil {
			return nil, err
		}
	}
	return uc.repo.ListTarget(laporanID)
}

// ---- Mahasiswa: status & pengumpulan ----

// StatusSaya mengembalikan status laporan besar mahasiswa + window kelasnya.
func (uc *LaporanBesarUsecase) StatusSaya(userID int) (*dto.LaporanBesarStatusResponse, error) {
	user, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	l, err := uc.repo.FindAktif()
	if err != nil {
		return nil, ErrNotFound
	}
	resp := &dto.LaporanBesarStatusResponse{
		LaporanBesarID: l.ID,
		Judul:          l.Judul,
		Deskripsi:      l.Deskripsi,
		ButuhLaporan:   l.ButuhLaporan,
		ButuhAnalisa:   l.ButuhAnalisa,
		Fase:           "",
		Status:         string(entity.LaporanBelum),
	}
	now := time.Now()

	// Window kelas mahasiswa (bila ada).
	if user.KelasID != nil {
		resp.KelasID = user.KelasID
		if t, err := uc.repo.FindTarget(l.ID, *user.KelasID); err == nil {
			resp.BukaAt = t.BukaAt
			d := t.DeadlineAt
			resp.DeadlineAt = &d
			resp.LateUntil = t.LateUntil
			resp.Fase = faseWindow(t, now)
			resp.SisaDetik = sisaDetik(batasFase(t, resp.Fase), now)
			resp.BisaKumpul = resp.Fase == FaseDibuka || resp.Fase == FaseTelat
		}
	}

	// Baris pengumpulan (bila sudah pernah ada).
	if p, err := uc.repo.FindPengumpulan(l.ID, userID); err == nil {
		resp.Status = p.Status
		resp.Sumber = p.Sumber
		resp.LaporanPath = p.LaporanPath
		resp.AnalisaPath = p.AnalisaPath
		resp.SubmittedAt = p.SubmittedAt
		resp.Terlambat = p.IsLate
		resp.OverridePada = p.OverridePada
		resp.OverrideAlasan = p.OverrideAlasan
	}
	return resp, nil
}

// UploadURL membuat signed upload URL + path unik untuk berkas mahasiswa.
func (uc *LaporanBesarUsecase) UploadURL(userID int, req dto.UploadURLRequest) (*dto.UploadURLResponse, error) {
	if !uc.sb.Enabled() {
		return nil, fmt.Errorf("%w: penyimpanan berkas belum dikonfigurasi", ErrConflict)
	}
	user, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	l, err := uc.repo.FindAktif()
	if err != nil {
		return nil, ErrNotFound
	}
	now := time.Now()
	if user.KelasID == nil {
		return nil, fmt.Errorf("%w: kelas mahasiswa belum diatur", ErrBadRequest)
	}
	t, err := uc.repo.FindTarget(l.ID, *user.KelasID)
	if err != nil {
		return nil, fmt.Errorf("%w: laporan besar belum dibuka untuk kelas Anda", ErrForbidden)
	}
	fase := faseWindow(t, now)
	if fase == FaseBelumDibuka {
		return nil, fmt.Errorf("%w: laporan besar belum dibuka", ErrForbidden)
	}
	if fase == FaseDitutup {
		return nil, fmt.Errorf("%w: waktu pengumpulan laporan besar telah ditutup", ErrForbidden)
	}

	ext := strings.ToLower(strings.TrimSpace(req.Ext))
	jenis := req.Jenis
	// Path: laporan-besar/<laporanID>/<mhsID>/<jenis>-<unixnano>.<ext>
	objPath := fmt.Sprintf("%d/%d/%s-%d.%s", l.ID, userID, jenis, now.UnixNano(), ext)

	url, err := uc.sb.SignedUploadURL(objPath, uc.ttlUp)
	if err != nil {
		return nil, err
	}
	return &dto.UploadURLResponse{UploadURL: url, Path: objPath}, nil
}

// Kumpul: re-validate waktu di server, lalu upsert baris pengumpulan.
func (uc *LaporanBesarUsecase) Kumpul(userID int, req dto.KumpulRequest) (*entity.PengumpulanLaporanBesar, error) {
	user, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	l, err := uc.repo.FindAktif()
	if err != nil {
		return nil, ErrNotFound
	}
	if user.KelasID == nil {
		return nil, fmt.Errorf("%w: kelas mahasiswa belum diatur", ErrBadRequest)
	}
	t, err := uc.repo.FindTarget(l.ID, *user.KelasID)
	if err != nil {
		return nil, fmt.Errorf("%w: laporan besar belum dibuka untuk kelas Anda", ErrForbidden)
	}

	now := time.Now()
	fase := faseWindow(t, now)
	if fase == FaseBelumDibuka {
		return nil, fmt.Errorf("%w: laporan besar belum dibuka", ErrForbidden)
	}
	if fase == FaseDitutup {
		return nil, fmt.Errorf("%w: waktu pengumpulan laporan besar telah ditutup", ErrForbidden)
	}

	// Validasi berkas wajib sesuai konfigurasi laporan.
	laporanPath := trimPtr(req.LaporanPath)
	analisaPath := trimPtr(req.AnalisaPath)
	if l.ButuhLaporan && laporanPath == nil {
		return nil, fmt.Errorf("%w: berkas laporan (PDF) wajib diunggah", ErrBadRequest)
	}
	if l.ButuhAnalisa && analisaPath == nil {
		return nil, fmt.Errorf("%w: berkas analisa (DOCX) wajib diunggah", ErrBadRequest)
	}
	if laporanPath == nil && analisaPath == nil {
		return nil, fmt.Errorf("%w: tidak ada berkas yang diunggah", ErrBadRequest)
	}

	// Ambil baris lama (kalau ada) supaya path yang tidak dikirim tetap dipertahankan.
	var existing *entity.PengumpulanLaporanBesar
	if p, err := uc.repo.FindPengumpulan(l.ID, userID); err == nil {
		existing = p
	}

	p := &entity.PengumpulanLaporanBesar{
		LaporanBesarID: l.ID,
		MahasiswaID:    userID,
		Status:         string(entity.LaporanTerkumpul),
	}
	sumber := string(entity.LaporanSumberWeb)
	p.Sumber = &sumber
	p.SubmittedAt = &now
	p.IsLate = fase == FaseTelat

	if laporanPath != nil {
		p.LaporanPath = laporanPath
	} else if existing != nil {
		p.LaporanPath = existing.LaporanPath
	}
	if analisaPath != nil {
		p.AnalisaPath = analisaPath
	} else if existing != nil {
		p.AnalisaPath = existing.AnalisaPath
	}
	// Pertahankan centang manual & override yang sudah ada.
	if existing != nil {
		p.ManualOleh = existing.ManualOleh
		p.ManualPada = existing.ManualPada
		p.ManualCatatan = existing.ManualCatatan
		p.OverrideOleh = existing.OverrideOleh
		p.OverrideAlasan = existing.OverrideAlasan
		p.OverridePada = existing.OverridePada
	}

	// Ambil signed URL (public URL TIDAK dipakai; bucket privat).
	if p.LaporanPath != nil {
		if u, err := uc.sb.SignedDownloadURL(*p.LaporanPath, uc.ttlRead, ""); err == nil {
			p.LaporanURL = &u
		}
	}
	if p.AnalisaPath != nil {
		if u, err := uc.sb.SignedDownloadURL(*p.AnalisaPath, uc.ttlRead, ""); err == nil {
			p.AnalisaURL = &u
		}
	}

	if err := uc.repo.UpsertPengumpulan(p); err != nil {
		return nil, err
	}
	// Baca ulang agar baris yang tersimpan otoritatif.
	if fresh, err := uc.repo.FindPengumpulan(l.ID, userID); err == nil {
		return fresh, nil
	}
	return p, nil
}

// BatalkanKumpul: mahasiswa membatalkan pengumpulan. Hanya boleh bila baris ada,
// belum dicentang manual/override, dan window masih terbuka.
func (uc *LaporanBesarUsecase) BatalkanKumpul(userID int) error {
	user, err := uc.users.FindByID(userID)
	if err != nil {
		return ErrUnauthorized
	}
	l, err := uc.repo.FindAktif()
	if err != nil {
		return ErrNotFound
	}
	p, err := uc.repo.FindPengumpulan(l.ID, userID)
	if err != nil {
		return fmt.Errorf("%w: belum ada pengumpulan untuk dibatalkan", ErrNotFound)
	}
	if p.ManualOleh != nil || p.OverridePada != nil {
		return fmt.Errorf("%w: pengumpulan yang sudah diverifikasi/di-override tidak bisa dibatalkan", ErrForbidden)
	}
	if user.KelasID != nil {
		if t, err := uc.repo.FindTarget(l.ID, *user.KelasID); err == nil {
			if faseWindow(t, time.Now()) == FaseDitutup {
				return fmt.Errorf("%w: waktu pengumpulan telah ditutup", ErrForbidden)
			}
		}
	}
	p.Status = string(entity.LaporanBelum)
	p.Sumber = nil
	p.SubmittedAt = nil
	p.IsLate = false
	p.LaporanURL = nil
	p.AnalisaURL = nil
	return uc.repo.UpsertPengumpulan(p)
}

// ---- Admin: rekap pengumpulan ----

// ListPengumpulan: daftar seluruh mahasiswa dari kelas yang ditarget + status + jejak.
func (uc *LaporanBesarUsecase) ListPengumpulan(laporanID int) ([]dto.PengumpulanRow, error) {
	if _, err := uc.repo.FindByID(laporanID); err != nil {
		return nil, ErrNotFound
	}
	targets, err := uc.repo.ListTarget(laporanID)
	if err != nil {
		return nil, err
	}
	kelasIDs := make([]int, 0, len(targets))
	for _, t := range targets {
		kelasIDs = append(kelasIDs, t.KelasID)
	}
	mhs, err := uc.repo.ListMahasiswaByKelas(kelasIDs)
	if err != nil {
		return nil, err
	}
	rows, err := uc.repo.ListPengumpulan(laporanID)
	if err != nil {
		return nil, err
	}
	byMhs := make(map[int]entity.PengumpulanLaporanBesar, len(rows))
	for _, r := range rows {
		byMhs[r.MahasiswaID] = r
	}

	out := make([]dto.PengumpulanRow, 0, len(mhs))
	for _, m := range mhs {
		row := dto.PengumpulanRow{
			MahasiswaID: m.ID,
			NIM:         m.NIM,
			Nama:        m.Nama,
			KelasID:     m.KelasID,
			Status:      string(entity.LaporanBelum),
		}
		if m.Kelas != nil {
			nama := m.Kelas.NamaKelas
			row.NamaKelas = &nama
		}
		if p, ok := byMhs[m.ID]; ok {
			row.Status = p.Status
			row.Sumber = p.Sumber
			row.SubmittedAt = p.SubmittedAt
			row.IsLate = p.IsLate
			row.LaporanPath = p.LaporanPath
			row.AnalisaPath = p.AnalisaPath
			row.ManualOleh = p.ManualOleh
			row.ManualPada = p.ManualPada
			row.ManualCatatan = p.ManualCatatan
			row.OverrideOleh = p.OverrideOleh
			row.OverrideAlasan = p.OverrideAlasan
			row.OverridePada = p.OverridePada
		}
		out = append(out, row)
	}
	return out, nil
}

// Mark: centang manual oleh asisten (sumber=manual). Status 'terkumpul' mencatat jejak.
func (uc *LaporanBesarUsecase) Mark(laporanID, mhsID, olehID int, req dto.MarkRequest) (*entity.PengumpulanLaporanBesar, error) {
	if _, err := uc.repo.FindByID(laporanID); err != nil {
		return nil, ErrNotFound
	}
	p, err := uc.repo.FindPengumpulan(laporanID, mhsID)
	if err != nil {
		// Buat baris lazy.
		p = &entity.PengumpulanLaporanBesar{
			LaporanBesarID: laporanID,
			MahasiswaID:    mhsID,
			Status:         string(entity.LaporanBelum),
		}
	}
	if req.Status == string(entity.LaporanTerkumpul) {
		now := time.Now()
		sumber := string(entity.LaporanSumberManual)
		p.Status = string(entity.LaporanTerkumpul)
		p.Sumber = &sumber
		p.ManualOleh = &olehID
		p.ManualPada = &now
		if req.Catatan != "" {
			cat := req.Catatan
			p.ManualCatatan = &cat
		}
		if p.SubmittedAt == nil {
			p.SubmittedAt = &now
		}
	} else {
		// Kembalikan ke belum: bersihkan jejak manual, pertahankan override.
		p.Status = string(entity.LaporanBelum)
		p.Sumber = nil
		p.ManualOleh = nil
		p.ManualPada = nil
		p.ManualCatatan = nil
	}
	if err := uc.repo.UpsertPengumpulan(p); err != nil {
		return nil, err
	}
	if fresh, err := uc.repo.FindPengumpulan(laporanID, mhsID); err == nil {
		return fresh, nil
	}
	return p, nil
}

// SetOverride: buka blokir gerbang ujian (TIDAK mengubah status).
func (uc *LaporanBesarUsecase) SetOverride(laporanID, mhsID, olehID int, req dto.OverrideRequest) (*entity.PengumpulanLaporanBesar, error) {
	if _, err := uc.repo.FindByID(laporanID); err != nil {
		return nil, ErrNotFound
	}
	alasan := strings.TrimSpace(req.Alasan)
	if alasan == "" {
		return nil, fmt.Errorf("%w: alasan override wajib diisi", ErrBadRequest)
	}
	p, err := uc.repo.FindPengumpulan(laporanID, mhsID)
	if err != nil {
		p = &entity.PengumpulanLaporanBesar{
			LaporanBesarID: laporanID,
			MahasiswaID:    mhsID,
			Status:         string(entity.LaporanBelum),
		}
	}
	now := time.Now()
	p.OverrideOleh = &olehID
	p.OverrideAlasan = &alasan
	p.OverridePada = &now
	if err := uc.repo.UpsertPengumpulan(p); err != nil {
		return nil, err
	}
	if fresh, err := uc.repo.FindPengumpulan(laporanID, mhsID); err == nil {
		return fresh, nil
	}
	return p, nil
}

// RevokeOverride: cabut override (set NULL).
func (uc *LaporanBesarUsecase) RevokeOverride(laporanID, mhsID int) (*entity.PengumpulanLaporanBesar, error) {
	p, err := uc.repo.FindPengumpulan(laporanID, mhsID)
	if err != nil {
		return nil, fmt.Errorf("%w: tidak ada override untuk dicabut", ErrNotFound)
	}
	p.OverrideOleh = nil
	p.OverrideAlasan = nil
	p.OverridePada = nil
	if err := uc.repo.UpsertPengumpulan(p); err != nil {
		return nil, err
	}
	if fresh, err := uc.repo.FindPengumpulan(laporanID, mhsID); err == nil {
		return fresh, nil
	}
	return p, nil
}

// ---- Akses berkas (privat) ----

// SignedBerkas: validasi izin + kembalikan signed URL. mahasiswaID==0 (staf) = akses penuh.
// Bila mahasiswaID != 0, mahasiswa hanya boleh mengakses berkasnya sendiri.
func (uc *LaporanBesarUsecase) SignedBerkas(laporanID, mhsID, pemintaID int, jenis string, forDownload bool) (string, string, error) {
	if pemintaID != 0 && pemintaID != mhsID {
		return "", "", ErrForbidden
	}
	jenis = strings.ToLower(strings.TrimSpace(jenis))
	if jenis != "laporan" && jenis != "analisa" {
		return "", "", fmt.Errorf("%w: jenis berkas tidak valid", ErrBadRequest)
	}
	p, err := uc.repo.FindPengumpulan(laporanID, mhsID)
	if err != nil {
		return "", "", fmt.Errorf("%w: berkas belum ada", ErrNotFound)
	}
	var objPath *string
	if jenis == "laporan" {
		objPath = p.LaporanPath
	} else {
		objPath = p.AnalisaPath
	}
	if objPath == nil || *objPath == "" {
		return "", "", fmt.Errorf("%w: berkas %s belum diunggah", ErrNotFound, jenis)
	}
	name := namaUnduhan(p.MahasiswaID, jenis, *objPath)
	if forDownload {
		u, err := uc.sb.SignedDownloadURL(*objPath, uc.ttlRead, name)
		if err != nil {
			return "", "", err
		}
		return u, name, nil
	}
	u, err := uc.sb.SignedDownloadURL(*objPath, uc.ttlRead, "")
	if err != nil {
		return "", "", err
	}
	return u, name, nil
}

// DownloadBatch mengembalikan daftar signed URL (tanpa zip server-side).
func (uc *LaporanBesarUsecase) DownloadBatch(laporanID int, req dto.DownloadBatchRequest) ([]dto.DownloadBatchResult, error) {
	if _, err := uc.repo.FindByID(laporanID); err != nil {
		return nil, ErrNotFound
	}
	out := make([]dto.DownloadBatchResult, 0, len(req.Items))
	for _, it := range req.Items {
		jenis := strings.ToLower(strings.TrimSpace(it.Jenis))
		if jenis != "laporan" && jenis != "analisa" {
			continue
		}
		p, err := uc.repo.FindPengumpulan(laporanID, it.MhsID)
		if err != nil {
			continue
		}
		var objPath *string
		if jenis == "laporan" {
			objPath = p.LaporanPath
		} else {
			objPath = p.AnalisaPath
		}
		if objPath == nil || *objPath == "" {
			continue
		}
		name := namaUnduhan(it.MhsID, jenis, *objPath)
		u, err := uc.sb.SignedDownloadURL(*objPath, uc.ttlRead, name)
		if err != nil {
			continue
		}
		out = append(out, dto.DownloadBatchResult{
			MhsID:    it.MhsID,
			Jenis:    jenis,
			Filename: name,
			URL:      u,
		})
	}
	return out, nil
}

// ---- Gerbang ujian ----

// CekGerbangLaporanBesar dipanggil Mulai (ujian praktik). Bila ada laporan besar
// aktif untuk kelas mahasiswa, wajib status='terkumpul' ATAU ada override_pada.
// Mengembalikan error bila gerbang tertutup.
func (uc *LaporanBesarUsecase) CekGerbangLaporanBesar(userID int, kelasID *int) error {
	if kelasID == nil {
		return nil // tanpa kelas, tidak ada target → gerbang tidak berlaku
	}
	l, err := uc.repo.FindAktif()
	if err != nil || l == nil {
		return nil // tidak ada laporan besar aktif → gerbang tidak berlaku
	}
	if _, err := uc.repo.FindTarget(l.ID, *kelasID); err != nil {
		return nil // kelas ini tidak ditarget → gerbang tidak berlaku
	}
	p, err := uc.repo.FindPengumpulan(l.ID, userID)
	if err != nil {
		return fmt.Errorf("%w: Laporan Besar belum dikumpulkan. Hubungi asisten.", ErrBadRequest)
	}
	if p.OverridePada != nil {
		return nil
	}
	if p.Status != string(entity.LaporanTerkumpul) {
		return fmt.Errorf("%w: Laporan Besar belum dikumpulkan. Hubungi asisten.", ErrBadRequest)
	}
	return nil
}

// ---- util ----

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// namaUnduhan menyusun nama berkas unduhan yang ramah (jenis + ekstensi asli).
func namaUnduhan(mhsID int, jenis, objPath string) string {
	ext := path.Ext(objPath)
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("%d-%s%s", mhsID, jenis, ext)
}
