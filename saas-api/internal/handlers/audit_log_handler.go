package handlers

import (
	"net/http"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuditLogHandler struct {
	auditLogRepo *repositories.AuditLogRepository
}

func NewAuditLogHandler(auditLogRepo *repositories.AuditLogRepository) *AuditLogHandler {
	return &AuditLogHandler{
		auditLogRepo: auditLogRepo,
	}
}

// List retrieves audit logs with pagination and optional filters
func (h *AuditLogHandler) List(c *gin.Context) {
	// Get pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// Get optional filters
	var orgID *uuid.UUID
	var userID *uuid.UUID
	var action *string
	var resourceType *string

	// Get org_id from query or context
	orgIDStr := c.Query("org_id")
	if orgIDStr == "" {
		// Try to get from context (for regular users)
		orgIDFromCtx, exists := c.Get("org_id")
		if exists && orgIDFromCtx != nil {
			orgIDStr = orgIDFromCtx.(string)
		}
	}
	if orgIDStr != "" {
		parsed, err := uuid.Parse(orgIDStr)
		if err == nil {
			orgID = &parsed
		}
	}

	// Get user_id from query
	userIDStr := c.Query("user_id")
	if userIDStr != "" {
		parsed, err := uuid.Parse(userIDStr)
		if err == nil {
			userID = &parsed
		}
	}

	// Get action filter
	actionStr := c.Query("action")
	if actionStr != "" {
		action = &actionStr
	}

	// Get resource_type filter
	resourceTypeStr := c.Query("resource_type")
	if resourceTypeStr != "" {
		resourceType = &resourceTypeStr
	}

	// Get audit logs
	logs, total, err := h.auditLogRepo.List(c.Request.Context(), orgID, userID, action, resourceType, page, limit)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to retrieve audit logs",
		})
		return
	}

	// Calculate total pages
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"data":        logs,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

// GetByID retrieves a specific audit log by ID
func (h *AuditLogHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid audit log ID",
		})
		return
	}

	log, err := h.auditLogRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to retrieve audit log",
		})
		return
	}

	c.JSON(http.StatusOK, log)
}
