package nats

import (
	"context"
	"sync"
)

// MockClient is a mock implementation of NATS client for testing
type MockClient struct {
	subscriptions map[string][]func(context.Context, []byte) error
	mu            sync.RWMutex
}

// NewMockClient creates a new mock NATS client
func NewMockClient() *MockClient {
	return &MockClient{
		subscriptions: make(map[string][]func(context.Context, []byte) error),
	}
}

// Subscribe subscribes to a subject with a handler
func (m *MockClient) Subscribe(ctx context.Context, subject string, handler func(context.Context, []byte) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.subscriptions[subject]; !ok {
		m.subscriptions[subject] = make([]func(context.Context, []byte) error, 0)
	}
	m.subscriptions[subject] = append(m.subscriptions[subject], handler)
	return nil
}

// Publish publishes a message to a subject
func (m *MockClient) Publish(ctx context.Context, subject string, data []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Call all handlers for matching subjects
	for sub, handlers := range m.subscriptions {
		if matchSubject(sub, subject) {
			for _, handler := range handlers {
				go handler(ctx, data)
			}
		}
	}
	return nil
}

// matchSubject checks if a subscription pattern matches a subject
func matchSubject(pattern, subject string) bool {
	// Simple wildcard matching for testing
	// In production, use proper NATS wildcard matching
	if pattern == subject {
		return true
	}

	// Handle single wildcard (*)
	// For simplicity, just check if pattern contains * and subject starts with prefix
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(subject) >= len(prefix) && subject[:len(prefix)] == prefix
	}

	return false
}

// Close closes the mock client
func (m *MockClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions = make(map[string][]func(context.Context, []byte) error)
	return nil
}

// IsConnected returns true (mock is always connected)
func (m *MockClient) IsConnected() bool {
	return true
}
