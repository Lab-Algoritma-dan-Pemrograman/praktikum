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

type UserHandler struct {
	uc       *usecase.UserUsecase
	auditLog *usecase.AuditLogUsecase
}

func NewUserHandler(uc *usecase.UserUsecase, al *usecase.AuditLogUsecase) *UserHandler {
	return &UserHandler{uc: uc, auditLog: al}
}

// ---- Mahasiswa ----

// ListMahasiswa GET /api/admin/users
// @Summary Daftar Mahasiswa
// @Description Mengambil data mahasiswa dengan filter opsional (kelas_id, shift)
// @Tags Admin - User
// @Security bearerAuth
// @Produce json
// @Param kelas_id query int false "ID Kelas"
// @Param shift query int false "Shift"
// @Success 200 {object} response.Envelope{data=[]entity.User}
// @Router /admin/users [get]
func (h *UserHandler) ListMahasiswa(c *gin.Context) {
	res, err := h.uc.ListMahasiswa(queryIntPtr(c, "kelas_id"), queryIntPtr(c, "shift"))
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar mahasiswa", res)
}

// CreateMahasiswa POST /api/admin/users
// @Summary Tambah Mahasiswa
// @Description Menambahkan data mahasiswa baru
// @Tags Admin - User
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UserRequest true "Payload Mahasiswa"
// @Success 201 {object} response.Envelope{data=entity.User}
// @Router /admin/users [post]
func (h *UserHandler) CreateMahasiswa(c *gin.Context) {
	var req dto.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.CreateMahasiswa(req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "CREATE_USER", "Membuat mahasiswa baru: "+res.Nama+" (NIM: "+res.NIM+")", clientIP(c), c.Request.UserAgent())
	response.Created(c, "Mahasiswa dibuat", res)
}

// BulkUpsertMahasiswa POST /api/admin/users/bulk
// @Summary Import Mahasiswa (Bulk)
// @Description Menambahkan atau memperbarui banyak data mahasiswa sekaligus dari JSON (biasanya hasil parsing CSV di Frontend). Jika NIM sudah ada, data akan di-update.
// @Tags Admin - User
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.UserBulkRequest true "Payload Bulk Mahasiswa"
// @Success 200 {object} response.Envelope{data=dto.BulkResponse}
// @Router /admin/users/bulk [post]
func (h *UserHandler) BulkUpsertMahasiswa(c *gin.Context) {
	var req dto.UserBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.BulkUpsertMahasiswa(req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "IMPORT_USER", fmt.Sprintf("Mengimpor %d mahasiswa secara bulk", len(req.Users)), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Import mahasiswa berhasil diproses", res)
}

// UpdateMahasiswa PUT /api/admin/users/:id
// @Summary Perbarui Mahasiswa
// @Description Memperbarui data mahasiswa (termasuk reset perangkat jika diatur)
// @Tags Admin - User
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Mahasiswa"
// @Param request body dto.UserRequest true "Payload Mahasiswa"
// @Success 200 {object} response.Envelope{data=entity.User}
// @Router /admin/users/{id} [put]
func (h *UserHandler) UpdateMahasiswa(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.UpdateMahasiswa(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "UPDATE_USER", "Memperbarui data mahasiswa: "+res.Nama+" (NIM: "+res.NIM+")", clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Mahasiswa diperbarui", res)
}

// Delete DELETE /api/admin/users/:id
// @Summary Hapus User
// @Description Menghapus pengguna (mahasiswa) dari sistem
// @Tags Admin - User
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID User"
// @Success 200 {object} response.Envelope
// @Router /admin/users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.Delete(id, middleware.UserID(c), middleware.Role(c)); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "DELETE_USER", fmt.Sprintf("Menghapus user dengan ID %d", id), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "User dihapus", nil)
}

// ResetPassword POST /api/admin/users/:id/reset-password
// @Summary Reset Password
// @Description Me-reset password mahasiswa (kosong). Mahasiswa harus register ulang.
// @Tags Admin - User
// @Security bearerAuth
// @Produce json
// @Param id path int true "ID Mahasiswa"
// @Success 200 {object} response.Envelope
// @Router /admin/users/{id}/reset-password [post]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := h.uc.ResetPassword(id); err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "RESET_PASSWORD", fmt.Sprintf("Mereset password mahasiswa dengan ID %d", id), clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Password direset (mahasiswa harus register ulang)", nil)
}

// ---- Asisten ----

// ListAsisten GET /api/admin/asisten & /api/info/asisten
// @Summary Daftar Asisten
// @Description Mengambil daftar seluruh asisten
// @Tags Info, Admin - Asisten
// @Security bearerAuth
// @Produce json
// @Success 200 {object} response.Envelope{data=[]entity.User}
// @Router /info/asisten [get]
// @Router /admin/asisten [get]
func (h *UserHandler) ListAsisten(c *gin.Context) {
	res, err := h.uc.ListAsisten()
	if err != nil {
		mapError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "Daftar asisten", res)
}

// CreateAsisten POST /api/admin/asisten
// @Summary Tambah Asisten
// @Description Menambahkan akun asisten baru
// @Tags Admin - Asisten
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param request body dto.AsistenRequest true "Payload"
// @Success 201 {object} response.Envelope{data=entity.User}
// @Router /admin/asisten [post]
func (h *UserHandler) CreateAsisten(c *gin.Context) {
	var req dto.AsistenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.CreateAsisten(req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "CREATE_USER", "Membuat asisten baru: "+res.Nama+" (NIM: "+res.NIM+")", clientIP(c), c.Request.UserAgent())
	response.Created(c, "Asisten dibuat", res)
}

// UpdateAsisten PUT /api/admin/asisten/:id
// @Summary Perbarui Asisten
// @Description Mengubah data akun asisten
// @Tags Admin - Asisten
// @Security bearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID Asisten"
// @Param request body dto.AsistenRequest true "Payload"
// @Success 200 {object} response.Envelope{data=entity.User}
// @Router /admin/asisten/{id} [put]
func (h *UserHandler) UpdateAsisten(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var req dto.AsistenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Input tidak valid", err.Error())
		return
	}
	res, err := h.uc.UpdateAsisten(id, req)
	if err != nil {
		mapError(c, err)
		return
	}
	_ = h.auditLog.LogAction(middleware.UserID(c), "", "UPDATE_USER", "Memperbarui data asisten: "+res.Nama+" (NIM: "+res.NIM+")", clientIP(c), c.Request.UserAgent())
	response.OK(c, http.StatusOK, "Asisten diperbarui", res)
}
