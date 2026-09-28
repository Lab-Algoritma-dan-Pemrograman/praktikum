package handler

import (
	"fmt"
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	_ "lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type AktivasiHandler struct {
	uc       *usecase.AktivasiUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewAktivasiHandler(uc *usecase.AktivasiUsecase, al *usecase.AuditLogUsecase) *AktivasiHandler {
	return &AktivasiHandler{uc: uc, auditLog: al}
}

// List GET /api/admin/aktivasi
// @Summary Daftar aktivasi
// @Description Mengambil daftar seluruh sesi yang sedang/pernah diaktifkan
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=[]entity.AktivasiSesi}
// @Router /admin/aktivasi [get]
func (h *AktivasiHandler) List(c *gin.Context) {
	res, err := h.uc.List()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar aktivasi", res)
}

// Get GET /api/admin/aktivasi/:id
// @Summary Detail aktivasi
// @Description Mengambil detail satu aktivasi
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Success 200 {object} response.Envelope{data=entity.AktivasiSesi}
// @Router /admin/aktivasi/{id} [get]
func (h *AktivasiHandler) Get(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	res, err := h.uc.Get(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Detail aktivasi", res)
}

// Aktivasi POST /api/admin/aktivasi (aktifkan sesi + gacha)
// @Summary Mengaktifkan sesi praktikum (Gacha Soal)
// @Description Mengaktifkan sesi untuk kelas tertentu dan melakukan gacha soal pretest/posttest/ujian
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.AktivasiRequest true "Payload Aktivasi"
// @Success 201 {object} response.Envelope{data=entity.AktivasiSesi}
// @Router /admin/aktivasi [post]
func (h *AktivasiHandler) Aktivasi(c *gin.Context) {
	var req dto.AktivasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Aktivasi(req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Sesi diaktifkan", res)
}

// BukaTutupCourse POST /api/admin/aktivasi-course/buka-tutup
// @Summary Buka/Tutup Course
// @Description Membuka atau menutup course. Jika ditutup, akan melakukan auto-submit massal.
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.BukaTutupCourseRequest true "Payload Buka Tutup"
// @Success 200 {object} response.Envelope{data=string}
// @Router /admin/aktivasi-course/buka-tutup [post]
func (h *AktivasiHandler) BukaTutupCourse(c *gin.Context) {
	var req dto.BukaTutupCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.BukaTutupCourse(req)
	if err != nil {
		mapError(c, err)
		return
	}
	msg := "Course ditutup (auto-submit massal dijalankan)"
	action := "BUKA_TUTUP_COURSE"
	if req.UnlockOnly {
		msg = "Kunci peserta dilepas. Pengerjaan direset, isi jawaban tetap tersimpan, timer mulai ulang dari peserta pertama."
		action = "BUKA_KUNCI_COURSE"
	} else if req.IsOpen {
		msg = "Course dibuka"
	}
	// Buka/tutup course berdampak massal (auto-submit semua peserta / melepas
	// kunci semua peserta) dan sebelumnya tidak meninggalkan jejak sama sekali.
	if h.auditLog != nil {
		_ = h.auditLog.LogAction(middleware.UserID(c), "", action,
			fmt.Sprintf("aktivasi_course_id=%d aktivasi_sesi_id=%d course_id=%d is_open=%v unlock_only=%v",
				res.ID, res.AktivasiSesiID, res.CourseID, res.IsOpen, req.UnlockOnly),
			clientIP(c), c.Request.UserAgent())
	}
	response.OK(c, http.StatusOK, msg, res)
}

// AddSusulan POST /api/admin/aktivasi/:id/susulan
// @Summary Tambah Peserta Susulan
// @Description Menambahkan mahasiswa sebagai peserta susulan ke suatu aktivasi sesi
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Param request body dto.SusulanRequest true "Payload Susulan"
// @Success 201 {object} response.Envelope
// @Router /admin/aktivasi/{id}/susulan [post]
func (h *AktivasiHandler) AddSusulan(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.SusulanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	req.AktivasiSesiID = id
	if err := h.uc.AddSusulan(req); err != nil {
		mapError(c, err)
		return
	}
	response.Created(c, "Mahasiswa susulan didaftarkan", nil)
}

// ListSusulan GET /api/admin/aktivasi/:id/susulan
// @Summary Daftar Peserta Susulan
// @Description Melihat daftar mahasiswa yang ditambahkan sebagai peserta susulan
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Success 200 {object} response.Envelope{data=[]entity.PesertaSusulan}
// @Router /admin/aktivasi/{id}/susulan [get]
func (h *AktivasiHandler) ListSusulan(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	res, err := h.uc.ListSusulan(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar susulan", res)
}

// RemoveSusulan DELETE /api/admin/aktivasi/:id/susulan/:mahasiswaId
// @Summary Hapus Peserta Susulan
// @Description Menghapus mahasiswa dari daftar peserta susulan
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Param mahasiswaId path int true "ID Mahasiswa"
// @Success 200 {object} response.Envelope
// @Router /admin/aktivasi/{id}/susulan/{mahasiswaId} [delete]
func (h *AktivasiHandler) RemoveSusulan(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	mid, ok := idParam(c, "mahasiswaId")
	if !ok {
		return
	}
	if err := h.uc.RemoveSusulan(id, mid); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Susulan dihapus", nil)
}

// GenerateToken POST /api/admin/aktivasi/:id/token
// @Summary Generate PIN/Token Ujian
// @Description Men-generate Token 6 digit acak untuk suatu AktivasiSesi
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Success 200 {object} response.Envelope{data=entity.AktivasiSesi}
// @Router /admin/aktivasi/{id}/token [post]
func (h *AktivasiHandler) GenerateToken(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	res, err := h.uc.GenerateToken(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Token berhasil digenerate", res)
}

// Delete DELETE /api/admin/aktivasi/:id
// @Summary Hapus Aktivasi (beserta semua jawaban & nilai di bawahnya)
// @Description Menghapus satu aktivasi sesi + cascade: jawaban mahasiswa, nilai, soal terpilih, susulan, aktivasi_course.
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Success 200 {object} response.Envelope
// @Router /admin/aktivasi/{id} [delete]
func (h *AktivasiHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.DeleteSesi(id); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Aktivasi beserta seluruh data terkait dihapus", nil)
}

// Peserta GET /api/admin/aktivasi/:id/peserta
// @Summary Daftar peserta + status pengerjaan (live monitoring)
// @Description Peserta yang sudah join/mengerjakan aktivasi ini beserta status per course.
// @Tags Admin - Aktivasi
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Aktivasi"
// @Success 200 {object} response.Envelope
// @Router /admin/aktivasi/{id}/peserta [get]
func (h *AktivasiHandler) Peserta(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	res, err := h.uc.PesertaProgress(id)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar peserta", res)
}
