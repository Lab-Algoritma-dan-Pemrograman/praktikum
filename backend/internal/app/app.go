// Package app adalah composition root: merakit seluruh dependency (DB, repository,
// usecase, handler, router) jadi satu *gin.Engine. Dipakai bersama oleh entrypoint
// server lokal (cmd/server) maupun entrypoint serverless Vercel (api/index.go),
// supaya tidak ada duplikasi wiring.
package app

import (
	"lab-ap/config"
	"lab-ap/database"
	"lab-ap/internal/delivery/http/handler"
	"lab-ap/internal/delivery/http/route"
	"lab-ap/internal/repository"
	"lab-ap/internal/usecase"
	"lab-ap/pkg/cwasm"
	"lab-ap/pkg/glot"
	"lab-ap/pkg/hash"
	"lab-ap/pkg/jwt"
	"lab-ap/pkg/ollama"
	"lab-ap/pkg/supabase"

	"github.com/gin-gonic/gin"
)

// Deps menampung dependency yang masih dibutuhkan oleh pemanggil setelah Build
// (mis. server lokal butuh JawabanUC untuk menjalankan sweeper goroutine).
type Deps struct {
	JawabanUC *usecase.JawabanUsecase
}

// Build merakit semua dependency dan mengembalikan engine Gin siap pakai.
func Build(cfg *config.Config) (*gin.Engine, *Deps, error) {
	db, err := database.Connect(cfg.DSN(), cfg.AppEnv == "development")
	if err != nil {
		return nil, nil, err
	}

	jm := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpireHours)
	sb := supabase.New(cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.SupabaseBucket)
	oc := ollama.NewClient(cfg)
	glotClient := glot.New(cfg.GlotURL, cfg.GlotToken)
	cwasmCompiler := cwasm.New(cfg.CwasmClang, cfg.CwasmSysroot)

	// ---- Repository ----
	userRepo := repository.NewUserRepository(db)
	kelasRepo := repository.NewKelasRepository(db)
	jadwalRepo := repository.NewJadwalRepository(db)
	konfRepo := repository.NewKonfigurasiRepository(db)
	pedomanRepo := repository.NewPedomanRepository(db)
	sesiRepo := repository.NewSesiRepository(db)
	courseRepo := repository.NewCourseRepository(db)
	soalRepo := repository.NewSoalRepository(db)
	aktivasiRepo := repository.NewAktivasiRepository(db)
	terpilihRepo := repository.NewSoalTerpilihRepository(db)
	jawabanRepo := repository.NewJawabanRepository(db)
	pengerjaanRepo := repository.NewPengerjaanRepository(db)
	ampuanRepo := repository.NewAmpuanRepository(db)
	rekapRepo := repository.NewRekapRepository(db)
	penilaianTxRepo := repository.NewPenilaianTxRepo(db)
	aktivasiTxRepo := repository.NewAktivasiTxRepo(db)
	auditLogRepo := repository.NewAuditLogRepository(db)
	belajarRepo := repository.NewBelajarRepository(db)

	// ---- Usecase ----
	authUC := usecase.NewAuthUsecase(userRepo, kelasRepo, jm, cfg, hash.FbScryptConfig{
		SignerKey:     cfg.FbScryptSignerKey,
		SaltSeparator: cfg.FbScryptSaltSeparator,
		Rounds:        cfg.FbScryptRounds,
		MemCost:       cfg.FbScryptMemCost,
	})
	profileUC := usecase.NewProfileUsecase(userRepo)
	dashboardUC := usecase.NewDashboardUsecase(userRepo, aktivasiRepo, pengerjaanRepo)
	sesiUC := usecase.NewSesiUsecase(sesiRepo, courseRepo)
	soalUC := usecase.NewSoalUsecase(soalRepo, courseRepo, terpilihRepo)
	aktivasiUC := usecase.NewAktivasiUsecase(aktivasiRepo, sesiRepo, courseRepo, kelasRepo, jawabanRepo, pengerjaanRepo, soalUC, aktivasiTxRepo)
	jawabanUC := usecase.NewJawabanUsecase(aktivasiRepo, terpilihRepo, jawabanRepo, pengerjaanRepo, userRepo, courseRepo)
	penilaianUC := usecase.NewPenilaianUsecase(jawabanRepo, pengerjaanRepo, userRepo, penilaianTxRepo)
	konfUC := usecase.NewKonfigurasiUsecase(konfRepo)
	userUC := usecase.NewUserUsecase(userRepo, kelasRepo)
	kelasUC := usecase.NewKelasUsecase(kelasRepo)
	jadwalUC := usecase.NewJadwalUsecase(jadwalRepo)
	pedomanUC := usecase.NewPedomanUsecase(pedomanRepo)
	praktikumUC := usecase.NewPraktikumUsecase(sesiRepo, courseRepo, aktivasiRepo, pengerjaanRepo, userRepo, jadwalRepo, terpilihRepo)
	ampuanUC := usecase.NewAmpuanUsecase(ampuanRepo)
	rekapUC := usecase.NewRekapUsecase(rekapRepo, kelasRepo)
	aiGradingUC := usecase.NewAIGradingUsecase(jawabanRepo, penilaianUC, oc, konfRepo)
	auditLogUC := usecase.NewAuditLogUsecase(auditLogRepo, userRepo)
	belajarUC := usecase.NewBelajarUsecase(belajarRepo)

	// ---- Handler ----
	h := route.Handlers{
		Auth:         handler.NewAuthHandler(authUC, profileUC, auditLogUC),
		Dashboard:    handler.NewDashboardHandler(dashboardUC),
		Sesi:         handler.NewSesiHandler(sesiUC, auditLogUC),
		Aktivasi:     handler.NewAktivasiHandler(aktivasiUC),
		Soal:         handler.NewSoalHandler(soalUC, auditLogUC),
		Jawaban:      handler.NewJawabanHandler(jawabanUC),
		Penilaian:    handler.NewPenilaianHandler(penilaianUC, auditLogUC),
		Konfigurasi:  handler.NewKonfigurasiHandler(konfUC, cfg.OllamaModel, auditLogUC),
		Profile:      handler.NewProfileHandler(profileUC),
		Jadwal:       handler.NewJadwalHandler(jadwalUC),
		User:         handler.NewUserHandler(userUC, auditLogUC),
		Kelas:        handler.NewKelasHandler(kelasUC),
		Pedoman:      handler.NewPedomanHandler(pedomanUC),
		Praktikum:    handler.NewPraktikumHandler(praktikumUC),
		Upload:       handler.NewUploadHandler(sb),
		Ampuan:       handler.NewAmpuanHandler(ampuanUC, userUC),
		Rekap:        handler.NewRekapHandler(rekapUC),
		AIGrading:    handler.NewAIGradingHandler(aiGradingUC),
		RekapJawaban: handler.NewRekapJawabanHandler(penilaianUC),
		Cron:         handler.NewCronHandler(jawabanUC, cfg.CronSecret),
		Run:          handler.NewRunHandler(glotClient),
		Compile:      handler.NewCompileHandler(cwasmCompiler),
		AuditLog:     handler.NewAuditLogHandler(auditLogUC),
		Belajar:      handler.NewBelajarHandler(belajarUC),
	}

	r := route.Setup(cfg, jm, userRepo, konfRepo, h)
	return r, &Deps{JawabanUC: jawabanUC}, nil
}
