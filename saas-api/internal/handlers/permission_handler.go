package handlers

import (
	"net/http"

	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permRepo *repositories.PermissionRepository
}

func NewPermissionHandler(permRepo *repositories.PermissionRepository) *PermissionHandler {
	return &PermissionHandler{permRepo: permRepo}
}

func (h *PermissionHandler) List(c *gin.Context) {
	permissions, err := h.permRepo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list permissions",
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}
