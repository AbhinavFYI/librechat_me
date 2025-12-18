package configs

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// Server configurations
	Server ServerConfig

	// Weaviate configurations
	WeaviateHost   string
	WeaviatePort   string
	WeaviateScheme string

	// Database configurations (Read)
	DbUser     string
	DbPassword string
	DbHost     string
	DbPort     string
	DbName     string

	// Write database configurations
	DbWuser     string
	DbWpassword string
	DbWhost     string
	DbWport     string
	DbWname     string

	// JWT configurations
	JWT JWTConfig

	// Redis configurations
	MemoryDBRedisURL      string
	MemoryDBRedisUsername string
	MemoryDBRedisPassword string

	// Application configurations
	AppEnv      string
	LogLevel    string
	StoragePath string
}

// ServerConfig holds server-related configurations
type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  int // seconds
	WriteTimeout int // seconds
	IdleTimeout  int // seconds
}

// JWTConfig holds JWT-related configurations
type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  int // minutes
	RefreshTokenTTL int // days
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_HOST", "0.0.0.0")
	viper.SetDefault("SERVER_READ_TIMEOUT", 15)
	viper.SetDefault("SERVER_WRITE_TIMEOUT", 15)
	viper.SetDefault("SERVER_IDLE_TIMEOUT", 60)
	viper.SetDefault("JWT_SECRET", "your-super-secret-jwt-key-change-in-production")
	viper.SetDefault("JWT_ACCESS_TTL", 1440) // 1 day in minutes
	viper.SetDefault("JWT_REFRESH_TTL", 7)   // 7 days
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("STORAGE_PATH", "uploads")
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("WEAVIATE_HOST", "10.10.6.13")
	viper.SetDefault("WEAVIATE_PORT", "7080")
	viper.SetDefault("WEAVIATE_SCHEME", "http")

	return &Config{
		// Server configurations
		Server: ServerConfig{
			Port:         viper.GetString("SERVER_PORT"),
			Host:         viper.GetString("SERVER_HOST"),
			ReadTimeout:  viper.GetInt("SERVER_READ_TIMEOUT"),
			WriteTimeout: viper.GetInt("SERVER_WRITE_TIMEOUT"),
			IdleTimeout:  viper.GetInt("SERVER_IDLE_TIMEOUT"),
		},

		// Weaviate configurations
		WeaviateHost:   viper.GetString("WEAVIATE_HOST"),
		WeaviatePort:   viper.GetString("WEAVIATE_PORT"),
		WeaviateScheme: viper.GetString("WEAVIATE_SCHEME"),
		DbUser:         viper.GetString("ALCHEMY_DB_USER"),
		DbPassword:     viper.GetString("ALCHEMY_DB_PASSWORD"),
		DbHost:         viper.GetString("ALCHEMY_DB_HOST"),
		DbPort:         viper.GetString("ALCHEMY_DB_PORT"),
		DbName:         "nucleus", //TODO: Change this to correct db name

		// Write database configurations
		DbWuser:     viper.GetString("ALCHEMY_DB_W_USER"),
		DbWpassword: viper.GetString("ALCHEMY_DB_W_PASSWORD"),
		DbWhost:     viper.GetString("ALCHEMY_DB_W_HOST"),
		DbWport:     viper.GetString("ALCHEMY_DB_W_PORT"),
		DbWname:     "nucleus", //TODO: Change this to correct db name

		// JWT configurations
		JWT: JWTConfig{
			SecretKey:       viper.GetString("JWT_SECRET"),
			AccessTokenTTL:  viper.GetInt("JWT_ACCESS_TTL"),
			RefreshTokenTTL: viper.GetInt("JWT_REFRESH_TTL"),
		},

		// Redis configurations
		MemoryDBRedisURL:      viper.GetString("REDIS_URL"),
		MemoryDBRedisUsername: viper.GetString("REDIS_USERNAME"),
		MemoryDBRedisPassword: viper.GetString("REDIS_PASSWORD"),

		// Application configurations
		AppEnv:      viper.GetString("APP_ENV"),
		LogLevel:    viper.GetString("LOG_LEVEL"),
		StoragePath: viper.GetString("STORAGE_PATH"),
	}
}
