package handler

import (
	"net/http"

	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// CronHandler menangani job terjadwal yang dipicu cron eksternal (cron-job.org).
// Menggantikan sweeper goroutine yang tidak bisa hidup di serverless.
type CronHandler struct {
	jawaban *usecase.JawabanUsecase
	rumpang *usecase.RumpangUsecase
	secret  string
}

func NewCronHandler(j *usecase.JawabanUsecase, r *usecase.RumpangUsecase, secret string) *CronHandler {
	return &CronHandler{jawaban: j, rumpang: r, secret: secret}
}

// AutoSubmit POST /api/cron/auto-submit
// @Summary Auto-submit pengerjaan kedaluwarsa (cron)
// @Description Dipicu cron eksternal tiap ~1 menit. Wajib header X-Cron-Secret.
// @Tags Cron
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /cron/auto-submit [post]
func (h *CronHandler) AutoSubmit(c *gin.Context) {
	// Tolak bila secret kosong (salah konfigurasi) atau tidak cocok.
	if h.secret == "" || c.GetHeader("X-Cron-Secret") != h.secret {
		response.Fail(c, http.StatusUnauthorized, "Akses ditolak", nil)
		return
	}
	n, err := h.jawaban.AutoSubmitExpired()
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Gagal auto-submit", err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Auto-submit selesai", gin.H{"submitted": n})
}

// Rumpang POST /api/cron/rumpang
// @Summary Buatkan kode rumpang yang belum ada (cron)
// @Description Dipicu cron eksternal tiap ~30 menit. Menyapu soal coding keterampilan yang punya kunci tapi belum punya kode_rumpang. Wajib header X-Cron-Secret.
// @Tags Cron
// @Produce json
// @Success 200 {object} response.Envelope
// @Router /cron/rumpang [post]
func (h *CronHandler) Rumpang(c *gin.Context) {
	// Tolak bila secret kosong (salah konfigurasi) atau tidak cocok.
	if h.secret == "" || c.GetHeader("X-Cron-Secret") != h.secret {
		response.Fail(c, http.StatusUnauthorized, "Akses ditolak", nil)
		return
	}
	n, gagal, err := h.rumpang.Sapu(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Gagal menyapu rumpang", err.Error())
		return
	}
	response.OK(c, http.StatusOK, "Sapu rumpang selesai", gin.H{"dibuat": n, "gagal": gagal})
}
