package middleware

import (
	"context"
	"net/http"
	"strings"

	"saas-api/internal/auth"
	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	tokenService *auth.TokenService
}

func NewAuthMiddleware(tokenService *auth.TokenService) *AuthMiddleware {
	return &AuthMiddleware{tokenService: tokenService}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
				Error:   errors.ErrUnauthorized.Code,
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		tokenString, err := m.tokenService.ExtractTokenFromHeader(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
				Error:   errors.ErrUnauthorized.Code,
				Message: err.Error(),
			})
			c.Abort()
			return
		}

		claims, err := m.tokenService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
				Error:   errors.ErrUnauthorized.Code,
				Message: "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID.String())
		c.Set("email", claims.Email)
		if claims.OrgID != nil {
			c.Set("org_id", claims.OrgID.String())
		}
		c.Set("is_super_admin", claims.IsSuperAdmin)

		// Set RLS context variables for PostgreSQL
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, "user_id", claims.UserID.String())
		ctx = context.WithValue(ctx, "org_id", "")
		if claims.OrgID != nil {
			ctx = context.WithValue(ctx, "org_id", claims.OrgID.String())
		}
		ctx = context.WithValue(ctx, "is_super_admin", claims.IsSuperAdmin)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequireAuthForStatic is similar to RequireAuth but also accepts tokens from query parameters or cookies
// This is needed for static file requests that can't send Authorization headers (e.g., window.open, img src)
func (m *AuthMiddleware) RequireAuthForStatic() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string
		var err error

		// Try to get token from Authorization header first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString, err = m.tokenService.ExtractTokenFromHeader(authHeader)
			if err == nil && tokenString != "" {
				// Token found in header, validate it
				claims, err := m.tokenService.ValidateToken(tokenString)
				if err == nil {
					// Set user context
					c.Set("user_id", claims.UserID.String())
					c.Set("email", claims.Email)
					if claims.OrgID != nil {
						c.Set("org_id", claims.OrgID.String())
					}
					c.Set("is_super_admin", claims.IsSuperAdmin)

					// Set RLS context variables for PostgreSQL
					ctx := c.Request.Context()
					ctx = context.WithValue(ctx, "user_id", claims.UserID.String())
					ctx = context.WithValue(ctx, "org_id", "")
					if claims.OrgID != nil {
						ctx = context.WithValue(ctx, "org_id", claims.OrgID.String())
					}
					ctx = context.WithValue(ctx, "is_super_admin", claims.IsSuperAdmin)
					c.Request = c.Request.WithContext(ctx)

					c.Next()
					return
				}
			}
		}

		// Try to get token from query parameter (for browser requests)
		tokenString = c.Query("token")
		if tokenString != "" {
			// Remove "Bearer " prefix if present
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
			tokenString = strings.TrimSpace(tokenString)

			claims, err := m.tokenService.ValidateToken(tokenString)
			if err == nil {
				// Set user context
				c.Set("user_id", claims.UserID.String())
				c.Set("email", claims.Email)
				if claims.OrgID != nil {
					c.Set("org_id", claims.OrgID.String())
				}
				c.Set("is_super_admin", claims.IsSuperAdmin)

				// Set RLS context variables for PostgreSQL
				ctx := c.Request.Context()
				ctx = context.WithValue(ctx, "user_id", claims.UserID.String())
				ctx = context.WithValue(ctx, "org_id", "")
				if claims.OrgID != nil {
					ctx = context.WithValue(ctx, "org_id", claims.OrgID.String())
				}
				ctx = context.WithValue(ctx, "is_super_admin", claims.IsSuperAdmin)
				c.Request = c.Request.WithContext(ctx)

				c.Next()
				return
			}
		}

		// Try to get token from cookie
		cookie, err := c.Cookie("access_token")
		if err == nil && cookie != "" {
			// Remove "Bearer " prefix if present
			tokenString = strings.TrimPrefix(cookie, "Bearer ")
			tokenString = strings.TrimSpace(tokenString)

			claims, err := m.tokenService.ValidateToken(tokenString)
			if err == nil {
				// Set user context
				c.Set("user_id", claims.UserID.String())
				c.Set("email", claims.Email)
				if claims.OrgID != nil {
					c.Set("org_id", claims.OrgID.String())
				}
				c.Set("is_super_admin", claims.IsSuperAdmin)

				// Set RLS context variables for PostgreSQL
				ctx := c.Request.Context()
				ctx = context.WithValue(ctx, "user_id", claims.UserID.String())
				ctx = context.WithValue(ctx, "org_id", "")
				if claims.OrgID != nil {
					ctx = context.WithValue(ctx, "org_id", claims.OrgID.String())
				}
				ctx = context.WithValue(ctx, "is_super_admin", claims.IsSuperAdmin)
				c.Request = c.Request.WithContext(ctx)

				c.Next()
				return
			}
		}

		// No valid token found
		c.JSON(http.StatusUnauthorized, errors.ErrorResponse{
			Error:   errors.ErrUnauthorized.Code,
			Message: "Authorization required. Provide token via Authorization header, 'token' query parameter, or 'access_token' cookie.",
		})
		c.Abort()
	}
}

func (m *AuthMiddleware) RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isSuperAdmin, exists := c.Get("is_super_admin")
		if !exists || !isSuperAdmin.(bool) {
			c.JSON(http.StatusForbidden, errors.ErrorResponse{
				Error:   errors.ErrForbidden.Code,
				Message: "Super admin access required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *AuthMiddleware) GetClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		ips := strings.Split(ip, ",")
		return strings.TrimSpace(ips[0])
	}
	ip = c.GetHeader("X-Real-IP")
	if ip != "" {
		return ip
	}
	return c.ClientIP()
}
