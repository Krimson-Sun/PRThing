package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"pr-service/internal/app"
	"pr-service/internal/config"
	"pr-service/internal/db"
	"pr-service/internal/handler"
	"pr-service/internal/logger"
	"pr-service/internal/repository"
	"pr-service/internal/service/assignment"
	"pr-service/internal/service/pullrequest"
	"pr-service/internal/service/team"
	"pr-service/internal/service/user"
)

func main() {
	// Initialize logger
	log := logger.NewLogger("pr-service", "info", "json", false)
	defer func() {
		_ = log.Sync()
	}()

	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal("Failed to load config", zap.Error(err))
	}

	// Override config from environment variables for Docker
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
		cfg.Database.Port = dbPort
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		cfg.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		cfg.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database.DBName = dbName
	}
	if dbSSL := os.Getenv("DB_SSLMODE"); dbSSL != "" {
		cfg.Database.SSLMode = dbSSL
	}

	// Connect to database
	ctx := context.Background()
	dbURL := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	// Test connection
	if err := dbPool.Ping(ctx); err != nil {
		log.Fatal("Failed to ping database", zap.Error(err))
	}
	log.Info("Successfully connected to database")

	// Initialize context manager for transactions
	contextManager := db.NewContextManager(dbPool, log)

	// Initialize repositories
	teamRepo := repository.NewTeamRepository(contextManager)
	userRepo := repository.NewUserRepository(contextManager)
	prRepo := repository.NewPRRepository(contextManager)

	// Initialize services
	assignmentStrategy := assignment.NewStrategy()
	teamService := team.NewService(teamRepo, userRepo, contextManager)
	userService := user.NewService(userRepo, prRepo, contextManager, assignmentStrategy)
	prService := pullrequest.NewService(prRepo, userRepo, contextManager, assignmentStrategy)

	// Initialize handlers
	teamHandler := handler.NewTeamHandler(teamService, log)
	userHandler := handler.NewUserHandler(userService, log)
	prHandler := handler.NewPRHandler(prService, log)
	healthHandler := handler.NewHealthHandler()
	docsHandler := handler.NewDocsHandler("openapi.yml")
	statsHandler := handler.NewStatsHandler(prService, log)

	// Initialize and start HTTP server
	server := app.NewServer(cfg, log, teamHandler, userHandler, prHandler, healthHandler, docsHandler, statsHandler)

	// Start server in goroutine
	go func() {
		log.Info("Starting HTTP server", zap.Int("port", cfg.Server.Port))
		if err := server.Start(); err != nil {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server stopped")
}
