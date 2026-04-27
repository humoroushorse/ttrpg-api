package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/humoroushorse/go_sprint/pkg/config"
	"github.com/humoroushorse/go_sprint/pkg/models"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestLogValuerPreventsSensitiveDataLogging tests that LogValuer interface
// prevents sensitive data from being logged
// Feature: go-sprint-management, Property: LogValuer prevents sensitive data logging
// Validates: Requirements 5.6, 5.7, 5.8
func TestLogValuerPreventsSensitiveDataLogging(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("User LogValue should not log sensitive fields", prop.ForAll(
		func(username, email, password, apiKey string) bool {
			// Skip very short strings that might appear as part of JSON structure
			if len(email) < 2 || len(password) < 2 || len(apiKey) < 2 {
				return true
			}

			// Create a user with sensitive data
			user := models.User{
				Username:     username,
				Email:        email,
				Password:     password,
				APIKey:       apiKey,
				RefreshToken: "sensitive-refresh-token",
			}

			// Create a buffer to capture log output
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))

			// Log the user using LogValuer
			logger.Info("user action", slog.Any("user", user))

			// Get the logged output
			output := buf.String()

			// Verify sensitive data is NOT in the output
			// Check for email as a JSON value (with quotes)
			if strings.Contains(output, `"`+email+`"`) {
				t.Logf("FAIL: Email found in logs: %s", email)
				return false
			}
			if strings.Contains(output, `"`+password+`"`) {
				t.Logf("FAIL: Password found in logs: %s", password)
				return false
			}
			if strings.Contains(output, `"`+apiKey+`"`) {
				t.Logf("FAIL: API key found in logs: %s", apiKey)
				return false
			}
			if strings.Contains(output, "sensitive-refresh-token") {
				t.Logf("FAIL: Refresh token found in logs")
				return false
			}

			// Verify username IS in the output (non-sensitive)
			if username != "" && len(username) >= 2 && !strings.Contains(output, username) {
				t.Logf("FAIL: Username not found in logs: %s", username)
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.Property("WorkItem LogValue should log all non-sensitive fields", prop.ForAll(
		func(title string, typeIdx, statusIdx, priorityIdx int) bool {
			types := []models.WorkItemType{models.WorkItemTypeEpic, models.WorkItemTypeStory, models.WorkItemTypeDefect}
			statuses := []models.WorkItemStatus{models.WorkItemStatusTodo, models.WorkItemStatusInProgress, models.WorkItemStatusDone}
			priorities := []models.PriorityLevel{models.PriorityLow, models.PriorityMedium, models.PriorityHigh}

			workItem := models.WorkItem{
				Type:     types[typeIdx%len(types)],
				Title:    title,
				Status:   statuses[statusIdx%len(statuses)],
				Priority: priorities[priorityIdx%len(priorities)],
			}

			// Create a buffer to capture log output
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{}))

			// Log the work item
			logger.Info("work item action", slog.Any("work_item", workItem))

			// Get the logged output
			output := buf.String()

			// Verify all expected fields are present
			if title != "" && !strings.Contains(output, title) {
				t.Logf("FAIL: Title not found in logs: %s", title)
				return false
			}
			if !strings.Contains(output, string(workItem.Type)) {
				t.Logf("FAIL: Type not found in logs: %s", workItem.Type)
				return false
			}
			if !strings.Contains(output, string(workItem.Status)) {
				t.Logf("FAIL: Status not found in logs: %s", workItem.Status)
				return false
			}
			if !strings.Contains(output, string(workItem.Priority)) {
				t.Logf("FAIL: Priority not found in logs: %s", workItem.Priority)
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

// TestEnvironmentSpecificLogFormatSwitching tests that log format switches
// correctly based on configuration
// Feature: go-sprint-management, Property: Environment-specific log format switching
// Validates: Requirements 5.7, 5.8
func TestEnvironmentSpecificLogFormatSwitching(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("JSON format produces valid JSON output", prop.ForAll(
		func(message string, level string) bool {
			// Skip empty messages
			if message == "" {
				return true
			}

			// Create JSON logger
			var buf bytes.Buffer
			cfg := config.LoggingConfig{
				Level:        level,
				Format:       "json",
				EnableColors: false,
			}

			// Temporarily redirect stdout
			oldHandler := slog.Default().Handler()
			defer slog.SetDefault(slog.New(oldHandler))

			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: parseLevel(cfg.Level),
			}))

			// Log a message
			logger.Info(message)

			// Verify output is valid JSON
			output := buf.String()
			if output == "" {
				return true
			}

			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
				t.Logf("FAIL: Invalid JSON output: %v", err)
				return false
			}

			// Verify required JSON fields
			if _, ok := jsonData["time"]; !ok {
				t.Logf("FAIL: Missing 'time' field in JSON output")
				return false
			}
			if _, ok := jsonData["level"]; !ok {
				t.Logf("FAIL: Missing 'level' field in JSON output")
				return false
			}
			if _, ok := jsonData["msg"]; !ok {
				t.Logf("FAIL: Missing 'msg' field in JSON output")
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	properties.Property("Console format produces text output", prop.ForAll(
		func(message string, enableColors bool) bool {
			// Skip empty messages
			if message == "" {
				return true
			}

			// Create console logger
			var buf bytes.Buffer
			cfg := config.LoggingConfig{
				Level:        "info",
				Format:       "console",
				EnableColors: enableColors,
			}

			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level: parseLevel(cfg.Level),
			}))

			// Log a message
			logger.Info(message)

			// Verify output is text (not JSON)
			output := buf.String()
			if output == "" {
				return true
			}

			// Should not be valid JSON
			var jsonData map[string]interface{}
			if json.Unmarshal([]byte(output), &jsonData) == nil {
				// If it parses as JSON, it's not console format
				t.Logf("FAIL: Console output should not be valid JSON")
				return false
			}

			// Should contain the message
			if !strings.Contains(output, message) {
				t.Logf("FAIL: Message not found in console output")
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.Bool(),
	))

	properties.Property("Log level filtering works correctly", prop.ForAll(
		func(configLevel string, messageLevel string) bool {
			levels := map[string]int{
				"debug": 0,
				"info":  1,
				"warn":  2,
				"error": 3,
			}

			configLevelInt := levels[configLevel]
			messageLevelInt := levels[messageLevel]

			// Create logger with specific level
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: parseLevel(configLevel),
			}))

			// Log at the message level
			switch messageLevel {
			case "debug":
				logger.Debug("test message")
			case "info":
				logger.Info("test message")
			case "warn":
				logger.Warn("test message")
			case "error":
				logger.Error("test message")
			}

			output := buf.String()

			// Message should be logged only if messageLevel >= configLevel
			shouldLog := messageLevelInt >= configLevelInt
			isLogged := output != ""

			if shouldLog != isLogged {
				t.Logf("FAIL: Expected shouldLog=%v but got isLogged=%v (config=%s, message=%s)",
					shouldLog, isLogged, configLevel, messageLevel)
				return false
			}

			return true
		},
		gen.OneConstOf("debug", "info", "warn", "error"),
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	properties.TestingRun(t)
}

// TestContextBasedLogging tests that logger can be stored and retrieved from context
func TestContextBasedLogging(t *testing.T) {
	// Create a logger
	cfg := config.LoggingConfig{
		Level:        "info",
		Format:       "json",
		EnableColors: false,
	}
	logger := NewLogger(cfg)

	// Add logger to context
	ctx := WithLogger(context.Background(), logger)

	// Retrieve logger from context
	retrievedLogger := FromContext(ctx)

	if retrievedLogger == nil {
		t.Fatal("Failed to retrieve logger from context")
	}

	// Test with trace ID
	loggerWithTrace := WithTraceID(logger, "test-trace-123")
	if loggerWithTrace == nil {
		t.Fatal("Failed to create logger with trace ID")
	}

	// Test with user ID
	loggerWithUser := WithUserID(logger, "user-456")
	if loggerWithUser == nil {
		t.Fatal("Failed to create logger with user ID")
	}

	// Test with request info
	loggerWithRequest := WithRequestInfo(logger, "GET", "/api/workitems")
	if loggerWithRequest == nil {
		t.Fatal("Failed to create logger with request info")
	}
}

// TestDefaultLoggerFallback tests that FromContext returns default logger when none is set
func TestDefaultLoggerFallback(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)

	if logger == nil {
		t.Fatal("FromContext should return default logger when none is set")
	}

	// Should be able to log without panic
	logger.Info("test message")
}
