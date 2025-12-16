package middleware

import (
	"context"
	"fmt"

	"saas-api/internal/database"

	"github.com/gin-gonic/gin"
)

type RLSMiddleware struct {
	db *database.DB
}

func NewRLSMiddleware(db *database.DB) *RLSMiddleware {
	return &RLSMiddleware{db: db}
}

func (m *RLSMiddleware) SetRLSContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		orgID, _ := c.Get("org_id")
		isSuperAdmin, _ := c.Get("is_super_admin")

		// Set PostgreSQL session variables for RLS
		ctx := c.Request.Context()
		
		// Execute SET commands for RLS in a transaction context
		// Note: RLS variables need to be set per-connection
		// This is a simplified version - in production, you may want to use connection pooling with context
		if userID != nil {
			ctx = context.WithValue(ctx, "app.current_user_id", fmt.Sprintf("%v", userID))
		}
		if orgID != nil && orgID != "" {
			ctx = context.WithValue(ctx, "app.current_org_id", fmt.Sprintf("%v", orgID))
		}
		if isSuperAdmin != nil {
			ctx = context.WithValue(ctx, "app.is_super_admin", fmt.Sprintf("%v", isSuperAdmin))
		}

		// Note: Actual RLS variable setting should be done at the connection level
		// For now, we're storing in context for potential use
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

