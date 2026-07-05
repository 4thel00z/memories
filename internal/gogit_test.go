package internal

import (
	"context"
	"errors"
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

// TestGitRepositoryRejectsTraversalKey verifies the keyToPath containment guard
// blocks a traversal key even when it bypasses NewKey (Key is a raw string type),
// and that nothing is written outside the .mem directory.
func TestGitRepositoryRejectsTraversalKey(t *testing.T) {
	repo, scope := setupGitRepo(t)
	ctx := context.Background()

	escapeTarget := filepath.Join(filepath.Dir(scope.Path), "pwned")
	mem := &Memory{Key: Key("../../pwned"), Content: []byte("owned")}

	if err := repo.Save(ctx, mem); err == nil {
		t.Fatal("expected Save with traversal key to error")
	}
	if _, err := os.Stat(escapeTarget); !os.IsNotExist(err) {
		t.Fatalf("traversal key escaped the store: wrote %s", escapeTarget)
	}

	if _, err := repo.Get(ctx, Key("../../../etc/hosts")); err == nil {
		t.Fatal("expected Get with traversal key to error")
	}
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

// TestGitRepositoryInitBranchIsMain guards against a go-git pitfall: setting
// init.defaultBranch in the config after Init does not re-point HEAD, so
// stores used to be created on master despite DefaultBranch being "main".
func TestGitRepositoryInitBranchIsMain(t *testing.T) {
	repo, _ := setupGitRepo(t)

	current, err := repo.Current(context.Background())
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}
	if current.Name != DefaultBranch {
		t.Errorf("new store on branch %q, want %q", current.Name, DefaultBranch)
	}
}

// TestGitRepositoryListNestedMetadataNames verifies that only the store's own
// top-level metadata is hidden from List; keys that merely contain "vectors"
// or "config.yaml" deeper in their paths are real memories.
func TestGitRepositoryListNestedMetadataNames(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	for _, k := range []string{"notes/vectors/idea", "apps/web/config.yaml"} {
		key, err := NewKey(k)
		if err != nil {
			t.Fatalf("new key %q: %v", k, err)
		}
		if err := repo.Save(ctx, NewMemory(key, []byte("content"))); err != nil {
			t.Fatalf("save %q: %v", k, err)
		}
	}

	memories, err := repo.List(ctx, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	found := make(map[string]bool)
	for _, m := range memories {
		found[m.Key.String()] = true
	}
	for _, k := range []string{"notes/vectors/idea", "apps/web/config.yaml"} {
		if !found[k] {
			t.Errorf("List hid key %q", k)
		}
	}
}

func TestGitRepositorySaveAndCommit(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("atomic/key")
	commit, err := repo.SaveAndCommit(ctx, NewMemory(key, []byte("v1")), "set: atomic/key")
	if err != nil {
		t.Fatalf("save and commit: %v", err)
	}
	if commit == nil || commit.Hash == "" {
		t.Fatal("expected a real commit for a new key")
	}
	if commit.Message != "set: atomic/key" {
		t.Errorf("message = %q", commit.Message)
	}

	diff, err := repo.Diff(ctx, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff != "" {
		t.Errorf("worktree dirty after SaveAndCommit: %q", diff)
	}
}

// TestGitRepositorySaveAndCommitUnchanged guards the idempotency contract:
// writing the same value twice must be a no-op, not an empty-commit error.
func TestGitRepositorySaveAndCommitUnchanged(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("same/key")
	if _, err := repo.SaveAndCommit(ctx, NewMemory(key, []byte("v")), "set: same/key"); err != nil {
		t.Fatalf("first save: %v", err)
	}

	commit, err := repo.SaveAndCommit(ctx, NewMemory(key, []byte("v")), "set: same/key")
	if err != nil {
		t.Fatalf("unchanged save must not error: %v", err)
	}
	if commit != nil {
		t.Errorf("unchanged save produced commit %s", commit.Hash)
	}
}

func TestGitRepositoryDeleteAndCommit(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	key, _ := NewKey("doomed/key")
	if _, err := repo.SaveAndCommit(ctx, NewMemory(key, []byte("bye")), "set: doomed/key"); err != nil {
		t.Fatalf("save: %v", err)
	}

	commit, err := repo.DeleteAndCommit(ctx, key, "del: doomed/key")
	if err != nil {
		t.Fatalf("delete and commit: %v", err)
	}
	if commit == nil {
		t.Fatal("expected a commit for the deletion")
	}
	if _, err := repo.Get(ctx, key); err != ErrNotFound {
		t.Errorf("get after delete = %v, want ErrNotFound", err)
	}
}

// TestGitRepositoryConcurrentSaveAndCommit exercises the cross-process race:
// two repository handles on the same store write different keys concurrently.
// The per-store lock must serialize them so both keys land committed.
func TestGitRepositoryConcurrentSaveAndCommit(t *testing.T) {
	repo1, scope := setupGitRepo(t)
	repo2, err := NewGitRepository(scope)
	if err != nil {
		t.Fatalf("second repo handle: %v", err)
	}

	ctx := context.Background()
	errs := make(chan error, 2)

	go func() {
		key, _ := NewKey("worker/a")
		_, err := repo1.SaveAndCommit(ctx, NewMemory(key, []byte("from repo1")), "set: worker/a")
		errs <- err
	}()
	go func() {
		key, _ := NewKey("worker/b")
		_, err := repo2.SaveAndCommit(ctx, NewMemory(key, []byte("from repo2")), "set: worker/b")
		errs <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent save %d: %v", i, err)
		}
	}

	for _, k := range []string{"worker/a", "worker/b"} {
		key, _ := NewKey(k)
		if _, err := repo1.Get(ctx, key); err != nil {
			t.Errorf("get %s: %v", k, err)
		}
	}

	diff, err := repo1.Diff(ctx, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff != "" {
		t.Errorf("worktree dirty after concurrent commits: %q", diff)
	}
}

// TestGitRepositoryCreateExistingBranch guards against ref force-moves:
// creating a branch that already exists must fail with ErrBranchExists and
// must not move the existing ref (go-git's SetReference silently overwrites).
func TestGitRepositoryCreateExistingBranch(t *testing.T) {
	repo, _ := setupGitRepo(t)
	ctx := context.Background()

	mainBefore, err := repo.Current(ctx)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	if _, err := repo.Create(ctx, "experiments"); err != nil {
		t.Fatalf("create experiments: %v", err)
	}
	if err := repo.Switch(ctx, "experiments"); err != nil {
		t.Fatalf("switch: %v", err)
	}

	key, _ := NewKey("exp/idea")
	if _, err := repo.SaveAndCommit(ctx, NewMemory(key, []byte("risky")), "set: exp/idea"); err != nil {
		t.Fatalf("save on experiments: %v", err)
	}

	_, err = repo.Create(ctx, "main")
	if !errors.Is(err, ErrBranchExists) {
		t.Fatalf("create existing = %v, want ErrBranchExists", err)
	}

	branches, err := repo.ListBranches(ctx)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	for _, b := range branches {
		if b.Name == "main" && b.Head != mainBefore.Head {
			t.Errorf("main moved from %s to %s", mainBefore.Head, b.Head)
		}
	}
}

// TestGitRepositoryCommitAll covers the hand-edit flow: files changed in the
// store worktree outside of mem (nothing staged) must be staged and committed,
// while store metadata stays out of history and a clean tree is a nil no-op.
func TestGitRepositoryCommitAll(t *testing.T) {
	repo, scope := setupGitRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(scope.MemPath, "hand-edited"), []byte("external tool wrote this"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(scope.ConfigPath(), []byte("embeddings:\n  dimension: 768\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	commit, err := repo.CommitAll(ctx, "auto: watch commit")
	if err != nil {
		t.Fatalf("commit all: %v", err)
	}
	if commit == nil {
		t.Fatal("expected a commit for the hand edit")
	}

	key, _ := NewKey("hand-edited")
	if _, err := repo.Get(ctx, key); err != nil {
		t.Errorf("get hand-edited: %v", err)
	}

	diff, err := repo.Diff(ctx, "")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if diff != "" {
		t.Errorf("tree dirty after CommitAll: %q", diff)
	}

	again, err := repo.CommitAll(ctx, "auto: watch commit")
	if err != nil {
		t.Fatalf("second commit all: %v", err)
	}
	if again != nil {
		t.Errorf("clean tree produced commit %s", again.Hash)
	}
}
