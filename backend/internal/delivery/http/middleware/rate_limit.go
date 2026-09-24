package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"lab-ap/pkg/response"

	"github.com/gin-gonic/gin"
)

// rateLimiter adalah pembatas laju berbasis fixed window in-memory.
// Dipakai untuk melindungi endpoint sensitif (login/register/cek-nim) dari
// brute-force, dan endpoint mahal (run/compile-c) dari penyalahgunaan.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string]*window
	limit  int
	window time.Duration
}

type window struct {
	count int
	reset time.Time
}

// RateLimit membuat middleware yang membatasi maksimum `limit` request per `window`
// untuk tiap IP. Entri kedaluwarsa dibersihkan periodik oleh janitor.
func RateLimit(limit int, win time.Duration) gin.HandlerFunc {
	return newRateLimiter(limit, win, func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	})
}

// RateLimitUser membatasi per USER (JWT), fallback ke IP bila belum terautentikasi.
// Dipakai untuk /praktikum/run dan /praktikum/compile-c: dua endpoint ini
// mengeksekusi compiler di server, jadi satu akun tidak boleh membanjiri antrian
// (DoS CPU/RAM). Satu IP dengan banyak akun tetap terbatas karena kuncinya user.
func RateLimitUser(limit int, win time.Duration) gin.HandlerFunc {
	return newRateLimiter(limit, win, func(c *gin.Context) string {
		if uid := UserID(c); uid > 0 {
			return "user:" + strconv.Itoa(uid)
		}
		return "ip:" + c.ClientIP()
	})
}

func newRateLimiter(limit int, win time.Duration, keyFn func(*gin.Context) string) gin.HandlerFunc {
	rl := &rateLimiter{
		hits:   make(map[string]*window),
		limit:  limit,
		window: win,
	}
	rl.startJanitor()

	return func(c *gin.Context) {
		if !rl.allow(keyFn(c)) {
			response.Fail(c, http.StatusTooManyRequests,
				"Terlalu banyak permintaan. Coba lagi beberapa saat.", nil)
			return
		}
		c.Next()
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	w, ok := rl.hits[key]
	if !ok || now.After(w.reset) {
		rl.hits[key] = &window{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if w.count >= rl.limit {
		return false
	}
	w.count++
	return true
}

// startJanitor membersihkan entri kedaluwarsa secara berkala agar map tidak tumbuh tanpa batas.
func (rl *rateLimiter) startJanitor() {
	go func() {
		t := time.NewTicker(rl.window)
		defer t.Stop()
		for range t.C {
			rl.mu.Lock()
			now := time.Now()
			for k, w := range rl.hits {
				if now.After(w.reset) {
					delete(rl.hits, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
}
