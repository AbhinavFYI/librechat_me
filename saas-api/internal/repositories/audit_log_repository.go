package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"saas-api/internal/database"
	"saas-api/internal/models"
	"saas-api/pkg/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditLogRepository struct {
	db *database.DB
}

func NewAuditLogRepository(db *database.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// List retrieves audit logs with pagination and optional filters
func (r *AuditLogRepository) List(ctx context.Context, orgID *uuid.UUID, userID *uuid.UUID, action *string, resourceType *string, page, limit int) ([]*models.AuditLog, int, error) {
	offset := (page - 1) * limit

	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIndex := 1

	if orgID != nil {
		whereClause += fmt.Sprintf(" AND org_id = $%d", argIndex)
		args = append(args, *orgID)
		argIndex++
	}

	if userID != nil {
		whereClause += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, *userID)
		argIndex++
	}

	if action != nil && *action != "" {
		whereClause += fmt.Sprintf(" AND action = $%d", argIndex)
		args = append(args, *action)
		argIndex++
	}

	if resourceType != nil && *resourceType != "" {
		whereClause += fmt.Sprintf(" AND resource_type = $%d", argIndex)
		args = append(args, *resourceType)
		argIndex++
	}

	// Count total records
	countQuery := "SELECT COUNT(*) FROM audit_logs " + whereClause
	var total int
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to count audit logs", errors.ErrInternalServer.Status)
	}

	// Get paginated results
	limitArg := argIndex
	offsetArg := argIndex + 1
	query := fmt.Sprintf(`
		SELECT id, user_id, org_id, ip_address, user_agent, action, 
			resource_type, resource_id, status, metadata, error_message, created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, limitArg, offsetArg)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to list audit logs", errors.ErrInternalServer.Status)
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var metadataJSON []byte
		var ipAddressStr *string
		var userAgentStr *string

		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.OrgID,
			&ipAddressStr,
			&userAgentStr,
			&log.Action,
			&log.ResourceType,
			&log.ResourceID,
			&log.Status,
			&metadataJSON,
			&log.ErrorMessage,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to scan audit log", errors.ErrInternalServer.Status)
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
				log.Metadata = make(map[string]interface{})
			}
		} else {
			log.Metadata = make(map[string]interface{})
		}

		log.IPAddress = ipAddressStr
		log.UserAgent = userAgentStr

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, errors.WrapError(err, "INTERNAL_ERROR", "Failed to iterate audit logs", errors.ErrInternalServer.Status)
	}

	return logs, total, nil
}

// Create creates a new audit log entry
func (r *AuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			user_id, org_id, ip_address, user_agent, action, 
			resource_type, resource_id, status, metadata, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`

	var metadataJSON []byte
	var err error
	if log.Metadata != nil {
		metadataJSON, err = json.Marshal(log.Metadata)
		if err != nil {
			return errors.WrapError(err, "INTERNAL_ERROR", "Failed to marshal metadata", errors.ErrInternalServer.Status)
		}
	}

	err = r.db.Pool.QueryRow(ctx, query,
		log.UserID,
		log.OrgID,
		log.IPAddress,
		log.UserAgent,
		log.Action,
		log.ResourceType,
		log.ResourceID,
		log.Status,
		metadataJSON,
		log.ErrorMessage,
	).Scan(&log.ID, &log.CreatedAt)

	if err != nil {
		return errors.WrapError(err, "INTERNAL_ERROR", "Failed to create audit log", errors.ErrInternalServer.Status)
	}

	return nil
}

// GetByID retrieves an audit log by ID
func (r *AuditLogRepository) GetByID(ctx context.Context, id int64) (*models.AuditLog, error) {
	query := `
		SELECT id, user_id, org_id, ip_address, user_agent, action, 
			resource_type, resource_id, status, metadata, error_message, created_at
		FROM audit_logs
		WHERE id = $1
	`

	log := &models.AuditLog{}
	var metadataJSON []byte
	var ipAddressStr *string
	var userAgentStr *string

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&log.ID,
		&log.UserID,
		&log.OrgID,
		&ipAddressStr,
		&userAgentStr,
		&log.Action,
		&log.ResourceType,
		&log.ResourceID,
		&log.Status,
		&metadataJSON,
		&log.ErrorMessage,
		&log.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, errors.ErrNotFound
	}
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to get audit log", errors.ErrInternalServer.Status)
	}

	// Parse metadata JSON
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
			log.Metadata = make(map[string]interface{})
		}
	} else {
		log.Metadata = make(map[string]interface{})
	}

	log.IPAddress = ipAddressStr
	log.UserAgent = userAgentStr

	return log, nil
}
