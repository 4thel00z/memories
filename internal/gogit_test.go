package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T) (*GitRepository, Scope) {
	t.Helper()
	tmpDir := t.TempDir()
	scope := Scope{
		Type:    ScopeProject,
		Path:    tmpDir,
		MemPath: filepath.Join(tmpDir, ".mem"),
	}

	if err := os.MkdirAll(scope.VectorPath(), 0755); err != nil {
		t.Fatalf("mkdir vectors: %v", err)
	}
	if err := InitRepository(scope); err != nil {
		t.Fatalf("init repo: %v", err)
	}

	repo, err := NewGitRepository(scope)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	return repo, scope
}

func TestGitRepositorySaveAndGet(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("test/key")
	mem := NewMemory(key, []byte("hello"))

	if err := repo.Save(ctx, mem); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(got.Content) != "hello" {
		t.Errorf("content = %q, want %q", string(got.Content), "hello")
	}
}

func TestGitRepositoryDelete(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("to-delete")
	mem := NewMemory(key, []byte("bye"))

	if err := repo.Save(ctx, mem); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := repo.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}

	exists, err := repo.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists {
		t.Error("key still exists after delete")
	}
}

func TestGitRepositoryDeleteNotFound(t *testing.T) {
	repo, _ := setupGitRepo(t)

	key, _ := NewKey("nonexistent")
	err := repo.Delete(context.Background(), key)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGitRepositoryList(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	for _, name := range []string{"a/one", "a/two", "b/three"} {
		key, _ := NewKey(name)
		if err := repo.Save(ctx, NewMemory(key, []byte("content"))); err != nil {
			t.Fatalf("save %s: %v", name, err)
		}
	}

	all, err := repo.List(ctx, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	filtered, err := repo.List(ctx, "a")
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 with prefix 'a', got %d", len(filtered))
	}
}

func TestGitRepositoryExists(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("check-exists")
	exists, err := repo.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists {
		t.Error("should not exist yet")
	}

	if err := repo.Save(ctx, NewMemory(key, []byte("hi"))); err != nil {
		t.Fatalf("save: %v", err)
	}

	exists, err = repo.Exists(ctx, key)
	if err != nil {
		t.Fatalf("exists after save: %v", err)
	}
	if !exists {
		t.Error("should exist after save")
	}
}

func TestGitRepositoryCommitAndLog(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("logged")
	if err := repo.Save(ctx, NewMemory(key, []byte("data"))); err != nil {
		t.Fatalf("save: %v", err)
	}

	commit, err := repo.Commit(ctx, "test commit")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.Hash == "" {
		t.Error("commit hash is empty")
	}
	if commit.Message != "test commit" {
		t.Errorf("message = %q, want %q", commit.Message, "test commit")
	}

	commits, err := repo.Log(ctx, 10)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(commits) < 2 { // init + test commit
		t.Errorf("expected at least 2 commits, got %d", len(commits))
	}
}

func TestGitRepositoryBranch(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	current, err := repo.Current(ctx)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.Name == "" {
		t.Error("current branch name is empty")
	}

	_, err = repo.Create(ctx, "feature")
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}

	branches, err := repo.ListBranches(ctx)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(branches) < 2 {
		t.Errorf("expected at least 2 branches, got %d", len(branches))
	}

	if err := repo.Switch(ctx, "feature"); err != nil {
		t.Fatalf("switch: %v", err)
	}

	after, err := repo.Current(ctx)
	if err != nil {
		t.Fatalf("current after switch: %v", err)
	}
	if after.Name != "feature" {
		t.Errorf("current branch = %q, want %q", after.Name, "feature")
	}
}

func TestGitRepositoryDiffWorktree(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	// Clean tree = no diff
	diff, err := repo.Diff(ctx, "")
	if err != nil {
		t.Fatalf("diff empty: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff, got %q", diff)
	}

	// Stage a file = diff shows it
	key, _ := NewKey("diffed")
	if err := repo.Save(ctx, NewMemory(key, []byte("new stuff"))); err != nil {
		t.Fatalf("save: %v", err)
	}

	diff, err = repo.Diff(ctx, "")
	if err != nil {
		t.Fatalf("diff with changes: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff after staging")
	}
}
