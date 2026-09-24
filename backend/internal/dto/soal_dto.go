package dto

// SoalRequest: buat/ubah soal dalam pool course.
type SoalRequest struct {
	CourseID      int     `json:"course_id" binding:"required"`
	JenisSoal     string  `json:"jenis_soal" binding:"required,oneof=essay coding"`
	Difficulty    *string `json:"difficulty" binding:"omitempty,oneof=easy medium hard"`
	KategoriUjian *string `json:"kategori_ujian" binding:"omitempty,oneof=modul_1 modul_2 modul_3 modul_4_5 modul_6 flowchart"`
	TeksSoal      string  `json:"teks_soal" binding:"required"`
	GambarURL     *string `json:"gambar_url"`
	Poin          float64 `json:"poin" binding:"min=0"`
	KunciJawaban  *string `json:"kunci_jawaban"`
}
