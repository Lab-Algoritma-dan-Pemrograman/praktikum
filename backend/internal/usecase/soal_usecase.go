package usecase

import (
	"fmt"
	"math/rand"
	"regexp"

	"github.com/microcosm-cc/bluemonday"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
)

type SoalUsecase struct {
	soal     repository.SoalRepository
	course   repository.CourseRepository
	terpilih repository.SoalTerpilihRepository
}

func NewSoalUsecase(s repository.SoalRepository, c repository.CourseRepository, t repository.SoalTerpilihRepository) *SoalUsecase {
	return &SoalUsecase{soal: s, course: c, terpilih: t}
}

// ---- CRUD pool ----

func (uc *SoalUsecase) Create(req dto.SoalRequest) (*entity.Soal, error) {
	if _, err := uc.course.FindByID(req.CourseID); err != nil {
		return nil, ErrNotFound
	}
	p := createSanitizer()

	s := &entity.Soal{
		CourseID:  req.CourseID,
		JenisSoal: entity.JenisSoal(req.JenisSoal),
		TeksSoal:  p.Sanitize(req.TeksSoal),
		GambarURL: req.GambarURL,
		Poin:      req.Poin,
	}

	if req.KunciJawaban != nil {
		sanitized := p.Sanitize(*req.KunciJawaban)
		s.KunciJawaban = &sanitized
	}
	if req.Difficulty != nil {
		d := entity.Difficulty(*req.Difficulty)
		s.Difficulty = &d
	}
	if req.KategoriUjian != nil {
		k := entity.KategoriUjian(*req.KategoriUjian)
		s.KategoriUjian = &k
	}
	if err := uc.soal.Create(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *SoalUsecase) Update(id int, req dto.SoalRequest) (*entity.Soal, error) {
	s, err := uc.soal.FindByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	p := createSanitizer()

	s.JenisSoal = entity.JenisSoal(req.JenisSoal)
	s.TeksSoal = p.Sanitize(req.TeksSoal)
	s.GambarURL = req.GambarURL
	s.Poin = req.Poin

	if req.KunciJawaban != nil {
		sanitized := p.Sanitize(*req.KunciJawaban)
		s.KunciJawaban = &sanitized
	} else {
		s.KunciJawaban = nil
	}
	s.Difficulty = nil
	if req.Difficulty != nil {
		d := entity.Difficulty(*req.Difficulty)
		s.Difficulty = &d
	}
	s.KategoriUjian = nil
	if req.KategoriUjian != nil {
		k := entity.KategoriUjian(*req.KategoriUjian)
		s.KategoriUjian = &k
	}
	if err := uc.soal.Update(s); err != nil {
		return nil, err
	}
	return uc.soal.FindByID(id)
}

func (uc *SoalUsecase) Delete(id int) error { return uc.soal.Delete(id) }
func (uc *SoalUsecase) ListByCourse(courseID int) ([]entity.Soal, error) {
	return uc.soal.ListByCourse(courseID)
}

// ---- Pengacakan ke soal_terpilih ----

// distribusiDifficulty mengembalikan jumlah soal per difficulty. Pakai kuota
// per course bila admin mengisinya; selain itu fallback ke default per jenis.
func distribusiDifficulty(course *entity.Course) map[entity.Difficulty]int {
	if course.KuotaEasy != nil || course.KuotaMedium != nil || course.KuotaHard != nil {
		return map[entity.Difficulty]int{
			entity.DiffEasy:   intOrZero(course.KuotaEasy),
			entity.DiffMedium: intOrZero(course.KuotaMedium),
			entity.DiffHard:   intOrZero(course.KuotaHard),
		}
	}
	switch course.Jenis {
	case entity.CoursePretest:
		// default: 1 easy, 2 medium, 2 hard (total 5)
		return map[entity.Difficulty]int{entity.DiffEasy: 1, entity.DiffMedium: 2, entity.DiffHard: 2}
	case entity.CoursePosttest:
		// default: 1 easy, 1 medium, 1 hard (total 3)
		return map[entity.Difficulty]int{entity.DiffEasy: 1, entity.DiffMedium: 1, entity.DiffHard: 1}
	default:
		return nil
	}
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// PilihSoalUntukCourse mengacak soal dari pool sesuai jenis course dan
// mengembalikan hasilnya TANPA menulis ke DB. Memungkinkan caller memvalidasi
// ketersediaan pool lebih dulu, lalu menulis seluruh aktivasi secara atomik.
func (uc *SoalUsecase) PilihSoalUntukCourse(course *entity.Course) ([]entity.Soal, error) {
	var dipilih []entity.Soal

	switch course.Jenis {
	case entity.CoursePretest, entity.CoursePosttest:
		dist := distribusiDifficulty(course)
		for diff, n := range dist {
			pool, err := uc.soal.PoolByDifficulty(course.ID, diff)
			if err != nil {
				return nil, err
			}
			picked, err := pickRandom(pool, n, course.Jenis, string(diff))
			if err != nil {
				return nil, err
			}
			dipilih = append(dipilih, picked...)
		}
	case entity.CourseKeterampilan:
		pool, err := uc.soal.PoolAll(course.ID)
		if err != nil {
			return nil, err
		}
		picked, err := pickRandom(pool, 1, course.Jenis, "keterampilan")
		if err != nil {
			return nil, err
		}
		dipilih = append(dipilih, picked...)
	case entity.CourseUjianPraktik:
		for _, kat := range entity.SemuaKategoriUjian {
			pool, err := uc.soal.PoolByKategori(course.ID, kat)
			if err != nil {
				return nil, err
			}
			picked, err := pickRandom(pool, 1, course.Jenis, string(kat))
			if err != nil {
				return nil, err
			}
			dipilih = append(dipilih, picked...)
		}
	}

	return dipilih, nil
}

// pickRandom memilih n soal acak dari pool; error jika pool kurang.
func pickRandom(pool []entity.Soal, n int, jenis entity.JenisCourse, label string) ([]entity.Soal, error) {
	if len(pool) < n {
		return nil, fmt.Errorf("%w: pool soal '%s' (%s) hanya %d, butuh %d",
			ErrBadRequest, label, jenis, len(pool), n)
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	return pool[:n], nil
}

// createSanitizer mengatur policy bluemonday agar mengizinkan style dari Tiptap.
func createSanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// Izinkan class dan data atribut untuk code block, dsb.
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("data-type", "data-language", "data-id", "data-latex").Globally()

	// Izinkan inline styles yang umum dipakai editor Tiptap/Edra
	p.AllowStyles("text-align").Matching(regexp.MustCompile(`(?i)^(left|right|center|justify)$`)).Globally()
	p.AllowStyles("color", "background-color").Matching(regexp.MustCompile(`(?i)^(transparent|#[0-9a-fA-F]+|rgba?\([^)]+\)|[a-zA-Z]+)$`)).Globally()

	return p
}
