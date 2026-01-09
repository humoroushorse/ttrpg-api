package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/humoroushorse/go_sprint/internal/middleware"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error     ErrorDetail `json:"error"`
	TraceID   string      `json:"trace_id"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorDetail contains detailed error information
type ErrorDetail struct {
	Code             string            `json:"code"`
	Message          string            `json:"message"`
	Details          map[string]any    `json:"details,omitempty"`
	ValidationErrors []ValidationError `json:"validation_errors,omitempty"`
}

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes
const (
	ErrCodeValidation    = "VALIDATION_ERROR"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeConflict      = "CONFLICT"
	ErrCodeInternal      = "INTERNAL_ERROR"
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeInvalidInput  = "INVALID_INPUT"
	ErrCodeDatabaseError = "DATABASE_ERROR"
	ErrCodeServiceError  = "SERVICE_ERROR"
)

// RespondWithError sends a standardized error response
func RespondWithError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string, details map[string]any) {
	traceID := middleware.GetTraceIDFromContext(r.Context())

	errorResponse := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
		TraceID:   traceID,
		Timestamp: time.Now().UTC(),
	}

	// Log the error
	logger := middleware.LoggerFromContext(r.Context())
	logger.Error("request error",
		slog.Int("status", statusCode),
		slog.String("error_code", code),
		slog.String("error_message", message),
		slog.Any("details", details),
	)

	RespondWithJSON(w, statusCode, errorResponse)
}

// RespondWithValidationError sends a validation error response
func RespondWithValidationError(w http.ResponseWriter, r *http.Request, validationErrors []ValidationError) {
	traceID := middleware.GetTraceIDFromContext(r.Context())

	errorResponse := ErrorResponse{
		Error: ErrorDetail{
			Code:             ErrCodeValidation,
			Message:          "Validation failed",
			ValidationErrors: validationErrors,
		},
		TraceID:   traceID,
		Timestamp: time.Now().UTC(),
	}

	// Log the validation error
	logger := middleware.LoggerFromContext(r.Context())
	logger.Warn("validation error",
		slog.Int("error_count", len(validationErrors)),
		slog.Any("errors", validationErrors),
	)

	RespondWithJSON(w, http.StatusBadRequest, errorResponse)
}

// RespondWithJSON sends a JSON response
func RespondWithJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// If we can't encode the response, log it but don't try to send another response
		slog.Error("failed to encode JSON response", slog.Any("error", err))
	}
}

// RespondWithSuccess sends a successful JSON response
func RespondWithSuccess(w http.ResponseWriter, r *http.Request, statusCode int, payload any) {
	logger := middleware.LoggerFromContext(r.Context())
	logger.Debug("sending success response",
		slog.Int("status", statusCode),
	)

	RespondWithJSON(w, statusCode, payload)
}

// Common error responses

// RespondNotFound sends a 404 Not Found response
func RespondNotFound(w http.ResponseWriter, r *http.Request, resource string) {
	RespondWithError(w, r, http.StatusNotFound, ErrCodeNotFound,
		resource+" not found", nil)
}

// RespondUnauthorized sends a 401 Unauthorized response
func RespondUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "Unauthorized"
	}
	RespondWithError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, message, nil)
}

// RespondForbidden sends a 403 Forbidden response
func RespondForbidden(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "Forbidden"
	}
	RespondWithError(w, r, http.StatusForbidden, ErrCodeForbidden, message, nil)
}

// RespondConflict sends a 409 Conflict response
func RespondConflict(w http.ResponseWriter, r *http.Request, message string, details map[string]any) {
	RespondWithError(w, r, http.StatusConflict, ErrCodeConflict, message, details)
}

// RespondBadRequest sends a 400 Bad Request response
func RespondBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	RespondWithError(w, r, http.StatusBadRequest, ErrCodeBadRequest, message, nil)
}

// RespondInternalError sends a 500 Internal Server Error response
func RespondInternalError(w http.ResponseWriter, r *http.Request, err error) {
	details := make(map[string]any)
	if err != nil {
		details["error"] = err.Error()
	}
	RespondWithError(w, r, http.StatusInternalServerError, ErrCodeInternal,
		"Internal server error", details)
}
