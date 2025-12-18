package services

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
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
	UploadedAt   time.Time              `json:"uploaded_at"`
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
	doc := &repositories.Document{
		UserID:       req.UserID,
		OrgID:        req.OrgID,
		Name:         filename,
		FilePath:     req.FilePath,
		JsonFilePath: jsonFilePath,
		FolderID:     req.FolderID,
		Metadata:     req.Metadata,
		Status:       repositories.DocumentStatusPending,
	}

	documentID, err := s.repositories.Document.Create(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create document record: %w", err)
	}

	// Submit job to worker pool (async processing) with the database ID
	job, err := s.WorkerPool.SubmitJob(documentID, req.FilePath, jsonFilePath, req.FolderID, req.Metadata)
	if err != nil {
		// Update document status to failed if job submission fails
		s.repositories.Document.MarkFailed(ctx, documentID, err.Error())
		return nil, err
	}

	return &UploadDocumentResponse{
		DocumentID:      documentID,
		Filename:        filename,
		Status:          string(job.Status),
		ProcessingJobID: documentID,
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

	// First check in-memory job status
	job, err := s.WorkerPool.GetJobStatus(id)
	if err == nil && job != nil {
		return &DocumentInfo{
			DocumentID:  job.ID,
			Name:        path.Base(job.FilePath),
			FilePath:    job.FilePath,
			FolderID:    job.FolderID,
			Metadata:    job.Metadata,
			Status:      string(job.Status),
			UploadedAt:  job.CreatedAt,
			ProcessedAt: job.CompletedAt,
		}, nil
	}

	// Fall back to database
	doc, err := s.repositories.Document.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &DocumentInfo{
		DocumentID:   doc.ID,
		Name:         doc.Name,
		FilePath:     doc.FilePath,
		FolderID:     doc.FolderID,
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
	docs, _, err := s.repositories.Document.ListByFolder(ctx, nil, nil, 1, 100)
	if err != nil {
		return nil, err
	}

	result := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		result[i] = DocumentInfo{
			DocumentID:   doc.ID,
			Name:         doc.Name,
			FilePath:     doc.FilePath,
			FolderID:     doc.FolderID,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.ErrorMessage,
			UploadedAt:   doc.UploadedAt,
			ProcessedAt:  doc.ProcessedAt,
		}
	}

	return result, nil
}

// GetDocumentsWithFilter retrieves documents with optional folder filter and pagination
func (s *DocumentService) GetDocumentsWithFilter(ctx context.Context, req *GetDocumentsRequest) (*GetDocumentsResponse, error) {
	docs, totalCount, err := s.repositories.Document.ListByFolder(ctx, req.FolderID, req.OrgID, req.Page, req.Limit)
	if err != nil {
		return nil, err
	}

	result := make([]DocumentInfo, len(docs))
	for i, doc := range docs {
		result[i] = DocumentInfo{
			DocumentID:   doc.ID,
			Name:         doc.Name,
			FilePath:     doc.FilePath,
			FolderID:     doc.FolderID,
			Metadata:     doc.Metadata,
			Status:       string(doc.Status),
			ErrorMessage: doc.ErrorMessage,
			UploadedAt:   doc.UploadedAt,
			ProcessedAt:  doc.ProcessedAt,
		}
	}

	return &GetDocumentsResponse{
		Documents:  result,
		TotalCount: totalCount,
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

	// Delete from Weaviate
	err = s.GetWeaviateClient().DeleteDocumentClasses(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document from Weaviate: %w", err)
	}

	// Delete from PostgreSQL
	err = s.repositories.Document.Delete(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document from database: %w", err)
	}

	// Remove from worker pool if exists
	s.WorkerPool.jobsMu.Lock()
	delete(s.WorkerPool.jobs, documentID)
	s.WorkerPool.jobsMu.Unlock()

	fmt.Printf("Document %s (ID: %d) deleted successfully\n", doc.Name, documentID)
	return nil
}
