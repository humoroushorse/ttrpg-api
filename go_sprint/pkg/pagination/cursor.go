package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Cursor represents a pagination cursor containing the position information
type Cursor struct {
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
}

// Config holds pagination configuration
type Config struct {
	DefaultPageSize int32
	MaxPageSize     int32
}

// DefaultConfig returns the default pagination configuration
func DefaultConfig() Config {
	return Config{
		DefaultPageSize: 50,
		MaxPageSize:     100,
	}
}

// EncodeCursor encodes a cursor to a base64 string
func EncodeCursor(c Cursor) (string, error) {
	if c.ID == "" {
		return "", nil
	}

	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}

	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeCursor decodes a base64 cursor string
func DecodeCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, nil
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return Cursor{}, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return cursor, nil
}

// ValidatePageSize validates and normalizes the page size
func ValidatePageSize(size *int32, config Config) int32 {
	if size == nil || *size <= 0 {
		return config.DefaultPageSize
	}

	if *size > config.MaxPageSize {
		return config.MaxPageSize
	}

	return *size
}

// Response represents a paginated response
type Response struct {
	HasMore    bool
	NextCursor *string
}

// NewResponse creates a pagination response
func NewResponse(items int, limit int32, lastItem *Cursor) (Response, error) {
	hasMore := items == int(limit)

	var nextCursor *string
	if hasMore && lastItem != nil {
		encoded, err := EncodeCursor(*lastItem)
		if err != nil {
			return Response{}, fmt.Errorf("failed to encode cursor: %w", err)
		}
		if encoded != "" {
			nextCursor = &encoded
		}
	}

	return Response{
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

// Errors
var (
	ErrInvalidCursor   = errors.New("invalid cursor format")
	ErrInvalidPageSize = errors.New("invalid page size")
)
