package models

import (
	"time"

	"github.com/google/uuid"
)

// Bookkeeping mirrors py_dnd's MixinBookeepingCreate + MixinBookeepingUpdate.
type Bookkeeping struct {
	CreatedAt time.Time `json:"created_at"`
	CreatedBy uuid.UUID `json:"created_by"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy uuid.UUID `json:"updated_by"`
}
