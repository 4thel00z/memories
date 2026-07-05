package main

import (
	"context"
	"sync"

	"github.com/4thel00z/memories/internal"
)

// lazyProvider is an internal.Provider that defers config loading and client
// construction until the first LLM call. It is passed (non-nil) into the use
// cases at startup, but commands that never call a provider — get, list,
// status, log — never trigger initialization or fail on a missing provider.
type lazyProvider struct {
	once sync.Once
	init func(ctx context.Context) (internal.Provider, error)
	p    internal.Provider
	err  error
}

var _ internal.Provider = (*lazyProvider)(nil)

func newLazyProvider(init func(ctx context.Context) (internal.Provider, error)) *lazyProvider {
	return &lazyProvider{init: init}
}

// resolve performs one-time initialization and returns the underlying provider.
func (l *lazyProvider) resolve(ctx context.Context) (internal.Provider, error) {
	l.once.Do(func() {
		l.p, l.err = l.init(ctx)
	})
	return l.p, l.err
}

func (l *lazyProvider) Complete(ctx context.Context, prompt string) (string, error) {
	p, err := l.resolve(ctx)
	if err != nil {
		return "", err
	}
	return p.Complete(ctx, prompt)
}

func (l *lazyProvider) GenerateObject(ctx context.Context, prompt string, target any) error {
	p, err := l.resolve(ctx)
	if err != nil {
		return err
	}
	return p.GenerateObject(ctx, prompt, target)
}

func (l *lazyProvider) Stream(ctx context.Context, prompt string) (<-chan string, error) {
	p, err := l.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return p.Stream(ctx, prompt)
}
