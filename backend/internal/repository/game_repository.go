package repository

import (
	"time"

	"lab-ap/internal/entity"

	"gorm.io/gorm"
)

// BarisPeringkat satu baris papan peringkat (hasil join users + profil_belajar).
type BarisPeringkat struct {
	UserID int    `json:"user_id"`
	NIM    string `json:"nim"`
	Nama   string `json:"nama"`
	Kelas  string `json:"kelas"`
	FotoURL string `json:"foto_url"`
	XP     int    `json:"xp"`
	Level  int    `json:"level"`
	Streak int    `json:"streak"`
}

// BarisSesiAktif satu baris monitoring (join sesi_aktif + users + kelas).
type BarisSesiAktif struct {
	UserID        int       `json:"user_id"`
	NIM           string    `json:"nim"`
	Nama          string    `json:"nama"`
	Kelas         string    `json:"kelas"`
	Role          string    `json:"role"`
	Aktivitas     string    `json:"aktivitas"`
	DetakTerakhir time.Time `json:"detak_terakhir"`
}

type GameRepository interface {
	ListSoal(bahasa string) ([]entity.GameSoal, error)
	FindSoal(id int) (*entity.GameSoal, error)
	CreateSoal(s *entity.GameSoal) error
	UpdateSoal(s *entity.GameSoal) error
	DeleteSoal(id int) error

	Konfigurasi() (*entity.GameKonfigurasi, error)
	SimpanKonfigurasi(k *entity.GameKonfigurasi) error

	CatatRiwayat(r *entity.GameRiwayat) error
	HitungMainSejak(userID int, jenis string, sejak time.Time) (int64, error)

	Peringkat(limit int, kelasID *int, namaKelas string) ([]BarisPeringkat, error)
	PeringkatUser(userID int, kelasID *int, namaKelas string) (int, error)

	UpsertSesiAktif(userID int, aktivitas string) error
	ListSesiAktif(dalamDetik int) ([]BarisSesiAktif, error)
}

type gameRepository struct{ db *gorm.DB }

func NewGameRepository(db *gorm.DB) GameRepository { return &gameRepository{db: db} }

func (r *gameRepository) ListSoal(bahasa string) ([]entity.GameSoal, error) {
	var out []entity.GameSoal
	q := r.db.Order("id asc")
	if bahasa != "" {
		q = q.Where("bahasa = ?", bahasa)
	}
	return out, q.Find(&out).Error
}

func (r *gameRepository) FindSoal(id int) (*entity.GameSoal, error) {
	var s entity.GameSoal
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *gameRepository) CreateSoal(s *entity.GameSoal) error { return r.db.Create(s).Error }
func (r *gameRepository) UpdateSoal(s *entity.GameSoal) error { return r.db.Save(s).Error }
func (r *gameRepository) DeleteSoal(id int) error {
	return r.db.Delete(&entity.GameSoal{}, "id = ?", id).Error
}

func (r *gameRepository) Konfigurasi() (*entity.GameKonfigurasi, error) {
	var k entity.GameKonfigurasi
	if err := r.db.First(&k, "id = ?", "default").Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *gameRepository) SimpanKonfigurasi(k *entity.GameKonfigurasi) error {
	return r.db.Exec(`
		insert into game_konfigurasi (id, bug_hunt_aktif, bug_hunt_c_aktif, bug_hunt_py_aktif, batas_mingguan)
		values (?, ?, ?, ?, ?)
		on conflict (id) do update set
			bug_hunt_aktif = excluded.bug_hunt_aktif,
			bug_hunt_c_aktif = excluded.bug_hunt_c_aktif,
			bug_hunt_py_aktif = excluded.bug_hunt_py_aktif,
			batas_mingguan = excluded.batas_mingguan
	`, k.ID, k.BugHuntAktif, k.BugHuntCAktif, k.BugHuntPyAktif, k.BatasMingguan).Error
}

func (r *gameRepository) CatatRiwayat(g *entity.GameRiwayat) error { return r.db.Create(g).Error }

func (r *gameRepository) HitungMainSejak(userID int, jenis string, sejak time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&entity.GameRiwayat{}).
		Where("user_id = ? and jenis_game = ? and dimainkan >= ?", userID, jenis, sejak).
		Count(&n).Error
	return n, err
}

// Peringkat: hanya role mahasiswa, urut XP. Kelas ikut lewat join.
func (r *gameRepository) Peringkat(limit int, kelasID *int, namaKelas string) ([]BarisPeringkat, error) {
	var out []BarisPeringkat
	q := r.db.Table("profil_belajar pb").
		Select(`pb.user_id, u.nim, u.nama, coalesce(k.nama_kelas,'') as kelas,
		        coalesce(u.foto_url,'') as foto_url, pb.xp, pb.level_angka as level, pb.streak`).
		Joins("join users u on u.id = pb.user_id").
		Joins("left join kelas k on k.id = u.kelas_id").
		Where("u.role = ?", "mahasiswa").
		Order("pb.xp desc, u.nama asc").
		Limit(limit)
	if kelasID != nil {
		q = q.Where("u.kelas_id = ?", *kelasID)
	} else if namaKelas != "" {
		q = q.Where("k.nama_kelas = ?", namaKelas)
	}
	return out, q.Scan(&out).Error
}

// PeringkatUser mengembalikan posisi user (1 = teratas).
func (r *gameRepository) PeringkatUser(userID int, kelasID *int, namaKelas string) (int, error) {
	var xp int
	if err := r.db.Table("profil_belajar").Select("xp").
		Where("user_id = ?", userID).Scan(&xp).Error; err != nil {
		return 0, err
	}
	var n int64
	q := r.db.Table("profil_belajar pb").
		Joins("join users u on u.id = pb.user_id").
		Joins("left join kelas k on k.id = u.kelas_id").
		Where("u.role = ? and pb.xp > ?", "mahasiswa", xp)
	if kelasID != nil {
		q = q.Where("u.kelas_id = ?", *kelasID)
	} else if namaKelas != "" {
		q = q.Where("k.nama_kelas = ?", namaKelas)
	}
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return int(n) + 1, nil
}

func (r *gameRepository) UpsertSesiAktif(userID int, aktivitas string) error {
	return r.db.Exec(`
		insert into sesi_aktif (user_id, aktivitas, detak_terakhir)
		values (?, ?, now())
		on conflict (user_id) do update set
			aktivitas = excluded.aktivitas, detak_terakhir = now()
	`, userID, aktivitas).Error
}

// ListSesiAktif hanya yang berdetak dalam N detik terakhir (default dipakai 120s).
func (r *gameRepository) ListSesiAktif(dalamDetik int) ([]BarisSesiAktif, error) {
	var out []BarisSesiAktif
	batas := time.Now().Add(-time.Duration(dalamDetik) * time.Second)
	err := r.db.Table("sesi_aktif s").
		Select(`s.user_id, u.nim, u.nama, coalesce(k.nama_kelas,'') as kelas,
		        u.role, s.aktivitas, s.detak_terakhir`).
		Joins("join users u on u.id = s.user_id").
		Joins("left join kelas k on k.id = u.kelas_id").
		Where("s.detak_terakhir >= ?", batas).
		Order("s.detak_terakhir desc").
		Scan(&out).Error
	return out, err
}
