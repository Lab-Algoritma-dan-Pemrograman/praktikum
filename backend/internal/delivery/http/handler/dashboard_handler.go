package handler

import (
	"net/http"

	_ "lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	uc *usecase.DashboardUsecase
}

func NewDashboardHandler(uc *usecase.DashboardUsecase) *DashboardHandler {
	return &DashboardHandler{uc: uc}
}

// Statistik GET /api/admin/dashboard
//
//	@Summary		Statistik Dashboard Admin
//	@Description	Mengambil data statistik umum untuk dashboard admin
//	@Tags			Admin - Dashboard
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=dto.StatistikResponse}
//	@Router			/admin/dashboard [get]
func (h *DashboardHandler) Statistik(c *gin.Context) {
	res, err := h.uc.Statistik()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Statistik dashboard", res)
}
