package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestConfigValidation tests the configuration validation logic
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:        "info",
					Format:       "json",
					EnableColors: false,
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server port - too low",
			config: &Config{
				Server: ServerConfig{
					Port:            0,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid server port - too high",
			config: &Config{
				Server: ServerConfig{
					Port:            70000,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "missing database master URL",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "database master URL is required",
		},
		{
			name: "invalid max idle connections",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    30, // More than max open
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "database max idle connections cannot exceed max open connections",
		},
		{
			name: "invalid log level",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "invalid",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
		{
			name: "invalid log format",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "nats://localhost:4222",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "xml",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "invalid log format",
		},
		{
			name: "missing NATS URL",
			config: &Config{
				Server: ServerConfig{
					Port:            8080,
					ReadTimeout:     30 * time.Second,
					WriteTimeout:    30 * time.Second,
					ShutdownTimeout: 30 * time.Second,
				},
				Database: DatabaseConfig{
					MasterURL:       "postgres://localhost/db",
					ReplicaURL:      "postgres://localhost/db",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
				},
				NATS: NATSConfig{
					URL:           "",
					ReconnectWait: 2 * time.Second,
					MaxReconnects: 10,
				},
				Auth: AuthConfig{
					ServiceURL:  "http://localhost:8081",
					KeycloakURL: "http://localhost:8080",
					Realm:       "test-realm",
					ClientID:    "test-client",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Metrics: MetricsConfig{
					Enabled: true,
					Path:    "/metrics",
				},
			},
			wantErr: true,
			errMsg:  "NATS URL is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Config.Validate() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestLoadFromEnv tests loading configuration from environment variables
func TestLoadFromEnv(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"SERVER_PORT",
		"DATABASE_MASTER_URL",
		"DATABASE_REPLICA_URL",
		"NATS_URL",
		"AUTH_SERVICE_URL",
		"KEYCLOAK_URL",
		"KEYCLOAK_REALM",
		"KEYCLOAK_CLIENT_ID",
		"LOG_LEVEL",
		"LOG_FORMAT",
	}
	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
	}

	// Restore environment after test
	defer func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	t.Run("load with defaults", func(t *testing.T) {
		// Clear all environment variables
		for _, key := range envVars {
			os.Unsetenv(key)
		}

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("LoadFromEnv() error = %v", err)
		}

		// Check defaults
		if cfg.Server.Port != 8080 {
			t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
		}
		if cfg.Logging.Level != "info" {
			t.Errorf("Expected default log level 'info', got %s", cfg.Logging.Level)
		}
		if cfg.Logging.Format != "json" {
			t.Errorf("Expected default log format 'json', got %s", cfg.Logging.Format)
		}
	})

	t.Run("load with custom values", func(t *testing.T) {
		// Set custom environment variables
		os.Setenv("SERVER_PORT", "9090")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_FORMAT", "console")
		os.Setenv("LOG_ENABLE_COLORS", "true")

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("LoadFromEnv() error = %v", err)
		}

		// Check custom values
		if cfg.Server.Port != 9090 {
			t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
		}
		if cfg.Logging.Level != "debug" {
			t.Errorf("Expected log level 'debug', got %s", cfg.Logging.Level)
		}
		if cfg.Logging.Format != "console" {
			t.Errorf("Expected log format 'console', got %s", cfg.Logging.Format)
		}
		if !cfg.Logging.EnableColors {
			t.Errorf("Expected colors enabled, got false")
		}
	})

	t.Run("fail-fast on invalid configuration", func(t *testing.T) {
		// Set invalid port
		os.Setenv("SERVER_PORT", "99999")

		_, err := LoadFromEnv()
		if err == nil {
			t.Error("Expected error for invalid port, got nil")
		}
	})
}

// TestEnvironmentVariableHelpers tests the helper functions
func TestEnvironmentVariableHelpers(t *testing.T) {
	t.Run("getEnv", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test_value")
		defer os.Unsetenv("TEST_VAR")

		if got := getEnv("TEST_VAR", "default"); got != "test_value" {
			t.Errorf("getEnv() = %v, want %v", got, "test_value")
		}

		if got := getEnv("NONEXISTENT_VAR", "default"); got != "default" {
			t.Errorf("getEnv() = %v, want %v", got, "default")
		}
	})

	t.Run("getEnvAsInt", func(t *testing.T) {
		os.Setenv("TEST_INT", "42")
		defer os.Unsetenv("TEST_INT")

		if got := getEnvAsInt("TEST_INT", 0); got != 42 {
			t.Errorf("getEnvAsInt() = %v, want %v", got, 42)
		}

		if got := getEnvAsInt("NONEXISTENT_INT", 99); got != 99 {
			t.Errorf("getEnvAsInt() = %v, want %v", got, 99)
		}

		os.Setenv("INVALID_INT", "not_a_number")
		defer os.Unsetenv("INVALID_INT")
		if got := getEnvAsInt("INVALID_INT", 99); got != 99 {
			t.Errorf("getEnvAsInt() with invalid value = %v, want default %v", got, 99)
		}
	})

	t.Run("getEnvAsBool", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")

		if got := getEnvAsBool("TEST_BOOL", false); got != true {
			t.Errorf("getEnvAsBool() = %v, want %v", got, true)
		}

		if got := getEnvAsBool("NONEXISTENT_BOOL", false); got != false {
			t.Errorf("getEnvAsBool() = %v, want %v", got, false)
		}
	})

	t.Run("getEnvAsDuration", func(t *testing.T) {
		os.Setenv("TEST_DURATION", "5s")
		defer os.Unsetenv("TEST_DURATION")

		if got := getEnvAsDuration("TEST_DURATION", 0); got != 5*time.Second {
			t.Errorf("getEnvAsDuration() = %v, want %v", got, 5*time.Second)
		}

		if got := getEnvAsDuration("NONEXISTENT_DURATION", 10*time.Second); got != 10*time.Second {
			t.Errorf("getEnvAsDuration() = %v, want %v", got, 10*time.Second)
		}
	})
}
