package route

import (
	"net/http"
	"time"

	"lab-ap/config"
	"lab-ap/internal/delivery/http/handler"
	"lab-ap/internal/delivery/http/middleware"
	"lab-ap/internal/entity"
	"lab-ap/internal/repository"
	"lab-ap/pkg/jwt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Handlers mengumpulkan seluruh handler untuk diregistrasi.
type Handlers struct {
	Auth        *handler.AuthHandler
	Dashboard   *handler.DashboardHandler
	Sesi        *handler.SesiHandler
	Aktivasi    *handler.AktivasiHandler
	Soal        *handler.SoalHandler
	Jawaban     *handler.JawabanHandler
	Penilaian   *handler.PenilaianHandler
	Konfigurasi *handler.KonfigurasiHandler
	Profile     *handler.ProfileHandler
	Jadwal      *handler.JadwalHandler
	User        *handler.UserHandler
	Kelas       *handler.KelasHandler
	Pedoman     *handler.PedomanHandler
	Praktikum   *handler.PraktikumHandler
	Upload      *handler.UploadHandler
	Ampuan       *handler.AmpuanHandler
	Rekap        *handler.RekapHandler
	AIGrading    *handler.AIGradingHandler
	RekapJawaban *handler.RekapJawabanHandler
	Cron         *handler.CronHandler
	Run          *handler.RunHandler
	Compile      *handler.CompileHandler
	AuditLog     *handler.AuditLogHandler
	Belajar      *handler.BelajarHandler
	Game         *handler.GameHandler
}

// HealthCheck GET /api/health
// @Summary Health Check
// @Description Mengecek status service backend
// @Tags Info
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "lab-ap"})
}

// Setup membangun engine Gin + seluruh route & middleware.
func Setup(cfg *config.Config, jm *jwt.Manager, userRepo repository.UserRepository, konfRepo repository.KonfigurasiRepository, h Handlers) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.MaxMultipartMemory = 10 << 20 // 10 MiB: batasi buffer multipart di memori
	r.Use(gin.Logger(), gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/api/health", HealthCheck)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")

	// ---- Cron (dipicu cron eksternal, dijaga X-Cron-Secret) ----
	api.POST("/cron/auto-submit", h.Cron.AutoSubmit)

	// ---- Auth (publik) ----
	// Rate-limit untuk meredam brute-force: maks 20 percobaan/menit per IP.
	auth := api.Group("/auth")
	auth.Use(middleware.RateLimit(20, time.Minute))
	{
		auth.POST("/cek-nim", h.Auth.CekNIM)
		auth.POST("/login", h.Auth.Login)
		auth.POST("/register", h.Auth.Register)
		auth.POST("/forgot-password", h.Auth.ForgotPassword)
		auth.POST("/reset-password", h.Auth.ResetPassword)
	}

	// ---- Info (publik) ----
	info := api.Group("/info")
	{
		info.GET("/asisten", h.User.ListAsisten)
		info.GET("/laporan", h.Pedoman.List)
		info.GET("/modul", h.Konfigurasi.PublicModul)
		info.GET("/jadwal", h.Jadwal.List)
		info.GET("/jadwal/config", h.Konfigurasi.PublicJadwalConfig)
		info.GET("/kelas/:id/mahasiswa", h.Ampuan.PublicKelasMahasiswa)
		info.GET("/announcements", h.Konfigurasi.PublicAnnouncements)
	}

	authmw := middleware.Auth(cfg, jm, userRepo)

	// ---- Terautentikasi (semua role) ----
	authed := api.Group("")
	authed.Use(authmw)
	{
		authed.POST("/auth/logout", h.Auth.Logout)
		authed.GET("/auth/me", h.Auth.Me)
		authed.GET("/profile", h.Profile.Get)
		authed.PUT("/profile", h.Profile.Update)

		// Belajar (elearning). Materi di sini TANPA kunci jawaban.
		authed.GET("/belajar/level", h.Belajar.ListLevel)
		authed.GET("/belajar/modul", h.Belajar.ListModul)
		authed.GET("/belajar/materi", h.Belajar.ListMateri)
		authed.GET("/belajar/materi/:id", h.Belajar.GetMateri)
		authed.GET("/belajar/progres", h.Belajar.ProgresSaya)
		authed.POST("/belajar/materi/:id/selesai", h.Belajar.SelesaikanMateri)
		authed.GET("/belajar/pencapaian", h.Belajar.PencapaianSaya)
		authed.GET("/belajar/sesi", h.Belajar.SesiBelajar)

		// Game bug-hunt. Soal dikirim tanpa kunci; jawaban dinilai server.
		authed.GET("/game/konfigurasi", h.Game.Konfigurasi)
		authed.GET("/game/status", h.Game.StatusMain)
		authed.POST("/game/mulai", h.Game.MulaiMain)
		authed.POST("/game/selesai", h.Game.SelesaiMain)
		authed.GET("/game/peringkat", h.Game.Peringkat)
		authed.POST("/game/detak", h.Game.Detak)
	}

	// ---- Praktikum (role user) ----
	prak := api.Group("/praktikum")
	prak.Use(authmw, middleware.RequireRole(string(entity.RoleMahasiswa)))
	{
		prak.GET("/dashboard", h.Praktikum.Dashboard)
		prak.GET("/sesi", h.Praktikum.ListSesi)
		prak.GET("/ruang", h.Jawaban.GetRuang)
		prak.POST("/mulai", h.Jawaban.Mulai)
		prak.POST("/autosave", h.Jawaban.AutoSave)
		prak.POST("/submit", h.Jawaban.Submit)
		prak.POST("/run", h.Run.Execute)
		prak.POST("/compile-c", h.Compile.CompileC)
	}

	// ---- Admin (role admin + superadmin) ----
	admin := api.Group("/admin")
	admin.Use(authmw, middleware.RequireRole(string(entity.RoleAsisten), string(entity.RoleKoordinator)))
	{
		admin.GET("/dashboard", h.Dashboard.Statistik)

		// Users (mahasiswa)
		admin.GET("/users", h.User.ListMahasiswa)
		admin.POST("/users", h.User.CreateMahasiswa)
		admin.POST("/users/bulk", h.User.BulkUpsertMahasiswa)
		admin.PUT("/users/:id", h.User.UpdateMahasiswa)
		admin.DELETE("/users/:id", h.User.Delete)
		admin.POST("/users/:id/reset-password", h.User.ResetPassword)

		// Materi (LENGKAP dengan kunci jawaban -- jangan dipakai frontend mahasiswa)
		admin.GET("/materi", h.Belajar.AdminListMateri)
		admin.GET("/materi/:id", h.Belajar.AdminGetMateri)
		admin.POST("/materi", h.Belajar.AdminCreateMateri)
		admin.PUT("/materi/:id", h.Belajar.AdminUpdateMateri)
		admin.DELETE("/materi/:id", h.Belajar.AdminDeleteMateri)

		// Belajar: koreksi progres mahasiswa
		admin.PUT("/belajar/kurikulum", h.Belajar.AdminSimpanKurikulum)
		admin.DELETE("/belajar/kurikulum", h.Belajar.AdminKosongkanKurikulum)
		admin.GET("/belajar/users", h.Belajar.AdminListUserBelajar)
		admin.GET("/belajar/users/:id/progres", h.Belajar.AdminProgresUser)
		admin.PUT("/belajar/users/:id/akses-level", h.Belajar.AdminSetAksesLevel)
		admin.POST("/belajar/users/:id/reset", h.Belajar.AdminResetProgres)
		admin.PUT("/belajar/users/:id/xp", h.Belajar.AdminSetXP)

		// Game & monitoring
		admin.GET("/monitoring/sesi", h.Game.SesiAktif)
		admin.GET("/game/soal", h.Game.AdminListSoal)
		admin.POST("/game/soal", h.Game.AdminCreateSoal)
		admin.PUT("/game/soal/:id", h.Game.AdminUpdateSoal)
		admin.DELETE("/game/soal/:id", h.Game.AdminDeleteSoal)
		admin.PUT("/game/konfigurasi", h.Game.AdminSimpanKonfigurasi)

		// Kelas
		admin.GET("/kelas", h.Kelas.List)
		admin.POST("/kelas", h.Kelas.Create)
		admin.PUT("/kelas/:id", h.Kelas.Update)
		admin.DELETE("/kelas/:id", h.Kelas.Delete)

		// Asisten
		admin.GET("/asisten", h.User.ListAsisten)
		admin.POST("/asisten", h.User.CreateAsisten)
		admin.PUT("/asisten/:id", h.User.UpdateAsisten)

		// Jadwal
		admin.GET("/jadwal", h.Jadwal.List)
		admin.POST("/jadwal", h.Jadwal.Create)
		admin.PUT("/jadwal/:id", h.Jadwal.Update)
		admin.DELETE("/jadwal/:id", h.Jadwal.Delete)

		// Pedoman
		admin.GET("/pedoman", h.Pedoman.List)
		admin.POST("/pedoman", h.Pedoman.Create)
		admin.PUT("/pedoman/:id", h.Pedoman.Update)
		admin.DELETE("/pedoman/:id", h.Pedoman.Delete)

		// Konfigurasi (termasuk modul & gdrive jadwal URL)
		admin.GET("/konfigurasi", h.Konfigurasi.All)
		admin.GET("/konfigurasi/ai-model", h.Konfigurasi.AIModel)
		admin.POST("/konfigurasi", h.Konfigurasi.Set)

		// Sesi & Course
		admin.GET("/sesi", h.Sesi.List)
		admin.GET("/sesi/:id", h.Sesi.Get)
		admin.POST("/sesi", h.Sesi.Create)
		admin.PUT("/sesi/:id", h.Sesi.Update)
		admin.DELETE("/sesi/:id", h.Sesi.Delete)
		admin.GET("/sesi/:id/course", h.Sesi.ListCourse)
		admin.POST("/sesi/:id/course", h.Sesi.CreateCourse)
		admin.PUT("/course/:courseId", h.Sesi.UpdateCourse)
		admin.DELETE("/course/:courseId", h.Sesi.DeleteCourse)

		// Soal
		admin.GET("/soal", h.Soal.ListByCourse)
		admin.POST("/soal", h.Soal.Create)
		admin.PUT("/soal/:id", h.Soal.Update)
		admin.DELETE("/soal/:id", h.Soal.Delete)

		// Aktivasi
		admin.GET("/aktivasi", h.Aktivasi.List)
		admin.POST("/aktivasi", h.Aktivasi.Aktivasi)
		admin.GET("/aktivasi/:id", h.Aktivasi.Get)
		admin.DELETE("/aktivasi/:id", h.Aktivasi.Delete)
		admin.POST("/aktivasi/:id/token", h.Aktivasi.GenerateToken)
		admin.POST("/aktivasi/:id/susulan", h.Aktivasi.AddSusulan)
		admin.GET("/aktivasi/:id/susulan", h.Aktivasi.ListSusulan)
		admin.GET("/aktivasi/:id/peserta", h.Aktivasi.Peserta)
		admin.DELETE("/aktivasi/:id/susulan/:mahasiswaId", h.Aktivasi.RemoveSusulan)
		admin.POST("/aktivasi-course/buka-tutup", h.Aktivasi.BukaTutupCourse)

		// Penilaian
		admin.GET("/penilaian/rekap", h.Penilaian.Rekap)
		admin.POST("/penilaian", h.Penilaian.SetNilai)
		admin.POST("/keaktifan", h.Penilaian.SetKeaktifan)
		// AI grading SINKRON 1-per-1 (frontend yang loop).
		admin.GET("/penilaian/ai-grade/targets", h.AIGrading.ListTargets)
		admin.POST("/penilaian/ai-grade/one", h.AIGrading.GradeOne)

		// Rekap Jawaban Global & Bulk Actions
		admin.GET("/rekap-jawaban", h.RekapJawaban.GetRekapJawabanGlobal)
		admin.POST("/penilaian/bulk-action", h.RekapJawaban.BulkAction)
		admin.POST("/jawaban/inject", h.Jawaban.AdminInjectJawaban)

		// Rekap
		admin.GET("/rekap/kelas/:id_kelas", h.Rekap.GetRekapKelas)

		// Upload (Supabase)
		admin.POST("/upload", h.Upload.Upload)

		admin.GET("/ampuan", h.Ampuan.List)
		admin.POST("/ampuan", h.Ampuan.Create)
		admin.DELETE("/ampuan/:id", h.Ampuan.Delete)

		// Audit Log
		admin.GET("/audit-logs", h.AuditLog.GetLogs)
	}

	// ---- Superadmin (role superadmin only, hidden ninja) ----
	superadmin := api.Group("/superadmin")
	superadmin.Use(authmw, middleware.RequireRole(string(entity.RoleKoordinator)))
	{
		superadmin.GET("/permissions", h.Konfigurasi.GetRolePermissions)
		superadmin.PUT("/permissions", h.Konfigurasi.SetRolePermissions)
	}

	return r
}
