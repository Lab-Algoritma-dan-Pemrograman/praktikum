package repository

import (
	"errors"

	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

type JawabanRepository interface {
	Create(j *entity.JawabanMahasiswa) error
	Update(j *entity.JawabanMahasiswa) error
	FindByMahasiswaSoal(mahasiswaID, soalTerpilihID int) (*entity.JawabanMahasiswa, error)
	FindByID(id int) (*entity.JawabanMahasiswa, error)
	// ListByMahasiswaCourse: seluruh jawaban mahasiswa untuk satu aktivasi+course.
	ListByMahasiswaCourse(mahasiswaID, aktivasiSesiID, courseID int) ([]entity.JawabanMahasiswa, error)
	// MarkSubmittedForCourse: tandai submitted semua jawaban belum-submit pada aktivasi+course
	// (auto-submit massal saat course ditutup). Mengembalikan jumlah baris terdampak.
	MarkSubmittedForCourse(aktivasiSesiID, courseID int) (int64, error)
	// UnmarkSubmittedForCourse: kebalikan MarkSubmittedForCourse, dipakai saat
	// course DIBUKA ULANG agar peserta bisa mengerjakan & submit ulang.
	// Isi jawaban_teks dibiarkan (tidak dihapus), hanya status submit yang dilepas.
	UnmarkSubmittedForCourse(aktivasiSesiID, courseID int) (int64, error)
	// MarkSubmittedForMahasiswaCourse: auto-submit untuk 1 mahasiswa (timer habis).
	MarkSubmittedForMahasiswaCourse(mahasiswaID, aktivasiSesiID, courseID int) (int64, error)
	// SumNilai: total nilai mahasiswa untuk satu aktivasi+course.
	SumNilai(mahasiswaID, aktivasiSesiID, courseID int) (float64, error)
	// ListRekap: rekap jawaban untuk satu aktivasi+course (semua mahasiswa).
	ListRekap(aktivasiSesiID, courseID int) ([]entity.JawabanMahasiswa, error)
	// GetAllJawabanFlat: rekap global untuk halaman admin
	GetAllJawabanFlat(kelasID, sesiID int, search, jenis string, limit int) ([]entity.JawabanMahasiswa, error)
	// BulkResetNilai: reset nilai ke null untuk daftar jawaban_id
	BulkResetNilai(jawabanIDs []int) error
	// BulkDelete: hapus jawaban berdasarkan daftar jawaban_id
	BulkDelete(jawabanIDs []int) error
}

type jawabanRepository struct{ db *gorm.DB }

func NewJawabanRepository(db *gorm.DB) JawabanRepository { return &jawabanRepository{db: db} }

func (r *jawabanRepository) Create(j *entity.JawabanMahasiswa) error { return r.db.Create(j).Error }
func (r *jawabanRepository) Update(j *entity.JawabanMahasiswa) error { return r.db.Save(j).Error }

func (r *jawabanRepository) FindByMahasiswaSoal(mahasiswaID, soalTerpilihID int) (*entity.JawabanMahasiswa, error) {
	var j entity.JawabanMahasiswa
	err := r.db.Where("mahasiswa_id = ? AND soal_terpilih_id = ?", mahasiswaID, soalTerpilihID).
		First(&j).Error
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *jawabanRepository) FindByID(id int) (*entity.JawabanMahasiswa, error) {
	var j entity.JawabanMahasiswa
	if err := r.db.Preload("SoalTerpilih").Preload("SoalTerpilih.Soal").
		Preload("SoalTerpilih.Course").First(&j, id).Error; err != nil {
		return nil, err
	}
	return &j, nil
}

// soalTerpilihIDsForCourse helper: ambil id soal_terpilih untuk aktivasi+course.
func (r *jawabanRepository) soalTerpilihIDsForCourse(aktivasiSesiID, courseID int) ([]int, error) {
	var ids []int
	err := r.db.Model(&entity.SoalTerpilih{}).
		Where("aktivasi_sesi_id = ? AND course_id = ?", aktivasiSesiID, courseID).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *jawabanRepository) ListByMahasiswaCourse(mahasiswaID, aktivasiSesiID, courseID int) ([]entity.JawabanMahasiswa, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []entity.JawabanMahasiswa{}, nil
	}
	var js []entity.JawabanMahasiswa
	err = r.db.Preload("SoalTerpilih").Preload("SoalTerpilih.Soal").
		Where("mahasiswa_id = ? AND soal_terpilih_id IN ?", mahasiswaID, ids).
		Find(&js).Error
	return js, err
}

func (r *jawabanRepository) MarkSubmittedForCourse(aktivasiSesiID, courseID int) (int64, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.Model(&entity.JawabanMahasiswa{}).
		Where("soal_terpilih_id IN ? AND is_submitted = ?", ids, false).
		Updates(map[string]interface{}{"is_submitted": true, "waktu_submit": gorm.Expr("NOW()")})
	return res.RowsAffected, res.Error
}

// UnmarkSubmittedForCourse melepas status submit seluruh jawaban satu course (buka ulang ujian).
func (r *jawabanRepository) UnmarkSubmittedForCourse(aktivasiSesiID, courseID int) (int64, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.Model(&entity.JawabanMahasiswa{}).
		Where("soal_terpilih_id IN ? AND is_submitted = ?", ids, true).
		Updates(map[string]interface{}{"is_submitted": false, "waktu_submit": nil})
	return res.RowsAffected, res.Error
}

func (r *jawabanRepository) MarkSubmittedForMahasiswaCourse(mahasiswaID, aktivasiSesiID, courseID int) (int64, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.Model(&entity.JawabanMahasiswa{}).
		Where("mahasiswa_id = ? AND soal_terpilih_id IN ? AND is_submitted = ?", mahasiswaID, ids, false).
		Updates(map[string]interface{}{"is_submitted": true, "waktu_submit": gorm.Expr("NOW()")})
	return res.RowsAffected, res.Error
}

func (r *jawabanRepository) SumNilai(mahasiswaID, aktivasiSesiID, courseID int) (float64, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	var total *float64
	err = r.db.Model(&entity.JawabanMahasiswa{}).
		Where("mahasiswa_id = ? AND soal_terpilih_id IN ?", mahasiswaID, ids).
		Select("COALESCE(SUM(nilai),0)").Scan(&total).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

func (r *jawabanRepository) ListRekap(aktivasiSesiID, courseID int) ([]entity.JawabanMahasiswa, error) {
	ids, err := r.soalTerpilihIDsForCourse(aktivasiSesiID, courseID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []entity.JawabanMahasiswa{}, nil
	}
	var js []entity.JawabanMahasiswa
	err = r.db.Preload("SoalTerpilih").Preload("SoalTerpilih.Soal").
		Where("soal_terpilih_id IN ?", ids).
		Order("mahasiswa_id asc").Find(&js).Error
	return js, err
}

func (r *jawabanRepository) GetAllJawabanFlat(kelasID, sesiID int, search, jenis string, limit int) ([]entity.JawabanMahasiswa, error) {
	var js []entity.JawabanMahasiswa

	query := r.db.Preload("Mahasiswa").Preload("Mahasiswa.Kelas").
		Preload("SoalTerpilih").
		Preload("SoalTerpilih.Soal").
		Preload("SoalTerpilih.Course").
		Preload("SoalTerpilih.AktivasiSesi").
		Preload("SoalTerpilih.AktivasiSesi.Sesi")

	// Joins untuk memfilter
	query = query.Joins("JOIN users ON users.id = jawaban_mahasiswa.mahasiswa_id").
		Joins("JOIN soal_terpilih ON soal_terpilih.id = jawaban_mahasiswa.soal_terpilih_id").
		Joins("JOIN course ON course.id = soal_terpilih.course_id").
		Joins("JOIN aktivasi_sesi ON aktivasi_sesi.id = soal_terpilih.aktivasi_sesi_id")

	if kelasID > 0 {
		query = query.Where("users.kelas_id = ?", kelasID)
	}
	if sesiID > 0 {
		query = query.Where("aktivasi_sesi.sesi_praktikum_id = ?", sesiID)
	}
	if jenis != "" && jenis != "all" {
		query = query.Where("course.jenis = ?", jenis)
	}
	if search != "" {
		searchLike := "%" + search + "%"
		query = query.Where("(users.nama ILIKE ? OR users.nim ILIKE ?)", searchLike, searchLike)
	}

	query = query.Order("jawaban_mahasiswa.waktu_submit DESC")
	if limit > 0 {
		query = query.Limit(limit) // batasi read: tanpa search hanya N terbaru
	}
	err := query.Find(&js).Error
	return js, err
}

func (r *jawabanRepository) BulkResetNilai(jawabanIDs []int) error {
	if len(jawabanIDs) == 0 {
		return nil
	}
	return r.db.Model(&entity.JawabanMahasiswa{}).
		Where("id IN ?", jawabanIDs).
		Updates(map[string]interface{}{"nilai": nil, "feedback": nil}).Error
}

func (r *jawabanRepository) BulkDelete(jawabanIDs []int) error {
	if len(jawabanIDs) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", jawabanIDs).Delete(&entity.JawabanMahasiswa{}).Error
}
