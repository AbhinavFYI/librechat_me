package services

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"saas-api/internal/repositories"
	"saas-api/pkg/weaviate"

	"github.com/google/uuid"
)

// DocumentService handles document operations
type DocumentService struct {
	*BaseService
	ResourcesBasePath string
	JsonBasePath      string
	WorkerPool        *DocumentWorkerPool
}

// NewDocumentService creates a new document service
func NewDocumentService(base *BaseService) *DocumentService {
	resourcesBasePath := os.Getenv("RESOURCES_BASE_PATH")
	jsonBasePath := os.Getenv("JSON_BASE_PATH")

	// Create worker pool with document repository
	workerPool := NewDocumentWorkerPool(
		base.weaviateClient,
		base.repositories.Document,
		DefaultWorkerPoolConfig(),
	)

	return &DocumentService{
		BaseService:       base,
		ResourcesBasePath: resourcesBasePath,
		JsonBasePath:      jsonBasePath,
		WorkerPool:        workerPool,
	}
}

// InitSchema initializes the database schema for documents
func (s *DocumentService) InitSchema(ctx context.Context) error {
	return s.repositories.Document.CreateSchema(ctx)
}

// StartWorkers starts the document processing workers
func (s *DocumentService) StartWorkers() {
	s.WorkerPool.Start()
}

// StopWorkers stops all workers gracefully
func (s *DocumentService) StopWorkers() {
	s.WorkerPool.Stop()
}

// UploadDocumentRequest represents the request for uploading a document
type UploadDocumentRequest struct {
	UserID   string     // UUID as string
	OrgID    *uuid.UUID // Nullable for superadmins
	FilePath string
	FolderID *string
	Metadata map[string]interface{}
}

// UploadDocumentResponse represents the response after uploading a document
type UploadDocumentResponse struct {
	DocumentID      string        `json:"document_id"`
	Filename        string        `json:"filename"`
	Status          string        `json:"status"`
	ProcessingJobID string        `json:"processing_job_id"`
	TimeTaken       time.Duration `json:"time_taken"`
}

// DocumentInfo represents a document's full information
type DocumentInfo struct {
	DocumentID   string                 `json:"document_id"`
	Name         string                 `json:"name"`
	FilePath     string                 `json:"file_path"`
	FolderID     *string                `json:"folder_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Status       string                 `json:"status"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	UploadedAt   *time.Time             `json:"uploaded_at,omitempty"`
	ProcessedAt  *time.Time             `json:"processed_at,omitempty"`
}

// UploadDocument handles the document upload business logic asynchronously
func (s *DocumentService) UploadDocument(ctx context.Context, req *UploadDocumentRequest) (*UploadDocumentResponse, error) {
	startTime := time.Now()

	// Prepare file paths
	filename := path.Base(req.FilePath)
	filePathWithoutExtension := strings.TrimSuffix(req.FilePath, path.Ext(req.FilePath))
	jsonFilePath := path.Join(s.JsonBasePath, path.Base(filePathWithoutExtension)+"_chunks.json")

	// Create document record in database first to get the ID
	// Parse user ID
	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Validate and parse org ID
	if req.OrgID == nil {
		return nil, fmt.Errorf("org_id is required")
	}
	orgID := *req.OrgID

	// Convert folder ID from *string to *uuid.UUID
	var folderUUID *uuid.UUID
	if req.FolderID != nil {
		parsed, err := uuid.Parse(*req.FolderID)
		if err == nil {
			folderUUID = &parsed
		}
	}

	doc := &repositories.Document{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         filename,
		FilePath:     &req.FilePath,
		JsonFilePath: &jsonFilePath,
		FolderID:     folderUUID,
		Metadata:     req.Metadata,
		Status:       repositories.DocumentStatusPending,
		CreatedBy:    &userUUID,
	}

	err = s.repositories.Document.Create(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	return &UploadDocumentResponse{
		DocumentID:      doc.ID.String(),
		Filename:        filename,
		Status:          string(doc.Status),
		ProcessingJobID: doc.ID.String(),
		TimeTaken:       time.Since(startTime),
	}, nil
}

// GetJobStatus returns the status of a document processing job
func (s *DocumentService) GetJobStatus(ctx context.Context, jobID string) (*DocumentInfo, error) {
	// Parse jobID as UUID
	id, err := uuid.Parse(jobID)
	if err != nil {
		return nil, fmt.Errorf("invalid job ID: %s", jobID)
	}

	// Get document from database
	doc, err := s.repositories.Document.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var filePath string
	if doc.FilePath != nil {
		filePath = *doc.FilePath
	}

	var folderIDStr *string
	if doc.FolderID != nil {
		folderID := doc.FolderID.String()
		folderIDStr = &folderID
	}

	return &DocumentInfo{
		DocumentID:   doc.ID.String(),
		Name:         doc.Name,
		FilePath:     filePath,
		FolderID:     folderIDStr,
		Metadata:     doc.Metadata,
		Status:       string(doc.Status),
		ErrorMessage: doc.Content.ErrorMessage,
		UploadedAt:   doc.UploadedAt,
		ProcessedAt:  doc.ProcessedAt,
	}, nil
}

// GetAllJobs returns all document processing jobs (from memory and database)
func (s *DocumentService) GetAllJobs(ctx context.Context) []*DocumentJob {
	return s.WorkerPool.GetAllJobs()
}

// GetDocumentsRequest represents the request for getting documents with filters
type GetDocumentsRequest struct {
	FolderID *string
	OrgID    *uuid.UUID // Nullable for superadmins
	Page     int
	Limit    int
}

// GetDocumentsResponse represents paginated document response
type GetDocumentsResponse struct {
	Documents  []DocumentInfo `json:"documents"`
	TotalCount int            `json:"total_count"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
}

// GetDocuments retrieves all documents with their status
func (s *DocumentService) GetDocuments(ctx context.Context) ([]DocumentInfo, error) {
	// Use zero UUID for "all orgs" query
	var zeroUUID uuid.UUID
	docs, _, err := s.repositories.Document.ListByFolder(ctx, nil, zeroUUID, 1, 100)
	if err != nil {
		return nil, err
	}

	result := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		var filePath string
		if doc.FilePath != nil {
			filePath = *doc.FilePath
		}

		var folderIDStr *string
		if doc.FolderID != nil {
			folderID := doc.FolderID.String()
			folderIDStr = &folderID
		}

		result[i] = DocumentInfo{
			DocumentID:   doc.ID.String(),
			Name:         doc.Name,
			FilePath:     filePath,
			FolderID:     folderIDStr,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.Content.ErrorMessage,
			UploadedAt:   doc.UploadedAt,
			ProcessedAt:  doc.ProcessedAt,
		}
	}

	return result, nil
}

// GetDocumentsWithFilter retrieves documents with optional folder filter and pagination
func (s *DocumentService) GetDocumentsWithFilter(ctx context.Context, req *GetDocumentsRequest) (*GetDocumentsResponse, error) {
	// Convert folder ID from *string to *uuid.UUID
	var folderUUID *uuid.UUID
	if req.FolderID != nil {
		parsed, err := uuid.Parse(*req.FolderID)
		if err == nil {
			folderUUID = &parsed
		}
	}

	// Use zero UUID if orgID is nil
	orgID := uuid.UUID{}
	if req.OrgID != nil {
		orgID = *req.OrgID
	}

	docs, totalCount, err := s.repositories.Document.ListByFolder(ctx, folderUUID, orgID, req.Page, req.Limit)
	if err != nil {
		return nil, err
	}

	result := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		var filePath string
		if doc.FilePath != nil {
			filePath = *doc.FilePath
		}

		var folderIDStr *string
		if doc.FolderID != nil {
			folderID := doc.FolderID.String()
			folderIDStr = &folderID
		}

		result[i] = DocumentInfo{
			DocumentID:   doc.ID.String(),
			Name:         doc.Name,
			FilePath:     filePath,
			FolderID:     folderIDStr,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.Content.ErrorMessage,
			UploadedAt:   doc.UploadedAt,
			ProcessedAt:  doc.ProcessedAt,
		}
	}

	return &GetDocumentsResponse{
		Documents:  result,
		TotalCount: int(totalCount),
		Page:       req.Page,
		Limit:      req.Limit,
	}, nil
}

// SearchDocuments searches for documents in Weaviate
func (s *DocumentService) SearchDocuments(ctx context.Context,
	query string,
	collection string,
	score float64,
	alpha float32,
) ([]weaviate.Chunk, error) {
	results, err := s.GetWeaviateClient().QueryHybridWithCollection(ctx, query, collection, score, alpha)
	if err != nil {
		return nil, err
	}

	fmt.Println("Results:", results)
	return results, nil
}

// DeleteDocument deletes a document from both Weaviate and PostgreSQL
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID uuid.UUID) error {
	// First, verify the document exists
	doc, err := s.repositories.Document.GetByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// Delete from Weaviate (convert UUID to int64 for Weaviate - using hash or first 8 bytes)
	// Weaviate uses document ID as int64, so we need to convert UUID
	// For now, we'll use a hash of the UUID to get a consistent int64
	docIDInt64 := int64(documentID[0])<<56 | int64(documentID[1])<<48 | int64(documentID[2])<<40 | int64(documentID[3])<<32 |
		int64(documentID[4])<<24 | int64(documentID[5])<<16 | int64(documentID[6])<<8 | int64(documentID[7])

	err = s.GetWeaviateClient().DeleteDocumentClasses(ctx, docIDInt64)
	if err != nil {
		// Log but don't fail - Weaviate might not have this document
		fmt.Printf("Warning: Failed to delete document from Weaviate: %v\n", err)
	}

	// Delete from PostgreSQL
	err = s.repositories.Document.Delete(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document from database: %w", err)
	}

	fmt.Printf("Document %s (ID: %s) deleted successfully\n", doc.Name, documentID)
	return nil
}
