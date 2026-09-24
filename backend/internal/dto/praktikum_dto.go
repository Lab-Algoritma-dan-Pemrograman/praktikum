package dto

// CourseUserItem: status course untuk seorang mahasiswa dalam aktivasi.
type CourseUserItem struct {
	CourseID         int      `json:"course_id"`
	AktivasiCourseID int      `json:"aktivasi_course_id"`
	Jenis            string   `json:"jenis"`
	Judul            string   `json:"judul"`
	DurasiMenit      int      `json:"durasi_menit"`
	IsOpen           bool     `json:"is_open"`
	Status           string   `json:"status"`
	TotalNilai       *float64 `json:"total_nilai"`
}

// SesiUserItem: satu sesi dari sudut pandang mahasiswa.
type SesiUserItem struct {
	SesiID         int              `json:"sesi_id"`
	Judul          string           `json:"judul"`
	Deskripsi      string           `json:"deskripsi"`
	Urutan         int              `json:"urutan"`
	IsUjianPraktik bool             `json:"is_ujian_praktik"`
	Aktif          bool             `json:"aktif"`
	Susulan        bool             `json:"susulan"`
	AktivasiSesiID *int             `json:"aktivasi_sesi_id"`
	Courses        []CourseUserItem `json:"courses"`
}

// DashboardUserResponse: data dashboard mahasiswa.
type DashboardUserResponse struct {
	Profil       UserResponse      `json:"profil"`
	Jadwal       *JadwalInfo       `json:"jadwal"`
	SesiAktif    []SesiUserItem    `json:"sesi_aktif"`
	RiwayatNilai []NilaiCourseItem `json:"riwayat_nilai"`
}

// JadwalInfo: jadwal ringkas mahasiswa.
type JadwalInfo struct {
	Hari       string `json:"hari"`
	JamMulai   string `json:"jam_mulai"`
	JamSelesai string `json:"jam_selesai"`
	Keterangan string `json:"keterangan"`
}

// NilaiCourseItem: nilai mahasiswa per course yang sudah dinilai.
type NilaiCourseItem struct {
	SesiJudul  string   `json:"sesi_judul"`
	Jenis      string   `json:"jenis"`
	Status     string   `json:"status"`
	TotalNilai *float64 `json:"total_nilai"`
}

// RunRequest: jalankan kode (C/Python) lewat sandbox eksternal.
type RunRequest struct {
	Language string `json:"language" binding:"required,oneof=c python"`
	Source   string `json:"source" binding:"required"`
	Stdin    string `json:"stdin"`
}

// RunResponse: hasil eksekusi kode.
type RunResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"error"`
}

// CompileRequest: kompilasi sumber C ke wasm32-wasi.
type CompileRequest struct {
	Source string `json:"source" binding:"required"`
}

// CompileResponse: hasil kompilasi. Wasm = base64 (kosong bila gagal); Stderr = pesan clang.
type CompileResponse struct {
	Wasm   string `json:"wasm"`
	Stderr string `json:"stderr"`
}
