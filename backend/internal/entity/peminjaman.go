package entity

import "time"

// Peminjaman barang lab oleh user (termasuk role 'peminjam').
type Peminjaman struct {
	ID            int        `gorm:"primaryKey;autoIncrement" json:"id"`
	InventarisID  int        `gorm:"not null" json:"inventaris_id"`
	PeminjamID    int        `gorm:"not null" json:"peminjam_id"`
	Jumlah        int        `gorm:"not null;default:1" json:"jumlah"`
	Status        string     `gorm:"type:varchar(12);not null;default:diajukan" json:"status"`
	TglPinjam     time.Time  `gorm:"not null;default:now()" json:"tgl_pinjam"`
	TglKembali    *time.Time `json:"tgl_kembali"`
	DisetujuiOleh *int       `json:"disetujui_oleh"`
	Catatan       string     `gorm:"type:varchar(255)" json:"catatan"`
}

func (Peminjaman) TableName() string { return "peminjaman" }

// Status sah, cocok CHECK peminjaman_status_check di 004.
const (
	PinjamDiajukan = "diajukan"
	PinjamDipinjam = "dipinjam"
	PinjamKembali  = "kembali"
	PinjamDitolak  = "ditolak"
)

func PeminjamanStatusValid(s string) bool {
	switch s {
	case PinjamDiajukan, PinjamDipinjam, PinjamKembali, PinjamDitolak:
		return true
	}
	return false
}
