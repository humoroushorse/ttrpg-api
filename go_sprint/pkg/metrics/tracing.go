package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Span represents a distributed tracing span
type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]interface{}
	Events     []SpanEvent
	Status     SpanStatus
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]interface{}
}

// SpanStatus represents the status of a span
type SpanStatus struct {
	Code    StatusCode
	Message string
}

// StatusCode represents span status codes
type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOK
	StatusCodeError
)

// Tracer manages distributed tracing
type Tracer struct {
	serviceName string
	spans       map[string]*Span
}

// NewTracer creates a new tracer
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		spans:       make(map[string]*Span),
	}
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	traceID := GetTraceIDFromContext(ctx)
	if traceID == "" {
		// Check if there's a parent span with a trace ID
		if parentSpan := GetSpanFromContext(ctx); parentSpan != nil {
			traceID = parentSpan.TraceID
		} else {
			traceID = generateTraceID()
		}
	}

	span := &Span{
		TraceID:    traceID,
		SpanID:     generateSpanID(),
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     []SpanEvent{},
		Status: SpanStatus{
			Code: StatusCodeUnset,
		},
	}

	// Get parent span ID from context if available
	if parentSpan := GetSpanFromContext(ctx); parentSpan != nil {
		span.ParentID = parentSpan.SpanID
	}

	// Store span
	t.spans[span.SpanID] = span

	// Add span to context
	ctx = context.WithValue(ctx, spanContextKey, span)
	// Also ensure trace ID is in context
	ctx = SetTraceIDInContext(ctx, traceID)

	return ctx, span
}

// EndSpan ends a span
func (s *Span) End() {
	s.EndTime = time.Now()
}

// SetAttribute sets an attribute on the span
func (s *Span) SetAttribute(key string, value interface{}) {
	s.Attributes[key] = value
}

// SetStatus sets the status of the span
func (s *Span) SetStatus(code StatusCode, message string) {
	s.Status = SpanStatus{
		Code:    code,
		Message: message,
	}
}

// AddEvent adds an event to the span
func (s *Span) AddEvent(name string, attributes map[string]interface{}) {
	event := SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attributes,
	}
	s.Events = append(s.Events, event)
}

// RecordError records an error on the span
func (s *Span) RecordError(err error) {
	s.SetStatus(StatusCodeError, err.Error())
	s.AddEvent("exception", map[string]interface{}{
		"exception.type":    fmt.Sprintf("%T", err),
		"exception.message": err.Error(),
	})
}

// Context keys for tracing
type contextKey string

const (
	traceIDContextKey contextKey = "trace_id"
	spanContextKey    contextKey = "span"
)

// GetTraceIDFromContext retrieves the trace ID from context
func GetTraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDContextKey).(string); ok {
		return traceID
	}
	return ""
}

// SetTraceIDInContext sets the trace ID in context
func SetTraceIDInContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDContextKey, traceID)
}

// GetSpanFromContext retrieves the current span from context
func GetSpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey).(*Span); ok {
		return span
	}
	return nil
}

// generateTraceID generates a unique trace ID
func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// generateSpanID generates a unique span ID
func generateSpanID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// TracingMiddleware creates middleware for HTTP request tracing
func TracingMiddleware(tracer *Tracer) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start span for this request
			ctx, span := tracer.StartSpan(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			// Set HTTP attributes
			span.SetAttribute("http.method", r.Method)
			span.SetAttribute("http.url", r.URL.String())
			span.SetAttribute("http.host", r.Host)
			span.SetAttribute("http.scheme", r.URL.Scheme)
			span.SetAttribute("http.user_agent", r.UserAgent())

			// Wrap response writer to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Set response attributes
			span.SetAttribute("http.status_code", rw.statusCode)
			span.SetAttribute("http.response_size", rw.size)

			// Set span status based on HTTP status code
			if rw.statusCode >= 400 {
				span.SetStatus(StatusCodeError, fmt.Sprintf("HTTP %d", rw.statusCode))
			} else {
				span.SetStatus(StatusCodeOK, "")
			}
		})
	}
}

// TraceDatabase wraps database operations with tracing
func TraceDatabase(ctx context.Context, tracer *Tracer, operation, table string, fn func() error) error {
	ctx, span := tracer.StartSpan(ctx, "db."+operation)
	defer span.End()

	span.SetAttribute("db.operation", operation)
	span.SetAttribute("db.table", table)
	span.SetAttribute("db.system", "postgresql")

	err := fn()
	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetStatus(StatusCodeOK, "")
	return nil
}

// TraceNATS wraps NATS operations with tracing
func TraceNATS(ctx context.Context, tracer *Tracer, subject, operation string, fn func() error) error {
	ctx, span := tracer.StartSpan(ctx, "nats."+operation)
	defer span.End()

	span.SetAttribute("messaging.system", "nats")
	span.SetAttribute("messaging.destination", subject)
	span.SetAttribute("messaging.operation", operation)

	// Extract trace ID and add to span
	traceID := GetTraceIDFromContext(ctx)
	if traceID != "" {
		span.SetAttribute("trace.id", traceID)
	}

	err := fn()
	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetStatus(StatusCodeOK, "")
	return nil
}

// PropagateTraceToNATS adds trace context to NATS message headers
func PropagateTraceToNATS(ctx context.Context) map[string]string {
	headers := make(map[string]string)

	// Add trace ID
	if traceID := GetTraceIDFromContext(ctx); traceID != "" {
		headers["trace-id"] = traceID
	}

	// Add span ID if available
	if span := GetSpanFromContext(ctx); span != nil {
		headers["span-id"] = span.SpanID
		headers["parent-span-id"] = span.ParentID
	}

	return headers
}

// ExtractTraceFromNATS extracts trace context from NATS message headers
func ExtractTraceFromNATS(ctx context.Context, headers map[string]string) context.Context {
	// Extract trace ID
	if traceID, ok := headers["trace-id"]; ok {
		ctx = SetTraceIDInContext(ctx, traceID)
	}

	return ctx
}

// PerformanceMonitor tracks performance metrics
type PerformanceMonitor struct {
	tracer    *Tracer
	threshold time.Duration
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(tracer *Tracer, threshold time.Duration) *PerformanceMonitor {
	return &PerformanceMonitor{
		tracer:    tracer,
		threshold: threshold,
	}
}

// MonitorOperation monitors an operation and alerts if it exceeds threshold
func (pm *PerformanceMonitor) MonitorOperation(ctx context.Context, name string, fn func() error) error {
	ctx, span := pm.tracer.StartSpan(ctx, name)
	defer span.End()

	start := time.Now()
	err := fn()
	duration := time.Since(start)

	span.SetAttribute("duration_ms", duration.Milliseconds())

	if duration > pm.threshold {
		span.AddEvent("performance_threshold_exceeded", map[string]interface{}{
			"threshold_ms": pm.threshold.Milliseconds(),
			"actual_ms":    duration.Milliseconds(),
		})
	}

	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetStatus(StatusCodeOK, "")
	return nil
}
