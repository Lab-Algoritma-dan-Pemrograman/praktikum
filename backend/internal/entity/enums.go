package entity

// Enum-enum domain (disimpan sebagai string/varchar di MySQL).

type RoleType string

const (
	RoleMahasiswa   RoleType = "mahasiswa"
	RoleAsisten     RoleType = "asisten"
	RoleKoordinator RoleType = "koordinator"
)

// AllowedEmailDomains: domain populer yang diizinkan untuk register.
// itpln.ac.id bersifat template: hanya diizinkan bila config EmailAllowItpln = true.
var AllowedEmailDomains = map[string]bool{
	"gmail.com":    true,
	"yahoo.com":    true,
	"outlook.com":  true,
	"hotmail.com":  true,
}

type JenisCourse string

const (
	CoursePretest      JenisCourse = "pretest"
	CoursePosttest     JenisCourse = "posttest"
	CourseKeterampilan JenisCourse = "keterampilan"
	CourseUjianPraktik JenisCourse = "ujian_praktik"
)

type JenisSoal string

const (
	SoalEssay  JenisSoal = "essay"
	SoalCoding JenisSoal = "coding"
)

type Difficulty string

const (
	DiffEasy   Difficulty = "easy"
	DiffMedium Difficulty = "medium"
	DiffHard   Difficulty = "hard"
)

type KategoriUjian string

const (
	KatModul1    KategoriUjian = "modul_1"
	KatModul2    KategoriUjian = "modul_2"
	KatModul3    KategoriUjian = "modul_3"
	KatModul45   KategoriUjian = "modul_4_5"
	KatModul6    KategoriUjian = "modul_6"
	KatFlowchart KategoriUjian = "flowchart"
)

// SemuaKategoriUjian dipakai saat mengacak 1 soal per kategori.
var SemuaKategoriUjian = []KategoriUjian{
	KatModul1, KatModul2, KatModul3, KatModul45, KatModul6, KatFlowchart,
}

type StatusPengerjaan string

const (
	StatusBelum  StatusPengerjaan = "belum_dikerjakan"
	StatusSedang StatusPengerjaan = "sedang_dikerjakan"
	StatusSelesai StatusPengerjaan = "selesai"
)
