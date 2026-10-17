package usecase

import (
	"errors"
	"fmt"
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"

	"gorm.io/gorm"
)

type JawabanUsecase struct {
	aktivasi   repository.AktivasiRepository
	terpilih   repository.SoalTerpilihRepository
	jawaban    repository.JawabanRepository
	pengerjaan repository.PengerjaanRepository
	users      repository.UserRepository
	course     repository.CourseRepository
}

func NewJawabanUsecase(
	a repository.AktivasiRepository,
	t repository.SoalTerpilihRepository,
	j repository.JawabanRepository,
	p repository.PengerjaanRepository,
	u repository.UserRepository,
	c repository.CourseRepository,
) *JawabanUsecase {
	return &JawabanUsecase{aktivasi: a, terpilih: t, jawaban: j, pengerjaan: p, users: u, course: c}
}

// cekAkses memverifikasi mahasiswa boleh mengakses aktivasi (kelas+shift cocok ATAU susulan).
// Untuk ujian praktik (aktivasi.Gelombang terisi), gelombang mahasiswa juga harus cocok.
func (uc *JawabanUsecase) cekAkses(user *entity.User, aktivasi *entity.AktivasiSesi) (bool, error) {
	cocok := user.KelasID != nil && user.Shift != nil &&
		*user.KelasID == aktivasi.KelasID && *user.Shift == aktivasi.Shift
	if cocok && aktivasi.Gelombang != nil {
		cocok = user.Gelombang != nil && *user.Gelombang == *aktivasi.Gelombang
	}
	if cocok {
		return true, nil
	}
	return uc.aktivasi.IsSusulan(aktivasi.ID, user.ID)
}

// loadKonteks mengambil user, aktivasi, aktivasi_course, dan course.
func (uc *JawabanUsecase) loadKonteks(userID, aktivasiSesiID, courseID int) (*entity.User, *entity.AktivasiSesi, *entity.AktivasiCourse, *entity.Course, error) {
	user, err := uc.users.FindByID(userID)
	if err != nil {
		return nil, nil, nil, nil, ErrUnauthorized
	}
	aktivasi, err := uc.aktivasi.FindSesiByID(aktivasiSesiID)
	if err != nil {
		return nil, nil, nil, nil, ErrNotFound
	}
	ac, err := uc.aktivasi.FindCourse(aktivasiSesiID, courseID)
	if err != nil {
		return nil, nil, nil, nil, ErrForbidden // course tidak diaktifkan untuk aktivasi ini
	}
	course, err := uc.course.FindByID(courseID)
	if err != nil {
		return nil, nil, nil, nil, ErrNotFound
	}
	ok, err := uc.cekAkses(user, aktivasi)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if !ok {
		return nil, nil, nil, nil, ErrForbidden
	}
	return user, aktivasi, ac, course, nil
}

func deadlineFrom(mulai *time.Time, durasiMenit int) *time.Time {
	if mulai == nil {
		return nil
	}
	d := mulai.Add(time.Duration(durasiMenit) * time.Minute)
	return &d
}

// effectiveDeadline: timer GLOBAL — anchor = aktivasi_course.started_at (peserta
// pertama mulai), sama untuk semua peserta course. Susulan dikecualikan: pakai
// timer per-user (waktu_mulai sendiri) karena mengerjakan di hari berbeda.
func effectiveDeadline(ac *entity.AktivasiCourse, waktuMulai *time.Time, isSusulan bool, durasiMenit int) *time.Time {
	if isSusulan {
		return deadlineFrom(waktuMulai, durasiMenit)
	}
	return deadlineFrom(ac.StartedAt, durasiMenit)
}

// GetRuang mengembalikan data ruang pengerjaan (soal + jawaban tersimpan + timer).
// Read-only: tidak memulai timer.
func (uc *JawabanUsecase) GetRuang(userID, aktivasiSesiID, courseID int) (*dto.RuangCourseResponse, error) {
	_, aktivasi, ac, course, err := uc.loadKonteks(userID, aktivasiSesiID, courseID)
	if err != nil {
		return nil, err
	}
	return uc.buildRuang(userID, aktivasi, ac, course)
}

// Mulai menandai mahasiswa mulai mengerjakan (set waktu_mulai sekali).
func (uc *JawabanUsecase) Mulai(userID int, req dto.MulaiCourseRequest) (*dto.RuangCourseResponse, error) {
	_, aktivasi, ac, course, err := uc.loadKonteks(userID, req.AktivasiSesiID, req.CourseID)
	if err != nil {
		return nil, err
	}
	if !ac.IsOpen {
		return nil, ErrAlreadyDone
	}

	// Validasi Token hanya jika jenis course adalah ujian
	if course.Jenis == entity.CourseUjianPraktik {
		if aktivasi.Token != nil {
			if req.Token == nil || *req.Token != *aktivasi.Token {
				return nil, fmt.Errorf("%w: token ujian tidak valid", ErrBadRequest)
			}
		}
	}

	p, err := uc.pengerjaan.FindOrCreate(userID, req.AktivasiSesiID, req.CourseID)
	if err != nil {
		return nil, err
	}
	if p.Status == entity.StatusSelesai {
		return nil, ErrAlreadyDone
	}
	if p.WaktuMulai == nil {
		now := time.Now()
		p.WaktuMulai = &now
		p.Status = entity.StatusSedang
		if err := uc.pengerjaan.Update(p); err != nil {
			return nil, err
		}
		// Set anchor timer global (skip susulan). Atomic: hanya peserta pertama yang menang.
		if isSusulan, _ := uc.aktivasi.IsSusulan(aktivasi.ID, userID); !isSusulan {
			_ = uc.aktivasi.SetCourseStartedAtIfNull(ac.ID, now)
			// reload ac agar buildRuang memakai started_at otoritatif (milik kita / peserta pertama lain).
			if fresh, err := uc.aktivasi.FindCourse(aktivasi.ID, course.ID); err == nil {
				ac = fresh
			}
		}
	}
	return uc.buildRuang(userID, aktivasi, ac, course)
}

// AutoSave menyimpan jawaban satu soal (validasi server: open, belum lewat deadline, belum submit).
func (uc *JawabanUsecase) AutoSave(userID int, req dto.AutoSaveRequest) error {
	st, err := uc.terpilih.FindByID(req.SoalTerpilihID)
	if err != nil {
		return ErrNotFound
	}
	user, _, ac, course, err := uc.loadKonteks(userID, st.AktivasiSesiID, st.CourseID)
	if err != nil {
		return err
	}
	_ = user
	if !ac.IsOpen {
		return ErrAlreadyDone
	}
	// Validasi deadline (server-authoritative).
	isSusulan, _ := uc.aktivasi.IsSusulan(st.AktivasiSesiID, userID)
	p, err := uc.pengerjaan.Find(userID, st.AktivasiSesiID, st.CourseID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// belum mulai → mulai otomatis saat save pertama
		p, err = uc.pengerjaan.FindOrCreate(userID, st.AktivasiSesiID, st.CourseID)
		if err != nil {
			return err
		}
		now := time.Now()
		p.WaktuMulai = &now
		p.Status = entity.StatusSedang
		_ = uc.pengerjaan.Update(p)
		if !isSusulan {
			_ = uc.aktivasi.SetCourseStartedAtIfNull(ac.ID, now)
			if fresh, err := uc.aktivasi.FindCourse(st.AktivasiSesiID, st.CourseID); err == nil {
				ac = fresh
			}
		}
	}
	if p.Status == entity.StatusSelesai {
		return ErrAlreadyDone
	}
	if dl := effectiveDeadline(ac, p.WaktuMulai, isSusulan, course.DurasiMenit); dl != nil && time.Now().After(*dl) {
		// Lewat deadline → auto-submit & tolak.
		_ = uc.finalize(userID, st.AktivasiSesiID, st.CourseID)
		return ErrTimeUp
	}

	// Upsert jawaban.
	j, err := uc.jawaban.FindByMahasiswaSoal(userID, st.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		j = &entity.JawabanMahasiswa{
			MahasiswaID:    userID,
			SoalTerpilihID: st.ID,
			JawabanTeks:    req.JawabanTeks,
			UpdatedAt:      time.Now(),
		}
		return uc.jawaban.Create(j)
	}
	if j.IsSubmitted {
		return ErrAlreadyDone
	}
	j.JawabanTeks = req.JawabanTeks
	j.UpdatedAt = time.Now()
	return uc.jawaban.Update(j)
}

// Submit melakukan submit manual seluruh course untuk mahasiswa.
func (uc *JawabanUsecase) Submit(userID int, req dto.SubmitRequest) error {
	_, _, _, _, err := uc.loadKonteks(userID, req.AktivasiSesiID, req.CourseID)
	if err != nil {
		return err
	}
	return uc.finalize(userID, req.AktivasiSesiID, req.CourseID)
}

// finalize menandai semua jawaban mahasiswa submitted & pengerjaan selesai.
func (uc *JawabanUsecase) finalize(userID, aktivasiSesiID, courseID int) error {
	if _, err := uc.jawaban.MarkSubmittedForMahasiswaCourse(userID, aktivasiSesiID, courseID); err != nil {
		return err
	}
	p, err := uc.pengerjaan.FindOrCreate(userID, aktivasiSesiID, courseID)
	if err != nil {
		return err
	}
	now := time.Now()
	p.Status = entity.StatusSelesai
	p.WaktuSelesai = &now
	return uc.pengerjaan.Update(p)
}

// AutoSubmitExpired disapu oleh background job: auto-submit pengerjaan yang lewat deadline.
func (uc *JawabanUsecase) AutoSubmitExpired() (int, error) {
	expired, err := uc.pengerjaan.FindExpired()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range expired {
		if err := uc.finalize(e.MahasiswaID, e.AktivasiSesiID, e.CourseID); err != nil {
			continue
		}
		n++
	}
	return n, nil
}

// buildRuang menyusun response ruang course.
func (uc *JawabanUsecase) buildRuang(userID int, aktivasi *entity.AktivasiSesi, ac *entity.AktivasiCourse, course *entity.Course) (*dto.RuangCourseResponse, error) {
	soalTerpilih, err := uc.terpilih.ListByAktivasiCourse(aktivasi.ID, course.ID)
	if err != nil {
		return nil, err
	}
	jawabanList, err := uc.jawaban.ListByMahasiswaCourse(userID, aktivasi.ID, course.ID)
	if err != nil {
		return nil, err
	}
	jawabanMap := make(map[int]entity.JawabanMahasiswa, len(jawabanList))
	for _, j := range jawabanList {
		jawabanMap[j.SoalTerpilihID] = j
	}

	resp := &dto.RuangCourseResponse{
		AktivasiSesiID: aktivasi.ID,
		CourseID:       course.ID,
		Jenis:          string(course.Jenis),
		DurasiMenit:    course.DurasiMenit,
		IsOpen:         ac.IsOpen,
		RequireToken:   course.Jenis == entity.CourseUjianPraktik && aktivasi.Token != nil,
		Status:         string(entity.StatusBelum),
		Soal:           make([]dto.SoalTampilResponse, 0),
	}

	if p, err := uc.pengerjaan.Find(userID, aktivasi.ID, course.ID); err == nil {
		resp.Status = string(p.Status)
		resp.WaktuMulai = p.WaktuMulai
		isSusulan, _ := uc.aktivasi.IsSusulan(aktivasi.ID, userID)
		resp.Deadline = effectiveDeadline(ac, p.WaktuMulai, isSusulan, course.DurasiMenit)
	}

	for _, st := range soalTerpilih {
		item := dto.SoalTampilResponse{
			SoalTerpilihID: st.ID,
			Urutan:         st.Urutan,
		}
		if st.Soal != nil {
			item.JenisSoal = string(st.Soal.JenisSoal)
			item.TeksSoal = st.Soal.TeksSoal
			item.GambarURL = st.Soal.GambarURL
			item.Poin = st.Soal.Poin
			if st.Soal.KategoriUjian != nil {
				k := string(*st.Soal.KategoriUjian)
				item.KategoriUjian = &k
			}
		}
		if j, ok := jawabanMap[st.ID]; ok {
			item.JawabanTeks = j.JawabanTeks
			item.IsSubmitted = j.IsSubmitted
		} else if st.Soal != nil && st.Soal.KodeRumpang != nil && *st.Soal.KodeRumpang != "" &&
			st.Soal.JenisSoal == entity.SoalCoding {
			// Prefill kode rumpang (kelas karyawan): jawaban awal = kerangka
			// ber-blank, hanya bila mahasiswa belum pernah menyimpan apa pun.
			item.JawabanTeks = *st.Soal.KodeRumpang
		}
		resp.Soal = append(resp.Soal, item)
	}
	return resp, nil
}

// AdminInjectJawaban memungkinkan admin menginject jawaban baru atau mengedit jawaban
// existing untuk mahasiswa tertentu. Tidak ada validasi kelas+shift, deadline, atau is_open
// karena ini aksi admin eksplisit.
func (uc *JawabanUsecase) AdminInjectJawaban(req dto.AdminInjectJawabanRequest) error {
	// 1. Validasi soal_terpilih exists.
	st, err := uc.terpilih.FindByID(req.SoalTerpilihID)
	if err != nil {
		return fmt.Errorf("%w: soal_terpilih tidak ditemukan", ErrNotFound)
	}

	// 2. Validasi mahasiswa exists.
	if _, err := uc.users.FindByID(req.MahasiswaID); err != nil {
		return fmt.Errorf("%w: mahasiswa tidak ditemukan", ErrNotFound)
	}

	// 3. Pastikan pengerjaan_course ada.
	p, err := uc.pengerjaan.FindOrCreate(req.MahasiswaID, st.AktivasiSesiID, st.CourseID)
	if err != nil {
		return err
	}
	if p.WaktuMulai == nil {
		now := time.Now()
		p.WaktuMulai = &now
		p.Status = entity.StatusSedang
		_ = uc.pengerjaan.Update(p)
	}

	// 4. Upsert jawaban (create or update, bahkan jika sudah submitted).
	now := time.Now()
	j, err := uc.jawaban.FindByMahasiswaSoal(req.MahasiswaID, st.ID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Create baru.
		j = &entity.JawabanMahasiswa{
			MahasiswaID:    req.MahasiswaID,
			SoalTerpilihID: st.ID,
			JawabanTeks:    req.JawabanTeks,
			IsSubmitted:    req.AutoSubmit,
			UpdatedAt:      now,
		}
		if req.AutoSubmit {
			j.WaktuSubmit = &now
		}
		return uc.jawaban.Create(j)
	}

	// Update existing (termasuk yang sudah submitted — admin privilege).
	j.JawabanTeks = req.JawabanTeks
	j.UpdatedAt = now
	if req.AutoSubmit {
		j.IsSubmitted = true
		j.WaktuSubmit = &now
	}
	return uc.jawaban.Update(j)
}
