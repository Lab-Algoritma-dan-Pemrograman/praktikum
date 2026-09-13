package dto

import "lab-ap/internal/entity"

// ---- Game ----

type GameKonfigurasiRequest struct {
	BugHuntAktif   *bool `json:"bug_hunt_aktif"`
	BugHuntCAktif  *bool `json:"bug_hunt_c_aktif"`
	BugHuntPyAktif *bool `json:"bug_hunt_py_aktif"`
	BatasMingguan  *int  `json:"batas_mingguan"`
}

type GameSoalRequest struct {
	Bahasa     string `json:"bahasa"`
	Kesulitan  string `json:"kesulitan"`
	Judul      string `json:"judul"`
	Kode       string `json:"kode"`
	BarisBug   int    `json:"baris_bug"`
	Penjelasan string `json:"penjelasan"`
}

type StatusMainResponse struct {
	Boleh    bool   `json:"boleh"`
	Terpakai int    `json:"terpakai"`
	Batas    int    `json:"batas"`
	Alasan   string `json:"alasan,omitempty"`
}

type MulaiMainResponse struct {
	Soal     []entity.GameSoalMahasiswa `json:"soal"`
	Terpakai int                        `json:"terpakai"`
	Batas    int                        `json:"batas"`
}

// JawabanGame satu jawaban: baris yang ditunjuk pemain sebagai lokasi bug.
type JawabanGame struct {
	SoalID       int `json:"soal_id" binding:"required"`
	BarisDipilih int `json:"baris_dipilih"`
}

// SelesaiMainRequest berisi JAWABAN, bukan skor. Penilaian dilakukan server.
type SelesaiMainRequest struct {
	Jawaban []JawabanGame `json:"jawaban" binding:"required"`
}

type KoreksiJawaban struct {
	SoalID     int    `json:"soal_id"`
	Benar      bool   `json:"benar"`
	BarisBug   int    `json:"baris_bug"`
	Penjelasan string `json:"penjelasan"`
}

type SelesaiMainResponse struct {
	Benar     int              `json:"benar"`
	Total     int              `json:"total"`
	XPDidapat int              `json:"xp_didapat"`
	XPTotal   int              `json:"xp_total"`
	Koreksi   []KoreksiJawaban `json:"koreksi"`
}

// ---- Monitoring ----

type DetakRequest struct {
	Aktivitas string `json:"aktivitas"`
}

// ---- Pencapaian ----

type PencapaianSayaResponse struct {
	Semua    []entity.Pencapaian `json:"semua"`
	TerbukaID []string           `json:"terbuka_id"`
	BaruSaja []entity.Pencapaian `json:"baru_saja"`
}
