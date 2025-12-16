package repositories

import (
	"context"
	"time"

	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RoleRepository struct {
	db *database.DB
}

func NewRoleRepository(db *database.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	query := `
		INSERT INTO roles (id, org_id, name, type, description, is_default, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	
	err := r.db.Pool.QueryRow(ctx, query,
		role.ID, role.OrgID, role.Name, role.Type, role.Description,
		role.IsDefault, role.CreatedBy,
	).Scan(&role.CreatedAt, &role.UpdatedAt)
	
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create role", errors.ErrInternalServer.Status)
	}
	
	return nil
}

func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, org_id, name, type, description, is_default, created_at, updated_at, created_by
		FROM roles
		WHERE id = $1
	`
	
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&role.ID, &role.OrgID, &role.Name, &role.Type, &role.Description,
		&role.IsDefault, &role.CreatedAt, &role.UpdatedAt, &role.CreatedBy,
	)
	
	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get role", errors.ErrInternalServer.Status)
	}
	
	return role, nil
}

func (r *RoleRepository) GetByName(ctx context.Context, name string, orgID *uuid.UUID) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, org_id, name, type, description, is_default, created_at, updated_at, created_by
		FROM roles
		WHERE name = $1 AND ($2::uuid IS NULL OR org_id = $2)
		ORDER BY CASE WHEN org_id = $2 THEN 0 ELSE 1 END
		LIMIT 1
	`

	err := r.db.Pool.QueryRow(ctx, query, name, orgID).Scan(
		&role.ID, &role.OrgID, &role.Name, &role.Type, &role.Description,
		&role.IsDefault, &role.CreatedAt, &role.UpdatedAt, &role.CreatedBy,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get role by name", errors.ErrInternalServer.Status)
	}

	return role, nil
}

func (r *RoleRepository) List(ctx context.Context, orgID *uuid.UUID) ([]*models.Role, error) {
	var roles []*models.Role
	
	query := `
		SELECT id, org_id, name, type, description, is_default, created_at, updated_at, created_by
		FROM roles
		WHERE ($1::uuid IS NULL OR org_id = $1)
		ORDER BY created_at
	`
	
	rows, err := r.db.Pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list roles", errors.ErrInternalServer.Status)
	}
	defer rows.Close()
	
	for rows.Next() {
		role := &models.Role{}
		err := rows.Scan(
			&role.ID, &role.OrgID, &role.Name, &role.Type, &role.Description,
			&role.IsDefault, &role.CreatedAt, &role.UpdatedAt, &role.CreatedBy,
		)
		if err != nil {
			return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan role", errors.ErrInternalServer.Status)
		}
		roles = append(roles, role)
	}
	
	return roles, nil
}

func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	query := `
		UPDATE roles
		SET name = COALESCE($1, name),
			description = COALESCE($2, description),
			is_default = COALESCE($3, is_default),
			updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`
	
	err := r.db.Pool.QueryRow(ctx, query,
		role.Name, role.Description, role.IsDefault, role.ID,
	).Scan(&role.UpdatedAt)
	
	if err == pgx.ErrNoRows {
		return errors.ErrNotFound
	}
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update role", errors.ErrInternalServer.Status)
	}
	
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM roles WHERE id = $1`
	
	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete role", errors.ErrInternalServer.Status)
	}
	
	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}
	
	return nil
}

func (r *RoleRepository) GetPermissions(ctx context.Context, roleID uuid.UUID) ([]*models.Permission, error) {
	query := `
		SELECT p.id, p.resource, p.action, p.description, p.is_system, p.created_at
		FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.resource, p.action
	`
	
	rows, err := r.db.Pool.Query(ctx, query, roleID)
	if err != nil {
		return []*models.Permission{}, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get role permissions", errors.ErrInternalServer.Status)
	}
	defer rows.Close()
	
	permissions := make([]*models.Permission, 0) // Initialize as empty slice, not nil
	for rows.Next() {
		perm := &models.Permission{}
		err := rows.Scan(
			&perm.ID, &perm.Resource, &perm.Action, &perm.Description,
			&perm.IsSystem, &perm.CreatedAt,
		)
		if err != nil {
			return []*models.Permission{}, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan permission", errors.ErrInternalServer.Status)
		}
		permissions = append(permissions, perm)
	}
	
	return permissions, nil
}

func (r *RoleRepository) AssignPermissions(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID, grantedBy uuid.UUID) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to begin transaction", errors.ErrInternalServer.Status)
	}
	defer tx.Rollback(ctx)
	
	// Delete existing permissions
	_, err = tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete existing permissions", errors.ErrInternalServer.Status)
	}
	
	// Insert new permissions
	query := `INSERT INTO role_permissions (role_id, permission_id, granted_by) VALUES ($1, $2, $3)`
	for _, permID := range permissionIDs {
		_, err = tx.Exec(ctx, query, roleID, permID, grantedBy)
		if err != nil {
			return errors.WrapError(err, "INTERNAL_ERROR", "Failed to assign permission", errors.ErrInternalServer.Status)
		}
	}
	
	return tx.Commit(ctx)
}

func (r *RoleRepository) AssignRoleToUser(ctx context.Context, userID, roleID, assignedBy uuid.UUID, expiresAt *time.Time) error {
	query := `
		INSERT INTO user_roles (user_id, role_id, assigned_by, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, role_id) DO UPDATE
		SET assigned_by = $3, assigned_at = NOW(), expires_at = $4
	`
	
	_, err := r.db.Pool.Exec(ctx, query, userID, roleID, assignedBy, expiresAt)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to assign role to user", errors.ErrInternalServer.Status)
	}
	
	return nil
}

func (r *RoleRepository) RemoveRoleFromUser(ctx context.Context, userID, roleID uuid.UUID) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`
	
	result, err := r.db.Pool.Exec(ctx, query, userID, roleID)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to remove role from user", errors.ErrInternalServer.Status)
	}
	
	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}
	
	return nil
}
