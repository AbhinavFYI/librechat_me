package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FileHandler struct {
	folderRepo      *repositories.FolderRepository
	documentRepo    *repositories.DocumentRepository
	documentService interface {
		DeleteDocument(ctx context.Context, documentID int64) error
	}
	storagePath string // Base path for file storage: {storagePath}/{org_id}/folder/files
}

func NewFileHandler(folderRepo *repositories.FolderRepository, documentRepo *repositories.DocumentRepository, documentService interface {
	DeleteDocument(ctx context.Context, documentID int64) error
}, storagePath string) *FileHandler {
	return &FileHandler{
		folderRepo:      folderRepo,
		documentRepo:    documentRepo,
		documentService: documentService,
		storagePath:     storagePath,
	}
}

// Helper function to convert Document to File model for API compatibility
func documentToFile(doc *repositories.Document) *models.File {
	// Use document ID directly as int64 (no UUID conversion)
	// Handle nullable OrgID
	var orgID uuid.UUID
	if doc.OrgID != nil {
		orgID = *doc.OrgID
	}

	file := &models.File{
		ID:            doc.ID, // Use int64 directly
		OrgID:         orgID,
		FolderID:      doc.FolderID,
		Name:          doc.Name,
		StorageKey:    "",
		Version:       1,
		CreatedBy:     doc.CreatedBy,
		UpdatedBy:     doc.UpdatedBy,
		CreatedAt:     doc.CreatedAt,
		UpdatedAt:     doc.UpdatedAt,
		CreatedByName: doc.CreatedByName,
		UpdatedByName: doc.UpdatedByName,
		FolderName:    doc.FolderName,
	}

	// Extract from content JSONB
	if doc.Content.MimeType != nil {
		file.MimeType = doc.Content.MimeType
	}
	if doc.Content.SizeBytes != nil {
		file.SizeBytes = doc.Content.SizeBytes
	}
	if doc.Content.Checksum != nil {
		file.Checksum = doc.Content.Checksum
	}
	if doc.Content.Version != nil {
		file.Version = *doc.Content.Version
	}

	// Extract extension from name
	if ext := filepath.Ext(doc.Name); ext != "" {
		extLower := strings.ToLower(ext[1:])
		file.Extension = &extLower
	}

	// Use file_path as storage_key
	if doc.FilePath != nil {
		file.StorageKey = *doc.FilePath
	} else if doc.Content.Path != nil {
		file.StorageKey = *doc.Content.Path
	}

	return file
}

func (h *FileHandler) Create(c *gin.Context) {
	var req models.CreateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get user and org from context
	userID, exists := c.Get("user_id")
	if !exists || userID == nil {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User ID is required",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID format",
		})
		return
	}

	uid, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	// Get org_id from request body or context
	var orgUUID uuid.UUID
	if req.OrgID != nil {
		orgUUID = *req.OrgID
	} else {
		orgID, exists := c.Get("org_id")
		if !exists || orgID == nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Organization ID is required. Provide 'org_id' in request body or ensure you are associated with an organization",
			})
			return
		}

		orgIDStr, ok := orgID.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID format",
			})
			return
		}

		var err error
		orgUUID, err = uuid.Parse(orgIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID",
			})
			return
		}
	}

	// Extract extension from filename
	ext := filepath.Ext(req.Name)
	if ext != "" {
		ext = strings.ToLower(ext[1:]) // Remove the dot
	}

	// Generate storage key: {org_id}/{folder_path}/{file_name} (relative to storage path, not including storage path prefix)
	var storageKey string
	if req.FolderID != nil {
		folder, err := h.folderRepo.GetByID(c.Request.Context(), *req.FolderID)
		if err == nil {
			// Strip leading slash from folder path if present (folder.Path is like "/root/team/reports")
			folderPath := strings.TrimPrefix(folder.Path, "/")
			storageKey = filepath.Join(orgUUID.String(), folderPath, req.Name)
		} else {
			// If folder not found, store in org root
			storageKey = filepath.Join(orgUUID.String(), req.Name)
		}
	} else {
		// No folder, store in org root
		storageKey = filepath.Join(orgUUID.String(), req.Name)
	}

	// Normalize path separators (use forward slashes for consistency)
	storageKey = filepath.Clean(storageKey)
	storageKey = strings.ReplaceAll(storageKey, "\\", "/")

	// Determine MIME type from extension
	mimeType := getMimeType(ext)

	// Create document (file) in documents table
	doc := &repositories.Document{
		// ID will be auto-generated by BIGSERIAL
		OrgID:     &orgUUID,
		FolderID:  req.FolderID,
		Name:      req.Name,
		FilePath:  &storageKey,
		Status:    repositories.DocumentStatusCompleted,
		CreatedBy: &uid,
		Content: repositories.DocumentContent{
			MimeType:  &mimeType,
			SizeBytes: new(int64), // Will be set to 0 initially
			Version:   new(int),
			IsFolder:  new(bool),
		},
		Metadata: make(map[string]interface{}),
	}
	*doc.Content.SizeBytes = 0
	*doc.Content.Version = 1
	*doc.Content.IsFolder = false

	if ext != "" {
		doc.Content.Path = &storageKey
	}

	if err := h.documentRepo.Create(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create file: %v", err),
		})
		return
	}

	// Convert to File model for API response
	file := documentToFile(doc)
	c.JSON(http.StatusCreated, file)
}

func (h *FileHandler) List(c *gin.Context) {
	var orgUUID uuid.UUID
	var err error

	// Check if org_id is in query params (for super admins)
	orgIDStr := c.Query("org_id")
	if orgIDStr != "" {
		orgUUID, err = uuid.Parse(orgIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID in query parameter",
			})
			return
		}
	} else {
		// Get org_id from context (for regular users)
		orgID, exists := c.Get("org_id")
		if !exists || orgID == nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Organization ID is required. Provide 'org_id' in query parameter or ensure you are associated with an organization",
			})
			return
		}

		orgIDStr, ok := orgID.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID format",
			})
			return
		}

		orgUUID, err = uuid.Parse(orgIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID",
			})
			return
		}
	}

	folderIDStr := c.Query("folder_id")
	var folderID *uuid.UUID
	if folderIDStr != "" {
		fid, err := uuid.Parse(folderIDStr)
		if err == nil {
			folderID = &fid
		}
	}

	page := 1
	limit := 1000
	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Convert folderID from *uuid.UUID to *uuid.UUID for document repo
	var docFolderID *uuid.UUID
	if folderID != nil {
		docFolderID = folderID
	}

	documents, total, err := h.documentRepo.ListByFolder(c.Request.Context(), docFolderID, orgUUID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to list files: %v", err),
		})
		return
	}

	fmt.Printf("📁 /api/v1/files returned %d files (total: %d, folderID: %v, orgID: %s)\n", len(documents), total, folderID, orgUUID)
	for _, doc := range documents {
		fmt.Printf("  - File ID: %d, Name: %s\n", doc.ID, doc.Name)
	}

	// Convert documents to files for API compatibility
	files := make([]*models.File, len(documents))
	for i, doc := range documents {
		files[i] = documentToFile(doc)
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       files,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (h *FileHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")

	// Try to parse as int64 first (new format)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		// Try to parse as UUID (legacy format or deterministic UUID)
		fileUUID, uuidErr := uuid.Parse(idStr)
		if uuidErr != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid file ID format",
			})
			return
		}

		// Extract document ID from deterministic UUID format
		// Format: 00000000-0000-0000-0000-{12 digit doc ID}
		uuidStr := fileUUID.String()
		if strings.HasPrefix(uuidStr, "00000000-0000-0000-0000-") {
			// Extract the last 12 digits
			docIDStr := strings.TrimPrefix(uuidStr, "00000000-0000-0000-0000-")
			id, err = strconv.ParseInt(docIDStr, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, errors.ErrorResponse{
					Error:   errors.ErrValidation.Code,
					Message: "Invalid document ID in UUID",
				})
				return
			}
		} else {
			// Random UUID - cannot find
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Cannot find file with random UUID. Please use document ID.",
			})
			return
		}
	}

	doc, err := h.documentRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ErrorResponse{
			Error:   errors.ErrNotFound.Code,
			Message: "File not found",
		})
		return
	}

	// Check organization access - users can only access files from their own organization
	// Super admins can access files from any organization
	isSuperAdmin, _ := c.Get("is_super_admin")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		// Regular user - check if file belongs to their organization
		userOrgID, exists := c.Get("org_id")
		if !exists || userOrgID == nil {
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
		if err != nil || (doc.OrgID != nil && userOrgUUID != *doc.OrgID) {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this file. Files can only be accessed by users from the same organization.",
			})
			return
		}
	}
	// Super admin - allow access to any file

	// Convert to File model
	file := documentToFile(doc)

	// Check if download/preview is requested (default to serving file)
	downloadParam := c.Query("download")
	// Always serve file if download parameter is present (true, 1, or empty means serve file)
	// Only return JSON if explicitly requested with download=false
	if downloadParam != "false" {
		// Determine the actual file path from document
		var filePath string
		if doc.FilePath != nil {
			filePath = *doc.FilePath
		} else if doc.Content.Path != nil {
			filePath = *doc.Content.Path
		} else {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "File path not found in document",
			})
			return
		}

		// Check if the path already includes storage path prefix
		if strings.HasPrefix(filePath, h.storagePath+"/") || strings.HasPrefix(filePath, h.storagePath+"\\") {
			// Path already includes storage path prefix (legacy format)
			filePath = filepath.Clean(filePath)
		} else if filepath.IsAbs(filePath) {
			// It's an absolute path - use as-is
			filePath = filepath.Clean(filePath)
		} else {
			// Relative path - join with storagePath
			filePath = filepath.Join(h.storagePath, filePath)
		}

		// Clean the path
		filePath = filepath.Clean(filePath)

		// Serve file for download
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: fmt.Sprintf("File not found on disk: %s", filePath),
			})
			return
		}

		// Set appropriate content type based on file extension
		// Prioritize extension-based detection to ensure correct MIME type
		mimeType := "application/octet-stream"
		if doc.Content.MimeType != nil && *doc.Content.MimeType != "" {
			mimeType = *doc.Content.MimeType
		} else if file.Extension != nil && *file.Extension != "" {
			mimeType = getMimeType(*file.Extension)
		}

		// Set headers for file download
		c.Header("Content-Type", mimeType)
		// For HTML and PDF files, use inline to allow preview; for others, use attachment to force download
		if strings.HasPrefix(mimeType, "text/html") || mimeType == "application/pdf" {
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", doc.Name))
		} else {
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", doc.Name))
		}
		// Serve the file
		c.File(filePath)
		return
	}

	c.JSON(http.StatusOK, file)
}

func (h *FileHandler) Update(c *gin.Context) {
	idStr := c.Param("id")

	// Try to parse as int64 first (new format)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		// Try to parse as UUID (legacy format or deterministic UUID)
		fileUUID, uuidErr := uuid.Parse(idStr)
		if uuidErr != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid file ID format",
			})
			return
		}

		// Extract document ID from deterministic UUID format
		// Format: 00000000-0000-0000-0000-{12 digit doc ID}
		uuidStr := fileUUID.String()
		if strings.HasPrefix(uuidStr, "00000000-0000-0000-0000-") {
			// Extract the last 12 digits
			docIDStr := strings.TrimPrefix(uuidStr, "00000000-0000-0000-0000-")
			id, err = strconv.ParseInt(docIDStr, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, errors.ErrorResponse{
					Error:   errors.ErrValidation.Code,
					Message: "Invalid document ID in UUID",
				})
				return
			}
		} else {
			// Random UUID - cannot update
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Cannot update file with random UUID. Please use document ID.",
			})
			return
		}
	}

	var req models.UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get existing document
	doc, err := h.documentRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ErrorResponse{
			Error:   errors.ErrNotFound.Code,
			Message: "File not found",
		})
		return
	}

	// Update fields
	if req.Name != nil {
		doc.Name = *req.Name
		// Update extension if name changed
		ext := filepath.Ext(*req.Name)
		if ext != "" {
			extLower := strings.ToLower(ext[1:])
			doc.Content.MimeType = &extLower
		}
	}
	if req.FolderID != nil {
		doc.FolderID = req.FolderID
	}

	userID, exists := c.Get("user_id")
	if exists && userID != nil {
		if userIDStr, ok := userID.(string); ok {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				doc.UpdatedBy = &uid
			}
		}
	}

	if err := h.documentRepo.Update(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to update file: %v", err),
		})
		return
	}

	// Convert to File model
	file := documentToFile(doc)
	c.JSON(http.StatusOK, file)
}

func (h *FileHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")

	// Try to parse as int64 first (new format)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		// Try to parse as UUID (legacy format or deterministic UUID)
		fileUUID, uuidErr := uuid.Parse(idStr)
		if uuidErr != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid file ID format",
			})
			return
		}

		// Extract document ID from deterministic UUID format
		// Format: 00000000-0000-0000-0000-{12 digit doc ID}
		uuidStr := fileUUID.String()
		if strings.HasPrefix(uuidStr, "00000000-0000-0000-0000-") {
			// Extract the last 12 digits
			docIDStr := strings.TrimPrefix(uuidStr, "00000000-0000-0000-0000-")
			id, err = strconv.ParseInt(docIDStr, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, errors.ErrorResponse{
					Error:   errors.ErrValidation.Code,
					Message: "Invalid document ID in UUID",
				})
				return
			}
		} else {
			// Random UUID - try to find document by searching
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Cannot delete file with random UUID. Please use document ID.",
			})
			return
		}
	}

	// Get document info before deleting to get file_path for Weaviate deletion and file removal
	doc, err := h.documentRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, errors.ErrorResponse{
			Error:   errors.ErrNotFound.Code,
			Message: "File not found",
		})
		return
	}

	// Get file_path for deletion
	var filePath string
	if doc.FilePath != nil {
		filePath = *doc.FilePath
	} else if doc.Content.Path != nil {
		filePath = *doc.Content.Path
	}

	// Delete physical file from disk first
	if filePath != "" {
		// Determine the actual file path
		diskPath := filePath
		if !filepath.IsAbs(diskPath) {
			// Relative path - join with storagePath
			diskPath = filepath.Join(h.storagePath, diskPath)
		}
		diskPath = filepath.Clean(diskPath)

		// Check if file exists and delete it
		if _, err := os.Stat(diskPath); err == nil {
			if err := os.Remove(diskPath); err != nil {
				// Log error but don't fail the deletion - file might have been manually deleted
				fmt.Printf("Warning: Failed to delete physical file %s: %v\n", diskPath, err)
			} else {
				fmt.Printf("Successfully deleted physical file: %s\n", diskPath)
			}
		} else {
			fmt.Printf("Physical file not found (may have been deleted already): %s\n", diskPath)
		}
	}

	// Delete from Weaviate and database using document service
	// This handles both Weaviate deletion and database deletion in one call
	if h.documentService != nil {
		fmt.Printf("Deleting document from Weaviate and database: %s (file_path: %s)\n", doc.ID, filePath)
		if err := h.documentService.DeleteDocument(c.Request.Context(), doc.ID); err != nil {
			// If deletion fails, return error
			fmt.Printf("Error: Failed to delete document %s: %v\n", doc.ID, err)
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: fmt.Sprintf("Failed to delete file: %v", err),
			})
			return
		}
		fmt.Printf("Successfully deleted document %s from Weaviate and database\n", doc.ID)
	} else {
		// Fallback: delete from documents table directly if service not available
		fmt.Printf("Document service not available, deleting from database directly: %s\n", doc.ID)
		if err := h.documentRepo.Delete(c.Request.Context(), id); err != nil {
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: fmt.Sprintf("Failed to delete file: %v", err),
			})
			return
		}
		fmt.Printf("Successfully deleted document/file %s from database\n", id)
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (h *FileHandler) Upload(c *gin.Context) {
	// Handle file upload via multipart/form-data
	// For now, we'll create a file record and return it
	// Actual file storage would be handled separately (S3, local storage, etc.)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "No file provided",
		})
		return
	}

	folderIDStr := c.PostForm("folder_id")
	var folderID *uuid.UUID
	if folderIDStr != "" {
		fid, err := uuid.Parse(folderIDStr)
		if err == nil {
			folderID = &fid
		}
	}

	// Get org_id from form data or context
	orgIDStr := c.PostForm("org_id")
	var orgUUID uuid.UUID
	if orgIDStr != "" {
		var parseErr error
		orgUUID, parseErr = uuid.Parse(orgIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID in form data",
			})
			return
		}
	} else {
		orgID, exists := c.Get("org_id")
		if !exists || orgID == nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Organization ID is required. Provide 'org_id' in form data or ensure you are associated with an organization",
			})
			return
		}

		orgIDStr, ok := orgID.(string)
		if !ok {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID format",
			})
			return
		}

		var parseErr error
		orgUUID, parseErr = uuid.Parse(orgIDStr)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, errors.ErrorResponse{
				Error:   errors.ErrValidation.Code,
				Message: "Invalid organization ID",
			})
			return
		}
	}

	userID, exists := c.Get("user_id")
	if !exists || userID == nil {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User ID is required",
		})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID format",
		})
		return
	}

	uid, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	// Extract extension
	ext := filepath.Ext(fileHeader.Filename)
	if ext != "" {
		ext = strings.ToLower(ext[1:])
	}

	// Generate storage key: {org_id}/{folder_path}/{file_name} (relative to storage path, not including storage path prefix)
	var storageKey string
	if folderID != nil {
		folder, err := h.folderRepo.GetByID(c.Request.Context(), *folderID)
		if err == nil {
			// Strip leading slash from folder path if present (folder.Path is like "/root/team/reports")
			folderPath := strings.TrimPrefix(folder.Path, "/")
			storageKey = filepath.Join(orgUUID.String(), folderPath, fileHeader.Filename)
		} else {
			// If folder not found, store in org root
			storageKey = filepath.Join(orgUUID.String(), fileHeader.Filename)
		}
	} else {
		// No folder, store in org root
		storageKey = filepath.Join(orgUUID.String(), fileHeader.Filename)
	}

	// Normalize path separators (use forward slashes for consistency)
	storageKey = filepath.Clean(storageKey)
	storageKey = strings.ReplaceAll(storageKey, "\\", "/")

	// Determine MIME type from extension
	mimeType := getMimeType(ext)
	sizeBytes := fileHeader.Size
	version := 1
	isFolder := false

	// Create document (file) in documents table
	doc := &repositories.Document{
		// ID will be auto-generated by BIGSERIAL
		OrgID:     &orgUUID,
		FolderID:  folderID,
		Name:      fileHeader.Filename,
		FilePath:  &storageKey,
		Status:    repositories.DocumentStatusCompleted,
		CreatedBy: &uid,
		Content: repositories.DocumentContent{
			MimeType:  &mimeType,
			SizeBytes: &sizeBytes,
			Version:   &version,
			IsFolder:  &isFolder,
			Path:      &storageKey,
		},
		Metadata: make(map[string]interface{}),
	}

	if err := h.documentRepo.Create(c.Request.Context(), doc); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create file record: %v", err),
		})
		return
	}

	// Save actual file to disk at the storage key path
	// storageKey is relative to storagePath, so we need to join them
	fullStoragePath := filepath.Join(h.storagePath, storageKey)

	// Create directory structure if it doesn't exist
	storageDir := filepath.Dir(fullStoragePath)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		// If directory creation fails, still return error but don't create DB record
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create storage directory: %v", err),
		})
		return
	}

	// Open destination file
	dst, err := os.Create(fullStoragePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create file: %v", err),
		})
		return
	}
	defer dst.Close()

	// Open uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to open uploaded file: %v", err),
		})
		return
	}
	defer src.Close()

	// Copy file content
	bytesWritten, err := io.Copy(dst, src)
	if err != nil {
		// Clean up: remove file if copy fails
		os.Remove(fullStoragePath)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to save file: %v", err),
		})
		return
	}

	// Ensure data is written to disk
	if err := dst.Sync(); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to sync file to disk: %v", err),
		})
		return
	}

	// Verify file was written correctly
	if bytesWritten != fileHeader.Size {
		os.Remove(fullStoragePath)
		// Delete the document record we created
		h.documentRepo.Delete(c.Request.Context(), doc.ID)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File size mismatch: expected %d bytes, wrote %d bytes", fileHeader.Size, bytesWritten),
		})
		return
	}

	// Verify file exists and has correct size
	if fileInfo, err := os.Stat(fullStoragePath); err != nil {
		// Delete the document record we created
		h.documentRepo.Delete(c.Request.Context(), doc.ID)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File verification failed: %v", err),
		})
		return
	} else if fileInfo.Size() != fileHeader.Size {
		os.Remove(fullStoragePath)
		// Delete the document record we created
		h.documentRepo.Delete(c.Request.Context(), doc.ID)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File size verification failed: expected %d bytes, got %d bytes", fileHeader.Size, fileInfo.Size()),
		})
		return
	}

	// For PDF files, verify the file starts with PDF header
	if ext == "pdf" {
		// Read first few bytes to verify it's a valid PDF
		verifyFile, err := os.Open(fullStoragePath)
		if err == nil {
			header := make([]byte, 4)
			if n, err := verifyFile.Read(header); err == nil && n == 4 {
				if string(header) != "%PDF" {
					verifyFile.Close()
					os.Remove(fullStoragePath)
					// Delete the document record we created
					h.documentRepo.Delete(c.Request.Context(), doc.ID)
					c.JSON(http.StatusBadRequest, errors.ErrorResponse{
						Error:   errors.ErrValidation.Code,
						Message: "Invalid PDF file: file does not start with PDF header",
					})
					return
				}
			}
			verifyFile.Close()
		}
	}

	// Update document with actual file size
	*doc.Content.SizeBytes = fileHeader.Size
	if err := h.documentRepo.Update(c.Request.Context(), doc); err != nil {
		fmt.Printf("Warning: Failed to update document with file size: %v\n", err)
	}

	// Convert to File model for API response
	file := documentToFile(doc)
	c.JSON(http.StatusCreated, models.FileUploadResponse{
		File:    file,
		Message: "File uploaded successfully",
	})
}

func getMimeType(ext string) string {
	mimeTypes := map[string]string{
		"pdf":  "application/pdf",
		"doc":  "application/msword",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xls":  "application/vnd.ms-excel",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"txt":  "text/plain",
		"csv":  "text/csv",
		"html": "text/html; charset=utf-8",
		"htm":  "text/html; charset=utf-8",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"gif":  "image/gif",
		"zip":  "application/zip",
	}

	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
