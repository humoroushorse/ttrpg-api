package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Keycloak KeycloakConfig `yaml:"keycloak"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// KeycloakConfig holds Keycloak connection configuration
type KeycloakConfig struct {
	URL          string            `yaml:"url"`
	DefaultRealm string            `yaml:"default_realm"`
	ClientID     string            `yaml:"client_id"`
	ClientSecret string            `yaml:"client_secret"`
	AdminUser    string            `yaml:"admin_user"`
	AdminPass    string            `yaml:"admin_pass"`
	RealmClients map[string]string `yaml:"realm_clients"` // realm -> clientId mapping
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	MasterURL       string        `yaml:"master_url"`
	ReplicaURL      string        `yaml:"replica_url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level        string `yaml:"level"`
	Format       string `yaml:"format"` // "json" or "console"
	EnableColors bool   `yaml:"enable_colors"`
}

// LoadFromEnv loads configuration from environment variables with sensible defaults
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvAsInt("SERVER_PORT", 8081),
			ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Keycloak: KeycloakConfig{
			URL:          getEnv("KEYCLOAK_URL", "http://localhost:8080"),
			DefaultRealm: getEnv("KEYCLOAK_DEFAULT_REALM", "sprint-management"),
			ClientID:     getEnv("KEYCLOAK_CLIENT_ID", "sprint-management-ui"),
			ClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
			AdminUser:    getEnv("KEYCLOAK_ADMIN_USER", "admin"),
			AdminPass:    getEnv("KEYCLOAK_ADMIN_PASS", "admin"),
			RealmClients: parseRealmClients(getEnv("KEYCLOAK_REALM_CLIENTS", "ttrpg:auth-service,sprint-management:sprint-management-ui")),
		},
		Database: DatabaseConfig{
			MasterURL:       getEnv("DATABASE_MASTER_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable"),
			ReplicaURL:      getEnv("DATABASE_REPLICA_URL", "postgres://postgres:admin@localhost:5432/ttrpg-pg?sslmode=disable"),
			MaxOpenConns:    getEnvAsInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvAsDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Logging: LoggingConfig{
			Level:        getEnv("LOG_LEVEL", "info"),
			Format:       getEnv("LOG_FORMAT", "json"),
			EnableColors: getEnvAsBool("LOG_ENABLE_COLORS", false),
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

	// Keycloak validation
	if c.Keycloak.URL == "" {
		return fmt.Errorf("Keycloak URL is required")
	}
	if c.Keycloak.DefaultRealm == "" {
		return fmt.Errorf("Keycloak default realm is required")
	}
	if c.Keycloak.ClientID == "" {
		return fmt.Errorf("Keycloak client ID is required")
	}

	// Database validation
	if c.Database.MasterURL == "" {
		return fmt.Errorf("database master URL is required")
	}
	if c.Database.ReplicaURL == "" {
		return fmt.Errorf("database replica URL is required")
	}

	// Logging validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	return nil
}

// Helper functions to read environment variables

// parseRealmClients parses "realm1:client1,realm2:client2" into a map
func parseRealmClients(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, pair := range strings.Split(s, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}

// GetClientIDForRealm returns the client ID for a given realm, falling back to the default ClientID
func (kc *KeycloakConfig) GetClientIDForRealm(realm string) string {
	if clientID, ok := kc.RealmClients[realm]; ok {
		return clientID
	}
	return kc.ClientID
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
