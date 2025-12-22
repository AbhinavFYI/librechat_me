package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"saas-api/config"
	"saas-api/internal/auth"
	"saas-api/internal/handlers"
	"saas-api/internal/middleware"
	"saas-api/internal/repositories"
	"saas-api/internal/services"
	"saas-api/pkg/memorydb"
	"saas-api/pkg/postgres"
	"saas-api/pkg/weaviate"

	"saas-api/cmd/configs"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load .env file
	envPaths := []string{
		"../../.env", // From cmd/api/ to saas-api/.env
		".env",       // Current directory
	}

	envLoaded := false
	for _, path := range envPaths {
		if err := godotenv.Load(path); err == nil {
			log.Printf("Loaded .env from: %s", path)
			envLoaded = true
			break
		}
	}

	if !envLoaded {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := postgres.NewDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	orgRepo := repositories.NewOrganizationRepository(db)
	tokenRepo := repositories.NewRefreshTokenRepository(db)
	roleRepo := repositories.NewRoleRepository(db)
	permRepo := repositories.NewPermissionRepository(db)
	templateRepo := repositories.NewTemplateRepository(db)
	personaRepo := repositories.NewPersonaRepository(db)
	folderRepo := repositories.NewFolderRepository(db)
	// FileRepository is deprecated - using DocumentRepository instead
	// fileRepo := repositories.NewFileRepository(db)
	auditLogRepo := repositories.NewAuditLogRepository(db)
	screenerRepo := repositories.NewScreenerRepository(db)

	// Initialize Redis and Weaviate clients for document service
	ctx := context.Background()

	// Create minimal configs.Config for Redis and Weaviate
	// Import configs package to use its Config type
	type MinimalConfig struct {
		MemoryDBRedisURL      string
		MemoryDBRedisUsername string
		MemoryDBRedisPassword string
		WeaviateHost          string
		WeaviatePort          string
		WeaviateScheme        string
	}

	minimalConfig := &MinimalConfig{
		MemoryDBRedisURL:      os.Getenv("REDIS_URL"),
		MemoryDBRedisUsername: os.Getenv("REDIS_USERNAME"),
		MemoryDBRedisPassword: os.Getenv("REDIS_PASSWORD"),
		WeaviateHost:          os.Getenv("WEAVIATE_HOST"),
		WeaviatePort:          os.Getenv("WEAVIATE_PORT"),
		WeaviateScheme:        os.Getenv("WEAVIATE_SCHEME"),
	}

	// Set defaults
	if minimalConfig.WeaviateHost == "" {
		minimalConfig.WeaviateHost = "10.10.6.13"
	}
	if minimalConfig.WeaviatePort == "" {
		minimalConfig.WeaviatePort = "7080"
	}
	if minimalConfig.WeaviateScheme == "" {
		minimalConfig.WeaviateScheme = "http"
	}

	// Initialize Redis client
	var redisClient *memorydb.RedisClient
	if minimalConfig.MemoryDBRedisURL != "" {
		log.Printf("Attempting to connect to Redis at: %s", minimalConfig.MemoryDBRedisURL)
		// Create a configs.Config-compatible struct
		redisConfig := struct {
			MemoryDBRedisURL      string
			MemoryDBRedisUsername string
			MemoryDBRedisPassword string
		}{
			MemoryDBRedisURL:      minimalConfig.MemoryDBRedisURL,
			MemoryDBRedisUsername: minimalConfig.MemoryDBRedisUsername,
			MemoryDBRedisPassword: minimalConfig.MemoryDBRedisPassword,
		}

		// Create a proper configs.Config
		redisConfigFull := &configs.Config{
			MemoryDBRedisURL:      redisConfig.MemoryDBRedisURL,
			MemoryDBRedisUsername: redisConfig.MemoryDBRedisUsername,
			MemoryDBRedisPassword: redisConfig.MemoryDBRedisPassword,
		}

		var err error
		redisClient, err = memorydb.NewRedisClient(ctx, redisConfigFull)
		if err != nil {
			log.Printf("Failed to initialize Redis client: %v. Document service will not be available.", err)
			redisClient = nil
		} else {
			log.Printf("Redis client initialized successfully")
		}
	} else {
		log.Printf("REDIS_URL not set. Document service will not be available.")
	}

	// Initialize Weaviate client
	var weaviateClient *weaviate.WeaviateClient
	weaviateConfigFull := &configs.Config{
		WeaviateHost:   minimalConfig.WeaviateHost,
		WeaviatePort:   minimalConfig.WeaviatePort,
		WeaviateScheme: minimalConfig.WeaviateScheme,
	}

	log.Printf("Attempting to connect to Weaviate at: %s://%s:%s", minimalConfig.WeaviateScheme, minimalConfig.WeaviateHost, minimalConfig.WeaviatePort)

	// Try to initialize Weaviate (it may panic, so we catch it)
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Failed to initialize Weaviate client: %v. Document service will not be available.", r)
				weaviateClient = nil
			}
		}()
		weaviateClient = weaviate.NewWeaviateClient(weaviateConfigFull)
		if weaviateClient != nil {
			log.Printf("Weaviate client initialized successfully")
		}
	}()

	// Initialize document repositories
	docRepo := repositories.NewDocumentRepository(db, db)

	// Create repositories struct for document service
	repos := &repositories.Repositories{
		User:         userRepo,
		Organization: orgRepo,
		Role:         roleRepo,
		Permission:   permRepo,
		RefreshToken: tokenRepo,
		Document:     docRepo,
		Folder:       folderRepo,
		File:         nil, // Deprecated - using DocumentRepository instead
		Template:     templateRepo,
		Persona:      personaRepo,
		AuditLog:     auditLogRepo,
		Screener:     screenerRepo,
	}

	// Initialize services
	tokenService := auth.NewTokenService(cfg)
	authService := services.NewAuthService(userRepo, tokenRepo, tokenService, cfg)

	// Initialize document service (only if Redis and Weaviate are available)
	var documentHandler *handlers.DocumentHandler
	log.Printf("Checking document service dependencies - Redis: %v, Weaviate: %v", redisClient != nil, weaviateClient != nil)

	if redisClient != nil && weaviateClient != nil {
		log.Println("Initializing document service...")
		baseService := services.NewBaseService(repos, redisClient, weaviateClient)

		// Create services struct - this will create DocumentService internally
		// We need to pass a configs.Config, but we'll create a minimal one
		minimalConfigsConfig := &configs.Config{} // Empty config, document service uses env vars
		svcs := services.NewServices(baseService, userRepo, tokenRepo, tokenService, minimalConfigsConfig)

		// Initialize document schema
		if err := svcs.Document.InitSchema(ctx); err != nil {
			log.Printf("Warning: Failed to initialize document schema: %v", err)
		}

		documentHandler = handlers.NewDocumentHandler(svcs)
		log.Println("Document service initialized successfully")
	} else {
		if redisClient == nil {
			log.Println("Redis client is nil - Document service will not be available")
		}
		if weaviateClient == nil {
			log.Println("Weaviate client is nil - Document service will not be available")
		}
		log.Println("Document service not initialized (Redis or Weaviate unavailable)")
	}

	// Initialize middleware
	authMW := middleware.NewAuthMiddleware(tokenService)
	rlsMW := middleware.NewRLSMiddleware(db)
	permMW := middleware.NewPermissionMiddleware(userRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService, authMW, orgRepo)
	userHandler := handlers.NewUserHandler(userRepo, roleRepo, orgRepo)
	orgHandler := handlers.NewOrganizationHandler(orgRepo, roleRepo, permRepo)
	roleHandler := handlers.NewRoleHandler(roleRepo)
	permHandler := handlers.NewPermissionHandler(permRepo)
	templateHandler := handlers.NewTemplateHandler(templateRepo)
	personaHandler := handlers.NewPersonaHandler(personaRepo)
	folderHandler := handlers.NewFolderHandler(folderRepo, docRepo)

	// File handler removed - all file operations now use /api/v1/documents
	// The fileHandler is no longer needed as we use a unified documents API
	// fileHandler := handlers.NewFileHandler(folderRepo, docRepo, docService, cfg.App.StoragePath)
	staticHandler := handlers.NewStaticHandler(cfg.App.StoragePath, docRepo)
	libreChatHandler := handlers.NewLibreChatHandler()
	auditLogHandler := handlers.NewAuditLogHandler(auditLogRepo)
	screenerHandler := handlers.NewScreenerHandler(screenerRepo, userRepo)

	// Setup router
	router := setupRouter(cfg, authHandler, userHandler, orgHandler, roleHandler, permHandler, templateHandler, personaHandler, folderHandler, staticHandler, libreChatHandler, auditLogHandler, screenerHandler, documentHandler, authMW, rlsMW, permMW)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}

func setupRouter(
	cfg *config.Config,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	orgHandler *handlers.OrganizationHandler,
	roleHandler *handlers.RoleHandler,
	permHandler *handlers.PermissionHandler,
	templateHandler *handlers.TemplateHandler,
	personaHandler *handlers.PersonaHandler,
	folderHandler *handlers.FolderHandler,
	// fileHandler *handlers.FileHandler,
	staticHandler *handlers.StaticHandler,
	libreChatHandler *handlers.LibreChatHandler,
	auditLogHandler *handlers.AuditLogHandler,
	screenerHandler *handlers.ScreenerHandler,
	documentHandler *handlers.DocumentHandler, // Can be nil if not initialized
	authMW *middleware.AuthMiddleware,
	rlsMW *middleware.RLSMiddleware,
	permMW *middleware.PermissionMiddleware,
) *gin.Engine {
	if cfg.App.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.ErrorMiddleware())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "saas-api",
		})
	})

	// Public routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/send-otp", authHandler.SendOTP)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/resend-otp", authHandler.ResendOTP)
			auth.POST("/refresh", authHandler.RefreshToken)
			auth.POST("/logout", authMW.RequireAuth(), authHandler.Logout)
			auth.GET("/me", authMW.RequireAuth(), authHandler.Me)
		}

		// LibreChat routes (protected)
		librechat := v1.Group("/librechat")
		librechat.Use(authMW.RequireAuth())
		{
			librechat.GET("/credentials", libreChatHandler.GetCredentials)
			librechat.POST("/login", libreChatHandler.Login)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(authMW.RequireAuth())
		protected.Use(rlsMW.SetRLSContext())
		{
			// Users
			users := protected.Group("/users")
			{
				users.POST("", permMW.RequirePermission("users", "create"), userHandler.Create)
				users.GET("", userHandler.List)
				users.GET("/:id", userHandler.GetByID)
				users.PUT("/:id", permMW.RequirePermission("users", "update"), userHandler.Update)
				users.DELETE("/:id", permMW.RequirePermission("users", "delete"), userHandler.Delete)
				users.GET("/:id/permissions", userHandler.GetPermissions)
				users.POST("/:id/roles", permMW.RequirePermission("users", "update"), userHandler.AssignRole)
				users.DELETE("/:id/roles/:role_id", permMW.RequirePermission("users", "update"), userHandler.RemoveRole)
			}

			// Organizations
			orgs := protected.Group("/organizations")
			{
				orgs.POST("", permMW.RequirePermission("organizations", "create"), orgHandler.Create)
				orgs.GET("", orgHandler.List)
				orgs.GET("/:id", orgHandler.GetByID)
				orgs.PUT("/:id", permMW.RequirePermission("organizations", "update"), orgHandler.Update)
				orgs.DELETE("/:id", permMW.RequirePermission("organizations", "delete"), orgHandler.Delete)
			}

			// Roles
			roles := protected.Group("/roles")
			{
				roles.POST("", permMW.RequirePermission("roles", "create"), roleHandler.Create)
				roles.GET("", roleHandler.List)
				roles.GET("/:id", roleHandler.GetByID)
				roles.PUT("/:id", permMW.RequirePermission("roles", "update"), roleHandler.Update)
				roles.DELETE("/:id", permMW.RequirePermission("roles", "delete"), roleHandler.Delete)
				roles.GET("/:id/permissions", roleHandler.GetPermissions)
				roles.POST("/:id/permissions", permMW.RequirePermission("roles", "update"), roleHandler.AssignPermissions)
			}

			// Permissions
			permissions := protected.Group("/permissions")
			{
				permissions.GET("", permHandler.List)
			}

			// Templates
			templates := protected.Group("/templates")
			{
				templates.POST("", templateHandler.Create)
				templates.GET("", templateHandler.List)
				templates.GET("/:id", templateHandler.GetByID)
				templates.PUT("/:id", templateHandler.Update)
				templates.DELETE("/:id", templateHandler.Delete)
			}

			// Personas
			personas := protected.Group("/personas")
			{
				personas.POST("", personaHandler.Create)
				personas.GET("", personaHandler.List)
				personas.GET("/:id", personaHandler.GetByID)
				personas.PUT("/:id", personaHandler.Update)
				personas.DELETE("/:id", personaHandler.Delete)
			}

			// Folders - Only admin and superadmin can create/update/delete
			folders := protected.Group("/folders")
			{
				folders.POST("", permMW.RequirePermission("folders", "create"), folderHandler.Create)
				folders.GET("", folderHandler.List)
				folders.GET("/tree", folderHandler.GetTree)
				folders.GET("/:id", folderHandler.GetByID)
				folders.PUT("/:id", permMW.RequirePermission("folders", "update"), folderHandler.Update)
				folders.DELETE("/:id", permMW.RequirePermission("folders", "delete"), folderHandler.Delete)
				folders.GET("/:id/permissions", folderHandler.GetPermissions)
				folders.POST("/:id/permissions", permMW.RequirePermission("folders", "update"), folderHandler.AssignPermission)
				folders.DELETE("/:id/permissions/:role_id", permMW.RequirePermission("folders", "update"), folderHandler.RemovePermission)
			}

			// Files - Only admin and superadmin can create/update/delete
			// DEPRECATED: /api/v1/files routes - Use /api/v1/documents instead
			// All file operations now go through the documents API for consistency
			// Keeping these commented out for reference, can be removed later
			/*
				files := protected.Group("/files")
				{
					files.POST("", permMW.RequirePermission("files", "create"), fileHandler.Create)
					files.POST("/upload", permMW.RequirePermission("files", "create"), fileHandler.Upload)
					files.GET("", fileHandler.List)
					files.GET("/:id", fileHandler.GetByID)
					files.PUT("/:id", permMW.RequirePermission("files", "update"), fileHandler.Update)
					files.DELETE("/:id", permMW.RequirePermission("files", "delete"), fileHandler.Delete)
				}
			*/

			// Audit Logs - Read access for authenticated users
			auditLogs := protected.Group("/audit-logs")
			{
				auditLogs.GET("", auditLogHandler.List)
				auditLogs.GET("/:id", auditLogHandler.GetByID)
			}

			// Screeners - Save, list, run, and delete screeners
			screeners := protected.Group("/screeners")
			{
				screeners.POST("/save", screenerHandler.SaveScreener)
				screeners.GET("/saved", screenerHandler.GetSavedScreeners)
				screeners.POST("/:id/run", screenerHandler.RunSavedScreener)
				screeners.DELETE("/:id", screenerHandler.DeleteScreener)
			}

			// Documents - Upload, list, search, and delete documents
			// Only register if documentHandler is provided (requires Redis and Weaviate)
			if documentHandler != nil {
				documents := protected.Group("/documents")
				{
					documents.POST("/upload", authMW.RequireAuth(), documentHandler.UploadDocument())
					documents.GET("", documentHandler.GetDocumentsWithFilter())
					documents.GET("/search", documentHandler.SearchDocuments())
					documents.GET("/jobs/:job_id", documentHandler.GetJobStatus())
					documents.GET("/jobs", documentHandler.GetAllJobs())
					documents.GET("/:document_id/download", documentHandler.DownloadDocument())
					documents.DELETE("/:document_id", documentHandler.DeleteDocument())
				}
				log.Println("Document routes registered: /api/v1/documents")
			} else {
				log.Println("Document routes NOT registered - documentHandler is nil")
			}
		}

		// Super admin routes
		admin := v1.Group("/admin")
		admin.Use(authMW.RequireAuth())
		admin.Use(authMW.RequireSuperAdmin())
		{
			admin.GET("/users", userHandler.List)
			admin.GET("/organizations", orgHandler.List)
		}

		// Static file serving route (protected)
		// Route: /static/resources/folder/file/*
		// Uses RequireAuthForStatic to accept tokens from query params or cookies (for browser requests)
		static := router.Group("/static")
		static.Use(authMW.RequireAuthForStatic())
		static.Use(rlsMW.SetRLSContext())
		{
			static.GET("/resources/folder/file/*path", staticHandler.ServeFile)
		}
	}

	return router
}
