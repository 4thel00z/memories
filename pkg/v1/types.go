package v1

import "time"

// Memory represents a stored memory entry.
type Memory struct {
	Key       string    `json:"key"`
	Content   []byte    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SearchResult represents a semantic search hit.
type SearchResult struct {
	Key   string  `json:"key"`
	Score float32 `json:"score"`
}

// Commit represents a git commit in the memory store.
type Commit struct {
	Hash      string    `json:"hash"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
