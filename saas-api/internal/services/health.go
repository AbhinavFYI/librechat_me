package services

import (
	"context"
	"time"

	"saas-api/pkg/memorydb"
	"saas-api/pkg/postgres"
)

// HealthStatus represents the status of a service
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Details   string    `json:"details,omitempty"`
}

// HealthService handles health check operations
type HealthService struct {
	db       *postgres.DB          // Reader DB
	dbWriter *postgres.DB          // Writer DB
	redis    *memorydb.RedisClient // Redis client
}

// NewHealthService creates a new health service
func NewHealthService(
	db *postgres.DB,
	dbWriter *postgres.DB,
	redis *memorydb.RedisClient,
) *HealthService {
	return &HealthService{
		db:       db,
		dbWriter: dbWriter,
		redis:    redis,
	}
}

// CheckDatabase checks both read and write database connections
func (s *HealthService) CheckDatabase(ctx context.Context) map[string]HealthStatus {
	status := make(map[string]HealthStatus)

	// Check read database
	if err := s.db.Ping(ctx); err != nil {
		status["read_db"] = HealthStatus{
			Status:    "error",
			Timestamp: time.Now(),
			Details:   err.Error(),
		}
	} else {
		status["read_db"] = HealthStatus{
			Status:    "ok",
			Timestamp: time.Now(),
		}
	}

	// Check write database
	if err := s.dbWriter.Ping(ctx); err != nil {
		status["write_db"] = HealthStatus{
			Status:    "error",
			Timestamp: time.Now(),
			Details:   err.Error(),
		}
	} else {
		status["write_db"] = HealthStatus{
			Status:    "ok",
			Timestamp: time.Now(),
		}
	}

	return status
}

// CheckRedis checks Redis connection
func (s *HealthService) CheckRedis(ctx context.Context) map[string]HealthStatus {
	status := make(map[string]HealthStatus)

	if err := s.redis.Ping(ctx); err != nil {
		status["redis"] = HealthStatus{
			Status:    "error",
			Timestamp: time.Now(),
			Details:   err.Error(),
		}
	} else {
		status["redis"] = HealthStatus{
			Status:    "ok",
			Timestamp: time.Now(),
		}
	}

	return status
}

// CheckOverall checks all services
func (s *HealthService) CheckOverall(ctx context.Context) map[string]HealthStatus {
	status := make(map[string]HealthStatus)

	// Check databases
	dbStatus := s.CheckDatabase(ctx)
	for k, v := range dbStatus {
		status[k] = v
	}

	// Check Redis
	redisStatus := s.CheckRedis(ctx)
	for k, v := range redisStatus {
		status[k] = v
	}

	return status
}
