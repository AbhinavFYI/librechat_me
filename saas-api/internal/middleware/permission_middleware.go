package middleware

import (
	"net/http"

	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PermissionMiddleware struct {
	userRepo *repositories.UserRepository
}

func NewPermissionMiddleware(userRepo *repositories.UserRepository) *PermissionMiddleware {
	return &PermissionMiddleware{
		userRepo: userRepo,
	}
}

// RequirePermission checks if the authenticated user has a specific permission
func (m *PermissionMiddleware) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
				Error:   errors.ErrUnauthorized.Code,
				Message: "User not authenticated",
			})
			c.Abort()
			return
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
				Error:   errors.ErrUnauthorized.Code,
				Message: "Invalid user ID",
			})
			c.Abort()
			return
		}

		// Super admins bypass permission checks
		isSuperAdmin, _ := c.Get("is_super_admin")
		if isSuperAdmin != nil && isSuperAdmin.(bool) {
			c.Next()
			return
		}

		hasPermission, err := m.userRepo.HasPermission(c.Request.Context(), userID, resource, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: "Failed to check permissions",
			})
			c.Abort()
			return
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have permission to perform this action",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSameOrg ensures the user can only access resources in their own org
func (m *PermissionMiddleware) RequireSameOrg() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Super admins bypass org checks
		isSuperAdmin, _ := c.Get("is_super_admin")
		if isSuperAdmin != nil && isSuperAdmin.(bool) {
			c.Next()
			return
		}

		userOrgID, exists := c.Get("org_id")
		if !exists || userOrgID == nil || userOrgID == "" {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Organization context required",
			})
			c.Abort()
			return
		}

		// Store org_id for handlers to use
		c.Set("required_org_id", userOrgID)
		c.Next()
	}
}
