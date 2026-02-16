package v1

import (
	"context"
	"fmt"

	"github.com/4thel00z/memories/internal"
)

// Client provides programmatic access to the memory store.
type Client struct {
	resolver *internal.ScopeResolver
	repoFor  func(internal.Scope) (*internal.GitRepository, error)
	scope    string
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

	repoFor := func(scope internal.Scope) (*internal.GitRepository, error) {
		return internal.NewGitRepository(scope)
	}

	return &Client{
		resolver: resolver,
		repoFor:  repoFor,
		scope:    cfg.scope,
	}, nil
}

// Set creates or updates a memory.
func (c *Client) Set(ctx context.Context, key string, value []byte) error {
	k, err := internal.NewKey(key)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	scope := c.resolver.Resolve(c.scope)
	repo, err := c.repoFor(scope)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	mem := internal.NewMemory(k, value)
	if err := repo.Save(ctx, mem); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	_, err = repo.Commit(ctx, fmt.Sprintf("set: %s", key))
	return err
}

// Get retrieves a memory by key.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	k, err := internal.NewKey(key)
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	for _, scope := range c.resolveScopes() {
		repo, err := c.repoFor(scope)
		if err != nil {
			continue
		}
		mem, err := repo.Get(ctx, k)
		if err != nil {
			continue
		}
		return mem.Content, nil
	}

	return nil, fmt.Errorf("key %q: %w", key, internal.ErrNotFound)
}

// Delete removes a memory.
func (c *Client) Delete(ctx context.Context, key string) error {
	k, err := internal.NewKey(key)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	scope := c.resolver.Resolve(c.scope)
	repo, err := c.repoFor(scope)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}

	if err := repo.Delete(ctx, k); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	_, err = repo.Commit(ctx, fmt.Sprintf("del: %s", key))
	return err
}

// List returns all memories matching the prefix.
func (c *Client) List(ctx context.Context, prefix string) ([]Memory, error) {
	scope := c.resolver.Resolve(c.scope)
	repo, err := c.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	mems, err := repo.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	out := make([]Memory, 0, len(mems))
	for _, m := range mems {
		out = append(out, Memory{
			Key:       m.Key.String(),
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		})
	}
	return out, nil
}

// Close releases any resources held by the client.
func (c *Client) Close() error {
	return nil
}

func (c *Client) resolveScopes() []internal.Scope {
	if c.scope != "" {
		return []internal.Scope{c.resolver.Resolve(c.scope)}
	}
	return c.resolver.Cascade()
}
