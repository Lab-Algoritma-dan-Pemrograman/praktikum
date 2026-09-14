package usecase

import (
	"encoding/json"
	"strconv"
	"time"

	"gorm.io/datatypes"

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

// PencapaianSaya mengembalikan semua pencapaian + yang sudah terbuka,
// sekaligus MENGEVALUASI apakah ada yang baru terpenuhi.
//
// Evaluasi dilakukan di server: sebelumnya frontend yang memutuskan lalu
// menulis sendiri ke DB, sehingga pencapaian bisa dibuka dengan memanggil
// insert langsung. Sekarang client tidak bisa menentukan apa pun.
func (uc *BelajarUsecase) PencapaianSaya(userID int) (*dto.PencapaianSayaResponse, error) {
	semua, err := uc.repo.ListPencapaian()
	if err != nil {
		return nil, err
	}
	terbuka, err := uc.repo.ListPencapaianTerbuka(userID)
	if err != nil {
		return nil, err
	}
	sudah := make(map[string]bool, len(terbuka))
	for _, t := range terbuka {
		sudah[t.PencapaianID] = true
	}

	profil, err := uc.repo.FindProfil(userID)
	if err != nil {
		profil = &entity.ProfilBelajar{UserID: userID, LevelAngka: 1}
	}
	jumlahSelesai, _ := uc.repo.HitungMateriSelesai(userID)

	// Level yang sudah tuntas: semua materi di level itu selesai.
	levelTuntas := uc.levelTuntas(userID)

	baru := []entity.Pencapaian{}
	for _, p := range semua {
		if sudah[p.ID] {
			continue
		}
		if !uc.syaratTerpenuhi(p, profil, jumlahSelesai, levelTuntas) {
			continue
		}
		if err := uc.repo.BukaPencapaian(userID, p.ID); err != nil {
			continue
		}
		sudah[p.ID] = true
		baru = append(baru, p)
	}

	ids := make([]string, 0, len(sudah))
	for id := range sudah {
		ids = append(ids, id)
	}
	return &dto.PencapaianSayaResponse{Semua: semua, TerbukaID: ids, BaruSaja: baru}, nil
}

// syaratTerpenuhi mengevaluasi satu pencapaian. syarat_nilai bertipe text
// karena sumbernya campur angka ("1000") dan id level ("c-level-1").
func (uc *BelajarUsecase) syaratTerpenuhi(
	p entity.Pencapaian, profil *entity.ProfilBelajar,
	jumlahSelesai int64, levelTuntas map[string]bool,
) bool {
	switch p.SyaratTipe {
	case entity.SyaratLevelSelesai:
		return levelTuntas[p.SyaratNilai]
	case entity.SyaratXP:
		n, err := strconv.Atoi(p.SyaratNilai)
		return err == nil && profil.XP >= n
	case entity.SyaratStreak:
		n, err := strconv.Atoi(p.SyaratNilai)
		return err == nil && profil.Streak >= n
	case entity.SyaratJumlahMateri:
		n, err := strconv.Atoi(p.SyaratNilai)
		return err == nil && int(jumlahSelesai) >= n
	default:
		// SyaratSkorGame belum dievaluasi: butuh agregat riwayat game.
		return false
	}
}

// levelTuntas: level dianggap tuntas bila semua materinya sudah selesai.
func (uc *BelajarUsecase) levelTuntas(userID int) map[string]bool {
	out := map[string]bool{}
	levels, err := uc.repo.ListLevel()
	if err != nil {
		return out
	}
	progres, err := uc.repo.ListProgres(userID)
	if err != nil {
		return out
	}
	selesai := map[string]bool{}
	for _, p := range progres {
		if p.Selesai {
			selesai[p.MateriID] = true
		}
	}
	for _, lv := range levels {
		moduls, err := uc.repo.ListModul(lv.ID)
		if err != nil || len(moduls) == 0 {
			continue
		}
		total, tuntas := 0, 0
		for _, m := range moduls {
			materi, err := uc.repo.ListMateri(m.ID)
			if err != nil {
				continue
			}
			for _, mt := range materi {
				total++
				if selesai[mt.ID] {
					tuntas++
				}
			}
		}
		if total > 0 && total == tuntas {
			out[lv.ID] = true
		}
	}
	return out
}

// ResetProgres menghapus progres belajar seorang mahasiswa.
// levelID kosong = seluruh level. XP dihitung ulang dari materi yang tersisa,
// bukan dikurangi angka tetap, supaya tidak pernah meleset.
func (uc *BelajarUsecase) ResetProgres(userID int, levelID string) error {
	if levelID == "" {
		if err := uc.repo.HapusSemuaProgres(userID); err != nil {
			return err
		}
		if err := uc.repo.HapusPencapaianTerbuka(userID); err != nil {
			return err
		}
	} else {
		materiIDs, err := uc.materiDiLevel(levelID)
		if err != nil {
			return err
		}
		if _, err := uc.repo.HapusProgres(userID, materiIDs); err != nil {
			return err
		}
	}
	return uc.hitungUlangXP(userID)
}

// SetXP menetapkan XP secara manual (untuk koreksi asisten).
func (uc *BelajarUsecase) SetXP(userID, xp int) error {
	if xp < 0 {
		xp = 0
	}
	profil, err := uc.repo.FindProfil(userID)
	if err != nil {
		profil = &entity.ProfilBelajar{UserID: userID, LevelAngka: 1}
	}
	profil.XP = xp
	return uc.repo.SimpanProfil(profil)
}

func (uc *BelajarUsecase) materiDiLevel(levelID string) ([]string, error) {
	moduls, err := uc.repo.ListModul(levelID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range moduls {
		materi, err := uc.repo.ListMateri(m.ID)
		if err != nil {
			return nil, err
		}
		for _, mt := range materi {
			ids = append(ids, mt.ID)
		}
	}
	return ids, nil
}

// hitungUlangXP menjumlahkan ulang XP dari materi yang benar-benar selesai
// ditambah XP dari riwayat game, supaya angka selalu cocok dengan kenyataan.
func (uc *BelajarUsecase) hitungUlangXP(userID int) error {
	xpMateri, err := uc.repo.TotalXPMateriSelesai(userID)
	if err != nil {
		return err
	}
	total := xpMateri
	xpGame, err := uc.repo.TotalXPGame(userID)
	if err == nil {
		total += xpGame
	}
	profil, err := uc.repo.FindProfil(userID)
	if err != nil {
		profil = &entity.ProfilBelajar{UserID: userID, LevelAngka: 1}
	}
	profil.XP = total
	return uc.repo.SimpanProfil(profil)
}

// SesiBelajar menyiapkan seluruh data awal sesi elearning dalam satu panggilan:
// identitas, profil belajar, dan materi yang sudah selesai.
func (uc *BelajarUsecase) SesiBelajar(userID int) (*dto.SesiBelajarResponse, error) {
	b, err := uc.repo.SesiBelajar(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	progres, err := uc.repo.ListProgres(userID)
	if err != nil {
		return nil, err
	}
	selesai := make([]string, 0, len(progres))
	for _, p := range progres {
		if p.Selesai {
			selesai = append(selesai, p.MateriID)
		}
	}

	var akses interface{} = map[string]string{}
	if len(b.AksesLevel) > 0 {
		var tmp map[string]string
		if json.Unmarshal(b.AksesLevel, &tmp) == nil {
			akses = tmp
		}
	}

	return &dto.SesiBelajarResponse{
		UserID: b.UserID, NIM: b.NIM, Nama: b.Nama, Kelas: b.Kelas,
		Email: b.Email, FotoURL: b.FotoURL, Role: b.Role,
		XP: b.XP, Level: b.Level, Streak: b.Streak,
		AksesLevel: akses, MateriSelesai: selesai,
	}, nil
}

// ListUserBelajar daftar mahasiswa beserta profil belajarnya (layar admin).
func (uc *BelajarUsecase) ListUserBelajar() ([]dto.SesiBelajarResponse, error) {
	rows, err := uc.repo.ListUserBelajar()
	if err != nil {
		return nil, err
	}
	out := make([]dto.SesiBelajarResponse, 0, len(rows))
	for _, b := range rows {
		var akses interface{} = map[string]string{}
		if len(b.AksesLevel) > 0 {
			var tmp map[string]string
			if json.Unmarshal(b.AksesLevel, &tmp) == nil {
				akses = tmp
			}
		}
		out = append(out, dto.SesiBelajarResponse{
			UserID: b.UserID, NIM: b.NIM, Nama: b.Nama, Kelas: b.Kelas,
			Email: b.Email, FotoURL: b.FotoURL, Role: b.Role,
			XP: b.XP, Level: b.Level, Streak: b.Streak, AksesLevel: akses,
		})
	}
	return out, nil
}

// ProgresUser progres belajar milik mahasiswa tertentu (layar admin).
func (uc *BelajarUsecase) ProgresUser(userID int) ([]entity.ProgresBelajar, error) {
	return uc.repo.ListProgres(userID)
}

// SetAksesLevel menyetel akses level satu mahasiswa. Nilai yang diterima
// hanya auto/unlocked/locked supaya tidak ada nilai liar masuk basis data.
func (uc *BelajarUsecase) SetAksesLevel(userID int, akses map[string]string) error {
	bersih := map[string]string{}
	for k, v := range akses {
		if v == "auto" || v == "unlocked" || v == "locked" {
			bersih[k] = v
		}
	}
	b, err := json.Marshal(bersih)
	if err != nil {
		return err
	}
	return uc.repo.SetAksesLevel(userID, b)
}

// SimpanKurikulum menyimpan seluruh struktur kurikulum sekaligus.
// Urutan level/modul/materi diambil dari urutan dalam permintaan.
func (uc *BelajarUsecase) SimpanKurikulum(req dto.SimpanKurikulumRequest) error {
	var levels []entity.Level
	var moduls []entity.Modul
	var materi []entity.Materi

	for li, l := range req.Level {
		mode := l.ModeAkses
		if mode != "auto" && mode != "unlocked" && mode != "locked" {
			mode = "auto"
		}
		levels = append(levels, entity.Level{
			ID: l.ID, Judul: l.Judul, Deskripsi: l.Deskripsi,
			ModeAkses: mode, Terkunci: l.Terkunci, Urutan: li,
		})
		for mi, m := range l.Modul {
			moduls = append(moduls, entity.Modul{
				ID: m.ID, LevelID: l.ID, Judul: m.Judul, Urutan: mi,
			})
			for ti, t := range m.Materi {
				materi = append(materi, entity.Materi{
					ID: t.ID, ModulID: m.ID, Judul: t.Judul,
					Penjelasan: t.Penjelasan, ContohKode: t.ContohKode,
					KodeAwal: t.KodeAwal, Solusi: t.Solusi, Petunjuk: t.Petunjuk,
					Kuis:           keJSON(t.Kuis),
					KasusUji:       keJSON(t.KasusUji),
					AturanValidasi: keJSON(t.AturanValidasi),
					Urutan:         ti,
					XPHadiah:       t.XPHadiah,
				})
			}
		}
	}
	return uc.repo.SimpanKurikulum(levels, moduls, materi)
}

// KosongkanKurikulum menghapus seluruh kurikulum beserta progres yang menempel.
func (uc *BelajarUsecase) KosongkanKurikulum() error {
	return uc.repo.KosongkanKurikulum()
}

func keJSON(v interface{}) datatypes.JSON {
	if v == nil {
		return datatypes.JSON([]byte("null"))
	}
	b, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte("null"))
	}
	return datatypes.JSON(b)
}
