package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pr-service/internal/app/middleware"
	"pr-service/internal/config"
	"pr-service/internal/db"
	"pr-service/internal/handler"
	"pr-service/internal/logger"
	"pr-service/internal/repository"
	"pr-service/internal/service/assignment"
	"pr-service/internal/service/pullrequest"
	"pr-service/internal/service/team"
	"pr-service/internal/service/user"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// App is the main application structure
type App struct {
	cfg    *config.Config
	logger *zap.Logger
	pool   *pgxpool.Pool
	server *http.Server
}

// Server wraps http.Server for the application
type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
}

// NewApp creates and configures the application
func NewApp(cfg *config.Config) (*App, error) {
	// Initialize logger
	log := logger.NewLogger("pr-service", cfg.Logger.Level, cfg.Logger.Encoding, cfg.Logger.Development)

	// Build database DSN
	dbURL := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// Create DB connection pool
	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Error("Failed to parse DB config", zap.Error(err))
		return nil, err
	}

	poolCfg.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.Database.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.Database.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Error("Failed to connect to database", zap.Error(err))
		return nil, err
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Error("Failed to ping database", zap.Error(err))
		return nil, err
	}

	log.Info("Successfully connected to database")

	// Initialize context manager (transactor)
	ctxManager := db.NewContextManager(pool, log)

	// Initialize repositories
	teamRepo := repository.NewTeamRepository(ctxManager)
	userRepo := repository.NewUserRepository(ctxManager)
	prRepo := repository.NewPRRepository(ctxManager)

	// Initialize assignment strategy
	assignStrategy := assignment.NewStrategy()

	// Initialize services
	teamService := team.NewService(teamRepo, userRepo, ctxManager)
	userService := user.NewService(userRepo, prRepo, ctxManager, assignStrategy)
	prService := pullrequest.NewService(prRepo, userRepo, ctxManager, assignStrategy)

	// Initialize handlers
	teamHandler := handler.NewTeamHandler(teamService, log)
	userHandler := handler.NewUserHandler(userService, log)
	prHandler := handler.NewPRHandler(prService, log)
	healthHandler := handler.NewHealthHandler()
	docsHandler := handler.NewDocsHandler("openapi.yml")
	statsHandler := handler.NewStatsHandler(prService, log)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Team routes
	mux.HandleFunc("POST /team/add", teamHandler.AddTeam)
	mux.HandleFunc("GET /team/get", teamHandler.GetTeam)

	// User routes
	mux.HandleFunc("POST /users/setIsActive", userHandler.SetIsActive)
	mux.HandleFunc("GET /users/getReview", userHandler.GetReview)
	mux.HandleFunc("POST /users/deactivateTeamMembers", userHandler.BulkDeactivateTeamMembers)

	// PR routes
	mux.HandleFunc("POST /pullRequest/create", prHandler.CreatePR)
	mux.HandleFunc("POST /pullRequest/merge", prHandler.MergePR)
	mux.HandleFunc("POST /pullRequest/reassign", prHandler.ReassignReviewer)

	// Stats routes
	mux.HandleFunc("GET /stats/assignments", statsHandler.GetAssignmentStats)

	// Health route
	mux.HandleFunc("GET /health", healthHandler.Check)

	// Documentation routes
	mux.HandleFunc("GET /docs", docsHandler.ServeSwaggerUI)
	mux.HandleFunc("GET /openapi.yml", docsHandler.ServeOpenAPI)

	// Apply middleware chain: Recovery → Logging
	// Note: Error handling is done within handlers via middleware.WriteErrorResponse
	var handler http.Handler = mux
	handler = middleware.Logging(log)(handler)
	handler = middleware.Recovery(log)(handler)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &App{
		cfg:    cfg,
		logger: log,
		pool:   pool,
		server: server,
	}, nil
}

// Run starts the application
func (a *App) Run() error {
	// Start HTTP server in goroutine
	go func() {
		a.logger.Info("Starting HTTP server", zap.String("address", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	a.logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Error("Server forced to shutdown", zap.Error(err))
		return err
	}

	// Close database pool
	a.pool.Close()
	a.logger.Info("Database connection pool closed")

	a.logger.Info("Server exited gracefully")
	return nil
}

// NewServer creates a new HTTP server with configured routes and middleware
func NewServer(
	cfg *config.Config,
	log *zap.Logger,
	teamHandler *handler.TeamHandler,
	userHandler *handler.UserHandler,
	prHandler *handler.PRHandler,
	healthHandler *handler.HealthHandler,
	docsHandler *handler.DocsHandler,
	statsHandler *handler.StatsHandler,
) *Server {
	// Setup HTTP router
	mux := http.NewServeMux()

	// Team routes
	mux.HandleFunc("POST /team/add", teamHandler.AddTeam)
	mux.HandleFunc("GET /team/get", teamHandler.GetTeam)

	// User routes
	mux.HandleFunc("POST /users/setIsActive", userHandler.SetIsActive)
	mux.HandleFunc("GET /users/getReview", userHandler.GetReview)
	mux.HandleFunc("POST /users/deactivateTeamMembers", userHandler.BulkDeactivateTeamMembers)

	// PR routes
	mux.HandleFunc("POST /pullRequest/create", prHandler.CreatePR)
	mux.HandleFunc("POST /pullRequest/merge", prHandler.MergePR)
	mux.HandleFunc("POST /pullRequest/reassign", prHandler.ReassignReviewer)

	// Stats routes
	mux.HandleFunc("GET /stats/assignments", statsHandler.GetAssignmentStats)

	// Health route
	mux.HandleFunc("GET /health", healthHandler.Check)

	// Documentation routes
	mux.HandleFunc("GET /docs", docsHandler.ServeSwaggerUI)
	mux.HandleFunc("GET /openapi.yml", docsHandler.ServeOpenAPI)

	// Apply middleware chain: Recovery → Logging
	var handler http.Handler = mux
	handler = middleware.Logging(log)(handler)
	handler = middleware.Recovery(log)(handler)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Server{
		httpServer: httpServer,
		logger:     log,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", zap.String("address", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
