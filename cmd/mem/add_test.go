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

func setupAddTest(t *testing.T) (*internal.GitRepository, *internal.MemoryService, *internal.HistoryService) {
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

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	return repo, svc, hist
}

func TestAddCmdCreatesNew(t *testing.T) {
	repo, svc, hist := setupAddTest(t)

	cmd := NewAddCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"new/key", "hello world"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	mem, err := repo.Get(context.Background(), internal.Key("new/key"))
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}

	if string(mem.Content) != "hello world" {
		t.Errorf("content = %q, want %q", string(mem.Content), "hello world")
	}

	if out.String() != "Created new/key\n" {
		t.Errorf("output = %q, want %q", out.String(), "Created new/key\n")
	}
}

func TestAddCmdAppendsExisting(t *testing.T) {
	repo, svc, hist := setupAddTest(t)

	// Create initial memory
	key, _ := internal.NewKey("existing")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("first"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := repo.Commit(context.Background(), "test: setup"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Append via add command
	cmd := NewAddCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"existing", "second"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	want := "first\nsecond"
	if string(got.Content) != want {
		t.Errorf("content = %q, want %q", string(got.Content), want)
	}

	if out.String() != "Appended to existing\n" {
		t.Errorf("output = %q, want %q", out.String(), "Appended to existing\n")
	}
}

func TestAddCmdCreatesHistoryNode(t *testing.T) {
	repo, svc, hist := setupAddTest(t)

	// Add first
	cmd := NewAddCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"tracked", "version 1"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Add second
	cmd2 := NewAddCmd(func() *internal.MemoryService { return svc }, func() *internal.HistoryService { return hist })
	cmd2.SetArgs([]string{"tracked", "version 2"})
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second add: %v", err)
	}

	// Verify commit history has entries for both adds
	commits, err := repo.Log(context.Background(), 10)
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	// init commit + 2 add commits = at least 3
	if len(commits) < 3 {
		t.Errorf("expected at least 3 commits, got %d", len(commits))
	}
}
