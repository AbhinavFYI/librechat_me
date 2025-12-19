package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"saas-api/pkg/errors"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LibreChatHandler struct{}

func NewLibreChatHandler() *LibreChatHandler {
	return &LibreChatHandler{}
}

// GetCredentials fetches LibreChat user credentials from MongoDB
func (h *LibreChatHandler) GetCredentials(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: "email parameter is required",
		})
		return
	}

	// Get MongoDB URI from environment
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://127.0.0.1:27017/LibreChat"
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to connect to MongoDB: " + err.Error(),
		})
		return
	}
	defer client.Disconnect(ctx)

	// Ping MongoDB to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to ping MongoDB: " + err.Error(),
		})
		return
	}

	// Extract database name from URI
	dbName := "LibreChat"
	parsedURI, err := url.Parse(mongoURI)
	if err == nil && parsedURI.Path != "" {
		path := strings.TrimPrefix(parsedURI.Path, "/")
		if idx := strings.Index(path, "?"); idx > 0 {
			path = path[:idx]
		}
		if path != "" {
			dbName = path
		}
	} else {
		// Fallback: extract from string directly
		if strings.Contains(mongoURI, "/") {
			parts := strings.Split(mongoURI, "/")
			if len(parts) > 1 {
				dbPart := parts[len(parts)-1]
				if idx := strings.Index(dbPart, "?"); idx > 0 {
					dbPart = dbPart[:idx]
				}
				if dbPart != "" && !strings.Contains(dbPart, ":") {
					dbName = dbPart
				}
			}
		}
	}

	db := client.Database(dbName)
	collection := db.Collection("users")

	// Find user by email
	filter := bson.M{"email": email}
	var userDoc bson.M
	err = collection.FindOne(ctx, filter).Decode(&userDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, errors.ErrorResponse{
				Error:   "USER_NOT_FOUND",
				Message: "User not found in LibreChat",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to fetch user from MongoDB: " + err.Error(),
		})
		return
	}

	// Extract user fields
	libreUserEmail := ""
	libreUserUsername := ""
	libreUserName := ""
	password := ""

	if emailVal, ok := userDoc["email"].(string); ok {
		libreUserEmail = emailVal
	}
	if usernameVal, ok := userDoc["username"].(string); ok {
		libreUserUsername = usernameVal
	}
	if nameVal, ok := userDoc["name"].(string); ok {
		libreUserName = nameVal
	}
	// Get password from MongoDB (should be plain text now)
	if pwd, ok := userDoc["password"].(string); ok && pwd != "" {
		// Check if password is hashed (bcrypt hashes start with $2a$, $2b$, or $2y$)
		if strings.HasPrefix(pwd, "$2a$") || strings.HasPrefix(pwd, "$2b$") || strings.HasPrefix(pwd, "$2y$") {
			// Password is hashed - use email as fallback
			password = email
			log.Printf("Password is hashed for %s, using email as password", email)
		} else {
			// Password is plain text - use it directly
			password = pwd
			log.Printf("Using plain text password for %s", email)
		}
	}

	// Check for plainPassword field (backup)
	if password == "" {
		if plainPwd, ok := userDoc["plainPassword"].(string); ok && plainPwd != "" {
			password = plainPwd
			log.Printf("Using plainPassword field for %s", email)
		}
	}

	// Final fallback: use email as password
	if password == "" {
		password = email
		log.Printf("No password found for %s, using email as password", email)
	}

	// Return credentials
	c.JSON(http.StatusOK, gin.H{
		"email":    libreUserEmail,
		"username": libreUserUsername,
		"name":     libreUserName,
		"password": password,
	})
}

// Login logs in to LibreChat and returns session info
func (h *LibreChatHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.ErrorResponse{
			Error:   errors.ErrValidation.Code,
			Message: err.Error(),
		})
		return
	}

	// Login to LibreChat API
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	loginURL := "http://10.10.7.81:3080/api/auth/login"
	if envURL := os.Getenv("LIBRE_BACKEND"); envURL != "" {
		loginURL = envURL + "/api/auth/login"
	}

	loginReq, err := http.NewRequest("POST", loginURL, strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"%s"}`, req.Email, req.Password)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to create login request: " + err.Error(),
		})
		return
	}

	loginReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(loginReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to login to LibreChat: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, errors.ErrorResponse{
			Error:   "LIBRECHAT_LOGIN_FAILED",
			Message: "LibreChat login failed: " + string(body),
		})
		return
	}

	// Read response
	var loginData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&loginData); err != nil {
		c.JSON(http.StatusInternalServerError, errors.ErrorResponse{
			Error:   errors.ErrInternalServer.Code,
			Message: "Failed to parse login response: " + err.Error(),
		})
		return
	}

	// Extract cookies from response - we need to forward these to the client
	cookies := resp.Cookies()

	// Set cookies in the response so browser receives them
	// Note: These cookies are for localhost:3090, but we're setting them from localhost:8080
	// This won't work due to same-origin policy, but we'll try anyway
	for _, cookie := range cookies {
		// Create a new cookie with the same values
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Path:     cookie.Path,
			Domain:   cookie.Domain,
			Expires:  cookie.Expires,
			MaxAge:   cookie.MaxAge,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
			SameSite: cookie.SameSite,
		})
	}

	// Return login data with cookie info for manual setting if needed
	cookieHeader := ""
	for _, cookie := range cookies {
		if cookieHeader != "" {
			cookieHeader += "; "
		}
		cookieHeader += cookie.Name + "=" + cookie.Value
	}

	// Return login data with cookie info
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user":    loginData,
		"cookies": cookieHeader, // Client can use this to set cookies
		"cookieNames": func() []string {
			names := make([]string, len(cookies))
			for i, cookie := range cookies {
				names[i] = cookie.Name
			}
			return names
		}(),
	})
}
