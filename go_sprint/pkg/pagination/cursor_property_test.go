package pagination

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: go-sprint-management, Property: Cursor Round Trip
// For any valid cursor, encoding then decoding should produce an equivalent cursor
func TestProperty_CursorRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("encoding then decoding preserves cursor data", prop.ForAll(
		func(timestamp time.Time, idBytes [16]byte) bool {
			// Generate UUID from bytes
			id := uuid.UUID(idBytes).String()

			cursor := Cursor{
				Timestamp: timestamp,
				ID:        id,
			}

			// Encode
			encoded, err := EncodeCursor(cursor)
			if err != nil {
				t.Logf("Encoding failed: %v", err)
				return false
			}

			// Decode
			decoded, err := DecodeCursor(encoded)
			if err != nil {
				t.Logf("Decoding failed: %v", err)
				return false
			}

			// Verify ID matches
			if decoded.ID != cursor.ID {
				t.Logf("ID mismatch: got %v, want %v", decoded.ID, cursor.ID)
				return false
			}

			// Verify timestamp matches (within reasonable precision)
			if !decoded.Timestamp.Equal(cursor.Timestamp) {
				t.Logf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, cursor.Timestamp)
				return false
			}

			return true
		},
		gen.Time(),
		gen.Const([16]byte{}).Map(func(v [16]byte) [16]byte {
			return uuid.New()
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Page Size Validation
// For any page size input, ValidatePageSize should return a value within configured bounds
func TestProperty_PageSizeValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)
	config := DefaultConfig()

	properties.Property("validated page size is always within bounds", prop.ForAll(
		func(size int32) bool {
			result := ValidatePageSize(&size, config)

			// Result should never be negative
			if result < 0 {
				t.Logf("Result is negative: %v", result)
				return false
			}

			// Result should never exceed max
			if result > config.MaxPageSize {
				t.Logf("Result exceeds max: %v > %v", result, config.MaxPageSize)
				return false
			}

			// Result should be default or the requested size (capped at max)
			if size <= 0 {
				return result == config.DefaultPageSize
			} else if size > config.MaxPageSize {
				return result == config.MaxPageSize
			} else {
				return result == size
			}
		},
		gen.Int32Range(-1000, 1000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Pagination Response Consistency
// For any pagination response, HasMore should be true only when items equal limit
func TestProperty_PaginationResponseConsistency(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("HasMore is true only when items equal limit", prop.ForAll(
		func(itemCount int, limit int32) bool {
			// Ensure valid inputs
			if itemCount < 0 || limit <= 0 {
				return true // Skip invalid inputs
			}

			// Create a cursor if we have items
			var cursor *Cursor
			if itemCount > 0 {
				cursor = &Cursor{
					Timestamp: time.Now(),
					ID:        uuid.New().String(),
				}
			}

			resp, err := NewResponse(itemCount, limit, cursor)
			if err != nil {
				t.Logf("NewResponse failed: %v", err)
				return false
			}

			// HasMore should be true only when itemCount equals limit
			expectedHasMore := itemCount == int(limit)
			if resp.HasMore != expectedHasMore {
				t.Logf("HasMore mismatch: got %v, want %v (items=%d, limit=%d)",
					resp.HasMore, expectedHasMore, itemCount, limit)
				return false
			}

			// NextCursor should be present only when HasMore is true and cursor was provided
			hasCursor := resp.NextCursor != nil
			expectedHasCursor := expectedHasMore && cursor != nil
			if hasCursor != expectedHasCursor {
				t.Logf("NextCursor presence mismatch: got %v, want %v", hasCursor, expectedHasCursor)
				return false
			}

			return true
		},
		gen.IntRange(0, 200),
		gen.Int32Range(1, 100),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Empty Cursor Handling
// For any empty cursor string, decoding should return an empty cursor without error
func TestProperty_EmptyCursorHandling(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("empty cursor decodes to empty cursor", prop.ForAll(
		func() bool {
			decoded, err := DecodeCursor("")
			if err != nil {
				t.Logf("Decoding empty cursor failed: %v", err)
				return false
			}

			if decoded.ID != "" {
				t.Logf("Empty cursor should have empty ID, got: %v", decoded.ID)
				return false
			}

			return true
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Cursor Encoding Determinism
// For any cursor, encoding it multiple times should produce the same result
func TestProperty_CursorEncodingDeterminism(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("encoding is deterministic", prop.ForAll(
		func(timestamp time.Time, idBytes [16]byte) bool {
			// Generate UUID from bytes
			id := uuid.UUID(idBytes).String()

			cursor := Cursor{
				Timestamp: timestamp,
				ID:        id,
			}

			// Encode multiple times
			encoded1, err1 := EncodeCursor(cursor)
			if err1 != nil {
				t.Logf("First encoding failed: %v", err1)
				return false
			}

			encoded2, err2 := EncodeCursor(cursor)
			if err2 != nil {
				t.Logf("Second encoding failed: %v", err2)
				return false
			}

			// Results should be identical
			if encoded1 != encoded2 {
				t.Logf("Encodings differ: %v != %v", encoded1, encoded2)
				return false
			}

			return true
		},
		gen.Time(),
		gen.Const([16]byte{}).Map(func(v [16]byte) [16]byte {
			return uuid.New()
		}),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Page Size Boundary Conditions
// For any page size at boundaries (0, default, max), validation should handle correctly
func TestProperty_PageSizeBoundaryConditions(t *testing.T) {
	properties := gopter.NewProperties(nil)
	config := DefaultConfig()

	properties.Property("boundary values are handled correctly", prop.ForAll(
		func(boundary int) bool {
			var size int32
			switch boundary % 4 {
			case 0:
				size = 0
			case 1:
				size = config.DefaultPageSize
			case 2:
				size = config.MaxPageSize
			case 3:
				size = config.MaxPageSize + 1
			}

			result := ValidatePageSize(&size, config)

			// Verify result is within bounds
			if result < 1 || result > config.MaxPageSize {
				t.Logf("Boundary result out of bounds: %v", result)
				return false
			}

			// Verify specific boundary behavior
			if size == 0 {
				return result == config.DefaultPageSize
			} else if size == config.DefaultPageSize {
				return result == config.DefaultPageSize
			} else if size == config.MaxPageSize {
				return result == config.MaxPageSize
			} else if size > config.MaxPageSize {
				return result == config.MaxPageSize
			}

			return true
		},
		gen.Int(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
