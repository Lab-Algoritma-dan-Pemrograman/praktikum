package dto

// === User ===

// UserRequest: tambah/ubah data mahasiswa oleh admin.
type UserRequest struct {
	NIM       string  `json:"nim" binding:"required"`
	Nama      string  `json:"nama" binding:"required"`
	KelasID   *int    `json:"kelas_id"`
	Shift     *int    `json:"shift" binding:"omitempty,oneof=1 2"`
	Gelombang *int    `json:"gelombang" binding:"omitempty,oneof=1 2"`
	Kelompok  *string `json:"kelompok"`
}

// UserBulkRequest: payload untuk import mahasiswa secara masal via CSV (Frontend JSON).
type UserBulkRequest struct {
	Users []UserRequest `json:"users" binding:"required,dive"`
}

// UserBulkAttrsRequest: ubah atribut massal (gelombang/shift/kelas/kelompok) by NIM.
// Pointer nil = field tidak diubah; non-nil = set nilai (pakai SetGelombangNull untuk NULL).
type UserBulkAttrsRequest struct {
	NIMs              []string `json:"nims" binding:"required,min=1,dive,required"`
	Gelombang         *int     `json:"gelombang" binding:"omitempty,oneof=1 2"`
	SetGelombangNull  bool     `json:"set_gelombang_null"`
	Shift             *int     `json:"shift" binding:"omitempty,oneof=1 2"`
	KelasID           *int     `json:"kelas_id"`
	SetKelasNull      bool     `json:"set_kelas_null"`
	Kelompok          *string  `json:"kelompok"`
}

// BulkResponse: hasil dari operasi import/upsert masal.
type BulkResponse struct {
	TotalProcessed int `json:"total_processed"`
	TotalInserted  int `json:"total_inserted"`
	TotalUpdated   int `json:"total_updated"`
}

// ResetPasswordRequest: admin reset password mahasiswa (kosongkan & set belum register).
type ResetPasswordRequest struct {
	UserID int `json:"user_id" binding:"required"`
}

// AsistenRequest: tambah/ubah data asisten (role admin).
type AsistenRequest struct {
	NIM        string  `json:"nim" binding:"required"`
	Nama       string  `json:"nama" binding:"required"`
	NomorHP    *string `json:"nomor_hp"`
	MedsosLink *string `json:"medsos_link"`
	FotoURL    *string `json:"foto_url"`
	Password   *string `json:"password" binding:"omitempty,min=6"`
}

// === Kelas ===

type KelasRequest struct {
	NamaKelas string `json:"nama_kelas" binding:"required"`
}

// === Jadwal ===

type JadwalRequest struct {
	KelasID    int    `json:"kelas_id" binding:"required"`
	Shift      int    `json:"shift" binding:"required,oneof=1 2"`
	Hari       string `json:"hari" binding:"required"`
	JamMulai   string `json:"jam_mulai" binding:"required"`
	JamSelesai string `json:"jam_selesai" binding:"required"`
	Keterangan string `json:"keterangan"`
}

// === Pedoman ===

type PedomanRequest struct {
	NamaDokumen string `json:"nama_dokumen" binding:"required"`
	FileURL     string `json:"file_url" binding:"required"`
}
