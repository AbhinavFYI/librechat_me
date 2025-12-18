package defines

// JobStatus represents the current state of a document processing job
type JobStatus string

const (
	JobStatusPending    JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusEmbedding  JobStatus = "embedding"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)
