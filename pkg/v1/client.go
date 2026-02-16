package v1

import (
	"context"
	"fmt"

	"github.com/4thel00z/memories/internal"
)

// Client provides programmatic access to the memory store.
type Client struct {
	uc    *internal.UseCases
	scope string
}

// New creates a new Client with the given options.
func New(opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		dimension: 256,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	resolver := internal.NewScopeResolver()

	repoFor := func(scope internal.Scope) (internal.MemoryRepository, error) {
		return internal.NewGitRepository(scope)
	}
	histFor := func(scope internal.Scope) (internal.HistoryRepository, error) {
		return internal.NewGitRepository(scope)
	}

	nilIndex := func(scope internal.Scope) (internal.VectorIndex, error) {
		return nil, internal.ErrNoIndex
	}

	uc := &internal.UseCases{
		SetMemory:    internal.NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil),
		GetMemory:    internal.NewGetMemoryUseCase(resolver, repoFor),
		DeleteMemory: internal.NewDeleteMemoryUseCase(resolver, repoFor, nilIndex),
		ListMemories: internal.NewListMemoriesUseCase(resolver, repoFor),
		Commit:       internal.NewCommitUseCase(resolver, histFor),
	}

	return &Client{
		uc:    uc,
		scope: cfg.scope,
	}, nil
}

// Set creates or updates a memory.
func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	if err := c.uc.SetMemory.Execute(ctx, internal.SetMemoryInput{
		Key: key, Content: string(value), Scope: c.scope,
	}); err != nil {
		return fmt.Errorf("set: %w", err)
	}

	_, err := c.uc.Commit.Execute(ctx, internal.CommitInput{
		Message: fmt.Sprintf("set: %s", key), Scope: c.scope,
	})
	return err
}

// Get retrieves a memory by key.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := c.uc.GetMemory.Execute(ctx, internal.GetMemoryInput{
		Key: key, Scope: c.scope,
	})
	if err != nil {
		return nil, err
	}
	return []byte(out.Content), nil
}

// Delete removes a memory.
func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.uc.DeleteMemory.Execute(ctx, internal.DeleteMemoryInput{
		Key: key, Scope: c.scope,
	}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	_, err := c.uc.Commit.Execute(ctx, internal.CommitInput{
		Message: fmt.Sprintf("del: %s", key), Scope: c.scope,
	})
	return err
}

// List returns all memories matching the prefix.
func (c *Client) List(ctx context.Context, prefix string) ([]Memory, error) {
	out, err := c.uc.ListMemories.Execute(ctx, internal.ListMemoriesInput{
		Prefix: prefix, Scope: c.scope,
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	memories := make([]Memory, 0, len(out.Memories))
	for _, m := range out.Memories {
		memories = append(memories, Memory{
			Key:       m.Key,
			Content:   []byte(m.Content),
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		})
	}
	return memories, nil
}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	return nil
}
