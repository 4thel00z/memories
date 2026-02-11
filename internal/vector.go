package internal

import "context"

type Embedding struct {
	Vector    []float32
	Dimension int
	Model     string
}

func NewEmbedding(vec []float32, model string) Embedding {
	return Embedding{
		Vector:    vec,
		Dimension: len(vec),
		Model:     model,
	}
}

type SearchResult struct {
	Key   Key
	Score float32 // 0-1, higher is better
}

type VectorIndex interface {
	Add(ctx context.Context, key Key, emb Embedding) error
	Remove(ctx context.Context, key Key) error
	Search(ctx context.Context, query Embedding, k int) ([]SearchResult, error)
	Build(ctx context.Context, numTrees int) error
	Save(ctx context.Context) error
	Load(ctx context.Context) error
	Contains(ctx context.Context, key Key) bool
}
