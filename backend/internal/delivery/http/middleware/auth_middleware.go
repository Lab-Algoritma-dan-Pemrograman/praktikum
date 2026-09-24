package middleware

import (
	"net/http"
	"strings"

	"lab-ap/config"
	"lab-ap/internal/repository"
	"lab-ap/pkg/jwt"
	"lab-ap/pkg/response"
	supajwt "lab-ap/pkg/supabasejwt"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID = "user_id"
	CtxNIM    = "nim"
	CtxRole   = "role"
)

// Auth memverifikasi JWT dual-mode: legacy HS256 lokal dan/atau Supabase RS256/ES256.
// Role untuk Supabase SELALU resolve dari DB via supabase_user_id (tidak dari claim).
func Auth(cfg *config.Config, jm *jwt.Manager, userRepo repository.UserRepository) gin.HandlerFunc {
	mode := strings.ToLower(strings.TrimSpace(cfg.AuthMode))
	if mode == "" {
		mode = "legacy"
	}
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, http.StatusUnauthorized, "Token tidak ditemukan", nil)
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		if mode == "legacy" {
			if legacyAuth(c, jm, tokenStr, userRepo) {
				c.Next()
				return
			}
			response.Fail(c, http.StatusUnauthorized, "Token tidak valid / kedaluwarsa", nil)
			return
		}
		if mode == "supabase" {
			if supabaseAuth(c, cfg, userRepo, tokenStr) {
				c.Next()
				return
			}
			response.Fail(c, http.StatusUnauthorized, "Token tidak valid / kedaluwarsa", nil)
			return
		}
		// dual: coba Supabase dulu lalu legacy
		if supabaseAuth(c, cfg, userRepo, tokenStr) {
			c.Next()
			return
		}
		if legacyAuth(c, jm, tokenStr, userRepo) {
			c.Next()
			return
		}
		response.Fail(c, http.StatusUnauthorized, "Token tidak valid / kedaluwarsa", nil)
	}
}

func legacyAuth(c *gin.Context, jm *jwt.Manager, tokenStr string, userRepo repository.UserRepository) bool {
	claims, err := jm.Verify(tokenStr)
	if err != nil {
		return false
	}
	// CR-M3: identitas & ROLE di-resolve ulang dari DB, bukan dari claim.
	// Dulu claims.Role dipakai apa adanya, jadi user yang diturunkan/dihapus
	// tetap memegang akses admin sampai token 12 jam habis.
	if userRepo == nil {
		return false
	}
	u, err := userRepo.FindByID(claims.UserID)
	if err != nil || u == nil {
		return false
	}
	c.Set(CtxUserID, u.ID)
	c.Set(CtxNIM, u.NIM)
	c.Set(CtxRole, string(u.Role))
	return true
}

func supabaseAuth(c *gin.Context, cfg *config.Config, userRepo repository.UserRepository, tokenStr string) bool {
	claims, err := supajwt.VerifySupabaseToken(tokenStr, cfg.SupabaseJWKSURL, cfg.SupabaseJWTIssuer, cfg.SupabaseJWTAud)
	if err != nil {
		return false
	}
	if userRepo == nil {
		return false
	}
	u, err := userRepo.FindBySupabaseUserID(claims.Subject)
	if err != nil || u == nil {
		return false
	}
	c.Set(CtxUserID, u.ID)
	c.Set(CtxNIM, u.NIM)
	c.Set(CtxRole, string(u.Role))
	return true
}

// UserID helper
func UserID(c *gin.Context) int {
	if v, ok := c.Get(CtxUserID); ok {
		if id, ok := v.(int); ok {
			return id
		}
	}
	return 0
}

func Role(c *gin.Context) string {
	if v, ok := c.Get(CtxRole); ok {
		if r, ok := v.(string); ok {
			return r
		}
	}
	return ""
}
