package dto

import (
	"lab-ap/internal/entity"

	"gorm.io/datatypes"
)

// MateriRequest payload buat/ubah materi (khusus asisten & koordinator).
type MateriRequest struct {
	ID             string         `json:"id" binding:"required"`
	ModulID        string         `json:"modul_id" binding:"required"`
	Judul          string         `json:"judul" binding:"required"`
	Penjelasan     string         `json:"penjelasan"`
	ContohKode     string         `json:"contoh_kode"`
	KodeAwal       string         `json:"kode_awal"`
	Solusi         string         `json:"solusi"`
	Petunjuk       string         `json:"petunjuk"`
	Kuis           datatypes.JSON `json:"kuis"`
	KasusUji       datatypes.JSON `json:"kasus_uji"`
	AturanValidasi datatypes.JSON `json:"aturan_validasi"`
	Urutan         int            `json:"urutan"`
	XPHadiah       int            `json:"xp_hadiah"`
}

// ProgresSayaResponse progres belajar + profil xp/level milik user sendiri.
type ProgresSayaResponse struct {
	Progres []entity.ProgresBelajar `json:"progres"`
	Profil  *entity.ProfilBelajar   `json:"profil"`
}
