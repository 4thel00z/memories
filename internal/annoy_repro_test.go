package internal

import (
	"context"
	"testing"
)

func TestAnnoyIndexRemoveThenBuild(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewAnnoyIndex(dir, 4)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	k1, _ := NewKey("a")
	k2, _ := NewKey("b")
	_ = idx.Add(ctx, k1, NewEmbedding([]float32{1, 0, 0, 0}, "t"))
	_ = idx.Add(ctx, k2, NewEmbedding([]float32{0, 1, 0, 0}, "t"))
	if err := idx.Build(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if err := idx.Save(ctx); err != nil {
		t.Fatal(err)
	}
	// The mem del flow: fresh handle, Load, Remove, Build. Remove must not
	// clear the built flag, or Build panics on the loaded goannoy index.
	idx2, err := NewAnnoyIndex(dir, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx2.Load(ctx); err != nil {
		t.Fatal(err)
	}
	if err := idx2.Remove(ctx, k1); err != nil {
		t.Fatal(err)
	}
	if err := idx2.Build(ctx, 2); err != nil {
		t.Fatal(err)
	}
}
