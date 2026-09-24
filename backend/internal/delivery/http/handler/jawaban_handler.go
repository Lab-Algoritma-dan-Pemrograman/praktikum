package handler

import (
	"fmt"
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type JawabanHandler struct {
	uc       *usecase.JawabanUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewJawabanHandler(uc *usecase.JawabanUsecase, al *usecase.AuditLogUsecase) *JawabanHandler {
	return &JawabanHandler{uc: uc, auditLog: al}
}

// GetRuang GET /api/praktikum/ruang
// @Summary Masuk Ruang Ujian/Course
// @Description Memuat soal-soal dan state timer untuk suatu course
// @Tags Praktikum - Pengerjaan
// @Security bearerAuth
// @Produce json
// @Param aktivasi_sesi_id query int true "ID Aktivasi Sesi"
// @Param course_id query int true "ID Course"
// @Success 200 {object} response.Envelope{data=dto.RuangCourseResponse}
// @Router /praktikum/ruang [get]
func (h *JawabanHandler) GetRuang(c *gin.Context) {
	aks := queryIntPtr(c, "aktivasi_sesi_id")
	cid := queryIntPtr(c, "course_id")
	if aks == nil || cid == nil {
		response.Fail(c, http.StatusBadRequest, "aktivasi_sesi_id & course_id wajib", nil)
		return
	}
	res, err := h.uc.GetRuang(middleware.UserID(c), *aks, *cid)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Ruang course", res)
}

// Mulai POST /api/praktikum/mulai
// @Summary Mulai Ujian
// @Description Memulai pengerjaan (mencatat waktu mulai di server)
// @Tags Praktikum - Pengerjaan
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.MulaiCourseRequest true "Payload Mulai"
// @Success 200 {object} response.Envelope
// @Router /praktikum/mulai [post]
func (h *JawabanHandler) Mulai(c *gin.Context) {
	var req dto.MulaiCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.Mulai(middleware.UserID(c), req)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Mulai mengerjakan", res)
}

// AutoSave POST /api/praktikum/autosave
// @Summary Auto-Save Jawaban
// @Description Menyimpan jawaban untuk satu soal (dipanggil berkala oleh frontend)
// @Tags Praktikum - Pengerjaan
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.AutoSaveRequest true "Payload Autosave"
// @Success 200 {object} response.Envelope
// @Router /praktikum/autosave [post]
func (h *JawabanHandler) AutoSave(c *gin.Context) {
	var req dto.AutoSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.uc.AutoSave(middleware.UserID(c), req); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Tersimpan", nil)
}

// Submit POST /api/praktikum/submit
// @Summary Kumpulkan Ujian
// @Description Mengumpulkan seluruh jawaban dan menutup course (Manual Submit)
// @Tags Praktikum - Pengerjaan
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.SubmitRequest true "Payload Submit"
// @Success 200 {object} response.Envelope
// @Router /praktikum/submit [post]
func (h *JawabanHandler) Submit(c *gin.Context) {
	var req dto.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.uc.Submit(middleware.UserID(c), req); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Jawaban ter-submit", nil)
}

// AdminInjectJawaban POST /api/admin/jawaban/inject
// @Summary Admin Inject/Edit Jawaban
// @Description Admin menginject jawaban baru atau mengedit jawaban existing untuk mahasiswa tertentu
// @Tags Admin - Jawaban
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.AdminInjectJawabanRequest true "Payload Inject"
// @Success 200 {object} response.Envelope
// @Router /admin/jawaban/inject [post]
func (h *JawabanHandler) AdminInjectJawaban(c *gin.Context) {
	var req dto.AdminInjectJawabanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.uc.AdminInjectJawaban(req); err != nil {
		mapError(c, err)
		return
	}
	// CR-M2: injeksi/penimpaan jawaban mahasiswa adalah aksi istimewa —
	// wajib tercatat di audit log (dulu tanpa jejak sama sekali).
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "ADMIN_INJECT_JAWABAN",
		fmt.Sprintf("Inject jawaban: mahasiswa_id=%d soal_terpilih_id=%d auto_submit=%v",
			req.MahasiswaID, req.SoalTerpilihID, req.AutoSubmit),
		clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Jawaban berhasil diinject", nil)
}
