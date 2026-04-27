package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	authmiddleware "github.com/humoroushorse/go_auth/pkg/auth/middleware"
	api "github.com/humoroushorse/go_sprint/api/generated"
	"github.com/humoroushorse/go_sprint/internal/adapters"
	"github.com/humoroushorse/go_sprint/internal/handlers"
	"github.com/humoroushorse/go_sprint/internal/middleware"
	"github.com/humoroushorse/go_sprint/internal/repository"
	dependenciesRepo "github.com/humoroushorse/go_sprint/internal/repository/dependencies"
	sprintRepo "github.com/humoroushorse/go_sprint/internal/repository/sprints"
	workitemRepo "github.com/humoroushorse/go_sprint/internal/repository/workitems"
	burndownService "github.com/humoroushorse/go_sprint/internal/service/burndown"
	dependenciesService "github.com/humoroushorse/go_sprint/internal/service/dependencies"
	estimationService "github.com/humoroushorse/go_sprint/internal/service/estimation"
	sprintService "github.com/humoroushorse/go_sprint/internal/service/sprints"
	workitemService "github.com/humoroushorse/go_sprint/internal/service/workitems"
	"github.com/humoroushorse/go_sprint/pkg/cache"
	"github.com/humoroushorse/go_sprint/pkg/config"
	"github.com/humoroushorse/go_sprint/pkg/logging"
	"github.com/humoroushorse/go_sprint/pkg/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
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
	logger := logging.NewLogger(cfg.Logging)
	logger.Info("Starting Sprint Management Service",
		slog.String("version", "1.0.0"),
		slog.String("log_format", cfg.Logging.Format),
		slog.Bool("colors_enabled", cfg.Logging.EnableColors))

	// Test log levels to verify configuration
	logger.Debug("This is a DEBUG log - you should only see this if log level is DEBUG")
	logger.Info("This is an INFO log - you should see this if log level is INFO or DEBUG")
	logger.Warn("This is a WARN log - you should see this if log level is WARN, INFO, or DEBUG")
	logger.Error("This is an ERROR log - you should always see this unless log level is OFF")

	// Initialize database connection pools
	logger.Info("Connecting to database...")
	masterPool, err := pgxpool.New(context.Background(), cfg.Database.MasterURL)
	if err != nil {
		logger.Error("Failed to connect to master database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer masterPool.Close()

	replicaPool, err := pgxpool.New(context.Background(), cfg.Database.ReplicaURL)
	if err != nil {
		logger.Error("Failed to connect to replica database", slog.String("error", err.Error()))
		masterPool.Close()
		os.Exit(1)
	}
	defer replicaPool.Close()

	// Verify database connections
	if err := masterPool.Ping(context.Background()); err != nil {
		logger.Error("Failed to ping master database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := replicaPool.Ping(context.Background()); err != nil {
		logger.Error("Failed to ping replica database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("Database connected successfully")

	// Initialize repositories (they only need one pool - use master for writes)
	sprintRepository := sprintRepo.New(masterPool)
	workitemRepository := workitemRepo.New(masterPool)
	dependenciesRepository := dependenciesRepo.NewRepository(masterPool, replicaPool)
	projectRepository := repository.NewProjectRepository(masterPool, replicaPool)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(context.Background(), logger)
	go wsHub.Run()
	logger.Info("WebSocket hub started")

	// Initialize services
	sprintSvc := sprintService.NewService(sprintRepository, workitemRepository, logger)
	workitemSvc := workitemService.NewService(workitemRepository, projectRepository, wsHub, logger)
	burndownSvc := burndownService.NewService(sprintRepository, workitemRepository, logger)
	estimationSvc := estimationService.NewService(sprintRepository, workitemRepository, estimationService.Config{
		StoryPointScale: []int{1, 2, 3, 5, 8, 13, 21},
	}, logger)
	dependenciesSvc := dependenciesService.NewService(dependenciesRepository, logger)

	// Initialize cache with 5 minute TTL
	cacheInstance := cache.New(5 * time.Minute)

	// Initialize handlers
	sprintHandler := handlers.NewSprintHandler(sprintSvc, burndownSvc, estimationSvc, logger)
	workitemHandler := handlers.NewWorkItemHandler(workitemSvc, dependenciesSvc, logger, cacheInstance)

	// Initialize project repository and handler
	projectRepo := repository.NewProjectRepository(masterPool, replicaPool)
	projectHandler := handlers.NewProjectHandler(projectRepo, logger)

	// Initialize JWT validator for authentication
	jwtValidator := authmiddleware.NewJWTValidator(authmiddleware.JWTConfig{
		KeycloakURL: cfg.Auth.KeycloakURL,
		Realm:       cfg.Auth.Realm,
		ClientID:    cfg.Auth.ClientID,
	})

	// Initialize WebSocket handler with JWT adapter
	jwtAdapter := adapters.NewJWTValidatorAdapter(jwtValidator)
	wsHandler := websocket.NewHandler(wsHub, jwtAdapter, logger)

	// Setup reverse proxy to auth service
	authServiceURL, err := url.Parse(cfg.Auth.ServiceURL)
	if err != nil {
		logger.Error("Failed to parse auth service URL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	authProxy := httputil.NewSingleHostReverseProxy(authServiceURL)

	// Customize proxy to handle errors gracefully
	authProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Auth proxy error",
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"auth_service_unavailable","message":"Authentication service is unavailable"}`))
	}

	// Create HTTP router
	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		// Check database connection
		if err := masterPool.Ping(r.Context()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not ready","error":"database unhealthy"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// WebSocket endpoint (auth handled by WebSocket handler via token query param)
	mux.Handle("/api/v1/ws", middleware.LoggingMiddleware(logger)(http.HandlerFunc(wsHandler.ServeHTTP)))

	// Serve OpenAPI spec
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "api/openapi/combined-with-auth.yaml")
	})

	// Serve Swagger UI
	swaggerUIFS, err := fs.Sub(swaggerUI, "swagger-ui")
	if err != nil {
		logger.Error("Failed to load Swagger UI", slog.String("error", err.Error()))
	} else {
		mux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerUIFS))))
	}

	// Proxy auth endpoints to go_auth service
	mux.Handle("/api/v1/auth/", http.StripPrefix("/api/v1", authProxy))

	// Create authentication middleware
	authMiddlewareFunc := middleware.AuthMiddleware(jwtValidator, logger)

	// Create logging middleware
	loggingMiddlewareFunc := middleware.LoggingMiddleware(logger)

	// Work items endpoints
	mux.Handle("/api/v1/workitems", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Parse query parameters for ListWorkItems
			params := api.ListWorkItemsParams{}
			if limit := r.URL.Query().Get("limit"); limit != "" {
				if limitInt, err := strconv.Atoi(limit); err == nil {
					params.Limit = &limitInt
				}
			}
			if cursor := r.URL.Query().Get("cursor"); cursor != "" {
				params.Cursor = &cursor
			}
			if typ := r.URL.Query().Get("type"); typ != "" {
				workItemType := api.WorkItemType(typ)
				params.Type = &workItemType
			}
			if status := r.URL.Query().Get("status"); status != "" {
				workItemStatus := api.WorkItemStatus(status)
				params.Status = &workItemStatus
			}
			workitemHandler.ListWorkItems(w, r, params)
		case http.MethodPost:
			workitemHandler.CreateWorkItem(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Work item by ID endpoint
	mux.Handle("/api/v1/workitems/", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract path after /api/v1/workitems/
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/workitems/")
		if path == "" {
			http.Error(w, "Work item ID required", http.StatusBadRequest)
			return
		}

		// Check if this is a search endpoint
		if path == "search" {
			// TODO: Implement search endpoint
			http.Error(w, "Search endpoint not yet implemented", http.StatusNotImplemented)
			return
		}

		// Split path into segments
		segments := strings.Split(path, "/")

		// Parse work item ID from first segment
		id, err := uuid.Parse(segments[0])
		if err != nil {
			http.Error(w, "Invalid work item ID", http.StatusBadRequest)
			return
		}

		// Handle sub-resources
		if len(segments) > 1 {
			subResource := segments[1]
			switch subResource {
			case "dependencies":
				if len(segments) > 2 {
					// DELETE /api/v1/workitems/{id}/dependencies/{dependency_id}
					depID, err := uuid.Parse(segments[2])
					if err != nil {
						http.Error(w, "Invalid dependency ID", http.StatusBadRequest)
						return
					}
					if r.Method == http.MethodDelete {
						workitemHandler.DeleteWorkItemDependency(w, r, openapi_types.UUID(id), openapi_types.UUID(depID))
					} else {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
				} else {
					// GET or POST /api/v1/workitems/{id}/dependencies
					switch r.Method {
					case http.MethodGet:
						workitemHandler.GetWorkItemDependencies(w, r, openapi_types.UUID(id))
					case http.MethodPost:
						workitemHandler.CreateWorkItemDependency(w, r, openapi_types.UUID(id))
					default:
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
				}
			case "children":
				if r.Method == http.MethodGet {
					workitemHandler.GetChildWorkItems(w, r, openapi_types.UUID(id))
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.NotFound(w, r)
			}
			return
		}

		// Handle work item by ID (no sub-resource)
		switch r.Method {
		case http.MethodGet:
			workitemHandler.GetWorkItem(w, r, openapi_types.UUID(id))
		case http.MethodPut:
			workitemHandler.UpdateWorkItem(w, r, openapi_types.UUID(id))
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Sprints endpoints
	mux.Handle("/api/v1/sprints", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Parse query parameters for ListSprints
			params := api.ListSprintsParams{}
			if limit := r.URL.Query().Get("limit"); limit != "" {
				if limitInt, err := strconv.Atoi(limit); err == nil {
					params.Limit = &limitInt
				}
			}
			if cursor := r.URL.Query().Get("cursor"); cursor != "" {
				params.Cursor = &cursor
			}
			if status := r.URL.Query().Get("status"); status != "" {
				sprintStatus := api.SprintStatus(status)
				params.Status = &sprintStatus
			}
			sprintHandler.ListSprints(w, r, params)
		case http.MethodPost:
			sprintHandler.CreateSprint(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Sprint by ID endpoint
	mux.Handle("/api/v1/sprints/", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract path after /api/v1/sprints/
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/sprints/")
		if path == "" {
			http.Error(w, "Sprint ID required", http.StatusBadRequest)
			return
		}

		// Split path into segments
		segments := strings.Split(path, "/")

		// Check if this is a planning endpoint
		if segments[0] == "planning" && len(segments) >= 2 {
			switch segments[1] {
			case "capacity":
				if r.Method == http.MethodPost {
					sprintHandler.CalculateSprintCapacity(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "forecast":
				if r.Method == http.MethodPost {
					sprintHandler.ForecastSprintCompletion(w, r)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.NotFound(w, r)
			}
			return
		}

		// Parse sprint ID from first segment
		id, err := uuid.Parse(segments[0])
		if err != nil {
			http.Error(w, "Invalid sprint ID", http.StatusBadRequest)
			return
		}

		// Handle sub-resources
		if len(segments) > 1 {
			subResource := segments[1]
			switch subResource {
			case "metrics":
				if r.Method == http.MethodGet {
					sprintHandler.GetSprintMetrics(w, r, openapi_types.UUID(id))
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "burndown":
				if r.Method == http.MethodGet {
					sprintHandler.GetSprintBurndown(w, r, openapi_types.UUID(id))
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "velocity":
				if r.Method == http.MethodGet {
					sprintHandler.GetSprintVelocity(w, r, openapi_types.UUID(id))
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "workitems":
				if r.Method == http.MethodGet {
					params := api.GetSprintWorkItemsParams{}
					if status := r.URL.Query().Get("status"); status != "" {
						statusEnum := api.GetSprintWorkItemsParamsStatus(status)
						params.Status = &statusEnum
					}
					sprintHandler.GetSprintWorkItems(w, r, openapi_types.UUID(id), params)
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			case "close":
				if r.Method == http.MethodPost {
					sprintHandler.CloseSprint(w, r, openapi_types.UUID(id))
				} else {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			default:
				http.NotFound(w, r)
			}
			return
		}

		// Handle sprint by ID (no sub-resource)
		switch r.Method {
		case http.MethodGet:
			sprintHandler.GetSprint(w, r, openapi_types.UUID(id))
		case http.MethodPut:
			sprintHandler.UpdateSprint(w, r, openapi_types.UUID(id))
		case http.MethodDelete:
			sprintHandler.DeleteSprint(w, r, openapi_types.UUID(id))
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Projects endpoints
	mux.Handle("/api/v1/projects", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			projectHandler.ListProjects(w, r)
		case http.MethodPost:
			projectHandler.CreateProject(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Project by ID or key endpoint
	mux.Handle("/api/v1/projects/", loggingMiddlewareFunc(authMiddlewareFunc(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/")
		segments := strings.Split(path, "/")

		if len(segments) == 0 || segments[0] == "" {
			http.Error(w, "Invalid project path", http.StatusBadRequest)
			return
		}

		// Check if it's /projects/key/:key
		if segments[0] == "key" && len(segments) >= 2 {
			projectHandler.GetProjectByKey(w, r)
			return
		}

		// Otherwise it's /projects/:id
		switch r.Method {
		case http.MethodGet:
			projectHandler.GetProject(w, r)
		case http.MethodPut:
			projectHandler.UpdateProject(w, r)
		case http.MethodDelete:
			projectHandler.DeleteProject(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := `{
			"service": "Sprint Management API",
			"version": "1.0.0",
			"status": "operational",
			"endpoints": {
				"health": {
					"liveness": "/health/live",
					"readiness": "/health/ready"
				},
				"api": {
					"auth": {
						"login": "/api/v1/auth/login",
						"refresh": "/api/v1/auth/refresh",
						"logout": "/api/v1/auth/logout",
						"user": "/api/v1/auth/user",
						"register": "/api/v1/auth/register"
					},
					"workitems": "/api/v1/workitems",
					"sprints": "/api/v1/sprints"
				},
				"documentation": {
					"openapi": "/api/openapi.yaml",
					"swagger": "/swagger/"
				},
				"monitoring": {
					"metrics": "/metrics"
				}
			},
			"note": "This is a minimal server implementation. Full API functionality requires OpenAPI code generation and handler wiring."
		}`
		w.Write([]byte(response))
	})

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server starting",
			slog.String("address", addr),
			slog.String("swagger_ui", "http://localhost"+addr+"/swagger/"),
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
