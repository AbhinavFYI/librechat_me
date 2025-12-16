package repositories

import (
	"context"
	"fmt"
	"log"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FileRepository struct {
	db *database.DB
}

func NewFileRepository(db *database.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(ctx context.Context, file *models.File) error {
	query := `
		INSERT INTO files (id, org_id, folder_id, name, extension, mime_type, size_bytes, checksum, storage_key, version, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		file.ID, file.OrgID, file.FolderID, file.Name, file.Extension,
		file.MimeType, file.SizeBytes, file.Checksum, file.StorageKey, file.Version, file.CreatedBy,
	).Scan(&file.CreatedAt, &file.UpdatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create file", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *FileRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	file := &models.File{}
	var createdByName, updatedByName, folderName *string

	query := `
		SELECT f.id, f.org_id, f.folder_id, f.name, f.extension, f.mime_type, f.size_bytes,
			f.checksum, f.storage_key, f.version, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name,
			fo.name as folder_name
		FROM files f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		LEFT JOIN folders fo ON f.folder_id = fo.id
		WHERE f.id = $1
	`

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&file.ID, &file.OrgID, &file.FolderID, &file.Name, &file.Extension, &file.MimeType,
		&file.SizeBytes, &file.Checksum, &file.StorageKey, &file.Version,
		&file.CreatedBy, &file.UpdatedBy, &file.CreatedAt, &file.UpdatedAt,
		&createdByName, &updatedByName, &folderName,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get file", errors.ErrInternalServer.Status)
	}

	file.CreatedByName = createdByName
	file.UpdatedByName = updatedByName
	file.FolderName = folderName
	return file, nil
}

func (r *FileRepository) List(ctx context.Context, orgID uuid.UUID, folderID *uuid.UUID, page, limit int) ([]*models.File, int64, error) {
	var count int64
	var rows pgx.Rows
	var err error

	// Get total count
	countQuery := `SELECT COUNT(*) FROM files WHERE org_id = $1`
	args := []interface{}{orgID}
	if folderID != nil {
		countQuery += ` AND folder_id = $2`
		args = append(args, *folderID)
	}
	err = r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count files", errors.ErrInternalServer.Status)
	}

	// Get files
	offset := (page - 1) * limit
	query := `
		SELECT f.id, f.org_id, f.folder_id, f.name, f.extension, f.mime_type, f.size_bytes,
			f.checksum, f.storage_key, f.version, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name,
			fo.name as folder_name
		FROM files f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		LEFT JOIN folders fo ON f.folder_id = fo.id
		WHERE f.org_id = $1
	`
	queryArgs := []interface{}{orgID}
	if folderID != nil {
		query += ` AND f.folder_id = $2`
		queryArgs = append(queryArgs, *folderID)
	} else {
		query += ` AND f.folder_id IS NULL`
	}
	query += ` ORDER BY f.created_at DESC LIMIT $` + fmt.Sprintf("%d", len(queryArgs)+1) + ` OFFSET $` + fmt.Sprintf("%d", len(queryArgs)+2)
	queryArgs = append(queryArgs, limit, offset)

	rows, err = r.db.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list files", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&file.ID, &file.OrgID, &file.FolderID, &file.Name, &file.Extension, &file.MimeType,
			&file.SizeBytes, &file.Checksum, &file.StorageKey, &file.Version,
			&file.CreatedBy, &file.UpdatedBy, &file.CreatedAt, &file.UpdatedAt,
			&createdByName, &updatedByName, &folderName,
		)
		if err != nil {
			log.Printf("Error scanning file: %v", err)
			continue
		}

		file.CreatedByName = createdByName
		file.UpdatedByName = updatedByName
		file.FolderName = folderName
		files = append(files, file)
	}

	return files, count, nil
}

func (r *FileRepository) Update(ctx context.Context, file *models.File) error {
	query := `
		UPDATE files
		SET name = $1, folder_id = $2, updated_by = $3, updated_at = now()
		WHERE id = $4
		RETURNING updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		file.Name, file.FolderID, file.UpdatedBy, file.ID,
	).Scan(&file.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.ErrNotFound
		}
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to update file", errors.ErrInternalServer.Status)
	}

	return nil
}

func (r *FileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM files WHERE id = $1`
	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to delete file", errors.ErrInternalServer.Status)
	}

	if result.RowsAffected() == 0 {
		return errors.ErrNotFound
	}

	return nil
}

func (r *FileRepository) GetByFolder(ctx context.Context, folderID uuid.UUID) ([]*models.File, error) {
	query := `
		SELECT f.id, f.org_id, f.folder_id, f.name, f.extension, f.mime_type, f.size_bytes,
			f.checksum, f.storage_key, f.version, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name,
			fo.name as folder_name
		FROM files f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		LEFT JOIN folders fo ON f.folder_id = fo.id
		WHERE f.folder_id = $1
		ORDER BY f.name ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, folderID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get files by folder", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&file.ID, &file.OrgID, &file.FolderID, &file.Name, &file.Extension, &file.MimeType,
			&file.SizeBytes, &file.Checksum, &file.StorageKey, &file.Version,
			&file.CreatedBy, &file.UpdatedBy, &file.CreatedAt, &file.UpdatedAt,
			&createdByName, &updatedByName, &folderName,
		)
		if err != nil {
			log.Printf("Error scanning file: %v", err)
			continue
		}

		file.CreatedByName = createdByName
		file.UpdatedByName = updatedByName
		file.FolderName = folderName
		files = append(files, file)
	}

	return files, nil
}

// GetByStorageKeyPattern finds a file by matching storage_key pattern
// This is useful when the storage_key format might vary (e.g., with or without storage path prefix)
func (r *FileRepository) GetByStorageKeyPattern(ctx context.Context, pattern string) (*models.File, error) {
	file := &models.File{}
	var createdByName, updatedByName, folderName *string

	// Normalize pattern: replace backslashes with forward slashes for cross-platform compatibility
	normalizedPattern := strings.ReplaceAll(pattern, "\\", "/")

	// Try multiple patterns:
	// 1. Exact match
	// 2. Ends with pattern (removes storage path prefix)
	// 3. Starts with pattern (adds storage path prefix)
	// 4. Contains pattern
	// Also try with normalized separators
	query := `
		SELECT f.id, f.org_id, f.folder_id, f.name, f.extension, f.mime_type, f.size_bytes,
			f.checksum, f.storage_key, f.version, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name,
			fo.name as folder_name
		FROM files f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		LEFT JOIN folders fo ON f.folder_id = fo.id
		WHERE f.storage_key = $1
		   OR REPLACE(f.storage_key, '\', '/') = $1
		   OR f.storage_key LIKE '%' || $1
		   OR REPLACE(f.storage_key, '\', '/') LIKE '%' || $1
		   OR f.storage_key LIKE $1 || '%'
		   OR REPLACE(f.storage_key, '\', '/') LIKE $1 || '%'
		   OR f.storage_key LIKE '%' || $1 || '%'
		   OR REPLACE(f.storage_key, '\', '/') LIKE '%' || $1 || '%'
		ORDER BY 
			CASE 
				WHEN f.storage_key = $1 THEN 1
				WHEN REPLACE(f.storage_key, '\', '/') = $1 THEN 2
				WHEN f.storage_key LIKE $1 || '%' THEN 3
				WHEN REPLACE(f.storage_key, '\', '/') LIKE $1 || '%' THEN 4
				WHEN f.storage_key LIKE '%' || $1 THEN 5
				WHEN REPLACE(f.storage_key, '\', '/') LIKE '%' || $1 THEN 6
				ELSE 7
			END
		LIMIT 1
	`

	err := r.db.Pool.QueryRow(ctx, query, normalizedPattern).Scan(
		&file.ID, &file.OrgID, &file.FolderID, &file.Name, &file.Extension, &file.MimeType,
		&file.SizeBytes, &file.Checksum, &file.StorageKey, &file.Version,
		&file.CreatedBy, &file.UpdatedBy, &file.CreatedAt, &file.UpdatedAt,
		&createdByName, &updatedByName, &folderName,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to find file by storage key pattern", errors.ErrInternalServer.Status)
	}

	file.CreatedByName = createdByName
	file.UpdatedByName = updatedByName
	file.FolderName = folderName
	return file, nil
}

// GetByFilenameAndPath searches for a file by filename and path segments within an organization
// This is useful for finding files when the exact storage_key format is unknown
func (r *FileRepository) GetByFilenameAndPath(ctx context.Context, orgID *uuid.UUID, filename string, pathSegments []string) (*models.File, error) {
	file := &models.File{}
	var createdByName, updatedByName, folderName *string

	// Build query to search by filename and path segments
	query := `
		SELECT f.id, f.org_id, f.folder_id, f.name, f.extension, f.mime_type, f.size_bytes,
			f.checksum, f.storage_key, f.version, f.created_by, f.updated_by,
			f.created_at, f.updated_at,
			u1.full_name as created_by_name, u2.full_name as updated_by_name,
			fo.name as folder_name
		FROM files f
		LEFT JOIN users u1 ON f.created_by = u1.id
		LEFT JOIN users u2 ON f.updated_by = u2.id
		LEFT JOIN folders fo ON f.folder_id = fo.id
		WHERE f.name = $1
	`

	args := []interface{}{filename}
	argIndex := 2

	// If orgID is provided, filter by org
	if orgID != nil {
		query += fmt.Sprintf(" AND f.org_id = $%d", argIndex)
		args = append(args, *orgID)
		argIndex++
	}

	// Try to match path segments in storage_key
	if len(pathSegments) > 0 {
		// Build LIKE pattern for path segments
		pathPattern := "%" + strings.Join(pathSegments, "%") + "%"
		query += fmt.Sprintf(" AND (REPLACE(f.storage_key, '\\', '/') LIKE $%d OR f.storage_key LIKE $%d)", argIndex, argIndex)
		args = append(args, pathPattern)
	}

	query += " LIMIT 1"

	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(
		&file.ID, &file.OrgID, &file.FolderID, &file.Name, &file.Extension, &file.MimeType,
		&file.SizeBytes, &file.Checksum, &file.StorageKey, &file.Version,
		&file.CreatedBy, &file.UpdatedBy, &file.CreatedAt, &file.UpdatedAt,
		&createdByName, &updatedByName, &folderName,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.ErrNotFound
		}
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to find file by filename and path", errors.ErrInternalServer.Status)
	}

	file.CreatedByName = createdByName
	file.UpdatedByName = updatedByName
	file.FolderName = folderName
	return file, nil
}
