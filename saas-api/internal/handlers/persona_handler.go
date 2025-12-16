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

type PersonaHandler struct {
	personaRepo *repositories.PersonaRepository
}

func NewPersonaHandler(personaRepo *repositories.PersonaRepository) *PersonaHandler {
	return &PersonaHandler{
		personaRepo: personaRepo,
	}
}

func (h *PersonaHandler) Create(c *gin.Context) {
	var req models.CreatePersonaRequest
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

	isCustomTemplate := false
	if req.IsCustomTemplate != nil {
		isCustomTemplate = *req.IsCustomTemplate
	}

	if req.Content == nil {
		req.Content = make(map[string]interface{})
	}

	persona := &models.Persona{
		ID:               uuid.New(),
		OrgID:            orgIDPtr,
		TemplateID:       req.TemplateID,
		Name:             req.Name,
		Description:      req.Description,
		Content:          req.Content,
		IsCustomTemplate: isCustomTemplate,
		CreatedBy:        &uid,
	}

	if err := h.personaRepo.Create(c.Request.Context(), persona); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to create persona: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create persona",
		})
		return
	}

	// Fetch with joined data
	persona, err := h.personaRepo.GetByID(c.Request.Context(), persona.ID)
	if err != nil {
		c.JSON(http.StatusOK, persona) // Return without joined data if fetch fails
		return
	}

	c.JSON(http.StatusCreated, persona)
}

func (h *PersonaHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var orgID *uuid.UUID
	isSuperAdmin, _ := c.Get("is_super_admin")

	// Super admin sees all personas (orgID = nil)
	// Org admin sees only their org's personas
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if orgIDStr, exists := c.Get("org_id"); exists && orgIDStr != nil && orgIDStr != "" {
			uid, _ := uuid.Parse(orgIDStr.(string))
			orgID = &uid
		}
	}

	personas, total, err := h.personaRepo.List(c.Request.Context(), orgID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list personas",
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       personas,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (h *PersonaHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid persona ID",
		})
		return
	}

	persona, err := h.personaRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get persona",
		})
		return
	}

	c.JSON(http.StatusOK, persona)
}

func (h *PersonaHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid persona ID",
		})
		return
	}

	var req models.UpdatePersonaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get existing persona
	persona, err := h.personaRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get persona",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if persona.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update personas outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if persona.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update personas outside your organization",
			})
			return
		}
	}

	// Update fields
	if req.Name != nil {
		persona.Name = *req.Name
	}
	if req.Description != nil {
		persona.Description = req.Description
	}
	if req.TemplateID != nil {
		persona.TemplateID = req.TemplateID
	}
	if req.Content != nil {
		persona.Content = req.Content
	}
	if req.IsCustomTemplate != nil {
		persona.IsCustomTemplate = *req.IsCustomTemplate
	}

	userID, _ := c.Get("user_id")
	updatedBy, _ := uuid.Parse(userID.(string))
	persona.UpdatedBy = &updatedBy

	if err := h.personaRepo.Update(c.Request.Context(), persona); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update persona",
		})
		return
	}

	// Fetch with joined data
	persona, err = h.personaRepo.GetByID(c.Request.Context(), persona.ID)
	if err != nil {
		c.JSON(http.StatusOK, persona) // Return without joined data if fetch fails
		return
	}

	c.JSON(http.StatusOK, persona)
}

func (h *PersonaHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid persona ID",
		})
		return
	}

	// Get existing persona to check org
	persona, err := h.personaRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get persona",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if persona.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot delete personas outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if persona.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot delete personas outside your organization",
			})
			return
		}
	}

	userID, _ := c.Get("user_id")
	deletedBy, _ := uuid.Parse(userID.(string))

	if err := h.personaRepo.Delete(c.Request.Context(), id, deletedBy); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete persona",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Persona deleted successfully"})
}
