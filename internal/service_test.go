package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupServiceTest(t *testing.T) (*GitRepository, *ScopeResolver) {
	t.Helper()
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

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

	return repo, NewScopeResolver()
}

func TestMemoryServiceSetAndGet(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	svc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)

	if err := svc.Set(ctx, "svc/key", "svc value", ""); err != nil {
		t.Fatalf("set: %v", err)
	}

	mem, err := svc.Get(ctx, "svc/key", "")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if string(mem.Content) != "svc value" {
		t.Errorf("content = %q, want %q", string(mem.Content), "svc value")
	}
}

func TestMemoryServiceDelete(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	svc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)

	if err := svc.Set(ctx, "del-me", "bye", ""); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := svc.Delete(ctx, "del-me", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.Get(ctx, "del-me", "")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMemoryServiceList(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	svc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)

	for _, key := range []string{"ns/a", "ns/b", "other/c"} {
		if err := svc.Set(ctx, key, "val", ""); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}

	all, err := svc.List(ctx, "", "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}

	filtered, err := svc.List(ctx, "ns", "")
	if err != nil {
		t.Fatalf("list ns: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2, got %d", len(filtered))
	}
}

func TestMemoryServiceInvalidKey(t *testing.T) {
	repo, resolver := setupServiceTest(t)

	svc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)

	err := svc.Set(context.Background(), "", "val", "")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestHistoryServiceCommitAndLog(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	memSvc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)
	histSvc := NewHistoryService(resolver, func(s Scope) (*GitRepository, error) { return repo, nil })

	if err := memSvc.Set(ctx, "logged", "data", ""); err != nil {
		t.Fatalf("set: %v", err)
	}

	commit, err := histSvc.Commit(ctx, "test: service commit", "")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.Hash == "" {
		t.Error("commit hash is empty")
	}

	commits, err := histSvc.Log(ctx, 10, "")
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	found := false
	for _, c := range commits {
		if c.Message == "test: service commit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("commit not found in log")
	}
}

func TestSearchServiceKeyword(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	memSvc := NewMemoryService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)
	searchSvc := NewSearchService(
		resolver,
		func(s Scope) (*GitRepository, error) { return repo, nil },
		func(s Scope) (*AnnoyIndex, error) { return nil, ErrNoIndex },
		nil,
	)

	if err := memSvc.Set(ctx, "haystack", "needle in the content", ""); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := memSvc.Set(ctx, "other", "nothing here", ""); err != nil {
		t.Fatalf("set: %v", err)
	}

	results, err := searchSvc.Keyword(ctx, "needle", "")
	if err != nil {
		t.Fatalf("keyword search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Key.String() != "haystack" {
		t.Errorf("expected key 'haystack', got %q", results[0].Key.String())
	}
}

func TestBranchServiceCreateAndSwitch(t *testing.T) {
	repo, resolver := setupServiceTest(t)
	ctx := context.Background()

	branchSvc := NewBranchService(resolver, func(s Scope) (*GitRepository, error) { return repo, nil })

	if err := branchSvc.Create(ctx, "dev", ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := branchSvc.Switch(ctx, "dev", ""); err != nil {
		t.Fatalf("switch: %v", err)
	}

	current, err := branchSvc.Current(ctx, "")
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.Name != "dev" {
		t.Errorf("current = %q, want %q", current.Name, "dev")
	}
}
