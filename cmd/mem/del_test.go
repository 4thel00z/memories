package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/4thel00z/memories/internal"
)

func TestDelCmd(t *testing.T) {
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

	// Create a memory to delete
	key, _ := internal.NewKey("to-delete")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("will be deleted"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save memory: %v", err)
	}
	if _, err := repo.Commit(context.Background(), "test: add memory to delete"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	cmd := NewDelCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"to-delete"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Verify deleted
	exists, err := repo.Exists(context.Background(), key)
	if err != nil {
		t.Fatalf("exists check: %v", err)
	}
	if exists {
		t.Error("memory still exists after delete")
	}
}

func TestDelCmdNotFound(t *testing.T) {
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
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	cmd := NewDelCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"nonexistent"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}
