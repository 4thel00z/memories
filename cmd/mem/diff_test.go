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

func setupDiffTest(t *testing.T) (*internal.GitRepository, *internal.HistoryService) {
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

func TestDiffCmdNoChanges(t *testing.T) {
	_, hist := setupDiffTest(t)

	cmd := NewDiffCmd(func() *internal.HistoryService { return hist })

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "No changes") {
		t.Errorf("expected 'No changes' output, got %q", out.String())
	}
}

func TestDiffCmdWithChanges(t *testing.T) {
	repo, hist := setupDiffTest(t)

	// Stage a file but don't commit — diff should show it
	key, _ := internal.NewKey("diffme")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("diff content"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save: %v", err)
	}

	cmd := NewDiffCmd(func() *internal.HistoryService { return hist })

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if strings.Contains(output, "No changes") {
		t.Error("expected changes in diff output")
	}
	if !strings.Contains(output, "diff content") {
		t.Errorf("expected 'diff content' in output, got %q", output)
	}
}
