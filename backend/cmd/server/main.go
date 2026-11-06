package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lab-ap/config"
	_ "lab-ap/docs"
	"lab-ap/internal/app"
	"lab-ap/internal/usecase"
)

// @title           Lab-AP API
// @version         1.0
// @description     REST API Documentation for Web Lab-AP v3
// @termsOfService  http://swagger.io/terms/

// @contact.name   Developer
// @contact.email  naufalraihans@example.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api
func main() {
	cfg := config.Load()

	r, deps, err := app.Build(cfg)
	if err != nil {
		log.Fatalf("Gagal merakit aplikasi: %v", err)
	}

	// ---- Background sweeper (hanya untuk server lokal/persisten) ----
	// Vercel Functions dapat dihentikan saat idle. Auto-submit di Vercel harus
	// dipicu scheduler lewat endpoint cron, bukan mengandalkan goroutine ini.
	stop := make(chan struct{})
	if os.Getenv("VERCEL") == "" {
		startSweeper(deps.JawabanUC, cfg.SweeperInterval, stop)
	} else {
		log.Println("[sweeper] Vercel: gunakan scheduler untuk /api/cron/auto-submit")
	}

	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}

	go func() {
		log.Printf("🚀 Server berjalan di http://localhost:%s (env=%s)", cfg.AppPort, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ---- Graceful shutdown ----
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Mematikan server...")
	close(stop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown paksa: %v", err)
	}
	log.Println("Server berhenti dengan rapi.")
}

// startSweeper menjalankan job periodik auto-submit (server lokal/persisten).
func startSweeper(uc *usecase.JawabanUsecase, interval time.Duration, stop <-chan struct{}) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if n, err := uc.AutoSubmitExpired(); err != nil {
					log.Printf("[sweeper] error: %v", err)
				} else if n > 0 {
					log.Printf("[sweeper] auto-submit %d pengerjaan kedaluwarsa", n)
				}
			}
		}
	}()
}
