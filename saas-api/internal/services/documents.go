package services

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"saas-api/internal/models"
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
	if resourcesBasePath == "" {
		resourcesBasePath = "uploads" // Default to uploads directory
	}

	jsonBasePath := os.Getenv("JSON_BASE_PATH")
	if jsonBasePath == "" {
		jsonBasePath = resourcesBasePath // Default to same as resources path
	}

	// Create worker pool with document repository
	workerPool := NewDocumentWorkerPool(
		base.weaviateClient,
		base.repositories.Document,
		DefaultWorkerPoolConfig(),
	)

	// Start the worker pool
	workerPool.Start()
	fmt.Printf("🚀 Document worker pool started with %d workers\n", DefaultWorkerPoolConfig().WorkerCount)

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
	DocumentID      int64         `json:"document_id"`
	Filename        string        `json:"filename"`
	Status          string        `json:"status"`
	ProcessingJobID int64         `json:"processing_job_id"`
	TimeTaken       time.Duration `json:"time_taken"`
}

// DocumentInfo represents a document's full information
type DocumentInfo struct {
	DocumentID   int64                  `json:"document_id"`
	Name         string                 `json:"name"`
	FilePath     string                 `json:"file_path"`
	FolderID     *string                `json:"folder_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Status       string                 `json:"status"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
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

	// Convert folder ID from *string to *uuid.UUID
	var folderUUID *uuid.UUID
	if req.FolderID != nil && *req.FolderID != "" {
		parsed, err := uuid.Parse(*req.FolderID)
		if err == nil {
			folderUUID = &parsed
			fmt.Printf("📁 Using specified folder: %s\n", *req.FolderID)
		} else {
			fmt.Printf("⚠️  Invalid folder ID format: %s\n", *req.FolderID)
		}
	}

	// Only auto-assign to Resources folder if:
	// 1. No folder specified (folderUUID is nil AND req.FolderID is nil or empty)
	// 2. Org ID exists
	if folderUUID == nil && req.OrgID != nil && (req.FolderID == nil || *req.FolderID == "") {
		resourcesFolderID, err := s.getOrCreateResourcesFolder(ctx, *req.OrgID, &userUUID)
		if err != nil {
			fmt.Printf("⚠️  Failed to get/create Resources folder: %v\n", err)
			// Continue without folder - don't fail the upload
		} else {
			folderUUID = resourcesFolderID
			fmt.Printf("📁 Auto-assigned to Resources folder: %s\n", *resourcesFolderID)
		}
	} else if folderUUID != nil {
		fmt.Printf("📁 Document will be saved to folder: %s\n", *folderUUID)
	} else {
		fmt.Printf("📁 Document will be saved without folder (root level)\n")
	}

	// Check if document is in Reports folder - if so, don't process it
	isInReportsFolder := false
	if folderUUID != nil {
		folder, err := s.repositories.Folder.GetByID(ctx, *folderUUID)
		if err == nil {
			// Check if this is the Reports folder or a subfolder of Reports
			if strings.ToLower(folder.Name) == "reports" || strings.Contains(strings.ToLower(folder.Path), "/reports") {
				isInReportsFolder = true
				fmt.Printf("📋 Document is in Reports folder (%s) - will not be processed for AI\n", folder.Name)
			}
		}
	}

	// Set status based on whether it's in Reports folder
	// Reports folder documents are marked as completed immediately (no processing needed)
	docStatus := repositories.DocumentStatusPending
	if isInReportsFolder {
		docStatus = repositories.DocumentStatusCompleted
	}

	doc := &repositories.Document{
		// ID will be auto-generated by BIGSERIAL
		OrgID:        req.OrgID,
		Name:         filename,
		FilePath:     &req.FilePath,
		JsonFilePath: &jsonFilePath,
		FolderID:     folderUUID,
		Metadata:     req.Metadata,
		Status:       docStatus,
		CreatedBy:    &userUUID,
	}

	err = s.repositories.Document.Create(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// Only submit job to worker pool if NOT in Reports folder
	if !isInReportsFolder {
		if s.WorkerPool != nil {
			// Construct full disk path for Python worker (needs absolute path to read file)
			// req.FilePath is relative (e.g., "org_id/folder/file.pdf")
			// Worker needs full path (e.g., "uploads/org_id/folder/file.pdf")
			fullDiskPath := path.Join(s.ResourcesBasePath, req.FilePath)

			_, err = s.WorkerPool.SubmitJob(doc.ID, fullDiskPath, jsonFilePath, req.FolderID, req.Metadata)
			if err != nil {
				fmt.Printf("⚠️  Failed to submit job to worker pool: %v\n", err)
				// Don't fail the upload, just log the warning
			} else {
				fmt.Printf("✅ Job %d submitted to worker pool for processing\n", doc.ID)
			}
		} else {
			fmt.Printf("⚠️  Worker pool not initialized, document will remain in pending status\n")
		}
	} else {
		fmt.Printf("📋 Skipped processing for Reports folder document: %s\n", filename)
	}

	return &UploadDocumentResponse{
		DocumentID:      doc.ID,
		Filename:        filename,
		Status:          string(doc.Status),
		ProcessingJobID: doc.ID,
		TimeTaken:       time.Since(startTime),
	}, nil
}

// GetJobStatus returns the status of a document processing job
func (s *DocumentService) GetJobStatus(ctx context.Context, jobID string) (*DocumentInfo, error) {
	// Parse jobID as int64
	id, err := strconv.ParseInt(jobID, 10, 64)
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
		DocumentID:   doc.ID,
		Name:         doc.Name,
		FilePath:     filePath,
		FolderID:     folderIDStr,
		Metadata:     doc.Metadata,
		Status:       string(doc.Status),
		ErrorMessage: doc.ErrorMessage,
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
			DocumentID:   doc.ID,
			Name:         doc.Name,
			FilePath:     filePath,
			FolderID:     folderIDStr,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.ErrorMessage,
			CreatedAt:    doc.CreatedAt,
			UploadedAt:   doc.UploadedAt,
			ProcessedAt:  doc.ProcessedAt,
		}
	}

	return result, nil
}

// GetDocumentsWithFilter retrieves documents with optional folder filter and pagination
func (s *DocumentService) GetDocumentsWithFilter(ctx context.Context, req *GetDocumentsRequest) (*GetDocumentsResponse, error) {
	// Use zero UUID if orgID is nil
	orgID := uuid.UUID{}
	if req.OrgID != nil {
		orgID = *req.OrgID
	}

	// For chat documents selector: if no folder specified, return ALL documents (not just root)
	// For Resources page: if folder specified, return only documents in that folder
	var docs []*repositories.Document
	var totalCount int64
	var err error

	if req.FolderID != nil {
		// Specific folder requested - use ListByFolder
		parsed, parseErr := uuid.Parse(*req.FolderID)
		if parseErr == nil {
			folderUUID := &parsed
			docs, totalCount, err = s.repositories.Document.ListByFolder(ctx, folderUUID, orgID, req.Page, req.Limit)
		} else {
			return nil, fmt.Errorf("invalid folder ID: %w", parseErr)
		}
	} else {
		// No folder specified - return ALL documents for this org (chat documents selector)
		docs, totalCount, err = s.repositories.Document.ListAll(ctx, orgID, req.Page, req.Limit)
	}
	if err != nil {
		return nil, err
	}

	// fmt.Printf(" ListByFolder returned %d documents (totalCount: %d)\n", len(docs), totalCount)
	// for _, doc := range docs {
	// 	fmt.Printf("  - ID: %d, Name: %s, Status: %s\n", doc.ID, doc.Name, doc.Status)
	// }

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
			DocumentID:   doc.ID,
			Name:         doc.Name,
			FilePath:     filePath,
			FolderID:     folderIDStr,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.ErrorMessage,
			CreatedAt:    doc.CreatedAt,
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
func (s *DocumentService) DeleteDocument(ctx context.Context, documentID int64) error {
	// First, verify the document exists
	doc, err := s.repositories.Document.GetByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// Delete from Weaviate using the int64 document ID directly
	err = s.GetWeaviateClient().DeleteDocumentClasses(ctx, documentID)
	if err != nil {
		// Log but don't fail - Weaviate might not have this document
		fmt.Printf("Warning: Failed to delete document from Weaviate: %v\n", err)
	}

	// Delete from PostgreSQL
	err = s.repositories.Document.Delete(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document from database: %w", err)
	}

	fmt.Printf("Document %s (ID: %d) deleted successfully\n", doc.Name, documentID)
	return nil
}

// getOrCreateResourcesFolder gets or creates a "Resources" root folder for the organization
func (s *DocumentService) getOrCreateResourcesFolder(ctx context.Context, orgID uuid.UUID, createdBy *uuid.UUID) (*uuid.UUID, error) {
	// Try to find existing "Resources" folder (root level, no parent)
	folders, err := s.repositories.Folder.List(ctx, orgID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	// Look for existing "Resources" folder
	for _, folder := range folders {
		if folder.Name == "Resources" {
			folderID := folder.ID
			return &folderID, nil
		}
	}

	// Create new "Resources" folder
	newID := uuid.New()
	resourcesFolder := &models.Folder{
		ID:        newID,
		OrgID:     orgID,
		Name:      "Resources",
		Path:      "/Resources",
		ParentID:  nil,
		CreatedBy: createdBy,
	}

	err = s.repositories.Folder.Create(ctx, resourcesFolder)
	if err != nil {
		return nil, fmt.Errorf("failed to create Resources folder: %w", err)
	}

	fmt.Printf("✅ Created new Resources folder for org %s\n", orgID)
	return &newID, nil
}
