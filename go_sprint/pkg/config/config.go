package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	NATS       NATSConfig       `yaml:"nats"`
	Auth       AuthConfig       `yaml:"auth"`
	Logging    LoggingConfig    `yaml:"logging"`
	Metrics    MetricsConfig    `yaml:"metrics"`
	Estimation EstimationConfig `yaml:"estimation"`
	SoftDelete SoftDeleteConfig `yaml:"soft_delete"`
	RateLimit  RateLimitConfig  `yaml:"rate_limit"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	MasterURL       string        `yaml:"master_url"`
	ReplicaURL      string        `yaml:"replica_url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// NATSConfig holds NATS connection configuration
type NATSConfig struct {
	URL           string        `yaml:"url"`
	ReconnectWait time.Duration `yaml:"reconnect_wait"`
	MaxReconnects int           `yaml:"max_reconnects"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	ServiceURL  string `yaml:"service_url"`
	KeycloakURL string `yaml:"keycloak_url"`
	Realm       string `yaml:"realm"`
	ClientID    string `yaml:"client_id"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level        string `yaml:"level"`
	Format       string `yaml:"format"` // "json" or "console"
	EnableColors bool   `yaml:"enable_colors"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// EstimationConfig holds estimation and planning configuration
type EstimationConfig struct {
	StoryPointScale []int `yaml:"story_point_scale"`
}

// SoftDeleteConfig holds soft delete configuration
type SoftDeleteConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	BurstSize         int  `yaml:"burst_size"`
}

// LoadFromEnv loads configuration from environment variables with sensible defaults
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvAsInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			MasterURL:       getEnv("DATABASE_MASTER_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable"),
			ReplicaURL:      getEnv("DATABASE_REPLICA_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable"),
			MaxOpenConns:    getEnvAsInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvAsDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		NATS: NATSConfig{
			URL:           getEnv("NATS_URL", "nats://localhost:4222"),
			ReconnectWait: getEnvAsDuration("NATS_RECONNECT_WAIT", 2*time.Second),
			MaxReconnects: getEnvAsInt("NATS_MAX_RECONNECTS", 10),
		},
		Auth: AuthConfig{
			ServiceURL:  getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
			KeycloakURL: getEnv("KEYCLOAK_URL", "http://localhost:8080"),
			Realm:       getEnv("KEYCLOAK_REALM", "ttrpg"),
			ClientID:    getEnv("KEYCLOAK_CLIENT_ID", "auth-service"),
		},
		Logging: LoggingConfig{
			Level:        getEnv("LOG_LEVEL", "info"),
			Format:       getEnv("LOG_FORMAT", "json"),
			EnableColors: getEnvAsBool("LOG_ENABLE_COLORS", false),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvAsBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		Estimation: EstimationConfig{
			StoryPointScale: getEnvAsIntSlice("STORY_POINT_SCALE", []int{0, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89}),
		},
		SoftDelete: SoftDeleteConfig{
			RetentionDays: getEnvAsInt("SOFT_DELETE_RETENTION_DAYS", 30),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvAsBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvAsInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 60),
			BurstSize:         getEnvAsInt("RATE_LIMIT_BURST_SIZE", 10),
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// Validate performs validation on the configuration
func (c *Config) Validate() error {
	// Server validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server read timeout must be positive")
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server write timeout must be positive")
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server shutdown timeout must be positive")
	}

	// Database validation
	if c.Database.MasterURL == "" {
		return fmt.Errorf("database master URL is required")
	}
	if c.Database.ReplicaURL == "" {
		return fmt.Errorf("database replica URL is required")
	}
	if c.Database.MaxOpenConns < 1 {
		return fmt.Errorf("database max open connections must be at least 1")
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database max idle connections cannot be negative")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("database max idle connections cannot exceed max open connections")
	}

	// NATS validation
	if c.NATS.URL == "" {
		return fmt.Errorf("NATS URL is required")
	}
	if c.NATS.ReconnectWait <= 0 {
		return fmt.Errorf("NATS reconnect wait must be positive")
	}
	if c.NATS.MaxReconnects < 0 {
		return fmt.Errorf("NATS max reconnects cannot be negative")
	}

	// Auth validation
	if c.Auth.ServiceURL == "" {
		return fmt.Errorf("auth service URL is required")
	}
	if c.Auth.KeycloakURL == "" {
		return fmt.Errorf("Keycloak URL is required")
	}
	if c.Auth.Realm == "" {
		return fmt.Errorf("Keycloak realm is required")
	}
	if c.Auth.ClientID == "" {
		return fmt.Errorf("Keycloak client ID is required")
	}

	// Logging validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}
	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s (must be json or console)", c.Logging.Format)
	}

	// Metrics validation
	if c.Metrics.Enabled && c.Metrics.Path == "" {
		return fmt.Errorf("metrics path is required when metrics are enabled")
	}

	// Rate limit validation
	if c.RateLimit.Enabled {
		if c.RateLimit.RequestsPerMinute < 1 {
			return fmt.Errorf("rate limit requests per minute must be at least 1")
		}
		if c.RateLimit.BurstSize < 0 {
			return fmt.Errorf("rate limit burst size cannot be negative")
		}
	}

	return nil
}

// Helper functions to read environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsIntSlice(key string, defaultValue []int) []int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	// Parse comma-separated integers
	parts := []rune(valueStr)
	result := []int{}
	current := ""

	for _, ch := range parts {
		if ch == ',' || ch == ' ' {
			if current != "" {
				if val, err := strconv.Atoi(current); err == nil {
					result = append(result, val)
				}
				current = ""
			}
		} else {
			current += string(ch)
		}
	}

	// Add last value
	if current != "" {
		if val, err := strconv.Atoi(current); err == nil {
			result = append(result, val)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}

	return result
}
