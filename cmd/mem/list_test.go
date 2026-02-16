package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/4thel00z/memories/internal"
)

func TestListCmd(t *testing.T) {
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

	// Create several memories
	for _, name := range []string{"foo/a", "foo/b", "bar/c"} {
		key, _ := internal.NewKey(name)
		mem := &internal.Memory{
			Key:       key,
			Content:   []byte("content of " + name),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Save(context.Background(), mem); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	resolver := internal.NewScopeResolver()
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }

	listUC := internal.NewListMemoriesUseCase(resolver, repoFor)

	// List all
	cmd := NewListCmd(listUC)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 memories, got %d: %v", len(lines), lines)
	}
}

func TestListCmdWithPrefix(t *testing.T) {
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

	for _, name := range []string{"foo/a", "foo/b", "bar/c"} {
		key, _ := internal.NewKey(name)
		mem := &internal.Memory{
			Key:       key,
			Content:   []byte("content of " + name),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Save(context.Background(), mem); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	resolver := internal.NewScopeResolver()
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }

	listUC := internal.NewListMemoriesUseCase(resolver, repoFor)

	cmd := NewListCmd(listUC)
	cmd.SetArgs([]string{"foo"})
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 memories with prefix foo, got %d: %v", len(lines), lines)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "foo/") {
			t.Errorf("unexpected key %q, expected foo/ prefix", line)
		}
	}
}

func TestListCmdEmpty(t *testing.T) {
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

	listUC := internal.NewListMemoriesUseCase(resolver, repoFor)

	cmd := NewListCmd(listUC)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("expected empty output, got %q", out.String())
	}
}
