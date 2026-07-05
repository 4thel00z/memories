package v1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/4thel00z/memories/internal"
)

// Client provides programmatic access to the memory store.
type Client struct {
	uc    *internal.UseCases
	scope string
}

// Init creates a memory store: ./.mem in the working directory by default, or
// ~/.mem with WithScope("global"). An already-initialized store is a no-op, so
// Init is safe to call before every New.
func Init(opts ...Option) error {
	cfg := &clientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	resolver := internal.NewScopeResolver()

	var scope internal.Scope
	if cfg.scope == "global" {
		scope = resolver.Global()
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		scope = internal.Scope{
			Type:    internal.ScopeProject,
			Path:    cwd,
			MemPath: filepath.Join(cwd, ".mem"),
		}
	}

	if _, err := os.Stat(filepath.Join(scope.MemPath, ".git")); err == nil {
		return nil
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		return fmt.Errorf("create vectors directory: %w", err)
	}
	if err := internal.InitRepository(scope); err != nil {
		return fmt.Errorf("init repository: %w", err)
	}
	if _, err := os.Stat(scope.ConfigPath()); os.IsNotExist(err) {
		if err := internal.SaveConfig(scope, internal.DefaultConfig()); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}
	return nil
}

// New creates a new Client with the given options.
//
// If no scope is configured via WithScope and there is no project .mem store in
// the working directory tree, New returns an error rather than silently falling
// back to the user's global ~/.mem store. Pass WithScope("global") to opt into
// the global store explicitly.
func New(opts ...Option) (*Client, error) {
	cfg := &clientConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	resolver := internal.NewScopeResolver()

	if cfg.scope == "" {
		if _, ok := resolver.Project(); !ok {
			return nil, fmt.Errorf("no scope configured and no project .mem found; pass WithScope(\"global\") to use the global store")
		}
	}

	repoFor := func(scope internal.Scope) (internal.MemoryRepository, error) {
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
	}

	return &Client{
		uc:    uc,
		scope: cfg.scope,
	}, nil
}

// Set creates or updates a memory. The write and its commit happen under one
// repository lock, and re-setting an unchanged value is a no-op.
func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	if err := c.uc.SetMemory.Execute(ctx, internal.SetMemoryInput{
		Key: key, Content: string(value), Scope: c.scope,
		CommitMessage: fmt.Sprintf("set: %s", key),
	}); err != nil {
		return fmt.Errorf("set: %w", err)
	}
	return nil
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

// Delete removes a memory. The removal and its commit happen under one
// repository lock.
func (c *Client) Delete(ctx context.Context, key string) error {
	if err := c.uc.DeleteMemory.Execute(ctx, internal.DeleteMemoryInput{
		Key: key, Scope: c.scope,
		CommitMessage: fmt.Sprintf("del: %s", key),
	}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
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
