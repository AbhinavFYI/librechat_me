package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"saas-api/pkg/postgres"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DocumentStatus represents the processing status of a document
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "pending"
	DocumentStatusProcessing DocumentStatus = "processing"
	DocumentStatusEmbedding  DocumentStatus = "embedding"
	DocumentStatusCompleted  DocumentStatus = "completed"
	DocumentStatusFailed     DocumentStatus = "failed"
)

// DocumentContent represents the content JSONB structure
type DocumentContent struct {
	Description    *string                `json:"description,omitempty"`
	Path           *string                `json:"path,omitempty"`
	MimeType       *string                `json:"mime_type,omitempty"`
	SizeBytes      *int64                 `json:"size_bytes,omitempty"`
	Version        *int                   `json:"version,omitempty"`
	Checksum       *string                `json:"checksum,omitempty"`
	IsFolder       *bool                  `json:"is_folder,omitempty"`
	ErrorMessage   *string                `json:"error_message,omitempty"`
	ProcessingData map[string]interface{} `json:"processing_data,omitempty"`
}

// Document represents a document record in the database (unified files and documents table)
type Document struct {
	ID           int64                  `json:"id"`
	OrgID        *uuid.UUID             `json:"org_id,omitempty"`
	FolderID     *uuid.UUID             `json:"folder_id,omitempty"`
	ParentID     *int64                 `json:"parent_id,omitempty"`
	Name         string                 `json:"name"`
	FilePath     *string                `json:"file_path,omitempty"`
	JsonFilePath *string                `json:"json_file_path,omitempty"`
	Status       DocumentStatus         `json:"status"`
	Content      DocumentContent        `json:"content"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy    *uuid.UUID             `json:"created_by,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UploadedAt   *time.Time             `json:"uploaded_at,omitempty"`
	ProcessedAt  *time.Time             `json:"processed_at,omitempty"`
	UpdatedBy    *uuid.UUID             `json:"updated_by,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at"`
	DeletedBy    *uuid.UUID             `json:"deleted_by,omitempty"`
	DeletedAt    *time.Time             `json:"deleted_at,omitempty"`
	// Joined fields
	CreatedByName *string `json:"created_by_name,omitempty"`
	UpdatedByName *string `json:"updated_by_name,omitempty"`
	FolderName    *string `json:"folder_name,omitempty"`
}

// DocumentRepository handles document database operations
type DocumentRepository struct {
	db       *postgres.DB
	dbWriter *postgres.DB
}

// NewDocumentRepository creates a new document repository
func NewDocumentRepository(db *postgres.DB, dbWriter *postgres.DB) *DocumentRepository {
	return &DocumentRepository{
		db:       db,
		dbWriter: dbWriter,
	}
}

// CreateSchema creates the documents table if it doesn't exist (unified files and documents)
func (r *DocumentRepository) CreateSchema(ctx context.Context) error {
	query := `
		-- Create document_status enum if it doesn't exist
		DO $$ BEGIN
			CREATE TYPE document_status AS ENUM ('pending', 'processing', 'embedding', 'completed', 'failed');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		CREATE TABLE IF NOT EXISTS documents (
			id BIGSERIAL PRIMARY KEY,
			
			-- Multi-tenant org scope (nullable for superadmin documents)
			org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
			
			-- Hierarchy
			folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
			parent_id BIGINT REFERENCES documents(id) ON DELETE CASCADE,
			
			-- Minimal identity
			name VARCHAR(512) NOT NULL,
			
			-- File storage pointers
			file_path VARCHAR(1024),
			json_file_path VARCHAR(1024),
			
			-- Status
			status document_status DEFAULT 'completed',
			
			-- JSONB metadata
			content JSONB DEFAULT '{
				"description": null,
				"path": null,
				"mime_type": null,
				"size_bytes": 0,
				"version": 1,
				"checksum": null,
				"is_folder": false,
				"error_message": null,
				"processing_data": {}
			}'::jsonb NOT NULL,
			metadata JSONB DEFAULT '{}' NOT NULL,
			
			-- Audit
			created_by UUID REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMP DEFAULT NOW() NOT NULL,
			uploaded_at TIMESTAMP,
			processed_at TIMESTAMP,
			updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
			updated_at TIMESTAMP DEFAULT NOW() NOT NULL,
			deleted_by UUID REFERENCES users(id) ON DELETE SET NULL,
			deleted_at TIMESTAMP
		);

		-- Create indexes for common queries
		CREATE INDEX IF NOT EXISTS idx_documents_org_id ON documents(org_id);
		CREATE INDEX IF NOT EXISTS idx_documents_folder_id ON documents(folder_id) WHERE folder_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_documents_parent_id ON documents(parent_id) WHERE parent_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
		CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_documents_deleted_at ON documents(deleted_at) WHERE deleted_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_documents_file_path ON documents(file_path) WHERE file_path IS NOT NULL;
	`

	_, err := r.dbWriter.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create documents schema: %w", err)
	}

	return nil
}

// Create inserts a new document and returns the document with generated ID
func (r *DocumentRepository) Create(ctx context.Context, doc *Document) error {
	// ID will be auto-generated by BIGSERIAL

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	contentJSON, err := json.Marshal(doc.Content)
	if err != nil {
		contentJSON = []byte(`{"description": null, "path": null, "mime_type": null, "size_bytes": 0, "version": 1, "checksum": null, "is_folder": false, "error_message": null, "processing_data": {}}`)
	}

	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	if doc.UpdatedAt.IsZero() {
		doc.UpdatedAt = now
	}

	query := `
		INSERT INTO documents (org_id, folder_id, parent_id, name, file_path, json_file_path, status, content, metadata, created_by, created_at, uploaded_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`

	err = r.dbWriter.QueryRow(ctx, query,
		doc.OrgID,
		doc.FolderID,
		doc.ParentID,
		doc.Name,
		doc.FilePath,
		doc.JsonFilePath,
		doc.Status,
		contentJSON,
		metadataJSON,
		doc.CreatedBy,
		doc.CreatedAt,
		doc.UploadedAt,
		doc.UpdatedAt,
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}

	return nil
}

// GetByID retrieves a document by its ID
func (r *DocumentRepository) GetByID(ctx context.Context, id int64) (*Document, error) {
	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path, 
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.id = $1 AND d.deleted_at IS NULL
	`

	doc := &Document{}
	var metadataJSON, contentJSON []byte
	var createdByName, updatedByName, folderName *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&doc.ID,
		&doc.OrgID,
		&doc.FolderID,
		&doc.ParentID,
		&doc.Name,
		&doc.FilePath,
		&doc.JsonFilePath,
		&doc.Status,
		&contentJSON,
		&metadataJSON,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UploadedAt,
		&doc.ProcessedAt,
		&doc.UpdatedBy,
		&doc.UpdatedAt,
		&doc.DeletedBy,
		&doc.DeletedAt,
		&createdByName,
		&updatedByName,
		&folderName,
	)

	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}
	if len(contentJSON) > 0 {
		json.Unmarshal(contentJSON, &doc.Content)
	}

	doc.CreatedByName = createdByName
	doc.UpdatedByName = updatedByName
	doc.FolderName = folderName

	return doc, nil
}

// UpdateStatus updates the status of a document
func (r *DocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus, errorMessage *string) error {
	// Update status in content JSONB
	query := `
		UPDATE documents
		SET status = $1, 
		    content = jsonb_set(
		    	COALESCE(content, '{}'::jsonb),
		    	'{error_message}',
		    	COALESCE(to_jsonb($2::text), 'null'::jsonb)
		    ),
		    updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.dbWriter.Exec(ctx, query, status, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}

	return nil
}

// MarkEmbedding marks a document as embedding (document processing complete, starting vectorization)
func (r *DocumentRepository) MarkEmbedding(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE documents
		SET status = $1, processed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusEmbedding, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as embedding: %w", err)
	}

	return nil
}

// MarkProcessed marks a document as completed (only updates status and updated_at)
func (r *DocumentRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE documents
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusCompleted, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as completed: %w", err)
	}

	return nil
}

// MarkFailed marks a document as failed with an error message
func (r *DocumentRepository) MarkFailed(ctx context.Context, id uuid.UUID, errorMessage string) error {
	query := `
		UPDATE documents
		SET status = $1, 
		    content = jsonb_set(
		    	COALESCE(content, '{}'::jsonb),
		    	'{error_message}',
		    	to_jsonb($2::text)
		    ),
		    updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusFailed, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as failed: %w", err)
	}

	return nil
}

// ListAll retrieves ALL documents for an organization with pagination (files only, not folders)
func (r *DocumentRepository) ListAll(ctx context.Context, orgID uuid.UUID, page, limit int) ([]*Document, int64, error) {
	offset := (page - 1) * limit

	// Count query - all documents for this org, exclude folders and deleted
	countQuery := `
		SELECT COUNT(*) 
		FROM documents 
		WHERE deleted_at IS NULL
		AND (content->>'is_folder' IS NULL OR (content->>'is_folder')::boolean = false)
	`
	args := []interface{}{}

	// Only filter by org_id if it's not a zero UUID
	if orgID != uuid.Nil {
		countQuery += " AND org_id = $1"
		args = append(args, orgID)
	}

	var totalCount int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// List query - all documents for this org
	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.deleted_at IS NULL
		AND (d.content->>'is_folder' IS NULL OR (d.content->>'is_folder')::boolean = false)
	`
	queryArgs := []interface{}{}
	argIndex := 1

	// Only filter by org_id if it's not a zero UUID
	if orgID != uuid.Nil {
		query += fmt.Sprintf(" AND d.org_id = $%d", argIndex)
		queryArgs = append(queryArgs, orgID)
		argIndex++
	}

	query += fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	documents := make([]*Document, 0)
	for rows.Next() {
		doc := &Document{}
		var metadataJSON, contentJSON []byte
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&doc.ID, &doc.OrgID, &doc.FolderID, &doc.ParentID, &doc.Name, &doc.FilePath, &doc.JsonFilePath,
			&doc.Status, &contentJSON, &metadataJSON, &doc.CreatedBy, &doc.CreatedAt, &doc.UploadedAt, &doc.ProcessedAt,
			&doc.UpdatedBy, &doc.UpdatedAt, &doc.DeletedBy, &doc.DeletedAt,
			&createdByName, &updatedByName, &folderName,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			doc.Metadata = make(map[string]interface{})
		}
		if err := json.Unmarshal(contentJSON, &doc.Content); err != nil {
			doc.Content = DocumentContent{}
		}

		// Set names (already pointers)
		doc.CreatedByName = createdByName
		doc.UpdatedByName = updatedByName
		doc.FolderName = folderName

		documents = append(documents, doc)
	}

	return documents, totalCount, nil
}

// ListByFolder retrieves documents by folder ID and org_id with pagination (files only, not folders)
func (r *DocumentRepository) ListByFolder(ctx context.Context, folderID *uuid.UUID, orgID uuid.UUID, page, limit int) ([]*Document, int64, error) {
	offset := (page - 1) * limit

	// Count query - filter by folder_id and org_id (handle zero UUID for "all orgs"), exclude folders and deleted
	countQuery := `
		SELECT COUNT(*) 
		FROM documents 
		WHERE deleted_at IS NULL
		AND (content->>'is_folder' IS NULL OR (content->>'is_folder')::boolean = false)
	`
	args := []interface{}{}
	argIndex := 1

	// Only filter by org_id if it's not a zero UUID (zero UUID means "all orgs" for superadmins)
	if orgID != uuid.Nil {
		countQuery += fmt.Sprintf(" AND org_id = $%d", argIndex)
		args = append(args, orgID)
		argIndex++
	}

	if folderID != nil {
		countQuery += fmt.Sprintf(" AND folder_id = $%d", argIndex)
		args = append(args, *folderID)
		argIndex++
	} else {
		countQuery += " AND folder_id IS NULL"
	}

	var totalCount int64
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// List query - filter by folder_id and org_id (handle zero UUID for "all orgs")
	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.deleted_at IS NULL
		AND (d.content->>'is_folder' IS NULL OR (d.content->>'is_folder')::boolean = false)
	`
	queryArgs := []interface{}{}
	queryArgIndex := 1

	// Only filter by org_id if it's not a zero UUID (zero UUID means "all orgs" for superadmins)
	if orgID != uuid.Nil {
		query += fmt.Sprintf(" AND d.org_id = $%d", queryArgIndex)
		queryArgs = append(queryArgs, orgID)
		queryArgIndex++
	}

	if folderID != nil {
		query += fmt.Sprintf(" AND d.folder_id = $%d", queryArgIndex)
		queryArgs = append(queryArgs, *folderID)
		queryArgIndex++
	} else {
		query += " AND d.folder_id IS NULL"
	}

	query += fmt.Sprintf(" ORDER BY d.created_at DESC LIMIT $%d OFFSET $%d", queryArgIndex, queryArgIndex+1)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := r.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	documents := make([]*Document, 0)
	for rows.Next() {
		doc := &Document{}
		var metadataJSON, contentJSON []byte
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&doc.ID,
			&doc.OrgID,
			&doc.FolderID,
			&doc.ParentID,
			&doc.Name,
			&doc.FilePath,
			&doc.JsonFilePath,
			&doc.Status,
			&contentJSON,
			&metadataJSON,
			&doc.CreatedAt,
			&doc.UploadedAt,
			&doc.ProcessedAt,
			&doc.UpdatedBy,
			&doc.UpdatedAt,
			&doc.DeletedBy,
			&doc.DeletedAt,
			&createdByName,
			&updatedByName,
			&folderName,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &doc.Metadata)
		}
		if len(contentJSON) > 0 {
			json.Unmarshal(contentJSON, &doc.Content)
		}

		doc.CreatedByName = createdByName
		doc.UpdatedByName = updatedByName
		doc.FolderName = folderName

		documents = append(documents, doc)
	}

	return documents, totalCount, nil
}

// Delete removes a document by ID (hard delete)
func (r *DocumentRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM documents WHERE id = $1`

	result, err := r.dbWriter.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("document not found: %d", id)
	}

	return nil
}

// SoftDelete soft deletes a document by setting deleted_at
func (r *DocumentRepository) SoftDelete(ctx context.Context, id int64, deletedBy *uuid.UUID) error {
	query := `
		UPDATE documents
		SET deleted_at = NOW(), deleted_by = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := r.dbWriter.Exec(ctx, query, deletedBy, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete document: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("document not found: %d", id)
	}

	return nil
}

// DeleteByFilePath removes documents by file path (used when deleting files) - hard delete
func (r *DocumentRepository) DeleteByFilePath(ctx context.Context, filePath string) error {
	query := `DELETE FROM documents WHERE file_path = $1`

	_, err := r.dbWriter.Exec(ctx, query, filePath)
	if err != nil {
		return fmt.Errorf("failed to delete documents by file path: %w", err)
	}

	return nil
}

// GetByFilePath retrieves documents by file path (returns all matching documents)
// Tries multiple matching strategies to handle different file_path formats
func (r *DocumentRepository) GetByFilePath(ctx context.Context, filePath string) ([]*Document, error) {
	// Normalize the file path for matching
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")

	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.deleted_at IS NULL
		AND (d.file_path = $1
		   OR REPLACE(d.file_path, '\', '/') = $2
		   OR d.file_path LIKE '%' || $1
		   OR REPLACE(d.file_path, '\', '/') LIKE '%' || $2
		   OR d.file_path LIKE $1 || '%'
		   OR REPLACE(d.file_path, '\', '/') LIKE $2 || '%'
		   OR d.file_path LIKE '%' || $1 || '%'
		   OR REPLACE(d.file_path, '\', '/') LIKE '%' || $2 || '%')
	`

	rows, err := r.db.Query(ctx, query, filePath, normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents by file path: %w", err)
	}
	defer rows.Close()

	documents := make([]*Document, 0)
	for rows.Next() {
		doc := &Document{}
		var metadataJSON, contentJSON []byte
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&doc.ID,
			&doc.OrgID,
			&doc.FolderID,
			&doc.ParentID,
			&doc.Name,
			&doc.FilePath,
			&doc.JsonFilePath,
			&doc.Status,
			&contentJSON,
			&metadataJSON,
			&doc.CreatedBy,
			&doc.CreatedAt,
			&doc.UploadedAt,
			&doc.ProcessedAt,
			&doc.UpdatedBy,
			&doc.UpdatedAt,
			&doc.DeletedBy,
			&doc.DeletedAt,
			&createdByName,
			&updatedByName,
			&folderName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &doc.Metadata)
		}
		if len(contentJSON) > 0 {
			json.Unmarshal(contentJSON, &doc.Content)
		}

		doc.CreatedByName = createdByName
		doc.UpdatedByName = updatedByName
		doc.FolderName = folderName

		documents = append(documents, doc)
	}

	return documents, nil
}

// GetByStorageKeyPattern finds a document by matching file_path pattern (for static handler)
func (r *DocumentRepository) GetByStorageKeyPattern(ctx context.Context, pattern string) (*Document, error) {
	normalizedPattern := strings.ReplaceAll(pattern, "\\", "/")

	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.deleted_at IS NULL
		AND (d.file_path = $1
		   OR REPLACE(d.file_path, '\', '/') = $1
		   OR d.file_path LIKE '%' || $1
		   OR REPLACE(d.file_path, '\', '/') LIKE '%' || $1
		   OR d.file_path LIKE $1 || '%'
		   OR REPLACE(d.file_path, '\', '/') LIKE $1 || '%'
		   OR d.file_path LIKE '%' || $1 || '%'
		   OR REPLACE(d.file_path, '\', '/') LIKE '%' || $1 || '%')
		ORDER BY 
			CASE 
				WHEN d.file_path = $1 THEN 1
				WHEN REPLACE(d.file_path, '\', '/') = $1 THEN 2
				WHEN d.file_path LIKE $1 || '%' THEN 3
				WHEN REPLACE(d.file_path, '\', '/') LIKE $1 || '%' THEN 4
				WHEN d.file_path LIKE '%' || $1 THEN 5
				WHEN REPLACE(d.file_path, '\', '/') LIKE '%' || $1 THEN 6
				ELSE 7
			END
		LIMIT 1
	`

	doc := &Document{}
	var metadataJSON, contentJSON []byte
	var createdByName, updatedByName, folderName *string

	err := r.db.QueryRow(ctx, query, normalizedPattern).Scan(
		&doc.ID,
		&doc.OrgID,
		&doc.FolderID,
		&doc.ParentID,
		&doc.Name,
		&doc.FilePath,
		&doc.JsonFilePath,
		&doc.Status,
		&contentJSON,
		&metadataJSON,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UploadedAt,
		&doc.ProcessedAt,
		&doc.UpdatedBy,
		&doc.UpdatedAt,
		&doc.DeletedBy,
		&doc.DeletedAt,
		&createdByName,
		&updatedByName,
		&folderName,
	)

	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}
	if len(contentJSON) > 0 {
		json.Unmarshal(contentJSON, &doc.Content)
	}

	doc.CreatedByName = createdByName
	doc.UpdatedByName = updatedByName
	doc.FolderName = folderName

	return doc, nil
}

// GetByFilenameAndPath searches for a document by filename and path segments within an organization
func (r *DocumentRepository) GetByFilenameAndPath(ctx context.Context, orgID *uuid.UUID, filename string, pathSegments []string) (*Document, error) {
	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.name = $1
		AND d.deleted_at IS NULL
	`

	args := []interface{}{filename}
	argIndex := 2

	// If orgID is provided, filter by org
	if orgID != nil {
		query += fmt.Sprintf(" AND d.org_id = $%d", argIndex)
		args = append(args, *orgID)
		argIndex++
	}

	// Try to match path segments in file_path
	if len(pathSegments) > 0 {
		pathPattern := "%" + strings.Join(pathSegments, "%") + "%"
		query += fmt.Sprintf(" AND (REPLACE(d.file_path, '\\', '/') LIKE $%d OR d.file_path LIKE $%d)", argIndex, argIndex)
		args = append(args, pathPattern)
		argIndex++
	}

	query += " LIMIT 1"

	doc := &Document{}
	var metadataJSON, contentJSON []byte
	var createdByName, updatedByName, folderName *string

	err := r.db.QueryRow(ctx, query, args...).Scan(
		&doc.ID,
		&doc.OrgID,
		&doc.FolderID,
		&doc.ParentID,
		&doc.Name,
		&doc.FilePath,
		&doc.JsonFilePath,
		&doc.Status,
		&contentJSON,
		&metadataJSON,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UploadedAt,
		&doc.ProcessedAt,
		&doc.UpdatedBy,
		&doc.UpdatedAt,
		&doc.DeletedBy,
		&doc.DeletedAt,
		&createdByName,
		&updatedByName,
		&folderName,
	)

	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}
	if len(contentJSON) > 0 {
		json.Unmarshal(contentJSON, &doc.Content)
	}

	doc.CreatedByName = createdByName
	doc.UpdatedByName = updatedByName
	doc.FolderName = folderName

	return doc, nil
}

// Update updates a document
func (r *DocumentRepository) Update(ctx context.Context, doc *Document) error {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	contentJSON, err := json.Marshal(doc.Content)
	if err != nil {
		contentJSON = []byte(`{"description": null, "path": null, "mime_type": null, "size_bytes": 0, "version": 1, "checksum": null, "is_folder": false, "error_message": null, "processing_data": {}}`)
	}

	query := `
		UPDATE documents
		SET name = $1, folder_id = $2, file_path = $3, status = $4, content = $5, metadata = $6, 
		    processed_at = $7, updated_by = $8, updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err = r.dbWriter.QueryRow(ctx, query,
		doc.Name,
		doc.FolderID,
		doc.FilePath,
		doc.Status,
		contentJSON,
		metadataJSON,
		doc.ProcessedAt,
		doc.UpdatedBy,
		doc.ID,
	).Scan(&doc.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	return nil
}

// GetByFolder retrieves documents by folder ID (for folder handler)
func (r *DocumentRepository) GetByFolder(ctx context.Context, folderID uuid.UUID) ([]*Document, error) {
	query := `
		SELECT d.id, d.org_id, d.folder_id, d.parent_id, d.name, d.file_path, d.json_file_path,
		       d.status, d.content, d.metadata, d.created_by, d.created_at, d.uploaded_at, d.processed_at,
		       d.updated_by, d.updated_at, d.deleted_by, d.deleted_at,
		       u1.full_name as created_by_name, u2.full_name as updated_by_name,
		       COALESCE(f.name, '') as folder_name
		FROM documents d
		LEFT JOIN users u1 ON d.created_by = u1.id
		LEFT JOIN users u2 ON d.updated_by = u2.id
		LEFT JOIN folders f ON d.folder_id = f.id
		WHERE d.folder_id = $1 AND d.deleted_at IS NULL
		AND (d.content->>'is_folder' IS NULL OR (d.content->>'is_folder')::boolean = false)
		ORDER BY d.name ASC
	`

	rows, err := r.db.Query(ctx, query, folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents by folder: %w", err)
	}
	defer rows.Close()

	documents := make([]*Document, 0)
	for rows.Next() {
		doc := &Document{}
		var metadataJSON, contentJSON []byte
		var createdByName, updatedByName, folderName *string

		err := rows.Scan(
			&doc.ID,
			&doc.OrgID,
			&doc.FolderID,
			&doc.ParentID,
			&doc.Name,
			&doc.FilePath,
			&doc.JsonFilePath,
			&doc.Status,
			&contentJSON,
			&metadataJSON,
			&doc.CreatedBy,
			&doc.CreatedAt,
			&doc.UploadedAt,
			&doc.ProcessedAt,
			&doc.UpdatedBy,
			&doc.UpdatedAt,
			&doc.DeletedBy,
			&doc.DeletedAt,
			&createdByName,
			&updatedByName,
			&folderName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &doc.Metadata)
		}
		if len(contentJSON) > 0 {
			json.Unmarshal(contentJSON, &doc.Content)
		}

		doc.CreatedByName = createdByName
		doc.UpdatedByName = updatedByName
		doc.FolderName = folderName

		documents = append(documents, doc)
	}

	return documents, nil
}
