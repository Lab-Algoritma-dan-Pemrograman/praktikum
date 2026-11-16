package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/entity"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type KonfigurasiHandler struct {
	uc             *usecase.KonfigurasiUsecase
	defaultAIModel string // env OLLAMA_MODEL — dipakai bila override ai_model kosong
	auditLog       *usecase.AuditLogUsecase
}

func NewKonfigurasiHandler(uc *usecase.KonfigurasiUsecase, defaultAIModel string, al *usecase.AuditLogUsecase) *KonfigurasiHandler {
	return &KonfigurasiHandler{uc: uc, defaultAIModel: defaultAIModel, auditLog: al}
}

// All GET /api/admin/konfigurasi
//
//	@Summary		Seluruh Konfigurasi
//	@Description	Mengambil semua data konfigurasi global
//	@Tags			Admin - Konfigurasi
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope{data=[]entity.Konfigurasi}
//	@Router			/admin/konfigurasi [get]
func (h *KonfigurasiHandler) All(c *gin.Context) {
	res, err := h.uc.All()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Konfigurasi", res)
}

// Set POST /api/admin/konfigurasi
//
//	@Summary		Set Konfigurasi
//	@Description	Mengubah atau menambah konfigurasi global (Upsert)
//	@Tags			Admin - Konfigurasi
//	@Security		bearerAuth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		dto.KonfigurasiRequest	true	"Payload Konfigurasi"
//	@Success		200		{object}	response.Envelope
//	@Router			/admin/konfigurasi [post]
func (h *KonfigurasiHandler) Set(c *gin.Context) {
	var req dto.KonfigurasiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.uc.Set(req.Key, req.Value); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "SET_KONFIGURASI", fmt.Sprintf("Mengubah konfigurasi '%s' menjadi '%s'", req.Key, req.Value), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Konfigurasi disimpan", nil)
}

// AIModel GET /api/admin/konfigurasi/ai-model
//
//	@Summary		Model AI aktif
//	@Description	Model AI grading yang sedang dipakai: override (key ai_model) jika ada, jika kosong pakai default server (env).
//	@Tags			Admin - Konfigurasi
//	@Security		bearerAuth
//	@Produce		json
//	@Success		200	{object}	response.Envelope
//	@Router			/admin/konfigurasi/ai-model [get]
func (h *KonfigurasiHandler) AIModel(c *gin.Context) {
	override, _ := h.uc.Get(entity.KeyAIModel)
	effective := override
	if effective == "" {
		effective = h.defaultAIModel
	}
	response.OK(c, http.StatusOK, "Model AI aktif", gin.H{
		"effective": effective,
		"override":  override,
		"default":   h.defaultAIModel,
	})
}

// ---- Publik (info) ----

// PublicModul GET /api/info/modul
//
//	@Summary		File Modul Publik
//	@Description	Mengambil URL publik dari Modul Praktikum
//	@Tags			Info
//	@Produce		json
//	@Success		200	{object}	response.Envelope
//	@Router			/info/modul [get]
func (h *KonfigurasiHandler) PublicModul(c *gin.Context) {
	url, _ := h.uc.Get(entity.KeyModulFileURL)
	response.OK(c, http.StatusOK, "Modul praktikum", gin.H{"file_url": url})
}

// PublicJadwalConfig GET /api/info/jadwal/config
//
//	@Summary		Konfigurasi Jadwal Publik
//	@Description	Mengecek apakah jadwal menggunakan sistem internal atau GDrive URL
//	@Tags			Info
//	@Produce		json
//	@Success		200	{object}	response.Envelope
//	@Router			/info/jadwal/config [get]
func (h *KonfigurasiHandler) PublicJadwalConfig(c *gin.Context) {
	gdrive, _ := h.uc.Get(entity.KeyGDriveJadwalURL)
	mode, _ := h.uc.Get(entity.KeyJadwalMode)
	if mode == "" {
		mode = "internal"
	}
	response.OK(c, http.StatusOK, "Konfigurasi jadwal", gin.H{"mode": mode, "gdrive_url": gdrive})
}

// PublicAnnouncements GET /api/info/announcements
func (h *KonfigurasiHandler) PublicAnnouncements(c *gin.Context) {
	reschedules, _ := h.uc.Get("ann_reschedules")
	recruit, _ := h.uc.Get("ann_recruit")
	susulan, _ := h.uc.Get("ann_susulan")
	plagiarism, _ := h.uc.Get("ann_plagiarism")
	response.OK(c, http.StatusOK, "Announcements", gin.H{
		"reschedules": reschedules,
		"recruit":     recruit,
		"susulan":     susulan,
		"plagiarism":  plagiarism,
	})
}

// GetRolePermissions GET /api/superadmin/permissions
func (h *KonfigurasiHandler) GetRolePermissions(c *gin.Context) {
	raw, _ := h.uc.Get("role_permissions")
	if raw == "" {
		raw = "{}"
	}
	var m map[string]map[string]bool
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		m = map[string]map[string]bool{}
	}
	response.OK(c, http.StatusOK, "Role permissions", m)
}

// SetRolePermissions PUT /api/superadmin/permissions
func (h *KonfigurasiHandler) SetRolePermissions(c *gin.Context) {
	var perms map[string]map[string]bool
	if err := c.ShouldBindJSON(&perms); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	b, _ := json.Marshal(perms)
	if err := h.uc.Set("role_permissions", string(b)); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "SET_ROLE_PERMISSIONS", "Update role_permissions", clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Permissions disimpan", perms)
}
