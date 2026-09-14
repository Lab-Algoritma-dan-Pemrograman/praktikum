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

// ---- Admin belajar ----

type ResetProgresRequest struct {
	LevelID string `json:"level_id"`
}

type SetXPRequest struct {
	XP int `json:"xp"`
}

// ---- Sesi belajar ----

// SesiBelajarResponse: satu panggilan untuk menyiapkan sesi elearning.
// Menggabungkan identitas (dari token) dengan profil belajar, supaya
// frontend tidak perlu membaca tabel users sendiri.
type SesiBelajarResponse struct {
	UserID    int    `json:"user_id"`
	NIM       string `json:"nim"`
	Nama      string `json:"nama"`
	Kelas     string `json:"kelas"`
	Email     string `json:"email"`
	FotoURL   string `json:"foto_url"`
	Role      string `json:"role"`
	XP        int    `json:"xp"`
	Level     int    `json:"level"`
	Streak    int    `json:"streak"`
	AksesLevel interface{} `json:"akses_level"`
	MateriSelesai []string `json:"materi_selesai"`
}

type SetAksesLevelRequest struct {
	AksesLevel map[string]string `json:"akses_level"`
}

// ---- Simpan kurikulum penuh ----

type MateriKurikulumRequest struct {
	ID             string      `json:"id" binding:"required"`
	Judul          string      `json:"judul"`
	Penjelasan     string      `json:"penjelasan"`
	ContohKode     string      `json:"contoh_kode"`
	KodeAwal       string      `json:"kode_awal"`
	Solusi         string      `json:"solusi"`
	Petunjuk       string      `json:"petunjuk"`
	Kuis           interface{} `json:"kuis"`
	KasusUji       interface{} `json:"kasus_uji"`
	AturanValidasi interface{} `json:"aturan_validasi"`
	XPHadiah       int         `json:"xp_hadiah"`
}

type ModulKurikulumRequest struct {
	ID     string                   `json:"id" binding:"required"`
	Judul  string                   `json:"judul"`
	Materi []MateriKurikulumRequest `json:"materi"`
}

type LevelKurikulumRequest struct {
	ID        string                  `json:"id" binding:"required"`
	Judul     string                  `json:"judul"`
	Deskripsi string                  `json:"deskripsi"`
	ModeAkses string                  `json:"mode_akses"`
	Terkunci  bool                    `json:"terkunci"`
	Modul     []ModulKurikulumRequest `json:"modul"`
}

type SimpanKurikulumRequest struct {
	Level []LevelKurikulumRequest `json:"level" binding:"required"`
}
