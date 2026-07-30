package config

import (
	"testing"
)

func TestValidate_RejectsInvalidLogLevel(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Logging.Level = "verbose"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid log level, got nil")
	}
}

func TestValidate_AcceptsValidLogLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		cfg := validBaseConfig()
		cfg.Logging.Level = level
		if err := cfg.Validate(); err != nil {
			t.Errorf("level %q should be valid, got: %v", level, err)
		}
	}
}

func TestValidate_RejectsInvalidLogFormat(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Logging.Format = "text"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid log format, got nil")
	}
}

func TestValidate_RejectsInvalidPort(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Server.Port = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for port 0, got nil")
	}
}

// validBaseConfig returns a Config that passes Validate().
func validBaseConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8004,
			ReadTimeout:     30_000_000_000,
			WriteTimeout:    30_000_000_000,
			ShutdownTimeout: 30_000_000_000,
		},
		Database: DatabaseConfig{
			MasterURL:    "postgres://localhost/test",
			ReplicaURL:   "postgres://localhost/test",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
		},
		NATS: NATSConfig{
			URL:           "nats://localhost:4222",
			ReconnectWait: 2_000_000_000,
			MaxReconnects: 5,
		},
		Auth: AuthConfig{
			ServiceURL:  "http://localhost:8081",
			KeycloakURL: "http://localhost:8080",
			Realm:       "ttrpg",
			ClientID:    "auth-service",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
		},
	}
}
