package entity

import "time"

// Soal: pool soal milik sebuah course (per modul).
type Soal struct {
	ID            int            `gorm:"primaryKey;autoIncrement" json:"id"`
	CourseID      int            `gorm:"not null;index" json:"course_id"`
	JenisSoal     JenisSoal      `gorm:"type:varchar(10);not null" json:"jenis_soal"`
	Difficulty    *Difficulty    `gorm:"type:varchar(10)" json:"difficulty"`
	KategoriUjian *KategoriUjian `gorm:"type:varchar(15)" json:"kategori_ujian"`
	TeksSoal      string         `gorm:"type:text;not null" json:"teks_soal"`
	GambarURL     *string        `gorm:"type:varchar(500)" json:"gambar_url"`
	Poin          float64        `gorm:"not null;default:0" json:"poin"`
	KunciJawaban  *string        `gorm:"type:text" json:"kunci_jawaban"`
	// KodeRumpang: kerangka kode ber-blank __ / ____ untuk soal coding
	// keterampilan kelas karyawan. Tampil sebagai jawaban awal (prefill)
	// di editor mahasiswa; NULL = tanpa rumpang (editor kosongan).
	KodeRumpang *string `gorm:"type:text" json:"kode_rumpang,omitempty"`
	IsArsip       bool           `gorm:"column:is_arsip;not null;default:false" json:"is_arsip"`
	CreatedAt     time.Time      `json:"created_at"`
}

func (Soal) TableName() string { return "soal" }
