package websocket

import (
	"context"
	"strings"
	"sync"

	"github.com/humoroushorse/go_sprint/pkg/nats"
)

// MockNATSClient is a mock implementation of NATSClient for testing
type MockNATSClient struct {
	subscriptions map[string][]func(context.Context, *nats.Message) error
	mu            sync.RWMutex
}

// NewMockNATSClient creates a new mock NATS client
func NewMockNATSClient() *MockNATSClient {
	return &MockNATSClient{
		subscriptions: make(map[string][]func(context.Context, *nats.Message) error),
	}
}

// Subscribe subscribes to a subject with a handler
func (m *MockNATSClient) Subscribe(subject string, handler func(context.Context, *nats.Message) error) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.subscriptions[subject]; !ok {
		m.subscriptions[subject] = make([]func(context.Context, *nats.Message) error, 0)
	}
	m.subscriptions[subject] = append(m.subscriptions[subject], handler)
	return nil, nil
}

// Publish publishes a message to a subject (for testing)
func (m *MockNATSClient) Publish(ctx context.Context, subject string, msg *nats.Message) error {
	m.mu.RLock()
	handlers := make([]func(context.Context, *nats.Message) error, 0)

	// Collect all matching handlers
	for sub, subHandlers := range m.subscriptions {
		if matchSubject(sub, subject) {
			handlers = append(handlers, subHandlers...)
		}
	}
	m.mu.RUnlock()

	// Call handlers outside the lock to avoid deadlock
	for _, handler := range handlers {
		// Call handler synchronously for testing
		if err := handler(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// matchSubject checks if a subscription pattern matches a subject
func matchSubject(pattern, subject string) bool {
	// Simple wildcard matching for testing
	if pattern == subject {
		return true
	}

	// Handle wildcard (*) - matches any single token
	// For NATS-style wildcards: "sprint.*.workitem.created" matches "sprint.abc.workitem.created"
	patternParts := strings.Split(pattern, ".")
	subjectParts := strings.Split(subject, ".")

	if len(patternParts) != len(subjectParts) {
		return false
	}

	for i := range patternParts {
		if patternParts[i] == "*" {
			continue
		}
		if patternParts[i] != subjectParts[i] {
			return false
		}
	}

	return true
}

// Close closes the mock client
func (m *MockNATSClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions = make(map[string][]func(context.Context, *nats.Message) error)
	return nil
}

// IsConnected returns true (mock is always connected)
func (m *MockNATSClient) IsConnected() bool {
	return true
}
