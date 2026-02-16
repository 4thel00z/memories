package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupUseCaseTest(t *testing.T) (*GitRepository, *ScopeResolver) {
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

func TestSetAndGetUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	repoFor := func(s Scope) (MemoryRepository, error) { return repo, nil }
	histFor := func(s Scope) (HistoryRepository, error) { return repo, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	getUC := NewGetMemoryUseCase(resolver, repoFor)
	commitUC := NewCommitUseCase(resolver, histFor)

	if err := setUC.Execute(ctx, SetMemoryInput{Key: "uc/key", Content: "uc value"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	if _, err := commitUC.Execute(ctx, CommitInput{Message: "test: set uc/key"}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	out, err := getUC.Execute(ctx, GetMemoryInput{Key: "uc/key"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if out.Content != "uc value" {
		t.Errorf("content = %q, want %q", out.Content, "uc value")
	}
}

func TestDeleteUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	repoFor := func(s Scope) (MemoryRepository, error) { return repo, nil }
	histFor := func(s Scope) (HistoryRepository, error) { return repo, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	delUC := NewDeleteMemoryUseCase(resolver, repoFor, nilIndex)
	getUC := NewGetMemoryUseCase(resolver, repoFor)
	commitUC := NewCommitUseCase(resolver, histFor)

	if err := setUC.Execute(ctx, SetMemoryInput{Key: "del-me", Content: "bye"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := commitUC.Execute(ctx, CommitInput{Message: "test: set"}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := delUC.Execute(ctx, DeleteMemoryInput{Key: "del-me"}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := getUC.Execute(ctx, GetMemoryInput{Key: "del-me"})
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestListUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	repoFor := func(s Scope) (MemoryRepository, error) { return repo, nil }
	histFor := func(s Scope) (HistoryRepository, error) { return repo, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	listUC := NewListMemoriesUseCase(resolver, repoFor)
	commitUC := NewCommitUseCase(resolver, histFor)

	for _, key := range []string{"ns/a", "ns/b", "other/c"} {
		if err := setUC.Execute(ctx, SetMemoryInput{Key: key, Content: "val"}); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
	if _, err := commitUC.Execute(ctx, CommitInput{Message: "test: set all"}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	all, err := listUC.Execute(ctx, ListMemoriesInput{})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all.Memories) != 3 {
		t.Errorf("expected 3, got %d", len(all.Memories))
	}

	filtered, err := listUC.Execute(ctx, ListMemoriesInput{Prefix: "ns"})
	if err != nil {
		t.Fatalf("list ns: %v", err)
	}
	if len(filtered.Memories) != 2 {
		t.Errorf("expected 2, got %d", len(filtered.Memories))
	}
}

func TestInvalidKeyUseCase(t *testing.T) {
	_, resolver := setupUseCaseTest(t)

	repoFor := func(s Scope) (MemoryRepository, error) { return nil, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)

	err := setUC.Execute(context.Background(), SetMemoryInput{Key: "", Content: "val"})
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestCommitAndLogUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	repoFor := func(s Scope) (MemoryRepository, error) { return repo, nil }
	histFor := func(s Scope) (HistoryRepository, error) { return repo, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	commitUC := NewCommitUseCase(resolver, histFor)
	logUC := NewLogUseCase(resolver, histFor)

	if err := setUC.Execute(ctx, SetMemoryInput{Key: "logged", Content: "data"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	commit, err := commitUC.Execute(ctx, CommitInput{Message: "test: usecase commit"})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.Hash == "" {
		t.Error("commit hash is empty")
	}

	logOut, err := logUC.Execute(ctx, LogInput{Limit: 10})
	if err != nil {
		t.Fatalf("log: %v", err)
	}

	found := false
	for _, c := range logOut.Commits {
		if c.Message == "test: usecase commit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("commit not found in log")
	}
}

func TestKeywordSearchUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	repoFor := func(s Scope) (MemoryRepository, error) { return repo, nil }
	histFor := func(s Scope) (HistoryRepository, error) { return repo, nil }
	nilIndex := func(s Scope) (VectorIndex, error) { return nil, ErrNoIndex }

	setUC := NewSetMemoryUseCase(resolver, repoFor, nilIndex, nil, nil)
	commitUC := NewCommitUseCase(resolver, histFor)
	searchUC := NewKeywordSearchUseCase(resolver, repoFor)

	if err := setUC.Execute(ctx, SetMemoryInput{Key: "haystack", Content: "needle in the content"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := setUC.Execute(ctx, SetMemoryInput{Key: "other", Content: "nothing here"}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := commitUC.Execute(ctx, CommitInput{Message: "test: setup"}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	out, err := searchUC.Execute(ctx, SearchInput{Query: "needle"})
	if err != nil {
		t.Fatalf("keyword search: %v", err)
	}

	if len(out.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(out.Results))
	}
	if len(out.Results) > 0 && out.Results[0].Key != "haystack" {
		t.Errorf("expected key 'haystack', got %q", out.Results[0].Key)
	}
}

func TestBranchCreateAndSwitchUseCase(t *testing.T) {
	repo, resolver := setupUseCaseTest(t)
	ctx := context.Background()

	branchFor := func(s Scope) (BranchRepository, error) { return repo, nil }

	createUC := NewBranchCreateUseCase(resolver, branchFor)
	switchUC := NewBranchSwitchUseCase(resolver, branchFor)
	currentUC := NewBranchCurrentUseCase(resolver, branchFor)

	if _, err := createUC.Execute(ctx, BranchInput{Name: "dev"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := switchUC.Execute(ctx, BranchInput{Name: "dev"}); err != nil {
		t.Fatalf("switch: %v", err)
	}

	current, err := currentUC.Execute(ctx, BranchInput{})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.Name != "dev" {
		t.Errorf("current = %q, want %q", current.Name, "dev")
	}
}
