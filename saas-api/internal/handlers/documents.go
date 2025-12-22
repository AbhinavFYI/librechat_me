package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"saas-api/internal/services"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DocumentHandler struct {
	Services *services.Services
}

func NewDocumentHandler(services *services.Services) *DocumentHandler {
	return &DocumentHandler{Services: services}
}

// UploadDocument handles the POST /api/v1/documents/upload endpoint
func (h *DocumentHandler) UploadDocument() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user_id from context (should be set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists || userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			return
		}

		userIDStr, ok := userID.(string)
		if !ok || userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user ID",
			})
			return
		}

		// Parse multipart form
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "file is required",
			})
			return
		}

		// Get org_id from context or form data (nullable for superadmins)
		var orgID *uuid.UUID
		isSuperAdmin, _ := c.Get("is_super_admin")
		isSuperAdminBool := false
		if isSuperAdmin != nil {
			if val, ok := isSuperAdmin.(bool); ok {
				isSuperAdminBool = val
			}
		}

		// Try to get org_id from form data first
		if orgIDStr := c.PostForm("org_id"); orgIDStr != "" {
			if parsedOrgID, err := uuid.Parse(orgIDStr); err == nil {
				orgID = &parsedOrgID
			}
		} else if !isSuperAdminBool {
			// Non-superadmin: fallback to context org_id
			if orgIDVal, exists := c.Get("org_id"); exists && orgIDVal != nil {
				if orgIDStr, ok := orgIDVal.(string); ok && orgIDStr != "" {
					if parsedOrgID, err := uuid.Parse(orgIDStr); err == nil {
						orgID = &parsedOrgID
					}
				}
			}
		}

		// org_id is required for non-superadmin users only
		if !isSuperAdminBool && orgID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "org_id is required for non-superadmin users",
			})
			return
		}

		// Parse optional folder_id first (needed for path construction)
		var folderID *string
		var folderPath string
		if fid := c.PostForm("folder_id"); fid != "" {
			folderID = &fid
			// Get folder path from database
			if folderUUID, err := uuid.Parse(fid); err == nil {
				if folder, err := h.Services.GetRepositories().Folder.GetByID(c.Request.Context(), folderUUID); err == nil {
					// Use folder path (e.g., "/LLAMA/Whatsapp" -> "LLAMA/Whatsapp")
					folderPath = strings.TrimPrefix(folder.Path, "/")
				}
			}
		}

		// Construct file path with org_id and folder path
		filename := filepath.Clean(file.Filename)
		filename = strings.ReplaceAll(filename, " ", "_")
		filename = strings.ReplaceAll(filename, "'", "")

		// Construct file path (relative to ResourcesBasePath for database storage)
		// Full disk path for saving, relative path for database
		var diskPath string   // Full path for saving file to disk
		var dbFilePath string // Relative path for database storage

		if orgID != nil {
			// Include org_id and folder path for organization files
			// Structure: uploads/{org_id}/{folder_path}/filename
			var pathComponents []string
			pathComponents = append(pathComponents, h.Services.Document.ResourcesBasePath, orgID.String())
			if folderPath != "" {
				pathComponents = append(pathComponents, folderPath)
			}
			pathComponents = append(pathComponents, filename)

			// Create directory structure if it doesn't exist
			fileDir := path.Join(pathComponents[:len(pathComponents)-1]...)
			if err := os.MkdirAll(fileDir, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to create directory structure",
				})
				return
			}
			diskPath = path.Join(pathComponents...)

			// Store relative path in database: {org_id}/{folder_path}/filename
			var dbPathComponents []string
			dbPathComponents = append(dbPathComponents, orgID.String())
			if folderPath != "" {
				dbPathComponents = append(dbPathComponents, folderPath)
			}
			dbPathComponents = append(dbPathComponents, filename)
			dbFilePath = path.Join(dbPathComponents...)
		} else {
			// Superadmin files: uploads/{folder_path}/filename or uploads/filename
			var pathComponents []string
			pathComponents = append(pathComponents, h.Services.Document.ResourcesBasePath)
			if folderPath != "" {
				pathComponents = append(pathComponents, folderPath)
			}
			pathComponents = append(pathComponents, filename)

			// Create directory structure if it doesn't exist
			fileDir := path.Join(pathComponents[:len(pathComponents)-1]...)
			if err := os.MkdirAll(fileDir, 0755); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to create directory structure",
				})
				return
			}
			diskPath = path.Join(pathComponents...)

			// Store relative path in database: {folder_path}/filename or just filename
			if folderPath != "" {
				dbFilePath = path.Join(folderPath, filename)
			} else {
				dbFilePath = filename
			}
		}

		err = c.SaveUploadedFile(file, diskPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save file",
			})
			return
		}

		// Parse optional metadata (JSON string)
		var metadata map[string]interface{}
		if metadataStr := c.PostForm("metadata"); metadataStr != "" {
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid metadata format, expected JSON",
				})
				return
			}
		}
		// Prepare request
		req := &services.UploadDocumentRequest{
			UserID:   userIDStr,
			OrgID:    orgID,
			FilePath: dbFilePath, // Use relative path for database storage
			FolderID: folderID,
			Metadata: metadata,
		}

		// Call service to create document entry
		response, err := h.Services.Document.UploadDocument(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Get the created document to return as file object (for frontend compatibility)
		docInfo, err := h.Services.Document.GetJobStatus(c.Request.Context(), fmt.Sprintf("%d", response.DocumentID))
		if err != nil {
			// If we can't get the document, just return the basic response
			c.JSON(http.StatusOK, gin.H{
				"data":    response,
				"code":    http.StatusOK,
				"s":       "ok",
				"message": "Document uploaded successfully",
			})
			return
		}

		// Convert to file-compatible format for frontend
		var folderIDPtr *uuid.UUID
		if docInfo.FolderID != nil {
			if parsed, err := uuid.Parse(*docInfo.FolderID); err == nil {
				folderIDPtr = &parsed
			}
		}

		var orgIDValue uuid.UUID
		if orgID != nil {
			orgIDValue = *orgID
		}

		fileResponse := gin.H{
			"file": gin.H{
				"id":          response.DocumentID,
				"name":        response.Filename,
				"folder_id":   folderIDPtr,
				"org_id":      orgIDValue,
				"storage_key": docInfo.FilePath,
				"status":      response.Status,
				"created_at":  docInfo.CreatedAt,
				"updated_at":  docInfo.CreatedAt,
			},
			"message": "File uploaded successfully",
			"code":    http.StatusOK,
			"s":       "ok",
		}

		c.JSON(http.StatusOK, fileResponse)
	}
}

// GetDocuments handles the GET /api/v1/documents endpoint
func (h *DocumentHandler) GetDocuments() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Call service to get all documents
		documents, err := h.Services.Document.GetDocuments(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":    documents,
			"code":    http.StatusOK,
			"s":       "ok",
			"message": "Documents fetched successfully",
		})
	}
}

// SearchDocuments handles the GET /api/v1/documents/search endpoint
func (h *DocumentHandler) SearchDocuments() gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("query")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "query is required",
			})
			return
		}

		collection := c.Query("collection_id")
		if collection == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "collection is required",
			})
			return
		}

		//Add a validation for collection_id to be a number
		_, err := strconv.ParseInt(collection, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid collection_id format, expected int",
			})
			return
		}

		mode := c.Query("mode")

		if mode == "table" {
			collection = fmt.Sprintf("Document_%s_table", collection)
		} else {
			collection = fmt.Sprintf("Document_%s", collection)
		}

		score := 0.5
		if s := c.Query("score"); s != "" {
			score, err = strconv.ParseFloat(s, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid score format, expected float",
				})
				return
			}
		}

		alpha := float32(0.6)
		if a := c.Query("alpha"); a != "" {
			alphaFloat, err := strconv.ParseFloat(a, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid alpha format, expected float",
				})
				return
			}
			alpha = float32(alphaFloat)
		}

		results, err := h.Services.Document.SearchDocuments(c.Request.Context(), query, collection, score, alpha)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":    results,
			"code":    http.StatusOK,
			"s":       "ok",
			"message": "Documents searched successfully",
		})
	}
}

// GetDocumentsWithFilter handles the GET /api/v1/documents with query params endpoint
func (h *DocumentHandler) GetDocumentsWithFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse query parameters
		var folderID *string
		if fid := c.Query("folder_id"); fid != "" {
			folderID = &fid
		}

		// Get org_id from query or context (nullable for superadmins)
		var orgID *uuid.UUID
		isSuperAdmin, _ := c.Get("is_super_admin")
		isSuperAdminBool := false
		if isSuperAdmin != nil {
			if val, ok := isSuperAdmin.(bool); ok {
				isSuperAdminBool = val
			}
		}

		// Try to get org_id from query first
		if orgIDStr := c.Query("org_id"); orgIDStr != "" {
			if parsedOrgID, err := uuid.Parse(orgIDStr); err == nil {
				orgID = &parsedOrgID
			}
		} else {
			// Fallback to context
			if orgIDVal, exists := c.Get("org_id"); exists && orgIDVal != nil {
				if orgIDStr, ok := orgIDVal.(string); ok && orgIDStr != "" {
					if parsedOrgID, err := uuid.Parse(orgIDStr); err == nil {
						orgID = &parsedOrgID
					}
				}
			}
		}

		// For superadmins, org_id can be null - allow the request to proceed
		// For non-superadmin users, org_id is required
		if !isSuperAdminBool && orgID == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "org_id is required for non-superadmin users",
				"message": "Please provide org_id as a query parameter or ensure your JWT token includes is_super_admin claim",
			})
			return
		}

		// Parse pagination parameters with defaults
		page := 1
		if p := c.Query("page"); p != "" {
			if pageInt, err := parseIntWithDefault(p, 1); err == nil {
				page = pageInt
			}
		}

		limit := 50
		if l := c.Query("limit"); l != "" {
			if limitInt, err := parseIntWithDefault(l, 50); err == nil {
				limit = limitInt
			}
		}

		// Validate pagination
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 50
		}

		// Prepare request
		req := &services.GetDocumentsRequest{
			FolderID: folderID,
			OrgID:    orgID,
			Page:     page,
			Limit:    limit,
		}

		// Call service
		response, err := h.Services.Document.GetDocumentsWithFilter(c.Request.Context(), req)
		if err != nil {
			fmt.Printf("GetDocumentsWithFilter error: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":    response,
			"code":    http.StatusOK,
			"s":       "ok",
			"message": "Documents fetched successfully",
		})
	}
}

// GetJobStatus handles GET /api/v1/documents/jobs/:job_id endpoint
func (h *DocumentHandler) GetJobStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := c.Param("job_id")
		if jobID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "job_id is required",
			})
			return
		}

		docInfo, err := h.Services.Document.GetJobStatus(c.Request.Context(), jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data":    docInfo,
			"code":    http.StatusOK,
			"s":       "ok",
			"message": "Job status fetched successfully",
		})
	}
}

// GetAllJobs handles GET /api/v1/documents/jobs endpoint
func (h *DocumentHandler) GetAllJobs() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobs := h.Services.Document.GetAllJobs(c.Request.Context())

		response := make([]gin.H, 0, len(jobs))
		for _, job := range jobs {
			response = append(response, gin.H{
				"job_id":       job.ID,
				"status":       job.Status,
				"file_path":    job.FilePath,
				"created_at":   job.CreatedAt,
				"started_at":   job.StartedAt,
				"completed_at": job.CompletedAt,
				"error":        errorToString(job.Error),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"data":    response,
			"code":    http.StatusOK,
			"s":       "ok",
			"message": "Jobs fetched successfully",
		})
	}
}

// DeleteDocument handles DELETE /api/v1/documents/:document_id endpoint
func (h *DocumentHandler) DeleteDocument() gin.HandlerFunc {
	return func(c *gin.Context) {
		documentIDStr := c.Param("document_id")
		if documentIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "document_id is required",
			})
			return
		}

		// Parse document_id as int64
		documentID, err := strconv.ParseInt(documentIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid document_id format, expected integer",
			})
			return
		}

		// Call service to delete document
		err = h.Services.Document.DeleteDocument(c.Request.Context(), documentID)
		if err != nil {
			// Check if document not found
			if strings.Contains(err.Error(), "not found") {
				c.JSON(http.StatusNotFound, gin.H{
					"error": err.Error(),
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    http.StatusOK,
			"s":       "ok",
			"message": fmt.Sprintf("Document %d deleted successfully", documentID),
		})
	}
}

// parseIntWithDefault parses an integer string and returns default value on error
func parseIntWithDefault(s string, defaultVal int) (int, error) {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultVal, err
	}
	return result, nil
}

// errorToString converts error to string, returning nil if error is nil
func errorToString(err error) *string {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	return &errStr
}

// DownloadDocument handles the GET /api/v1/documents/:document_id/download endpoint
func (h *DocumentHandler) DownloadDocument() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get document ID from URL parameter
		documentIDStr := c.Param("document_id")
		_, err := strconv.ParseInt(documentIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid document ID",
			})
			return
		}

		// Get document from database
		doc, err := h.Services.Document.GetJobStatus(c.Request.Context(), documentIDStr)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Document not found",
			})
			return
		}

		// Check if file path exists
		if doc.FilePath == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Document file not found",
			})
			return
		}

		// Construct full file path
		filePath := doc.FilePath
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(h.Services.Document.ResourcesBasePath, filePath)
		}

		// Serve the file
		c.FileAttachment(filePath, doc.Name)
	}
}
