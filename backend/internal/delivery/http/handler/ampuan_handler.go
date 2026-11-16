package handler

import (
	"net/http"

	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type AmpuanHandler struct {
	uc   *usecase.AmpuanUsecase
	user *usecase.UserUsecase
}

func NewAmpuanHandler(uc *usecase.AmpuanUsecase, user *usecase.UserUsecase) *AmpuanHandler {
	return &AmpuanHandler{uc: uc, user: user}
}

// List GET /api/admin/ampuan
//
//	@Summary		Daftar Ampuan
//	@Description	Mengambil data dosen/asisten pengampu
//	@Tags			Admin - Ampuan
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]entity.AmpuanKelompok}
//	@Router			/admin/ampuan [get]
func (h *AmpuanHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar ampuan", res)
}

// Create POST /api/admin/ampuan
//
//	@Summary		Tambah Ampuan
//	@Description	Menambahkan data pengampu untuk kelompok/kelas
//	@Tags			Admin - Ampuan
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.AmpuanRequest	true	"Payload"
//	@Success		201		{object}	response.Envelope{data=entity.AmpuanKelompok}
//	@Router			/admin/ampuan [post]
func (h *AmpuanHandler) Create(c *gin.Context) {
	var req dto.AmpuanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Ampuan ditambahkan", res)
}

// Delete DELETE /api/admin/ampuan/:id
//
//	@Summary		Hapus Ampuan
//	@Description	Menghapus data pengampu
//	@Tags			Admin - Ampuan
//	@Security		bearerAuth
//	@Produce		json
//	@Param			id	path		int	true	"ID Ampuan"
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/ampuan/{id} [delete]
func (h *AmpuanHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Ampuan dihapus", nil)
}

// PublicKelasMahasiswa GET /api/info/kelas/:id/mahasiswa
//
//	@Summary		Data Kelas Mahasiswa
//	@Description	Mengambil data seluruh mahasiswa dalam satu kelas beserta data pengampu (Publik)
//	@Tags			Info
//	@Produce		json
//	@Param			id	path		int	true	"ID Kelas"
//	@Success		200	{object}	response.Envelope
//	@Router			/info/kelas/{id}/mahasiswa [get]
func (h *AmpuanHandler) PublicKelasMahasiswa(c *gin.Context) {
	kelasID, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhs, err := h.user.ListMahasiswa(&kelasID, nil)
	if err != nil {
		mapError(c, err)
		return
	}
	ampuan, err := h.uc.ListByKelas(kelasID)
	if err != nil {
		mapError(c, err)
		return
	}

	// Endpoint publik: kirim hanya field aman (tanpa PII mahasiswa).
	safe := make([]dto.PublicMahasiswaItem, 0, len(mhs))
	for _, m := range mhs {
		safe = append(safe, dto.PublicMahasiswaItem{
			ID:       m.ID,
			NIM:      m.NIM,
			Nama:     m.Nama,
			Shift:    m.Shift,
			Kelompok: m.Kelompok,
		})
	}

	response.OK(c, http.StatusOK, "Mahasiswa & ampuan kelas", gin.H{
		"mahasiswa": safe,
		"ampuan":    ampuan,
	})
}
