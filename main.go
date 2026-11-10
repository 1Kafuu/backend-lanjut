package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/database"
	_ "api-students/docs" // hasil swag init; mendaftarkan spesifikasi ke swagger
	"api-students/helper"
	"api-students/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

const minSecretLength = 32

// Blok komentar di bawah ini adalah informasi UMUM dokumentasi API. swag
// hanya membacanya dari file yang disebut pada flag -g (bawaannya main.go).
//
// @title                       API KRS Praktikum Backend Lanjut
// @version                     1.0
// @description                 API pendaftaran kelas (KRS), ulasan kelas, dan notifikasi.
// @description                 Seluruh endpoint selain /auth memerlukan access token.
// @BasePath                    /api/v1
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Isi dengan: Bearer <access_token>
func main() {
	config.LoadENV()
	logger := config.NewLogger()

	// Satu program, tiga cara menjalankan:
	//   go run . -role=api     hanya melayani HTTP
	//   go run . -role=worker  hanya relay outbox dan worker queue
	//   go run . -role=all     keduanya dalam satu proses
	role := flag.String("role", config.GetENV("APP_ROLE", "all"), "api | worker | all")
	flag.Parse()
	runAPI := *role == "api" || *role == "all"
	runWorker := *role == "worker" || *role == "all"
	if !runAPI && !runWorker {
		logger.Error("role tidak dikenal", slog.String("role", *role))
		os.Exit(1)
	}

	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error("gagal terhubung ke PostgreSQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	// ctx dibatalkan ketika Ctrl+C ditekan atau sistem mengirim SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app *fiber.App
	if runAPI {
		app = startAPI(logger, pool)
		if app == nil {
			os.Exit(1)
		}
	}

	var stopWorker func()
	if runWorker {
		stopWorker = startWorker(ctx, logger, pool)
		if stopWorker == nil {
			os.Exit(1)
		}
	}

	logger.Info("aplikasi berjalan", slog.String("role", *role))
	<-ctx.Done()

	// Urutan berhenti: pintu masuk HTTP ditutup lebih dulu, baru pekerja.
	if app != nil {
		if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
			logger.Error("shutdown HTTP tidak rapi", slog.String("error", err.Error()))
		}
	}
	if stopWorker != nil {
		stopWorker()
	}
	logger.Info("aplikasi berhenti dengan rapi")
}

// startAPI menyiapkan seluruh kebutuhan HTTP. Mengembalikan nil bila gagal.
func startAPI(logger *slog.Logger, pool *pgxpool.Pool) *fiber.App {
	jwtSecret := config.GetENV("JWT_SECRET", "")
	if len(jwtSecret) < minSecretLength {
		logger.Error("JWT_SECRET tidak diisi atau terlalu pendek",
			slog.Int("minimal_karakter", minSecretLength))
		return nil
	}

	mongoClient, err := database.NewMongoClient(context.Background())
	if err != nil {
		logger.Error("gagal terhubung ke MongoDB", slog.String("error", err.Error()))
		return nil
	}
	mongoDB := mongoClient.Database(config.GetENV("MONGO_DATABASE", "praktikum-backend-mongo"))

	jwtManager := helper.NewJWTManager(
		jwtSecret,
		config.GetENV("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetENVInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)
	refreshTTL := time.Duration(config.GetENVInt("JWT_REFRESH_TTL_DAYS", 7)) * 24 * time.Hour

	userRepository := repository.NewUserRepository(pool)
	tokenRepository := repository.NewTokenRepository(pool)
	studentRepository := repository.NewStudentRepository(pool)
	roleRepository := repository.NewRoleRepository(pool)
	rawPermissions, err := roleRepository.LoadPermissions(context.Background())
	if err != nil {
		logger.Error("gagal memuat permission", slog.String("error", err.Error()))
		return nil
	}
	perms := helper.NewPermissionSet(rawPermissions)
	logger.Info("permission dimuat", slog.Any("roles", perms.KnownRoles()))

	studentService := service.NewStudentService(studentRepository, perms)
	userService := service.NewUserService(userRepository, perms)
	prestasiRepository := repository.NewPrestasiReporsitory(pool)
	prestasiService := service.NewPrestasiService(prestasiRepository, studentRepository)
	authService := service.NewAuthService(userRepository, tokenRepository, jwtManager, perms, refreshTTL)
	courseRepository := repository.NewCourseRepository(pool)
	txManager := repository.NewTxManager(pool)
	outboxRepository := repository.NewOutboxRepository(pool)
	courseService := service.NewCourseService(courseRepository, outboxRepository, txManager)
	reviewRepository := repository.NewReviewRepository(mongoDB)
	if err := reviewRepository.EnsureIndexes(context.Background()); err != nil {
		logger.Error("gagal membuat index MongoDB", slog.String("error", err.Error()))
		return nil
	}
	reviewService := service.NewReviewService(reviewRepository, courseRepository, perms)
	portfolioRepository := repository.NewPortfolioRepository(mongoDB)
	if err := portfolioRepository.EnsureIndexes(context.Background()); err != nil {
		logger.Error(
			"gagal membuat index portofolio",
			slog.String("error", err.Error()),
		)
		return nil
	}
	portfolioService := service.NewPortfolioService(
		portfolioRepository,
		studentRepository,
		perms,
	)

	// Inspector hanya MEMBACA keadaan queue untuk endpoint admin. Ia tidak
	// menghubungi Redis saat dibuat, sehingga API tetap daftar walaupun
	// Redis sedang mati.
	inspector := asynq.NewInspector(config.RedisOpt())

	app := config.NewApp(logger, pool, routes.Dependencies{
		Pool:             pool,
		Mongo:            mongoClient,
		JWT:              jwtManager,
		Permissions:      perms,
		StudentService:   studentService,
		PrestasiService:  prestasiService,
		UserService:      userService,
		AuthService:      authService,
		CourseService:    courseService,
		ReviewService:    reviewService,
		PortfolioService: portfolioService,
		NotificationService: service.NewNotificationService(
			repository.NewNotificationRepository(pool),
			outboxRepository,
			inspector,
		),
		EnableSwagger: config.GetENV("SWAGGER_ENABLED", "false") == "true",
	})

	// Sumber daya milik API ditutup setelah server HTTP berhenti.
	app.Hooks().OnShutdown(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = inspector.Close()
		return mongoClient.Disconnect(ctx)
	})

	port := config.GetENV("APP_PORT", "3000")
	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error("server berhenti", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()
	logger.Info("HTTP berjalan", slog.String("port", port))
	return app
}
