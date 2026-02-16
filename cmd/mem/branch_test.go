package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func setupBranchTest(t *testing.T) *internal.BranchService {
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
	return internal.NewBranchService(resolver, func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil })
}

func TestBranchCmdList(t *testing.T) {
	svc := setupBranchTest(t)

	cmd := NewBranchCmd(func() *internal.BranchService { return svc })

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "* ") {
		t.Error("expected current branch marker '*' in output")
	}
}

func TestBranchCmdCreateAndSwitch(t *testing.T) {
	svc := setupBranchTest(t)

	cmd := NewBranchCmd(func() *internal.BranchService { return svc })
	cmd.SetArgs([]string{"feature"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "Switched to new branch feature") {
		t.Errorf("unexpected output: %q", out.String())
	}

	// Verify we're on the new branch
	current, err := svc.Current(cmd.Context(), "")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if current.Name != "feature" {
		t.Errorf("current branch = %q, want %q", current.Name, "feature")
	}
}

func TestBranchCmdDelete(t *testing.T) {
	svc := setupBranchTest(t)

	// Create a branch first
	createCmd := NewBranchCmd(func() *internal.BranchService { return svc })
	createCmd.SetArgs([]string{"to-delete"})
	var buf bytes.Buffer
	createCmd.SetOut(&buf)
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Switch back to main so we can delete
	switchCmd := NewBranchCmd(func() *internal.BranchService { return svc })
	switchCmd.SetArgs([]string{"main"})
	switchCmd.SetOut(&buf)
	if err := switchCmd.Execute(); err != nil {
		t.Fatalf("switch back: %v", err)
	}

	// Delete
	delCmd := NewBranchCmd(func() *internal.BranchService { return svc })
	delCmd.SetArgs([]string{"-d", "to-delete"})
	var out bytes.Buffer
	delCmd.SetOut(&out)

	if err := delCmd.Execute(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if !strings.Contains(out.String(), "Deleted branch to-delete") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestBranchCmdDeleteCurrentFails(t *testing.T) {
	svc := setupBranchTest(t)

	// Try to delete current branch
	current, err := svc.Current(context.Background(), "")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}

	cmd := NewBranchCmd(func() *internal.BranchService { return svc })
	cmd.SetArgs([]string{"-d", current.Name})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when deleting current branch")
	}
}
