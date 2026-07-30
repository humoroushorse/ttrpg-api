package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	NATS     NATSConfig     `yaml:"nats"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type DatabaseConfig struct {
	MasterURL       string        `yaml:"master_url"`
	ReplicaURL      string        `yaml:"replica_url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type NATSConfig struct {
	URL           string        `yaml:"url"`
	ReconnectWait time.Duration `yaml:"reconnect_wait"`
	MaxReconnects int           `yaml:"max_reconnects"`
}

type AuthConfig struct {
	ServiceURL  string `yaml:"service_url"`
	KeycloakURL string `yaml:"keycloak_url"`
	Realm       string `yaml:"realm"`
	ClientID    string `yaml:"client_id"`
}

type LoggingConfig struct {
	Level        string `yaml:"level"`
	Format       string `yaml:"format"` // "json" or "console"
	EnableColors bool   `yaml:"enable_colors"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvAsInt("SERVER_PORT", 8004),
			ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			MasterURL:       getEnv("DATABASE_MASTER_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable&search_path=dnd"),
			ReplicaURL:      getEnv("DATABASE_REPLICA_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable&search_path=dnd"),
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
			Format:       getEnv("LOG_FORMAT", getDefaultLogFormat()),
			EnableColors: getEnvAsBool("LOG_ENABLE_COLORS", isLocalDevelopment()),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvAsBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
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
	if c.NATS.URL == "" {
		return fmt.Errorf("NATS URL is required")
	}
	if c.NATS.ReconnectWait <= 0 {
		return fmt.Errorf("NATS reconnect wait must be positive")
	}
	if c.NATS.MaxReconnects < 0 {
		return fmt.Errorf("NATS max reconnects cannot be negative")
	}
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
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}
	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s (must be json or console)", c.Logging.Format)
	}
	if c.Metrics.Enabled && c.Metrics.Path == "" {
		return fmt.Errorf("metrics path is required when metrics are enabled")
	}
	return nil
}

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

func isLocalDevelopment() bool {
	env := os.Getenv("ENVIRONMENT")
	if env == "development" || env == "dev" || env == "local" {
		return true
	}
	if env == "production" || env == "prod" || env == "staging" {
		return false
	}
	// default to production-safe behavior when ENVIRONMENT is unset
	return false
}

func getDefaultLogFormat() string {
	if isLocalDevelopment() {
		return "console"
	}
	return "json"
}
