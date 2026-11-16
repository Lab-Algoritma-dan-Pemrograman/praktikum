package handler

import (
	"encoding/base64"
	"net/http"

	"lab-ap/internal/dto"
	"lab-ap/pkg/cwasm"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// maxCompileSource membatasi ukuran sumber yang dikompilasi (64 KB).
const maxCompileSource = 64 * 1024

type CompileHandler struct {
	cc *cwasm.Compiler
}

func NewCompileHandler(cc *cwasm.Compiler) *CompileHandler { return &CompileHandler{cc: cc} }

// CompileC POST /api/praktikum/compile-c
//
//	@Summary		Kompilasi kode C ke WebAssembly (wasm32-wasi)
//	@Description	Mengompilasi sumber C menjadi wasm untuk dijalankan interaktif di browser. Gagal kompilasi dikembalikan sebagai data (stderr), bukan HTTP error.
//	@Tags			Praktikum - Pengerjaan
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.CompileRequest	true	"Payload Compile"
//	@Success		200		{object}	response.Envelope{data=dto.CompileResponse}
//	@Router			/praktikum/compile-c [post]
func (h *CompileHandler) CompileC(c *gin.Context) {
	if !h.cc.Enabled() {
		response.Fail(c, http.StatusServiceUnavailable, "Fitur compile C belum dikonfigurasi", nil)
		return
	}
	var req dto.CompileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if len(req.Source) > maxCompileSource {
		response.Fail(c, http.StatusRequestEntityTooLarge, "Kode terlalu panjang (maks 64 KB)", nil)
		return
	}

	wasm, stderr, err := h.cc.Compile(req.Source)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, "Compiler error", err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Hasil kompilasi", dto.CompileResponse{
		Wasm:   base64.StdEncoding.EncodeToString(wasm), // "" bila gagal kompilasi
		Stderr: stderr,
	})
}
