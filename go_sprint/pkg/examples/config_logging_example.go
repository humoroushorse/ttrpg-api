package examples

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/pkg/config"
	"github.com/humoroushorse/go_sprint/pkg/logging"
	"github.com/humoroushorse/go_sprint/pkg/models"
)

// DemoConfigAndLogging demonstrates how to use configuration and logging together
func DemoConfigAndLogging() {
	// Load configuration from environment
	cfg, err := config.LoadFromEnv()
	if err != nil {
		panic(err)
	}

	// Create logger based on configuration
	logger := logging.NewLogger(cfg.Logging)

	// Add trace ID to logger
	traceID := uuid.New().String()
	logger = logging.WithTraceID(logger, traceID)

	// Log some information
	logger.Info("application started",
		slog.String("version", "1.0.0"),
		slog.Int("port", cfg.Server.Port),
	)

	// Create a context with logger
	ctx := logging.WithLogger(context.Background(), logger)

	// Simulate a request handler
	handleRequest(ctx)
}

func handleRequest(ctx context.Context) {
	// Get logger from context
	logger := logging.FromContext(ctx)

	// Add user ID to logger
	userID := uuid.New().String()
	logger = logging.WithUserID(logger, userID)

	// Add request info
	logger = logging.WithRequestInfo(logger, "POST", "/api/workitems")

	// Create a work item (demonstrates LogValuer)
	workItem := models.WorkItem{
		ID:       uuid.New(),
		Type:     models.WorkItemTypeStory,
		Title:    "Implement user authentication",
		Status:   models.WorkItemStatusTodo,
		Priority: models.PriorityHigh,
	}

	// Log the work item - LogValuer ensures safe logging
	logger.Info("work item created",
		slog.Any("work_item", workItem),
	)

	// Create a user (demonstrates sensitive data protection)
	user := models.User{
		ID:       uuid.New(),
		Username: "john.doe",
		Email:    "john.doe@example.com",
		Password: "super-secret-password", // This will NOT be logged
		APIKey:   "secret-api-key",        // This will NOT be logged
	}

	// Log the user - LogValuer prevents sensitive data from being logged
	logger.Info("user authenticated",
		slog.Any("user", user),
	)

	// Create a sprint (demonstrates LogValuer)
	sprint := models.Sprint{
		ID:              uuid.New(),
		Name:            "Sprint 1",
		Status:          models.SprintStatusActive,
		CommittedPoints: 50,
		CompletedPoints: 20,
	}

	// Log the sprint
	logger.Info("sprint updated",
		slog.Any("sprint", sprint),
	)
}
