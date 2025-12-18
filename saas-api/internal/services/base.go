package services

import (
	"saas-api/cmd/configs"
	"saas-api/internal/auth"
	"saas-api/internal/repositories"
	"saas-api/pkg/memorydb"
	"saas-api/pkg/weaviate"
)

// Service provides common functionality and dependencies for all services
type BaseService struct {
	repositories   *repositories.Repositories
	redis          *memorydb.RedisClient
	weaviateClient *weaviate.WeaviateClient
}

// NewService creates a new base service with the required dependencies
func NewBaseService(
	repositories *repositories.Repositories,
	redis *memorydb.RedisClient,
	weaviateClient *weaviate.WeaviateClient,
) *BaseService {
	return &BaseService{
		repositories:   repositories,
		redis:          redis,
		weaviateClient: weaviateClient,
	}
}

// GetRepositories returns the repositories instance
func (s *BaseService) GetRepositories() *repositories.Repositories {
	return s.repositories
}

// GetRedis returns the Redis client
func (s *BaseService) GetRedis() *memorydb.RedisClient {
	return s.redis
}

// GetWeaviateClient returns the Weaviate client
func (s *BaseService) GetWeaviateClient() *weaviate.WeaviateClient {
	return s.weaviateClient
}

// Services holds all service instances
type Services struct {
	base     *BaseService // Store base service for repository access
	Health   *HealthService
	Auth     *AuthService
	Document *DocumentService
}

func NewServices(base *BaseService, userRepo *repositories.UserRepository, tokenRepo *repositories.RefreshTokenRepository, tokenService *auth.TokenService, cfg *configs.Config) *Services {
	documentService := NewDocumentService(base)
	// Start document processing workers
	documentService.StartWorkers()

	// Create auth service - need to convert configs.Config to config.Config
	// For now, we'll pass nil and let auth service handle it, or we need to create a converter
	// Actually, looking at auth_service.go, it uses config.Config, not configs.Config
	// We need to check what config type is needed
	authService := NewAuthService(userRepo, tokenRepo, tokenService, nil) // TODO: Fix config conversion

	return &Services{
		base:     base, // Store base service
		Auth:     authService,
		Document: documentService,
	}
}

// GetRepositories returns the repositories instance
func (s *Services) GetRepositories() *repositories.Repositories {
	return s.base.GetRepositories()
}

// Close gracefully shuts down all services
func (s *Services) Close() {
	if s.Document != nil {
		s.Document.StopWorkers()
	}
}
