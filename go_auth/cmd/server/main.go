package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/humoroushorse/go_auth/internal/handlers"
	"github.com/humoroushorse/go_auth/internal/service/auth"
	"github.com/humoroushorse/go_auth/pkg/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed swagger-ui/*
var swaggerUI embed.FS

func main() {
	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	logger.Info("Starting Authentication Service", slog.String("version", "1.0.0"))

	// Initialize Keycloak service
	keycloakService := auth.NewKeycloakService(&cfg.Keycloak)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(keycloakService, &cfg.Keycloak, &cfg.Cookie, logger)

	// Create HTTP router
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		// Could check Keycloak connectivity here
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Serve OpenAPI spec
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "api/openapi/auth.yaml")
	})

	// Serve Swagger UI
	swaggerUIFS, err := fs.Sub(swaggerUI, "swagger-ui")
	if err != nil {
		logger.Error("Failed to load Swagger UI", slog.String("error", err.Error()))
	} else {
		mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(swaggerUIFS))))
	}

	// Auth endpoints — default realm (backwards compatible)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/auth/session/token", authHandler.Login)
	mux.HandleFunc("/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/session/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/logout", authHandler.Logout)
	mux.HandleFunc("/auth/session/logout", authHandler.Logout)
	mux.HandleFunc("/auth/user", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetUser(w, r)
		case http.MethodPost:
			authHandler.Register(w, r)
		case http.MethodPut:
			authHandler.UpdateUser(w, r)
		case http.MethodDelete:
			authHandler.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/auth/register", authHandler.Register)

	// Auth endpoints — realm-prefixed (/auth/{realm}/...)
	mux.HandleFunc("/auth/{realm}/login", authHandler.Login)
	mux.HandleFunc("/auth/{realm}/session/token", authHandler.Login)
	mux.HandleFunc("/auth/{realm}/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/{realm}/session/refresh", authHandler.Refresh)
	mux.HandleFunc("/auth/{realm}/logout", authHandler.Logout)
	mux.HandleFunc("/auth/{realm}/session/logout", authHandler.Logout)
	mux.HandleFunc("/auth/{realm}/user", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetUser(w, r)
		case http.MethodPost:
			authHandler.Register(w, r)
		case http.MethodPut:
			authHandler.UpdateUser(w, r)
		case http.MethodDelete:
			authHandler.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/auth/{realm}/register", authHandler.Register)

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"service": "Authentication Service",
			"version": "1.0.0",
			"status": "operational",
			"endpoints": {
				"health": {
					"liveness": "/health/live",
					"readiness": "/health/ready"
				},
				"auth": {
					"login": "/auth/login",
					"refresh": "/auth/refresh",
					"logout": "/auth/logout",
					"user": "/auth/user",
					"register": "/auth/register"
				},
				"documentation": {
					"openapi": "/api/openapi.yaml",
					"docs": "/docs/"
				},
				"monitoring": {
					"metrics": "/metrics"
				}
			}
		}`
		w.Write([]byte(response))
	})

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      enableCORS(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server starting",
			slog.String("address", addr),
			slog.String("docs", "http://localhost"+addr+"/docs/"),
			slog.String("openapi_spec", "http://localhost"+addr+"/api/openapi.yaml"),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
	}

	logger.Info("Server exited gracefully")
}

// enableCORS adds CORS headers to allow cross-origin requests
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
