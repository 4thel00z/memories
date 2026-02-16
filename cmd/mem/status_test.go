package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func TestStatusCmd(t *testing.T) {
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
	branchSvc := internal.NewBranchService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })

	cmd := NewStatusCmd(func() *internal.BranchService { return branchSvc })

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "On branch") {
		t.Errorf("expected 'On branch' in output, got %q", out.String())
	}
}
