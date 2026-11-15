package entity

import "time"

// Laporan besar is a standalone final practicum report with per-class submission
// windows. It is not tied to a practicum session.
type LaporanBesar struct {
	ID           string               `json:"id"`
	Judul        string               `json:"judul"`
	Deskripsi    string               `json:"deskripsi"`
	ButuhLaporan bool                 `json:"butuh_laporan"`
	ButuhAnalisa bool                 `json:"butuh_analisa"`
	Active       bool                 `json:"active"`
	Targets      []LaporanBesarTarget `json:"targets"`
	CreatedBy    int64                `json:"created_by"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    *time.Time           `json:"updated_at,omitempty"`
	Scope        Scope                `json:"scope"`
	Version      int64                `json:"version"`
}

// LaporanBesarTarget is a submission window for one class (and optionally shift).
// BukaAt nil = already open; LateUntil nil = no late window.
// Shift 0 applies to every shift of the class.
type LaporanBesarTarget struct {
	ClassID    string     `json:"class_id"`
	Shift      int        `json:"shift"`
	BukaAt     *time.Time `json:"buka_at,omitempty"`
	DeadlineAt time.Time  `json:"deadline_at"`
	LateUntil  *time.Time `json:"late_until,omitempty"`
}

// Laporan besar submission phases, computed server-side only.
const (
	LaporanBelumDibuka = "belum_dibuka"
	LaporanDibuka      = "dibuka"
	LaporanTelat       = "telat"
	LaporanDitutup     = "ditutup"
)

type LaporanPengumpulan struct {
	ID             string     `json:"id"`
	LaporanID      string     `json:"laporan_id"`
	UserID         int64      `json:"user_id"`
	Status         string     `json:"status"` // "belum" | "terkumpul"
	Sumber         string     `json:"sumber"` // "web" | "manual" | ""
	LaporanPath    string     `json:"laporan_path,omitempty"`
	AnalisaPath    string     `json:"analisa_path,omitempty"`
	SubmittedAt    *time.Time `json:"submitted_at,omitempty"`
	IsLate         bool       `json:"is_late"`
	ClassID        string     `json:"class_id,omitempty"`
	Shift          int        `json:"shift"`
	ManualOleh     *int64     `json:"manual_oleh,omitempty"`
	ManualPada     *time.Time `json:"manual_pada,omitempty"`
	ManualCatatan  string     `json:"manual_catatan,omitempty"`
	OverrideOleh   *int64     `json:"override_oleh,omitempty"`
	OverrideAlasan string     `json:"override_alasan,omitempty"`
	OverridePada   *time.Time `json:"override_pada,omitempty"`
	Version        int64      `json:"version"`
}

// Student-facing status view. Never leaks object paths or override identity.
type LaporanStatus struct {
	LaporanID    string     `json:"laporan_id"`
	Judul        string     `json:"judul"`
	Deskripsi    string     `json:"deskripsi"`
	ButuhLaporan bool       `json:"butuh_laporan"`
	ButuhAnalisa bool       `json:"butuh_analisa"`
	ClassID      string     `json:"class_id,omitempty"`
	Shift        int        `json:"shift"`
	BukaAt       *time.Time `json:"buka_at,omitempty"`
	DeadlineAt   *time.Time `json:"deadline_at,omitempty"`
	LateUntil    *time.Time `json:"late_until,omitempty"`
	Fase         string     `json:"fase"`
	SisaDetik    int64      `json:"sisa_detik"`
	BisaKumpul   bool       `json:"bisa_kumpul"`
	Status       string     `json:"status"` // "belum" | "terkumpul"
	Sumber       string     `json:"sumber"`
	Terlambat    bool       `json:"terlambat"`
	SubmittedAt  *time.Time `json:"submitted_at,omitempty"`
	HasLaporan   bool       `json:"has_laporan"`
	HasAnalisa   bool       `json:"has_analisa"`
}

// Admin recap row. Includes object paths (admin-only) and audit trail.
type LaporanPengumpulanRow struct {
	UserID         int64      `json:"user_id"`
	UserName       string     `json:"user_name"`
	ClassID        string     `json:"class_id,omitempty"`
	Shift          int        `json:"shift"`
	Status         string     `json:"status"`
	Sumber         string     `json:"sumber"`
	LaporanPath    string     `json:"laporan_path,omitempty"`
	AnalisaPath    string     `json:"analisa_path,omitempty"`
	SubmittedAt    *time.Time `json:"submitted_at,omitempty"`
	IsLate         bool       `json:"is_late"`
	ManualOleh     *int64     `json:"manual_oleh,omitempty"`
	ManualPada     *time.Time `json:"manual_pada,omitempty"`
	ManualCatatan  string     `json:"manual_catatan,omitempty"`
	OverrideOleh   *int64     `json:"override_oleh,omitempty"`
	OverrideAlasan string     `json:"override_alasan,omitempty"`
	OverridePada   *time.Time `json:"override_pada,omitempty"`
	Version        int64      `json:"version"`
}
