package v1

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	cacheDir  string
	dimension int
	scope     string
}

// WithCacheDir sets the model cache directory.
func WithCacheDir(dir string) Option {
	return func(c *clientConfig) {
		c.cacheDir = dir
	}
}

// WithDimension sets the embedding dimension.
func WithDimension(dim int) Option {
	return func(c *clientConfig) {
		c.dimension = dim
	}
}

// WithScope forces a specific scope (global or project).
func WithScope(scope string) Option {
	return func(c *clientConfig) {
		c.scope = scope
	}
}
