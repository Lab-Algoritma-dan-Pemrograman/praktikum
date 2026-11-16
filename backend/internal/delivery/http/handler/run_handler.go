package handler

import (
	"net/http"

	"lab-ap/internal/dto"
	"lab-ap/pkg/glot"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// maxRunSource membatasi ukuran kode yang dijalankan (64 KB).
const maxRunSource = 64 * 1024

type RunHandler struct {
	glot *glot.Client
}

func NewRunHandler(g *glot.Client) *RunHandler { return &RunHandler{glot: g} }

// Execute POST /api/praktikum/run
//
//	@Summary		Jalankan kode (C/Python)
//	@Description	Menjalankan kode mahasiswa di sandbox eksternal (Glot.io) dan mengembalikan output.
//	@Tags			Praktikum - Pengerjaan
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.RunRequest	true	"Payload Run"
//	@Success		200		{object}	response.Envelope{data=dto.RunResponse}
//	@Router			/praktikum/run [post]
func (h *RunHandler) Execute(c *gin.Context) {
	if !h.glot.Enabled() {
		response.Fail(c, http.StatusServiceUnavailable, "Fitur run code belum dikonfigurasi", nil)
		return
	}
	var req dto.RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if len(req.Source) > maxRunSource {
		response.Fail(c, http.StatusRequestEntityTooLarge, "Kode terlalu panjang (maks 64 KB)", nil)
		return
	}

	res, err := h.glot.Run(req.Language, req.Source, req.Stdin)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, "Gagal menjalankan kode", err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Hasil eksekusi", dto.RunResponse{
		Stdout: res.Stdout,
		Stderr: res.Stderr,
		Error:  res.Error,
	})
}
