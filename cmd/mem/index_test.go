package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func TestIndexStatusCmd(t *testing.T) {
	// Status doesn't need a real service
	svc := setupSearchTest(t)

	cmd := NewIndexCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"status"})

	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !strings.Contains(out.String(), "Index status") {
		t.Errorf("expected status message, got %q", out.String())
	}
}

func TestIndexRebuildNoEmbedder(t *testing.T) {
	svc := setupSearchTest(t)

	cmd := NewIndexCmd(func() *internal.SearchService { return svc })
	cmd.SetArgs([]string{"rebuild"})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for rebuild without embedder")
	}
}
