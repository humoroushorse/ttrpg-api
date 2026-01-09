package nats

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	natsgo "github.com/nats-io/nats.go"
)

// TestNATSIntegration tests NATS client functionality
// This test requires a running NATS server
func TestNATSIntegration(t *testing.T) {
	// Skip if NATS_URL is not set (for CI/CD environments without NATS)
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
		t.Logf("NATS_URL not set, using default: %s", natsURL)
	}

	// Try to connect to NATS
	testConn, err := natsgo.Connect(natsURL)
	if err != nil {
		t.Skipf("Skipping NATS integration test: NATS server not available at %s: %v", natsURL, err)
		return
	}
	testConn.Close()

	t.Run("NewClient", testNewClient)
	t.Run("PublishAndSubscribe", testPublishAndSubscribe)
	t.Run("RequestResponse", testRequestResponse)
	t.Run("TraceIDCorrelation", testTraceIDCorrelation)
	t.Run("QueueSubscribe", testQueueSubscribe)
	t.Run("Reconnection", testReconnection)
}

func testNewClient(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ReconnectWait:  1 * time.Second,
		MaxReconnects:  5,
		ConnectionName: "test-client",
		RequestTimeout: 2 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("Client should be connected")
	}
}

func testPublishAndSubscribe(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ConnectionName: "test-pub-sub",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	// Create a channel to receive messages
	received := make(chan *Message, 1)

	// Subscribe to a test subject
	subject := "test.publish.subscribe"
	_, err = client.Subscribe(subject, func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Give subscription time to be established
	time.Sleep(100 * time.Millisecond)

	// Publish a message
	traceID := uuid.New().String()
	ctx := WithTraceID(context.Background(), traceID)

	testData := map[string]interface{}{
		"test_field": "test_value",
		"number":     42,
	}

	err = client.Publish(ctx, subject, testData)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if msg.TraceID != traceID {
			t.Errorf("Expected trace ID %s, got %s", traceID, msg.TraceID)
		}
		if msg.Service != "sprint" {
			t.Errorf("Expected service 'sprint', got %s", msg.Service)
		}
		if msg.Data["test_field"] != "test_value" {
			t.Errorf("Expected test_field 'test_value', got %v", msg.Data["test_field"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func testRequestResponse(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ConnectionName: "test-req-resp",
		RequestTimeout: 2 * time.Second,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create responder client
	responderClient, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create responder client: %v", err)
	}
	defer responderClient.Close()

	// Create requester client
	requesterClient, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create requester client: %v", err)
	}
	defer requesterClient.Close()

	// Set up responder
	subject := "test.request.response"
	_, err = responderClient.Subscribe(subject, func(ctx context.Context, msg *Message) error {
		// Send response
		responseData := map[string]interface{}{
			"echo":       msg.Data,
			"responded":  true,
			"request_id": msg.CorrelationID,
		}

		// Publish response (in real scenario, would use msg.Reply)
		// For this test, we'll use a response subject
		responseSubject := subject + ".response." + msg.CorrelationID
		return responderClient.Publish(ctx, responseSubject, responseData)
	})
	if err != nil {
		t.Fatalf("Failed to subscribe responder: %v", err)
	}

	// Give subscription time to be established
	time.Sleep(100 * time.Millisecond)

	// Send request
	traceID := uuid.New().String()
	ctx := WithTraceID(context.Background(), traceID)

	requestData := map[string]interface{}{
		"action": "test",
		"value":  123,
	}

	// For this test, we'll use a simpler publish/subscribe pattern
	// In production, use Request() method with proper reply subjects
	correlationID := uuid.New().String()
	responseSubject := subject + ".response." + correlationID

	// Subscribe to response
	responseChan := make(chan *Message, 1)
	_, err = requesterClient.Subscribe(responseSubject, func(ctx context.Context, msg *Message) error {
		responseChan <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe to response: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish request
	err = requesterClient.Publish(ctx, subject, requestData)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Wait for response
	select {
	case resp := <-responseChan:
		if resp.TraceID != traceID {
			t.Errorf("Expected trace ID %s, got %s", traceID, resp.TraceID)
		}
		if responded, ok := resp.Data["responded"].(bool); !ok || !responded {
			t.Error("Expected responded to be true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for response")
	}
}

func testTraceIDCorrelation(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ConnectionName: "test-trace-id",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	// Test trace ID propagation
	traceID := uuid.New().String()
	ctx := WithTraceID(context.Background(), traceID)

	// Verify GetTraceID works
	extractedTraceID := GetTraceID(ctx)
	if extractedTraceID != traceID {
		t.Errorf("Expected trace ID %s, got %s", traceID, extractedTraceID)
	}

	// Test that trace ID is included in messages
	received := make(chan *Message, 1)
	subject := "test.trace.correlation"

	_, err = client.Subscribe(subject, func(ctx context.Context, msg *Message) error {
		received <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	testData := map[string]interface{}{
		"test": "trace_id_correlation",
	}

	err = client.Publish(ctx, subject, testData)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	select {
	case msg := <-received:
		if msg.TraceID != traceID {
			t.Errorf("Trace ID not preserved: expected %s, got %s", traceID, msg.TraceID)
		}
		if msg.CorrelationID == "" {
			t.Error("Correlation ID should be set")
		}
		if msg.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func testQueueSubscribe(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ConnectionName: "test-queue",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create two clients for queue group
	client1, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create client1: %v", err)
	}
	defer client1.Close()

	client2, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create client2: %v", err)
	}
	defer client2.Close()

	// Publisher client
	publisher, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create publisher: %v", err)
	}
	defer publisher.Close()

	subject := "test.queue.subscribe"
	queueGroup := "test-workers"

	received1 := make(chan *Message, 10)
	received2 := make(chan *Message, 10)

	// Subscribe both clients to same queue group
	_, err = client1.QueueSubscribe(subject, queueGroup, func(ctx context.Context, msg *Message) error {
		received1 <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to queue subscribe client1: %v", err)
	}

	_, err = client2.QueueSubscribe(subject, queueGroup, func(ctx context.Context, msg *Message) error {
		received2 <- msg
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to queue subscribe client2: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Publish multiple messages
	ctx := context.Background()
	messageCount := 10

	for i := 0; i < messageCount; i++ {
		testData := map[string]interface{}{
			"message_number": i,
		}
		err = publisher.Publish(ctx, subject, testData)
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Collect messages with timeout
	timeout := time.After(3 * time.Second)
	count1 := 0
	count2 := 0
	totalReceived := 0

	for totalReceived < messageCount {
		select {
		case <-received1:
			count1++
			totalReceived++
		case <-received2:
			count2++
			totalReceived++
		case <-timeout:
			t.Fatalf("Timeout: received %d/%d messages (client1: %d, client2: %d)",
				totalReceived, messageCount, count1, count2)
		}
	}

	// Both clients should have received some messages (load balancing)
	// In practice, distribution might not be perfectly even
	t.Logf("Message distribution: client1=%d, client2=%d", count1, count2)

	if count1 == 0 || count2 == 0 {
		t.Error("Queue group should distribute messages to both subscribers")
	}
}

func testReconnection(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ReconnectWait:  100 * time.Millisecond,
		MaxReconnects:  5,
		ConnectionName: "test-reconnect",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("Client should be connected initially")
	}

	// Test that client reports connection status correctly
	// Note: We can't easily test actual reconnection without stopping/starting NATS
	// This test just verifies the connection status methods work

	// Close and verify
	err = client.Close()
	if err != nil {
		t.Errorf("Failed to close client: %v", err)
	}

	// After close, IsConnected should return false
	if client.IsConnected() {
		t.Error("Client should not be connected after close")
	}
}

// Benchmark tests

func BenchmarkPublish(b *testing.B) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = natsgo.DefaultURL
	}

	cfg := Config{
		URL:            natsURL,
		ConnectionName: "bench-publish",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce logging for benchmark
	}))

	client, err := NewClient(cfg, logger)
	if err != nil {
		b.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	subject := "bench.publish"
	testData := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := client.Publish(ctx, subject, testData)
		if err != nil {
			b.Fatalf("Failed to publish: %v", err)
		}
	}
}
