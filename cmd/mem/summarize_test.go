package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func TestSummarizeCmdNoProvider(t *testing.T) {
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
	svc := internal.NewSummarizeService(
		resolver,
		func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil },
		nil, // no provider
	)

	cmd := NewSummarizeCmd(func() *internal.SummarizeService { return svc })

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for summarize without provider")
	}
}
