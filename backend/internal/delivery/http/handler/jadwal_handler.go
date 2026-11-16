package handler

import (
	"net/http"

	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type JadwalHandler struct {
	uc *usecase.JadwalUsecase
}

func NewJadwalHandler(uc *usecase.JadwalUsecase) *JadwalHandler { return &JadwalHandler{uc: uc} }

// List GET /api/info/jadwal & /api/admin/jadwal
//
//	@Summary		Daftar Jadwal Praktikum
//	@Description	Mengambil daftar jadwal praktikum laboratorium
//	@Tags			Info, Admin - Jadwal
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]entity.Jadwal}
//	@Router			/info/jadwal [get]
//	@Router			/admin/jadwal [get]
func (h *JadwalHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar jadwal", res)
}

// Create POST /api/admin/jadwal
//
//	@Summary		Buat Jadwal Baru
//	@Description	Menambahkan jadwal praktikum baru (Admin)
//	@Tags			Admin - Jadwal
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.JadwalRequest	true	"Payload Jadwal"
//	@Success		201		{object}	response.Envelope{data=entity.Jadwal}
//	@Router			/admin/jadwal [post]
func (h *JadwalHandler) Create(c *gin.Context) {
	var req dto.JadwalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Jadwal dibuat", res)
}

// Update PUT /api/admin/jadwal/:id
//
//	@Summary		Perbarui Jadwal
//	@Description	Mengubah data jadwal praktikum yang ada (Admin)
//	@Tags			Admin - Jadwal
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"ID Jadwal"
//	@Param			request	body		dto.JadwalRequest	true	"Payload Jadwal"
//	@Success		200		{object}	response.Envelope{data=entity.Jadwal}
//	@Router			/admin/jadwal/{id} [put]
func (h *JadwalHandler) Update(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.JadwalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Update(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Jadwal diperbarui", res)
}

// Delete DELETE /api/admin/jadwal/:id
//
//	@Summary		Hapus Jadwal
//	@Description	Menghapus jadwal praktikum (Admin)
//	@Tags			Admin - Jadwal
//	@Security		bearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID Jadwal"
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/jadwal/{id} [delete]
func (h *JadwalHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Jadwal dihapus", nil)
}
