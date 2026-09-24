package handler

import (
	"net/http"

	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/dto"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth     *usecase.AuthUsecase
	profile  *usecase.ProfileUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewAuthHandler(a *usecase.AuthUsecase, p *usecase.ProfileUsecase, al *usecase.AuditLogUsecase) *AuthHandler {
	return &AuthHandler{auth: a, profile: p, auditLog: al}
}

// CekNIM POST /api/auth/cek-nim
// @Summary Cek status NIM
// @Description Memeriksa apakah NIM sudah terdaftar (langkah 1 first-time login)
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.CekNIMRequest true "Payload NIM"
// @Success 200 {object} response.Envelope{data=dto.CekNIMResponse}
// @Failure 400 {object} response.Envelope
// @Router /auth/cek-nim [post]
func (h *AuthHandler) CekNIM(c *gin.Context) {
	var req dto.CekNIMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "NIM wajib diisi", err.Error())
		return
	}
	res, err := h.auth.CekNIM(req.NIM)
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Status NIM", res)
}

// Login POST /api/auth/login
// @Summary Login Pengguna
// @Description Login menggunakan NIM dan Password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Payload Login"
// @Success 200 {object} response.Envelope{data=dto.AuthResponse}
// @Failure 400 {object} response.Envelope
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.auth.Login(req)
	if err != nil {
		// MEDIUM-01: pesan generik — user-ada vs user-tidak-ada vs password
		// salah semuanya balas 401 yang sama (mencegah enumerasi NIM/email).
		_ = h.auditLog.LogAction(0, req.Identifier, "LOGIN_FAILED", "Gagal login", clientIP(c), c.Request.UserAgent())
		response.Fail(c, http.StatusUnauthorized, "NIM/email atau password salah", nil)
		return
	}
	_ = h.auditLog.LogAction(0, req.Identifier, "LOGIN", "Login berhasil", clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Login berhasil", res)
}

// Register POST /api/auth/register
// @Summary Registrasi Mahasiswa
// @Description Mendaftarkan password untuk pertama kali bagi mahasiswa yang sudah memiliki NIM terdaftar
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Payload Register"
// @Success 201 {object} response.Envelope{data=dto.AuthResponse}
// @Failure 400 {object} response.Envelope
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Password minimal 6 karakter", err.Error())
		return
	}
	res, err := h.auth.Register(req)
	if err != nil {
		_ = h.auditLog.LogAction(0, req.NIM, "REGISTER_FAILED", "Gagal registrasi: "+err.Error(), clientIP(c), c.Request.UserAgent())
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(0, req.NIM, "REGISTER", "Registrasi akun berhasil", clientIP(c), c.Request.UserAgent())
	response.Created(c, "Registrasi berhasil", res)
}

// Logout POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.UserID(c)
	_ = h.auditLog.LogAction(userID, "", "LOGOUT", "Logout berhasil", clientIP(c), c.Request.UserAgent())
	h.auth.Logout(userID)
	response.OK(c, http.StatusOK, "Logout berhasil", nil)
}

// ForgotPassword POST /api/auth/forgot-password (selalu 200 generik)
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Email tidak valid", err.Error())
		return
	}
	_ = h.auth.ForgotPassword(req.Email)
	response.OK(c, http.StatusOK, "Jika email terdaftar, kode OTP telah dikirim. Cek inbox Anda.", nil)
}

// ResetPassword POST /api/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordViaOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	if err := h.auth.ResetPassword(req); err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Password berhasil direset. Silakan login.", nil)
}

// Me GET /api/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	u, err := h.profile.Get(middleware.UserID(c))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Profil", u)
}
