package repository

import (
	"lab-ap/internal/entity"

	"gorm.io/datatypes"
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
	HapusProgres(userID int, materiIDs []string) (int64, error)
	HapusSemuaProgres(userID int) error
	HapusPencapaianTerbuka(userID int) error
	TotalXPGame(userID int) (int, error)
	TotalXPMateriSelesai(userID int) (int, error)
	SesiBelajar(userID int) (*BarisSesiBelajar, error)
	ListUserBelajar() ([]BarisSesiBelajar, error)
	SetAksesLevel(userID int, akses []byte) error
	SimpanKurikulum(levels []entity.Level, moduls []entity.Modul, materi []entity.Materi) error
	KosongkanKurikulum() error
	TandaiSelesai(p *entity.ProgresBelajar) error
	FindProfil(userID int) (*entity.ProfilBelajar, error)
	SimpanProfil(p *entity.ProfilBelajar) error
}

// BarisSesiBelajar hasil join users + kelas + profil_belajar.
type BarisSesiBelajar struct {
	UserID     int    `json:"user_id"`
	NIM        string `json:"nim"`
	Nama       string `json:"nama"`
	Kelas      string `json:"kelas"`
	Email      string `json:"email"`
	FotoURL    string `json:"foto_url"`
	Role       string `json:"role"`
	XP         int    `json:"xp"`
	Level      int    `json:"level"`
	Streak     int    `json:"streak"`
	AksesLevel []byte `json:"akses_level"`
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

// HapusProgres menghapus progres untuk materi tertentu, kembalikan jumlah baris terhapus.
func (r *belajarRepository) HapusProgres(userID int, materiIDs []string) (int64, error) {
	if len(materiIDs) == 0 {
		return 0, nil
	}
	res := r.db.Where("user_id = ? and materi_id in ?", userID, materiIDs).
		Delete(&entity.ProgresBelajar{})
	return res.RowsAffected, res.Error
}

func (r *belajarRepository) HapusSemuaProgres(userID int) error {
	return r.db.Where("user_id = ?", userID).Delete(&entity.ProgresBelajar{}).Error
}

func (r *belajarRepository) HapusPencapaianTerbuka(userID int) error {
	return r.db.Where("user_id = ?", userID).Delete(&entity.PencapaianTerbuka{}).Error
}

// SesiBelajar mengambil identitas + profil belajar sekali jalan.
// profil_belajar dibuat otomatis kalau belum ada, supaya mahasiswa baru
// tidak perlu langkah pendaftaran terpisah.
func (r *belajarRepository) SesiBelajar(userID int) (*BarisSesiBelajar, error) {
	if err := r.db.Exec(`
		insert into profil_belajar (user_id) values (?)
		on conflict (user_id) do nothing
	`, userID).Error; err != nil {
		return nil, err
	}
	var out BarisSesiBelajar
	err := r.db.Table("users u").
		Select(`u.id as user_id, u.nim, u.nama, coalesce(k.nama_kelas,'') as kelas,
		        coalesce(u.email,'') as email, coalesce(u.foto_url,'') as foto_url, u.role,
		        pb.xp, pb.level_angka as level, pb.streak, pb.akses_level`).
		Joins("left join kelas k on k.id = u.kelas_id").
		Joins("join profil_belajar pb on pb.user_id = u.id").
		Where("u.id = ?", userID).
		Take(&out).Error
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SimpanKurikulum menulis level, modul, dan materi dalam satu transaksi.
// Dipakai layar admin saat menyimpan struktur kurikulum sekaligus.
func (r *belajarRepository) SimpanKurikulum(levels []entity.Level, moduls []entity.Modul, materi []entity.Materi) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range levels {
			if err := tx.Save(&levels[i]).Error; err != nil {
				return err
			}
		}
		for i := range moduls {
			if err := tx.Save(&moduls[i]).Error; err != nil {
				return err
			}
		}
		for i := range materi {
			if err := tx.Save(&materi[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// KosongkanKurikulum menghapus seluruh materi, modul, dan level.
// Progres mahasiswa ikut terhapus lewat FK cascade, jadi ini memang destruktif.
func (r *belajarRepository) KosongkanKurikulum() error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("delete from materi").Error; err != nil {
			return err
		}
		if err := tx.Exec("delete from modul").Error; err != nil {
			return err
		}
		return tx.Exec("delete from level").Error
	})
}

// ListUserBelajar daftar mahasiswa + profil belajar untuk layar admin.
func (r *belajarRepository) ListUserBelajar() ([]BarisSesiBelajar, error) {
	var out []BarisSesiBelajar
	err := r.db.Table("users u").
		Select(`u.id as user_id, u.nim, u.nama, coalesce(k.nama_kelas,'') as kelas,
		        coalesce(u.email,'') as email, coalesce(u.foto_url,'') as foto_url, u.role,
		        coalesce(pb.xp,0) as xp, coalesce(pb.level_angka,1) as level,
		        coalesce(pb.streak,0) as streak, pb.akses_level`).
		Joins("left join kelas k on k.id = u.kelas_id").
		Joins("left join profil_belajar pb on pb.user_id = u.id").
		Where("u.role = ?", "mahasiswa").
		Order("coalesce(pb.xp,0) desc").
		Find(&out).Error
	return out, err
}

// SetAksesLevel menyetel buka/tutup level per mahasiswa.
func (r *belajarRepository) SetAksesLevel(userID int, akses []byte) error {
	if err := r.db.Exec(`
		insert into profil_belajar (user_id) values (?)
		on conflict (user_id) do nothing
	`, userID).Error; err != nil {
		return err
	}
	return r.db.Model(&entity.ProfilBelajar{}).
		Where("user_id = ?", userID).
		Update("akses_level", datatypes.JSON(akses)).Error
}

// TotalXPMateriSelesai menjumlahkan xp_hadiah dari materi yang sudah selesai.
func (r *belajarRepository) TotalXPMateriSelesai(userID int) (int, error) {
	var total *int
	err := r.db.Table("progres_belajar pb").
		Select("sum(m.xp_hadiah)").
		Joins("join materi m on m.id = pb.materi_id").
		Where("pb.user_id = ? and pb.selesai = true", userID).
		Scan(&total).Error
	if err != nil || total == nil {
		return 0, err
	}
	return *total, nil
}

// TotalXPGame menjumlahkan XP dari seluruh riwayat main.
func (r *belajarRepository) TotalXPGame(userID int) (int, error) {
	var total *int
	err := r.db.Table("game_riwayat").Select("sum(xp_didapat)").
		Where("user_id = ?", userID).Scan(&total).Error
	if err != nil || total == nil {
		return 0, err
	}
	return *total, nil
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
