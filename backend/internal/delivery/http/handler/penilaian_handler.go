package handler

import (
	"fmt"
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type PenilaianHandler struct {
	uc       *usecase.PenilaianUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewPenilaianHandler(uc *usecase.PenilaianUsecase, al *usecase.AuditLogUsecase) *PenilaianHandler {
	return &PenilaianHandler{uc: uc, auditLog: al}
}

// Rekap GET /api/admin/penilaian/rekap
//
//	@Summary		Rekap Nilai
//	@Description	Mengambil rekapitulasi nilai jawaban untuk satu course pada suatu sesi
//	@Tags			Admin - Penilaian
//	@Security		bearerAuth
//	@Produce		json
//	@Param			aktivasi_sesi_id	query		int	true	"ID Aktivasi Sesi"
//	@Param			course_id			query		int	true	"ID Course"
//	@Success		200					{object}	response.Envelope{data=dto.RekapResponse}
//	@Router			/admin/penilaian/rekap [get]
func (h *PenilaianHandler) Rekap(c *gin.Context) {
	aks := queryIntPtr(c, "aktivasi_sesi_id")
	cid := queryIntPtr(c, "course_id")
	if aks == nil || cid == nil {
		response.Fail(c, http.StatusBadRequest, "aktivasi_sesi_id & course_id wajib", nil)
		return
	}
	res, err := h.uc.Rekap(*aks, *cid)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Rekap jawaban", res)
}

// SetNilai POST /api/admin/penilaian
//
//	@Summary		Set Nilai Jawaban
//	@Description	Memberikan atau mengubah nilai (skor) pada jawaban mahasiswa secara manual
//	@Tags			Admin - Penilaian
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.NilaiRequest	true	"Payload Nilai"
//	@Success		200		{object}	response.Envelope
//	@Router			/admin/penilaian [post]
func (h *PenilaianHandler) SetNilai(c *gin.Context) {
	var req dto.NilaiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.SetNilai(req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "SET_NILAI", fmt.Sprintf("Menilai jawaban ID %d dengan skor %.1f", req.JawabanID, req.Nilai), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Nilai disimpan", res)
}

// SetKeaktifan POST /api/admin/keaktifan
//
//	@Summary		Set Keaktifan
//	@Description	Set nilai keaktifan (partisipasi) per pengerjaan pretest/posttest, massal.
//	@Tags			Admin - Penilaian
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.KeaktifanRequest	true	"Payload Keaktifan"
//	@Success		200		{object}	response.Envelope
//	@Router			/admin/keaktifan [post]
func (h *PenilaianHandler) SetKeaktifan(c *gin.Context) {
	var req dto.KeaktifanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.uc.SetKeaktifanBulk(req.Items); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "SET_KEAKTIFAN", fmt.Sprintf("Mengubah status keaktifan bulk untuk %d item pengerjaan", len(req.Items)), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Keaktifan disimpan", nil)
}
