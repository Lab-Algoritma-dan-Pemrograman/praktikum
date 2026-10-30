package dto

import "time"

// === Laporan Besar (admin) ===

// LaporanBesarRequest: buat/ubah tugas laporan besar.
type LaporanBesarRequest struct {
	Judul        string `json:"judul" binding:"omitempty,max=200"`
	Deskripsi    string `json:"deskripsi"`
	IsAktif      *bool  `json:"is_aktif"`
	ButuhLaporan *bool  `json:"butuh_laporan"`
	ButuhAnalisa *bool  `json:"butuh_analisa"`
}

// LaporanBesarTargetRequest: satu window per kelas + shift (opsional).
type LaporanBesarTargetRequest struct {
	KelasID    int        `json:"kelas_id" binding:"required"`
	Shift      *int       `json:"shift" binding:"omitempty,oneof=1 2"`
	BukaAt     *time.Time `json:"buka_at"`
	DeadlineAt time.Time  `json:"deadline_at" binding:"required"`
	LateUntil  *time.Time `json:"late_until"`
}

// SetTargetRequest: payload set/ubah seluruh window kelas (replace).
type SetTargetRequest struct {
	Targets []LaporanBesarTargetRequest `json:"targets" binding:"required,dive"`
}

// === Pengumpulan berkas (mahasiswa) ===

// UploadURLRequest: minta signed URL unggah berkas laporan besar.
type UploadURLRequest struct {
	Jenis string `json:"jenis" binding:"required,oneof=laporan analisa"`
	Ext   string `json:"ext" binding:"required,oneof=pdf docx"`
}

// UploadURLResponse: URL unggah + path yang harus dicatat mahasiswa.
type UploadURLResponse struct {
	UploadURL string `json:"upload_url"`
	Path      string `json:"path"`
}

// KumpulRequest: mahasiswa menandai berkas laporan besar sudah diunggah.
type KumpulRequest struct {
	LaporanPath *string `json:"laporan_path"`
	AnalisaPath *string `json:"analisa_path"`
}

// === Centang manual / override (admin) ===

// MarkRequest: asisten mencentang manual pengumpulan (via MS Teams).
type MarkRequest struct {
	Status  string `json:"status" binding:"required,oneof=belum terkumpul"`
	Catatan string `json:"catatan" binding:"max=300"`
}

// OverrideRequest: buka blokir gerbang ujian untuk mahasiswa tertentu.
type OverrideRequest struct {
	Alasan string `json:"alasan" binding:"required,max=500"`
}

// === Download batch (admin) ===

// DownloadBatchRequest: minta signed URL banyak berkas sekaligus (tanpa zip di server).
type DownloadBatchRequest struct {
	Items []DownloadBatchItem `json:"items" binding:"required,dive"`
}

type DownloadBatchItem struct {
	MhsID int    `json:"mhs_id" binding:"required"`
	Jenis string `json:"jenis" binding:"required,oneof=laporan analisa"`
}

// DownloadBatchResult: satu entri hasil signed URL.
type DownloadBatchResult struct {
	MhsID    int    `json:"mhs_id"`
	Jenis    string `json:"jenis"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// === Response ===

// LaporanBesarStatusResponse: status laporan besar milik mahasiswa.
type LaporanBesarStatusResponse struct {
	LaporanBesarID int    `json:"laporan_besar_id"`
	Judul          string `json:"judul"`
	Deskripsi      string `json:"deskripsi"`
	ButuhLaporan   bool   `json:"butuh_laporan"`
	ButuhAnalisa   bool   `json:"butuh_analisa"`

	KelasID    *int       `json:"kelas_id"`
	BukaAt     *time.Time `json:"buka_at"`
	DeadlineAt *time.Time `json:"deadline_at"`
	LateUntil  *time.Time `json:"late_until"`

	// Fase window: "belum_dibuka" | "dibuka" | "telat" | "ditutup". Server-authoritative.
	Fase       string `json:"fase"`
	IsLate     bool   `json:"is_late"`
	SisaDetik  int64  `json:"sisa_detik"` // sisa waktu menuju batas fase aktif (<=0 bila lewat)
	BisaKumpul bool   `json:"bisa_kumpul"`

	Status      string     `json:"status"`
	Sumber      *string    `json:"sumber"`
	LaporanPath *string    `json:"laporan_path"`
	AnalisaPath *string    `json:"analisa_path"`
	SubmittedAt *time.Time `json:"submitted_at"`
	Terlambat   bool       `json:"terlambat"`

	OverridePada   *time.Time `json:"override_pada"`
	OverrideAlasan *string    `json:"override_alasan"`
}

// PengumpulanRow: satu baris mahasiswa pada rekap pengumpulan admin.
type PengumpulanRow struct {
	MahasiswaID int     `json:"mahasiswa_id"`
	NIM         string  `json:"nim"`
	Nama        string  `json:"nama"`
	KelasID     *int    `json:"kelas_id"`
	NamaKelas   *string `json:"nama_kelas"`

	Status      string     `json:"status"`
	Sumber      *string    `json:"sumber"`
	SubmittedAt *time.Time `json:"submitted_at"`
	IsLate      bool       `json:"is_late"`
	DeadlineAt  *time.Time `json:"deadline_at"`

	LaporanPath *string `json:"laporan_path"`
	AnalisaPath *string `json:"analisa_path"`

	ManualOleh    *int       `json:"manual_oleh"`
	ManualPada    *time.Time `json:"manual_pada"`
	ManualCatatan *string    `json:"manual_catatan"`

	OverrideOleh   *int       `json:"override_oleh"`
	OverrideAlasan *string    `json:"override_alasan"`
	OverridePada   *time.Time `json:"override_pada"`
}
