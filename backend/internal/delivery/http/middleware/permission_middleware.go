package middleware

import (
	"encoding/json"
	"net/http"

	"lab-ap/internal/repository"
	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequirePermission cek konfigurasi key=role_permissions JSON {"asisten":{"users:write":true}}
// superadmin selalu bypass. Lazy: baca dari DB tiap request (kecil, jarang ganti).
func RequirePermission(konfRepo repository.KonfigurasiRepository, resource, action string) gin.HandlerFunc {
	key := resource + ":" + action
	return func(c *gin.Context) {
		if Role(c) == "koordinator" {
			c.Next()
			return
		}
		// tanpa repo → allow (fallback saat bootstrap)
		if konfRepo == nil {
			c.Next()
			return
		}
		konf, err := konfRepo.Get("role_permissions")
		if err != nil || konf == nil || konf.Value == "" {
			c.Next()
			return
		}
		var perms map[string]map[string]bool
		if err := json.Unmarshal([]byte(konf.Value), &perms); err != nil {
			c.Next()
			return
		}
		role := Role(c)
		if m, ok := perms[role]; ok {
			if allowed, ok := m[key]; ok && !allowed {
				response.Fail(c, http.StatusForbidden, "Akses ditolak: hak "+key+" dinonaktifkan untuk role "+role, nil)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
