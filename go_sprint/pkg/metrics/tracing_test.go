package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTracer(t *testing.T) {
	tracer := NewTracer("test-service")
	if tracer == nil {
		t.Fatal("Expected tracer to be created")
	}
	if tracer.serviceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", tracer.serviceName)
	}
}

func TestStartSpan(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation")

	if span == nil {
		t.Fatal("Expected span to be created")
	}
	if span.Name != "test-operation" {
		t.Errorf("Expected span name 'test-operation', got '%s'", span.Name)
	}
	if span.TraceID == "" {
		t.Error("Expected trace ID to be set")
	}
	if span.SpanID == "" {
		t.Error("Expected span ID to be set")
	}
	if span.StartTime.IsZero() {
		t.Error("Expected start time to be set")
	}
}

func TestSpanEnd(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "test-operation")

	if !span.EndTime.IsZero() {
		t.Error("Expected end time to be zero before End() is called")
	}

	span.End()

	if span.EndTime.IsZero() {
		t.Error("Expected end time to be set after End() is called")
	}
}

func TestSpanSetAttribute(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "test-operation")

	span.SetAttribute("key1", "value1")
	span.SetAttribute("key2", 42)

	if span.Attributes["key1"] != "value1" {
		t.Errorf("Expected attribute 'key1' to be 'value1', got '%v'", span.Attributes["key1"])
	}
	if span.Attributes["key2"] != 42 {
		t.Errorf("Expected attribute 'key2' to be 42, got '%v'", span.Attributes["key2"])
	}
}

func TestSpanSetStatus(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "test-operation")

	span.SetStatus(StatusCodeOK, "success")

	if span.Status.Code != StatusCodeOK {
		t.Errorf("Expected status code OK, got %v", span.Status.Code)
	}
	if span.Status.Message != "success" {
		t.Errorf("Expected status message 'success', got '%s'", span.Status.Message)
	}
}

func TestSpanAddEvent(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "test-operation")

	span.AddEvent("test-event", map[string]interface{}{
		"key": "value",
	})

	if len(span.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "test-event" {
		t.Errorf("Expected event name 'test-event', got '%s'", span.Events[0].Name)
	}
}

func TestSpanRecordError(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "test-operation")

	testErr := errors.New("test error")
	span.RecordError(testErr)

	if span.Status.Code != StatusCodeError {
		t.Errorf("Expected status code Error, got %v", span.Status.Code)
	}
	if span.Status.Message != "test error" {
		t.Errorf("Expected status message 'test error', got '%s'", span.Status.Message)
	}
	if len(span.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "exception" {
		t.Errorf("Expected event name 'exception', got '%s'", span.Events[0].Name)
	}
}

func TestGetTraceIDFromContext(t *testing.T) {
	ctx := context.Background()

	// Test with no trace ID
	traceID := GetTraceIDFromContext(ctx)
	if traceID != "" {
		t.Errorf("Expected empty trace ID, got '%s'", traceID)
	}

	// Test with trace ID
	ctx = SetTraceIDInContext(ctx, "test-trace-123")
	traceID = GetTraceIDFromContext(ctx)
	if traceID != "test-trace-123" {
		t.Errorf("Expected trace ID 'test-trace-123', got '%s'", traceID)
	}
}

func TestGetSpanFromContext(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	// Test with no span
	span := GetSpanFromContext(ctx)
	if span != nil {
		t.Error("Expected nil span")
	}

	// Test with span
	ctx, span = tracer.StartSpan(ctx, "test-operation")
	retrievedSpan := GetSpanFromContext(ctx)
	if retrievedSpan == nil {
		t.Fatal("Expected span to be retrieved")
	}
	if retrievedSpan.SpanID != span.SpanID {
		t.Errorf("Expected span ID '%s', got '%s'", span.SpanID, retrievedSpan.SpanID)
	}
}

func TestTracingMiddleware(t *testing.T) {
	tracer := NewTracer("test-service")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify span is in context
		span := GetSpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span in context")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := TracingMiddleware(tracer)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
}

func TestTraceDatabase(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	// Test successful operation
	err := TraceDatabase(ctx, tracer, "SELECT", "users", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test failed operation
	testErr := errors.New("database error")
	err = TraceDatabase(ctx, tracer, "INSERT", "users", func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error '%v', got '%v'", testErr, err)
	}
}

func TestTraceNATS(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := SetTraceIDInContext(context.Background(), "trace-123")

	// Test successful operation
	err := TraceNATS(ctx, tracer, "test.subject", "publish", func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test failed operation
	testErr := errors.New("nats error")
	err = TraceNATS(ctx, tracer, "test.subject", "publish", func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error '%v', got '%v'", testErr, err)
	}
}

func TestPropagateTraceToNATS(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := SetTraceIDInContext(context.Background(), "trace-123")
	ctx, span := tracer.StartSpan(ctx, "test-operation")

	headers := PropagateTraceToNATS(ctx)

	if headers["trace-id"] != "trace-123" {
		t.Errorf("Expected trace-id 'trace-123', got '%s'", headers["trace-id"])
	}
	if headers["span-id"] != span.SpanID {
		t.Errorf("Expected span-id '%s', got '%s'", span.SpanID, headers["span-id"])
	}
}

func TestExtractTraceFromNATS(t *testing.T) {
	ctx := context.Background()
	headers := map[string]string{
		"trace-id": "trace-456",
	}

	ctx = ExtractTraceFromNATS(ctx, headers)

	traceID := GetTraceIDFromContext(ctx)
	if traceID != "trace-456" {
		t.Errorf("Expected trace ID 'trace-456', got '%s'", traceID)
	}
}

func TestPerformanceMonitor(t *testing.T) {
	tracer := NewTracer("test-service")
	monitor := NewPerformanceMonitor(tracer, 100*time.Millisecond)

	ctx := context.Background()

	// Test operation within threshold
	err := monitor.MonitorOperation(ctx, "fast-operation", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test operation exceeding threshold
	err = monitor.MonitorOperation(ctx, "slow-operation", func() error {
		time.Sleep(150 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test operation with error
	testErr := errors.New("operation failed")
	err = monitor.MonitorOperation(ctx, "failing-operation", func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error '%v', got '%v'", testErr, err)
	}
}

func TestSpanParentChild(t *testing.T) {
	tracer := NewTracer("test-service")
	ctx := context.Background()

	// Create parent span
	ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation")

	// Create child span
	_, childSpan := tracer.StartSpan(ctx, "child-operation")

	if childSpan.ParentID != parentSpan.SpanID {
		t.Errorf("Expected child parent ID '%s', got '%s'", parentSpan.SpanID, childSpan.ParentID)
	}
	if childSpan.TraceID != parentSpan.TraceID {
		t.Errorf("Expected child trace ID '%s', got '%s'", parentSpan.TraceID, childSpan.TraceID)
	}
}
