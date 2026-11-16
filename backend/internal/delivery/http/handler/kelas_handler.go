package handler

import (
	"net/http"

	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type KelasHandler struct {
	uc *usecase.KelasUsecase
}

func NewKelasHandler(uc *usecase.KelasUsecase) *KelasHandler { return &KelasHandler{uc: uc} }

// List GET /api/admin/kelas
//
//	@Summary		Daftar Kelas
//	@Description	Mengambil daftar kelas yang tersedia
//	@Tags			Admin - Kelas
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]entity.Kelas}
//	@Router			/admin/kelas [get]
func (h *KelasHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar kelas", res)
}

// Create POST /api/admin/kelas
//
//	@Summary		Tambah Kelas
//	@Description	Menambahkan kelas baru
//	@Tags			Admin - Kelas
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.KelasRequest	true	"Payload"
//	@Success		201		{object}	response.Envelope{data=entity.Kelas}
//	@Router			/admin/kelas [post]
func (h *KelasHandler) Create(c *gin.Context) {
	var req dto.KelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Kelas dibuat", res)
}

// Update PUT /api/admin/kelas/:id
//
//	@Summary		Perbarui Kelas
//	@Description	Mengubah data kelas
//	@Tags			Admin - Kelas
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"ID Kelas"
//	@Param			request	body		dto.KelasRequest	true	"Payload"
//	@Success		200		{object}	response.Envelope{data=entity.Kelas}
//	@Router			/admin/kelas/{id} [put]
func (h *KelasHandler) Update(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.KelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Update(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Kelas diperbarui", res)
}

// Delete DELETE /api/admin/kelas/:id
//
//	@Summary		Hapus Kelas
//	@Description	Menghapus kelas
//	@Tags			Admin - Kelas
//	@Security		bearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID Kelas"
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/kelas/{id} [delete]
func (h *KelasHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Kelas dihapus", nil)
}
