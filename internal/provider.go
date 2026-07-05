package internal

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoProvider indicates no LLM provider is configured in any scope.
var ErrNoProvider = errors.New("no LLM provider configured; run `mem provider add <name>`")

// ResolveProvider builds the default LLM provider from the first scope in the
// cascade that has one configured. A scope with exactly one provider and no
// explicit default uses that provider.
func ResolveProvider(ctx context.Context, resolver *ScopeResolver) (Provider, error) {
	for _, scope := range resolver.Cascade() {
		cfg, err := LoadConfig(scope)
		if err != nil {
			continue
		}

		name := cfg.DefaultProvider
		if name == "" {
			switch len(cfg.Providers) {
			case 0:
				continue
			case 1:
				for only := range cfg.Providers {
					name = only
				}
			default:
				return nil, fmt.Errorf("multiple providers configured but no default; run `mem provider default <name>`")
			}
		}

		pc, ok := cfg.Providers[name]
		if !ok {
			return nil, fmt.Errorf("default provider %q not found in %s", name, scope.ConfigPath())
		}

		// A hosted endpoint cannot work without a key; fail here with a usable
		// message instead of surfacing a raw HTTP 401 later. A custom base_url
		// may point at a keyless local server, so only the default endpoint is
		// checked.
		if pc.APIKey == "" && pc.BaseURL == "" {
			return nil, fmt.Errorf("provider %q has no api_key configured; set one with `mem provider add %s`", name, name)
		}

		return NewFantasyProvider(ctx, FantasyConfig{
			Provider: name,
			APIKey:   pc.APIKey,
			BaseURL:  pc.BaseURL,
			Model:    pc.Model,
		})
	}

	return nil, ErrNoProvider
}
