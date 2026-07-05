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

func setupIndexTest(t *testing.T) (*internal.RebuildIndexUseCase, *internal.IndexStatusUseCase) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
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

	return internal.NewRebuildIndexUseCase(resolver, repoFor, nilIndex, nil),
		internal.NewIndexStatusUseCase(resolver)
}

func TestIndexStatusCmdNoIndex(t *testing.T) {
	rebuildUC, statusUC := setupIndexTest(t)

	cmd := NewIndexCmd(rebuildUC, statusUC)
	cmd.SetArgs([]string{"status"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "No index built") {
		t.Errorf("expected no-index message, got %q", out.String())
	}
}

func TestIndexStatusCmdWithIndex(t *testing.T) {
	rebuildUC, statusUC := setupIndexTest(t)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	vectors := filepath.Join(cwd, ".mem", "vectors")
	if err := os.WriteFile(filepath.Join(vectors, "index.ann"), []byte("stub"), 0600); err != nil {
		t.Fatal(err)
	}
	mapping := `{"key_to_id":{"a":0,"b":1},"id_to_key":{"0":"a","1":"b"},"next_id":2}`
	if err := os.WriteFile(filepath.Join(vectors, "mapping.json"), []byte(mapping), 0600); err != nil {
		t.Fatal(err)
	}

	cmd := NewIndexCmd(rebuildUC, statusUC)
	cmd.SetArgs([]string{"status"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "Vectors:   2") {
		t.Errorf("expected 2 vectors in status, got %q", out.String())
	}
}

func TestIndexRebuildNoEmbedder(t *testing.T) {
	rebuildUC, statusUC := setupIndexTest(t)

	cmd := NewIndexCmd(rebuildUC, statusUC)
	cmd.SetArgs([]string{"rebuild"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for rebuild without embedder")
	}
}
