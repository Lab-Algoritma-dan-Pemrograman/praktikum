package handler

import (
	"net/http"
	"strconv"

	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type AIGradingHandler struct {
	aiUsecase usecase.AIGradingUsecase
}

func NewAIGradingHandler(ai usecase.AIGradingUsecase) *AIGradingHandler {
	return &AIGradingHandler{aiUsecase: ai}
}

// ListTargets GET /api/admin/penilaian/ai-grade/targets?aktivasi_sesi_id=&course_id=
//
//	@Summary		Daftar jawaban yang perlu dinilai AI
//	@Description	Mengembalikan daftar jawaban_id (submitted, belum dinilai) untuk satu course
//	@Tags			Admin - Penilaian
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=dto.AIGradeTargetsResponse}
//	@Router			/admin/penilaian/ai-grade/targets [get]
func (h *AIGradingHandler) ListTargets(c *gin.Context) {
	aks, _ := strconv.Atoi(c.Query("aktivasi_sesi_id"))
	cid, _ := strconv.Atoi(c.Query("course_id"))
	if aks == 0 || cid == 0 {
		response.Fail(c, http.StatusBadRequest, "aktivasi_sesi_id & course_id wajib", nil)
		return
	}
	res, err := h.aiUsecase.ListTargets(aks, cid)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Target AI grading", res)
}

// GradeOne POST /api/admin/penilaian/ai-grade/one
//
//	@Summary		Nilai satu jawaban dengan AI (sinkron)
//	@Description	Menilai SATU jawaban memakai AI lalu menyimpan hasilnya. Frontend memanggil ini berulang.
//	@Tags			Admin - Penilaian
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AIGradeOneRequest	true	"ID Jawaban"
//	@Success		200		{object}	response.Envelope{data=dto.AIGradeOneResponse}
//	@Router			/admin/penilaian/ai-grade/one [post]
func (h *AIGradingHandler) GradeOne(c *gin.Context) {
	var req dto.AIGradeOneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.aiUsecase.GradeOne(req.JawabanID)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Jawaban dinilai AI", res)
}
