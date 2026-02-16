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
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }
	histFor := func(s internal.Scope) (internal.HistoryRepository, error) { return repo, nil }
	nilIndex := func(s internal.Scope) (internal.VectorIndex, error) { return nil, internal.ErrNoIndex }

	setUC := internal.NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	commitUC := internal.NewCommitUseCase(resolver, histFor)

	cmd := NewSetCmd(setUC, commitUC)
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
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }
	histFor := func(s internal.Scope) (internal.HistoryRepository, error) { return repo, nil }
	nilIndex := func(s internal.Scope) (internal.VectorIndex, error) { return nil, internal.ErrNoIndex }

	setUC := internal.NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	commitUC := internal.NewCommitUseCase(resolver, histFor)

	// Set initial value
	cmd := NewSetCmd(setUC, commitUC)
	cmd.SetArgs([]string{"mykey", "first"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first set: %v", err)
	}

	// Overwrite
	cmd2 := NewSetCmd(setUC, commitUC)
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
