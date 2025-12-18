package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type StaticHandler struct {
	storagePath  string
	documentRepo *repositories.DocumentRepository
}

func NewStaticHandler(storagePath string, documentRepo *repositories.DocumentRepository) *StaticHandler {
	return &StaticHandler{
		storagePath:  storagePath,
		documentRepo: documentRepo,
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
	// But if requestPath already starts with storagePath, use it as-is
	var fullPath string
	if strings.HasPrefix(requestPath, h.storagePath+"/") || strings.HasPrefix(requestPath, h.storagePath+"\\") {
		// Path already includes storage path prefix
		fullPath = filepath.Clean(requestPath)
	} else {
		// Path is relative to storage path
		fullPath = filepath.Join(h.storagePath, requestPath)
	}

	// Normalize path separators
	fullPath = filepath.Clean(fullPath)

	// Debug logging
	fmt.Printf("Static handler: Looking for file at path: %s (requestPath: %s, storagePath: %s)\n", fullPath, requestPath, h.storagePath)

	// Check if file exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try alternative paths in case of legacy storage_key formats
			// 1. Try with storage path prefix if not already included
			altPath1 := filepath.Join(h.storagePath, requestPath)
			if altPath1 != fullPath {
				if info, err2 := os.Stat(altPath1); err2 == nil {
					fileInfo = info
					fullPath = altPath1
					err = nil
				}
			}

			// 2. If still not found, try removing storage path prefix if it exists
			if err != nil && strings.HasPrefix(requestPath, h.storagePath+"/") {
				relPath := strings.TrimPrefix(requestPath, h.storagePath+"/")
				altPath2 := filepath.Join(h.storagePath, relPath)
				if info, err2 := os.Stat(altPath2); err2 == nil {
					fileInfo = info
					fullPath = altPath2
					err = nil
				}
			}

			// 3. If still not found, try to lookup file in database by path pattern
			if err != nil && h.documentRepo != nil {
				fmt.Printf("File not found on disk, trying database lookup for: %s\n", requestPath)

				// Try to find document by file_path pattern
				dbDoc, dbErr := h.documentRepo.GetByStorageKeyPattern(context.Background(), requestPath)
				if dbErr == nil && dbDoc != nil {
					var dbFilePath string
					if dbDoc.FilePath != nil {
						dbFilePath = *dbDoc.FilePath
					} else if dbDoc.Content.Path != nil {
						dbFilePath = *dbDoc.Content.Path
					}

					if dbFilePath != "" {
						fmt.Printf("Found document in database by pattern: file_path=%s\n", dbFilePath)
						// Found document in database, construct path from file_path
						if !filepath.IsAbs(dbFilePath) {
							dbFilePath = filepath.Join(h.storagePath, dbFilePath)
						}
						dbFilePath = filepath.Clean(dbFilePath)
						fmt.Printf("Trying database file_path: %s\n", dbFilePath)

						// Try the database file_path
						if info, err2 := os.Stat(dbFilePath); err2 == nil {
							fmt.Printf("File found at database file_path: %s\n", dbFilePath)
							fileInfo = info
							fullPath = dbFilePath
							err = nil
						} else {
							fmt.Printf("File not found at database file_path: %s, trying filename lookup\n", dbFilePath)
							// Also try by filename and path segments
							pathParts := strings.Split(requestPath, "/")
							if len(pathParts) > 0 {
								filename := pathParts[len(pathParts)-1]
								var orgID *uuid.UUID
								if len(pathParts) > 1 {
									if parsedUUID, parseErr := uuid.Parse(pathParts[0]); parseErr == nil {
										orgID = &parsedUUID
									}
								}
								pathSegments := pathParts[:len(pathParts)-1]

								fmt.Printf("Trying filename lookup: filename=%s, orgID=%v, pathSegments=%v\n", filename, orgID, pathSegments)
								dbDoc2, dbErr2 := h.documentRepo.GetByFilenameAndPath(context.Background(), orgID, filename, pathSegments)
								if dbErr2 == nil && dbDoc2 != nil {
									var dbFilePath2 string
									if dbDoc2.FilePath != nil {
										dbFilePath2 = *dbDoc2.FilePath
									} else if dbDoc2.Content.Path != nil {
										dbFilePath2 = *dbDoc2.Content.Path
									}

									if dbFilePath2 != "" {
										fmt.Printf("Found document in database by filename: file_path=%s\n", dbFilePath2)
										if !filepath.IsAbs(dbFilePath2) {
											dbFilePath2 = filepath.Join(h.storagePath, dbFilePath2)
										}
										dbFilePath2 = filepath.Clean(dbFilePath2)
										fmt.Printf("Trying filename lookup path: %s\n", dbFilePath2)

										if info, err2 := os.Stat(dbFilePath2); err2 == nil {
											fmt.Printf("File found at filename lookup path: %s\n", dbFilePath2)
											fileInfo = info
											fullPath = dbFilePath2
											err = nil
										} else {
											fmt.Printf("File not found at filename lookup path: %s\n", dbFilePath2)
										}
									}
								} else {
									fmt.Printf("Document not found in database by filename: %v\n", dbErr2)
								}
							}
						}
					}
				} else {
					fmt.Printf("Document not found in database by pattern: %v\n", dbErr)
				}
			}
		}

		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, errors.ErrorResponse{
					Error:   errors.ErrNotFound.Code,
					Message: fmt.Sprintf("File not found: %s (tried: %s)", requestPath, fullPath),
				})
				return
			}
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: fmt.Sprintf("Failed to access file: %v", err),
			})
			return
		}
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

	// Validate access - Super admins can access any file regardless of org_id
	if !isSuperAdminBool {
		if pathOrgUUID != nil {
			// Path contains org_id - validate user has access to this org
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
		} else {
			// Path doesn't contain org_id (legacy format like "LLAMA/Whatsapp/login2.png")
			// Only allow super admin access
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this file",
			})
			return
		}
	}
	// Super admin can access any file - no validation needed

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
