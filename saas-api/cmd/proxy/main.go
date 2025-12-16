package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var jwtSecret []byte
var libreJWTSecret []byte
var libreJWTRefreshSecret []byte
var libreBackend string
var libreFrontend string
var cookieName = "libre_jwt"
var mongoURI string
var mainAPIURL string

// LibreChat User struct for MongoDB
type LibreChatUser struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Name             string             `bson:"name" json:"name"`
	Username         string             `bson:"username" json:"username"`
	Email            string             `bson:"email" json:"email"`
	EmailVerified    bool               `bson:"emailVerified" json:"emailVerified"`
	Avatar           *string            `bson:"avatar,omitempty" json:"avatar"`
	Provider         string             `bson:"provider" json:"provider"`
	Role             string             `bson:"role" json:"role"`
	Plugins          []interface{}      `bson:"plugins" json:"plugins"`
	TwoFactorEnabled bool               `bson:"twoFactorEnabled" json:"twoFactorEnabled"`
	TermsAccepted    bool               `bson:"termsAccepted" json:"termsAccepted"`
	Personalization  bson.M             `bson:"personalization" json:"personalization"`
	BackupCodes      []interface{}      `bson:"backupCodes" json:"backupCodes"`
	RefreshToken     []interface{}      `bson:"refreshToken" json:"refreshToken"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
	Version          int                `bson:"__v" json:"__v"`
}

// Our API User struct (partial)
type APIUser struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	FirstName     *string   `json:"first_name,omitempty"`
	LastName      *string   `json:"last_name,omitempty"`
	FullName      string    `json:"full_name"`
	AvatarURL     *string   `json:"avatar_url,omitempty"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func getLibreBackend() string {
	if target := os.Getenv("LIBRE_BACKEND"); target != "" {
		return target
	}
	return "http://localhost:3080" // Default LibreChat backend port
}

func getLibreFrontend() string {
	if target := os.Getenv("LIBRE_FRONTEND"); target != "" {
		return target
	}
	return "http://localhost:3090" // Default LibreChat frontend port
}

func getMongoURI() string {
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		return uri
	}
	return "mongodb://localhost:27017/LibreChat" // Default MongoDB URI
}

func getMainAPIURL() string {
	if url := os.Getenv("MAIN_API_URL"); url != "" {
		return url
	}
	return "http://localhost:8080" // Default main API URL
}

func init() {
	// Try multiple paths to find .env file
	envPaths := []string{
		"../.env",    // From cmd/proxy/ to saas-api/.env
		"../../.env", // From cmd/proxy/ to root/.env
		".env",       // Current directory
		"/Users/aep014/Desktop/custotm_librechat/saas-api/.env", // Absolute path
	}

	envLoaded := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			log.Printf("✅ Loaded .env from: %s", path)
			envLoaded = true
			break
		}
	}

	if !envLoaded {
		log.Printf("⚠️  No .env file found, trying to read from environment or set defaults")
		// If .env not found, try to set from known values
		if os.Getenv("LIBRE_JWT_SECRET") == "" {
			// Try to read from LibreChat's .env if available
			if err := godotenv.Load("../../InstiLibreChat/.env"); err == nil {
				log.Printf("✅ Loaded LibreChat .env, copying JWT secrets")
			}
		}
	}

	// Read environment variables after loading .env
	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
	libreJWTSecret = []byte(os.Getenv("LIBRE_JWT_SECRET"))
	libreJWTRefreshSecret = []byte(os.Getenv("LIBRE_JWT_REFRESH_SECRET"))

	// If still not set, try reading from JWT_SECRET and JWT_REFRESH_SECRET (LibreChat's naming)
	if len(libreJWTSecret) == 0 {
		libreJWTSecret = []byte(os.Getenv("JWT_SECRET"))
	}
	if len(libreJWTRefreshSecret) == 0 {
		libreJWTRefreshSecret = []byte(os.Getenv("JWT_REFRESH_SECRET"))
	}

	mongoURI = getMongoURI()
	libreBackend = getLibreBackend()
	libreFrontend = getLibreFrontend()
	mainAPIURL = getMainAPIURL()

	if len(jwtSecret) == 0 {
		jwtSecret = []byte("mysecret123") // fallback for development
		log.Println("Warning: Using default JWT secret. Set JWT_SECRET environment variable in production.")
	}

	// If LibreChat JWT secrets are still not set, use the known values from LibreChat's .env
	// These MUST match LibreChat's JWT_SECRET and JWT_REFRESH_SECRET
	if len(libreJWTSecret) == 0 {
		log.Println("WARNING: LIBRE_JWT_SECRET not found in .env - using hardcoded fallback")
		libreJWTSecret = []byte("16f8c0ef4a5d391b26034086c628469d3f9f497f08163ab9b40137092f2909ef")
		log.Println("⚠️  Using hardcoded LIBRE_JWT_SECRET - ensure this matches LibreChat's JWT_SECRET")
	}
	if len(libreJWTRefreshSecret) == 0 {
		log.Println("WARNING: LIBRE_JWT_REFRESH_SECRET not found in .env - using hardcoded fallback")
		libreJWTRefreshSecret = []byte("eaa5191f2914e30b9387fd84e254e4ba6fc51b4654968a9b0803b456a54b8418")
		log.Println("⚠️  Using hardcoded LIBRE_JWT_REFRESH_SECRET - ensure this matches LibreChat's JWT_REFRESH_SECRET")
	}

	log.Printf("LibreChat backend: %s\n", libreBackend)
	log.Printf("LibreChat frontend: %s\n", libreFrontend)
	log.Printf("MongoDB URI: %s\n", mongoURI)
	log.Printf("Main API URL: %s\n", mainAPIURL)
	if len(libreJWTSecret) > 0 {
		log.Printf("LIBRE_JWT_SECRET: ✅ Set (length: %d)", len(libreJWTSecret))
	}
	if len(libreJWTRefreshSecret) > 0 {
		log.Printf("LIBRE_JWT_REFRESH_SECRET: ✅ Set (length: %d)", len(libreJWTRefreshSecret))
	}
}

type LoginReq struct {
	Email        string `json:"email"`
	RefreshToken string `json:"refresh_token,omitempty"` // Optional: refresh token from main API
}

// setCORSHeaders sets CORS headers for cross-origin requests
func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "3600")
}

// generateRandomPassword generates a random password for LibreChat
func generateRandomPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback if crypto/rand fails
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// fetchUserFromAPI fetches user data from the main API
func fetchUserFromAPI(email string) (*APIUser, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Fetch users list from API (with high limit to find the user)
	// Note: This requires the API to be accessible without auth or we need a service account
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/users?limit=1000", mainAPIURL), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Warning: Could not fetch user from API: %v. Using email only.", err)
		return &APIUser{
			Email:         email,
			EmailVerified: true,
			FullName:      email,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Warning: API returned status %d. Using email only.", resp.StatusCode)
		return &APIUser{
			Email:         email,
			EmailVerified: true,
			FullName:      email,
		}, nil
	}

	var response struct {
		Data []APIUser `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Warning: Could not decode API response: %v. Using email only.", err)
		return &APIUser{
			Email:         email,
			EmailVerified: true,
			FullName:      email,
		}, nil
	}

	// Find user by email (case-insensitive)
	for _, user := range response.Data {
		if strings.EqualFold(user.Email, email) {
			return &user, nil
		}
	}

	// User not found in list, return minimal data
	log.Printf("Warning: User %s not found in API response. Using email only.", email)
	return &APIUser{
		Email:         email,
		EmailVerified: true,
		FullName:      email,
	}, nil
}

// createOrUpdateLibreChatUser creates or updates a user in LibreChat's MongoDB
func createOrUpdateLibreChatUser(user *APIUser, refreshToken string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Attempting to connect to MongoDB: %s", mongoURI)

	// Connect to MongoDB
	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("MongoDB connection error: %v", err)
		return "", fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Ping MongoDB to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Printf("MongoDB ping failed: %v", err)
		return "", fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	log.Printf("MongoDB connection successful")

	// Extract database name from URI or use default
	// MongoDB URI format: mongodb://host:port/database
	dbName := "LibreChat"

	// Try to parse as URL first
	parsedURI, err := url.Parse(mongoURI)
	if err == nil && parsedURI.Path != "" {
		// Path will be like "/LibreChat" or "/LibreChat?options"
		path := strings.TrimPrefix(parsedURI.Path, "/")
		if idx := strings.Index(path, "?"); idx > 0 {
			path = path[:idx]
		}
		if path != "" {
			dbName = path
		}
	} else {
		// Fallback: extract from string directly
		// Format: mongodb://host:port/database
		if strings.Contains(mongoURI, "/") {
			parts := strings.Split(mongoURI, "/")
			if len(parts) > 1 {
				dbPart := parts[len(parts)-1]
				// Remove query parameters if any
				if idx := strings.Index(dbPart, "?"); idx > 0 {
					dbPart = dbPart[:idx]
				}
				// Make sure it's not a port number (contains :)
				if dbPart != "" && !strings.Contains(dbPart, ":") {
					dbName = dbPart
				}
			}
		}
	}

	log.Printf("Using database: %s, collection: users", dbName)

	// Get database and collection
	db := client.Database(dbName)
	collection := db.Collection("users")

	// Generate username from email (take part before @)
	username := strings.Split(user.Email, "@")[0]

	// Generate name from FullName or FirstName/LastName
	name := user.FullName
	if name == "" {
		if user.FirstName != nil && user.LastName != nil {
			name = *user.FirstName + " " + *user.LastName
		} else if user.FirstName != nil {
			name = *user.FirstName
		} else if user.LastName != nil {
			name = *user.LastName
		} else {
			name = username
		}
	}
	if name == "" {
		name = username
	}

	// Prepare refresh token array
	refreshTokenArray := []interface{}{}
	if refreshToken != "" {
		refreshTokenArray = []interface{}{refreshToken}
	}

	// Prepare LibreChat user document
	// No password needed - authentication is handled by proxy JWT cookie
	// MongoDB is only used for storing chat history
	libreUserDoc := bson.M{
		"name":             name,
		"username":         username,
		"email":            user.Email,
		"emailVerified":    user.EmailVerified,
		"avatar":           user.AvatarURL,
		"provider":         "header",
		"role":             "USER",
		"plugins":          []interface{}{},
		"twoFactorEnabled": false,
		"termsAccepted":    false,
		"personalization":  bson.M{},
		"backupCodes":      []interface{}{},
		"refreshToken":     refreshTokenArray,
		// No password field - authentication via proxy JWT cookie
		"createdAt": time.Now(),
		"updatedAt": time.Now(),
		"__v":       0,
	}

	// Check if user already exists - use bson.M to avoid refreshToken decode issues
	filter := bson.M{"email": user.Email}
	var existingUserDoc bson.M
	err = collection.FindOne(ctx, filter).Decode(&existingUserDoc)

	userExists := err == nil
	hasDecodeError := err != nil && strings.Contains(err.Error(), "refreshToken")

	var mongoUserID string

	if err == mongo.ErrNoDocuments {
		// User doesn't exist, create new
		log.Printf("User not found, creating new user: %s (%s) for chat history", name, user.Email)
		result, err := collection.InsertOne(ctx, libreUserDoc)
		if err != nil {
			log.Printf("MongoDB insert error: %v", err)
			return "", fmt.Errorf("failed to create user in MongoDB: %w", err)
		}
		if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
			mongoUserID = oid.Hex()
		}
		log.Printf("Successfully created LibreChat user: %s (%s) with ID: %v (for chat history)", name, user.Email, result.InsertedID)
	} else if err != nil && !hasDecodeError {
		log.Printf("MongoDB find error: %v", err)
		return "", fmt.Errorf("failed to check existing user: %w", err)
	}

	// User exists (or we're continuing despite decode error), update it
	if userExists || hasDecodeError {
		// User exists, update it
		if hasDecodeError {
			log.Printf("RefreshToken decoding error, but user exists. Continuing with update...")
		}
		log.Printf("User exists, updating: %s (%s)", name, user.Email)
		update := bson.M{
			"$set": bson.M{
				"name":          name,
				"username":      username,
				"emailVerified": user.EmailVerified,
				"avatar":        user.AvatarURL,
				"updatedAt":     time.Now(),
			},
		}

		// No password needed - authentication via proxy JWT cookie
		// MongoDB is only for chat history storage

		// Check refreshToken field type and convert to array if needed
		var existingDoc bson.M
		if err := collection.FindOne(ctx, filter).Decode(&existingDoc); err == nil {
			// Check refreshToken field type and convert to array if needed
			if refreshToken != "" {
				existingRefreshToken := existingDoc["refreshToken"]
				if existingRefreshToken == nil {
					// Field doesn't exist, set as array
					update["$set"].(bson.M)["refreshToken"] = []interface{}{refreshToken}
					log.Printf("Setting refreshToken as new array for user: %s", user.Email)
				} else {
					// Check if it's already an array (primitive.A or []interface{})
					switch v := existingRefreshToken.(type) {
					case primitive.A, []interface{}:
						// It's an array, use $addToSet
						update["$addToSet"] = bson.M{
							"refreshToken": refreshToken,
						}
						log.Printf("Adding refresh token to array for user: %s", user.Email)
					default:
						// It's a string or other type, convert to array
						update["$set"].(bson.M)["refreshToken"] = []interface{}{refreshToken}
						log.Printf("Converting refreshToken from %T to array for user: %s", v, user.Email)
					}
				}
			}
		} else if refreshToken != "" {
			// User doesn't exist in existingDoc, but we're updating - set as array
			update["$set"].(bson.M)["refreshToken"] = []interface{}{refreshToken}
			log.Printf("Setting refreshToken as new array for user: %s", user.Email)
		}

		result, err := collection.UpdateOne(ctx, filter, update)
		if err != nil {
			log.Printf("MongoDB update error: %v", err)
			return "", fmt.Errorf("failed to update user in MongoDB: %w", err)
		}
		log.Printf("Successfully updated LibreChat user: %s (%s), Matched: %d, Modified: %d",
			name, user.Email, result.MatchedCount, result.ModifiedCount)
		if refreshToken != "" {
			log.Printf("Added refresh token to LibreChat user: %s", user.Email)
		}

		// Get the user ID from existing document
		var updatedDoc bson.M
		if err := collection.FindOne(ctx, filter).Decode(&updatedDoc); err == nil {
			if id, ok := updatedDoc["_id"].(primitive.ObjectID); ok {
				mongoUserID = id.Hex()
			}
		}
	}

	return mongoUserID, nil
}

// createLibreChatSession creates a session in MongoDB for LibreChat authentication
func createLibreChatSession(userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("userID is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Creating LibreChat session - connecting to MongoDB: %s", mongoURI)

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("MongoDB connection error in createLibreChatSession: %v", err)
		return "", fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Ping MongoDB to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		log.Printf("MongoDB ping failed in createLibreChatSession: %v", err)
		return "", fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	log.Printf("MongoDB connection verified for session creation")

	// Parse database name from URI
	dbName := "LibreChat"
	parsedURI, err := url.Parse(mongoURI)
	if err == nil && parsedURI.Path != "" {
		dbPart := strings.TrimPrefix(parsedURI.Path, "/")
		if dbPart != "" && !strings.Contains(dbPart, ":") {
			dbName = dbPart
		}
	}

	db := client.Database(dbName)
	sessionsCollection := db.Collection("sessions")

	// Convert userID string to ObjectID
	userObjectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user ID: %w", err)
	}

	// LibreChat's session creation flow:
	// 1. Create session document first (to get session._id)
	// 2. Generate JWT with id (userID) and sessionId (session._id)
	// 3. Hash the JWT token using SHA-256
	// 4. Store refreshTokenHash (hashed) in session, NOT refreshToken (raw)
	// 5. Return raw (unhashed) token for cookie

	if len(libreJWTRefreshSecret) == 0 {
		return "", fmt.Errorf("LIBRE_JWT_REFRESH_SECRET not set")
	}

	expirationTime := time.Now().Add(7 * 24 * time.Hour) // 7 days

	// Step 1: Create session document first (without refreshTokenHash - we'll update it)
	sessionDoc := bson.M{
		"user":       userObjectID,
		"expiration": expirationTime,
		"createdAt":  time.Now(),
		"updatedAt":  time.Now(),
	}

	result, err := sessionsCollection.InsertOne(ctx, sessionDoc)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	sessionObjectID := result.InsertedID.(primitive.ObjectID)
	sessionID := sessionObjectID.Hex()

	// Step 2: Generate JWT with id (userID) and sessionId (session._id)
	// This matches LibreChat's generateRefreshToken function
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":        userID,    // User ID as string
		"sessionId": sessionID, // Session ID as string
		"exp":       expirationTime.Unix(),
		"iat":       time.Now().Unix(),
	})
	refreshTokenString, err := refreshToken.SignedString(libreJWTRefreshSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Step 3: Hash the refresh token using SHA-256 (matching LibreChat's hashToken function)
	hash := sha256.Sum256([]byte(refreshTokenString))
	refreshTokenHash := hex.EncodeToString(hash[:])

	// Step 4: Update session with refreshTokenHash (NOT refreshToken)
	update := bson.M{
		"$set": bson.M{
			"refreshTokenHash": refreshTokenHash,
			"updatedAt":        time.Now(),
		},
	}
	_, err = sessionsCollection.UpdateOne(ctx, bson.M{"_id": sessionObjectID}, update)
	if err != nil {
		return "", fmt.Errorf("failed to update session with refreshTokenHash: %w", err)
	}

	log.Printf("Created LibreChat session for user %s: %s (with refreshTokenHash stored)", userID, sessionID)

	// Step 5: Return raw (unhashed) token for cookie
	return refreshTokenString, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		setCORSHeaders(w, r)
		w.WriteHeader(http.StatusOK)
		return
	}

	setCORSHeaders(w, r)

	// Only allow POST method for login
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("LoginHandler: Failed to decode request body: %v", err)
		http.Error(w, "bad request: invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		log.Printf("LoginHandler: Email is required but was empty")
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	// Fetch user data from main API
	user, err := fetchUserFromAPI(req.Email)
	if err != nil {
		log.Printf("Warning: Failed to fetch user from API: %v", err)
		// Continue with login even if API fetch fails
		user = &APIUser{
			Email:         req.Email,
			EmailVerified: true,
			FullName:      req.Email,
		}
	}

	// Create or update user in LibreChat MongoDB (sync to get user ID)
	log.Printf("Starting MongoDB user sync for: %s (refresh_token provided: %v)", req.Email, req.RefreshToken != "")
	mongoUserID, err := createOrUpdateLibreChatUser(user, req.RefreshToken)
	if err != nil {
		log.Printf("ERROR creating/updating LibreChat user: %v", err)
		// Continue anyway - user might still work
	} else {
		log.Printf("SUCCESS: LibreChat user sync completed for: %s (MongoDB ID: %s)", req.Email, mongoUserID)

		// Create session and generate LibreChat tokens if we have the user ID
		if mongoUserID != "" {
			log.Printf("Creating LibreChat session for MongoDB user ID: %s", mongoUserID)
			// Create session in MongoDB (required for LibreChat authentication)
			refreshTokenString, err := createLibreChatSession(mongoUserID)
			if err != nil {
				log.Printf("ERROR: Failed to create LibreChat session: %v", err)
				log.Printf("WARNING: Continuing without session - authentication may fail")
			} else {
				log.Printf("SUCCESS: LibreChat session created, refresh token generated")
				if len(libreJWTSecret) > 0 {
					// Create LibreChat access token (JWT signed with JWT_SECRET)
					accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
						"id":  mongoUserID,
						"exp": time.Now().Add(24 * time.Hour).Unix(), // 24 hour expiry
						"iat": time.Now().Unix(),
					})
					accessTokenString, err := accessToken.SignedString(libreJWTSecret)
					if err == nil {
						// Set LibreChat's refreshToken cookie
						refreshTokenCookie := &http.Cookie{
							Name:     "refreshToken",
							Value:    refreshTokenString,
							Path:     "/",
							Domain:   "",
							Expires:  time.Now().Add(7 * 24 * time.Hour), // 7 days
							MaxAge:   7 * 24 * 3600,
							Secure:   false,
							HttpOnly: true,
							SameSite: http.SameSiteLaxMode,
						}
						http.SetCookie(w, refreshTokenCookie)

						// Set token_provider cookie
						tokenProviderCookie := &http.Cookie{
							Name:     "token_provider",
							Value:    "librechat",
							Path:     "/",
							Domain:   "",
							Expires:  time.Now().Add(7 * 24 * time.Hour),
							MaxAge:   7 * 24 * 3600,
							Secure:   false,
							HttpOnly: true,
							SameSite: http.SameSiteLaxMode,
						}
						http.SetCookie(w, tokenProviderCookie)

						// Store LibreChat access token in response header for frontend
						w.Header().Set("X-LibreChat-Token", accessTokenString)
						log.Printf("Set LibreChat authentication tokens for user: %s (MongoDB ID: %s)", req.Email, mongoUserID)
					} else {
						log.Printf("Failed to generate LibreChat access token: %v", err)
					}
				} else {
					log.Printf("ERROR: LIBRE_JWT_SECRET not set - cannot generate access token")
				}
			}
		} else {
			log.Printf("WARNING: mongoUserID is empty - cannot create session")
		}
	}

	// create JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": req.Email,
		"exp":   time.Now().Add(6 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	// Set secure, HttpOnly cookie. Must be served over HTTPS in browsers for Secure flag to work.
	// For development, we'll use Secure: false
	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    tokenString,
		Path:     "/",
		Domain:   "", // leave empty for host-only cookie
		Expires:  time.Now().Add(6 * time.Hour),
		MaxAge:   6 * 3600,
		Secure:   false,                // Set to true in production with HTTPS
		HttpOnly: true,                 // not accessible via JS
		SameSite: http.SameSiteLaxMode, // Changed to Lax for iframe compatibility
	}
	http.SetCookie(w, cookie)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// verifyToken returns email from token or error
func verifyToken(tokenString string) (string, error) {
	tokenString = strings.TrimSpace(tokenString)
	// allow formats: "Bearer xxx" or raw token
	if strings.HasPrefix(strings.ToLower(tokenString), "bearer ") {
		tokenString = tokenString[7:]
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		// Verify alg
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if email, ok := claims["email"].(string); ok {
			return email, nil
		}
	}
	return "", fmt.Errorf("invalid token")
}

// proxyWebsocket proxies a websocket connection to the target websocket endpoint.
func proxyWebsocket(w http.ResponseWriter, r *http.Request, targetUrl *url.URL, email string) {
	// prepare dialer to backend
	dialer := websocket.DefaultDialer

	// copy headers for backend, injecting our user header
	requestHeader := http.Header{}
	for k, v := range r.Header {
		// Skip websocket-related headers that gorilla/websocket handles automatically
		lowerK := strings.ToLower(k)
		if lowerK == "sec-websocket-key" ||
			lowerK == "sec-websocket-accept" ||
			lowerK == "upgrade" ||
			lowerK == "connection" ||
			lowerK == "sec-websocket-version" ||
			lowerK == "sec-websocket-protocol" {
			continue
		}
		for _, vv := range v {
			requestHeader.Add(k, vv)
		}
	}

	// inject user header
	if email != "" {
		requestHeader.Set("X-Authenticated-User", email)
		requestHeader.Set("X-User-From-Proxy", email)
	}

	// Build target ws url: wss:// or ws:// depending on target scheme
	wsScheme := "ws"
	if targetUrl.Scheme == "https" || targetUrl.Scheme == "wss" {
		wsScheme = "wss"
	}
	targetWsUrl := url.URL{
		Scheme:   wsScheme,
		Host:     targetUrl.Host,
		Path:     r.URL.Path, // pass through path after /proxy
		RawQuery: r.URL.RawQuery,
	}

	backendConn, resp, err := dialer.Dial(targetWsUrl.String(), requestHeader)
	if err != nil {
		log.Printf("websocket dial error: %v (resp: %+v)\n", err, resp)
		http.Error(w, "Error connecting to backend websocket: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Upgrade incoming request to websocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true }, // allow from any origin for demo; adjust in prod
	}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v\n", err)
		backendConn.Close()
		return
	}

	// Start copying messages between clientConn and backendConn
	errc := make(chan error, 2)

	go func() {
		defer backendConn.Close()
		defer clientConn.Close()
		for {
			mt, message, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := backendConn.WriteMessage(mt, message); err != nil {
				errc <- err
				return
			}
		}
	}()

	go func() {
		defer backendConn.Close()
		defer clientConn.Close()
		for {
			mt, message, err := backendConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(mt, message); err != nil {
				errc <- err
				return
			}
		}
	}()

	// wait for error
	<-errc
}

// stripPrefixPath removes the proxy prefix and returns the modified request path for backend.
func stripPrefixPath(r *http.Request, prefix string) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
}

func main() {
	// parse targets
	backendTarget, err := url.Parse(libreBackend)
	if err != nil {
		log.Fatal(err)
	}

	frontendTarget, err := url.Parse(libreFrontend)
	if err != nil {
		log.Fatal(err)
	}

	// Backend API proxy (for /api and /oauth)
	backendProxy := httputil.NewSingleHostReverseProxy(backendTarget)

	// Backend proxy transport
	backendProxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	originalBackendDirector := backendProxy.Director
	backendProxy.Director = func(req *http.Request) {
		originalBackendDirector(req)
		req.Host = backendTarget.Host

		// Check Authorization header for tokens
		authHeader := req.Header.Get("Authorization")

		// If Authorization header exists, check if it's a LibreChat token or proxy token
		if authHeader != "" {
			// Remove "Bearer " prefix if present
			token := authHeader
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}

			// First, try to verify as LibreChat token (signed with LIBRE_JWT_SECRET)
			if len(libreJWTSecret) > 0 {
				if _, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
					}
					return libreJWTSecret, nil
				}); err == nil {
					// This is a LibreChat access token - forward it to LibreChat backend
					// Don't delete it, let it pass through
					log.Printf("DEBUG: Forwarding LibreChat access token to backend")
					goto forwardCookies
				}
			}

			// Not a LibreChat token, try to verify as proxy token
			// Extract token from cookie or Authorization header
			var proxyToken string
			if c, err := req.Cookie(cookieName); err == nil {
				proxyToken = c.Value
			}
			if proxyToken == "" {
				proxyToken = token
			}

			email := ""
			if proxyToken != "" {
				if e, err := verifyToken(proxyToken); err == nil {
					email = e
				}
			}

			if email != "" {
				// This is a proxy token - extract email and forward user info
				req.Header.Set("X-Authenticated-User", email)
				req.Header.Set("X-User-From-Proxy", email)
				req.Header.Del("Authorization")
			} else {
				// Unknown token format - forward as-is (might be valid for LibreChat)
				log.Printf("DEBUG: Unknown token format, forwarding Authorization header as-is")
			}
		} else {
			// No Authorization header, check for proxy token in cookie
			if c, err := req.Cookie(cookieName); err == nil {
				if email, err := verifyToken(c.Value); err == nil {
					req.Header.Set("X-Authenticated-User", email)
					req.Header.Set("X-User-From-Proxy", email)
				}
			}
		}

	forwardCookies:

		// Forward LibreChat cookies (refreshToken and token_provider) to LibreChat backend
		// The httputil.ReverseProxy automatically forwards cookies from the client request,
		// but we need to ensure LibreChat-specific cookies are explicitly forwarded
		// Ensure cookies are forwarded for /api/auth/refresh requests
		// (Debug logging removed to reduce log noise - uncomment if needed for troubleshooting)
		if strings.Contains(req.URL.Path, "/api/auth/refresh") {
			// Ensure cookies are in the header that will be sent to LibreChat backend
			// httputil.ReverseProxy should handle this automatically, but let's be explicit
			refreshTokenCookie, _ := req.Cookie("refreshToken")
			tokenProviderCookie, _ := req.Cookie("token_provider")
			cookieHeader := req.Header.Get("Cookie")

			if refreshTokenCookie != nil && !strings.Contains(cookieHeader, "refreshToken=") {
				if cookieHeader != "" {
					req.Header.Set("Cookie", cookieHeader+"; refreshToken="+refreshTokenCookie.Value)
				} else {
					req.Header.Set("Cookie", "refreshToken="+refreshTokenCookie.Value)
				}
				cookieHeader = req.Header.Get("Cookie")
			}
			if tokenProviderCookie != nil && !strings.Contains(cookieHeader, "token_provider=") {
				if cookieHeader != "" {
					req.Header.Set("Cookie", cookieHeader+"; token_provider="+tokenProviderCookie.Value)
				} else {
					req.Header.Set("Cookie", "token_provider="+tokenProviderCookie.Value)
				}
			}
		}
	}

	// Frontend proxy (for Vite dev server)
	frontendProxy := httputil.NewSingleHostReverseProxy(frontendTarget)

	// Frontend proxy transport
	frontendProxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	originalFrontendDirector := frontendProxy.Director
	frontendProxy.Director = func(req *http.Request) {
		originalFrontendDirector(req)
		req.Host = frontendTarget.Host
		// DON'T strip /proxy/ prefix - LibreChat is now configured with base: '/proxy/'
		// So it expects paths like /proxy/c/new and serves from that base
		// Keep the full path including /proxy/ when forwarding
	}

	// Custom response modifier for frontend to handle CORS, preserve headers, and rewrite URLs in HTML
	frontendProxy.ModifyResponse = func(resp *http.Response) error {
		// Set CORS headers on the response
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Credentials", "true")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		// Rewrite URLs in HTML responses to use /proxy/ prefix
		contentType := resp.Header.Get("Content-Type")
		if strings.Contains(contentType, "text/html") {
			// Read the response body
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()

			htmlContent := string(bodyBytes)

			// DON'T rewrite URLs - LibreChat is configured with base: '/proxy/'
			// It already generates all URLs with /proxy/ prefix automatically
			// Only rewrite WebSocket URLs to use proxy port (9443) instead of LibreChat port (3090)

			// Only rewrite WebSocket URLs to use proxy port (9443) instead of LibreChat port (3090)
			// Keep the /proxy/ prefix since LibreChat is configured with base: '/proxy/'
			htmlContent = strings.ReplaceAll(htmlContent, `ws://localhost:3090/`, `ws://localhost:9443/`)
			htmlContent = strings.ReplaceAll(htmlContent, `wss://localhost:3090/`, `wss://localhost:9443/`)
			htmlContent = strings.ReplaceAll(htmlContent, `"ws://localhost:3090/`, `"ws://localhost:9443/`)
			htmlContent = strings.ReplaceAll(htmlContent, `"wss://localhost:3090/`, `"wss://localhost:9443/`)
			htmlContent = strings.ReplaceAll(htmlContent, `'ws://localhost:3090/`, `'ws://localhost:9443/`)
			htmlContent = strings.ReplaceAll(htmlContent, `'wss://localhost:3090/`, `'wss://localhost:9443/`)

			// Don't modify base tag - LibreChat should already have it set correctly to /proxy/
			// Only fix if it's incorrectly set (shouldn't happen with base config)

			// No router fix script needed - LibreChat is configured with base: '/proxy/'
			// Paths are forwarded as-is (e.g., /proxy/c/new) and LibreChat handles them correctly
			routerFixScript := `` // Empty - no fix needed

			// Insert the script as early as possible - right after <head> tag
			if strings.Contains(htmlContent, "<head>") {
				// Insert right after <head> tag
				htmlContent = strings.Replace(htmlContent, "<head>", "<head>"+routerFixScript, 1)
			} else if strings.Contains(htmlContent, "</head>") {
				// If no opening head tag, insert before closing head
				htmlContent = strings.Replace(htmlContent, "</head>", routerFixScript+"</head>", 1)
			} else {
				// Last resort: insert at the very beginning
				htmlContent = routerFixScript + htmlContent
			}

			// Replace the response body
			resp.Body = io.NopCloser(strings.NewReader(htmlContent))
			resp.ContentLength = int64(len(htmlContent))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(htmlContent)))
		}

		return nil
	}

	// Helper function to extract email from request
	extractEmailFromRequest := func(r *http.Request) string {
		var token string
		if c, err := r.Cookie(cookieName); err == nil {
			token = c.Value
		}
		if token == "" {
			token = r.Header.Get("Authorization")
		}
		if token != "" {
			if email, err := verifyToken(token); err == nil {
				return email
			}
		}
		return ""
	}

	// Saas-API proxy (for /api/v1/* - forwards to port 8080)
	saasAPITarget, err := url.Parse(mainAPIURL)
	if err != nil {
		log.Fatalf("Failed to parse saas-api URL: %v", err)
	}
	saasAPIProxy := httputil.NewSingleHostReverseProxy(saasAPITarget)
	saasAPIProxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	saasAPIProxy.Director = func(req *http.Request) {
		req.URL.Scheme = saasAPITarget.Scheme
		req.URL.Host = saasAPITarget.Host
		req.Host = saasAPITarget.Host
		// Preserve the Authorization header if present (for saas-api token)
		// The token will be in the Authorization header from the frontend
		// Query parameters (including token query param for static files) are automatically preserved
		// by req.URL.RawQuery, no action needed
		log.Printf("DEBUG: saasAPIProxy.Director - forwarding to %s://%s%s (query: %s)", req.URL.Scheme, req.URL.Host, req.URL.Path, req.URL.RawQuery)
	}
	// Add error handler to catch and log proxy errors
	saasAPIProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("ERROR: saasAPIProxy error for %s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, fmt.Sprintf("Bad Gateway: %v", err), http.StatusBadGateway)
	}

	// Static file routes - route /static/* to saas-api (port 8080)
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Log the request for debugging
		log.Printf("DEBUG: Proxying static file request: %s %s (query: %s)", r.Method, r.URL.Path, r.URL.RawQuery)
		// Ensure query parameters are preserved
		// The saasAPIProxy.Director should preserve them, but let's make sure
		saasAPIProxy.ServeHTTP(w, r)
	})

	// Backend API routes (/api and /oauth)
	// But exclude /api/v1/* which goes to saas-api
	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// Route /api/v1/* to saas-api (port 8080)
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			setCORSHeaders(w, r)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			saasAPIProxy.ServeHTTP(w, r)
			return
		}

		email := extractEmailFromRequest(r)

		// Handle websocket for backend
		if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			proxyWebsocket(w, r, backendTarget, email)
			return
		}

		backendProxy.ServeHTTP(w, r)
	})

	http.HandleFunc("/oauth/", func(w http.ResponseWriter, r *http.Request) {
		backendProxy.ServeHTTP(w, r)
	})

	// Frontend routes - proxy everything else to Vite dev server
	// This includes: /@vite/client, /src/, /@react-refresh, /c/new, etc.
	http.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) {
		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusOK)
			return
		}

		// If the path is /proxy/api/v1/*, route to saas-api (port 8080)
		if strings.HasPrefix(r.URL.Path, "/proxy/api/v1/") {
			setCORSHeaders(w, r)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			// Strip /proxy prefix and route to saas-api
			stripPrefixPath(r, "/proxy")
			saasAPIProxy.ServeHTTP(w, r)
			return
		}

		// If the path is /proxy/api/ or /proxy/oauth/, route to backend instead of frontend
		if strings.HasPrefix(r.URL.Path, "/proxy/api/") || strings.HasPrefix(r.URL.Path, "/proxy/oauth/") {
			// Strip /proxy prefix and route to backend
			stripPrefixPath(r, "/proxy")
			email := extractEmailFromRequest(r)

			// Handle WebSocket for backend
			if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				proxyWebsocket(w, r, backendTarget, email)
				return
			}

			// Route to backend API
			backendProxy.ServeHTTP(w, r)
			return
		}

		// DON'T strip /proxy prefix for frontend routes - LibreChat is configured with base: '/proxy/'
		// Keep the full path /proxy/c/new when forwarding to LibreChat
		// stripPrefixPath(r, "/proxy")  // Keep prefix - LibreChat expects it

		email := extractEmailFromRequest(r)

		// If websocket Upgrade header present, handle websocket proxying
		if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			proxyWebsocket(w, r, frontendTarget, email)
			return
		}

		// Otherwise use HTTP reverse proxy
		frontendProxy.ServeHTTP(w, r)
	})

	// Handle Vite-specific routes that might be requested from root
	// These should be proxied to frontend even when accessed without /proxy/ prefix
	viteHandler := func(w http.ResponseWriter, r *http.Request) {
		email := extractEmailFromRequest(r)
		if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") || strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			proxyWebsocket(w, r, frontendTarget, email)
			return
		}
		frontendProxy.ServeHTTP(w, r)
	}

	// Register Vite routes
	http.HandleFunc("/@vite/", viteHandler)
	http.HandleFunc("/@react-refresh", viteHandler)
	http.HandleFunc("/src/", viteHandler)
	http.HandleFunc("/node_modules/", viteHandler)
	http.HandleFunc("/@fs/", viteHandler)

	// Default handler - proxy everything else to frontend (for direct access without /proxy prefix)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// /login is handled by its own handler above, so we don't need to check here

		// Skip API, OAuth, and static routes (already handled above)
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/oauth/") || strings.HasPrefix(r.URL.Path, "/static/") {
			return
		}

		email := extractEmailFromRequest(r)

		// Handle websocket - Vite HMR WebSocket might connect to root path
		// Check for WebSocket upgrade or if it's a WebSocket request (has token query param)
		if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
			strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
			(r.URL.Query().Get("token") != "" && r.Method == "GET") {
			// For WebSocket at root, we need to proxy to frontend
			// But first, check if this is actually a WebSocket request
			if strings.EqualFold(r.Header.Get("Connection"), "Upgrade") ||
				strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				proxyWebsocket(w, r, frontendTarget, email)
				return
			}
			// If it has a token param but isn't a WebSocket upgrade, it might be Vite trying to connect
			// Redirect or proxy to /proxy/ path
			if r.URL.Query().Get("token") != "" {
				// This is likely Vite HMR WebSocket - proxy to frontend
				proxyWebsocket(w, r, frontendTarget, email)
				return
			}
		}

		// Proxy to frontend
		frontendProxy.ServeHTTP(w, r)
	})

	// Endpoint to get LibreChat credentials from MongoDB
	http.HandleFunc("/get-librechat-credentials", func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		email := r.URL.Query().Get("email")
		if email == "" {
			http.Error(w, "email parameter required", http.StatusBadRequest)
			return
		}

		// Get JWT from cookie OR from Authorization header (main app token)
		var verifiedEmail string

		// Try cookie first
		cookie, err := r.Cookie(cookieName)
		if err == nil && cookie != nil {
			// Verify JWT from cookie
			token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return jwtSecret, nil
			})

			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if claimEmail, ok := claims["email"].(string); ok {
						verifiedEmail = claimEmail
					}
				}
			}
		}

		// If cookie didn't work, try Authorization header (main app token)
		if verifiedEmail == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				// Verify token with main API
				client := &http.Client{Timeout: 5 * time.Second}
				req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/auth/me", mainAPIURL), nil)
				if err == nil {
					req.Header.Set("Authorization", authHeader)
					resp, err := client.Do(req)
					if err == nil && resp.StatusCode == 200 {
						var userData map[string]interface{}
						if json.NewDecoder(resp.Body).Decode(&userData) == nil {
							if userEmail, ok := userData["email"].(string); ok {
								verifiedEmail = userEmail
								log.Printf("Verified user via main API token: %s", verifiedEmail)
							}
						}
						resp.Body.Close()
					} else if err != nil {
						log.Printf("Error verifying main API token: %v", err)
					} else {
						log.Printf("Main API token verification failed with status: %d", resp.StatusCode)
					}
				}
			}
		}

		// If still not verified, return unauthorized
		if verifiedEmail == "" {
			log.Printf("Unauthorized: No valid JWT cookie or Authorization token")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Verify the email matches
		if verifiedEmail != email {
			log.Printf("Email mismatch: verified=%s, requested=%s", verifiedEmail, email)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Fetch user from LibreChat MongoDB
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		clientOptions := options.Client().ApplyURI(mongoURI)
		client, err := mongo.Connect(ctx, clientOptions)
		if err != nil {
			log.Printf("MongoDB connection error: %v", err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		defer client.Disconnect(ctx)

		// Extract database name
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
		}

		db := client.Database(dbName)
		collection := db.Collection("users")

		filter := bson.M{"email": email}
		// Use bson.M to avoid refreshToken decoding issues
		var userDoc bson.M
		err = collection.FindOne(ctx, filter).Decode(&userDoc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "user not found in LibreChat", http.StatusNotFound)
				return
			}
			log.Printf("MongoDB find error: %v", err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		// Extract user fields from document
		libreUserEmail := ""
		libreUserUsername := ""
		libreUserName := ""

		if emailVal, ok := userDoc["email"].(string); ok {
			libreUserEmail = emailVal
		}
		if usernameVal, ok := userDoc["username"].(string); ok {
			libreUserUsername = usernameVal
		}
		if nameVal, ok := userDoc["name"].(string); ok {
			libreUserName = nameVal
		}

		if libreUserEmail == "" {
			log.Printf("Email not found in user document")
			http.Error(w, "invalid user document", http.StatusInternalServerError)
			return
		}

		// No password needed - authentication is handled by proxy JWT cookie
		// MongoDB is only for chat history storage
		log.Printf("Returning user info for %s (no password needed - using JWT cookie auth)", email)

		response := map[string]interface{}{
			"email":    libreUserEmail,
			"username": libreUserUsername,
			"name":     libreUserName,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// login endpoint (also handle explicitly)
	// Register /login handler with method check wrapper
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			setCORSHeaders(w, r)
			w.WriteHeader(http.StatusOK)
			return
		}

		// GET, HEAD, and other read-only requests should be proxied to frontend (for the login page)
		// This includes browser reloads, favicon requests, etc.
		if r.Method == "GET" || r.Method == "HEAD" {
			frontendProxy.ServeHTTP(w, r)
			return
		}

		// POST requests go to loginHandler
		if r.Method == "POST" {
			loginHandler(w, r)
			return
		}

		// For any other method, return method not allowed
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	port := "9443" // Default port
	if p := os.Getenv("PROXY_PORT"); p != "" {
		port = p
	}

	// For development, use HTTP. For production, use HTTPS with certs
	useHTTPS := os.Getenv("USE_HTTPS") == "true"

	srv := &http.Server{
		Addr: ":" + port,
		// Good practice: set timeouts to avoid Slowloris
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if useHTTPS {
		certFile := "cert.pem"
		keyFile := "key.pem"
		srv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
		}

		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			log.Fatalf("cert.pem not found in current directory; please create or place cert.pem and key.pem alongside the server binary")
		}

		// Graceful shutdown
		go func() {
			log.Printf("Starting HTTPS proxy server on https://localhost:%s\n", port)
			log.Printf("Backend proxy: %s\n", libreBackend)
			log.Printf("Frontend proxy: %s\n", libreFrontend)
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("ListenAndServeTLS(): %v", err)
			}
		}()
	} else {
		// Graceful shutdown
		go func() {
			log.Printf("Starting HTTP proxy server on http://localhost:%s\n", port)
			log.Printf("Backend proxy: %s\n", libreBackend)
			log.Printf("Frontend proxy: %s\n", libreFrontend)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("ListenAndServe(): %v", err)
			}
		}()
	}

	// handle shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
