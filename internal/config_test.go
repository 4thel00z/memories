package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Embeddings.Backend != "gollama" {
		t.Errorf("expected backend 'gollama', got %q", cfg.Embeddings.Backend)
	}
	if cfg.Embeddings.Dimension != 768 {
		t.Errorf("expected dimension 768, got %d", cfg.Embeddings.Dimension)
	}
	if cfg.Providers == nil {
		t.Error("expected providers map to be initialized")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(cfg.Providers))
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: memPath,
	}

	cfg := DefaultConfig()
	cfg.DefaultProvider = "test-provider"
	cfg.Providers["myp"] = ProviderConfig{
		APIKey: "sk-test",
		Model:  "gpt-4",
	}

	if err := SaveConfig(scope, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(scope)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.DefaultProvider != "test-provider" {
		t.Errorf("default provider = %q, want %q", loaded.DefaultProvider, "test-provider")
	}
	if p, ok := loaded.Providers["myp"]; !ok {
		t.Error("expected provider 'myp' to exist")
	} else {
		if p.APIKey != "sk-test" {
			t.Errorf("api key = %q, want %q", p.APIKey, "sk-test")
		}
		if p.Model != "gpt-4" {
			t.Errorf("model = %q, want %q", p.Model, "gpt-4")
		}
	}
}

func TestLoadConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: memPath,
	}

	cfg, err := LoadConfig(scope)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Should return default config when file doesn't exist
	if cfg.Embeddings.Backend != "gollama" {
		t.Errorf("expected default backend, got %q", cfg.Embeddings.Backend)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: memPath,
	}

	configPath := scope.ConfigPath()
	if err := os.WriteFile(configPath, []byte("{{invalid yaml:::"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := LoadConfig(scope)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConfigHooksRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: memPath,
	}

	cfg := DefaultConfig()
	cfg.Hooks = HooksConfig{
		PostCommit: PostCommitHookConfig{
			Enabled:   true,
			Scope:     "project",
			Strategy:  "all",
			Script:    "./my-hook.sh",
			KeyPrefix: "hooks/commits",
			Quiet:     false,
		},
	}

	if err := SaveConfig(scope, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(scope)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	hook := loaded.Hooks.PostCommit
	if hook.Enabled != true {
		t.Errorf("Enabled = %v, want true", hook.Enabled)
	}
	if hook.Scope != "project" {
		t.Errorf("Scope = %q, want %q", hook.Scope, "project")
	}
	if hook.Strategy != "all" {
		t.Errorf("Strategy = %q, want %q", hook.Strategy, "all")
	}
	if hook.Script != "./my-hook.sh" {
		t.Errorf("Script = %q, want %q", hook.Script, "./my-hook.sh")
	}
	if hook.KeyPrefix != "hooks/commits" {
		t.Errorf("KeyPrefix = %q, want %q", hook.KeyPrefix, "hooks/commits")
	}
	if hook.Quiet != false {
		t.Errorf("Quiet = %v, want false", hook.Quiet)
	}
}

func TestConfigDefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, ".mem")
	if err := os.MkdirAll(memPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: memPath,
	}

	// Write minimal config with no providers key
	configPath := scope.ConfigPath()
	if err := os.WriteFile(configPath, []byte("embeddings:\n  backend: custom\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(scope)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Embeddings.Backend != "custom" {
		t.Errorf("backend = %q, want %q", cfg.Embeddings.Backend, "custom")
	}

	// Providers should be initialized to empty map even if not in YAML
	if cfg.Providers == nil {
		t.Error("expected providers to be initialized")
	}
}
