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

func setupEditTest(t *testing.T) (*internal.GitRepository, *internal.GetMemoryUseCase, *internal.SetMemoryUseCase, *internal.CommitUseCase) {
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
	repoFor := func(s internal.Scope) (internal.MemoryRepository, error) { return repo, nil }
	histFor := func(s internal.Scope) (internal.HistoryRepository, error) { return repo, nil }
	nilIndex := func(s internal.Scope) (internal.VectorIndex, error) { return nil, internal.ErrNoIndex }

	getUC := internal.NewGetMemoryUseCase(resolver, repoFor)
	setUC := internal.NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	commitUC := internal.NewCommitUseCase(resolver, histFor)

	return repo, getUC, setUC, commitUC
}

func TestEditCmdCreatesNew(t *testing.T) {
	repo, getUC, setUC, commitUC := setupEditTest(t)

	// Use a script that writes content to the file as the "editor"
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'edited content' > \"$1\"\n"), 0755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editorScript)

	cmd := NewEditCmd(getUC, setUC, commitUC)
	cmd.SetArgs([]string{"new/edited"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if out.String() != "Created new/edited\n" {
		t.Errorf("output = %q, want %q", out.String(), "Created new/edited\n")
	}

	mem, err := repo.Get(context.Background(), internal.Key("new/edited"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(mem.Content) != "edited content\n" {
		t.Errorf("content = %q, want %q", string(mem.Content), "edited content\n")
	}
}

func TestEditCmdUpdatesExisting(t *testing.T) {
	repo, getUC, setUC, commitUC := setupEditTest(t)

	// Create initial memory
	key, _ := internal.NewKey("existing/edit")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("old content"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := repo.Commit(context.Background(), "setup"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Editor that replaces content
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\necho 'new content' > \"$1\"\n"), 0755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editorScript)

	cmd := NewEditCmd(getUC, setUC, commitUC)
	cmd.SetArgs([]string{"existing/edit"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if out.String() != "Updated existing/edit\n" {
		t.Errorf("output = %q, want %q", out.String(), "Updated existing/edit\n")
	}

	got, err := repo.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(got.Content) != "new content\n" {
		t.Errorf("content = %q, want %q", string(got.Content), "new content\n")
	}
}

func TestEditCmdNoChanges(t *testing.T) {
	repo, getUC, setUC, commitUC := setupEditTest(t)

	// Create initial memory
	key, _ := internal.NewKey("nochange")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("same content"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := repo.Commit(context.Background(), "setup"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Editor that doesn't change the file
	editorScript := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\n# do nothing\n"), 0755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editorScript)

	cmd := NewEditCmd(getUC, setUC, commitUC)
	cmd.SetArgs([]string{"nochange"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if out.String() != "No changes.\n" {
		t.Errorf("output = %q, want %q", out.String(), "No changes.\n")
	}
}
