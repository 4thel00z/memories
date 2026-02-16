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

func setupSearchTest(t *testing.T) *internal.SearchService {
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
	return internal.NewSearchService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)
}

func TestSearchCmdKeyword(t *testing.T) {
	svc := setupSearchTest(t)

	cmd := NewSearchCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"milk"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "project/todo") {
		t.Errorf("expected 'project/todo' in results, got %q", output)
	}
}

func TestSearchCmdKeywordNoMatch(t *testing.T) {
	svc := setupSearchTest(t)

	cmd := NewSearchCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"zzzznonexistent"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("expected empty output for no match, got %q", out.String())
	}
}

func TestSearchCmdKeywordMatchesKey(t *testing.T) {
	svc := setupSearchTest(t)

	cmd := NewSearchCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"meeting"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "notes/meeting") {
		t.Errorf("expected 'notes/meeting' in results, got %q", output)
	}
}

func TestSearchCmdSemanticNoEmbedder(t *testing.T) {
	svc := setupSearchTest(t)

	cmd := NewSearchCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"-s", "installation"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for semantic search without embedder")
	}
}
