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

func TestGetCmd(t *testing.T) {
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

	// Create a memory directly
	key, _ := internal.NewKey("test/key")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("test content"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save memory: %v", err)
	}

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)

	cmd := NewGetCmd(func() *internal.MemoryService { return svc })
	cmd.SetArgs([]string{"test/key"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if out.String() != "test content" {
		t.Errorf("output = %q, want %q", out.String(), "test content")
	}
}

func TestGetCmdNotFound(t *testing.T) {
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

	cmd := NewGetCmd(func() *internal.MemoryService { return svc })
	cmd.SetArgs([]string{"nonexistent"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestGetCmdJSON(t *testing.T) {
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

	key, _ := internal.NewKey("jsonkey")
	mem := &internal.Memory{
		Key:       key,
		Content:   []byte("json value"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), mem); err != nil {
		t.Fatalf("save memory: %v", err)
	}

	resolver := internal.NewScopeResolver()
	svc := internal.NewMemoryService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex },
		nil,
	)

	cmd := NewGetCmd(func() *internal.MemoryService { return svc })
	cmd.Root().PersistentFlags().Bool("json", false, "")
	cmd.SetArgs([]string{"jsonkey", "--json"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !bytes.Contains(out.Bytes(), []byte(`"key": "jsonkey"`)) {
		t.Errorf("output missing key field: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"content": "json value"`)) {
		t.Errorf("output missing content field: %s", out.String())
	}
}
