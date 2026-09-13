package entity

import (
	"time"

	"gorm.io/datatypes"
)

// ProfilBelajar menyimpan xp/level/streak elearning. 1:1 dengan users,
// dipisah agar tabel identitas tetap bersih.
type ProfilBelajar struct {
	UserID        int            `gorm:"primaryKey" json:"user_id"`
	XP            int            `gorm:"not null;default:0" json:"xp"`
	LevelAngka    int            `gorm:"not null;default:1" json:"level_angka"`
	Streak        int            `gorm:"not null;default:0" json:"streak"`
	WaktuBelajar  int            `gorm:"not null;default:0" json:"waktu_belajar"`
	TerakhirAktif *time.Time     `json:"terakhir_aktif"`
	AksesLevel    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"akses_level"`
	AksesAsesmen  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"akses_asesmen"`
}

func (ProfilBelajar) TableName() string { return "profil_belajar" }

// ProgresBelajar satu materi yang diselesaikan seorang user.
type ProgresBelajar struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int        `gorm:"not null" json:"user_id"`
	MateriID    string     `gorm:"type:varchar(64);not null" json:"materi_id"`
	Selesai     bool       `gorm:"not null;default:false" json:"selesai"`
	SelesaiPada *time.Time `json:"selesai_pada"`
}

func (ProgresBelajar) TableName() string { return "progres_belajar" }

// Pencapaian (achievement). SyaratNilai bertipe string karena sumbernya
// campur: angka ("1000") untuk xp/streak, id level ("c-level-1") untuk level_completed.
type Pencapaian struct {
	ID          string `gorm:"type:varchar(64);primaryKey" json:"id"`
	Judul       string `gorm:"type:varchar(150);not null" json:"judul"`
	Deskripsi   string `gorm:"type:text" json:"deskripsi"`
	Ikon        string `gorm:"type:varchar(100)" json:"ikon"`
	SyaratTipe  string `gorm:"type:varchar(50)" json:"syarat_tipe"`
	SyaratNilai string `gorm:"type:varchar(64)" json:"syarat_nilai"`
}

func (Pencapaian) TableName() string { return "pencapaian" }

// PencapaianTerbuka = pencapaian yang sudah diraih user.
type PencapaianTerbuka struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int       `gorm:"not null" json:"user_id"`
	PencapaianID string    `gorm:"type:varchar(64);not null" json:"pencapaian_id"`
	DibukaPada   time.Time `gorm:"not null;default:now()" json:"dibuka_pada"`
}

func (PencapaianTerbuka) TableName() string { return "pencapaian_terbuka" }

// Syarat pencapaian yang sah.
const (
	SyaratXP             = "xp"
	SyaratStreak         = "streak"
	SyaratJumlahMateri   = "lesson_count"
	SyaratLevelSelesai   = "level_completed"
	SyaratSkorGame       = "game_score"
)
