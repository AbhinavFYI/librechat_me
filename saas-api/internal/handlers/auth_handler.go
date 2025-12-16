package handlers

import (
	"net/http"

	"saas-api/internal/middleware"
	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/internal/services"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authService *services.AuthService
	authMW      *middleware.AuthMiddleware
	orgRepo     *repositories.OrganizationRepository
}

func NewAuthHandler(authService *services.AuthService, authMW *middleware.AuthMiddleware, orgRepo *repositories.OrganizationRepository) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		authMW:      authMW,
		orgRepo:     orgRepo,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	ipAddress := h.authMW.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	response, err := h.authService.Login(c.Request.Context(), &req, ipAddress, userAgent)
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
			Message: "Login failed",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var refreshToken string

	// Try to get refresh token from JSON body first
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		refreshToken = req.RefreshToken
	} else {
		// Fallback: try to get from cookie
		cookie, err := c.Cookie("refreshToken")
		if err == nil && cookie != "" {
			refreshToken = cookie
		} else {
			// Also try "refresh_token" cookie name
			cookie, err = c.Cookie("refresh_token")
			if err == nil && cookie != "" {
				refreshToken = cookie
			}
		}
	}

	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "Refresh token is required. Provide it in request body or cookie.",
		})
		return
	}

	ipAddress := h.authMW.GetClientIP(c)

	response, err := h.authService.RefreshToken(c.Request.Context(), refreshToken, ipAddress)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			// Provide more detailed error message
			errorMessage := appErr.Message
			if appErr.Code == errors.ErrUnauthorized.Code {
				errorMessage = "Token refresh failed: " + appErr.Message
			}
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: errorMessage,
			})
			return
		}
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "Token refresh failed: Invalid or expired refresh token",
		})
		return
	}

	// Set cookies for the new tokens
	c.SetCookie("access_token", response.AccessToken, int(response.ExpiresIn), "/", "", false, true)
	if response.RefreshToken != "" {
		c.SetCookie("refreshToken", response.RefreshToken, 30*24*60*60, "/", "", false, true) // 30 days
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	var req models.RefreshTokenRequest
	refreshToken := ""
	if c.ShouldBindJSON(&req) == nil {
		refreshToken = req.RefreshToken
	}

	uid, _ := uuid.Parse(userID.(string))
	err := h.authService.Logout(c.Request.Context(), refreshToken, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Logout failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "User not authenticated",
		})
		return
	}

	// Get user details from service
	// This would require a user service method
	orgID, _ := c.Get("org_id")
	isSuperAdmin, _ := c.Get("is_super_admin")

	response := gin.H{
		"user_id":        userID,
		"email":          c.GetString("email"),
		"org_id":         orgID,
		"is_super_admin": isSuperAdmin,
	}

	// If user has an organization, fetch organization details including logo
	if orgID != nil && orgID != "" {
		orgUUID, err := uuid.Parse(orgID.(string))
		if err == nil {
			org, err := h.orgRepo.GetByID(c.Request.Context(), orgUUID)
			if err == nil && org != nil {
				response["org_name"] = org.Name
				if org.LogoURL != nil {
					response["org_logo_url"] = *org.LogoURL
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// SendOTP sends OTP to user's email
func (h *AuthHandler) SendOTP(c *gin.Context) {
	var req models.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.SendOTP(c.Request.Context(), req.Email)
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
			Message: "Failed to send OTP",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// VerifyOTP verifies OTP and returns login tokens
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req models.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	ipAddress := h.authMW.GetClientIP(c)
	userAgent := c.GetHeader("User-Agent")

	response, err := h.authService.VerifyOTP(c.Request.Context(), req.Email, req.OTP, ipAddress, userAgent)
	if err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			c.JSON(appErr.Status, errors.ErrorResponse{
				Error:   appErr.Code,
				Message: appErr.Message,
			})
			return
		}
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "OTP verification failed",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ResendOTP resends OTP to user's email
func (h *AuthHandler) ResendOTP(c *gin.Context) {
	var req models.ResendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	response, err := h.authService.ResendOTP(c.Request.Context(), req.Email)
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
			Message: "Failed to resend OTP",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
