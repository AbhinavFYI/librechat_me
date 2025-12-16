package handlers

import (
	"fmt"
	"log"
	"net/http"

	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoleHandler struct {
	roleRepo *repositories.RoleRepository
}

func NewRoleHandler(roleRepo *repositories.RoleRepository) *RoleHandler {
	return &RoleHandler{roleRepo: roleRepo}
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get org_id from context or request
	orgID, _ := c.Get("org_id")
	isSuperAdmin, _ := c.Get("is_super_admin")

	var orgIDPtr *uuid.UUID
	var roleType string

	// Super admin can specify org_id in request, or create system role (org_id = NULL)
	if isSuperAdmin != nil && isSuperAdmin.(bool) {
		if req.OrgID != nil {
			// Super admin specified org_id in request
			orgIDPtr = req.OrgID
			roleType = "org_defined"
		} else if orgID != nil && orgID != "" {
			// Super admin has org context, use it
			uid, _ := uuid.Parse(orgID.(string))
			orgIDPtr = &uid
			roleType = "org_defined"
		} else {
			// Super admin creating system role (no org_id)
			orgIDPtr = nil
			roleType = "system"
		}
	} else {
		// Org admin or regular user - must use their own org
		if orgID == nil || orgID == "" {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Organization context required",
			})
			return
		}
		uid, _ := uuid.Parse(orgID.(string))
		orgIDPtr = &uid
		roleType = "org_defined"
	}

	userID, _ := c.Get("user_id")
	createdBy, _ := uuid.Parse(userID.(string))

	role := &models.Role{
		ID:          uuid.New(),
		OrgID:       orgIDPtr,
		Name:        req.Name,
		Type:        roleType,
		Description: req.Description,
		IsDefault:   false,
		CreatedBy:   &createdBy,
	}

	if req.IsDefault != nil {
		role.IsDefault = *req.IsDefault
	}

	if err := h.roleRepo.Create(c.Request.Context(), role); err != nil {
		log.Printf("Failed to create role: %v", err)
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create role: %v", err),
		})
		return
	}

	// Assign permissions if provided
	if len(req.PermissionIDs) > 0 {
		if err := h.roleRepo.AssignPermissions(c.Request.Context(), role.ID, req.PermissionIDs, createdBy); err != nil {
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: "Failed to assign permissions to role",
			})
			return
		}
	}

	c.JSON(http.StatusCreated, role)
}

func (h *RoleHandler) List(c *gin.Context) {
	orgID, _ := c.Get("org_id")
	isSuperAdmin, _ := c.Get("is_super_admin")

	var orgIDPtr *uuid.UUID
	if orgID != nil && orgID != "" {
		uid, _ := uuid.Parse(orgID.(string))
		orgIDPtr = &uid
	} else if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		orgIDPtr = nil // Only super admins can see system roles
	}

	roles, err := h.roleRepo.List(c.Request.Context(), orgIDPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list roles",
		})
		return
	}

	c.JSON(http.StatusOK, roles)
}

func (h *RoleHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get role",
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	var req models.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get role",
		})
		return
	}

	// Check if role is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if role.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update roles outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if role.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update roles outside your organization",
			})
			return
		}
	}

	// Update fields
	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Description != nil {
		role.Description = req.Description
	}
	if req.IsDefault != nil {
		role.IsDefault = *req.IsDefault
	}

	if err := h.roleRepo.Update(c.Request.Context(), role); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update role",
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	// Check if role is in same org (unless super admin)
	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err == nil {
		isSuperAdmin, _ := c.Get("is_super_admin")
		userOrgID, _ := c.Get("org_id")
		if isSuperAdmin == nil || !isSuperAdmin.(bool) {
			if role.OrgID == nil || userOrgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot delete roles outside your organization",
				})
				return
			}
			uid, _ := uuid.Parse(userOrgID.(string))
			if role.OrgID.String() != uid.String() {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot delete roles outside your organization",
				})
				return
			}
		}
	}

	if err := h.roleRepo.Delete(c.Request.Context(), id); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role deleted successfully"})
}

func (h *RoleHandler) GetPermissions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	permissions, err := h.roleRepo.GetPermissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get role permissions",
		})
		return
	}

	// Ensure we always return an array, never null
	if permissions == nil {
		permissions = make([]*models.Permission, 0)
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	var req struct {
		PermissionIDs []uuid.UUID `json:"permission_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Check if role is in same org (unless super admin)
	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err == nil {
		isSuperAdmin, _ := c.Get("is_super_admin")
		userOrgID, _ := c.Get("org_id")
		if isSuperAdmin == nil || !isSuperAdmin.(bool) {
			if role.OrgID == nil || userOrgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot assign permissions to roles outside your organization",
				})
				return
			}
			uid, _ := uuid.Parse(userOrgID.(string))
			if role.OrgID.String() != uid.String() {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot assign permissions to roles outside your organization",
				})
				return
			}
		}
	}

	userID, _ := c.Get("user_id")
	grantedBy, _ := uuid.Parse(userID.(string))

	if err := h.roleRepo.AssignPermissions(c.Request.Context(), id, req.PermissionIDs, grantedBy); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to assign permissions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permissions assigned successfully"})
}
