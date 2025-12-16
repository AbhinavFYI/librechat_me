package middleware

import (
	"log"
	"net/http"

	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
)

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			
			// Check if it's an AppError
			if appErr, ok := err.Err.(*errors.AppError); ok {
				c.JSON(appErr.Status, errors.ErrorResponse{
					Error:   appErr.Code,
					Message: appErr.Message,
				})
				return
			}

			// Generic error
			log.Printf("Error: %v", err)
			c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
				Error:   errors.ErrInternalServer.Code,
				Message: "Internal server error",
			})
		}
	}
}

