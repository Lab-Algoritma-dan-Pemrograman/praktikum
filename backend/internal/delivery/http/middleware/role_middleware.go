package middleware

import (
	"net/http"

	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequireRole memastikan user memiliki salah satu role yang diizinkan.
// Harus dipasang SETELAH Auth (response.Fail sudah AbortWithStatusJSON, jadi
// request yang ditolak tidak pernah sampai ke handler di belakangnya).
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		if !allowed[Role(c)] {
			response.Fail(c, http.StatusForbidden, "Akses ditolak untuk role Anda", nil)
			return
		}
		c.Next()
	}
}
