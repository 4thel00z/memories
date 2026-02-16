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

func setupLogTest(t *testing.T) (*internal.GitRepository, *internal.LogUseCase) {
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

	// Create some commits
	for i, name := range []string{"first", "second", "third"} {
		key, _ := internal.NewKey(name)
		mem := &internal.Memory{
			Key:       key,
			Content:   []byte("content " + name),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := repo.Save(context.Background(), mem); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		if _, err := repo.Commit(context.Background(), "add: "+name); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}

	resolver := internal.NewScopeResolver()
	histFor := func(s internal.Scope) (internal.HistoryRepository, error) { return repo, nil }

	logUC := internal.NewLogUseCase(resolver, histFor)

	return repo, logUC
}

func TestLogCmd(t *testing.T) {
	_, logUC := setupLogTest(t)

	cmd := NewLogCmd(logUC)

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "add: first") {
		t.Errorf("missing 'add: first' in output: %s", output)
	}
	if !strings.Contains(output, "add: third") {
		t.Errorf("missing 'add: third' in output: %s", output)
	}
}

func TestLogCmdOneline(t *testing.T) {
	_, logUC := setupLogTest(t)

	cmd := NewLogCmd(logUC)
	cmd.SetArgs([]string{"--oneline"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// init commit + 3 add commits = 4
	if len(lines) < 4 {
		t.Errorf("expected at least 4 oneline entries, got %d: %v", len(lines), lines)
	}

	// Each line should be short: hash + message
	for _, line := range lines {
		if !strings.Contains(line, " ") {
			t.Errorf("oneline entry missing space: %q", line)
		}
	}
}

func TestLogCmdLimit(t *testing.T) {
	_, logUC := setupLogTest(t)

	cmd := NewLogCmd(logUC)
	cmd.SetArgs([]string{"-n", "2", "--oneline"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 entries with -n 2, got %d: %v", len(lines), lines)
	}
}
