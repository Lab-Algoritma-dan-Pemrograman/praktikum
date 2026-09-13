package entity

import (
	"time"

	"gorm.io/datatypes"
)

// Level kurikulum elearning (mis. "c-level-1"). ID text, dipertahankan dari sumber.
type Level struct {
	ID        string `gorm:"type:varchar(64);primaryKey" json:"id"`
	Judul     string `gorm:"type:varchar(200);not null" json:"judul"`
	Deskripsi string `gorm:"type:text" json:"deskripsi"`
	ModeAkses string `gorm:"type:varchar(20);not null;default:auto" json:"mode_akses"`
	Terkunci  bool   `gorm:"not null;default:false" json:"terkunci"`
	Urutan    int    `gorm:"not null;default:0" json:"urutan"`
}

func (Level) TableName() string { return "level" }

// Modul di dalam satu level (mis. "c-level-1-m1").
type Modul struct {
	ID      string `gorm:"type:varchar(64);primaryKey" json:"id"`
	LevelID string `gorm:"type:varchar(64);not null" json:"level_id"`
	Judul   string `gorm:"type:varchar(200);not null" json:"judul"`
	Urutan  int    `gorm:"not null;default:0" json:"urutan"`
}

func (Modul) TableName() string { return "modul" }

// Materi satu pelajaran. Kolom Solusi adalah kunci jawaban:
// JANGAN kirim ke mahasiswa -- pakai MateriMahasiswa (view materi_mahasiswa).
type Materi struct {
	ID             string         `gorm:"type:varchar(64);primaryKey" json:"id"`
	ModulID        string         `gorm:"type:varchar(64);not null" json:"modul_id"`
	Judul          string         `gorm:"type:varchar(200);not null" json:"judul"`
	Penjelasan     string         `gorm:"type:text" json:"penjelasan"`
	ContohKode     string         `gorm:"type:text" json:"contoh_kode"`
	KodeAwal       string         `gorm:"type:text" json:"kode_awal"`
	Solusi         string         `gorm:"type:text" json:"solusi,omitempty"`
	Petunjuk       string         `gorm:"type:text" json:"petunjuk"`
	Kuis           datatypes.JSON `gorm:"type:jsonb" json:"kuis"`
	KasusUji       datatypes.JSON `gorm:"type:jsonb" json:"kasus_uji"`
	AturanValidasi datatypes.JSON `gorm:"type:jsonb" json:"aturan_validasi"`
	Urutan         int            `gorm:"not null;default:0" json:"urutan"`
	XPHadiah       int            `gorm:"not null;default:0" json:"xp_hadiah"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (Materi) TableName() string { return "materi" }

// MateriMahasiswa = view materi_mahasiswa, tanpa kolom Solusi.
// Dipakai untuk semua endpoint yang diakses mahasiswa.
type MateriMahasiswa struct {
	ID             string         `gorm:"type:varchar(64);primaryKey" json:"id"`
	ModulID        string         `gorm:"type:varchar(64)" json:"modul_id"`
	Judul          string         `gorm:"type:varchar(200)" json:"judul"`
	Penjelasan     string         `gorm:"type:text" json:"penjelasan"`
	ContohKode     string         `gorm:"type:text" json:"contoh_kode"`
	KodeAwal       string         `gorm:"type:text" json:"kode_awal"`
	Petunjuk       string         `gorm:"type:text" json:"petunjuk"`
	Kuis           datatypes.JSON `gorm:"type:jsonb" json:"kuis"`
	KasusUji       datatypes.JSON `gorm:"type:jsonb" json:"kasus_uji"`
	AturanValidasi datatypes.JSON `gorm:"type:jsonb" json:"aturan_validasi"`
	Urutan         int            `json:"urutan"`
	XPHadiah       int            `json:"xp_hadiah"`
}

func (MateriMahasiswa) TableName() string { return "materi_mahasiswa" }
