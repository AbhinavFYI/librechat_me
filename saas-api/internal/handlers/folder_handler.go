package handlers

import (
	"net/http"
	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FolderHandler struct {
	folderRepo   *repositories.FolderRepository
	documentRepo *repositories.DocumentRepository
}

func NewFolderHandler(folderRepo *repositories.FolderRepository, documentRepo *repositories.DocumentRepository) *FolderHandler {
	return &FolderHandler{
		folderRepo:   folderRepo,
		documentRepo: documentRepo,
	}
}

func (h *FolderHandler) Create(c *gin.Context) {
	var req models.CreateFolderRequest
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

	folder := &models.Folder{
		ID:        uuid.New(),
		OrgID:     orgUUID,
		Name:      req.Name,
		ParentID:  req.ParentID,
		CreatedBy: &uid,
	}

	if err := h.folderRepo.Create(c.Request.Context(), folder); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create folder",
		})
		return
	}

	c.JSON(http.StatusCreated, folder)
}

func (h *FolderHandler) List(c *gin.Context) {
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

	parentIDStr := c.Query("parent_id")
	var parentID *uuid.UUID
	if parentIDStr != "" {
		pid, err := uuid.Parse(parentIDStr)
		if err == nil {
			parentID = &pid
		}
	}

	folders, err := h.folderRepo.List(c.Request.Context(), orgUUID, parentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list folders",
		})
		return
	}

	c.JSON(http.StatusOK, folders)
}

func (h *FolderHandler) GetTree(c *gin.Context) {
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

	folders, err := h.folderRepo.GetTree(c.Request.Context(), orgUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get folder tree",
		})
		return
	}

	// Also get files for each folder
	for _, folder := range folders {
		documents, _ := h.documentRepo.GetByFolder(c.Request.Context(), folder.ID)
		// Convert documents to files
		files := make([]*models.File, len(documents))
		for i, doc := range documents {
			files[i] = documentToFile(doc)
		}
		folder.Files = files
		// Recursively get files for children
		h.populateFiles(c, folder)
	}

	c.JSON(http.StatusOK, folders)
}

func (h *FolderHandler) populateFiles(c *gin.Context, folder *models.Folder) {
	documents, _ := h.documentRepo.GetByFolder(c.Request.Context(), folder.ID)
	// Convert documents to files
	files := make([]*models.File, len(documents))
	for i, doc := range documents {
		files[i] = documentToFile(doc)
	}
	folder.Files = files
	for _, child := range folder.Children {
		h.populateFiles(c, child)
	}
}

func (h *FolderHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	folder, err := h.folderRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && appErr == errors.ErrNotFound {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "Folder not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get folder",
		})
		return
	}

	// Check organization access - users can only access folders from their own organization
	// Super admins can access folders from any organization
	// Users from the same org can access each other's folders
	isSuperAdmin, _ := c.Get("is_super_admin")
	if isSuperAdmin == nil || !isSuperAdmin.(bool) {
		// Regular user - check if folder belongs to their organization
		userOrgID, exists := c.Get("org_id")
		if !exists || userOrgID == nil {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this folder",
			})
			return
		}

		userOrgIDStr, ok := userOrgID.(string)
		if !ok {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this folder",
			})
			return
		}

		userOrgUUID, err := uuid.Parse(userOrgIDStr)
		if err != nil || userOrgUUID != folder.OrgID {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "You do not have access to this folder. Folders can only be accessed by users from the same organization.",
			})
			return
		}
	}
	// Super admin - allow access to any folder

	// Get files in this folder
	documents, _ := h.documentRepo.GetByFolder(c.Request.Context(), id)
	// Convert documents to files
	files := make([]*models.File, len(documents))
	for i, doc := range documents {
		files[i] = documentToFile(doc)
	}
	folder.Files = files

	// Get permissions
	permissions, _ := h.folderRepo.GetPermissions(c.Request.Context(), id)
	folder.Permissions = permissions

	c.JSON(http.StatusOK, folder)
}

func (h *FolderHandler) Update(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	var req models.UpdateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get existing folder
	folder, err := h.folderRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok && appErr == errors.ErrNotFound {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "Folder not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get folder",
		})
		return
	}

	// Update fields
	if req.Name != nil {
		folder.Name = *req.Name
	}
	if req.ParentID != nil {
		folder.ParentID = req.ParentID
	}

	userID, exists := c.Get("user_id")
	if exists && userID != nil {
		if userIDStr, ok := userID.(string); ok {
			if uid, err := uuid.Parse(userIDStr); err == nil {
				folder.UpdatedBy = &uid
			}
		}
	}

	if err := h.folderRepo.Update(c.Request.Context(), folder); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to update folder",
		})
		return
	}

	c.JSON(http.StatusOK, folder)
}

func (h *FolderHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	if err := h.folderRepo.Delete(c.Request.Context(), id); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete folder",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Folder deleted successfully"})
}

func (h *FolderHandler) GetPermissions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	permissions, err := h.folderRepo.GetPermissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get permissions",
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *FolderHandler) AssignPermission(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	var req models.AssignFolderPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	if err := h.folderRepo.AssignPermission(c.Request.Context(), id, req.RoleID, req.Permission); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to assign permission",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permission assigned successfully"})
}

func (h *FolderHandler) RemovePermission(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid folder ID",
		})
		return
	}

	roleIDStr := c.Param("role_id")
	roleID, err := uuid.Parse(roleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid role ID",
		})
		return
	}

	if err := h.folderRepo.RemovePermission(c.Request.Context(), id, roleID); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to remove permission",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Permission removed successfully"})
}
