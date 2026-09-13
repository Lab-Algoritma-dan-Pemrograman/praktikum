package entity

import "time"

// Absensi kehadiran pada sesi praktikum. Cocok dgn 004_domain.sql.
type Absensi struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int       `gorm:"not null" json:"user_id"`
	SesiID      *int      `json:"sesi_id"`
	KelasID     *int      `json:"kelas_id"`
	Status      string    `gorm:"type:varchar(12);not null;default:hadir" json:"status"`
	Waktu       time.Time `gorm:"not null;default:now()" json:"waktu"`
	DicatatOleh *int      `json:"dicatat_oleh"`
	Catatan     string    `gorm:"type:varchar(255)" json:"catatan"`
}

func (Absensi) TableName() string { return "absensi" }

// Status sah, cocok CHECK absensi_status_check.
const (
	AbsensiHadir = "hadir"
	AbsensiIzin  = "izin"
	AbsensiSakit = "sakit"
	AbsensiAlfa  = "alfa"
)

func AbsensiStatusValid(s string) bool {
	switch s {
	case AbsensiHadir, AbsensiIzin, AbsensiSakit, AbsensiAlfa:
		return true
	}
	return false
}
