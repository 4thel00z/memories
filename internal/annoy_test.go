package internal

import (
	"context"
	"testing"
)

func TestAnnoyIndexAddAndSearch(t *testing.T) {
	tmpDir := t.TempDir()
	dim := 3

	idx, err := NewAnnoyIndex(tmpDir, dim)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	ctx := context.Background()

	key1, _ := NewKey("doc/one")
	key2, _ := NewKey("doc/two")

	if err := idx.Add(ctx, key1, Embedding{Vector: []float32{1.0, 0.0, 0.0}}); err != nil {
		t.Fatalf("add key1: %v", err)
	}
	if err := idx.Add(ctx, key2, Embedding{Vector: []float32{0.0, 1.0, 0.0}}); err != nil {
		t.Fatalf("add key2: %v", err)
	}

	if err := idx.Build(ctx, 2); err != nil {
		t.Fatalf("build: %v", err)
	}

	results, err := idx.Search(ctx, Embedding{Vector: []float32{1.0, 0.1, 0.0}}, 2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	if results[0].Key.String() != "doc/one" {
		t.Errorf("expected closest match to be 'doc/one', got %q", results[0].Key.String())
	}
}

func TestAnnoyIndexRemove(t *testing.T) {
	tmpDir := t.TempDir()
	dim := 3

	idx, err := NewAnnoyIndex(tmpDir, dim)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	ctx := context.Background()
	key, _ := NewKey("removeme")

	if err := idx.Add(ctx, key, Embedding{Vector: []float32{1.0, 0.0, 0.0}}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if !idx.Contains(ctx, key) {
		t.Error("expected key to exist after add")
	}

	if err := idx.Remove(ctx, key); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if idx.Contains(ctx, key) {
		t.Error("expected key to be gone after remove")
	}
}

func TestAnnoyIndexDimensionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	dim := 3

	idx, err := NewAnnoyIndex(tmpDir, dim)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	ctx := context.Background()
	key, _ := NewKey("bad")

	err = idx.Add(ctx, key, Embedding{Vector: []float32{1.0, 0.0}})
	if err == nil {
		t.Error("expected dimension mismatch error on add")
	}

	// Build so we can test search dimension mismatch
	if err := idx.Build(ctx, 1); err != nil {
		t.Fatalf("build: %v", err)
	}

	_, err = idx.Search(ctx, Embedding{Vector: []float32{1.0, 0.0}}, 1)
	if err == nil {
		t.Error("expected dimension mismatch error on search")
	}
}

func TestAnnoyIndexSearchBeforeBuild(t *testing.T) {
	tmpDir := t.TempDir()

	idx, err := NewAnnoyIndex(tmpDir, 3)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	_, err = idx.Search(context.Background(), Embedding{Vector: []float32{1.0, 0.0, 0.0}}, 1)
	if err == nil {
		t.Error("expected error when searching before build")
	}
}

func TestAnnoyIndexSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	dim := 3
	ctx := context.Background()

	// Create and populate index
	idx1, err := NewAnnoyIndex(tmpDir, dim)
	if err != nil {
		t.Fatalf("new index 1: %v", err)
	}

	key, _ := NewKey("persist/me")
	if err := idx1.Add(ctx, key, Embedding{Vector: []float32{0.5, 0.5, 0.0}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := idx1.Build(ctx, 2); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := idx1.Save(ctx); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Load into a new index
	idx2, err := NewAnnoyIndex(tmpDir, dim)
	if err != nil {
		t.Fatalf("new index 2: %v", err)
	}
	if err := idx2.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}

	if !idx2.Contains(ctx, key) {
		t.Error("expected key to be present after load")
	}

	results, err := idx2.Search(ctx, Embedding{Vector: []float32{0.5, 0.5, 0.0}}, 1)
	if err != nil {
		t.Fatalf("search after load: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Key.String() != "persist/me" {
		t.Errorf("expected 'persist/me', got %q", results[0].Key.String())
	}
}
