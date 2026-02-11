package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func TestSetCmd(t *testing.T) {
	tmpDir := t.TempDir()
	scope := internal.Scope{
		Type:    internal.ScopeProject,
		Path:    tmpDir,
		MemPath: filepath.Join(tmpDir, ".mem"),
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	if err := internal.InitRepository(scope); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	repo, err := internal.NewGitRepository(scope)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, nil },
		nil,
	)
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	cmd := NewSetCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"test/key", "test value"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify the memory was created
	mem, err := repo.Get(cmd.Context(), internal.Key("test/key"))
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}

	if string(mem.Content) != "test value" {
		t.Errorf("content = %q, want %q", string(mem.Content), "test value")
	}
}

func TestSetCmdOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	scope := internal.Scope{
		Type:    internal.ScopeProject,
		Path:    tmpDir,
		MemPath: filepath.Join(tmpDir, ".mem"),
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	if err := internal.InitRepository(scope); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	repo, err := internal.NewGitRepository(scope)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, nil },
		nil,
	)
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	// Set initial value
	cmd := NewSetCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"mykey", "first"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first set: %v", err)
	}

	// Overwrite
	cmd2 := NewSetCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd2.SetArgs([]string{"mykey", "second"})
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second set: %v", err)
	}

	mem, err := repo.Get(cmd.Context(), internal.Key("mykey"))
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}

	if string(mem.Content) != "second" {
		t.Errorf("content = %q, want %q", string(mem.Content), "second")
	}
}
