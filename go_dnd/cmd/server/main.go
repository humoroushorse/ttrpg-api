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
	"syscall"

	authmiddleware "github.com/humoroushorse/go_auth/pkg/auth/middleware"
	api "github.com/humoroushorse/go_dnd/api/generated"
	"github.com/humoroushorse/go_dnd/internal/handlers"
	"github.com/humoroushorse/go_dnd/internal/middleware"
	repoSources "github.com/humoroushorse/go_dnd/internal/repository/sources"
	repoSpells "github.com/humoroushorse/go_dnd/internal/repository/spells"
	repoUsers "github.com/humoroushorse/go_dnd/internal/repository/users"
	svcSources "github.com/humoroushorse/go_dnd/internal/service/sources"
	svcSpells "github.com/humoroushorse/go_dnd/internal/service/spells"
	svcUsers "github.com/humoroushorse/go_dnd/internal/service/users"
	"github.com/humoroushorse/go_dnd/pkg/config"
	"github.com/humoroushorse/go_dnd/pkg/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed swagger-ui/*
var swaggerUI embed.FS

// server wires all handlers and implements api.ServerInterface.
type server struct {
	spells  *handlers.SpellHandler
	sources *handlers.SourceHandler
	users   *handlers.UserHandler
	master  *pgxpool.Pool
}

func (s *server) ListSpells(w http.ResponseWriter, r *http.Request, params api.ListSpellsParams) {
	s.spells.ListSpells(w, r, params)
}
func (s *server) QuerySpells(w http.ResponseWriter, r *http.Request, params api.QuerySpellsParams) {
	s.spells.QuerySpells(w, r, params)
}
func (s *server) CreateSpell(w http.ResponseWriter, r *http.Request) {
	s.spells.CreateSpell(w, r)
}
func (s *server) BulkLoadSpells(w http.ResponseWriter, r *http.Request) {
	s.spells.BulkLoadSpells(w, r)
}
func (s *server) QuerySources(w http.ResponseWriter, r *http.Request, params api.QuerySourcesParams) {
	s.sources.QuerySources(w, r, params)
}
func (s *server) BulkLoadSources(w http.ResponseWriter, r *http.Request) {
	s.sources.BulkLoadSources(w, r)
}
func (s *server) GetUser(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	s.users.GetUser(w, r, id)
}
func (s *server) UpdateUser(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	s.users.UpdateUser(w, r, id)
}
func (s *server) HealthLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
func (s *server) HealthReady(w http.ResponseWriter, r *http.Request) {
	if err := s.master.Ping(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not ready","error":"database unhealthy"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg.Logging)
	logger.Info("starting dnd service", slog.Int("port", cfg.Server.Port))

	masterPool, err := pgxpool.New(context.Background(), cfg.Database.MasterURL)
	if err != nil {
		logger.Error("failed to connect to master db", "error", err)
		os.Exit(1)
	}
	defer masterPool.Close()

	replicaPool, err := pgxpool.New(context.Background(), cfg.Database.ReplicaURL)
	if err != nil {
		logger.Error("failed to connect to replica db", "error", err)
		os.Exit(1)
	}
	defer replicaPool.Close()

	if err := masterPool.Ping(context.Background()); err != nil {
		logger.Error("master db ping failed", "error", err)
		os.Exit(1)
	}
	if err := replicaPool.Ping(context.Background()); err != nil {
		logger.Error("replica db ping failed", "error", err)
		os.Exit(1)
	}

	spellRepo := repoSpells.NewRepository(masterPool, replicaPool)
	sourceRepo := repoSources.NewRepository(masterPool, replicaPool)
	userRepo := repoUsers.NewRepository(masterPool, replicaPool)

	srv := &server{
		spells:  handlers.NewSpellHandler(svcSpells.NewService(spellRepo, sourceRepo)),
		sources: handlers.NewSourceHandler(svcSources.NewService(sourceRepo)),
		users:   handlers.NewUserHandler(svcUsers.NewService(userRepo)),
		master:  masterPool,
	}

	jwtValidator := authmiddleware.NewJWTValidator(authmiddleware.JWTConfig{
		KeycloakURL: cfg.Auth.KeycloakURL,
		Realm:       cfg.Auth.Realm,
		ClientID:    cfg.Auth.ClientID,
	})

	optionalAuthMW := middleware.OptionalAuthMiddleware(jwtValidator, logger)

	chiRouter := api.HandlerWithOptions(srv, api.ChiServerOptions{
		BaseURL: "",
		Middlewares: []api.MiddlewareFunc{
			func(next http.Handler) http.Handler {
				return middleware.LoggingMiddleware(next)
			},
			func(next http.Handler) http.Handler {
				return middleware.CORSMiddleware(next)
			},
			func(next http.Handler) http.Handler {
				return middleware.RecoveryMiddleware(next)
			},
			func(next http.Handler) http.Handler {
				return optionalAuthMW(next)
			},
		},
	})

	// Auth proxy
	authServiceURL, err := url.Parse(cfg.Auth.ServiceURL)
	if err != nil {
		logger.Error("invalid auth service URL", "error", err)
		os.Exit(1)
	}
	authProxy := httputil.NewSingleHostReverseProxy(authServiceURL)
	authProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("auth proxy error", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"auth_service_unavailable"}`))
	}

	mux := http.NewServeMux()

	// Metrics
	mux.Handle("/metrics", promhttp.Handler())

	// OpenAPI spec
	mux.HandleFunc("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "api/openapi/combined-with-auth.yaml")
	})

	// Swagger UI → /docs/
	swaggerFS, err := fs.Sub(swaggerUI, "swagger-ui")
	if err != nil {
		logger.Error("failed to load swagger UI", "error", err)
	} else {
		mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(swaggerFS))))
	}

	// Root info endpoint — also delegates all unmatched routes to chi router
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"name":"TTRPG D&D API","version":"1.0.0","docs_url":"http://localhost:%d/docs/","openapi_url":"http://localhost:%d/api/openapi.yaml","health":{"live":"/health/live","ready":"/health/ready"}}`,
				cfg.Server.Port, cfg.Server.Port)
			return
		}
		chiRouter.ServeHTTP(w, r)
	})

	// Auth proxy — no JWT required, go_auth handles its own auth
	mux.Handle("/api/v1/auth/", http.StripPrefix("/api/v1", authProxy))
	mux.Handle("/auth/", authProxy)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("server listening",
			slog.String("addr", addr),
			slog.String("docs", fmt.Sprintf("http://localhost:%d/docs/", cfg.Server.Port)),
			slog.String("openapi_spec", fmt.Sprintf("http://localhost:%d/api/openapi.yaml", cfg.Server.Port)),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	logger.Info("server exited")
}
