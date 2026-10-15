package usecase

import (
	"testing"

	"lab-ap/internal/entity"
)

func courseOf(jenis entity.JenisCourse) entity.Course {
	return entity.Course{ID: int(jenis[0]), Jenis: jenis}
}

func TestPilihCourseNormal_Reguler(t *testing.T) {
	courses := []entity.Course{
		courseOf(entity.CoursePretest),
		courseOf(entity.CoursePosttest),
		courseOf(entity.CourseKeterampilan),
	}

	// Gacha wajib bila sesi punya pre/post test.
	if _, err := pilihCourseNormal(courses, ""); err == nil {
		t.Fatal("sesi reguler tanpa gacha_pilihan harus error")
	}

	got, err := pilihCourseNormal(courses, "pretest")
	if err != nil || len(got) != 2 {
		t.Fatalf("pretest+keterampilan harap 2, dapat %d err=%v", len(got), err)
	}
	for _, c := range got {
		if c.Jenis != entity.CoursePretest && c.Jenis != entity.CourseKeterampilan {
			t.Fatalf("jenis tak terduga: %s", c.Jenis)
		}
	}

	got, err = pilihCourseNormal(courses, "posttest")
	if err != nil || len(got) != 2 {
		t.Fatalf("posttest+keterampilan harap 2, dapat %d err=%v", len(got), err)
	}
}

func TestPilihCourseNormal_Karyawan(t *testing.T) {
	// Sesi karyawan: hanya keterampilan — gacha tidak wajib.
	courses := []entity.Course{courseOf(entity.CourseKeterampilan)}

	got, err := pilihCourseNormal(courses, "")
	if err != nil || len(got) != 1 || got[0].Jenis != entity.CourseKeterampilan {
		t.Fatalf("harap 1 keterampilan tanpa error, dapat %v err=%v", got, err)
	}

	// Gacha terisi tapi course-nya tidak ada → diabaikan, keterampilan tetap dipakai.
	got, err = pilihCourseNormal(courses, "pretest")
	if err != nil || len(got) != 1 || got[0].Jenis != entity.CourseKeterampilan {
		t.Fatalf("gacha asing harus diabaikan, dapat %v err=%v", got, err)
	}
}
