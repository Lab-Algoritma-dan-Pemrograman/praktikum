package repository

import (
	"errors"

	"lab-ap/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrLaporanBesarTanpaTarget: tidak ada baris target untuk kelas mahasiswa.
var ErrLaporanBesarTanpaTarget = errors.New("kelas belum punya window laporan besar")

type LaporanBesarRepository interface {
	// Laporan (induk)
	Create(l *entity.LaporanBesar) error
	Update(l *entity.LaporanBesar) error
	Delete(id int) error
	FindByID(id int) (*entity.LaporanBesar, error)
	FindAktif() (*entity.LaporanBesar, error)
	List() ([]entity.LaporanBesar, error)

	// Target per kelas (dan opsional per shift)
	UpsertTarget(t *entity.LaporanBesarTarget) error
	ListTarget(laporanID int) ([]entity.LaporanBesarTarget, error)
	FindTarget(laporanID, kelasID int, shift *int) (*entity.LaporanBesarTarget, error)
	DeleteTarget(laporanID, kelasID int) error

	// Pengumpulan per mahasiswa
	FindPengumpulan(laporanID, mahasiswaID int) (*entity.PengumpulanLaporanBesar, error)
	UpsertPengumpulan(p *entity.PengumpulanLaporanBesar) error
	ListPengumpulan(laporanID int) ([]entity.PengumpulanLaporanBesar, error)
	ListMahasiswaByKelas(kelasIDs []int) ([]entity.User, error)
}

type laporanBesarRepository struct{ db *gorm.DB }

func NewLaporanBesarRepository(db *gorm.DB) LaporanBesarRepository {
	return &laporanBesarRepository{db: db}
}

func (r *laporanBesarRepository) Create(l *entity.LaporanBesar) error {
	return r.db.Create(l).Error
}

func (r *laporanBesarRepository) Update(l *entity.LaporanBesar) error {
	return r.db.Save(l).Error
}

func (r *laporanBesarRepository) Delete(id int) error {
	return r.db.Delete(&entity.LaporanBesar{}, id).Error
}

func (r *laporanBesarRepository) FindByID(id int) (*entity.LaporanBesar, error) {
	var l entity.LaporanBesar
	if err := r.db.First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

// FindAktif mengambil laporan besar yang sedang aktif. Desain: hanya boleh ada
// satu baris aktif. Baris terbaru menang bila data lama menyimpang.
func (r *laporanBesarRepository) FindAktif() (*entity.LaporanBesar, error) {
	var l entity.LaporanBesar
	if err := r.db.Where("is_aktif = ?", true).Order("id desc").First(&l).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *laporanBesarRepository) List() ([]entity.LaporanBesar, error) {
	var ls []entity.LaporanBesar
	return ls, r.db.Order("id desc").Find(&ls).Error
}

// UpsertTarget menyimpan window per (kelas, shift); konflik pada
// (laporan_besar_id, kelas_id, shift) akan memperbarui window yang sudah ada.
func (r *laporanBesarRepository) UpsertTarget(t *entity.LaporanBesarTarget) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "laporan_besar_id"}, {Name: "kelas_id"}, {Name: "shift"}},
		DoUpdates: clause.AssignmentColumns([]string{"shift", "buka_at", "deadline_at", "late_until"}),
	}).Create(t).Error
}

func (r *laporanBesarRepository) ListTarget(laporanID int) ([]entity.LaporanBesarTarget, error) {
	var ts []entity.LaporanBesarTarget
	return ts, r.db.Preload("Kelas").Where("laporan_besar_id = ?", laporanID).Order("kelas_id asc").Find(&ts).Error
}

// FindTarget mencari window untuk (laporan, kelas, shift). Prioritas:
// 1) baris spesifik shift kalau ada, 2) fallback ke shift NULL (berlaku semua).
func (r *laporanBesarRepository) FindTarget(laporanID, kelasID int, shift *int) (*entity.LaporanBesarTarget, error) {
	var t entity.LaporanBesarTarget
	if shift != nil {
		if err := r.db.Where("laporan_besar_id = ? AND kelas_id = ? AND shift = ?", laporanID, kelasID, *shift).First(&t).Error; err == nil {
			return &t, nil
		}
	}
	if err := r.db.Where("laporan_besar_id = ? AND kelas_id = ? AND shift IS NULL", laporanID, kelasID).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *laporanBesarRepository) DeleteTarget(laporanID, kelasID int) error {
	return r.db.Where("laporan_besar_id = ? AND kelas_id = ?", laporanID, kelasID).
		Delete(&entity.LaporanBesarTarget{}).Error
}

func (r *laporanBesarRepository) FindPengumpulan(laporanID, mahasiswaID int) (*entity.PengumpulanLaporanBesar, error) {
	var p entity.PengumpulanLaporanBesar
	if err := r.db.Where("laporan_besar_id = ? AND mahasiswa_id = ?", laporanID, mahasiswaID).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertPengumpulan membuat baris secara lazy (pertama kali aksi) atau memperbarui
// baris yang sudah ada pada konflik (laporan_besar_id, mahasiswa_id).
func (r *laporanBesarRepository) UpsertPengumpulan(p *entity.PengumpulanLaporanBesar) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "laporan_besar_id"}, {Name: "mahasiswa_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "sumber",
			"laporan_url", "laporan_path", "analisa_url", "analisa_path",
			"submitted_at", "is_late",
			"manual_oleh", "manual_pada", "manual_catatan",
			"override_oleh", "override_alasan", "override_pada",
		}),
	}).Create(p).Error
}

func (r *laporanBesarRepository) ListPengumpulan(laporanID int) ([]entity.PengumpulanLaporanBesar, error) {
	var ps []entity.PengumpulanLaporanBesar
	return ps, r.db.Where("laporan_besar_id = ?", laporanID).Find(&ps).Error
}

// ListMahasiswaByKelas mengambil seluruh mahasiswa pada daftar kelas target.
func (r *laporanBesarRepository) ListMahasiswaByKelas(kelasIDs []int) ([]entity.User, error) {
	var us []entity.User
	if len(kelasIDs) == 0 {
		return us, nil
	}
	err := r.db.Preload("Kelas").
		Where("role = ? AND kelas_id IN ?", entity.RoleMahasiswa, kelasIDs).
		Order("kelas_id asc, nim asc").
		Find(&us).Error
	return us, err
}
