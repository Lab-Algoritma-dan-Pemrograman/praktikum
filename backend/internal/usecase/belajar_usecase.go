package usecase

import (
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
)

type BelajarUsecase struct {
	repo repository.BelajarRepository
}

func NewBelajarUsecase(r repository.BelajarRepository) *BelajarUsecase {
	return &BelajarUsecase{repo: r}
}

func (uc *BelajarUsecase) ListLevel() ([]entity.Level, error) { return uc.repo.ListLevel() }

func (uc *BelajarUsecase) ListModul(levelID string) ([]entity.Modul, error) {
	return uc.repo.ListModul(levelID)
}

// ListMateriMahasiswa dipakai mahasiswa: kunci jawaban tidak ikut.
func (uc *BelajarUsecase) ListMateriMahasiswa(modulID string) ([]entity.MateriMahasiswa, error) {
	return uc.repo.ListMateriMahasiswa(modulID)
}

func (uc *BelajarUsecase) GetMateriMahasiswa(id string) (*entity.MateriMahasiswa, error) {
	m, err := uc.repo.FindMateriMahasiswa(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return m, nil
}

// ListMateri dipakai asisten/koordinator: termasuk kunci jawaban.
func (uc *BelajarUsecase) ListMateri(modulID string) ([]entity.Materi, error) {
	return uc.repo.ListMateri(modulID)
}

func (uc *BelajarUsecase) GetMateri(id string) (*entity.Materi, error) {
	m, err := uc.repo.FindMateri(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return m, nil
}

func (uc *BelajarUsecase) CreateMateri(req dto.MateriRequest) (*entity.Materi, error) {
	if req.ID == "" || req.ModulID == "" || req.Judul == "" {
		return nil, ErrBadRequest
	}
	m := &entity.Materi{
		ID:             req.ID,
		ModulID:        req.ModulID,
		Judul:          req.Judul,
		Penjelasan:     req.Penjelasan,
		ContohKode:     req.ContohKode,
		KodeAwal:       req.KodeAwal,
		Solusi:         req.Solusi,
		Petunjuk:       req.Petunjuk,
		Kuis:           req.Kuis,
		KasusUji:       req.KasusUji,
		AturanValidasi: req.AturanValidasi,
		Urutan:         req.Urutan,
		XPHadiah:       req.XPHadiah,
	}
	if err := uc.repo.CreateMateri(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *BelajarUsecase) UpdateMateri(id string, req dto.MateriRequest) (*entity.Materi, error) {
	m, err := uc.repo.FindMateri(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if req.ModulID != "" {
		m.ModulID = req.ModulID
	}
	if req.Judul != "" {
		m.Judul = req.Judul
	}
	m.Penjelasan = req.Penjelasan
	m.ContohKode = req.ContohKode
	m.KodeAwal = req.KodeAwal
	m.Solusi = req.Solusi
	m.Petunjuk = req.Petunjuk
	m.Kuis = req.Kuis
	m.KasusUji = req.KasusUji
	m.AturanValidasi = req.AturanValidasi
	m.Urutan = req.Urutan
	m.XPHadiah = req.XPHadiah
	if err := uc.repo.UpdateMateri(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *BelajarUsecase) DeleteMateri(id string) error {
	if _, err := uc.repo.FindMateri(id); err != nil {
		return ErrNotFound
	}
	return mapDeleteErr(uc.repo.DeleteMateri(id))
}

// ProgresSaya mengembalikan progres + profil xp/level milik user sendiri.
func (uc *BelajarUsecase) ProgresSaya(userID int) (*dto.ProgresSayaResponse, error) {
	progres, err := uc.repo.ListProgres(userID)
	if err != nil {
		return nil, err
	}
	out := &dto.ProgresSayaResponse{Progres: progres}
	if p, err := uc.repo.FindProfil(userID); err == nil {
		out.Profil = p
	}
	return out, nil
}

// SelesaikanMateri menandai materi selesai dan menambah XP sesuai xp_hadiah.
// XP hanya ditambah saat transisi belum-selesai -> selesai, supaya tak dobel.
func (uc *BelajarUsecase) SelesaikanMateri(userID int, materiID string) (*entity.ProfilBelajar, error) {
	m, err := uc.repo.FindMateri(materiID)
	if err != nil {
		return nil, ErrNotFound
	}

	sudah := false
	if list, err := uc.repo.ListProgres(userID); err == nil {
		for _, p := range list {
			if p.MateriID == materiID && p.Selesai {
				sudah = true
				break
			}
		}
	}

	now := time.Now()
	if err := uc.repo.TandaiSelesai(&entity.ProgresBelajar{
		UserID: userID, MateriID: materiID, Selesai: true, SelesaiPada: &now,
	}); err != nil {
		return nil, err
	}

	profil, err := uc.repo.FindProfil(userID)
	if err != nil {
		profil = &entity.ProfilBelajar{UserID: userID, LevelAngka: 1}
	}
	if !sudah {
		profil.XP += m.XPHadiah
	}
	profil.TerakhirAktif = &now
	if err := uc.repo.SimpanProfil(profil); err != nil {
		return nil, err
	}
	return profil, nil
}
