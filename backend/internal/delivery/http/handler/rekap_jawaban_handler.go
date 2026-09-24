package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type RekapJawabanHandler struct {
	penilaianUsecase *usecase.PenilaianUsecase
	auditLog         *usecase.AuditLogUsecase
}

func NewRekapJawabanHandler(p *usecase.PenilaianUsecase, al *usecase.AuditLogUsecase) *RekapJawabanHandler {
	return &RekapJawabanHandler{penilaianUsecase: p, auditLog: al}
}

// GetRekapJawabanGlobal GET /api/admin/rekap-jawaban
// @Summary Rekap Jawaban Global (Flat List)
// @Description Mengambil data jawaban secara flat untuk keperluan tabel rekap global
// @Tags Admin - Rekap
// @Security bearerAuth
// @Produce json
// @Param kelas_id query int false "Filter berdasarkan ID Kelas"
// @Param sesi_id query int false "Filter berdasarkan ID Sesi"
// @Param search query string false "Filter NIM atau Nama"
// @Param jenis query string false "Filter Jenis Tes (pretest, posttest, dll)"
// @Success 200 {object} response.Envelope{data=dto.RekapJawabanResponse}
// @Router /admin/rekap-jawaban [get]
func (h *RekapJawabanHandler) GetRekapJawabanGlobal(c *gin.Context) {
	kelasID, _ := strconv.Atoi(c.Query("kelas_id"))
	sesiID, _ := strconv.Atoi(c.Query("sesi_id"))
	search := c.Query("search")
	jenis := c.Query("jenis")

	resp, err := h.penilaianUsecase.GetRekapJawabanGlobal(kelasID, sesiID, search, jenis)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	response.OK(c, http.StatusOK, "Rekap jawaban", resp)
}

// BulkAction POST /api/admin/penilaian/bulk-action
// @Summary Bulk Action Penilaian (Reset / Hapus)
// @Description Mereset nilai atau menghapus jawaban secara masal
// @Tags Admin - Penilaian
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BulkActionRequest true "Daftar ID dan Aksi"
// @Success 200 {object} response.Envelope
// @Router /admin/penilaian/bulk-action [post]
func (h *RekapJawabanHandler) BulkAction(c *gin.Context) {
	var req dto.BulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if len(req.JawabanIDs) == 0 {
		response.Fail(c, http.StatusBadRequest, "Daftar jawaban kosong", nil)
		return
	}

	var err error
	switch req.Action {
	case "delete":
		err = h.penilaianUsecase.BulkDeleteJawaban(req.JawabanIDs)
	case "reset_nilai":
		err = h.penilaianUsecase.BulkResetNilai(req.JawabanIDs)
	case "buka_kunci":
		err = h.penilaianUsecase.BulkUnlock(req.JawabanIDs)
	default:
		// CR-M2: dulu action tak dikenal lolos dan dibalas "berhasil" tanpa efek.
		response.Fail(c, http.StatusBadRequest, "Aksi tidak dikenal: "+req.Action, nil)
		return
	}

	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// CR-M2: aksi destruktif WAJIB meninggalkan jejak audit (siapa, berapa, ID mana).
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "BULK_"+strings.ToUpper(req.Action),
		fmt.Sprintf("Bulk action %s atas %d jawaban (IDs: %v)", req.Action, len(req.JawabanIDs), req.JawabanIDs),
		clientIP(c), c.Request.UserAgent())

	response.OK(c, http.StatusOK, "Bulk action "+req.Action+" berhasil", nil)
}
