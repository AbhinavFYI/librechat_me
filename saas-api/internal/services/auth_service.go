package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"saas-api/config"
	"saas-api/internal/auth"
	"saas-api/internal/models"
	"saas-api/internal/repositories"
	"saas-api/pkg/errors"
	"saas-api/pkg/utils"

	"github.com/google/uuid"
)

type AuthService struct {
	userRepo     *repositories.UserRepository
	tokenRepo    *repositories.RefreshTokenRepository
	tokenService *auth.TokenService
	config       *config.Config
}

func NewAuthService(
	userRepo *repositories.UserRepository,
	tokenRepo *repositories.RefreshTokenRepository,
	tokenService *auth.TokenService,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		tokenService: tokenService,
		config:       cfg,
	}
}

// canLoginViaOTP checks if a user can login via OTP
// Returns true if user is:
// - Super admin (if status is active), OR
// - Has org_role = "admin" (if status is active), OR
// - Is verified (email_verified = true) AND active (status = "active")
// Users with status "suspended" or "pending" cannot login (except super admins with active status)
// OrgRole (admin/user/viewer) is separate from Roles (permissions)
func (s *AuthService) canLoginViaOTP(ctx context.Context, user *models.User) (bool, error) {
	// Check user status first - suspended and pending users cannot login
	if user.Status == "suspended" {
		log.Printf("canLoginViaOTP: User %s is suspended and cannot login", user.Email)
		return false, errors.NewError("ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact your administrator.", 403)
	}

	if user.Status == "pending" {
		log.Printf("canLoginViaOTP: User %s is pending and cannot login", user.Email)
		return false, errors.NewError("ACCOUNT_PENDING", "Your account is pending approval. Please contact your administrator.", 403)
	}

	// Only active users can proceed
	if user.Status != "active" {
		log.Printf("canLoginViaOTP: User %s has invalid status: %s", user.Email, user.Status)
		return false, errors.NewError("ACCOUNT_INACTIVE", "Your account is not active. Please contact your administrator.", 403)
	}

	// Super admins can login via OTP (if status is active)
	if user.IsSuperAdmin {
		log.Printf("canLoginViaOTP: User %s is super admin with active status", user.Email)
		return true, nil
	}

	// Check if user has org_role = "admin" (login eligibility is based on OrgRole, not Role)
	if user.OrgRole != nil && *user.OrgRole == "admin" {
		log.Printf("canLoginViaOTP: User %s has org_role = admin with active status", user.Email)
		return true, nil
	}

	// Check if user is verified and active
	if user.EmailVerified {
		log.Printf("canLoginViaOTP: User %s is verified and active", user.Email)
		return true, nil
	}

	log.Printf("canLoginViaOTP: User %s is not authorized to login via OTP (not verified)", user.Email)
	return false, errors.NewError("ACCOUNT_NOT_VERIFIED", "Your account is not verified. Please verify your email.", 403)
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest, ipAddress, userAgent string) (*models.LoginResponse, error) {
	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// Log the error for debugging (remove in production)
		log.Printf("Login failed - GetByEmail error for %s: %v", req.Email, err)
		// Return unauthorized for both user not found and other errors
		// to prevent user enumeration attacks
		return nil, errors.ErrUnauthorized
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, errors.NewError("ACCOUNT_LOCKED", "Account is locked due to failed login attempts", 423)
	}

	// Verify password
	passwordValid := utils.CheckPasswordHash(req.Password, user.PasswordHash)
	if !passwordValid {
		log.Printf("Login failed - Password mismatch for user %s", req.Email)
		_ = s.userRepo.IncrementFailedLoginAttempts(ctx, user.ID)
		return nil, errors.ErrUnauthorized
	}

	// Check user status - suspended and pending users cannot login
	if user.Status == "suspended" {
		log.Printf("Login failed - User %s is suspended", req.Email)
		return nil, errors.NewError("ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact your administrator.", 403)
	}

	if user.Status == "pending" {
		log.Printf("Login failed - User %s is pending approval", req.Email)
		return nil, errors.NewError("ACCOUNT_PENDING", "Your account is pending approval. Please contact your administrator.", 403)
	}

	// Only active users can login
	if user.Status != "active" {
		log.Printf("Login failed - User %s has invalid status: %s", req.Email, user.Status)
		return nil, errors.NewError("ACCOUNT_INACTIVE", "Your account is not active. Please contact your administrator.", 403)
	}

	// Update login info
	_ = s.userRepo.UpdateLoginInfo(ctx, user.ID, ipAddress)

	// Generate tokens
	accessToken, err := s.tokenService.GenerateAccessToken(user)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate access token", errors.ErrInternalServer.Status)
	}

	refreshToken, err := s.tokenService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate refresh token", errors.ErrInternalServer.Status)
	}

	// Store refresh token
	tokenHash := utils.HashToken(refreshToken)
	refreshTokenModel := &models.RefreshToken{
		ID:         uuid.New(),
		UserID:     user.ID,
		TokenHash:  tokenHash,
		DeviceInfo: map[string]interface{}{"user_agent": userAgent},
		IPAddress:  &ipAddress,
		UserAgent:  &userAgent,
		ExpiresAt:  time.Now().Add(time.Duration(s.config.JWT.RefreshTokenTTL) * 24 * time.Hour),
	}

	if err := s.tokenRepo.Create(ctx, refreshTokenModel); err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to store refresh token", errors.ErrInternalServer.Status)
	}

	// Clear password hash from response
	user.PasswordHash = ""

	// Get user permissions
	permissions, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		log.Printf("Warning: Failed to get user permissions for %s: %v", user.Email, err)
		permissions = []*models.Permission{} // Return empty permissions on error
	}

	return &models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    s.config.JWT.AccessTokenTTL * 60, // seconds
		User:         user,
		Permissions:  convertPermissions(permissions),
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string, ipAddress string) (*models.LoginResponse, error) {
	// Trim whitespace from token (in case of encoding issues)
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		log.Printf("RefreshToken: Empty refresh token provided")
		return nil, errors.NewError("INVALID_TOKEN", "Refresh token is required", 401)
	}

	// Validate refresh token JWT first
	claims, err := s.tokenService.ValidateToken(refreshToken)
	if err != nil {
		log.Printf("RefreshToken: JWT validation failed: %v", err)
		return nil, errors.NewError("INVALID_TOKEN", "Invalid refresh token: JWT validation failed", 401)
	}

	// Calculate hash and lookup in database
	tokenHash := utils.HashToken(refreshToken)
	storedToken, err := s.tokenRepo.GetByHash(ctx, tokenHash)
	if err != nil {
		// Token not found in database
		log.Printf("RefreshToken: Token not found in database for userID: %s, error: %v", claims.UserID, err)
		return nil, errors.NewError("TOKEN_NOT_FOUND", "Refresh token not found or has been revoked", 401)
	}

	// Check if token is expired
	if storedToken.ExpiresAt.Before(time.Now()) {
		log.Printf("RefreshToken: Token expired for userID: %s, expired at: %v, now: %v",
			claims.UserID, storedToken.ExpiresAt, time.Now())
		return nil, errors.NewError("TOKEN_EXPIRED", "Refresh token has expired. Please login again", 401)
	}

	// Check if token is revoked
	if storedToken.RevokedAt != nil {
		log.Printf("RefreshToken: Token has been revoked for userID: %s", claims.UserID)
		return nil, errors.NewError("TOKEN_REVOKED", "Refresh token has been revoked", 401)
	}

	// Verify the token belongs to the user from JWT claims
	if storedToken.UserID != claims.UserID {
		log.Printf("RefreshToken: User ID mismatch - stored: %s, claims: %s", storedToken.UserID, claims.UserID)
		return nil, errors.NewError("TOKEN_MISMATCH", "Refresh token does not match user", 401)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		log.Printf("RefreshToken: User not found: %s, error: %v", claims.UserID, err)
		return nil, errors.NewError("USER_NOT_FOUND", "User associated with token not found", 401)
	}

	// Check user status - suspended and pending users cannot refresh tokens
	if user.Status == "suspended" {
		log.Printf("RefreshToken: User %s is suspended", user.Email)
		return nil, errors.NewError("ACCOUNT_SUSPENDED", "Your account has been suspended. Please contact your administrator.", 403)
	}

	if user.Status == "pending" {
		log.Printf("RefreshToken: User %s is pending approval", user.Email)
		return nil, errors.NewError("ACCOUNT_PENDING", "Your account is pending approval. Please contact your administrator.", 403)
	}

	// Only active users can refresh tokens
	if user.Status != "active" {
		log.Printf("RefreshToken: User status is not active: %s (status: %s)", user.Email, user.Status)
		return nil, errors.NewError("ACCOUNT_INACTIVE", "Your account is not active. Please contact your administrator.", 403)
	}

	// Update last used
	_ = s.tokenRepo.UpdateLastUsed(ctx, storedToken.ID)

	// Generate new access token
	accessToken, err := s.tokenService.GenerateAccessToken(user)
	if err != nil {
		log.Printf("RefreshToken: Failed to generate access token for user %s: %v", user.Email, err)
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate access token", errors.ErrInternalServer.Status)
	}

	// Clear password hash
	user.PasswordHash = ""

	// Get user permissions
	permissions, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		log.Printf("Warning: Failed to get user permissions for %s: %v", user.Email, err)
		permissions = []*models.Permission{} // Return empty permissions on error
	}

	return &models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken, // Return same refresh token
		TokenType:    "Bearer",
		ExpiresIn:    s.config.JWT.AccessTokenTTL * 60,
		User:         user,
		Permissions:  convertPermissions(permissions),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string, userID uuid.UUID) error {
	if refreshToken != "" {
		tokenHash := utils.HashToken(refreshToken)
		storedToken, err := s.tokenRepo.GetByHash(ctx, tokenHash)
		if err == nil {
			_ = s.tokenRepo.Revoke(ctx, storedToken.ID, userID, "User logged out")
		}
	} else {
		// Revoke all tokens for user
		_ = s.tokenRepo.RevokeAllForUser(ctx, userID, userID)
	}
	return nil
}

func (s *AuthService) GetClientIP(r interface{}) string {
	// This will be implemented in middleware
	return ""
}

// SendOTP sends OTP to user's email
func (s *AuthService) SendOTP(ctx context.Context, email string) (*models.SendOTPResponse, error) {
	// Check if user exists
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		log.Printf("SendOTP: User not found for email %s: %v", email, err)
		// Return error to indicate user doesn't exist (for super admin check)
		return nil, errors.NewError("USER_NOT_FOUND", "User not found", 404)
	}

	log.Printf("SendOTP: User found - Email: %s, IsSuperAdmin: %v, Status: %s, EmailVerified: %v", email, user.IsSuperAdmin, user.Status, user.EmailVerified)

	// Check if user can login via OTP (super admin, org admin, or verified+active)
	canLogin, err := s.canLoginViaOTP(ctx, user)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to check user permissions", errors.ErrInternalServer.Status)
	}
	if !canLogin {
		log.Printf("SendOTP: User %s is not authorized to login via OTP", email)
		return nil, errors.NewError("UNAUTHORIZED", "Only super admins, organization admins, or verified active users can login via OTP", 403)
	}

	// Generate OTP
	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate OTP", errors.ErrInternalServer.Status)
	}

	// Set OTP in database
	expiresAt := utils.GetOTPExpiryTime()
	if err := s.userRepo.SetOTP(ctx, user.ID, otp, expiresAt); err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to store OTP", errors.ErrInternalServer.Status)
	}

	// Send OTP via email
	log.Printf("SendOTP: Attempting to send OTP email to %s", email)
	if err := utils.SendOTPEmail(email, otp); err != nil {
		log.Printf("SendOTP: Failed to send OTP email to %s", email)
		log.Printf("SendOTP: Error details: %v", err)
		log.Printf("SendOTP: OTP generated was: %s (stored in database)", otp)
		// Return a more descriptive error
		return nil, errors.WrapError(err, "EMAIL_SEND_FAILED", fmt.Sprintf("Failed to send OTP email: %v", err), errors.ErrInternalServer.Status)
	}
	log.Printf("SendOTP: OTP email sent successfully to %s", email)

	return &models.SendOTPResponse{
		Message: "OTP sent to your email",
		Email:   email,
	}, nil
}

// VerifyOTP verifies OTP and returns login tokens
func (s *AuthService) VerifyOTP(ctx context.Context, email, otp, ipAddress, userAgent string) (*models.LoginResponse, error) {
	// Verify OTP (this will increment attempts if invalid)
	if err := s.userRepo.VerifyOTP(ctx, email, otp); err != nil {
		if appErr, ok := err.(*errors.AppError); ok {
			return nil, appErr
		}
		return nil, errors.NewError("INVALID_OTP", "Invalid OTP", 401)
	}

	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, errors.ErrUnauthorized
	}

	// Check if user can login via OTP (super admin, org admin, or verified+active)
	canLogin, err := s.canLoginViaOTP(ctx, user)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to check user permissions", errors.ErrInternalServer.Status)
	}
	if !canLogin {
		log.Printf("VerifyOTP: User %s is not authorized to login via OTP", email)
		return nil, errors.NewError("UNAUTHORIZED", "Only super admins, organization admins, or verified active users can login via OTP", 403)
	}

	// Update login info
	_ = s.userRepo.UpdateLoginInfo(ctx, user.ID, ipAddress)

	// Generate tokens
	accessToken, err := s.tokenService.GenerateAccessToken(user)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate access token", errors.ErrInternalServer.Status)
	}

	refreshToken, err := s.tokenService.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate refresh token", errors.ErrInternalServer.Status)
	}

	// Store refresh token
	tokenHash := utils.HashToken(refreshToken)
	refreshTokenModel := &models.RefreshToken{
		ID:         uuid.New(),
		UserID:     user.ID,
		TokenHash:  tokenHash,
		DeviceInfo: map[string]interface{}{"user_agent": userAgent},
		IPAddress:  &ipAddress,
		UserAgent:  &userAgent,
		ExpiresAt:  time.Now().Add(time.Duration(s.config.JWT.RefreshTokenTTL) * 24 * time.Hour),
	}

	if err := s.tokenRepo.Create(ctx, refreshTokenModel); err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to store refresh token", errors.ErrInternalServer.Status)
	}

	// Clear password hash from response
	user.PasswordHash = ""

	// Get user permissions
	permissions, err := s.userRepo.GetUserPermissions(ctx, user.ID)
	if err != nil {
		log.Printf("Warning: Failed to get user permissions for %s: %v", user.Email, err)
		permissions = []*models.Permission{} // Return empty permissions on error
	}

	return &models.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    s.config.JWT.AccessTokenTTL * 60, // seconds
		User:         user,
		Permissions:  convertPermissions(permissions),
	}, nil
}

// convertPermissions converts []*models.Permission to []models.Permission
func convertPermissions(perms []*models.Permission) []models.Permission {
	result := make([]models.Permission, len(perms))
	for i, p := range perms {
		result[i] = *p
	}
	return result
}

// ResendOTP resends OTP to user's email
func (s *AuthService) ResendOTP(ctx context.Context, email string) (*models.SendOTPResponse, error) {
	// Check if user exists
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists or not (security)
		return &models.SendOTPResponse{
			Message: "OTP resent to your email",
			Email:   email,
		}, nil
	}

	// Check if user can login via OTP (super admin, org admin, or verified+active)
	canLogin, err := s.canLoginViaOTP(ctx, user)
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to check user permissions", errors.ErrInternalServer.Status)
	}
	if !canLogin {
		log.Printf("ResendOTP: User %s is not authorized to login via OTP", email)
		return nil, errors.NewError("UNAUTHORIZED", "Only super admins, organization admins, or verified active users can login via OTP", 403)
	}

	// Generate new OTP
	otp, err := utils.GenerateOTP()
	if err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to generate OTP", errors.ErrInternalServer.Status)
	}

	// Set OTP in database
	expiresAt := utils.GetOTPExpiryTime()
	if err := s.userRepo.SetOTP(ctx, user.ID, otp, expiresAt); err != nil {
		return nil, errors.WrapError(err, "INTERNAL_ERROR", "Failed to store OTP", errors.ErrInternalServer.Status)
	}

	// Send OTP via email
	if err := utils.SendOTPEmail(email, otp); err != nil {
		log.Printf("Failed to send OTP email to %s: %v", email, err)
		// Still return success to prevent revealing email issues
	}

	return &models.SendOTPResponse{
		Message: "OTP resent to your email",
		Email:   email,
	}, nil
}
