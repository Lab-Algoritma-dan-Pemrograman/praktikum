package repository

import (
	"errors"
	"time"

	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

type PengerjaanRepository interface {
	Create(p *entity.PengerjaanCourse) error
	Update(p *entity.PengerjaanCourse) error
	Find(mahasiswaID, aktivasiSesiID, courseID int) (*entity.PengerjaanCourse, error)
	// FindOrCreate mengembalikan record yang ada atau membuat baru (status belum_dikerjakan).
	FindOrCreate(mahasiswaID, aktivasiSesiID, courseID int) (*entity.PengerjaanCourse, error)
	ListByMahasiswa(mahasiswaID int) ([]entity.PengerjaanCourse, error)
	ListByAktivasiCourse(aktivasiSesiID, courseID int) ([]entity.PengerjaanCourse, error)
	// MarkSelesaiForCourse: set status selesai utk semua yg belum selesai (mass auto-submit).
	MarkSelesaiForCourse(aktivasiSesiID, courseID int) error
	// ResetForCourse: lepas kunci pengerjaan seluruh peserta (status kembali
	// belum_dikerjakan, waktu_mulai/selesai dikosongkan). Dipakai saat course
	// DIBUKA ULANG: tanpa ini peserta tetap read-only di ruang ujian walau
	// course sudah is_open=true (gejala "tidak bisa mengetik").
	// Mengembalikan jumlah baris terdampak. total_nilai/keaktifan TIDAK diubah.
	ResetForCourse(aktivasiSesiID, courseID int) (int64, error)
	ProgressSummary(aktivasiSesiID, courseID int) (ProgressSummary, error)
	// FindExpired mengambil pengerjaan yang sedang berjalan & sudah lewat deadline (untuk sweeper).
	FindExpired() ([]ExpiredPengerjaan, error)
	// SetKeaktifanIfTest set keaktifan HANYA bila course pengerjaan = pretest/posttest.
	// Mengembalikan RowsAffected (0 = id tak ada / bukan pretest/posttest).
	SetKeaktifanIfTest(pengerjaanID int, nilai float64) (int64, error)
	// ListPesertaProgress: peserta yang sudah join/mengerjakan suatu aktivasi (semua course),
	// join nama/nim + judul course + status. Untuk monitoring live di panel admin.
	ListPesertaProgress(aktivasiSesiID int) ([]PesertaProgress, error)
}

// PesertaProgress: satu baris status pengerjaan peserta per course (untuk monitoring).
type PesertaProgress struct {
	MahasiswaID  int        `json:"mahasiswa_id"`
	NIM          string     `json:"nim"`
	Nama         string     `json:"nama"`
	CourseID     int        `json:"course_id"`
	JudulCourse  string     `json:"judul_course"`
	Status       string     `json:"status"`
	WaktuMulai   *time.Time `json:"waktu_mulai"`
	WaktuSelesai *time.Time `json:"waktu_selesai"`
}

// ProgressSummary: ringkasan progress per course aktivasi.
type ProgressSummary struct {
	Selesai int64 `json:"selesai"`
	Sedang  int64 `json:"sedang"`
	Belum   int64 `json:"belum"`
}

// ExpiredPengerjaan: pengerjaan yang sudah lewat deadline (waktu_mulai + durasi_menit).
type ExpiredPengerjaan struct {
	MahasiswaID    int
	AktivasiSesiID int
	CourseID       int
}

type pengerjaanRepository struct{ db *gorm.DB }

func NewPengerjaanRepository(db *gorm.DB) PengerjaanRepository {
	return &pengerjaanRepository{db: db}
}

func (r *pengerjaanRepository) Create(p *entity.PengerjaanCourse) error { return r.db.Create(p).Error }
func (r *pengerjaanRepository) Update(p *entity.PengerjaanCourse) error { return r.db.Save(p).Error }

func (r *pengerjaanRepository) Find(mahasiswaID, aktivasiSesiID, courseID int) (*entity.PengerjaanCourse, error) {
	var p entity.PengerjaanCourse
	err := r.db.Where("mahasiswa_id = ? AND aktivasi_sesi_id = ? AND course_id = ?",
		mahasiswaID, aktivasiSesiID, courseID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *pengerjaanRepository) FindOrCreate(mahasiswaID, aktivasiSesiID, courseID int) (*entity.PengerjaanCourse, error) {
	p, err := r.Find(mahasiswaID, aktivasiSesiID, courseID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	np := &entity.PengerjaanCourse{
		MahasiswaID:    mahasiswaID,
		AktivasiSesiID: aktivasiSesiID,
		CourseID:       courseID,
		Status:         entity.StatusBelum,
	}
	if err := r.db.Create(np).Error; err != nil {
		return nil, err
	}
	return np, nil
}

func (r *pengerjaanRepository) ListByMahasiswa(mahasiswaID int) ([]entity.PengerjaanCourse, error) {
	var ps []entity.PengerjaanCourse
	err := r.db.Where("mahasiswa_id = ?", mahasiswaID).Find(&ps).Error
	return ps, err
}

func (r *pengerjaanRepository) ListByAktivasiCourse(aktivasiSesiID, courseID int) ([]entity.PengerjaanCourse, error) {
	var ps []entity.PengerjaanCourse
	err := r.db.Where("aktivasi_sesi_id = ? AND course_id = ?", aktivasiSesiID, courseID).Find(&ps).Error
	return ps, err
}

func (r *pengerjaanRepository) ListPesertaProgress(aktivasiSesiID int) ([]PesertaProgress, error) {
	var out []PesertaProgress
	err := r.db.Table("pengerjaan_course AS pc").
		Select("pc.mahasiswa_id, u.nim, u.nama, pc.course_id, c.judul AS judul_course, pc.status, pc.waktu_mulai, pc.waktu_selesai").
		Joins("JOIN users u ON u.id = pc.mahasiswa_id").
		Joins("JOIN course c ON c.id = pc.course_id").
		Where("pc.aktivasi_sesi_id = ?", aktivasiSesiID).
		Order("u.nama ASC, pc.course_id ASC").
		Scan(&out).Error
	return out, err
}

func (r *pengerjaanRepository) MarkSelesaiForCourse(aktivasiSesiID, courseID int) error {
	return r.db.Model(&entity.PengerjaanCourse{}).
		Where("aktivasi_sesi_id = ? AND course_id = ? AND status <> ?", aktivasiSesiID, courseID, entity.StatusSelesai).
		Updates(map[string]interface{}{"status": entity.StatusSelesai, "waktu_selesai": gorm.Expr("NOW()")}).Error
}

// ResetForCourse melepas kunci seluruh peserta satu course (buka ulang ujian).
func (r *pengerjaanRepository) ResetForCourse(aktivasiSesiID, courseID int) (int64, error) {
	res := r.db.Model(&entity.PengerjaanCourse{}).
		Where("aktivasi_sesi_id = ? AND course_id = ? AND status <> ?", aktivasiSesiID, courseID, entity.StatusBelum).
		Updates(map[string]interface{}{
			"status":        entity.StatusBelum,
			"waktu_mulai":   nil,
			"waktu_selesai": nil,
		})
	return res.RowsAffected, res.Error
}

func (r *pengerjaanRepository) ProgressSummary(aktivasiSesiID, courseID int) (ProgressSummary, error) {
	var s ProgressSummary
	base := r.db.Model(&entity.PengerjaanCourse{}).
		Where("aktivasi_sesi_id = ? AND course_id = ?", aktivasiSesiID, courseID)
	if err := base.Session(&gorm.Session{}).Where("status = ?", entity.StatusSelesai).Count(&s.Selesai).Error; err != nil {
		return s, err
	}
	if err := base.Session(&gorm.Session{}).Where("status = ?", entity.StatusSedang).Count(&s.Sedang).Error; err != nil {
		return s, err
	}
	if err := base.Session(&gorm.Session{}).Where("status = ?", entity.StatusBelum).Count(&s.Belum).Error; err != nil {
		return s, err
	}
	return s, nil
}

func (r *pengerjaanRepository) SetKeaktifanIfTest(pengerjaanID int, nilai float64) (int64, error) {
	res := r.db.Exec(`UPDATE pengerjaan_course SET keaktifan = ?
		WHERE id = ? AND course_id IN (SELECT id FROM course WHERE jenis IN ('pretest','posttest'))`,
		nilai, pengerjaanID)
	return res.RowsAffected, res.Error
}

func (r *pengerjaanRepository) FindExpired() ([]ExpiredPengerjaan, error) {
	var out []ExpiredPengerjaan
	// Deadline = anchor + course.durasi_menit. Anchor: reguler pakai ac.started_at
	// (timer global, peserta pertama), susulan pakai pc.waktu_mulai (per-user).
	// Hanya yg status sedang_dikerjakan.
	anchor := "CASE WHEN ps.id IS NOT NULL THEN pc.waktu_mulai ELSE ac.started_at END"
	err := r.db.Table("pengerjaan_course AS pc").
		Select("pc.mahasiswa_id, pc.aktivasi_sesi_id, pc.course_id").
		Joins("JOIN course c ON c.id = pc.course_id").
		Joins("JOIN aktivasi_course ac ON ac.aktivasi_sesi_id = pc.aktivasi_sesi_id AND ac.course_id = pc.course_id").
		Joins("LEFT JOIN peserta_susulan ps ON ps.aktivasi_sesi_id = pc.aktivasi_sesi_id AND ps.mahasiswa_id = pc.mahasiswa_id").
		Where("pc.status = ?", entity.StatusSedang).
		Where("(" + anchor + ") IS NOT NULL").
		Where("NOW() > (" + anchor + ") + (c.durasi_menit * interval '1 minute')").
		Scan(&out).Error
	return out, err
}
