package handler

import (
	"net/http"
	"strconv"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type GameHandler struct {
	uc *usecase.GameUsecase
}

func NewGameHandler(uc *usecase.GameUsecase) *GameHandler { return &GameHandler{uc: uc} }

// Konfigurasi GET /api/game/konfigurasi
// @Summary Konfigurasi Game
// @Tags Game
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=entity.GameKonfigurasi}
// @Router /game/konfigurasi [get]
func (h *GameHandler) Konfigurasi(c *gin.Context) {
	response.OK(c, http.StatusOK, "Konfigurasi game", h.uc.Konfigurasi())
}

// StatusMain GET /api/game/status
// @Summary Status Kuota Main
// @Description Apakah user masih boleh main minggu ini
// @Tags Game
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.StatusMainResponse}
// @Router /game/status [get]
func (h *GameHandler) StatusMain(c *gin.Context) {
	res, err := h.uc.StatusMain(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Status main", res)
}

// MulaiMain POST /api/game/mulai?bahasa=c&jumlah=5
// @Summary Mulai Bug Hunt
// @Description Mengambil soal acak TANPA kunci jawaban
// @Tags Game
// @Security bearerAuth
// @Produce json
// @Param bahasa query string true "c atau python"
// @Param jumlah query int false "jumlah soal"
// @Success 200 {object} response.Envelope{data=dto.MulaiMainResponse}
// @Router /game/mulai [post]
func (h *GameHandler) MulaiMain(c *gin.Context) {
	jumlah, _ := strconv.Atoi(c.DefaultQuery("jumlah", "5"))
	res, err := h.uc.MulaiMain(middleware.UserID(c), c.Query("bahasa"), jumlah)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Soal siap", res)
}

// SelesaiMain POST /api/game/selesai
// @Summary Kirim Jawaban Bug Hunt
// @Description Client mengirim JAWABAN, bukan skor. Penilaian dan XP dihitung server.
// @Tags Game
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.SelesaiMainRequest true "Jawaban"
// @Success 200 {object} response.Envelope{data=dto.SelesaiMainResponse}
// @Router /game/selesai [post]
func (h *GameHandler) SelesaiMain(c *gin.Context) {
	var req dto.SelesaiMainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.SelesaiMain(middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Hasil permainan", res)
}

// Peringkat GET /api/game/peringkat?limit=10&kelas_id=
// @Summary Papan Peringkat
// @Tags Game
// @Security bearerAuth
// @Produce json
// @Param limit query int false "jumlah baris"
// @Param kelas_id query int false "saring per kelas"
// @Success 200 {object} response.Envelope
// @Router /game/peringkat [get]
func (h *GameHandler) Peringkat(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	kelasID := queryIntPtr(c, "kelas_id")
	baris, err := h.uc.Peringkat(limit, kelasID)
	if err != nil {
		mapError(c, err)
		return
	}
	posisi, err := h.uc.PeringkatSaya(middleware.UserID(c), kelasID)
	if err != nil {
		posisi = 0
	}
	response.OK(c, http.StatusOK, "Papan peringkat", gin.H{
		"peringkat":    baris,
		"posisi_saya": posisi,
	})
}

// Detak POST /api/game/detak
// @Summary Heartbeat Sesi Aktif
// @Description Menandai user sedang online. Identitas diambil dari token.
// @Tags Game
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.DetakRequest false "Aktivitas"
// @Success 200 {object} response.Envelope
// @Router /game/detak [post]
func (h *GameHandler) Detak(c *gin.Context) {
	var req dto.DetakRequest
	_ = c.ShouldBindJSON(&req)
	if err := h.uc.Detak(middleware.UserID(c), req.Aktivitas); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Detak tercatat", nil)
}

// ---- Admin ----

// SesiAktif GET /api/admin/monitoring/sesi
// @Summary Daftar Sesi Aktif
// @Description User yang berdetak dalam 2 menit terakhir
// @Tags Admin - Monitoring
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /admin/monitoring/sesi [get]
func (h *GameHandler) SesiAktif(c *gin.Context) {
	res, err := h.uc.SesiAktif()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Sesi aktif", res)
}

// AdminListSoal GET /api/admin/game/soal?bahasa=
// @Summary Daftar Soal Game (dengan kunci jawaban)
// @Tags Admin - Game
// @Security bearerAuth
// @Produce json
// @Param bahasa query string false "c atau python"
// @Success 200 {object} response.Envelope{data=[]entity.GameSoal}
// @Router /admin/game/soal [get]
func (h *GameHandler) AdminListSoal(c *gin.Context) {
	res, err := h.uc.AdminListSoal(c.Query("bahasa"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar soal game", res)
}

// AdminCreateSoal POST /api/admin/game/soal
// @Summary Tambah Soal Game
// @Tags Admin - Game
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.GameSoalRequest true "Payload"
// @Success 201 {object} response.Envelope{data=entity.GameSoal}
// @Router /admin/game/soal [post]
func (h *GameHandler) AdminCreateSoal(c *gin.Context) {
	var req dto.GameSoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.AdminCreateSoal(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Soal dibuat", res)
}

// AdminUpdateSoal PUT /api/admin/game/soal/:id
// @Summary Perbarui Soal Game
// @Tags Admin - Game
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Soal"
// @Param request body dto.GameSoalRequest true "Payload"
// @Success 200 {object} response.Envelope{data=entity.GameSoal}
// @Router /admin/game/soal/{id} [put]
func (h *GameHandler) AdminUpdateSoal(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.GameSoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.AdminUpdateSoal(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Soal diperbarui", res)
}

// AdminDeleteSoal DELETE /api/admin/game/soal/:id
// @Summary Hapus Soal Game
// @Tags Admin - Game
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Soal"
// @Success 200 {object} response.Envelope
// @Router /admin/game/soal/{id} [delete]
func (h *GameHandler) AdminDeleteSoal(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.AdminDeleteSoal(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Soal dihapus", nil)
}

// AdminSimpanKonfigurasi PUT /api/admin/game/konfigurasi
// @Summary Ubah Konfigurasi Game
// @Tags Admin - Game
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.GameKonfigurasiRequest true "Payload"
// @Success 200 {object} response.Envelope{data=entity.GameKonfigurasi}
// @Router /admin/game/konfigurasi [put]
func (h *GameHandler) AdminSimpanKonfigurasi(c *gin.Context) {
	var req dto.GameKonfigurasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.SimpanKonfigurasi(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Konfigurasi disimpan", res)
}
