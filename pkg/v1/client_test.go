package v1

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/4thel00z/memories/internal"
)

func setupClientTest(t *testing.T) *Client {
	t.Helper()
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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

	client, err := New()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	return client
}

func TestClientSetAndGet(t *testing.T) {
	client := setupClientTest(t)

	ctx := context.Background()

	if err := client.Set(ctx, "test/key", []byte("hello world")); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := client.Get(ctx, "test/key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(got) != "hello world" {
		t.Errorf("content = %q, want %q", string(got), "hello world")
	}
}

func TestClientDelete(t *testing.T) {
	client := setupClientTest(t)

	ctx := context.Background()

	if err := client.Set(ctx, "to-delete", []byte("bye")); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := client.Delete(ctx, "to-delete"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := client.Get(ctx, "to-delete")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestClientList(t *testing.T) {
	client := setupClientTest(t)

	ctx := context.Background()

	for _, key := range []string{"foo/a", "foo/b", "bar/c"} {
		if err := client.Set(ctx, key, []byte("content")); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	all, err := client.List(ctx, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 memories, got %d", len(all))
	}

	foos, err := client.List(ctx, "foo")
	if err != nil {
		t.Fatalf("list foo: %v", err)
	}
	if len(foos) != 2 {
		t.Errorf("expected 2 foo memories, got %d", len(foos))
	}
}

func TestClientGetNotFound(t *testing.T) {
	client := setupClientTest(t)

	_, err := client.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestClientInvalidKey(t *testing.T) {
	client := setupClientTest(t)

	err := client.Set(context.Background(), "", []byte("x"))
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestInit(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Init is idempotent: a second call on an existing store is a no-op.
	if err := Init(); err != nil {
		t.Fatalf("second init: %v", err)
	}

	client, err := New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	if err := client.Set(ctx, "boot/key", []byte("v")); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := client.Get(ctx, "boot/key")
	if err != nil || string(got) != "v" {
		t.Fatalf("get = %q, %v", got, err)
	}
}
