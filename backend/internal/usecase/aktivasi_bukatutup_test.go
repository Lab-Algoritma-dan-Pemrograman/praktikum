package usecase

import (
	"testing"
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository/mocks"

	"github.com/stretchr/testify/mock"
)

// Regresi 2026-09-28 (PK Modul 1, kelas 6 shift 1): course ditutup lalu dibuka
// ulang, tetapi 19 peserta tetap read-only di ruang ujian sehingga "tidak bisa
// mengetik". Penyebabnya buka-ulang hanya mengubah is_open; status pengerjaan
// peserta dan anchor timer global dibiarkan. Test ini mengunci perilaku baru:
// buka ulang WAJIB melepas kunci peserta.
func TestBukaTutupCourse_BukaUlangMelepasKunci(t *testing.T) {
	anchor := time.Date(2026, 9, 28, 11, 56, 48, 0, time.UTC)
	ac := &entity.AktivasiCourse{
		ID:             9,
		AktivasiSesiID: 5,
		CourseID:       1,
		IsOpen:         false,
		StartedAt:      &anchor,
	}

	mAkt := &mocks.AktivasiRepository{}
	mAkt.On("FindCourseByID", 9).Return(ac, nil)
	mAkt.On("UpdateCourse", mock.AnythingOfType("*entity.AktivasiCourse")).
		Run(func(args mock.Arguments) {
			saved := args.Get(0).(*entity.AktivasiCourse)
			if !saved.IsOpen {
				t.Error("is_open harus true setelah dibuka")
			}
			if saved.OpenedAt == nil {
				t.Error("opened_at harus diisi")
			}
			if saved.ClosedAt != nil {
				t.Error("closed_at harus dikosongkan saat dibuka")
			}
			if saved.StartedAt != nil {
				t.Error("started_at (anchor timer global) harus dikosongkan supaya durasi mulai dari peserta pertama lagi")
			}
		}).
		Return(nil)

	mJawaban := &mocks.JawabanRepository{}
	mJawaban.On("UnmarkSubmittedForCourse", 5, 1).Return(int64(5), nil)

	mPengerjaan := &mocks.PengerjaanRepository{}
	mPengerjaan.On("ResetForCourse", 5, 1).Return(int64(19), nil)

	uc := &AktivasiUsecase{aktivasi: mAkt, jawaban: mJawaban, pengerjaan: mPengerjaan}
	if _, err := uc.BukaTutupCourse(dto.BukaTutupCourseRequest{AktivasiCourseID: 9, IsOpen: true}); err != nil {
		t.Fatalf("buka ulang tidak boleh error: %v", err)
	}

	mJawaban.AssertCalled(t, "UnmarkSubmittedForCourse", 5, 1)
	mPengerjaan.AssertCalled(t, "ResetForCourse", 5, 1)
	// Auto-submit massal hanya untuk penutupan.
	mJawaban.AssertNotCalled(t, "MarkSubmittedForCourse", mock.Anything, mock.Anything)
	mPengerjaan.AssertNotCalled(t, "MarkSelesaiForCourse", mock.Anything, mock.Anything)
}

// Menutup course tetap auto-submit massal seperti semula.
func TestBukaTutupCourse_TutupAutoSubmitMassal(t *testing.T) {
	ac := &entity.AktivasiCourse{ID: 9, AktivasiSesiID: 5, CourseID: 1, IsOpen: true}

	mAkt := &mocks.AktivasiRepository{}
	mAkt.On("FindCourseByID", 9).Return(ac, nil)
	mAkt.On("UpdateCourse", mock.AnythingOfType("*entity.AktivasiCourse")).
		Run(func(args mock.Arguments) {
			saved := args.Get(0).(*entity.AktivasiCourse)
			if saved.IsOpen {
				t.Error("is_open harus false setelah ditutup")
			}
			if saved.ClosedAt == nil {
				t.Error("closed_at harus diisi")
			}
		}).
		Return(nil)

	mJawaban := &mocks.JawabanRepository{}
	mJawaban.On("MarkSubmittedForCourse", 5, 1).Return(int64(5), nil)

	mPengerjaan := &mocks.PengerjaanRepository{}
	mPengerjaan.On("MarkSelesaiForCourse", 5, 1).Return(nil)

	uc := &AktivasiUsecase{aktivasi: mAkt, jawaban: mJawaban, pengerjaan: mPengerjaan}
	if _, err := uc.BukaTutupCourse(dto.BukaTutupCourseRequest{AktivasiCourseID: 9, IsOpen: false}); err != nil {
		t.Fatalf("tutup course tidak boleh error: %v", err)
	}

	mJawaban.AssertCalled(t, "MarkSubmittedForCourse", 5, 1)
	mPengerjaan.AssertCalled(t, "MarkSelesaiForCourse", 5, 1)
	mJawaban.AssertNotCalled(t, "UnmarkSubmittedForCourse", mock.Anything, mock.Anything)
	mPengerjaan.AssertNotCalled(t, "ResetForCourse", mock.Anything, mock.Anything)
}

// Course yang tidak ada -> ErrNotFound, tanpa menyentuh repo lain.
func TestBukaTutupCourse_CourseTidakAda(t *testing.T) {
	mAkt := &mocks.AktivasiRepository{}
	mAkt.On("FindCourseByID", 999).Return((*entity.AktivasiCourse)(nil), errNotFoundStub{})

	uc := &AktivasiUsecase{aktivasi: mAkt}
	if _, err := uc.BukaTutupCourse(dto.BukaTutupCourseRequest{AktivasiCourseID: 999, IsOpen: true}); err == nil {
		t.Fatal("course tidak ada harus error")
	}
}

type errNotFoundStub struct{}

func (errNotFoundStub) Error() string { return "not found" }
