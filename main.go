package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/database"
	"api-students/helper"
	"api-students/routes"
)

const minSecretLength = 32

func main() {
	config.LoadENV()
	logger := config.NewLogger()

	jwtSecret := config.GetENV("JWT_SECRET", "")
	if len(jwtSecret) < minSecretLength {
		logger.Error("JWT_SECRET tidak diisi atau terlalu pendek",
			slog.Int("minimal_karakter", minSecretLength),
			slog.Int("dapat", len(jwtSecret)))
		os.Exit(1)
	}

	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error("gagal terhubung ke database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	jwtManager := helper.NewJWTManager(
		jwtSecret,
		config.GetENV("JWT_ISSUER", "praktikum-backend"),
		time.Duration(config.GetENVInt("JWT_ACCESS_TTL_MINUTES", 15))*time.Minute,
	)
	refreshTTL := time.Duration(config.GetENVInt("JWT_REFRESH_TTL_DAYS", 7)) * 24 * time.Hour

	userRepository := repository.NewUserRepository(pool)
	tokenRepository := repository.NewTokenRepository(pool)
	studentRepository := repository.NewStudentRepository(pool)

	studentService := service.NewStudentService(studentRepository)
	prestasiRepository := repository.NewPrestasiReporsitory(pool)
	prestasiService := service.NewPrestasiService(prestasiRepository, studentRepository)
	authService := service.NewAuthService(userRepository, tokenRepository, jwtManager, refreshTTL)

	app := config.NewApp(logger, pool, routes.Dependencies{
		Pool:           pool,
		JWT:            jwtManager,
		StudentService:  studentService,
		PrestasiService: prestasiService,
		AuthService:    authService,
	})

	port := config.GetENV("APP_PORT", "3000")

	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error("server berhenti", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	logger.Info("server berjalan", slog.String("port", port))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("sinyal berhenti diterima, menutup server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Error("gagal menutup server dengan rapi", slog.String("error", err.Error()))
	}
	logger.Info("server berhenti dengan rapi")
}