package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"saas-api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	userRepo *repositories.UserRepository
	roleRepo *repositories.RoleRepository
	orgRepo  *repositories.OrganizationRepository
}

func NewUserHandler(userRepo *repositories.UserRepository, roleRepo *repositories.RoleRepository, orgRepo *repositories.OrganizationRepository) *UserHandler {
	return &UserHandler{
		userRepo: userRepo,
		roleRepo: roleRepo,
		orgRepo:  orgRepo,
	}
}

func (h *UserHandler) Create(c *gin.Context) {
	var req models.CreateUserRequest
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

	// If not super admin, must use their own org
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if orgID == nil || orgID == "" {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Organization context required",
			})
			return
		}
		uid, _ := uuid.Parse(orgID.(string))
		req.OrgID = &uid
	} else {
		// Super admin must specify org by id or name (they may not have their own org)
		if req.OrgID == nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Organization must be specified. Provide 'org_id' in the request",
			})
			return
		}
		// Verify the organization exists
		org, err := h.orgRepo.GetByID(c.Request.Context(), *req.OrgID)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: fmt.Sprintf("Organization with id '%s' not found", *req.OrgID),
			})
			return
		}
		// Use the verified org ID
		req.OrgID = &org.ID
	}

	// Hash password
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to hash password",
		})
		return
	}

	// Set org_role (default to "user" if not specified)
	orgRole := "user"
	if req.OrgRole != nil && *req.OrgRole != "" {
		// Validate org_role value
		validOrgRoles := map[string]bool{"admin": true, "user": true, "viewer": true}
		if validOrgRoles[*req.OrgRole] {
			orgRole = *req.OrgRole
		} else {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid org_role. Must be one of: admin, user, viewer",
			})
			return
		}
	}

	user := &models.User{
		ID:            uuid.New(),
		OrgID:         req.OrgID,
		Email:         req.Email,
		PasswordHash:  passwordHash,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Phone:         req.Phone,
		IsSuperAdmin:  false,
		OrgRole:       &orgRole,
		Status:        "pending",
		EmailVerified: false,
		Timezone:      "UTC",
		Locale:        "en-US",
		Metadata:      make(map[string]interface{}),
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create user: %v", err),
		})
		return
	}

	// Assign role if provided (by ID or name)
	var roleToAssign *uuid.UUID
	currentUserID, _ := c.Get("user_id")
	assignedBy, _ := uuid.Parse(currentUserID.(string))

	if req.RoleID != nil {
		// Role specified by UUID
		roleToAssign = req.RoleID
	} else if req.RoleName != nil && *req.RoleName != "" {
		// Role specified by name - look it up
		var orgIDForLookup *uuid.UUID
		if user.OrgID != nil {
			orgIDForLookup = user.OrgID
		} else if orgID != nil && orgID != "" {
			uid, _ := uuid.Parse(orgID.(string))
			orgIDForLookup = &uid
		}

		role, err := h.roleRepo.GetByName(c.Request.Context(), *req.RoleName, orgIDForLookup)
		if err == nil {
			roleToAssign = &role.ID
		} else {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: fmt.Sprintf("Role '%s' not found in organization", *req.RoleName),
			})
			return
		}
	}

	// Assign the role if we have one
	if roleToAssign != nil {
		role, err := h.roleRepo.GetByID(c.Request.Context(), *roleToAssign)
		if err != nil {
			log.Printf("Failed to get role by ID %s: %v", *roleToAssign, err)
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: fmt.Sprintf("Role not found: %v", err),
			})
			return
		}

		if isSuperAdmin == nil || !isSuperAdmin.(bool) {
			// Non-super admin: verify role belongs to same org
			if role.OrgID == nil || orgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot assign system roles",
				})
				return
			} else {
				uid, _ := uuid.Parse(orgID.(string))
				if role.OrgID.String() == uid.String() {
					if err := h.roleRepo.AssignRoleToUser(c.Request.Context(), user.ID, *roleToAssign, assignedBy, nil); err != nil {
						log.Printf("Failed to assign role to user: %v", err)
						c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
							Error:   errors.ErrInternalServer.Code,
							Message: fmt.Sprintf("Failed to assign role: %v", err),
						})
						return
					}
				} else {
					c.JSON(http.StatusForbidden, errors.ErrorResponse{
						Error:   errors.ErrForbidden.Code,
						Message: "Cannot assign roles from other organizations",
					})
					return
				}
			}
		} else {
			// Super admin can assign any role
			if err := h.roleRepo.AssignRoleToUser(c.Request.Context(), user.ID, *roleToAssign, assignedBy, nil); err != nil {
				log.Printf("Failed to assign role to user: %v", err)
				c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
					Error:   errors.ErrInternalServer.Code,
					Message: fmt.Sprintf("Failed to assign role: %v", err),
				})
				return
			}
		}
	} else if user.OrgID != nil {
		// If no role specified, assign default role for the org
		roles, err := h.roleRepo.List(c.Request.Context(), user.OrgID)
		if err == nil {
			for _, role := range roles {
				if role.IsDefault {
					currentUserID, _ := c.Get("user_id")
					assignedBy, _ := uuid.Parse(currentUserID.(string))
					if err := h.roleRepo.AssignRoleToUser(c.Request.Context(), user.ID, role.ID, assignedBy, nil); err != nil {
						log.Printf("Failed to assign default role to user: %v", err)
						// Don't fail user creation if default role assignment fails, just log it
					}
					break
				}
			}
		}
	}

	user.PasswordHash = ""
	c.JSON(http.StatusCreated, user)
}

func (h *UserHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get user",
		})
		return
	}

	// Fetch roles for the user
	roles, err := h.userRepo.GetUserRoles(c.Request.Context(), user.ID)
	if err == nil {
		user.Roles = make([]models.Role, len(roles))
		for i, role := range roles {
			user.Roles[i] = *role
		}
	}

	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var orgID *uuid.UUID
	isSuperAdmin, _ := c.Get("is_super_admin")

	// Super admin can filter by org_id via query parameter, or see all users (orgID = nil)
	// Org admin sees only their org's users
	if isSuperAdmin != nil && isSuperAdmin.(bool) {
		// Super admin: check for org_id query parameter for filtering
		if orgIDParam := c.Query("org_id"); orgIDParam != "" {
			uid, err := uuid.Parse(orgIDParam)
			if err == nil {
				orgID = &uid
				log.Printf("Filtering users by org_id: %s", orgID.String())
			} else {
				log.Printf("Failed to parse org_id query parameter: %s, error: %v", orgIDParam, err)
			}
		} else {
			log.Printf("No org_id query parameter provided, returning all users")
		}
		// If no org_id param, orgID remains nil to get all users
	} else {
		// Org admin: use their own org
		if orgIDStr, exists := c.Get("org_id"); exists && orgIDStr != nil && orgIDStr != "" {
			uid, _ := uuid.Parse(orgIDStr.(string))
			orgID = &uid
		}
	}

	log.Printf("Calling userRepo.List with orgID: %v, page: %d, limit: %d", orgID, page, limit)
	users, total, err := h.userRepo.List(c.Request.Context(), orgID, page, limit)
	if err != nil {
		log.Printf("Error listing users: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list users",
		})
		return
	}
	log.Printf("Returning %d users (total: %d) for orgID: %v", len(users), total, orgID)

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       users,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get user",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if user.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update users outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if user.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot update users outside your organization",
			})
			return
		}
	}

	// Update fields
	if req.FirstName != nil {
		user.FirstName = req.FirstName
	}
	if req.LastName != nil {
		user.LastName = req.LastName
	}
	if req.Phone != nil {
		user.Phone = req.Phone
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	if req.Status != nil {
		// Validate status value - must match database enum: active, suspended, pending, deleted
		validStatuses := map[string]bool{"active": true, "suspended": true, "pending": true, "deleted": true}
		if validStatuses[*req.Status] {
			user.Status = *req.Status
		} else {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid status. Must be one of: active, pending, suspended, deleted",
			})
			return
		}
	}
	if req.OrgRole != nil {
		// Validate org_role value
		validOrgRoles := map[string]bool{"admin": true, "user": true, "viewer": true}
		if validOrgRoles[*req.OrgRole] {
			user.OrgRole = req.OrgRole
		} else {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid org_role. Must be one of: admin, user, viewer",
			})
			return
		}
	}
	if req.EmailVerified != nil {
		user.EmailVerified = *req.EmailVerified
		// Set EmailVerifiedAt timestamp when email is verified
		if *req.EmailVerified {
			if user.EmailVerifiedAt == nil {
				now := time.Now()
				user.EmailVerifiedAt = &now
			}
			// If already verified, keep existing timestamp
		} else {
			// Clear EmailVerifiedAt when email is unverified
			user.EmailVerifiedAt = nil
		}
	}
	if req.Timezone != nil {
		user.Timezone = *req.Timezone
	}
	if req.Locale != nil {
		user.Locale = *req.Locale
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update user",
		})
		return
	}

	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err == nil {
		isSuperAdmin, _ := c.Get("is_super_admin")
		userOrgID, _ := c.Get("org_id")
		if isSuperAdmin == nil || !isSuperAdmin.(bool) {
			if user.OrgID == nil || userOrgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot delete users outside your organization",
				})
				return
			}
			uid, _ := uuid.Parse(userOrgID.(string))
			if user.OrgID.String() != uid.String() {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot delete users outside your organization",
				})
				return
			}
		}
	}

	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandler) GetPermissions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	permissions, err := h.userRepo.GetUserPermissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get user permissions",
		})
		return
	}

	// Ensure we always return an array, never null
	if permissions == nil {
		permissions = make([]*models.Permission, 0)
	}

	c.JSON(http.StatusOK, permissions)
}

// AssignRole assigns a role to a user
func (h *UserHandler) AssignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	var req models.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Check if user is in same org (unless super admin)
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
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
			Message: "Failed to get user",
		})
		return
	}

	isSuperAdmin, _ := c.Get("is_super_admin")
	userOrgID, _ := c.Get("org_id")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		if user.OrgID == nil || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot assign roles to users outside your organization",
			})
			return
		}
		uid, _ := uuid.Parse(userOrgID.(string))
		if user.OrgID.String() != uid.String() {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Cannot assign roles to users outside your organization",
			})
			return
		}
	}

	// Get current user ID
	currentUserID, _ := c.Get("user_id")
	assignedBy, _ := uuid.Parse(currentUserID.(string))

	// Remove all existing roles for this user first (replace, not add)
	existingRoles, err := h.userRepo.GetUserRoles(c.Request.Context(), userID)
	if err == nil {
		for _, role := range existingRoles {
			if err := h.roleRepo.RemoveRoleFromUser(c.Request.Context(), userID, role.ID); err != nil {
				log.Printf("Warning: Failed to remove existing role %s from user: %v", role.ID, err)
			}
		}
	}

	// Assign the new role
	if err := h.roleRepo.AssignRoleToUser(c.Request.Context(), userID, req.RoleID, assignedBy, req.ExpiresAt); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to assign role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully"})
}

// RemoveRole removes a role from a user
func (h *UserHandler) RemoveRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	// Check if user is in same org (unless super admin)
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err == nil {
		isSuperAdmin, _ := c.Get("is_super_admin")
		userOrgID, _ := c.Get("org_id")
		if isSuperAdmin == nil || !isSuperAdmin.(bool) {
			if user.OrgID == nil || userOrgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot remove roles from users outside your organization",
				})
				return
			}
			uid, _ := uuid.Parse(userOrgID.(string))
			if user.OrgID.String() != uid.String() {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "Cannot remove roles from users outside your organization",
				})
				return
			}
		}
	}

	if err := h.roleRepo.RemoveRoleFromUser(c.Request.Context(), userID, roleID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to remove role",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role removed successfully"})
}
