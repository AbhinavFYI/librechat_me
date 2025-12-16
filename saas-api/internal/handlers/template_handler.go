package handlers

import (
	"log"
	"net/http"
	"strconv"

	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TemplateHandler struct {
	templateRepo *repositories.TemplateRepository
}

func NewTemplateHandler(templateRepo *repositories.TemplateRepository) *TemplateHandler {
	return &TemplateHandler{
		templateRepo: templateRepo,
	}
}

func (h *TemplateHandler) Create(c *gin.Context) {
	var req models.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get org_id from context
	orgID, _ := c.Get("org_id")
	isSuperAdmin, _ := c.Get("is_super_admin")
	userID, _ := c.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	var orgIDPtr *uuid.UUID
	if orgID != nil && orgID != "" {
		orgUUID, _ := uuid.Parse(orgID.(string))
		orgIDPtr = &orgUUID
	} else if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		c.JSON(http.StatusForbidden, errors.ErrorResponse{
			Error:   errors.ErrForbidden.Code,
			Message: "Organization context required",
		})
		return
	}

	isCustom := false
	if req.IsCustom != nil {
		isCustom = *req.IsCustom
	}

	if req.Content == nil {
		req.Content = make(map[string]interface{})
	}

	template := &models.Template{
		ID:          uuid.New(),
		OrgID:       orgIDPtr,
		Name:        req.Name,
		Description: req.Description,
		Framework:   req.Framework,
		IsCustom:    isCustom,
		Content:     req.Content,
		CreatedBy:   &uid,
	}

	if err := h.templateRepo.Create(c.Request.Context(), template); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to create template: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create template",
		})
		return
	}

	// Fetch with joined data
	template, err := h.templateRepo.GetByID(c.Request.Context(), template.ID)
	if err != nil {
		c.JSON(http.StatusOK, template) // Return without joined data if fetch fails
		return
	}

	c.JSON(http.StatusCreated, template)
}

func (h *TemplateHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var orgID *uuid.UUID
	isSuperAdmin, _ := c.Get("is_super_admin")

	// Super admin sees all templates (orgID = nil)
	// Org admin sees only their org's templates
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if orgIDStr, exists := c.Get("org_id"); exists && orgIDStr != nil && orgIDStr != "" {
			uid, _ := uuid.Parse(orgIDStr.(string))
			orgID = &uid
		}
	}

	templates, total, err := h.templateRepo.List(c.Request.Context(), orgID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list templates",
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       templates,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (h *TemplateHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid template ID",
		})
		return
	}

	template, err := h.templateRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get template",
		})
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *TemplateHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid template ID",
		})
		return
	}

	var req models.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get existing template
	template, err := h.templateRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get template",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if template.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update templates outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if template.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update templates outside your organization",
			})
			return
		}
	}

	// Update fields
	if req.Name != nil {
		template.Name = *req.Name
	}
	if req.Description != nil {
		template.Description = req.Description
	}
	if req.Framework != nil {
		template.Framework = req.Framework
	}
	if req.IsCustom != nil {
		template.IsCustom = *req.IsCustom
	}
	if req.Content != nil {
		template.Content = req.Content
	}

	userID, _ := c.Get("user_id")
	updatedBy, _ := uuid.Parse(userID.(string))
	template.UpdatedBy = &updatedBy

	if err := h.templateRepo.Update(c.Request.Context(), template); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update template",
		})
		return
	}

	// Fetch with joined data
	template, err = h.templateRepo.GetByID(c.Request.Context(), template.ID)
	if err != nil {
		c.JSON(http.StatusOK, template) // Return without joined data if fetch fails
		return
	}

	c.JSON(http.StatusOK, template)
}

func (h *TemplateHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid template ID",
		})
		return
	}

	// Get existing template to check org
	template, err := h.templateRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get template",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if template.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot delete templates outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if template.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot delete templates outside your organization",
			})
			return
		}
	}

	userID, _ := c.Get("user_id")
	deletedBy, _ := uuid.Parse(userID.(string))

	if err := h.templateRepo.Delete(c.Request.Context(), id, deletedBy); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete template",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}
