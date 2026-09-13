package handler

import (
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type BelajarHandler struct {
	uc *usecase.BelajarUsecase
}

func NewBelajarHandler(uc *usecase.BelajarUsecase) *BelajarHandler {
	return &BelajarHandler{uc: uc}
}

// ListLevel GET /api/belajar/level
// @Summary Daftar Level
// @Description Mengambil daftar level kurikulum elearning
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=[]entity.Level}
// @Router /belajar/level [get]
func (h *BelajarHandler) ListLevel(c *gin.Context) {
	res, err := h.uc.ListLevel()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar level", res)
}

// ListModul GET /api/belajar/modul?level_id=
// @Summary Daftar Modul
// @Description Mengambil daftar modul, opsional disaring per level
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Param level_id query string false "ID Level"
// @Success 200 {object} response.Envelope{data=[]entity.Modul}
// @Router /belajar/modul [get]
func (h *BelajarHandler) ListModul(c *gin.Context) {
	res, err := h.uc.ListModul(c.Query("level_id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar modul", res)
}

// ListMateri GET /api/belajar/materi?modul_id=
// @Summary Daftar Materi (tanpa kunci jawaban)
// @Description Materi untuk mahasiswa. Kolom solusi TIDAK disertakan.
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Param modul_id query string false "ID Modul"
// @Success 200 {object} response.Envelope{data=[]entity.MateriMahasiswa}
// @Router /belajar/materi [get]
func (h *BelajarHandler) ListMateri(c *gin.Context) {
	res, err := h.uc.ListMateriMahasiswa(c.Query("modul_id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar materi", res)
}

// GetMateri GET /api/belajar/materi/:id
// @Summary Detail Materi (tanpa kunci jawaban)
// @Description Detail materi untuk mahasiswa. Kolom solusi TIDAK disertakan.
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Param id path string true "ID Materi"
// @Success 200 {object} response.Envelope{data=entity.MateriMahasiswa}
// @Router /belajar/materi/{id} [get]
func (h *BelajarHandler) GetMateri(c *gin.Context) {
	res, err := h.uc.GetMateriMahasiswa(c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Detail materi", res)
}

// ProgresSaya GET /api/belajar/progres
// @Summary Progres Belajar Saya
// @Description Progres materi + profil XP/level milik user yang sedang login
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.ProgresSayaResponse}
// @Router /belajar/progres [get]
func (h *BelajarHandler) ProgresSaya(c *gin.Context) {
	res, err := h.uc.ProgresSaya(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Progres belajar", res)
}

// SelesaikanMateri POST /api/belajar/materi/:id/selesai
// @Summary Tandai Materi Selesai
// @Description Menandai materi selesai dan menambah XP. XP tidak dobel jika diulang.
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Param id path string true "ID Materi"
// @Success 200 {object} response.Envelope{data=entity.ProfilBelajar}
// @Router /belajar/materi/{id}/selesai [post]
func (h *BelajarHandler) SelesaikanMateri(c *gin.Context) {
	res, err := h.uc.SelesaikanMateri(middleware.UserID(c), c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Materi ditandai selesai", res)
}

// ---- Admin: materi berikut kunci jawaban ----

// AdminListMateri GET /api/admin/materi?modul_id=
// @Summary Daftar Materi (dengan kunci jawaban)
// @Description Materi lengkap termasuk kolom solusi. Khusus asisten & koordinator.
// @Tags Admin - Materi
// @Security bearerAuth
// @Produce json
// @Param modul_id query string false "ID Modul"
// @Success 200 {object} response.Envelope{data=[]entity.Materi}
// @Router /admin/materi [get]
func (h *BelajarHandler) AdminListMateri(c *gin.Context) {
	res, err := h.uc.ListMateri(c.Query("modul_id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar materi", res)
}

// AdminGetMateri GET /api/admin/materi/:id
// @Summary Detail Materi (dengan kunci jawaban)
// @Tags Admin - Materi
// @Security bearerAuth
// @Produce json
// @Param id path string true "ID Materi"
// @Success 200 {object} response.Envelope{data=entity.Materi}
// @Router /admin/materi/{id} [get]
func (h *BelajarHandler) AdminGetMateri(c *gin.Context) {
	res, err := h.uc.GetMateri(c.Param("id"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Detail materi", res)
}

// AdminCreateMateri POST /api/admin/materi
// @Summary Tambah Materi
// @Tags Admin - Materi
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.MateriRequest true "Payload"
// @Success 201 {object} response.Envelope{data=entity.Materi}
// @Router /admin/materi [post]
func (h *BelajarHandler) AdminCreateMateri(c *gin.Context) {
	var req dto.MateriRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.CreateMateri(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Materi dibuat", res)
}

// AdminUpdateMateri PUT /api/admin/materi/:id
// @Summary Perbarui Materi
// @Tags Admin - Materi
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path string true "ID Materi"
// @Param request body dto.MateriRequest true "Payload"
// @Success 200 {object} response.Envelope{data=entity.Materi}
// @Router /admin/materi/{id} [put]
func (h *BelajarHandler) AdminUpdateMateri(c *gin.Context) {
	var req dto.MateriRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.UpdateMateri(c.Param("id"), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Materi diperbarui", res)
}

// AdminDeleteMateri DELETE /api/admin/materi/:id
// @Summary Hapus Materi
// @Tags Admin - Materi
// @Security bearerAuth
// @Produce json
// @Param id path string true "ID Materi"
// @Success 200 {object} response.Envelope
// @Router /admin/materi/{id} [delete]
func (h *BelajarHandler) AdminDeleteMateri(c *gin.Context) {
	if err := h.uc.DeleteMateri(c.Param("id")); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Materi dihapus", nil)
}

// PencapaianSaya GET /api/belajar/pencapaian
// @Summary Pencapaian Saya
// @Description Semua pencapaian + yang sudah terbuka. Server mengevaluasi
// @Description apakah ada yang baru terpenuhi, client tidak bisa menentukan.
// @Tags Belajar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.PencapaianSayaResponse}
// @Router /belajar/pencapaian [get]
func (h *BelajarHandler) PencapaianSaya(c *gin.Context) {
	res, err := h.uc.PencapaianSaya(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Pencapaian", res)
}
