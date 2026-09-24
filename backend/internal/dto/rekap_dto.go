package dto

// RekapKolom merepresentasikan kolom dinamis dari hasil pivot (misal "course_1").
type RekapKolom struct {
	Key   string `json:"key"`   // identifier unik course, misal "course_1"
	Label string `json:"label"` // label human-readable, misal "Modul 1 - Pre-test"
}

// RekapSel: satu sel nilai per course. Total = murni + keaktifan (pretest/posttest).
type RekapSel struct {
	PengerjaanID  int      `json:"pengerjaan_id"`
	Murni         float64  `json:"murni"`
	Keaktifan     *float64 `json:"keaktifan"`      // null bila bukan pretest/posttest
	Total         float64  `json:"total"`          // murni + (keaktifan||0)
	EditKeaktifan bool     `json:"edit_keaktifan"` // true bila pretest/posttest
	NilaiAkhir    *float64 `json:"nilai_akhir"`    // konversi 0-100 (murni/max*100), khusus ujian praktik
}

// RekapMahasiswa merepresentasikan baris pada tabel pivot.
type RekapMahasiswa struct {
	NIM        string              `json:"nim"`
	Nama       string              `json:"nama"`
	Scores     map[string]RekapSel `json:"scores"` // Key: "course_<id>"
	TotalScore float64             `json:"total_score"`
}

// RekapKelasResponse adalah response lengkap untuk endpoint rekapitulasi.
type RekapKelasResponse struct {
	Columns []RekapKolom     `json:"columns"`
	Data    []RekapMahasiswa `json:"data"`
}
