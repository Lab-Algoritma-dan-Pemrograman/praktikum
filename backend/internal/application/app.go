package application

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/algohub/algohub-rewrite/backend/internal/config"
	httpdelivery "github.com/algohub/algohub-rewrite/backend/internal/delivery/http"
	auth "github.com/algohub/algohub-rewrite/backend/internal/platform/auth"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/grading"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/openapi"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/preview"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/returnto"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/runner"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/storage"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
	"github.com/algohub/algohub-rewrite/backend/internal/usecase"
)

type App struct {
	Handler http.Handler
	Close   func()
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (App, error) {
	if err := cfg.Validate(); err != nil {
		return App{}, err
	}
	var repo repository.IdentityRepository
	var provider auth.IdentityProvider
	var readiness repository.ReadinessRepository
	var practicumRepo repository.PracticumRepository
	var learningRepo repository.LearningRepository
	var academicRepo repository.AcademicRepository
	var codeRunner usecase.PracticumRunner = runner.Unconfigured{}
	var grader usecase.PracticumGrader = grading.Unconfigured{}
	if cfg.GradingBaseURL != "" && !cfg.EnableFixtures {
		grader = grading.NewOpenAI(cfg.GradingBaseURL, cfg.GradingModel, cfg.GradingAPIKey)
	}
	closePool := func() {}
	if cfg.EnableFixtures {
		repo, provider, readiness = repository.NewMemoryIdentityRepository(), auth.NewFixtureProvider(), fixtureReadiness{}
		if cfg.PracticumEnabled || cfg.ElearningEnabled {
			codeRunner = runner.NewFixture()
		}
		if cfg.PracticumEnabled {
			practicumRepo = repository.NewMemoryPracticumRepository()
			grader = grading.NewFixture()
			if err := repository.SeedPracticumFixtures(ctx, practicumRepo); err != nil {
				return App{}, errors.New("could not seed isolated Practicum fixtures")
			}
		}
		if cfg.ElearningEnabled {
			learningRepo = repository.NewMemoryLearningRepository()
			if err := repository.SeedLearningFixtures(ctx, learningRepo); err != nil {
				return App{}, errors.New("could not seed isolated E-learning fixtures")
			}
		}
		if cfg.SiakadEnabled {
			academicRepo = repository.NewMemoryAcademicRepository()
			if err := repository.SeedAcademicFixtures(ctx, academicRepo); err != nil {
				return App{}, errors.New("could not seed isolated SIAKAD fixtures")
			}
		}
	} else {
		poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			return App{}, errors.New("invalid database connection configuration")
		}
		if cfg.Environment != "local" && poolConfig.ConnConfig.TLSConfig == nil {
			return App{}, errors.New("database TLS is required outside local development")
		}
		poolConfig.ConnConfig.Fallbacks = nil
		poolConfig.ConnConfig.ConnectTimeout = 3 * time.Second
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
		poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "5000"
		poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
		poolConfig.MaxConns, poolConfig.MinConns = cfg.DBMaxConns, 0
		poolConfig.MaxConnLifetime, poolConfig.MaxConnIdleTime = 30*time.Minute, 5*time.Minute
		pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
		if err != nil {
			return App{}, errors.New("could not initialize database pool")
		}
		repo, readiness, closePool = repository.NewPostgresIdentityRepository(pool), databaseReadiness{pool: pool, practicum: cfg.PracticumEnabled, elearning: cfg.ElearningEnabled, siakad: cfg.SiakadEnabled}, pool.Close
		if cfg.PracticumEnabled {
			practicumRepo = repository.NewPostgresPracticumRepository(pool)
		}
		if cfg.ElearningEnabled {
			learningRepo = repository.NewPostgresLearningRepository(pool)
		}
		if cfg.SiakadEnabled {
			academicRepo = repository.NewPostgresAcademicRepository(pool)
		}
		provider = auth.NewSupabaseProvider(cfg.SupabaseURL, cfg.SupabaseAnonKey, &http.Client{Timeout: 8 * time.Second})
	}
	identity := usecase.NewIdentityService(usecase.IdentityOptions{Repository: repo, Provider: provider, Allowlist: returnto.FromOrigins(cfg.SSOAllowedOrigins), SessionTTL: cfg.SessionTTL})
	budget := min(15*time.Second, cfg.RequestTimeout/2)
	var rumpangGen usecase.RumpangGenerator
	if openaiGrader, ok := grader.(*grading.OpenAI); ok {
		rumpangGen = openaiGrader
	}
	var practicum *usecase.PracticumService
	if cfg.PracticumEnabled {
		// Leave time for admission and finalization around bounded provider calls.
		practicum = usecase.NewPracticumService(usecase.PracticumOptions{
			Repository: practicumRepo, IdentityRepository: repo, Runner: codeRunner,
			Grader: grader, Rumpang: rumpangGen, RunnerTimeout: budget, GradingTimeout: budget,
		})
	}
	var learning *usecase.LearningService
	if cfg.ElearningEnabled {
		var hintGen usecase.RumpangGenerator
		if openaiGrader, ok := grader.(*grading.OpenAI); ok {
			hintGen = openaiGrader
		}
		learning = usecase.NewLearningService(usecase.LearningOptions{
			Repository: learningRepo, IdentityRepository: repo, Runner: codeRunner, RunnerTimeout: budget,
			HintGenerator: hintGen, HintTimeout: budget,
			AllowSimulatedVerification: cfg.Environment == "local" && cfg.EnableFixtures,
		})
	}
	var academic *usecase.AcademicService
	if cfg.SiakadEnabled {
		academic = usecase.NewAcademicService(usecase.AcademicOptions{
			Repository: academicRepo, IdentityRepository: repo,
		})
	}
	var laporan *usecase.LaporanService
	if cfg.SiakadEnabled {
		var store storage.SignedStore = storage.Unconfigured{}
		if cfg.SupabaseURL != "" && cfg.SupabaseServiceKey != "" {
			store = storage.NewSupabase(cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.LaporanBucket)
		}
		laporan = usecase.NewLaporanService(usecase.LaporanOptions{
			Repository: academicRepo, IdentityRepository: repo, Storage: store,
		})
	}
	spec := openapi.Load(cfg.OpenAPIPath)
	playground := preview.New(cfg.PlaygroundFallbackURL, cfg.PlaygroundFallbackToken)
	handler := httpdelivery.NewRouter(httpdelivery.Dependencies{Config: cfg, Logger: logger, Readiness: usecase.NewReadinessService(readiness), OpenAPIYAML: spec.YAML, OpenAPIJSON: spec.JSON, Identity: identity, Repository: repo, Practicum: practicum, Learning: learning, Academic: academic, Laporan: laporan, Playground: playground})
	return App{Handler: handler, Close: closePool}, nil
}

type fixtureReadiness struct{}

func (fixtureReadiness) Check(context.Context) error { return nil }

type databaseReadiness struct {
	pool      *pgxpool.Pool
	practicum bool
	elearning bool
	siakad    bool
}

func (r databaseReadiness) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := r.pool.Exec(ctx, "select s.family_id, u.access_version from public.auth_sessions s join public.users u on u.id=s.user_id limit 0")
	if err == nil && r.practicum {
		_, err = r.pool.Exec(ctx, "select id from public.practicum_attempts limit 0")
	}
	if err == nil && r.elearning {
		_, err = r.pool.Exec(ctx, "select id from public.learning_progress limit 0")
	}
	if err == nil && r.siakad {
		_, err = r.pool.Exec(ctx, "select id from public.academic_classes limit 0")
	}
	return err
}
