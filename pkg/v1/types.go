package v1

import "time"

// Memory represents a stored memory entry.
type Memory struct {
	Key       string    `json:"key"`
	Content   []byte    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
