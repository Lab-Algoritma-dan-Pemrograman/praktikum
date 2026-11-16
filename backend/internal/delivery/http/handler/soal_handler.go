package handler

import (
	"fmt"
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type SoalHandler struct {
	uc       *usecase.SoalUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewSoalHandler(uc *usecase.SoalUsecase, al *usecase.AuditLogUsecase) *SoalHandler {
	return &SoalHandler{uc: uc, auditLog: al}
}

// ListByCourse GET /api/admin/soal
//
//	@Summary		Daftar Soal
//	@Description	Mengambil daftar soal berdasarkan ID course
//	@Tags			Admin - Soal
//	@Security		bearerAuth
//	@Produce		json
//	@Param			course_id	query		int	true	"ID Course"
//	@Success		200			{object}	response.Envelope{data=[]entity.Soal}
//	@Router			/admin/soal [get]
func (h *SoalHandler) ListByCourse(c *gin.Context) {
	cid := queryIntPtr(c, "course_id")
	if cid == nil {
		response.Fail(c, http.StatusBadRequest, "course_id wajib", nil)
		return
	}
	res, err := h.uc.ListByCourse(*cid)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar soal", res)
}

// Create POST /api/admin/soal
//
//	@Summary		Tambah Soal
//	@Description	Menambahkan soal baru ke dalam course
//	@Tags			Admin - Soal
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.SoalRequest	true	"Payload"
//	@Success		201		{object}	response.Envelope{data=entity.Soal}
//	@Router			/admin/soal [post]
func (h *SoalHandler) Create(c *gin.Context) {
	var req dto.SoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(req)
	if err != nil {
		mapError(c, err)
		return
	}
	teksRunes := []rune(res.TeksSoal)
	shortTeks := string(teksRunes)
	if len(teksRunes) > 30 {
		shortTeks = string(teksRunes[:30]) + "..."
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "CREATE_SOAL", "Membuat soal baru: '"+shortTeks+"'", clientIP(c), c.Request.UserAgent())
	response.Created(c, "Soal dibuat", res)
}

// Update PUT /api/admin/soal/:id
//
//	@Summary		Perbarui Soal
//	@Description	Mengubah data soal
//	@Tags			Admin - Soal
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int				true	"ID Soal"
//	@Param			request	body		dto.SoalRequest	true	"Payload"
//	@Success		200		{object}	response.Envelope{data=entity.Soal}
//	@Router			/admin/soal/{id} [put]
func (h *SoalHandler) Update(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.SoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Update(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	teksRunes := []rune(res.TeksSoal)
	shortTeks := string(teksRunes)
	if len(teksRunes) > 30 {
		shortTeks = string(teksRunes[:30]) + "..."
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "UPDATE_SOAL", fmt.Sprintf("Memperbarui soal: '%s' (ID: %d)", shortTeks, res.ID), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Soal diperbarui", res)
}

// Delete DELETE /api/admin/soal/:id
//
//	@Summary		Hapus Soal
//	@Description	Menghapus soal
//	@Tags			Admin - Soal
//	@Security		bearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID Soal"
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/soal/{id} [delete]
func (h *SoalHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "DELETE_SOAL", fmt.Sprintf("Menghapus soal dengan ID %d", id), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Soal dihapus", nil)
}
