package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	c := New(time.Minute)

	// Set a value
	c.Set("key1", "value1")

	// Get the value
	value, exists := c.Get("key1")
	if !exists {
		t.Fatal("Expected key to exist")
	}

	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}
}

func TestCache_GetNonExistent(t *testing.T) {
	c := New(time.Minute)

	value, exists := c.Get("nonexistent")
	if exists {
		t.Error("Expected key to not exist")
	}

	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
	}
}

func TestCache_Expiration(t *testing.T) {
	c := New(100 * time.Millisecond)

	// Set a value
	c.Set("key1", "value1")

	// Should exist immediately
	_, exists := c.Get("key1")
	if !exists {
		t.Fatal("Expected key to exist immediately")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not exist after expiration
	_, exists = c.Get("key1")
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestCache_CustomTTL(t *testing.T) {
	c := New(time.Minute)

	// Set with custom short TTL
	c.SetWithTTL("key1", "value1", 100*time.Millisecond)

	// Should exist immediately
	_, exists := c.Get("key1")
	if !exists {
		t.Fatal("Expected key to exist immediately")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should not exist after expiration
	_, exists = c.Get("key1")
	if exists {
		t.Error("Expected key to be expired")
	}
}

func TestCache_Delete(t *testing.T) {
	c := New(time.Minute)

	// Set a value
	c.Set("key1", "value1")

	// Delete it
	c.Delete("key1")

	// Should not exist
	_, exists := c.Get("key1")
	if exists {
		t.Error("Expected key to be deleted")
	}
}

func TestCache_Clear(t *testing.T) {
	c := New(time.Minute)

	// Set multiple values
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")

	// Clear all
	c.Clear()

	// None should exist
	_, exists := c.Get("key1")
	if exists {
		t.Error("Expected cache to be cleared")
	}

	if c.Size() != 0 {
		t.Errorf("Expected size 0, got %d", c.Size())
	}
}

func TestCache_Size(t *testing.T) {
	c := New(time.Minute)

	if c.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", c.Size())
	}

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	if c.Size() != 2 {
		t.Errorf("Expected size 2, got %d", c.Size())
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := New(time.Minute)
	ctx := context.Background()

	computeCalled := false
	compute := func(ctx context.Context) (interface{}, error) {
		computeCalled = true
		return "computed", nil
	}

	// First call should compute
	value, err := c.GetOrSet(ctx, "key1", compute)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}

	if !computeCalled {
		t.Error("Expected compute to be called")
	}

	if value != "computed" {
		t.Errorf("Expected 'computed', got %v", value)
	}

	// Second call should use cache
	computeCalled = false
	value, err = c.GetOrSet(ctx, "key1", compute)
	if err != nil {
		t.Fatalf("GetOrSet failed: %v", err)
	}

	if computeCalled {
		t.Error("Expected compute to not be called (cached)")
	}

	if value != "computed" {
		t.Errorf("Expected 'computed', got %v", value)
	}
}

func TestCache_GetOrSetError(t *testing.T) {
	c := New(time.Minute)
	ctx := context.Background()

	expectedErr := errors.New("compute error")
	compute := func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	}

	value, err := c.GetOrSet(ctx, "key1", compute)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
	}

	// Should not be cached
	_, exists := c.Get("key1")
	if exists {
		t.Error("Expected error result to not be cached")
	}
}

func TestCache_InvalidatePattern(t *testing.T) {
	c := New(time.Minute)

	// Set multiple values with different prefixes
	c.Set("user:1", "value1")
	c.Set("user:2", "value2")
	c.Set("sprint:1", "value3")
	c.Set("sprint:2", "value4")

	// Invalidate user: prefix
	c.InvalidatePattern("user:")

	// User keys should be gone
	_, exists := c.Get("user:1")
	if exists {
		t.Error("Expected user:1 to be invalidated")
	}

	_, exists = c.Get("user:2")
	if exists {
		t.Error("Expected user:2 to be invalidated")
	}

	// Sprint keys should still exist
	_, exists = c.Get("sprint:1")
	if !exists {
		t.Error("Expected sprint:1 to still exist")
	}

	_, exists = c.Get("sprint:2")
	if !exists {
		t.Error("Expected sprint:2 to still exist")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New(time.Minute)

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			c.Set("key", i)
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have a value
	_, exists := c.Get("key")
	if !exists {
		t.Error("Expected key to exist after concurrent writes")
	}
}
