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

func setupBranchTest(t *testing.T) (
	*internal.BranchCurrentUseCase,
	*internal.BranchListUseCase,
	*internal.BranchCreateUseCase,
	*internal.BranchSwitchUseCase,
	*internal.BranchDeleteUseCase,
) {
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
	branchFor := func(s internal.Scope) (internal.BranchRepository, error) { return repo, nil }

	return internal.NewBranchCurrentUseCase(resolver, branchFor),
		internal.NewBranchListUseCase(resolver, branchFor),
		internal.NewBranchCreateUseCase(resolver, branchFor),
		internal.NewBranchSwitchUseCase(resolver, branchFor),
		internal.NewBranchDeleteUseCase(resolver, branchFor)
}

func TestBranchCmdList(t *testing.T) {
	currentUC, listUC, createUC, switchUC, deleteUC := setupBranchTest(t)

	cmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)

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
	currentUC, listUC, createUC, switchUC, deleteUC := setupBranchTest(t)

	cmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)
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
	current, err := currentUC.Execute(context.Background(), internal.BranchInput{})
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if current.Name != "feature" {
		t.Errorf("current branch = %q, want %q", current.Name, "feature")
	}
}

func TestBranchCmdDelete(t *testing.T) {
	currentUC, listUC, createUC, switchUC, deleteUC := setupBranchTest(t)

	// Create a branch first
	createCmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)
	createCmd.SetArgs([]string{"to-delete"})
	var buf bytes.Buffer
	createCmd.SetOut(&buf)
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Switch back to main so we can delete
	switchCmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)
	switchCmd.SetArgs([]string{"main"})
	switchCmd.SetOut(&buf)
	if err := switchCmd.Execute(); err != nil {
		t.Fatalf("switch back: %v", err)
	}

	// Delete
	delCmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)
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
	currentUC, listUC, createUC, switchUC, deleteUC := setupBranchTest(t)

	// Try to delete current branch
	current, err := currentUC.Execute(context.Background(), internal.BranchInput{})
	if err != nil {
		t.Fatalf("get current: %v", err)
	}

	cmd := NewBranchCmd(currentUC, listUC, createUC, switchUC, deleteUC)
	cmd.SetArgs([]string{"-d", current.Name})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when deleting current branch")
	}
}
