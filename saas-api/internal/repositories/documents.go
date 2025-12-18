package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"saas-api/pkg/postgres"

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

// Document represents a document record in the database
type Document struct {
	ID           int64                  `json:"id"`
	UserID       string                 `json:"user_id"`          // UUID as string
	OrgID        *uuid.UUID             `json:"org_id,omitempty"` // Nullable for superadmins
	Name         string                 `json:"name"`
	FilePath     string                 `json:"file_path"`
	JsonFilePath string                 `json:"json_file_path"`
	FolderID     *string                `json:"folder_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Status       DocumentStatus         `json:"status"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	UploadedAt   time.Time              `json:"uploaded_at"`
	ProcessedAt  *time.Time             `json:"processed_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
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

// CreateSchema creates the documents table if it doesn't exist
func (r *DocumentRepository) CreateSchema(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS documents (
			id BIGSERIAL PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			org_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
			name VARCHAR(512) NOT NULL,
			file_path VARCHAR(1024) NOT NULL,
			json_file_path VARCHAR(1024),
			folder_id VARCHAR(64),
			metadata JSONB DEFAULT '{}',
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			error_message TEXT,
			uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		-- Create indexes for common queries
		CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id);
		CREATE INDEX IF NOT EXISTS idx_documents_org_id ON documents(org_id);
		CREATE INDEX IF NOT EXISTS idx_documents_folder_id ON documents(folder_id);
		CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
		CREATE INDEX IF NOT EXISTS idx_documents_uploaded_at ON documents(uploaded_at DESC);
	`

	_, err := r.dbWriter.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create documents schema: %w", err)
	}

	return nil
}

// Create inserts a new document and returns the generated ID
func (r *DocumentRepository) Create(ctx context.Context, doc *Document) (int64, error) {
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO documents (user_id, org_id, name, file_path, json_file_path, folder_id, metadata, status, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var id int64
	err = r.dbWriter.QueryRow(ctx, query,
		doc.UserID,
		doc.OrgID,
		doc.Name,
		doc.FilePath,
		doc.JsonFilePath,
		doc.FolderID,
		metadataJSON,
		doc.Status,
		time.Now(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("failed to create document: %w", err)
	}

	return id, nil
}

// GetByID retrieves a document by its ID
func (r *DocumentRepository) GetByID(ctx context.Context, id int64) (*Document, error) {
	query := `
		SELECT id, user_id, org_id, name, file_path, json_file_path, folder_id, metadata, status, 
		       error_message, uploaded_at, processed_at, created_at, updated_at
		FROM documents
		WHERE id = $1
	`

	doc := &Document{}
	var metadataJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&doc.ID,
		&doc.UserID,
		&doc.OrgID,
		&doc.Name,
		&doc.FilePath,
		&doc.JsonFilePath,
		&doc.FolderID,
		&metadataJSON,
		&doc.Status,
		&doc.ErrorMessage,
		&doc.UploadedAt,
		&doc.ProcessedAt,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &doc.Metadata)
	}

	return doc, nil
}

// UpdateStatus updates the status of a document
func (r *DocumentRepository) UpdateStatus(ctx context.Context, id int64, status DocumentStatus, errorMessage *string) error {
	query := `
		UPDATE documents
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.dbWriter.Exec(ctx, query, status, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}

	return nil
}

// MarkEmbedding marks a document as embedding (document processing complete, starting vectorization)
func (r *DocumentRepository) MarkEmbedding(ctx context.Context, id int64) error {
	query := `
		UPDATE documents
		SET status = $1, processed_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusEmbedding, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as embedding: %w", err)
	}

	return nil
}

// MarkProcessed marks a document as completed (only updates status and updated_at)
func (r *DocumentRepository) MarkProcessed(ctx context.Context, id int64) error {
	query := `
		UPDATE documents
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusCompleted, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as completed: %w", err)
	}

	return nil
}

// MarkFailed marks a document as failed with an error message
func (r *DocumentRepository) MarkFailed(ctx context.Context, id int64, errorMessage string) error {
	query := `
		UPDATE documents
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.dbWriter.Exec(ctx, query, DocumentStatusFailed, errorMessage, id)
	if err != nil {
		return fmt.Errorf("failed to mark document as failed: %w", err)
	}

	return nil
}

// ListByFolder retrieves documents by folder ID and org_id with pagination
func (r *DocumentRepository) ListByFolder(ctx context.Context, folderID *string, orgID *uuid.UUID, page, limit int) ([]*Document, int, error) {
	offset := (page - 1) * limit

	// Count query - filter by folder_id and org_id (org_id can be null for superadmins)
	countQuery := `
		SELECT COUNT(*) 
		FROM documents 
		WHERE ($1::varchar IS NULL OR folder_id = $1)
		AND ($2::uuid IS NULL OR org_id = $2)
	`
	var totalCount int
	err := r.db.QueryRow(ctx, countQuery, folderID, orgID).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// List query - filter by folder_id and org_id (org_id can be null for superadmins)
	query := `
		SELECT id, user_id, org_id, name, file_path, json_file_path, folder_id, metadata, status, 
		       error_message, uploaded_at, processed_at, created_at, updated_at
		FROM documents
		WHERE ($1::varchar IS NULL OR folder_id = $1)
		AND ($2::uuid IS NULL OR org_id = $2)
		ORDER BY uploaded_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(ctx, query, folderID, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	documents := make([]*Document, 0)
	for rows.Next() {
		doc := &Document{}
		var metadataJSON []byte

		err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.OrgID,
			&doc.Name,
			&doc.FilePath,
			&doc.JsonFilePath,
			&doc.FolderID,
			&metadataJSON,
			&doc.Status,
			&doc.ErrorMessage,
			&doc.UploadedAt,
			&doc.ProcessedAt,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document: %w", err)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &doc.Metadata)
		}

		documents = append(documents, doc)
	}

	return documents, totalCount, nil
}

// Delete removes a document by ID
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
