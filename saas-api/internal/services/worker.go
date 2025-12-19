package services

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"saas-api/cmd/defines"
	"saas-api/internal/repositories"
	"saas-api/pkg/weaviate"
	"sync"
	"time"

	fylogger "github.com/FyersDev/trading-logger-go"
)

// DocumentJob represents a document processing job
type DocumentJob struct {
	ID           int64
	FilePath     string
	JsonFilePath string
	FolderID     *string
	Metadata     map[string]interface{}
	Status       defines.JobStatus
	Error        error
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
}

// DocumentWorkerPool manages document processing workers
type DocumentWorkerPool struct {
	jobQueue       chan *DocumentJob
	jobs           map[int64]*DocumentJob
	jobsMu         sync.RWMutex
	weaviateClient *weaviate.WeaviateClient
	documentRepo   *repositories.DocumentRepository
	workerCount    int
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	WorkerCount int
	QueueSize   int
}

// DefaultWorkerPoolConfig returns sensible defaults
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		WorkerCount: 3,
		QueueSize:   100,
	}
}

// NewDocumentWorkerPool creates a new worker pool
func NewDocumentWorkerPool(weaviateClient *weaviate.WeaviateClient, documentRepo *repositories.DocumentRepository, config *WorkerPoolConfig) *DocumentWorkerPool {
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &DocumentWorkerPool{
		jobQueue:       make(chan *DocumentJob, config.QueueSize),
		jobs:           make(map[int64]*DocumentJob),
		weaviateClient: weaviateClient,
		documentRepo:   documentRepo,
		workerCount:    config.WorkerCount,
		ctx:            ctx,
		cancel:         cancel,
	}

	return pool
}

// Start initializes and starts all workers
func (p *DocumentWorkerPool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	fylogger.InfoLog(p.ctx, fmt.Sprintf("Started %d document processing workers", p.workerCount), nil)
}

// Stop gracefully shuts down all workers
func (p *DocumentWorkerPool) Stop() {
	p.cancel()
	close(p.jobQueue)
	p.wg.Wait()
	fylogger.InfoLog(p.ctx, "Document worker pool stopped", nil)
}

// worker is the main worker loop
func (p *DocumentWorkerPool) worker(id int) {
	defer p.wg.Done()

	fylogger.InfoLog(p.ctx, fmt.Sprintf("Worker %d started", id), nil)

	for {
		select {
		case <-p.ctx.Done():
			fylogger.InfoLog(p.ctx, fmt.Sprintf("Worker %d shutting down", id), nil)
			return
		case job, ok := <-p.jobQueue:
			if !ok {
				return
			}
			p.processJob(job, id)
		}
	}
}

// processJob handles the actual document processing
func (p *DocumentWorkerPool) processJob(job *DocumentJob, workerID int) {
	fylogger.InfoLog(p.ctx, fmt.Sprintf("Worker %d processing job %d", workerID, job.ID), nil)

	// Update job status to processing
	now := time.Now()
	p.updateJobStatus(job.ID, defines.JobStatusProcessing, nil)
	job.StartedAt = &now

	// Use virtual environment's Python to ensure all dependencies are available
	// When running from cmd/api, we need to go up to the saas-api root
	cmd := exec.Command("python", "../docling/document_process.py", job.FilePath, job.JsonFilePath)

	// Capture both stdout and stderr to see what's happening
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Log the actual Python error output
		errorDetails := fmt.Sprintf("Python stderr: %s, Python stdout: %s", stderr.String(), stdout.String())
		fylogger.ErrorLog(p.ctx, fmt.Sprintf("Worker %d: Failed to process document. %s", workerID, errorDetails), err, nil)
		p.updateJobStatus(job.ID, defines.JobStatusFailed, fmt.Errorf("%s: %w", errorDetails, err))
		return
	}

	// Log successful processing output if any
	if stdout.Len() > 0 {
		fylogger.InfoLog(p.ctx, fmt.Sprintf("Worker %d: Python output: %s", workerID, stdout.String()), nil)
	}

	// Mark as embedding (document processing complete, starting vectorization)
	p.updateJobStatus(job.ID, defines.JobStatusEmbedding, nil)

	// Populate Weaviate with chunks
	err := p.weaviateClient.PopulateFromMarkdownChunks(
		p.ctx,
		job.JsonFilePath,
		weaviate.DefaultPopulateConfig(),
		job.ID,
	)

	if err != nil {
		fylogger.ErrorLog(p.ctx, fmt.Sprintf("Worker %d: Failed to populate Weaviate", workerID), err, nil)
		p.updateJobStatus(job.ID, defines.JobStatusFailed, err)
		return
	}

	// Mark as completed (embedding done)
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	p.updateJobStatus(job.ID, defines.JobStatusCompleted, nil)

	fylogger.InfoLog(p.ctx, fmt.Sprintf("Worker %d: Job %d completed successfully", workerID, job.ID), nil)
}

// SubmitJob adds a new job to the queue (ID must be pre-assigned from database)
func (p *DocumentWorkerPool) SubmitJob(documentID int64, filePath, jsonFilePath string, folderID *string, metadata map[string]interface{}) (*DocumentJob, error) {
	job := &DocumentJob{
		ID:           documentID,
		FilePath:     filePath,
		JsonFilePath: jsonFilePath,
		FolderID:     folderID,
		Metadata:     metadata,
		Status:       defines.JobStatusPending,
		CreatedAt:    time.Now(),
	}

	// Store job for status tracking
	p.jobsMu.Lock()
	p.jobs[job.ID] = job
	p.jobsMu.Unlock()

	// Submit to queue (non-blocking with timeout)
	select {
	case p.jobQueue <- job:
		fylogger.InfoLog(p.ctx, fmt.Sprintf("Job %d submitted to queue", job.ID), nil)
		return job, nil
	case <-time.After(5 * time.Second):
		// Queue is full
		p.jobsMu.Lock()
		delete(p.jobs, job.ID)
		p.jobsMu.Unlock()
		return nil, fmt.Errorf("job queue is full, please try again later")
	}
}

// GetJobStatus retrieves the current status of a job
func (p *DocumentWorkerPool) GetJobStatus(jobID int64) (*DocumentJob, error) {
	p.jobsMu.RLock()
	defer p.jobsMu.RUnlock()

	job, exists := p.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %d", jobID)
	}

	return job, nil
}

// updateJobStatus updates the status of a job in memory and database
func (p *DocumentWorkerPool) updateJobStatus(jobID int64, status defines.JobStatus, err error) {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()

	if job, exists := p.jobs[jobID]; exists {
		job.Status = status
		job.Error = err
		if status == defines.JobStatusCompleted || status == defines.JobStatusFailed {
			now := time.Now()
			job.CompletedAt = &now
		}
	}

	// Update status in database
	if p.documentRepo != nil {
		var errMsg *string
		if err != nil {
			msg := err.Error()
			errMsg = &msg
		}

		// Update document status based on job status
		switch status {
		case defines.JobStatusEmbedding:
			// Update status to embedding
			doc, getErr := p.documentRepo.GetByID(p.ctx, jobID)
			if getErr != nil {
				fmt.Printf("Failed to get document %d for embedding status update: %v\n", jobID, getErr)
			} else {
				doc.Status = repositories.DocumentStatusEmbedding
				if updateErr := p.documentRepo.Update(p.ctx, doc); updateErr != nil {
					fmt.Printf("Failed to update document %d to embedding status: %v\n", jobID, updateErr)
				} else {
					fmt.Printf("Document %d status updated to: embedding\n", jobID)
				}
			}
		case defines.JobStatusCompleted:
			// Update status to completed
			doc, getErr := p.documentRepo.GetByID(p.ctx, jobID)
			if getErr != nil {
				fmt.Printf("Failed to get document %d for completed status update: %v\n", jobID, getErr)
			} else {
				doc.Status = repositories.DocumentStatusCompleted
				now := time.Now()
				doc.ProcessedAt = &now
				if updateErr := p.documentRepo.Update(p.ctx, doc); updateErr != nil {
					fmt.Printf("Failed to update document %d to completed status: %v\n", jobID, updateErr)
				} else {
					fmt.Printf("Document %d status updated to: completed\n", jobID)
				}
			}
		case defines.JobStatusFailed:
			// Update status to failed with error message
			doc, getErr := p.documentRepo.GetByID(p.ctx, jobID)
			if getErr != nil {
				fmt.Printf("Failed to get document %d for failed status update: %v\n", jobID, getErr)
			} else {
				doc.Status = repositories.DocumentStatusFailed
				if errMsg != nil {
					doc.Content.ErrorMessage = errMsg
				}
				if updateErr := p.documentRepo.Update(p.ctx, doc); updateErr != nil {
					fmt.Printf("Failed to update document %d to failed status: %v\n", jobID, updateErr)
				} else {
					fmt.Printf("Document %d status updated to: failed\n", jobID)
				}
			}
		case defines.JobStatusProcessing:
			// Update status to processing
			doc, getErr := p.documentRepo.GetByID(p.ctx, jobID)
			if getErr != nil {
				fmt.Printf("Failed to get document %d for processing status update: %v\n", jobID, getErr)
			} else {
				doc.Status = repositories.DocumentStatusProcessing
				if updateErr := p.documentRepo.Update(p.ctx, doc); updateErr != nil {
					fmt.Printf("Failed to update document %d to processing status: %v\n", jobID, updateErr)
				} else {
					fmt.Printf("Document %d status updated to: processing\n", jobID)
				}
			}
		default:
			// Update to custom status
			doc, getErr := p.documentRepo.GetByID(p.ctx, jobID)
			if getErr != nil {
				fmt.Printf("Failed to get document %d for status update: %v\n", jobID, getErr)
			} else {
				doc.Status = repositories.DocumentStatus(status)
				if errMsg != nil {
					doc.Content.ErrorMessage = errMsg
				}
				if updateErr := p.documentRepo.Update(p.ctx, doc); updateErr != nil {
					fmt.Printf("Failed to update document %d status: %v\n", jobID, updateErr)
				} else {
					fmt.Printf("Document %d status updated to: %s\n", jobID, status)
				}
			}
		}
	}
}

// GetAllJobs returns all jobs (for monitoring/debugging)
func (p *DocumentWorkerPool) GetAllJobs() []*DocumentJob {
	p.jobsMu.RLock()
	defer p.jobsMu.RUnlock()

	jobs := make([]*DocumentJob, 0, len(p.jobs))
	for _, job := range p.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CleanupOldJobs removes completed/failed jobs older than the specified duration from memory
func (p *DocumentWorkerPool) CleanupOldJobs(maxAge time.Duration) int {
	p.jobsMu.Lock()
	defer p.jobsMu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, job := range p.jobs {
		if (job.Status == defines.JobStatusCompleted || job.Status == defines.JobStatusFailed) &&
			job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(p.jobs, id)
			removed++
		}
	}

	return removed
}

// GetJobStatusFromDB retrieves job status from database (for jobs not in memory)
// TODO: Update to use UUID instead of int64
func (p *DocumentWorkerPool) GetJobStatusFromDB(ctx context.Context, jobID int64) (*repositories.Document, error) {
	// Worker pool is disabled until UUID migration is complete
	return nil, fmt.Errorf("worker pool needs to be updated to use UUID")
}
