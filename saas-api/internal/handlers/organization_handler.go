package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrganizationHandler struct {
	orgRepo  *repositories.OrganizationRepository
	roleRepo *repositories.RoleRepository
	permRepo *repositories.PermissionRepository
}

func NewOrganizationHandler(orgRepo *repositories.OrganizationRepository, roleRepo *repositories.RoleRepository, permRepo *repositories.PermissionRepository) *OrganizationHandler {
	return &OrganizationHandler{
		orgRepo:  orgRepo,
		roleRepo: roleRepo,
		permRepo: permRepo,
	}
}

func (h *OrganizationHandler) Create(c *gin.Context) {
	var req models.CreateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	// Auto-generate slug from name (internal use only, not exposed in API)
	// Generate slug from name: lowercase, replace spaces with hyphens, remove special chars
	slug := strings.ToLower(req.Name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove any characters that aren't alphanumeric or hyphen
	var builder strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}
	slug = builder.String()
	// Remove multiple consecutive hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Ensure it's not empty
	if slug == "" {
		slug = "organization"
	}

	// Check if slug exists and append number suffix if needed (slug, slug-1, slug-2, etc.)
	baseSlug := slug
	counter := 0
	maxAttempts := 100 // Prevent infinite loop
	for counter < maxAttempts {
		existingOrg, err := h.orgRepo.GetBySlug(c.Request.Context(), slug)
		// If GetBySlug returns ErrNotFound, the slug is available
		if err != nil {
			if appErr, ok := err.(*errors.AppError); ok && appErr == errors.ErrNotFound {
				// Slug is available
				break
			}
			// Some other error occurred, log it but continue
			log.Printf("Warning: Error checking slug availability for '%s': %v", slug, err)
		}
		// Slug exists (existingOrg != nil), try next number
		if existingOrg != nil {
			counter++
			slug = fmt.Sprintf("%s-%d", baseSlug, counter)
		} else {
			// Slug is available (err was ErrNotFound)
			break
		}
	}
	if counter >= maxAttempts {
		log.Printf("Warning: Could not find available slug after %d attempts for base slug '%s'", maxAttempts, baseSlug)
	}

	// Set default values
	maxUsers := 100
	maxStorageGB := 1

	// Override with request values if provided
	if req.MaxUsers != nil {
		maxUsers = *req.MaxUsers
	}
	if req.MaxStorageGB != nil {
		maxStorageGB = *req.MaxStorageGB
	}

	org := &models.Organization{
		ID:                  uuid.New(),
		Name:                req.Name,
		LegalName:           req.LegalName,
		Slug:                slug,
		LogoURL:             req.LogoURL,
		Website:             req.Website,
		AddressLine1:        req.AddressLine1,
		City:                req.City,
		StateProvince:       req.StateProvince,
		PostalCode:          req.PostalCode,
		Country:             req.Country,
		PrimaryContactName:  req.PrimaryContactName,
		PrimaryContactEmail: req.PrimaryContactEmail,
		PrimaryContactPhone: req.PrimaryContactPhone,
		BillingEmail:        req.BillingEmail,
		SubscriptionPlan:    "free",
		SubscriptionStatus:  "active",
		MaxUsers:            maxUsers,
		MaxStorageGB:        maxStorageGB,
		Timezone:            "UTC",
		DateFormat:          "YYYY-MM-DD",
		Locale:              "en-US",
		Status:              "active",
		CreatedBy:           &uid,
		Settings:            req.Settings,
		Metadata:            make(map[string]interface{}),
	}

	if req.SubscriptionPlan != nil {
		org.SubscriptionPlan = *req.SubscriptionPlan
	}

	if org.Settings == nil {
		org.Settings = make(map[string]interface{})
	}

	if err := h.orgRepo.Create(c.Request.Context(), org); err != nil {
		log.Printf("Failed to create organization - Error: %v, Type: %T", err, err)
		if appErr, ok := err.(*errors.AppError); ok {
			log.Printf("AppError details - Code: %s, Message: %s, Status: %d", appErr.Code, appErr.Message, appErr.Status)
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Non-AppError: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to create organization: %v", err),
		})
		return
	}

	// Create default roles for the organization
	if err := h.createDefaultRoles(c.Request.Context(), org.ID, &uid); err != nil {
		log.Printf("Warning: Failed to create default roles for organization %s: %v", org.ID, err)
		// Don't fail organization creation if role creation fails, just log it
	}

	c.JSON(http.StatusCreated, org)
}

func (h *OrganizationHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid organization ID",
		})
		return
	}

	org, err := h.orgRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get organization",
		})
		return
	}

	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	orgs, total, err := h.orgRepo.List(c.Request.Context(), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to list organizations",
		})
		return
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:       orgs,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}

func (h *OrganizationHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid organization ID",
		})
		return
	}

	var req models.UpdateOrganizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	org, err := h.orgRepo.GetByID(c.Request.Context(), id)
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
			Message: "Failed to get organization",
		})
		return
	}

	// Update fields
	if req.Name != nil {
		org.Name = *req.Name
	}
	if req.LegalName != nil {
		org.LegalName = req.LegalName
	}
	if req.LogoURL != nil {
		org.LogoURL = req.LogoURL
	}
	if req.Website != nil {
		org.Website = req.Website
	}
	if req.AddressLine1 != nil {
		org.AddressLine1 = req.AddressLine1
	}
	if req.City != nil {
		org.City = req.City
	}
	if req.StateProvince != nil {
		org.StateProvince = req.StateProvince
	}
	if req.PostalCode != nil {
		org.PostalCode = req.PostalCode
	}
	if req.Country != nil {
		org.Country = req.Country
	}
	if req.PrimaryContactName != nil {
		org.PrimaryContactName = req.PrimaryContactName
	}
	if req.PrimaryContactEmail != nil {
		org.PrimaryContactEmail = req.PrimaryContactEmail
	}
	if req.PrimaryContactPhone != nil {
		org.PrimaryContactPhone = req.PrimaryContactPhone
	}
	if req.BillingEmail != nil {
		org.BillingEmail = req.BillingEmail
	}
	if req.SubscriptionPlan != nil {
		org.SubscriptionPlan = *req.SubscriptionPlan
	}
	if req.Status != nil {
		// Validate status value - must match database enum: active, suspended, pending, deleted
		validStatuses := map[string]bool{"active": true, "suspended": true, "pending": true, "deleted": true}
		if validStatuses[*req.Status] {
			org.Status = *req.Status
		} else {
			log.Printf("Invalid status value: %s, allowed values: active, suspended, pending, deleted", *req.Status)
		}
	}
	if req.Settings != nil {
		org.Settings = req.Settings
	}

	userID, _ := c.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))
	org.UpdatedBy = &uid

	if err := h.orgRepo.Update(c.Request.Context(), org); err != nil {
		log.Printf("Failed to update organization - Error: %v, Type: %T", err, err)
		if appErr, ok := err.(*errors.AppError); ok {
			log.Printf("AppError details - Code: %s, Message: %s, Status: %d", appErr.Code, appErr.Message, appErr.Status)
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Non-AppError: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: fmt.Sprintf("Failed to update organization: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid organization ID",
		})
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := uuid.Parse(userID.(string))

	reason := "Deleted by user"
	if err := h.orgRepo.Delete(c.Request.Context(), id, uid, reason); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete organization",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Organization deleted successfully"})
}

// createDefaultRoles creates default roles (Org Admin, User, Viewer) for a new organization
func (h *OrganizationHandler) createDefaultRoles(ctx context.Context, orgID uuid.UUID, createdBy *uuid.UUID) error {
	// Create "Org Admin" role
	adminRole := &models.Role{
		ID:          uuid.New(),
		OrgID:       &orgID,
		Name:        "Org Admin",
		Type:        "org_defined",
		Description: stringPtr("Full administrative access to organization"),
		IsDefault:   false,
		CreatedBy:   createdBy,
	}
	if err := h.roleRepo.Create(ctx, adminRole); err != nil {
		return err
	}

	// Create "User" role (default)
	userRole := &models.Role{
		ID:          uuid.New(),
		OrgID:       &orgID,
		Name:        "User",
		Type:        "org_defined",
		Description: stringPtr("Standard user access"),
		IsDefault:   true,
		CreatedBy:   createdBy,
	}
	if err := h.roleRepo.Create(ctx, userRole); err != nil {
		return err
	}

	// Create "Viewer" role
	viewerRole := &models.Role{
		ID:          uuid.New(),
		OrgID:       &orgID,
		Name:        "Viewer",
		Type:        "org_defined",
		Description: stringPtr("Read-only access"),
		IsDefault:   false,
		CreatedBy:   createdBy,
	}
	if err := h.roleRepo.Create(ctx, viewerRole); err != nil {
		return err
	}

	// Get all permissions and assign them to Org Admin role
	permissions, err := h.permRepo.List(ctx)
	if err != nil {
		return err
	}

	permissionIDs := make([]uuid.UUID, len(permissions))
	for i, perm := range permissions {
		permissionIDs[i] = perm.ID
	}

	// Assign all permissions to Org Admin role
	if err := h.roleRepo.AssignPermissions(ctx, adminRole.ID, permissionIDs, *createdBy); err != nil {
		return err
	}

	// Assign read/list permissions to default "User" role (initial users get read-only access)
	readPermissionIDs := []uuid.UUID{}
	for _, perm := range permissions {
		// Assign all "read" and "list" permissions to User role
		if perm.Action == "read" || perm.Action == "list" {
			readPermissionIDs = append(readPermissionIDs, perm.ID)
		}
	}
	if len(readPermissionIDs) > 0 {
		if err := h.roleRepo.AssignPermissions(ctx, userRole.ID, readPermissionIDs, *createdBy); err != nil {
			log.Printf("Warning: Failed to assign read permissions to User role: %v", err)
			// Don't fail org creation if this fails
		}
	}

	// Assign read/list permissions to "Viewer" role as well
	if len(readPermissionIDs) > 0 {
		if err := h.roleRepo.AssignPermissions(ctx, viewerRole.ID, readPermissionIDs, *createdBy); err != nil {
			log.Printf("Warning: Failed to assign read permissions to Viewer role: %v", err)
			// Don't fail org creation if this fails
		}
	}

	return nil
}

func stringPtr(s string) *string {
	return &s
}
