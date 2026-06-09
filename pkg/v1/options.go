package v1

// Option configures a Client.
type Option func(*clientConfig)

type clientConfig struct {
	scope string
}

// WithScope forces a specific scope (global or project).
func WithScope(scope string) Option {
	return func(c *clientConfig) {
		c.scope = scope
	}
}
