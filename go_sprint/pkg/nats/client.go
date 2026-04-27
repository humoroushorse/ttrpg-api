package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Client wraps NATS connection with standardized patterns
type Client struct {
	conn   *nats.Conn
	logger *slog.Logger
}

// Config holds NATS client configuration
type Config struct {
	URL            string        `yaml:"url"`
	ReconnectWait  time.Duration `yaml:"reconnect_wait"`
	MaxReconnects  int           `yaml:"max_reconnects"`
	ConnectionName string        `yaml:"connection_name"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

// Message represents a standardized NATS message
type Message struct {
	TraceID       string                 `json:"trace_id"`
	CorrelationID string                 `json:"correlation_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Service       string                 `json:"service"`
	Version       string                 `json:"version"`
	Data          map[string]interface{} `json:"data"`
}

// NewClient creates a new NATS client with connection management
func NewClient(cfg Config, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Set defaults
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 2 * time.Second
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = 10
	}
	if cfg.ConnectionName == "" {
		cfg.ConnectionName = "sprint-service"
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 5 * time.Second
	}

	// Configure connection options
	opts := []nats.Option{
		nats.Name(cfg.ConnectionName),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Error("NATS disconnected", slog.String("error", err.Error()))
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", slog.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Warn("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error",
				slog.String("error", err.Error()),
				slog.String("subject", sub.Subject),
			)
		}),
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("NATS client connected",
		slog.String("url", cfg.URL),
		slog.String("name", cfg.ConnectionName),
	)

	return &Client{
		conn:   conn,
		logger: logger,
	}, nil
}

// Close closes the NATS connection
func (c *Client) Close() error {
	if c.conn != nil && !c.conn.IsClosed() {
		c.conn.Close()
		c.logger.Info("NATS connection closed")
	}
	return nil
}

// IsConnected returns true if the client is connected to NATS
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}

// Publish publishes a message to a subject with trace ID correlation
func (c *Client) Publish(ctx context.Context, subject string, data interface{}) error {
	traceID := GetTraceID(ctx)

	msg := Message{
		TraceID:       traceID,
		CorrelationID: uuid.New().String(),
		Timestamp:     time.Now().UTC(),
		Service:       "sprint",
		Version:       "v1",
		Data:          convertToMap(data),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create NATS message with headers
	natsMsg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  c.createHeaders(traceID, msg.CorrelationID),
	}

	if err := c.conn.PublishMsg(natsMsg); err != nil {
		c.logger.Error("failed to publish message",
			slog.String("subject", subject),
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("failed to publish message: %w", err)
	}

	c.logger.Debug("message published",
		slog.String("subject", subject),
		slog.String("trace_id", traceID),
		slog.String("correlation_id", msg.CorrelationID),
	)

	return nil
}

// Request sends a request and waits for a response with timeout
func (c *Client) Request(ctx context.Context, subject string, data interface{}, timeout time.Duration) (*Message, error) {
	traceID := GetTraceID(ctx)

	msg := Message{
		TraceID:       traceID,
		CorrelationID: uuid.New().String(),
		Timestamp:     time.Now().UTC(),
		Service:       "sprint",
		Version:       "v1",
		Data:          convertToMap(data),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create NATS message with headers
	natsMsg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  c.createHeaders(traceID, msg.CorrelationID),
	}

	c.logger.Debug("sending request",
		slog.String("subject", subject),
		slog.String("trace_id", traceID),
		slog.Duration("timeout", timeout),
	)

	// Send request and wait for response
	resp, err := c.conn.RequestMsg(natsMsg, timeout)
	if err != nil {
		c.logger.Error("request failed",
			slog.String("subject", subject),
			slog.String("trace_id", traceID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Parse response
	var respMsg Message
	if err := json.Unmarshal(resp.Data, &respMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.logger.Debug("received response",
		slog.String("subject", subject),
		slog.String("trace_id", traceID),
		slog.String("correlation_id", respMsg.CorrelationID),
	)

	return &respMsg, nil
}

// Subscribe subscribes to a subject with a handler function
func (c *Client) Subscribe(subject string, handler func(ctx context.Context, msg *Message) error) (*nats.Subscription, error) {
	sub, err := c.conn.Subscribe(subject, func(natsMsg *nats.Msg) {
		// Parse message
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			c.logger.Error("failed to unmarshal message",
				slog.String("subject", subject),
				slog.String("error", err.Error()),
			)
			return
		}

		// Create context with trace ID
		ctx := context.WithValue(context.Background(), traceIDKey, msg.TraceID)

		c.logger.Debug("message received",
			slog.String("subject", subject),
			slog.String("trace_id", msg.TraceID),
			slog.String("correlation_id", msg.CorrelationID),
		)

		// Call handler
		if err := handler(ctx, &msg); err != nil {
			c.logger.Error("handler error",
				slog.String("subject", subject),
				slog.String("trace_id", msg.TraceID),
				slog.String("error", err.Error()),
			)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	c.logger.Info("subscribed to subject",
		slog.String("subject", subject),
	)

	return sub, nil
}

// QueueSubscribe subscribes to a subject with queue group for load balancing
func (c *Client) QueueSubscribe(subject, queue string, handler func(ctx context.Context, msg *Message) error) (*nats.Subscription, error) {
	sub, err := c.conn.QueueSubscribe(subject, queue, func(natsMsg *nats.Msg) {
		// Parse message
		var msg Message
		if err := json.Unmarshal(natsMsg.Data, &msg); err != nil {
			c.logger.Error("failed to unmarshal message",
				slog.String("subject", subject),
				slog.String("queue", queue),
				slog.String("error", err.Error()),
			)
			return
		}

		// Create context with trace ID
		ctx := context.WithValue(context.Background(), traceIDKey, msg.TraceID)

		c.logger.Debug("message received",
			slog.String("subject", subject),
			slog.String("queue", queue),
			slog.String("trace_id", msg.TraceID),
			slog.String("correlation_id", msg.CorrelationID),
		)

		// Call handler
		if err := handler(ctx, &msg); err != nil {
			c.logger.Error("handler error",
				slog.String("subject", subject),
				slog.String("queue", queue),
				slog.String("trace_id", msg.TraceID),
				slog.String("error", err.Error()),
			)
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to queue subscribe: %w", err)
	}

	c.logger.Info("subscribed to subject with queue",
		slog.String("subject", subject),
		slog.String("queue", queue),
	)

	return sub, nil
}

// createHeaders creates standard NATS message headers
func (c *Client) createHeaders(traceID, correlationID string) nats.Header {
	headers := nats.Header{}
	headers.Set("Trace-ID", traceID)
	headers.Set("Correlation-ID", correlationID)
	headers.Set("Timestamp", time.Now().UTC().Format(time.RFC3339))
	headers.Set("Service", "sprint")
	headers.Set("Version", "v1")
	return headers
}

// convertToMap converts any data to map[string]interface{}
func convertToMap(data interface{}) map[string]interface{} {
	if data == nil {
		return make(map[string]interface{})
	}

	// If already a map, return it
	if m, ok := data.(map[string]interface{}); ok {
		return m
	}

	// Convert via JSON marshaling/unmarshaling
	jsonData, err := json.Marshal(data)
	if err != nil {
		return make(map[string]interface{})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return make(map[string]interface{})
	}

	return result
}

// Context key for trace ID
type contextKey string

const traceIDKey contextKey = "trace_id"

// GetTraceID extracts trace ID from context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	// Generate new trace ID if not found
	return uuid.New().String()
}

// WithTraceID adds trace ID to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}
