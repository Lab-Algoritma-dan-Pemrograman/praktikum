package usecase

import (
	"errors"
	"testing"

	"lab-ap/internal/entity"
	"lab-ap/internal/repository/mocks"

	"github.com/stretchr/testify/mock"
)

// userKaryawan membangun user kelas karyawan (TE AD / TE AE) dengan
// relasi Kelas ter-preload seperti hasil FindByID (Preload "Kelas").
func userKaryawan(namaKelas string) *entity.User {
	kid, shift := 7, 1
	return &entity.User{
		ID:      99,
		KelasID: &kid,
		Shift:   &shift,
		Kelas:   &entity.Kelas{ID: 7, NamaKelas: namaKelas},
	}
}

func aktivasiTigaCourse() *entity.AktivasiSesi {
	mk := func(id int, jenis entity.JenisCourse) entity.AktivasiCourse {
		return entity.AktivasiCourse{
			ID:       id,
			CourseID: id,
			Course:   &entity.Course{ID: id, Jenis: jenis, DurasiMenit: 20},
		}
	}
	return &entity.AktivasiSesi{
		ID: 5,
		AktivasiCourses: []entity.AktivasiCourse{
			mk(1, entity.CoursePretest),
			mk(2, entity.CourseKeterampilan),
			mk(3, entity.CoursePosttest),
		},
	}
}

func TestBuildCourses_KaryawanSembunyiPrePost(t *testing.T) {
	mP := &mocks.PengerjaanRepository{}
	mP.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("tidak ada"))
	uc := &PraktikumUsecase{pengerjaan: mP}
	for _, nama := range []string{"TE AD", "TE AE"} {
		got := uc.buildCourses(99, userKaryawan(nama), aktivasiTigaCourse())
		if len(got) != 1 {
			t.Fatalf("%s: harap 1 course (keterampilan), dapat %d", nama, len(got))
		}
		if got[0].Jenis != string(entity.CourseKeterampilan) {
			t.Fatalf("%s: harap keterampilan, dapat %s", nama, got[0].Jenis)
		}
	}
}

func TestBuildCourses_RegulerTetapLengkap(t *testing.T) {
	mP := &mocks.PengerjaanRepository{}
	mP.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("tidak ada"))
	uc := &PraktikumUsecase{pengerjaan: mP}
	kid, shift := 3, 1
	reguler := &entity.User{
		ID:      11,
		KelasID: &kid,
		Shift:   &shift,
		Kelas:   &entity.Kelas{ID: 3, NamaKelas: "TE B"},
	}
	got := uc.buildCourses(11, reguler, aktivasiTigaCourse())
	if len(got) != 3 {
		t.Fatalf("kelas reguler harap 3 course, dapat %d", len(got))
	}
}

func TestBuildCourses_KelasNilTetapLengkap(t *testing.T) {
	// Kelas belum ter-preload (nil) → jangan sembunyikan apa pun.
	mP := &mocks.PengerjaanRepository{}
	mP.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("tidak ada"))
	uc := &PraktikumUsecase{pengerjaan: mP}
	kid, shift := 7, 1
	tanpaKelas := &entity.User{ID: 99, KelasID: &kid, Shift: &shift}
	got := uc.buildCourses(99, tanpaKelas, aktivasiTigaCourse())
	if len(got) != 3 {
		t.Fatalf("kelas nil harap 3 course, dapat %d", len(got))
	}
}
