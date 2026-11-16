package handler

import (
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	_ "lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// PraktikumHandler melayani sisi mahasiswa (dashboard + daftar sesi).
type PraktikumHandler struct {
	uc *usecase.PraktikumUsecase
}

func NewPraktikumHandler(uc *usecase.PraktikumUsecase) *PraktikumHandler {
	return &PraktikumHandler{uc: uc}
}

// Dashboard GET /api/praktikum/dashboard
//
//	@Summary		Dashboard Mahasiswa
//	@Description	Mengambil data statistik dashboard khusus mahasiswa
//	@Tags			Praktikum
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=dto.DashboardUserResponse}
//	@Router			/praktikum/dashboard [get]
func (h *PraktikumHandler) Dashboard(c *gin.Context) {
	res, err := h.uc.Dashboard(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Dashboard mahasiswa", res)
}

// ListSesi GET /api/praktikum/sesi
//
//	@Summary		Daftar Sesi Mahasiswa
//	@Description	Mengambil daftar sesi praktikum yang tersedia untuk mahasiswa terkait
//	@Tags			Praktikum
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]dto.SesiUserItem}
//	@Router			/praktikum/sesi [get]
func (h *PraktikumHandler) ListSesi(c *gin.Context) {
	res, err := h.uc.ListSesi(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar sesi", res)
}
