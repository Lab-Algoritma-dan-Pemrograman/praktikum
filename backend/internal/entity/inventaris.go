package entity

import "time"

// Inventaris barang lab.
type Inventaris struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Kode           string    `gorm:"type:varchar(50);not null;unique" json:"kode"`
	Nama           string    `gorm:"type:varchar(150);not null" json:"nama"`
	JumlahTotal    int       `gorm:"not null;default:1" json:"jumlah_total"`
	JumlahTersedia int       `gorm:"not null;default:1" json:"jumlah_tersedia"`
	Kondisi        string    `gorm:"type:varchar(20);not null;default:baik" json:"kondisi"`
	CreatedAt      time.Time `json:"created_at"`
}

func (Inventaris) TableName() string { return "inventaris" }
