package main

import (
	"context"
	"sync"

	"github.com/4thel00z/memories/internal"
)

// lazyEmbedder is an internal.Embedder that defers the expensive model
// download/load until the first time an embedding operation is actually needed.
// It is passed (non-nil) into the use cases at startup, but commands that never
// embed — get, list, status, log — never trigger initialization.
type lazyEmbedder struct {
	once sync.Once
	init func() (internal.Embedder, error)
	emb  internal.Embedder
	err  error
}

var _ internal.Embedder = (*lazyEmbedder)(nil)

func newLazyEmbedder(init func() (internal.Embedder, error)) *lazyEmbedder {
	return &lazyEmbedder{init: init}
}

// resolve performs one-time initialization and returns the underlying embedder.
func (l *lazyEmbedder) resolve() (internal.Embedder, error) {
	l.once.Do(func() {
		l.emb, l.err = l.init()
	})
	return l.emb, l.err
}

func (l *lazyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return e.Embed(ctx, text)
}

func (l *lazyEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e, err := l.resolve()
	if err != nil {
		return nil, err
	}
	return e.EmbedBatch(ctx, texts)
}

func (l *lazyEmbedder) Dimension() int {
	e, err := l.resolve()
	if err != nil {
		return 0
	}
	return e.Dimension()
}

func (l *lazyEmbedder) Device() string {
	e, err := l.resolve()
	if err != nil {
		return ""
	}
	return e.Device()
}

func (l *lazyEmbedder) Close() error {
	// Only close if initialization actually happened; never trigger it here.
	if l.emb != nil {
		return l.emb.Close()
	}
	return nil
}
