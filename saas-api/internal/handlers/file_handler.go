package handlers

import (
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
	fileRepo    *repositories.FileRepository
	folderRepo  *repositories.FolderRepository
	storagePath string // Base path for file storage: {storagePath}/{org_id}/folder/files
}

func NewFileHandler(fileRepo *repositories.FileRepository, folderRepo *repositories.FolderRepository, storagePath string) *FileHandler {
	return &FileHandler{
		fileRepo:    fileRepo,
		folderRepo:  folderRepo,
		storagePath: storagePath,
	}
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

	// Generate storage key: {base_path}/{org_id}/{folder_path}/{file_name}
	var storageKey string
	if req.FolderID != nil {
		folder, err := h.folderRepo.GetByID(c.Request.Context(), *req.FolderID)
		if err == nil {
			storageKey = filepath.Join(h.storagePath, orgUUID.String(), folder.Path, req.Name)
		} else {
			// If folder not found, store in org root
			storageKey = filepath.Join(h.storagePath, orgUUID.String(), req.Name)
		}
	} else {
		// No folder, store in org root
		storageKey = filepath.Join(h.storagePath, orgUUID.String(), req.Name)
	}

	// Normalize path separators
	storageKey = filepath.Clean(storageKey)

	file := &models.File{
		ID:         uuid.New(),
		OrgID:      orgUUID,
		FolderID:   req.FolderID,
		Name:       req.Name,
		Extension:  &ext,
		StorageKey: storageKey,
		Version:    1,
		CreatedBy:  &uid,
	}

	if err := h.fileRepo.Create(c.Request.Context(), file); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create file",
		})
		return
	}

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

	files, total, err := h.fileRepo.List(c.Request.Context(), orgUUID, folderID, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list files",
		})
		return
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
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid file ID",
		})
		return
	}

	file, err := h.fileRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && appErr == errors.ErrNotFound {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "File not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get file",
		})
		return
	}

	// Check organization access - users can only access files from their own organization
	// Super admins can access files from any organization
	// Users from the same org can access each other's files
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
		if err != nil || userOrgUUID != file.OrgID {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this file. Files can only be accessed by users from the same organization.",
			})
			return
		}
	}
	// Super admin - allow access to any file

	// Check if download/preview is requested (default to serving file)
	downloadParam := c.Query("download")
	// Always serve file if download parameter is present (true, 1, or empty means serve file)
	// Only return JSON if explicitly requested with download=false
	if downloadParam != "false" {
		// Determine the actual file path
		// StorageKey might be absolute or relative to storagePath
		filePath := file.StorageKey

		// Check if the path is already a valid absolute path that exists
		// If not, try to construct it relative to storagePath
		if filepath.IsAbs(filePath) {
			// It's an absolute path - check if it exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				// Absolute path doesn't exist, try relative to storagePath
				// Remove leading slash if present and join with storagePath
				relPath := strings.TrimPrefix(filePath, "/")
				filePath = filepath.Join(h.storagePath, relPath)
			}
		} else {
			// Relative path - join with storagePath
			filePath = filepath.Join(h.storagePath, filePath)
		}

		// Clean the path
		filePath = filepath.Clean(filePath)

		// Serve file for download
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// Try the original storageKey as well
			if filePath != file.StorageKey {
				if _, err2 := os.Stat(file.StorageKey); os.IsNotExist(err2) {
					c.JSON(http.StatusNotFound, errors.ErrorResponse{
						Error:   errors.ErrNotFound.Code,
						Message: fmt.Sprintf("File not found on disk. Tried: %s and %s", filePath, file.StorageKey),
					})
					return
				}
				filePath = file.StorageKey
			} else {
				c.JSON(http.StatusNotFound, errors.ErrorResponse{
					Error:   errors.ErrNotFound.Code,
					Message: fmt.Sprintf("File not found on disk: %s", filePath),
				})
				return
			}
		}

		// Set appropriate content type based on file extension
		// Prioritize extension-based detection to ensure correct MIME type
		mimeType := "application/octet-stream"
		if file.Extension != nil && *file.Extension != "" {
			mimeType = getMimeType(*file.Extension)
		} else if file.MimeType != nil && *file.MimeType != "application/octet-stream" {
			// Only use stored MIME type if extension is not available and it's not the default
			mimeType = *file.MimeType
		}

		// Set headers for file download
		c.Header("Content-Type", mimeType)
		// For HTML and PDF files, use inline to allow preview; for others, use attachment to force download
		if strings.HasPrefix(mimeType, "text/html") || mimeType == "application/pdf" {
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", file.Name))
		} else {
			c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", file.Name))
		}
		// Serve the file
		c.File(filePath)
		return
	}

	c.JSON(http.StatusOK, file)
}

func (h *FileHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid file ID",
		})
		return
	}

	var req models.UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get existing file
	file, err := h.fileRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && appErr == errors.ErrNotFound {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "File not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get file",
		})
		return
	}

	// Update fields
	if req.Name != nil {
		file.Name = *req.Name
		// Update extension if name changed
		ext := filepath.Ext(*req.Name)
		if ext != "" {
			ext = strings.ToLower(ext[1:])
			file.Extension = &ext
		}
	}
	if req.FolderID != nil {
		file.FolderID = req.FolderID
	}

	userID, exists := c.Get("user_id")
	if exists && userID != nil {
		if userIDStr, ok := userID.(string); ok {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				file.UpdatedBy = &uid
			}
		}
	}

	if err := h.fileRepo.Update(c.Request.Context(), file); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update file",
		})
		return
	}

	c.JSON(http.StatusOK, file)
}

func (h *FileHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid file ID",
		})
		return
	}

	if err := h.fileRepo.Delete(c.Request.Context(), id); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete file",
		})
		return
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

	// Generate storage key: {base_path}/{org_id}/{folder_path}/{file_name}
	// Example: uploads/550e8400-e29b-41d4-a716-446655440000/root/team/reports/document.pdf
	var storageKey string
	if folderID != nil {
		folder, err := h.folderRepo.GetByID(c.Request.Context(), *folderID)
		if err == nil {
			// Use folder path relative to org root
			storageKey = filepath.Join(h.storagePath, orgUUID.String(), folder.Path, fileHeader.Filename)
		} else {
			// If folder not found, store in org root
			storageKey = filepath.Join(h.storagePath, orgUUID.String(), fileHeader.Filename)
		}
	} else {
		// No folder, store in org root
		storageKey = filepath.Join(h.storagePath, orgUUID.String(), fileHeader.Filename)
	}

	// Normalize path separators
	storageKey = filepath.Clean(storageKey)

	// Determine MIME type from extension
	mimeType := getMimeType(ext)

	file := &models.File{
		ID:         uuid.New(),
		OrgID:      orgUUID,
		FolderID:   folderID,
		Name:       fileHeader.Filename,
		Extension:  &ext,
		MimeType:   &mimeType,
		SizeBytes:  &fileHeader.Size,
		StorageKey: storageKey,
		Version:    1,
		CreatedBy:  &uid,
	}

	if err := h.fileRepo.Create(c.Request.Context(), file); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create file record",
		})
		return
	}

	// Save actual file to disk at the storage key path
	// Create directory structure if it doesn't exist
	storageDir := filepath.Dir(storageKey)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		// If directory creation fails, still return error but don't create DB record
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create storage directory: %v", err),
		})
		return
	}

	// Open destination file
	dst, err := os.Create(storageKey)
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
		os.Remove(storageKey)
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
		os.Remove(storageKey)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File size mismatch: expected %d bytes, wrote %d bytes", fileHeader.Size, bytesWritten),
		})
		return
	}

	// Verify file exists and has correct size
	if fileInfo, err := os.Stat(storageKey); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File verification failed: %v", err),
		})
		return
	} else if fileInfo.Size() != fileHeader.Size {
		os.Remove(storageKey)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("File size verification failed: expected %d bytes, got %d bytes", fileHeader.Size, fileInfo.Size()),
		})
		return
	}

	// For PDF files, verify the file starts with PDF header
	if ext == "pdf" {
		// Read first few bytes to verify it's a valid PDF
		verifyFile, err := os.Open(storageKey)
		if err == nil {
			header := make([]byte, 4)
			if n, err := verifyFile.Read(header); err == nil && n == 4 {
				if string(header) != "%PDF" {
					verifyFile.Close()
					os.Remove(storageKey)
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
