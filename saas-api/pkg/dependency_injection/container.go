package dependency_injection

import (
	"context"

	"saas-api/cmd/configs"
	"saas-api/internal/auth"
	"saas-api/internal/handlers"
	"saas-api/internal/middleware"
	"saas-api/internal/repositories"
	"saas-api/internal/services"
	"saas-api/pkg/memorydb"
	"saas-api/pkg/postgres"
	"saas-api/pkg/weaviate"

	fylogger "github.com/FyersDev/trading-logger-go"
)

type Container struct {
	Config        *configs.Config
	DB            *postgres.DB
	DBWriter      *postgres.DB
	RedisClient   *memorydb.RedisClient
	HealthService *services.HealthService
	Repositories  *repositories.Repositories
	Handlers      *handlers.Handlers
	Services      *services.Services
}

func NewContainer(ctx context.Context, config *configs.Config) (*Container, error) {
	// Initialize redis client
	redisClient, err := memorydb.NewRedisClient(ctx, config)
	if err != nil {
		fylogger.ErrorLog(context.Background(), "Failed to initialize redis client", err, nil)
		return nil, err
	}
	// Initialize database clients
	db, err := postgres.NewPostgresClient(ctx, config)
	if err != nil {
		fylogger.ErrorLog(context.Background(), "Failed to initialize read database", err, nil)
		return nil, err
	}

	dbWriter, err := postgres.NewPostgresClientWrite(ctx, config)
	if err != nil {
		fylogger.ErrorLog(context.Background(), "Failed to initialize write database", err, nil)
		db.Close()
		return nil, err
	}

	repos := repositories.NewRepositories(db, dbWriter)
	weaviateClient := weaviate.NewWeaviateClient(config)
	baseService := services.NewBaseService(repos, redisClient, weaviateClient)

	// Get repositories for auth service
	userRepo := repos.User
	tokenRepo := repos.RefreshToken

	// Create token service - need config.Config, but we have configs.Config
	// For now, create a minimal token service or convert config
	// TODO: Fix config conversion - token service needs config.Config
	tokenService := auth.NewTokenService(nil) // Will need proper config

	service := services.NewServices(baseService, userRepo, tokenRepo, tokenService, config)

	// Create middleware needed for handlers
	authMW := middleware.NewAuthMiddleware(tokenService)

	// Create handlers using existing constructors (no duplication)
	handlers := handlers.NewHandlers(service, config.StoragePath, service.Auth, authMW)

	// Initialize database schemas
	if err := service.Document.InitSchema(ctx); err != nil {
		fylogger.ErrorLog(ctx, "Failed to initialize document schema", err, nil)
		// Don't fail startup, just log the error - schema might already exist
	}

	// Initialize health services
	healthService := services.NewHealthService(db, dbWriter, redisClient)

	return &Container{
		Config:        config,
		DB:            db,
		DBWriter:      dbWriter,
		RedisClient:   redisClient,
		HealthService: healthService,
		Repositories:  repos,
		Handlers:      handlers,
		Services:      service,
	}, nil
}

func (c *Container) Close() {
	// Stop services first (workers)
	if c.Services != nil {
		c.Services.Close()
	}

	if c.DB != nil {
		c.DB.Close()
	}
	if c.DBWriter != nil {
		c.DBWriter.Close()
	}
	if c.RedisClient != nil {
		c.RedisClient.Close()
	}
}
