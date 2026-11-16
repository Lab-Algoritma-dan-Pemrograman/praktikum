package handler

import (
	"net/http"
	"strconv"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuditLogHandler struct {
	uc *usecase.AuditLogUsecase
}

func NewAuditLogHandler(uc *usecase.AuditLogUsecase) *AuditLogHandler {
	return &AuditLogHandler{uc: uc}
}

// GetLogs GET /api/admin/audit-logs
//
//	@Summary		Lihat Log Aktivitas
//	@Description	Mengambil riwayat aktivitas sistem dengan filter & pagination
//	@Tags			Admin - Audit Log
//	@Security		bearerAuth
//	@Produce		json
//	@Param			search	query		string	false	"Cari NIM/Nama/Deskripsi"
//	@Param			role	query		string	false	"Filter Role (admin/user)"
//	@Param			action	query		string	false	"Filter Action"
//	@Param			page	query		int		false	"Nomor Halaman"
//	@Param			limit	query		int		false	"Jumlah data per halaman"
//	@Success		200		{object}	response.Envelope{data=object}
//	@Router			/admin/audit-logs [get]
func (h *AuditLogHandler) GetLogs(c *gin.Context) {
	search := c.Query("search")
	role := c.Query("role")
	action := c.Query("action")

	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	logs, total, err := h.uc.GetLogs(search, role, action, page, limit, middleware.Role(c))
	if err != nil {
		mapError(c, err)
		return
	}

	response.OK(c, http.StatusOK, "Log aktivitas berhasil diambil", gin.H{
		"logs":  logs,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
