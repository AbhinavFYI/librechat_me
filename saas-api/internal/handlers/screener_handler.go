package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ScreenerHandler struct {
	screenerRepo *repositories.ScreenerRepository
	userRepo     *repositories.UserRepository
}

func NewScreenerHandler(screenerRepo *repositories.ScreenerRepository, userRepo *repositories.UserRepository) *ScreenerHandler {
	return &ScreenerHandler{
		screenerRepo: screenerRepo,
		userRepo:     userRepo,
	}
}

// SaveScreener saves a new screener
// POST /api/v1/screeners/save
func (h *ScreenerHandler) SaveScreener(c *gin.Context) {
	var req models.CreateScreenerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Get user_id from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	// Get org_id from user
	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to get user information",
		})
		return
	}

	// Superadmins don't have org_id, which is allowed
	// Regular users must have an org_id
	if !user.IsSuperAdmin && user.OrgID == nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "User organization not found",
		})
		return
	}

	// Clean and normalize optional fields - remove all quotes and whitespace
	cleanedUniverseList := h.cleanStringValue(req.UniverseList)
	cleanedTableName := h.cleanStringValue(req.TableName)
	trimmedExplainer := h.trimStringPointer(req.Explainer)

	// Check if screener with same name already exists for this user (regardless of is_active)
	existingScreener, err := h.screenerRepo.GetByName(c.Request.Context(), uid, req.ScreenerName)
	if err != nil && err != errors.ErrNotFound {
		log.Printf("Failed to check screener existence: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to check screener existence",
		})
		return
	}

	// If screener exists, toggle is_active and update data
	if existingScreener != nil {
		// Update the existing screener with new data and activate it
		existingScreener.TableName = cleanedTableName
		existingScreener.Query = req.Query
		existingScreener.UniverseList = cleanedUniverseList
		existingScreener.Explainer = trimmedExplainer
		existingScreener.IsActive = true

		if err := h.screenerRepo.Update(c.Request.Context(), existingScreener); err != nil {
			if appErr, ok := err.(*errors.AppError); ok {
				c.JSON(appErr.Status, errors.ErrorResponse{
					Error:   appErr.Code,
					Message: appErr.Message,
				})
				return
			}
			log.Printf("Failed to update screener: %v", err)
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: "Failed to update screener",
			})
			return
		}

		c.JSON(http.StatusOK, models.SaveScreenerResponse{
			Status: "success",
			Data:   existingScreener,
		})
		return
	}

	// Create new screener if it doesn't exist
	screener := &models.Screener{
		OrgID:        user.OrgID, // Can be nil for superadmins
		UserID:       uid,
		ScreenerName: req.ScreenerName,
		TableName:    cleanedTableName,
		Query:        req.Query,
		UniverseList: cleanedUniverseList,
		Explainer:    trimmedExplainer,
		IsActive:     true,
	}

	if err := h.screenerRepo.Create(c.Request.Context(), screener); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to save screener: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to save screener",
		})
		return
	}

	c.JSON(http.StatusCreated, models.SaveScreenerResponse{
		Status: "success",
		Data:   screener,
	})
}

// GetSavedScreeners gets all saved screeners for the authenticated user
// GET /api/v1/screeners/saved
func (h *ScreenerHandler) GetSavedScreeners(c *gin.Context) {
	// Get user_id from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	screeners, err := h.screenerRepo.ListByUser(c.Request.Context(), uid)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to fetch saved screeners: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to fetch saved screeners",
		})
		return
	}

	c.JSON(http.StatusOK, models.ListScreenersResponse{
		Status: "success",
		Data:   screeners,
	})
}

// DeleteScreener deletes a saved screener
// DELETE /api/v1/screeners/:id
func (h *ScreenerHandler) DeleteScreener(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid screener ID",
		})
		return
	}

	// Get user_id from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	if err := h.screenerRepo.Delete(c.Request.Context(), id, uid); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		log.Printf("Failed to delete screener: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to delete screener",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Screener deleted successfully",
	})
}

// RunSavedScreener runs a saved screener by fetching its data from DB and calling the external API
// POST /api/v1/screeners/:id/run
func (h *ScreenerHandler) RunSavedScreener(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid screener ID",
		})
		return
	}

	// Get user_id from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	uid, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Invalid user ID",
		})
		return
	}

	// Fetch screener from database
	screener, err := h.screenerRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == errors.ErrNotFound {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   errors.ErrNotFound.Code,
				Message: "Screener not found",
			})
			return
		}
		log.Printf("Failed to fetch screener: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to fetch screener",
		})
		return
	}

	// Verify screener belongs to the user
	if screener.UserID != uid {
		c.JSON(http.StatusForbidden, errors.ErrorResponse{
			Error:   errors.ErrForbidden.Code,
			Message: "You don't have permission to run this screener",
		})
		return
	}

	// Build the external API URL with parameters from database
	// The API requires properly URL-encoded query parameters
	baseURL := "http://internal-trading-internal-ALB-1101911574.ap-south-1.elb.amazonaws.com:80/tess/query"

	// Build query parameters using url.Values for proper URL encoding
	queryParams := url.Values{}
	queryParams.Set("query", screener.Query)

	// Add universeList with proper single quotes format
	// The database stores clean values without quotes, so we add them here
	if screener.UniverseList != nil && *screener.UniverseList != "" {
		universeValue := "'" + *screener.UniverseList + "'"
		queryParams.Set("universeList", universeValue)
	}

	// Add tableName with proper single quotes format
	// The database stores clean values without quotes, so we add them here
	if screener.TableName != nil && *screener.TableName != "" {
		tableValue := "'" + *screener.TableName + "'"
		queryParams.Set("tableName", tableValue)
	}

	// Build the full URL with properly encoded query parameters
	fullURL := baseURL + "?" + queryParams.Encode()
	log.Printf("Calling external API: %s", fullURL)

	// Create request with the properly encoded URL
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create request",
		})
		return
	}

	// Create HTTP client and execute request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to call external API: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to execute screener query",
		})
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read API response: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to read screener results",
		})
		return
	}

	// Check if external API returned an error
	if resp.StatusCode != http.StatusOK {
		log.Printf("External API returned error: %d - %s", resp.StatusCode, string(body))
		c.JSON(resp.StatusCode, gin.H{
			"status":  "error",
			"message": "External API error",
			"error":   string(body),
		})
		return
	}

	// Parse the response to validate it's valid JSON
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse API response: %v", err)
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to parse screener results",
		})
		return
	}

	// Return the external API response directly
	c.Data(http.StatusOK, "application/json", body)
}

// trimStringPointer trims whitespace from a string pointer and returns nil if the result is empty
func (h *ScreenerHandler) trimStringPointer(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// cleanStringValue removes all surrounding quotes (both single and double) and whitespace
// from a string value, returning just the clean content
func (h *ScreenerHandler) cleanStringValue(s *string) *string {
	if s == nil {
		return nil
	}

	cleaned := strings.TrimSpace(*s)

	// Keep removing outer quotes (both single and double) until none are left
	for len(cleaned) > 0 {
		firstChar := cleaned[0]
		lastChar := cleaned[len(cleaned)-1]

		// Check if surrounded by quotes (single or double)
		if (firstChar == '\'' || firstChar == '"') && (lastChar == '\'' || lastChar == '"') {
			cleaned = cleaned[1 : len(cleaned)-1]
			cleaned = strings.TrimSpace(cleaned)
		} else {
			break
		}
	}

	if cleaned == "" {
		return nil
	}

	return &cleaned
}
