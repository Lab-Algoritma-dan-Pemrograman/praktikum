package dto

// AktivasiRequest: aktifkan sesi untuk kelas + shift, sekaligus gacha pretest/posttest.
type AktivasiRequest struct {
	SesiPraktikumID int `json:"sesi_praktikum_id" binding:"required"`
	KelasID         int `json:"kelas_id" binding:"required"`
	Shift           int `json:"shift" binding:"required,oneof=1 2"`
	// Gelombang: 1 atau 2, hanya untuk sesi ujian praktik. Diabaikan untuk sesi normal.
	Gelombang *int `json:"gelombang" binding:"omitempty,oneof=1 2"`
	// GachaPilihan: "pretest" atau "posttest" (course mana yang dipakai untuk sesi normal).
	// Kosong/diabaikan untuk sesi ujian praktik.
	GachaPilihan string `json:"gacha_pilihan" binding:"omitempty,oneof=pretest posttest"`
}

// BukaTutupCourseRequest: buka/tutup course per aktivasi.
type BukaTutupCourseRequest struct {
	AktivasiCourseID int  `json:"aktivasi_course_id" binding:"required"`
	IsOpen           bool `json:"is_open"`
}

// SusulanRequest: daftarkan mahasiswa susulan ke aktivasi.
// AktivasiSesiID diambil dari path param.
type SusulanRequest struct {
	AktivasiSesiID int    `json:"aktivasi_sesi_id"`
	MahasiswaID    int    `json:"mahasiswa_id" binding:"required"`
	Alasan         string `json:"alasan"`
}
