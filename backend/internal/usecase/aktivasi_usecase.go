package usecase

import (
	"crypto/rand"
	"errors"
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"

	"gorm.io/gorm"
)

type AktivasiUsecase struct {
	aktivasi   repository.AktivasiRepository
	sesi       repository.SesiRepository
	course     repository.CourseRepository
	kelas      repository.KelasRepository
	jawaban    repository.JawabanRepository
	pengerjaan repository.PengerjaanRepository
	soalUC     *SoalUsecase
	tx         *repository.AktivasiTxRepo
}

func NewAktivasiUsecase(
	a repository.AktivasiRepository,
	s repository.SesiRepository,
	c repository.CourseRepository,
	k repository.KelasRepository,
	j repository.JawabanRepository,
	p repository.PengerjaanRepository,
	soalUC *SoalUsecase,
	tx *repository.AktivasiTxRepo,
) *AktivasiUsecase {
	return &AktivasiUsecase{aktivasi: a, sesi: s, course: c, kelas: k, jawaban: j, pengerjaan: p, soalUC: soalUC, tx: tx}
}

func (uc *AktivasiUsecase) List() ([]entity.AktivasiSesi, error) { return uc.aktivasi.ListSesi() }
func (uc *AktivasiUsecase) ListActive() ([]entity.AktivasiSesi, error) {
	return uc.aktivasi.ListActiveSesi()
}

func (uc *AktivasiUsecase) Get(id int) (*entity.AktivasiSesi, error) {
	a, err := uc.aktivasi.FindSesiByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return a, nil
}

func generateRandomPIN() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	// crypto/rand: token akses ujian tidak boleh mudah ditebak.
	if _, err := rand.Read(b); err != nil {
		return "", err // jangan jatuh ke PIN deterministik yang bisa ditebak
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

func (uc *AktivasiUsecase) GenerateToken(id int) (*entity.AktivasiSesi, error) {
	aks, err := uc.aktivasi.FindSesiByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	pin, err := generateRandomPIN()
	if err != nil {
		return nil, err
	}
	aks.Token = &pin
	if err := uc.aktivasi.UpdateSesi(aks); err != nil {
		return nil, err
	}
	return aks, nil
}

// Aktivasi mengaktifkan sesi untuk kelas+shift, melakukan gacha (untuk sesi normal),
// membuat baris aktivasi_course untuk course terpilih, lalu mengacak soal-nya.
func (uc *AktivasiUsecase) Aktivasi(req dto.AktivasiRequest) (*entity.AktivasiSesi, error) {
	sesi, err := uc.sesi.FindByID(req.SesiPraktikumID)
	if err != nil {
		return nil, ErrNotFound
	}
	if _, err := uc.kelas.FindByID(req.KelasID); err != nil {
		return nil, ErrNotFound
	}

	// Gelombang hanya berlaku untuk ujian praktik; sesi normal selalu NULL.
	var gelombang *int
	if sesi.IsUjianPraktik {
		gelombang = req.Gelombang
	}

	// Cek duplikat aktivasi (per gelombang untuk ujian praktik).
	if _, err := uc.aktivasi.FindSesiByComposite(req.SesiPraktikumID, req.KelasID, req.Shift, gelombang); err == nil {
		return nil, ErrConflict
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	courses, err := uc.course.ListBySesi(sesi.ID)
	if err != nil {
		return nil, err
	}

	// Tentukan course mana yang dibuatkan aktivasi_course.
	var dipakai []entity.Course
	if sesi.IsUjianPraktik {
		for _, c := range courses {
			if c.Jenis == entity.CourseUjianPraktik {
				dipakai = append(dipakai, c)
			}
		}
		if len(dipakai) == 0 {
			return nil, errors.New("sesi ujian praktik belum punya course ujian_praktik")
		}
	} else {
		// Gacha: pretest ATAU posttest (wajib pilih), + keterampilan.
		if req.GachaPilihan == "" {
			return nil, errors.New("gacha_pilihan (pretest/posttest) wajib untuk sesi normal")
		}
		pilihan := entity.JenisCourse(req.GachaPilihan)
		for _, c := range courses {
			if c.Jenis == pilihan || c.Jenis == entity.CourseKeterampilan {
				dipakai = append(dipakai, c)
			}
		}
		if len(dipakai) == 0 {
			return nil, errors.New("course terpilih tidak tersedia di sesi ini")
		}
	}

	// Precompute: acak soal & susun seluruh baris SEBELUM menulis apa pun.
	// Jika pool soal kurang, error muncul di sini tanpa menyisakan state setengah jadi.
	aktivasiCourses := make([]entity.AktivasiCourse, 0, len(dipakai))
	var terpilih []entity.SoalTerpilih
	for i, c := range dipakai {
		course := c
		aktivasiCourses = append(aktivasiCourses, entity.AktivasiCourse{
			CourseID: course.ID,
			IsOpen:   false,
			Urutan:   urutanCourse(course.Jenis, i),
		})
		picks, err := uc.soalUC.PilihSoalUntukCourse(&course)
		if err != nil {
			return nil, err
		}
		for j, s := range picks {
			terpilih = append(terpilih, entity.SoalTerpilih{
				CourseID: course.ID,
				SoalID:   s.ID,
				Urutan:   j + 1,
			})
		}
	}

	// Tulis aktivasi_sesi + aktivasi_course + soal_terpilih secara ATOMIK.
	aks := &entity.AktivasiSesi{
		SesiPraktikumID: req.SesiPraktikumID,
		KelasID:         req.KelasID,
		Shift:           req.Shift,
		Gelombang:       gelombang,
		IsActive:        true,
		ActivatedAt:     time.Now(),
	}
	if err := uc.tx.CreateActivation(aks, aktivasiCourses, terpilih); err != nil {
		return nil, err
	}

	return uc.aktivasi.FindSesiByID(aks.ID)
}

// urutanCourse menentukan urutan default buka course.
func urutanCourse(jenis entity.JenisCourse, fallback int) int {
	switch jenis {
	case entity.CoursePretest:
		return 1
	case entity.CourseKeterampilan:
		return 2
	case entity.CoursePosttest:
		return 3
	case entity.CourseUjianPraktik:
		return 1
	default:
		return fallback + 1
	}
}

// BukaTutupCourse mengubah is_open sebuah aktivasi_course.
// Saat menutup (is_open=false): auto-submit massal semua jawaban belum-submit
// dan tandai pengerjaan_course = selesai.
// Saat membuka (is_open=true): kunci peserta DILEPAS kembali (pengerjaan ->
// belum_dikerjakan, is_submitted direset) dan anchor timer global dikosongkan,
// supaya peserta bisa mengetik & mengerjakan ulang. Sebelumnya buka-ulang hanya
// mengubah is_open, sehingga seluruh peserta tetap read-only di ruang ujian
// (gejala "tidak bisa mengetik" setelah course ditutup lalu dibuka).
func (uc *AktivasiUsecase) BukaTutupCourse(req dto.BukaTutupCourseRequest) (*entity.AktivasiCourse, error) {
	ac, err := uc.aktivasi.FindCourseByID(req.AktivasiCourseID)
	if err != nil {
		return nil, ErrNotFound
	}
	now := time.Now()
	ac.IsOpen = req.IsOpen
	if req.IsOpen {
		ac.OpenedAt = &now
		ac.ClosedAt = nil
		// Anchor timer global (peserta pertama) dimulai ulang dari nol.
		ac.StartedAt = nil
	} else {
		ac.ClosedAt = &now
	}
	if err := uc.aktivasi.UpdateCourse(ac); err != nil {
		return nil, err
	}

	if !req.IsOpen {
		// Auto-submit massal.
		if _, err := uc.jawaban.MarkSubmittedForCourse(ac.AktivasiSesiID, ac.CourseID); err != nil {
			return nil, err
		}
		if err := uc.pengerjaan.MarkSelesaiForCourse(ac.AktivasiSesiID, ac.CourseID); err != nil {
			return nil, err
		}
	} else {
		// Buka ulang: lepas kunci peserta. Isi jawaban tidak dihapus.
		if _, err := uc.jawaban.UnmarkSubmittedForCourse(ac.AktivasiSesiID, ac.CourseID); err != nil {
			return nil, err
		}
		if _, err := uc.pengerjaan.ResetForCourse(ac.AktivasiSesiID, ac.CourseID); err != nil {
			return nil, err
		}
	}
	return ac, nil
}

// ---- Susulan ----

func (uc *AktivasiUsecase) AddSusulan(req dto.SusulanRequest) error {
	if _, err := uc.aktivasi.FindSesiByID(req.AktivasiSesiID); err != nil {
		return ErrNotFound
	}
	p := &entity.PesertaSusulan{
		AktivasiSesiID: req.AktivasiSesiID,
		MahasiswaID:    req.MahasiswaID,
		Alasan:         req.Alasan,
		CreatedAt:      time.Now(),
	}
	if err := uc.aktivasi.AddSusulan(p); err != nil {
		return ErrConflict
	}
	return nil
}

func (uc *AktivasiUsecase) RemoveSusulan(aktivasiSesiID, mahasiswaID int) error {
	return uc.aktivasi.RemoveSusulan(aktivasiSesiID, mahasiswaID)
}

func (uc *AktivasiUsecase) ListSusulan(aktivasiSesiID int) ([]entity.PesertaSusulan, error) {
	return uc.aktivasi.ListSusulan(aktivasiSesiID)
}

// DeleteSesi menghapus satu aktivasi beserta SEMUA data anaknya (jawaban, nilai,
// soal terpilih, susulan, aktivasi_course) — cascade di repository.
func (uc *AktivasiUsecase) DeleteSesi(id int) error {
	return uc.aktivasi.DeleteSesi(id)
}

// PesertaProgress: daftar peserta yang sudah join/mengerjakan aktivasi + status live.
func (uc *AktivasiUsecase) PesertaProgress(aktivasiSesiID int) ([]repository.PesertaProgress, error) {
	return uc.pengerjaan.ListPesertaProgress(aktivasiSesiID)
}
