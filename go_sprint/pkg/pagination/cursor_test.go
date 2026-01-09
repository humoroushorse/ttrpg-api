package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncodeCursor(t *testing.T) {
	tests := []struct {
		name    string
		cursor  Cursor
		wantErr bool
	}{
		{
			name: "valid cursor",
			cursor: Cursor{
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				ID:        uuid.New().String(),
			},
			wantErr: false,
		},
		{
			name: "empty cursor",
			cursor: Cursor{
				Timestamp: time.Time{},
				ID:        "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeCursor(tt.cursor)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeCursor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.cursor.ID == "" && encoded != "" {
				t.Errorf("EncodeCursor() should return empty string for empty cursor")
			}

			if tt.cursor.ID != "" && encoded == "" {
				t.Errorf("EncodeCursor() should return non-empty string for valid cursor")
			}
		})
	}
}

func TestDecodeCursor(t *testing.T) {
	validCursor := Cursor{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		ID:        uuid.New().String(),
	}
	validEncoded, _ := EncodeCursor(validCursor)

	tests := []struct {
		name    string
		encoded string
		wantErr bool
	}{
		{
			name:    "valid encoded cursor",
			encoded: validEncoded,
			wantErr: false,
		},
		{
			name:    "empty cursor",
			encoded: "",
			wantErr: false,
		},
		{
			name:    "invalid base64",
			encoded: "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "invalid json",
			encoded: "aW52YWxpZC1qc29u", // base64 of "invalid-json"
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeCursor(tt.encoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeCursor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.encoded != "" {
				if decoded.ID == "" {
					t.Errorf("DecodeCursor() should return valid cursor for valid input")
				}
			}
		})
	}
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	original := Cursor{
		Timestamp: time.Date(2024, 1, 1, 12, 30, 45, 0, time.UTC),
		ID:        uuid.New().String(),
	}

	encoded, err := EncodeCursor(original)
	if err != nil {
		t.Fatalf("EncodeCursor() failed: %v", err)
	}

	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor() failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("Round trip failed: ID mismatch, got %v, want %v", decoded.ID, original.ID)
	}

	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Round trip failed: Timestamp mismatch, got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
}

func TestValidatePageSize(t *testing.T) {
	config := DefaultConfig()

	tests := []struct {
		name     string
		size     *int32
		expected int32
	}{
		{
			name:     "nil size returns default",
			size:     nil,
			expected: config.DefaultPageSize,
		},
		{
			name:     "zero size returns default",
			size:     ptr(int32(0)),
			expected: config.DefaultPageSize,
		},
		{
			name:     "negative size returns default",
			size:     ptr(int32(-10)),
			expected: config.DefaultPageSize,
		},
		{
			name:     "valid size within limits",
			size:     ptr(int32(25)),
			expected: 25,
		},
		{
			name:     "size exceeding max returns max",
			size:     ptr(int32(200)),
			expected: config.MaxPageSize,
		},
		{
			name:     "size equal to max",
			size:     ptr(int32(100)),
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePageSize(tt.size, config)
			if result != tt.expected {
				t.Errorf("ValidatePageSize() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewResponse(t *testing.T) {
	cursor := &Cursor{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		ID:        uuid.New().String(),
	}

	tests := []struct {
		name        string
		items       int
		limit       int32
		lastItem    *Cursor
		wantHasMore bool
		wantCursor  bool
	}{
		{
			name:        "full page with cursor",
			items:       50,
			limit:       50,
			lastItem:    cursor,
			wantHasMore: true,
			wantCursor:  true,
		},
		{
			name:        "partial page no cursor",
			items:       30,
			limit:       50,
			lastItem:    cursor,
			wantHasMore: false,
			wantCursor:  false,
		},
		{
			name:        "empty page",
			items:       0,
			limit:       50,
			lastItem:    nil,
			wantHasMore: false,
			wantCursor:  false,
		},
		{
			name:        "full page but no last item",
			items:       50,
			limit:       50,
			lastItem:    nil,
			wantHasMore: true,
			wantCursor:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := NewResponse(tt.items, tt.limit, tt.lastItem)
			if err != nil {
				t.Fatalf("NewResponse() error = %v", err)
			}

			if resp.HasMore != tt.wantHasMore {
				t.Errorf("NewResponse() HasMore = %v, want %v", resp.HasMore, tt.wantHasMore)
			}

			hasCursor := resp.NextCursor != nil
			if hasCursor != tt.wantCursor {
				t.Errorf("NewResponse() has cursor = %v, want %v", hasCursor, tt.wantCursor)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DefaultPageSize <= 0 {
		t.Errorf("DefaultConfig() DefaultPageSize should be positive, got %v", config.DefaultPageSize)
	}

	if config.MaxPageSize <= config.DefaultPageSize {
		t.Errorf("DefaultConfig() MaxPageSize should be greater than DefaultPageSize")
	}
}

// Helper function to create pointer to int32
func ptr(i int32) *int32 {
	return &i
}
