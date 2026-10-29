package handler

import (
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type LaporanBesarHandler struct {
	uc *usecase.LaporanBesarUsecase
}

func NewLaporanBesarHandler(uc *usecase.LaporanBesarUsecase) *LaporanBesarHandler {
	return &LaporanBesarHandler{uc: uc}
}

// ---- Mahasiswa ----

// StatusSaya GET /api/praktikum/laporan-besar
// @Summary Status Laporan Besar Saya
// @Description Status laporan besar mahasiswa + window kelas + sisa waktu
// @Tags Praktikum - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.LaporanBesarStatusResponse}
// @Router /praktikum/laporan-besar [get]
func (h *LaporanBesarHandler) StatusSaya(c *gin.Context) {
	res, err := h.uc.StatusSaya(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Status laporan besar", res)
}

// UploadURL POST /api/praktikum/laporan-besar/upload-url
// @Summary Minta URL Unggah Laporan Besar
// @Description Membuat signed upload URL ke bucket privat laporan besar
// @Tags Praktikum - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UploadURLRequest true "Payload"
// @Success 200 {object} response.Envelope{data=dto.UploadURLResponse}
// @Router /praktikum/laporan-besar/upload-url [post]
func (h *LaporanBesarHandler) UploadURL(c *gin.Context) {
	var req dto.UploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.UploadURL(middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "URL unggah dibuat", res)
}

// Kumpul POST /api/praktikum/laporan-besar/kumpul
// @Summary Kumpulkan Laporan Besar
// @Description Menandai berkas laporan besar sudah diunggah (validasi waktu server)
// @Tags Praktikum - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.KumpulRequest true "Payload"
// @Success 200 {object} response.Envelope
// @Router /praktikum/laporan-besar/kumpul [post]
func (h *LaporanBesarHandler) Kumpul(c *gin.Context) {
	var req dto.KumpulRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Kumpul(middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Laporan besar dikumpulkan", res)
}

// Batalkan DELETE /api/praktikum/laporan-besar/kumpul
// @Summary Batalkan Pengumpulan Laporan Besar
// @Description Membatalkan pengumpulan (bila belum diverifikasi/di-override)
// @Tags Praktikum - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /praktikum/laporan-besar/kumpul [delete]
func (h *LaporanBesarHandler) Batalkan(c *gin.Context) {
	if err := h.uc.BatalkanKumpul(middleware.UserID(c)); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Pengumpulan dibatalkan", nil)
}

// ---- Admin ----

// List GET /api/admin/laporan-besar
// @Summary Daftar Laporan Besar
// @Description Mengambil daftar tugas laporan besar
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=[]entity.LaporanBesar}
// @Router /admin/laporan-besar [get]
func (h *LaporanBesarHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar laporan besar", res)
}

// Create POST /api/admin/laporan-besar
// @Summary Tambah Laporan Besar
// @Description Membuat tugas laporan besar baru
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.LaporanBesarRequest true "Payload"
// @Success 201 {object} response.Envelope{data=entity.LaporanBesar}
// @Router /admin/laporan-besar [post]
func (h *LaporanBesarHandler) Create(c *gin.Context) {
	var req dto.LaporanBesarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Create(middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Laporan besar dibuat", res)
}

// GetDetail GET /api/admin/laporan-besar/:id
// @Summary Detail Laporan Besar
// @Description Detail laporan besar + target kelas
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Success 200 {object} response.Envelope
// @Router /admin/laporan-besar/{id} [get]
func (h *LaporanBesarHandler) GetDetail(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	l, ts, err := h.uc.GetDetail(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Detail laporan besar", gin.H{"laporan": l, "targets": ts})
}

// Update PUT /api/admin/laporan-besar/:id
// @Summary Perbarui Laporan Besar
// @Description Mengubah tugas laporan besar
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param request body dto.LaporanBesarRequest true "Payload"
// @Success 200 {object} response.Envelope{data=entity.LaporanBesar}
// @Router /admin/laporan-besar/{id} [put]
func (h *LaporanBesarHandler) Update(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.LaporanBesarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Update(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Laporan besar diperbarui", res)
}

// Delete DELETE /api/admin/laporan-besar/:id
// @Summary Hapus Laporan Besar
// @Description Menghapus tugas laporan besar
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Success 200 {object} response.Envelope
// @Router /admin/laporan-besar/{id} [delete]
func (h *LaporanBesarHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Laporan besar dihapus", nil)
}

// SetTarget PUT /api/admin/laporan-besar/:id/target
// @Summary Set Window Laporan Besar
// @Description Menyimpan window pengumpulan per kelas (replace)
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param request body dto.SetTargetRequest true "Payload"
// @Success 200 {object} response.Envelope{data=[]entity.LaporanBesarTarget}
// @Router /admin/laporan-besar/{id}/target [put]
func (h *LaporanBesarHandler) SetTarget(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.SetTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.SetTargets(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Window laporan besar disimpan", res)
}

// ListPengumpulan GET /api/admin/laporan-besar/:id/pengumpulan
// @Summary Rekap Pengumpulan Laporan Besar
// @Description Daftar mahasiswa kelas target + status + jejak pengumpulan
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Success 200 {object} response.Envelope{data=[]dto.PengumpulanRow}
// @Router /admin/laporan-besar/{id}/pengumpulan [get]
func (h *LaporanBesarHandler) ListPengumpulan(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	res, err := h.uc.ListPengumpulan(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Rekap pengumpulan", res)
}

// Mark PUT /api/admin/laporan-besar/:id/pengumpulan/:mhsId/mark
// @Summary Centang Manual Pengumpulan
// @Description Asisten mencentang manual pengumpulan mahasiswa (sumber=manual)
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param mhsId path int true "ID Mahasiswa"
// @Param request body dto.MarkRequest true "Payload"
// @Success 200 {object} response.Envelope
// @Router /admin/laporan-besar/{id}/pengumpulan/{mhsId}/mark [put]
func (h *LaporanBesarHandler) Mark(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhsID, ok := idParam(c, "mhsId")
	if !ok {
		return
	}
	var req dto.MarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Mark(id, mhsID, middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Pengumpulan diperbarui", res)
}

// SetOverride PUT /api/admin/laporan-besar/:id/pengumpulan/:mhsId/override
// @Summary Set Override Gerbang
// @Description Membuka blokir gerbang ujian untuk mahasiswa (tanpa ubah status)
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param mhsId path int true "ID Mahasiswa"
// @Param request body dto.OverrideRequest true "Payload"
// @Success 200 {object} response.Envelope
// @Router /admin/laporan-besar/{id}/pengumpulan/{mhsId}/override [put]
func (h *LaporanBesarHandler) SetOverride(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhsID, ok := idParam(c, "mhsId")
	if !ok {
		return
	}
	var req dto.OverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.SetOverride(id, mhsID, middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Override disimpan", res)
}

// RevokeOverride DELETE /api/admin/laporan-besar/:id/pengumpulan/:mhsId/override
// @Summary Cabut Override Gerbang
// @Description Mencabut override gerbang ujian mahasiswa
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param mhsId path int true "ID Mahasiswa"
// @Success 200 {object} response.Envelope
// @Router /admin/laporan-besar/{id}/pengumpulan/{mhsId}/override [delete]
func (h *LaporanBesarHandler) RevokeOverride(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhsID, ok := idParam(c, "mhsId")
	if !ok {
		return
	}
	res, err := h.uc.RevokeOverride(id, mhsID)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Override dicabut", res)
}

// Preview GET /api/admin/laporan-besar/:id/pengumpulan/:mhsId/preview?jenis=laporan
// @Summary Pratinjau Berkas Laporan Besar
// @Description Mengembalikan signed URL (tanpa nama unduhan) untuk dibuka frontend
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param mhsId path int true "ID Mahasiswa"
// @Param jenis query string true "Jenis berkas (laporan|analisa)"
// @Success 200 {object} response.Envelope{data=dto.BerkasURLResponse}
// @Router /admin/laporan-besar/{id}/pengumpulan/{mhsId}/preview [get]
func (h *LaporanBesarHandler) Preview(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhsID, ok := idParam(c, "mhsId")
	if !ok {
		return
	}
	url, name, err := h.uc.SignedBerkas(id, mhsID, 0, c.Query("jenis"), false)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "URL pratinjau", gin.H{"url": url, "filename": name})
}

// Download GET /api/admin/laporan-besar/:id/pengumpulan/:mhsId/download?jenis=analisa
// @Summary Unduh Berkas Laporan Besar
// @Description Mengembalikan signed URL (dengan nama unduhan) untuk diunduh frontend
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param mhsId path int true "ID Mahasiswa"
// @Param jenis query string true "Jenis berkas (laporan|analisa)"
// @Success 200 {object} response.Envelope{data=dto.BerkasURLResponse}
// @Router /admin/laporan-besar/{id}/pengumpulan/{mhsId}/download [get]
func (h *LaporanBesarHandler) Download(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mhsID, ok := idParam(c, "mhsId")
	if !ok {
		return
	}
	url, name, err := h.uc.SignedBerkas(id, mhsID, 0, c.Query("jenis"), true)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "URL unduhan", gin.H{"url": url, "filename": name})
}

// DownloadBatch POST /api/admin/laporan-besar/:id/download-batch
// @Summary Unduh Massal Berkas Laporan Besar
// @Description Mengembalikan daftar signed URL (tidak zip di server)
// @Tags Admin - Laporan Besar
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Laporan Besar"
// @Param request body dto.DownloadBatchRequest true "Payload"
// @Success 200 {object} response.Envelope{data=[]dto.DownloadBatchResult}
// @Router /admin/laporan-besar/{id}/download-batch [post]
func (h *LaporanBesarHandler) DownloadBatch(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.DownloadBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.DownloadBatch(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar berkas", res)
}
