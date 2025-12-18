package handlers

import (
	"context"
	"saas-api/internal/middleware"
	"saas-api/internal/services"

	"github.com/google/uuid"
)

// BaseHandler provides common functionality and dependencies for all handlers
type BaseHandler struct {
	services *services.Services
}

// NewBaseHandler creates a new base handler with the required service dependencies
func NewBaseHandler(services *services.Services) *BaseHandler {
	return &BaseHandler{
		services: services,
	}
}

// GetServices returns the services instance
func (h *BaseHandler) GetServices() *services.Services {
	return h.services
}

// Handlers holds all handler instances
type Handlers struct {
	Auth         *AuthHandler
	User         *UserHandler
	Document     *DocumentHandler
	Folder       *FolderHandler
	File         *FileHandler
	Permission   *PermissionHandler
	Role         *RoleHandler
	Organization *OrganizationHandler
	AuditLog     *AuditLogHandler
	Persona      *PersonaHandler
	Template     *TemplateHandler
	LibreChat    *LibreChatHandler
	Screener     *ScreenerHandler
	Static       *StaticHandler
}

// NewHandlers creates and returns all handler instances
// It uses the existing handler constructors to avoid duplication
func NewHandlers(services *services.Services, storagePath string, authService *services.AuthService, authMW *middleware.AuthMiddleware) *Handlers {
	repos := services.GetRepositories()

	// Get document service for file handler (if available)
	var docService interface {
		DeleteDocument(ctx context.Context, documentID uuid.UUID) error
	}
	if services.Document != nil {
		docService = services.Document
	}

	return &Handlers{
		Auth:         NewAuthHandler(authService, authMW, repos.Organization),
		User:         NewUserHandler(repos.User, repos.Role, repos.Organization),
		Document:     NewDocumentHandler(services),
		Folder:       NewFolderHandler(repos.Folder, repos.Document), // Update folder handler if needed
		File:         NewFileHandler(repos.Folder, repos.Document, docService, storagePath),
		Permission:   NewPermissionHandler(repos.Permission),
		Role:         NewRoleHandler(repos.Role),
		Organization: NewOrganizationHandler(repos.Organization, repos.Role, repos.Permission),
		AuditLog:     NewAuditLogHandler(repos.AuditLog),
		Persona:      NewPersonaHandler(repos.Persona),
		Template:     NewTemplateHandler(repos.Template),
		LibreChat:    NewLibreChatHandler(),
		Screener:     NewScreenerHandler(repos.Screener, repos.User),
		Static:       NewStaticHandler(storagePath, repos.Document),
	}
}
