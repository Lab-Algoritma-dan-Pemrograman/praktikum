// Package serverless membungkus internal/app supaya bisa diimpor dari
// entrypoint serverless Vercel (api/index.go), yang di-compile dari root
// repo sehingga terkena aturan "internal package" Go.
//
// ponytail: thin shim. Kalau root directory Vercel sudah di-set ke backend/,
// file ini tak diperlukan lagi — api/index.go bisa langsung import internal/app.
// Saat itu hapus file ini dan kembalikan import di api/index.go.
package serverless

import (
	"lab-ap/config"
	"lab-ap/internal/app"

	"github.com/gin-gonic/gin"
)

// Build adalah alias ke app.Build. Lihat app.go untuk isi sebenarnya.
func Build(cfg *config.Config) (*gin.Engine, *app.Deps, error) {
	return app.Build(cfg)
}
