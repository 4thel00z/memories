package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/4thel00z/memories/internal"
)

// setupE2E initializes a full app with all services backed by a real git repo.
func setupE2E(t *testing.T) (*app, *internal.GitRepository) {
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

	repo, err := internal.NewGitRepository(scope)
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}

	resolver := internal.NewScopeResolver()
	repoFor := func(s internal.Scope) (*internal.GitRepository, error) { return repo, nil }
	indexFor := func(s internal.Scope) (*internal.AnnoyIndex, error) { return nil, internal.ErrNoIndex }

	a := &app{
		resolver:     resolver,
		memorySvc:    internal.NewMemoryService(resolver, repoFor, indexFor, nil),
		historySvc:   internal.NewHistoryService(resolver, repoFor),
		branchSvc:    internal.NewBranchService(resolver, repoFor),
		searchSvc:    internal.NewSearchService(resolver, repoFor, indexFor, nil),
		summarizeSvc: internal.NewSummarizeService(resolver, repoFor, nil),
		providerSvc:  internal.NewProviderService(resolver),
	}
	return a, repo
}

func TestE2EFullWorkflow(t *testing.T) {
	a, _ := setupE2E(t)

	// 1. Set multiple memories
	for _, tc := range []struct {
		key, val string
	}{
		{"project/name", "memories"},
		{"project/lang", "go"},
		{"notes/todo", "write tests"},
	} {
		root := NewRootCmd("test", a)
		root.SetArgs([]string{"set", tc.key, tc.val})
		var out bytes.Buffer
		root.SetOut(&out)
		if err := root.Execute(); err != nil {
			t.Fatalf("set %s: %v", tc.key, err)
		}
	}

	// 2. Get and verify each memory
	for _, tc := range []struct {
		key, want string
	}{
		{"project/name", "memories"},
		{"project/lang", "go"},
		{"notes/todo", "write tests"},
	} {
		root := NewRootCmd("test", a)
		root.SetArgs([]string{"get", tc.key})
		var out bytes.Buffer
		root.SetOut(&out)
		if err := root.Execute(); err != nil {
			t.Fatalf("get %s: %v", tc.key, err)
		}
		if got := out.String(); got != tc.want {
			t.Errorf("get %s = %q, want %q", tc.key, got, tc.want)
		}
	}

	// 3. List all memories
	listRoot := NewRootCmd("test", a)
	listRoot.SetArgs([]string{"list"})
	var listOut bytes.Buffer
	listRoot.SetOut(&listOut)
	if err := listRoot.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(listOut.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("list: expected 3 lines, got %d: %v", len(lines), lines)
	}

	// 4. List with prefix filter
	listPrefixRoot := NewRootCmd("test", a)
	listPrefixRoot.SetArgs([]string{"list", "project"})
	var listPrefixOut bytes.Buffer
	listPrefixRoot.SetOut(&listPrefixOut)
	if err := listPrefixRoot.Execute(); err != nil {
		t.Fatalf("list project: %v", err)
	}
	lines = strings.Split(strings.TrimSpace(listPrefixOut.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("list project: expected 2 lines, got %d: %v", len(lines), lines)
	}

	// 5. Search by keyword
	searchRoot := NewRootCmd("test", a)
	searchRoot.SetArgs([]string{"search", "tests"})
	var searchOut bytes.Buffer
	searchRoot.SetOut(&searchOut)
	if err := searchRoot.Execute(); err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(searchOut.String(), "notes/todo") {
		t.Errorf("search 'tests' should find notes/todo, got: %q", searchOut.String())
	}

	// 6. Delete a memory
	delRoot := NewRootCmd("test", a)
	delRoot.SetArgs([]string{"del", "notes/todo"})
	var delOut bytes.Buffer
	delRoot.SetOut(&delOut)
	if err := delRoot.Execute(); err != nil {
		t.Fatalf("del: %v", err)
	}
	if !strings.Contains(delOut.String(), "Deleted notes/todo") {
		t.Errorf("del output = %q, want 'Deleted notes/todo'", delOut.String())
	}

	// 7. Verify deleted memory is gone
	getDelRoot := NewRootCmd("test", a)
	getDelRoot.SetArgs([]string{"get", "notes/todo"})
	var getOut bytes.Buffer
	getDelRoot.SetOut(&getOut)
	getDelRoot.SetErr(&getOut)
	if err := getDelRoot.Execute(); err == nil {
		t.Error("expected error getting deleted memory")
	}

	// 8. List should now have 2
	listAfterRoot := NewRootCmd("test", a)
	listAfterRoot.SetArgs([]string{"list"})
	var listAfterOut bytes.Buffer
	listAfterRoot.SetOut(&listAfterOut)
	if err := listAfterRoot.Execute(); err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	lines = strings.Split(strings.TrimSpace(listAfterOut.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("list after delete: expected 2, got %d: %v", len(lines), lines)
	}

	// 9. Check log has commits from set + del
	logRoot := NewRootCmd("test", a)
	logRoot.SetArgs([]string{"log", "--oneline"})
	var logOut bytes.Buffer
	logRoot.SetOut(&logOut)
	if err := logRoot.Execute(); err != nil {
		t.Fatalf("log: %v", err)
	}
	logLines := strings.Split(strings.TrimSpace(logOut.String()), "\n")
	// init commit + 3 sets + 1 del = at least 5
	if len(logLines) < 5 {
		t.Errorf("log: expected at least 5 commits, got %d: %v", len(logLines), logLines)
	}

	// 10. Status shows current branch
	statusRoot := NewRootCmd("test", a)
	statusRoot.SetArgs([]string{"status"})
	var statusOut bytes.Buffer
	statusRoot.SetOut(&statusOut)
	if err := statusRoot.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.HasPrefix(statusOut.String(), "On branch ") {
		t.Errorf("status output = %q, want 'On branch ...'", statusOut.String())
	}
}

func TestE2EBranchWorkflow(t *testing.T) {
	a, _ := setupE2E(t)

	// Set a memory on the default branch
	root := NewRootCmd("test", a)
	root.SetArgs([]string{"set", "shared/key", "on main"})
	var out bytes.Buffer
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("set on main: %v", err)
	}

	// Create and switch to a new branch
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"branch", "feature"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("branch feature: %v", err)
	}
	if !strings.Contains(out.String(), "Switched to new branch feature") {
		t.Errorf("branch output = %q", out.String())
	}

	// Status should show feature branch
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"status"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out.String(), "feature") {
		t.Errorf("status after branch = %q, want 'feature'", out.String())
	}

	// Set a memory on the feature branch
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"set", "feature/key", "on feature"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("set on feature: %v", err)
	}

	// List branches
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"branch"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("list branches: %v", err)
	}
	branchOutput := out.String()
	if !strings.Contains(branchOutput, "feature") {
		t.Errorf("branch list should contain 'feature', got: %q", branchOutput)
	}
	if !strings.Contains(branchOutput, "* feature") {
		t.Errorf("feature should be current branch, got: %q", branchOutput)
	}
}

func TestE2EAddAppend(t *testing.T) {
	a, _ := setupE2E(t)

	// Create initial memory
	root := NewRootCmd("test", a)
	root.SetArgs([]string{"set", "notes/log", "line one"})
	var out bytes.Buffer
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Append to it
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"add", "notes/log", "line two"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out.String(), "Appended") {
		t.Errorf("add output = %q, want 'Appended'", out.String())
	}

	// Verify content has both lines
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"get", "notes/log"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("get after add: %v", err)
	}
	content := out.String()
	if !strings.Contains(content, "line one") || !strings.Contains(content, "line two") {
		t.Errorf("content after add = %q, want both lines", content)
	}
}

func TestE2EDiffWorkflow(t *testing.T) {
	a, repo := setupE2E(t)

	// Commit initial state
	root := NewRootCmd("test", a)
	root.SetArgs([]string{"commit", "-m", "initial"})
	var out bytes.Buffer
	root.SetOut(&out)
	// May fail if nothing to commit - that's fine, init already committed
	_ = root.Execute()

	// Set a memory (auto-commits)
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"set", "diff/test", "hello"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Diff should show no changes (set auto-commits)
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"diff"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("diff: %v", err)
	}
	if !strings.Contains(out.String(), "No changes") {
		t.Errorf("diff after auto-commit = %q, want 'No changes'", out.String())
	}

	// Now save directly to repo without committing (bypassing auto-commit)
	ctx := context.Background()
	key, _ := internal.NewKey("diff/staged")
	mem := internal.NewMemory(key, []byte("staged content"))
	if err := repo.Save(ctx, mem); err != nil {
		t.Fatalf("direct save: %v", err)
	}

	// Diff should now show changes
	root = NewRootCmd("test", a)
	root.SetArgs([]string{"diff"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("diff with staged: %v", err)
	}
	if strings.Contains(out.String(), "No changes") {
		t.Error("diff should show changes after staging without commit")
	}
}

func TestE2EGetJSON(t *testing.T) {
	a, _ := setupE2E(t)

	root := NewRootCmd("test", a)
	root.SetArgs([]string{"set", "json/test", "hello json"})
	var out bytes.Buffer
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}

	root = NewRootCmd("test", a)
	root.SetArgs([]string{"get", "--json", "json/test"})
	out.Reset()
	root.SetOut(&out)
	if err := root.Execute(); err != nil {
		t.Fatalf("get --json: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `"key"`) {
		t.Errorf("JSON output should contain key field, got: %q", output)
	}
	if !strings.Contains(output, `"hello json"`) {
		t.Errorf("JSON output should contain content, got: %q", output)
	}
	if !strings.Contains(output, `"created_at"`) {
		t.Errorf("JSON output should contain created_at, got: %q", output)
	}
}
