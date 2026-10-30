package entity

import "time"

// StatusPengumpulanLaporan: status akhir pengumpulan laporan besar per mahasiswa.
type StatusPengumpulanLaporan string

const (
	LaporanBelum     StatusPengumpulanLaporan = "belum"
	LaporanTerkumpul StatusPengumpulanLaporan = "terkumpul"
)

// SumberPengumpulanLaporan: asal pengumpulan berkas.
type SumberPengumpulanLaporan string

const (
	LaporanSumberWeb    SumberPengumpulanLaporan = "web"    // mahasiswa unggah sendiri
	LaporanSumberManual SumberPengumpulanLaporan = "manual" // dicentang asisten (MS Teams)
)

// LaporanBesar: tugas laporan akhir praktikum. Berdiri sendiri, TIDAK nempel ke
// sesi/modul. Per-kelas punya window sendiri (lihat LaporanBesarTarget).
type LaporanBesar struct {
	ID           int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Judul        string     `gorm:"type:varchar(200);not null;default:'Laporan Besar'" json:"judul"`
	Deskripsi    string     `gorm:"type:text" json:"deskripsi"`
	IsAktif      bool       `gorm:"default:true" json:"is_aktif"`
	ButuhLaporan bool       `gorm:"default:true" json:"butuh_laporan"`
	ButuhAnalisa bool       `gorm:"default:true" json:"butuh_analisa"`
	CreatedBy    *int       `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
}

func (LaporanBesar) TableName() string { return "laporan_besar" }

// LaporanBesarTarget: window pengumpulan per kelas (dan opsional per shift).
// Buka_at NULL = langsung dibuka; late_until NULL = tanpa window telat.
// Shift NULL = berlaku untuk SEMUA shift; shift 1/2 = spesifik per shift.
type LaporanBesarTarget struct {
	ID             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	LaporanBesarID int        `gorm:"not null;uniqueIndex:uq_lapbes_target" json:"laporan_besar_id"`
	KelasID        int        `gorm:"not null;uniqueIndex:uq_lapbes_target" json:"kelas_id"`
	Shift          *int       `gorm:"uniqueIndex:uq_lapbes_target" json:"shift"`
	BukaAt         *time.Time `json:"buka_at"`
	DeadlineAt     time.Time  `gorm:"not null" json:"deadline_at"`
	LateUntil      *time.Time `json:"late_until"`

	Kelas *Kelas `gorm:"foreignKey:KelasID" json:"kelas,omitempty"`
}

func (LaporanBesarTarget) TableName() string { return "laporan_besar_target" }

// PengumpulanLaporanBesar: baris pengumpulan per (laporan, mahasiswa).
// Dibuat secara lazy (upsert). Tidak ada baris = dianggap belum.
type PengumpulanLaporanBesar struct {
	ID             int     `gorm:"primaryKey;autoIncrement" json:"id"`
	LaporanBesarID int     `gorm:"not null;uniqueIndex:uq_plb" json:"laporan_besar_id"`
	MahasiswaID    int     `gorm:"not null;uniqueIndex:uq_plb" json:"mahasiswa_id"`
	Status         string  `gorm:"type:varchar(20);not null;default:'belum'" json:"status"`
	Sumber         *string `gorm:"type:varchar(10)" json:"sumber"`

	LaporanURL  *string `gorm:"type:varchar(500)" json:"laporan_url"`
	LaporanPath *string `gorm:"type:varchar(300)" json:"laporan_path"`
	AnalisaURL  *string `gorm:"type:varchar(500)" json:"analisa_url"`
	AnalisaPath *string `gorm:"type:varchar(300)" json:"analisa_path"`

	SubmittedAt *time.Time `json:"submitted_at"`
	IsLate      bool       `gorm:"default:false" json:"is_late"`

	ManualOleh    *int       `json:"manual_oleh"`
	ManualPada    *time.Time `json:"manual_pada"`
	ManualCatatan *string    `gorm:"type:varchar(300)" json:"manual_catatan"`

	OverrideOleh   *int       `json:"override_oleh"`
	OverrideAlasan *string    `gorm:"type:varchar(500)" json:"override_alasan"`
	OverridePada   *time.Time `json:"override_pada"`
}

func (PengumpulanLaporanBesar) TableName() string { return "pengumpulan_laporan_besar" }
