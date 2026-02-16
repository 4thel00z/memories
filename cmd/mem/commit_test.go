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

func setupCommitTest(t *testing.T) (*internal.GitRepository, *internal.HistoryService) {
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
	hist := internal.NewHistoryService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	return repo, hist
}

func TestCommitCmd(t *testing.T) {
	repo, hist := setupCommitTest(t)

	// Stage a change first
	key, _ := internal.NewKey("commit-me")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("staged content"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save: %v", err)
	}

	cmd := NewCommitCmd(func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"-m", "test: commit test"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if len(output) == 0 {
		t.Fatal("expected output with commit hash")
	}

	// Verify commit appears in log
	commits, err := repo.Log(context.Background(), 10)
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	found := false
	for _, c := range commits {
		if c.Message == "test: commit test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("commit message not found in log")
	}
}

func TestCommitCmdNoMessage(t *testing.T) {
	_, hist := setupCommitTest(t)

	// Set EDITOR to false so it exits immediately with error
	t.Setenv("EDITOR", "false")

	cmd := NewCommitCmd(func() *internal.HistoryService { return hist })

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when editor fails")
	}
}

func TestCommitCmdEmptyWorktree(t *testing.T) {
	_, hist := setupCommitTest(t)

	cmd := NewCommitCmd(func() *internal.HistoryService { return hist })
	cmd.SetArgs([]string{"-m", "empty commit"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for empty worktree commit")
	}
}
