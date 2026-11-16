package handler

import (
	"net/http"

	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type PedomanHandler struct {
	uc *usecase.PedomanUsecase
}

func NewPedomanHandler(uc *usecase.PedomanUsecase) *PedomanHandler { return &PedomanHandler{uc: uc} }

// List GET /api/info/laporan & /api/admin/pedoman
//
//	@Summary		Daftar Pedoman Laporan
//	@Description	Mengambil daftar pedoman laporan (Admin dan Publik)
//	@Tags			Info, Admin - Pedoman
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]entity.PedomanLaporan}
//	@Router			/info/laporan [get]
//	@Router			/admin/pedoman [get]
func (h *PedomanHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar pedoman", res)
}

// Create POST /api/admin/pedoman
//
//	@Summary		Tambah Pedoman
//	@Description	Menambahkan pedoman laporan baru
//	@Tags			Admin - Pedoman
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.PedomanRequest	true	"Payload Pedoman"
//	@Success		201		{object}	response.Envelope{data=entity.PedomanLaporan}
//	@Router			/admin/pedoman [post]
func (h *PedomanHandler) Create(c *gin.Context) {
	var req dto.PedomanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Pedoman ditambahkan", res)
}

// Update PUT /api/admin/pedoman/:id
//
//	@Summary		Perbarui Pedoman
//	@Description	Mengubah pedoman laporan
//	@Tags			Admin - Pedoman
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"ID Pedoman"
//	@Param			request	body		dto.PedomanRequest	true	"Payload Pedoman"
//	@Success		200		{object}	response.Envelope{data=entity.PedomanLaporan}
//	@Router			/admin/pedoman/{id} [put]
func (h *PedomanHandler) Update(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.PedomanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Update(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Pedoman diperbarui", res)
}

// Delete DELETE /api/admin/pedoman/:id
//
//	@Summary		Hapus Pedoman
//	@Description	Menghapus pedoman laporan
//	@Tags			Admin - Pedoman
//	@Security		bearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID Pedoman"
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/pedoman/{id} [delete]
func (h *PedomanHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Pedoman dihapus", nil)
}
