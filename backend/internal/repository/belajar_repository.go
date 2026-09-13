package repository

import (
	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

// BelajarRepository melayani kurikulum elearning: level, modul, materi, progres.
type BelajarRepository interface {
	ListLevel() ([]entity.Level, error)
	ListModul(levelID string) ([]entity.Modul, error)

	// ListMateriMahasiswa memakai view materi_mahasiswa (TANPA kolom solusi).
	ListMateriMahasiswa(modulID string) ([]entity.MateriMahasiswa, error)
	FindMateriMahasiswa(id string) (*entity.MateriMahasiswa, error)

	// ListMateri / FindMateri mengembalikan kunci jawaban. Khusus asisten/koordinator.
	ListMateri(modulID string) ([]entity.Materi, error)
	FindMateri(id string) (*entity.Materi, error)
	CreateMateri(m *entity.Materi) error
	UpdateMateri(m *entity.Materi) error
	DeleteMateri(id string) error

	ListPencapaian() ([]entity.Pencapaian, error)
	ListPencapaianTerbuka(userID int) ([]entity.PencapaianTerbuka, error)
	BukaPencapaian(userID int, pencapaianID string) error
	HitungMateriSelesai(userID int) (int64, error)

	ListProgres(userID int) ([]entity.ProgresBelajar, error)
	TandaiSelesai(p *entity.ProgresBelajar) error
	FindProfil(userID int) (*entity.ProfilBelajar, error)
	SimpanProfil(p *entity.ProfilBelajar) error
}

type belajarRepository struct{ db *gorm.DB }

func NewBelajarRepository(db *gorm.DB) BelajarRepository { return &belajarRepository{db: db} }

func (r *belajarRepository) ListLevel() ([]entity.Level, error) {
	var out []entity.Level
	return out, r.db.Order("urutan asc, id asc").Find(&out).Error
}

func (r *belajarRepository) ListModul(levelID string) ([]entity.Modul, error) {
	var out []entity.Modul
	q := r.db.Order("urutan asc, id asc")
	if levelID != "" {
		q = q.Where("level_id = ?", levelID)
	}
	return out, q.Find(&out).Error
}

func (r *belajarRepository) ListMateriMahasiswa(modulID string) ([]entity.MateriMahasiswa, error) {
	var out []entity.MateriMahasiswa
	q := r.db.Order("urutan asc, id asc")
	if modulID != "" {
		q = q.Where("modul_id = ?", modulID)
	}
	return out, q.Find(&out).Error
}

func (r *belajarRepository) FindMateriMahasiswa(id string) (*entity.MateriMahasiswa, error) {
	var m entity.MateriMahasiswa
	if err := r.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *belajarRepository) ListMateri(modulID string) ([]entity.Materi, error) {
	var out []entity.Materi
	q := r.db.Order("urutan asc, id asc")
	if modulID != "" {
		q = q.Where("modul_id = ?", modulID)
	}
	return out, q.Find(&out).Error
}

func (r *belajarRepository) FindMateri(id string) (*entity.Materi, error) {
	var m entity.Materi
	if err := r.db.First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *belajarRepository) CreateMateri(m *entity.Materi) error { return r.db.Create(m).Error }
func (r *belajarRepository) UpdateMateri(m *entity.Materi) error { return r.db.Save(m).Error }

func (r *belajarRepository) DeleteMateri(id string) error {
	return r.db.Delete(&entity.Materi{}, "id = ?", id).Error
}

func (r *belajarRepository) ListPencapaian() ([]entity.Pencapaian, error) {
	var out []entity.Pencapaian
	return out, r.db.Order("id asc").Find(&out).Error
}

func (r *belajarRepository) ListPencapaianTerbuka(userID int) ([]entity.PencapaianTerbuka, error) {
	var out []entity.PencapaianTerbuka
	return out, r.db.Where("user_id = ?", userID).Find(&out).Error
}

// BukaPencapaian idempoten: unique (user_id, pencapaian_id).
func (r *belajarRepository) BukaPencapaian(userID int, pencapaianID string) error {
	return r.db.Exec(`
		insert into pencapaian_terbuka (user_id, pencapaian_id, dibuka_pada)
		values (?, ?, now())
		on conflict (user_id, pencapaian_id) do nothing
	`, userID, pencapaianID).Error
}

func (r *belajarRepository) HitungMateriSelesai(userID int) (int64, error) {
	var n int64
	err := r.db.Model(&entity.ProgresBelajar{}).
		Where("user_id = ? and selesai = true", userID).Count(&n).Error
	return n, err
}

func (r *belajarRepository) ListProgres(userID int) ([]entity.ProgresBelajar, error) {
	var out []entity.ProgresBelajar
	return out, r.db.Where("user_id = ?", userID).Find(&out).Error
}

// TandaiSelesai idempoten: unique (user_id, materi_id) -> upsert.
func (r *belajarRepository) TandaiSelesai(p *entity.ProgresBelajar) error {
	return r.db.Exec(`
		insert into progres_belajar (user_id, materi_id, selesai, selesai_pada)
		values (?, ?, ?, ?)
		on conflict (user_id, materi_id)
		do update set selesai = excluded.selesai, selesai_pada = excluded.selesai_pada
	`, p.UserID, p.MateriID, p.Selesai, p.SelesaiPada).Error
}

func (r *belajarRepository) FindProfil(userID int) (*entity.ProfilBelajar, error) {
	var p entity.ProfilBelajar
	if err := r.db.First(&p, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *belajarRepository) SimpanProfil(p *entity.ProfilBelajar) error {
	return r.db.Exec(`
		insert into profil_belajar (user_id, xp, level_angka, streak, waktu_belajar, terakhir_aktif)
		values (?, ?, ?, ?, ?, ?)
		on conflict (user_id) do update set
			xp = excluded.xp, level_angka = excluded.level_angka,
			streak = excluded.streak, waktu_belajar = excluded.waktu_belajar,
			terakhir_aktif = excluded.terakhir_aktif
	`, p.UserID, p.XP, p.LevelAngka, p.Streak, p.WaktuBelajar, p.TerakhirAktif).Error
}
