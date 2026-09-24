package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"
)

type RekapHandler struct {
	uc *usecase.RekapUsecase
}

func NewRekapHandler(uc *usecase.RekapUsecase) *RekapHandler {
	return &RekapHandler{uc: uc}
}

// GetRekapKelas GET /api/admin/rekap/kelas/:id_kelas
// @Summary Rekap Nilai per Kelas
// @Description Mengambil data nilai keseluruhan mahasiswa dalam satu kelas (Pivot Table)
// @Tags Admin - Rekap
// @Security bearerAuth
// @Produce json
// @Param id_kelas path int true "ID Kelas"
// @Success 200 {object} response.Envelope{data=dto.RekapKelasResponse}
// @Router /admin/rekap/kelas/{id_kelas} [get]
func (h *RekapHandler) GetRekapKelas(c *gin.Context) {
	id, ok := idParam(c, "id_kelas")
	if !ok {
		return
	}

	res, err := h.uc.GetRekapKelas(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Rekap berhasil dimuat", res)
}
