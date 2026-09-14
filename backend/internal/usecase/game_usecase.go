package usecase

import (
	"time"

	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
)

type GameUsecase struct {
	repo    repository.GameRepository
	belajar repository.BelajarRepository
}

func NewGameUsecase(r repository.GameRepository, b repository.BelajarRepository) *GameUsecase {
	return &GameUsecase{repo: r, belajar: b}
}

// xpPerJawabanBenar: satu jawaban benar = 20 XP. Nilainya ditentukan server,
// bukan dikirim client, supaya skor tidak bisa dipalsukan.
const xpPerJawabanBenar = 20

// awalMinggu mengembalikan Senin 00:00 waktu lokal server.
func awalMinggu(t time.Time) time.Time {
	hari := int(t.Weekday())
	if hari == 0 { // Minggu dihitung akhir pekan, bukan awal
		hari = 7
	}
	senin := t.AddDate(0, 0, -(hari - 1))
	return time.Date(senin.Year(), senin.Month(), senin.Day(), 0, 0, 0, 0, senin.Location())
}

func (uc *GameUsecase) konfigurasiAtauDefault() *entity.GameKonfigurasi {
	k, err := uc.repo.Konfigurasi()
	if err != nil {
		return &entity.GameKonfigurasi{
			ID: "default", BugHuntAktif: true, BugHuntCAktif: true,
			BugHuntPyAktif: true, BatasMingguan: 3,
		}
	}
	return k
}

func (uc *GameUsecase) Konfigurasi() *entity.GameKonfigurasi { return uc.konfigurasiAtauDefault() }

func (uc *GameUsecase) SimpanKonfigurasi(req dto.GameKonfigurasiRequest) (*entity.GameKonfigurasi, error) {
	k := uc.konfigurasiAtauDefault()
	k.ID = "default"
	if req.BugHuntAktif != nil {
		k.BugHuntAktif = *req.BugHuntAktif
	}
	if req.BugHuntCAktif != nil {
		k.BugHuntCAktif = *req.BugHuntCAktif
	}
	if req.BugHuntPyAktif != nil {
		k.BugHuntPyAktif = *req.BugHuntPyAktif
	}
	if req.BatasMingguan != nil {
		k.BatasMingguan = *req.BatasMingguan
	}
	if err := uc.repo.SimpanKonfigurasi(k); err != nil {
		return nil, err
	}
	return k, nil
}

// StatusMain memberi tahu apakah user masih boleh main minggu ini.
func (uc *GameUsecase) StatusMain(userID int) (*dto.StatusMainResponse, error) {
	k := uc.konfigurasiAtauDefault()
	if !k.BugHuntAktif {
		return &dto.StatusMainResponse{Boleh: false, Terpakai: 0, Batas: 0, Alasan: "Game sedang dinonaktifkan"}, nil
	}
	if k.BatasMingguan <= 0 {
		return &dto.StatusMainResponse{Boleh: true, Terpakai: 0, Batas: 0}, nil
	}
	n, err := uc.repo.HitungMainSejak(userID, "bug_hunt", awalMinggu(time.Now()))
	if err != nil {
		return nil, err
	}
	res := &dto.StatusMainResponse{Terpakai: int(n), Batas: k.BatasMingguan, Boleh: int(n) < k.BatasMingguan}
	if !res.Boleh {
		res.Alasan = "Batas main minggu ini sudah tercapai"
	}
	return res, nil
}

// MulaiMain mengembalikan soal TANPA kunci jawaban (baris_bug & penjelasan dibuang).
func (uc *GameUsecase) MulaiMain(userID int, bahasa string, jumlah int) (*dto.MulaiMainResponse, error) {
	if bahasa != "c" && bahasa != "python" {
		return nil, ErrBadRequest
	}
	k := uc.konfigurasiAtauDefault()
	if !k.BugHuntAktif ||
		(bahasa == "c" && !k.BugHuntCAktif) ||
		(bahasa == "python" && !k.BugHuntPyAktif) {
		return nil, ErrForbidden
	}
	st, err := uc.StatusMain(userID)
	if err != nil {
		return nil, err
	}
	if !st.Boleh {
		return nil, ErrForbidden
	}

	semua, err := uc.repo.ListSoal(bahasa)
	if err != nil {
		return nil, err
	}
	if jumlah <= 0 || jumlah > len(semua) {
		jumlah = len(semua)
	}
	// Acak stabil tanpa dependensi: pakai waktu sebagai sumber urutan.
	acak := make([]entity.GameSoal, len(semua))
	copy(acak, semua)
	n := len(acak)
	seed := int(time.Now().UnixNano() % 1000003)
	for i := n - 1; i > 0; i-- {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		j := seed % (i + 1)
		acak[i], acak[j] = acak[j], acak[i]
	}
	acak = acak[:jumlah]

	out := make([]entity.GameSoalMahasiswa, 0, len(acak))
	for _, s := range acak {
		out = append(out, entity.GameSoalMahasiswa{
			ID: s.ID, Bahasa: s.Bahasa, Kesulitan: s.Kesulitan, Judul: s.Judul, Kode: s.Kode,
		})
	}
	return &dto.MulaiMainResponse{Soal: out, Terpakai: st.Terpakai, Batas: st.Batas}, nil
}

// SelesaiMain menilai jawaban DI SERVER lalu menambah XP.
// Client mengirim jawaban, bukan skor -- skor palsu tidak mungkin.
func (uc *GameUsecase) SelesaiMain(userID int, req dto.SelesaiMainRequest) (*dto.SelesaiMainResponse, error) {
	if len(req.Jawaban) == 0 {
		return nil, ErrBadRequest
	}
	st, err := uc.StatusMain(userID)
	if err != nil {
		return nil, err
	}
	if !st.Boleh {
		return nil, ErrForbidden
	}

	benar := 0
	koreksi := make([]dto.KoreksiJawaban, 0, len(req.Jawaban))
	for _, j := range req.Jawaban {
		s, err := uc.repo.FindSoal(j.SoalID)
		if err != nil {
			return nil, ErrNotFound
		}
		ok := s.BarisBug == j.BarisDipilih
		if ok {
			benar++
		}
		koreksi = append(koreksi, dto.KoreksiJawaban{
			SoalID:     s.ID,
			Benar:      ok,
			BarisBug:   s.BarisBug,
			Penjelasan: s.Penjelasan,
		})
	}

	xp := benar * xpPerJawabanBenar
	if err := uc.repo.CatatRiwayat(&entity.GameRiwayat{
		UserID: userID, JenisGame: "bug_hunt", XPDidapat: xp, Dimainkan: time.Now(),
	}); err != nil {
		return nil, err
	}

	profil, err := uc.belajar.FindProfil(userID)
	if err != nil {
		profil = &entity.ProfilBelajar{UserID: userID, LevelAngka: 1}
	}
	profil.XP += xp
	now := time.Now()
	profil.TerakhirAktif = &now
	if err := uc.belajar.SimpanProfil(profil); err != nil {
		return nil, err
	}

	return &dto.SelesaiMainResponse{
		Benar: benar, Total: len(req.Jawaban), XPDidapat: xp,
		XPTotal: profil.XP, Koreksi: koreksi,
	}, nil
}

// ---- Admin: soal lengkap dengan kunci jawaban ----

func (uc *GameUsecase) AdminListSoal(bahasa string) ([]entity.GameSoal, error) {
	return uc.repo.ListSoal(bahasa)
}

func (uc *GameUsecase) AdminCreateSoal(req dto.GameSoalRequest) (*entity.GameSoal, error) {
	if req.Bahasa == "" || req.Judul == "" || req.Kode == "" {
		return nil, ErrBadRequest
	}
	s := &entity.GameSoal{
		Bahasa: req.Bahasa, Kesulitan: req.Kesulitan, Judul: req.Judul,
		Kode: req.Kode, BarisBug: req.BarisBug, Penjelasan: req.Penjelasan,
	}
	if err := uc.repo.CreateSoal(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *GameUsecase) AdminUpdateSoal(id int, req dto.GameSoalRequest) (*entity.GameSoal, error) {
	s, err := uc.repo.FindSoal(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if req.Bahasa != "" {
		s.Bahasa = req.Bahasa
	}
	if req.Kesulitan != "" {
		s.Kesulitan = req.Kesulitan
	}
	if req.Judul != "" {
		s.Judul = req.Judul
	}
	if req.Kode != "" {
		s.Kode = req.Kode
	}
	if req.BarisBug > 0 {
		s.BarisBug = req.BarisBug
	}
	if req.Penjelasan != "" {
		s.Penjelasan = req.Penjelasan
	}
	if err := uc.repo.UpdateSoal(s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *GameUsecase) AdminDeleteSoal(id int) error {
	if _, err := uc.repo.FindSoal(id); err != nil {
		return ErrNotFound
	}
	return mapDeleteErr(uc.repo.DeleteSoal(id))
}

// ---- Peringkat ----

func (uc *GameUsecase) Peringkat(limit int, kelasID *int, namaKelas string) ([]repository.BarisPeringkat, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	return uc.repo.Peringkat(limit, kelasID, namaKelas)
}

func (uc *GameUsecase) PeringkatSaya(userID int, kelasID *int, namaKelas string) (int, error) {
	return uc.repo.PeringkatUser(userID, kelasID, namaKelas)
}

// ---- Monitoring ----

// Detak mencatat "saya sedang online". Identitas diambil dari token,
// bukan dari body, supaya tidak bisa mengaku jadi orang lain.
func (uc *GameUsecase) Detak(userID int, aktivitas string) error {
	if aktivitas == "" {
		aktivitas = "lesson"
	}
	return uc.repo.UpsertSesiAktif(userID, aktivitas)
}

// SesiAktif: yang berdetak dalam 2 menit terakhir dianggap online.
func (uc *GameUsecase) SesiAktif() ([]repository.BarisSesiAktif, error) {
	return uc.repo.ListSesiAktif(120)
}
