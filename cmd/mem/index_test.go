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

func setupIndexTest(t *testing.T) *internal.RebuildIndexUseCase {
	t.Helper()
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

	// Seed memories
	for _, entry := range []struct {
		key     string
		content string
	}{
		{"project/readme", "This is the project readme with installation instructions"},
		{"project/todo", "Buy milk and eggs from the store"},
		{"notes/meeting", "Discussed quarterly targets and budget allocation"},
	} {
		key, _ := internal.NewKey(entry.key)
		mem := &internal.Memory{
			Key:       key,
			Content:   []byte(entry.content),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Save(context.Background(), mem); err != nil {
			t.Fatalf("save %s: %v", entry.key, err)
		}
	}

	resolver := internal.NewScopeResolver()
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }
	nilIndex := func(s internal.Scope) (internal.VectorIndex, error) { return nil, internal.ErrNoIndex }

	return internal.NewRebuildIndexUseCase(resolver, repoFor, nilIndex, nil)
}

func TestIndexStatusCmd(t *testing.T) {
	rebuildUC := setupIndexTest(t)

	cmd := NewIndexCmd(rebuildUC)
	cmd.SetArgs([]string{"status"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "Index status") {
		t.Errorf("expected status message, got %q", out.String())
	}
}

func TestIndexRebuildNoEmbedder(t *testing.T) {
	rebuildUC := setupIndexTest(t)

	cmd := NewIndexCmd(rebuildUC)
	cmd.SetArgs([]string{"rebuild"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for rebuild without embedder")
	}
}
