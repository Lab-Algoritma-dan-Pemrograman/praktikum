package entity

// Kelas praktikum (mis. TTL A, TTL B).
type Kelas struct {
	ID        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaKelas string `gorm:"type:varchar(100);not null" json:"nama_kelas"`
}

func (Kelas) TableName() string { return "kelas" }
