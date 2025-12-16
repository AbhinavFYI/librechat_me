package repositories

import (
	"context"

	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PermissionRepository struct {
	db *database.DB
}

func NewPermissionRepository(db *database.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) List(ctx context.Context) ([]*models.Permission, error) {
	var permissions []*models.Permission
	
	query := `
		SELECT id, resource, action, description, is_system, created_at
		FROM permissions
		ORDER BY resource, action
	`
	
	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list permissions", errors.ErrInternalServer.Status)
	}
	defer rows.Close()
	
	for rows.Next() {
		perm := &models.Permission{}
		err := rows.Scan(
			&perm.ID, &perm.Resource, &perm.Action, &perm.Description,
			&perm.IsSystem, &perm.CreatedAt,
		)
		if err != nil {
			return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan permission", errors.ErrInternalServer.Status)
		}
		permissions = append(permissions, perm)
	}
	
	return permissions, nil
}

func (r *PermissionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Permission, error) {
	perm := &models.Permission{}
	query := `
		SELECT id, resource, action, description, is_system, created_at
		FROM permissions
		WHERE id = $1
	`
	
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&perm.ID, &perm.Resource, &perm.Action, &perm.Description,
		&perm.IsSystem, &perm.CreatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get permission", errors.ErrInternalServer.Status)
	}
	
	return perm, nil
}

func (r *PermissionRepository) GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error) {
	perm := &models.Permission{}
	query := `
		SELECT id, resource, action, description, is_system, created_at
		FROM permissions
		WHERE resource = $1 AND action = $2
	`
	
	err := r.db.Pool.QueryRow(ctx, query, resource, action).Scan(
		&perm.ID, &perm.Resource, &perm.Action, &perm.Description,
		&perm.IsSystem, &perm.CreatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get permission", errors.ErrInternalServer.Status)
	}
	
	return perm, nil
}

