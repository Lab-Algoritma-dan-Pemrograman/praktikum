package usecase

import (
	"errors"
	"testing"
	"time"

	"lab-ap/internal/entity"
	"lab-ap/internal/repository/mocks"

	"github.com/stretchr/testify/mock"
)

func ptrTime(t time.Time) *time.Time { return &t }
func ptrStr(s string) *string        { return &s }

// TestFaseWindow memverifikasi aturan waktu server-authoritative.
func TestFaseWindow(t *testing.T) {
	deadline := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	late := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	// Tanpa buka_at (langsung dibuka) dan tanpa late_until.
	t1 := &entity.LaporanBesarTarget{DeadlineAt: deadline}
	if got := faseWindow(t1, deadline.Add(-time.Hour)); got != FaseDibuka {
		t.Fatalf("sebelum deadline harus dibuka, dapat %s", got)
	}
	if got := faseWindow(t1, deadline); got != FaseDitutup {
		t.Fatalf("tepat deadline tanpa late harus ditutup, dapat %s", got)
	}
	if got := faseWindow(t1, deadline.Add(time.Second)); got != FaseDitutup {
		t.Fatalf("lewat deadline tanpa late harus ditutup, dapat %s", got)
	}

	// Dengan buka_at di masa depan.
	buka := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	t2 := &entity.LaporanBesarTarget{BukaAt: &buka, DeadlineAt: deadline}
	if got := faseWindow(t2, buka.Add(-time.Minute)); got != FaseBelumDibuka {
		t.Fatalf("sebelum buka_at harus belum_dibuka, dapat %s", got)
	}
	if got := faseWindow(t2, buka); got != FaseDibuka {
		t.Fatalf("tepat buka_at harus dibuka, dapat %s", got)
	}

	// Window telat.
	t3 := &entity.LaporanBesarTarget{BukaAt: &buka, DeadlineAt: deadline, LateUntil: &late}
	if got := faseWindow(t3, deadline.Add(time.Hour)); got != FaseTelat {
		t.Fatalf("setelah deadline & sebelum late_until harus telat, dapat %s", got)
	}
	if got := faseWindow(t3, late); got != FaseTelat {
		t.Fatalf("tepat late_until harus telat, dapat %s", got)
	}
	if got := faseWindow(t3, late.Add(time.Second)); got != FaseDitutup {
		t.Fatalf("lewat late_until harus ditutup, dapat %s", got)
	}
}

// TestCekGerbangLaporanBesar: gerbang ujian praktik.
func TestCekGerbangLaporanBesar(t *testing.T) {
	kelasID := 2

	// Tanpa kelas → gerbang tidak berlaku.
	uc := &LaporanBesarUsecase{repo: &mocks.LaporanBesarRepository{}}
	if err := uc.CekGerbangLaporanBesar(7, nil, nil); err != nil {
		t.Fatalf("tanpa kelas harus lolos, dapat %v", err)
	}

	// Tidak ada laporan besar aktif → gerbang tidak berlaku.
	mNoAktif := &mocks.LaporanBesarRepository{}
	mNoAktif.On("FindAktif").Return(nil, errors.New("record not found"))
	ucNo := &LaporanBesarUsecase{repo: mNoAktif}
	if err := ucNo.CekGerbangLaporanBesar(7, &kelasID, nil); err != nil {
		t.Fatalf("tanpa laporan aktif harus lolos, dapat %v", err)
	}

	// Laporan aktif + kelas ditarget + pengumpulan terkumpul → lolos.
	mTerkumpul := &mocks.LaporanBesarRepository{}
	mTerkumpul.On("FindAktif").Return(&entity.LaporanBesar{ID: 1, IsAktif: true}, nil)
	mTerkumpul.On("FindTarget", 1, 2, mock.Anything).Return(&entity.LaporanBesarTarget{ID: 9, LaporanBesarID: 1, KelasID: 2}, nil)
	mTerkumpul.On("FindPengumpulan", 1, 7).Return(
		&entity.PengumpulanLaporanBesar{Status: string(entity.LaporanTerkumpul)}, nil)
	ucTerkumpul := &LaporanBesarUsecase{repo: mTerkumpul}
	if err := ucTerkumpul.CekGerbangLaporanBesar(7, &kelasID, nil); err != nil {
		t.Fatalf("terkumpul harus lolos, dapat %v", err)
	}

	// Laporan aktif + kelas ditarget + belum + TANPA override → ditolak ErrBadRequest.
	mBelum := &mocks.LaporanBesarRepository{}
	mBelum.On("FindAktif").Return(&entity.LaporanBesar{ID: 1, IsAktif: true}, nil)
	mBelum.On("FindTarget", 1, 2, mock.Anything).Return(&entity.LaporanBesarTarget{ID: 9, LaporanBesarID: 1, KelasID: 2}, nil)
	mBelum.On("FindPengumpulan", 1, 7).Return(&entity.PengumpulanLaporanBesar{Status: string(entity.LaporanBelum)}, nil)
	ucBelum := &LaporanBesarUsecase{repo: mBelum}
	err := ucBelum.CekGerbangLaporanBesar(7, &kelasID, nil)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("belum dikumpulkan harus ErrBadRequest, dapat %v", err)
	}

	// Belum ada baris pengumpulan → juga ditolak.
	mTanpaBaris := &mocks.LaporanBesarRepository{}
	mTanpaBaris.On("FindAktif").Return(&entity.LaporanBesar{ID: 1, IsAktif: true}, nil)
	mTanpaBaris.On("FindTarget", 1, 2, mock.Anything).Return(&entity.LaporanBesarTarget{ID: 9, LaporanBesarID: 1, KelasID: 2}, nil)
	mTanpaBaris.On("FindPengumpulan", 1, 7).Return(nil, errors.New("record not found"))
	ucTanpaBaris := &LaporanBesarUsecase{repo: mTanpaBaris}
	if err := ucTanpaBaris.CekGerbangLaporanBesar(7, &kelasID, nil); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("tanpa baris harus ErrBadRequest, dapat %v", err)
	}

	// Belum dikumpulkan TAPI ada override → lolos.
	now := time.Now()
	mOverride := &mocks.LaporanBesarRepository{}
	mOverride.On("FindAktif").Return(&entity.LaporanBesar{ID: 1, IsAktif: true}, nil)
	mOverride.On("FindTarget", 1, 2, mock.Anything).Return(&entity.LaporanBesarTarget{ID: 9, LaporanBesarID: 1, KelasID: 2}, nil)
	mOverride.On("FindPengumpulan", 1, 7).Return(
		&entity.PengumpulanLaporanBesar{Status: string(entity.LaporanBelum), OverridePada: &now}, nil)
	ucOverride := &LaporanBesarUsecase{repo: mOverride}
	if err := ucOverride.CekGerbangLaporanBesar(7, &kelasID, nil); err != nil {
		t.Fatalf("ada override harus lolos, dapat %v", err)
	}

	// Laporan aktif tapi kelas ini tidak ditarget → gerbang tidak berlaku.
	mTanpaTarget := &mocks.LaporanBesarRepository{}
	mTanpaTarget.On("FindAktif").Return(&entity.LaporanBesar{ID: 1, IsAktif: true}, nil)
	mTanpaTarget.On("FindTarget", 1, 2, mock.Anything).Return(nil, errors.New("record not found"))
	ucTanpaTarget := &LaporanBesarUsecase{repo: mTanpaTarget}
	if err := ucTanpaTarget.CekGerbangLaporanBesar(7, &kelasID, nil); err != nil {
		t.Fatalf("kelas tanpa target harus lolos, dapat %v", err)
	}
}

// TestSignedBerkasPemilik memverifikasi matrix akses berkas privat.
func TestSignedBerkasPemilik(t *testing.T) {
	m := &mocks.LaporanBesarRepository{}
	uc := &LaporanBesarUsecase{repo: m}

	// Mahasiswa lain (pemintaID != mhsID) → ditolak, tanpa menyentuh repo.
	if _, _, err := uc.SignedBerkas(1, 7, 8, "laporan", false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("mahasiswa lain harus ErrForbidden, dapat %v", err)
	}

	// Jenis tidak valid → ErrBadRequest.
	if _, _, err := uc.SignedBerkas(1, 7, 0, "salah", false); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("jenis invalid harus ErrBadRequest, dapat %v", err)
	}
}
