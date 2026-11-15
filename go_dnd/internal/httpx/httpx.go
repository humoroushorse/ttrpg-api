// Package httpx writes JSON responses and errors in the shape both D&D APIs share.
//
// Error bodies carry `detail` (what py_dnd's FastAPI sends and the UI reads) and the steering's
// `error`, `code` and `trace_id`, so clients of either API work unchanged.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/humoroushorse/go_dnd/internal/middleware"
	"github.com/humoroushorse/go_dnd/pkg/logging"
)

// Error is an error with the HTTP status and code it should be answered with.
type Error struct {
	Status int
	Code   string
	Detail string
	// Issues is a list the client can act on (what breaks a campaign's rules), sent as `issues`.
	Issues any
}

func (e *Error) Error() string { return e.Detail }

func newError(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Detail: fmt.Sprintf(format, args...)}
}

// BadRequest is a 400: the request can't be read at all (not JSON, not multipart, wrong file type).
func BadRequest(format string, args ...any) *Error {
	return newError(http.StatusBadRequest, "BAD_REQUEST", format, args...)
}

// Unprocessable is a 422: the request is readable but invalid (unknown field, bad filter, wrong type).
func Unprocessable(format string, args ...any) *Error {
	return newError(http.StatusUnprocessableEntity, "VALIDATION_ERROR", format, args...)
}

// NotFound is a 404.
func NotFound(format string, args ...any) *Error {
	return newError(http.StatusNotFound, "NOT_FOUND", format, args...)
}

// Forbidden is a 403: the caller may see the thing but not do this to it.
func Forbidden(format string, args ...any) *Error {
	return newError(http.StatusForbidden, "FORBIDDEN", format, args...)
}

// Gone is a 410: it existed but can't be used anymore (an expired or revoked invite).
func Gone(format string, args ...any) *Error {
	return newError(http.StatusGone, "GONE", format, args...)
}

// Conflict is a 409: a natural key that's taken, or a delete something still depends on.
func Conflict(format string, args ...any) *Error {
	return newError(http.StatusConflict, "CONFLICT", format, args...)
}

// TooManyRequests is a 429: slow down (pings at the table).
func TooManyRequests(format string, args ...any) *Error {
	return newError(http.StatusTooManyRequests, "TOO_MANY_REQUESTS", format, args...)
}

// Unauthorized is a 401.
func Unauthorized() *Error {
	return newError(http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
}

type errorBody struct {
	Detail  string `json:"detail"`
	Error   string `json:"error"`
	Code    string `json:"code"`
	TraceID string `json:"trace_id,omitempty"`
	Issues  any    `json:"issues,omitempty"`
}

// WriteJSON writes v as the JSON body.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteRaw writes a body that is already JSON (built by Postgres).
func WriteRaw(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteError answers with err's status if it's an *Error, otherwise logs it and sends a 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		logging.FromContext(r.Context()).Error("request failed", "error", err)
		e = newError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal Error")
	}
	WriteJSON(w, e.Status, errorBody{
		Detail:  e.Detail,
		Error:   e.Detail,
		Code:    e.Code,
		TraceID: middleware.GetTraceIDFromContext(r.Context()),
		Issues:  e.Issues,
	})
}

// Timestamp formats a time the way every API response does (2026-11-07T05:15:52.790080Z).
func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000Z") }
