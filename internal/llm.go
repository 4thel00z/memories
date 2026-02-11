package internal

import "context"

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	Device() string
	Close() error
}

type Provider interface {
	Complete(ctx context.Context, prompt string) (string, error)
	GenerateObject(ctx context.Context, prompt string, target any) error
	Stream(ctx context.Context, prompt string) (<-chan string, error)
}

// Structured output types for AI features

type Summary struct {
	Title     string   `json:"title"`
	Overview  string   `json:"overview"`
	KeyPoints []string `json:"key_points"`
	Tags      []string `json:"tags"`
}

type AutoTag struct {
	Tags       []string `json:"tags"`
	Category   string   `json:"category"`
	Confidence float32  `json:"confidence"`
}
