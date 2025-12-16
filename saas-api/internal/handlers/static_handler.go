package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"saas-api/pkg/errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StaticHandler struct {
	storagePath string
}

func NewStaticHandler(storagePath string) *StaticHandler {
	return &StaticHandler{
		storagePath: storagePath,
	}
}

// ServeFile serves files directly from the storage path
// Route: /static/resources/folder/file/*
// Example: /static/resources/folder/file/org_id/folder_path/file_name.pdf
// Or for super admin: /static/resources/folder/file/folder_name/file_name.pdf
func (h *StaticHandler) ServeFile(c *gin.Context) {
	// Get the file path from URL
	// URL pattern: /static/resources/folder/file/{path}
	// Gin's *path includes the leading slash, so we need to remove it
	requestPath := c.Param("path")
	if requestPath == "" {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "File path is required",
		})
		return
	}

	// Remove leading slash if present (Gin's *path includes it)
	requestPath = strings.TrimPrefix(requestPath, "/")

	// Clean the path to prevent directory traversal attacks
	requestPath = filepath.Clean(requestPath)
	if strings.Contains(requestPath, "..") {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid file path",
		})
		return
	}

	// Get user's org_id and super admin status for access validation
	userOrgID, _ := c.Get("org_id")
	isSuperAdmin, _ := c.Get("is_super_admin")
	isSuperAdminBool := isSuperAdmin != nil && isSuperAdmin.(bool)

	// Construct file path on disk
	// Path format: {storagePath}/{requestPath}
	fullPath := filepath.Join(h.storagePath, requestPath)

	// Check if file exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: fmt.Sprintf("File not found: %s", requestPath),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to access file",
		})
		return
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Path is a directory, not a file",
		})
		return
	}

	// Validate access based on org_id in path
	// Extract org_id from path (first segment)
	pathParts := strings.Split(requestPath, "/")
	var pathOrgUUID *uuid.UUID

	if len(pathParts) > 0 {
		firstSegment := pathParts[0]
		if parsedUUID, err := uuid.Parse(firstSegment); err == nil {
			pathOrgUUID = &parsedUUID
		}
	}

	// Validate access
	if pathOrgUUID != nil {
		// Path contains org_id - validate user has access to this org
		if !isSuperAdminBool {
			// Regular user - must belong to the same org
			if userOrgID == nil {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "You do not have access to this file",
				})
				return
			}

			userOrgIDStr, ok := userOrgID.(string)
			if !ok {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "You do not have access to this file",
				})
				return
			}

			userOrgUUID, err := uuid.Parse(userOrgIDStr)
			if err != nil || userOrgUUID != *pathOrgUUID {
				c.JSON(http.StatusForbidden, errors.ErrorResponse{
					Error:   errors.ErrForbidden.Code,
					Message: "You do not have access to this file. Files can only be accessed by users from the same organization.",
				})
				return
			}
		}
		// Super admin can access any org's files
	} else {
		// Path doesn't contain org_id (legacy format like "LLAMA/Whatsapp/login2.png")
		// Only allow super admin access
		if !isSuperAdminBool {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this file",
			})
			return
		}
	}

	// Access validated, determine MIME type and serve the file
	ext := filepath.Ext(fullPath)
	mimeType := getMimeType(strings.ToLower(strings.TrimPrefix(ext, ".")))

	// Set appropriate headers
	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(fullPath)))
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Serve the file
	c.File(fullPath)
}
