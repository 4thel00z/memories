package internal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// providerTestResolver returns a resolver whose global scope is an isolated
// temp dir and whose project scope is the (also temp) working directory.
func providerTestResolver(t *testing.T) *ScopeResolver {
	t.Helper()
	t.Chdir(t.TempDir())
	return &ScopeResolver{homeDir: t.TempDir()}
}

func writeProviderConfig(t *testing.T, scope Scope, cfg *Config) {
	t.Helper()
	if err := os.MkdirAll(scope.MemPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := SaveConfig(scope, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestResolveProviderNoneConfigured(t *testing.T) {
	resolver := providerTestResolver(t)

	_, err := ResolveProvider(context.Background(), resolver)
	if !errors.Is(err, ErrNoProvider) {
		t.Fatalf("err = %v, want ErrNoProvider", err)
	}
}

func TestResolveProviderExplicitDefault(t *testing.T) {
	resolver := providerTestResolver(t)

	cfg := DefaultConfig()
	cfg.Providers["openai"] = ProviderConfig{APIKey: "test-key", Model: "gpt-4o"}
	cfg.Providers["anthropic"] = ProviderConfig{APIKey: "test-key", Model: "claude-sonnet-5"}
	cfg.DefaultProvider = "anthropic"
	writeProviderConfig(t, resolver.Global(), cfg)

	provider, err := ResolveProvider(context.Background(), resolver)
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
}

func TestResolveProviderSingleImplicitDefault(t *testing.T) {
	resolver := providerTestResolver(t)

	cfg := DefaultConfig()
	cfg.Providers["openai"] = ProviderConfig{APIKey: "test-key", Model: "gpt-4o"}
	writeProviderConfig(t, resolver.Global(), cfg)

	provider, err := ResolveProvider(context.Background(), resolver)
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
}

func TestResolveProviderDanglingDefault(t *testing.T) {
	resolver := providerTestResolver(t)

	cfg := DefaultConfig()
	cfg.DefaultProvider = "ghost"
	writeProviderConfig(t, resolver.Global(), cfg)

	_, err := ResolveProvider(context.Background(), resolver)
	if err == nil || errors.Is(err, ErrNoProvider) {
		t.Fatalf("err = %v, want a dangling-default error", err)
	}
}

func TestResolveProviderProjectBeforeGlobal(t *testing.T) {
	resolver := providerTestResolver(t)

	globalCfg := DefaultConfig()
	globalCfg.Providers["openai"] = ProviderConfig{APIKey: "global-key", Model: "gpt-4o"}
	writeProviderConfig(t, resolver.Global(), globalCfg)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectScope := Scope{Type: ScopeProject, Path: cwd, MemPath: filepath.Join(cwd, ".mem")}
	projectCfg := DefaultConfig()
	projectCfg.DefaultProvider = "ghost"
	writeProviderConfig(t, projectScope, projectCfg)

	// The project scope wins, and its dangling default must surface as an
	// error rather than silently falling through to the global provider.
	_, err = ResolveProvider(context.Background(), resolver)
	if err == nil || errors.Is(err, ErrNoProvider) {
		t.Fatalf("err = %v, want project-scope dangling-default error", err)
	}
}

func TestResolveProviderEmptyKeyHostedEndpoint(t *testing.T) {
	resolver := providerTestResolver(t)

	cfg := DefaultConfig()
	cfg.Providers["openai"] = ProviderConfig{Model: "gpt-4o"}
	writeProviderConfig(t, resolver.Global(), cfg)

	_, err := ResolveProvider(context.Background(), resolver)
	if err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Fatalf("err = %v, want missing-api_key error", err)
	}
}

func TestResolveProviderEmptyKeyLocalEndpoint(t *testing.T) {
	resolver := providerTestResolver(t)

	// A custom base_url may point at a keyless local server (e.g. ollama);
	// resolution must not demand an api_key for it.
	cfg := DefaultConfig()
	cfg.Providers["openai"] = ProviderConfig{BaseURL: "http://localhost:11434/v1", Model: "llama3"}
	writeProviderConfig(t, resolver.Global(), cfg)

	provider, err := ResolveProvider(context.Background(), resolver)
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
}
