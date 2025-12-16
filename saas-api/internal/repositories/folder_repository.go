package repositories

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FolderRepository struct {
	db *database.DB
}

func NewFolderRepository(db *database.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

func (r *FolderRepository) Create(ctx context.Context, folder *models.Folder) error {
	// Calculate path based on parent
	var path string
	if folder.ParentID != nil {
		parent, err := r.GetByID(ctx, *folder.ParentID)
		if err != nil {
			return errors.WrapError(err, "NOT_FOUND", "Parent folder not found", errors.ErrNotFound.Status)
		}
		path = filepath.Join(parent.Path, folder.Name)
	} else {
		// Root folder
		path = "/" + folder.Name
	}

	// Normalize path
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	query := `
		INSERT INTO folders (id, org_id, parent_id, name, path, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		folder.ID, folder.OrgID, folder.ParentID, folder.Name, path, folder.CreatedBy,
	).Scan(&folder.CreatedAt, &folder.UpdatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create folder", errors.ErrInternalServer.Status)
	}

	folder.Path = path
	return nil
}

func (r *FolderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error) {
	folder := &models.Folder{}
	var createdByName, updatedByName *string

	query := `
		SELECT f.id, f.org_id, f.parent_id, f.name, f.path, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name
		FROM folders f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		WHERE f.id = $1
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&folder.ID, &folder.OrgID, &folder.ParentID, &folder.Name, &folder.Path,
		&folder.CreatedBy, &folder.UpdatedBy, &folder.CreatedAt, &folder.UpdatedAt,
		&createdByName, &updatedByName,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get folder", errors.ErrInternalServer.Status)
	}

	folder.CreatedByName = createdByName
	folder.UpdatedByName = updatedByName
	return folder, nil
}

func (r *FolderRepository) List(ctx context.Context, orgID uuid.UUID, parentID *uuid.UUID) ([]*models.Folder, error) {
	var rows pgx.Rows
	var err error

	if parentID != nil {
		query := `
			SELECT f.id, f.org_id, f.parent_id, f.name, f.path, f.created_by, f.updated_by,
				f.created_at, f.updated_at,
				u1.full_name as created_by_name, u2.full_name as updated_by_name
			FROM folders f
			LEFT JOIN users u1 ON f.created_by = u1.id
			LEFT JOIN users u2 ON f.updated_by = u2.id
			WHERE f.org_id = $1 AND f.parent_id = $2
			ORDER BY f.name ASC
		`
		rows, err = r.db.Pool.Query(ctx, query, orgID, *parentID)
	} else {
		query := `
			SELECT f.id, f.org_id, f.parent_id, f.name, f.path, f.created_by, f.updated_by,
				f.created_at, f.updated_at,
				u1.full_name as created_by_name, u2.full_name as updated_by_name
			FROM folders f
			LEFT JOIN users u1 ON f.created_by = u1.id
			LEFT JOIN users u2 ON f.updated_by = u2.id
			WHERE f.org_id = $1 AND f.parent_id IS NULL
			ORDER BY f.name ASC
		`
		rows, err = r.db.Pool.Query(ctx, query, orgID)
	}

	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list folders", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var folders []*models.Folder
	for rows.Next() {
		folder := &models.Folder{}
		var createdByName, updatedByName *string

		err := rows.Scan(
			&folder.ID, &folder.OrgID, &folder.ParentID, &folder.Name, &folder.Path,
			&folder.CreatedBy, &folder.UpdatedBy, &folder.CreatedAt, &folder.UpdatedAt,
			&createdByName, &updatedByName,
		)
		if err != nil {
			log.Printf("Error scanning folder: %v", err)
			continue
		}

		folder.CreatedByName = createdByName
		folder.UpdatedByName = updatedByName
		folders = append(folders, folder)
	}

	return folders, nil
}

func (r *FolderRepository) GetTree(ctx context.Context, orgID uuid.UUID) ([]*models.Folder, error) {
	// Get all folders for the org
	query := `
		SELECT f.id, f.org_id, f.parent_id, f.name, f.path, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name
		FROM folders f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		WHERE f.org_id = $1
		ORDER BY f.path ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get folder tree", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	// Build a map of all folders
	folderMap := make(map[uuid.UUID]*models.Folder)
	var rootFolders []*models.Folder

	for rows.Next() {
		folder := &models.Folder{}
		var createdByName, updatedByName *string

		err := rows.Scan(
			&folder.ID, &folder.OrgID, &folder.ParentID, &folder.Name, &folder.Path,
			&folder.CreatedBy, &folder.UpdatedBy, &folder.CreatedAt, &folder.UpdatedAt,
			&createdByName, &updatedByName,
		)
		if err != nil {
			log.Printf("Error scanning folder: %v", err)
			continue
		}

		folder.CreatedByName = createdByName
		folder.UpdatedByName = updatedByName
		folder.Children = []*models.Folder{}
		folderMap[folder.ID] = folder

		if folder.ParentID == nil {
			rootFolders = append(rootFolders, folder)
		}
	}

	// Build tree structure
	for _, folder := range folderMap {
		if folder.ParentID != nil {
			if parent, exists := folderMap[*folder.ParentID]; exists {
				parent.Children = append(parent.Children, folder)
			}
		}
	}

	return rootFolders, nil
}

func (r *FolderRepository) Update(ctx context.Context, folder *models.Folder) error {
	// If name or parent changed, recalculate path
	var path string
	if folder.ParentID != nil {
		parent, err := r.GetByID(ctx, *folder.ParentID)
		if err != nil {
			return errors.WrapError(err, "NOT_FOUND", "Parent folder not found", errors.ErrNotFound.Status)
		}
		path = filepath.Join(parent.Path, folder.Name)
	} else {
		path = "/" + folder.Name
	}

	path = filepath.Clean(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	query := `
		UPDATE folders
		SET name = $1, parent_id = $2, path = $3, updated_by = $4, updated_at = now()
		WHERE id = $5
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		folder.Name, folder.ParentID, path, folder.UpdatedBy, folder.ID,
	).Scan(&folder.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.ErrNotFound
		}
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update folder", errors.ErrInternalServer.Status)
	}

	folder.Path = path
	return nil
}

func (r *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if folder has children or files
	var childCount, fileCount int
	checkQuery := `
		SELECT 
			(SELECT COUNT(*) FROM folders WHERE parent_id = $1) as child_count,
			(SELECT COUNT(*) FROM files WHERE folder_id = $1) as file_count
	`
	err := r.db.Pool.QueryRow(ctx, checkQuery, id).Scan(&childCount, &fileCount)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to check folder", errors.ErrInternalServer.Status)
	}

	if childCount > 0 || fileCount > 0 {
		return errors.NewError("CONFLICT", "Cannot delete folder with children or files", 409)
	}

	query := `DELETE FROM folders WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete folder", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}

func (r *FolderRepository) GetPermissions(ctx context.Context, folderID uuid.UUID) ([]models.FolderPermission, error) {
	query := `
		SELECT fp.id, fp.folder_id, fp.role_id, fp.permission, fp.created_at,
			r.name as role_name
		FROM folder_permissions fp
		LEFT JOIN roles r ON fp.role_id = r.id
		WHERE fp.folder_id = $1
		ORDER BY r.name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, folderID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get folder permissions", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var permissions []models.FolderPermission
	for rows.Next() {
		perm := models.FolderPermission{}
		var roleName *string

		err := rows.Scan(
			&perm.ID, &perm.FolderID, &perm.RoleID, &perm.Permission, &perm.CreatedAt,
			&roleName,
		)
		if err != nil {
			log.Printf("Error scanning folder permission: %v", err)
			continue
		}

		perm.RoleName = roleName
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

func (r *FolderRepository) AssignPermission(ctx context.Context, folderID uuid.UUID, roleID uuid.UUID, permission string) error {
	// Check if permission already exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM folder_permissions WHERE folder_id = $1 AND role_id = $2)`
	err := r.db.Pool.QueryRow(ctx, checkQuery, folderID, roleID).Scan(&exists)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to check permission", errors.ErrInternalServer.Status)
	}

	if exists {
		// Update existing permission
		query := `UPDATE folder_permissions SET permission = $1 WHERE folder_id = $2 AND role_id = $3`
		_, err = r.db.Pool.Exec(ctx, query, permission, folderID, roleID)
	} else {
		// Create new permission
		query := `INSERT INTO folder_permissions (id, folder_id, role_id, permission) VALUES (gen_random_uuid(), $1, $2, $3)`
		_, err = r.db.Pool.Exec(ctx, query, folderID, roleID, permission)
	}

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to assign permission", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *FolderRepository) RemovePermission(ctx context.Context, folderID uuid.UUID, roleID uuid.UUID) error {
	query := `DELETE FROM folder_permissions WHERE folder_id = $1 AND role_id = $2`
	result, err := r.db.Pool.Exec(ctx, query, folderID, roleID)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to remove permission", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}

