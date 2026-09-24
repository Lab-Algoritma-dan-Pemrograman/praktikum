package middleware

import (
	"encoding/json"
	"net/http"

	"lab-ap/internal/repository"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequirePermission cek konfigurasi key=role_permissions JSON {"asisten":{"users:write":true}}
// koordinator selalu bypass. HIGH-04: FAIL-CLOSED — bila repo/config/JSON
// bermasalah, akses DITOLAK (kecuali koordinator), bukan dibiarkan lewat.
// Alasan: kegagalan baca config tidak boleh membuka pintu admin.
func RequirePermission(konfRepo repository.KonfigurasiRepository, resource, action string) gin.HandlerFunc {
	key := resource + ":" + action
	return func(c *gin.Context) {
		if Role(c) == "koordinator" {
			c.Next()
			return
		}
		deny := func(reason string) {
			response.Fail(c, http.StatusForbidden, "Akses ditolak: "+reason, nil)
			c.Abort()
		}
		if konfRepo == nil {
			deny("konfigurasi permission belum siap")
			return
		}
		konf, err := konfRepo.Get("role_permissions")
		if err != nil || konf == nil || konf.Value == "" {
			deny("konfigurasi permission belum diset (hubungi koordinator)")
			return
		}
		var perms map[string]map[string]bool
		if err := json.Unmarshal([]byte(konf.Value), &perms); err != nil {
			deny("konfigurasi permission rusak")
			return
		}
		// Explicit-deny menang; tanpa entri eksplisit per role+key, tolak juga.
		// Default ini aman karena seed wajib mengisi semua role non-koordinator
		// untuk semua key yang dipasang di router (lihat seed_role_permissions.sql).
		role := Role(c)
		if m, ok := perms[role]; ok {
			if allowed, ok := m[key]; ok && allowed {
				c.Next()
				return
			}
		}
		deny("hak " + key + " tidak diberikan untuk role " + role)
	}
}
