package entity

import "time"

// GameSoal satu soal bug-hunt.
type GameSoal struct {
	ID         int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Bahasa     string `gorm:"type:varchar(20);not null" json:"bahasa"`
	Kesulitan  string `gorm:"type:varchar(20);not null" json:"kesulitan"`
	Judul      string `gorm:"type:varchar(200);not null" json:"judul"`
	Kode       string `gorm:"type:text;not null" json:"kode"`
	BarisBug   int    `gorm:"column:baris_bug" json:"baris_bug"`
	Penjelasan string `gorm:"type:text" json:"penjelasan"`
}

func (GameSoal) TableName() string { return "game_soal" }

// GameSoalMahasiswa = soal tanpa jawaban (baris_bug & penjelasan disembunyikan).
// Dipakai saat mahasiswa bermain; jawaban dinilai di server.
type GameSoalMahasiswa struct {
	ID        int    `json:"id"`
	Bahasa    string `json:"bahasa"`
	Kesulitan string `json:"kesulitan"`
	Judul     string `json:"judul"`
	Kode      string `json:"kode"`
}

// GameRiwayat satu kali main.
type GameRiwayat struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"not null" json:"user_id"`
	JenisGame  string    `gorm:"type:varchar(50);not null" json:"jenis_game"`
	XPDidapat  int       `gorm:"column:xp_didapat;not null;default:0" json:"xp_didapat"`
	Dimainkan  time.Time `gorm:"column:dimainkan;not null;default:now()" json:"dimainkan"`
}

func (GameRiwayat) TableName() string { return "game_riwayat" }

// GameKonfigurasi saklar & batas main. Satu baris, id='default'.
type GameKonfigurasi struct {
	ID             string `gorm:"type:varchar(32);primaryKey" json:"id"`
	BugHuntAktif   bool   `gorm:"column:bug_hunt_aktif;not null;default:true" json:"bug_hunt_aktif"`
	BugHuntCAktif  bool   `gorm:"column:bug_hunt_c_aktif;not null;default:true" json:"bug_hunt_c_aktif"`
	BugHuntPyAktif bool   `gorm:"column:bug_hunt_py_aktif;not null;default:true" json:"bug_hunt_py_aktif"`
	BatasMingguan  int    `gorm:"column:batas_mingguan;not null;default:3" json:"batas_mingguan"`
}

func (GameKonfigurasi) TableName() string { return "game_konfigurasi" }

// SesiAktif heartbeat "sedang belajar sekarang". Satu baris per user.
type SesiAktif struct {
	UserID        int       `gorm:"primaryKey" json:"user_id"`
	Aktivitas     string    `gorm:"type:varchar(50);not null;default:lesson" json:"aktivitas"`
	DetakTerakhir time.Time `gorm:"column:detak_terakhir;not null;default:now()" json:"detak_terakhir"`
}

func (SesiAktif) TableName() string { return "sesi_aktif" }
