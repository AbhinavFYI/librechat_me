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
	"saas-api/pkg/postgres"

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
			log.Printf("✅ Loaded .env from: %s", path)
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
	fileRepo := repositories.NewFileRepository(db)
	auditLogRepo := repositories.NewAuditLogRepository(db)
	screenerRepo := repositories.NewScreenerRepository(db)

	// Initialize services
	tokenService := auth.NewTokenService(cfg)
	authService := services.NewAuthService(userRepo, tokenRepo, tokenService, cfg)

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
	folderHandler := handlers.NewFolderHandler(folderRepo, fileRepo)
	fileHandler := handlers.NewFileHandler(fileRepo, folderRepo, cfg.App.StoragePath)
	staticHandler := handlers.NewStaticHandler(cfg.App.StoragePath)
	libreChatHandler := handlers.NewLibreChatHandler()
	auditLogHandler := handlers.NewAuditLogHandler(auditLogRepo)
	screenerHandler := handlers.NewScreenerHandler(screenerRepo, userRepo)

	// Setup router
	// Note: Document routes are not included here as they require Redis and Weaviate
	// Document functionality is available through the dependency injection container
	router := setupRouter(cfg, authHandler, userHandler, orgHandler, roleHandler, permHandler, templateHandler, personaHandler, folderHandler, fileHandler, staticHandler, libreChatHandler, auditLogHandler, screenerHandler, nil, authMW, rlsMW, permMW)

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
	fileHandler *handlers.FileHandler,
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
			files := protected.Group("/files")
			{
				files.POST("", permMW.RequirePermission("files", "create"), fileHandler.Create)
				files.POST("/upload", permMW.RequirePermission("files", "create"), fileHandler.Upload)
				files.GET("", fileHandler.List)
				files.GET("/:id", fileHandler.GetByID)
				files.PUT("/:id", permMW.RequirePermission("files", "update"), fileHandler.Update)
				files.DELETE("/:id", permMW.RequirePermission("files", "delete"), fileHandler.Delete)
			}

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
					documents.GET("/documents", documentHandler.GetDocumentsWithFilter())
					documents.GET("/search", documentHandler.SearchDocuments())
					documents.GET("/jobs/:job_id", documentHandler.GetJobStatus())
					documents.GET("/jobs", documentHandler.GetAllJobs())
					documents.DELETE("/:document_id", documentHandler.DeleteDocument())
				}
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
