// Package handler adalah entrypoint serverless untuk Vercel.
//
// CATATAN BUILD: file ini di-compile Vercel dari root repo (bukan backend/),
// sehingga import "lab-ap/internal/app" ditolak aturan internal package Go.
// Agar tetap satu sumber kebenaran, isi app.Build+middleware+router disalin
// kembar ke pkg/serverless/ (package public, bebas diimpor dari mana saja).
// Perubahan app.go wajib dicerminkan ke pkg/serverless/app.go.
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"lab-ap/config"
	_ "lab-ap/docs"
	"lab-ap/pkg/serverless"

	"github.com/gin-gonic/gin"
)

var (
	engine *gin.Engine
	initMu sync.Mutex

	allowedOrigins map[string]bool
	originsOnce    sync.Once
)

func getAllowedOrigins() map[string]bool {
	originsOnce.Do(func() {
		cfg := config.Load()
		allowedOrigins = map[string]bool{}
		for _, o := range strings.Split(strings.Join(cfg.CORSOrigins, ","), ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrigins[o] = true
			}
		}
	})
	return allowedOrigins
}

func applyCORS(h http.Header, origin string) {
	if origin == "" || !getAllowedOrigins()[origin] {
		return
	}
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Vary", "Origin")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	h.Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
}

// ensureInit merakit engine sekali per instance hangat. Bila gagal (mis. DB
// belum siap), error dikembalikan agar bisa dilaporkan ke klien, lalu dicoba
// lagi pada request berikutnya (bukan crash).
func ensureInit() error {
	initMu.Lock()
	defer initMu.Unlock()

	if engine != nil {
		return nil
	}

	gin.SetMode(gin.ReleaseMode)
	cfg := config.Load()
	var err error
	engine, _, err = serverless.Build(cfg)
	if err != nil {
		return err
	}
	return nil
}

// Handler adalah entrypoint yang dipanggil Vercel untuk tiap request.
func Handler(w http.ResponseWriter, r *http.Request) {
	applyCORS(w.Header(), r.Header.Get("Origin"))
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := ensureInit(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"success":false,"message":"startup error","error":` + strconv.Quote(err.Error()) + `}`))
		return
	}
	// Vercel rewrite /(.*) -> /api/index?__path=$1 merusak path asli.
	// Path asli dikirim sebagai query __path. Restore sebelum ServeHTTP.
	if r.URL.Path == "/api/index" {
		if p := r.URL.Query().Get("__path"); p != "" {
			if !strings.HasPrefix(p, "/") {
				p = "/" + p
			}
			r.URL.Path = p
		}
	}
	engine.ServeHTTP(w, r)
}
