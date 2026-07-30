package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/humoroushorse/go_dnd/internal/middleware"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	TraceID string `json:"trace_id,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func respondError(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	respondJSON(w, status, ErrorResponse{
		Error:   msg,
		Code:    code,
		TraceID: middleware.GetTraceIDFromContext(r.Context()),
	})
}

func respondNotFound(w http.ResponseWriter, r *http.Request, msg string) {
	respondError(w, r, http.StatusNotFound, "NOT_FOUND", msg)
}

func respondBadRequest(w http.ResponseWriter, r *http.Request, msg string) {
	respondError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", msg)
}

func respondUnauthorized(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
}

func respondForbidden(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusForbidden, "FORBIDDEN", "permission denied")
}

func respondInternalError(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}
