package repositories

import (
	"saas-api/pkg/postgres"
)

// Repositories holds all repository instances
type Repositories struct {
	User         *UserRepository
	Organization *OrganizationRepository
	Role         *RoleRepository
	Permission   *PermissionRepository
	RefreshToken *RefreshTokenRepository
	Document     *DocumentRepository
	Folder       *FolderRepository
	File         *FileRepository
	Template     *TemplateRepository
	Persona      *PersonaRepository
	AuditLog     *AuditLogRepository
	Screener     *ScreenerRepository
}

// NewRepositories creates and returns all repository instances
func NewRepositories(db *postgres.DB, dbWriter *postgres.DB) *Repositories {
	return &Repositories{
		User:         NewUserRepository(db),
		Organization: NewOrganizationRepository(db),
		Role:         NewRoleRepository(db),
		Permission:   NewPermissionRepository(db),
		RefreshToken: NewRefreshTokenRepository(db),
		Document:     NewDocumentRepository(db, dbWriter),
		Folder:       NewFolderRepository(db),
		File:         NewFileRepository(db),
		Template:     NewTemplateRepository(db),
		Persona:      NewPersonaRepository(db),
		AuditLog:     NewAuditLogRepository(db),
		Screener:     NewScreenerRepository(db),
	}
}

