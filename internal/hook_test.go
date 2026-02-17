package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookMarker(t *testing.T) {
	script := HookScript("post-commit")
	assert.Contains(t, script, "#!/bin/sh")
	assert.Contains(t, script, HookMarker)
	assert.Contains(t, script, "mem hook run post-commit")
}

func TestIsManagedHook(t *testing.T) {
	assert.True(t, IsManagedHook(HookScript("post-commit")))
	assert.False(t, IsManagedHook("#!/bin/sh\necho hello"))
	assert.False(t, IsManagedHook(""))
}

func TestFindGitDir(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	found, err := FindGitDir(dir)
	assert.NoError(t, err)
	assert.Equal(t, gitDir, found)

	// non-git dir
	noGit := t.TempDir()
	_, err = FindGitDir(noGit)
	assert.Error(t, err)
}

// setupHookTestDir creates a temp dir with .git/hooks and .mem, chdirs to it,
// and returns the dir path, scope, resolver, and a cleanup func.
func setupHookTestDir(t *testing.T) (string, Scope, *ScopeResolver) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0755))
	memDir := filepath.Join(dir, ".mem")
	require.NoError(t, os.MkdirAll(memDir, 0755))

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	scope := Scope{Type: ScopeProject, Path: dir, MemPath: memDir}
	resolver := &ScopeResolver{homeDir: dir}
	return dir, scope, resolver
}

// --- InstallHookUseCase tests ---

func TestInstallHookUseCase(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)
	require.NoError(t, SaveConfig(scope, DefaultConfig()))

	uc := NewInstallHookUseCase(resolver)
	err := uc.Execute(context.Background(), InstallHookInput{
		Strategy: "extract",
		Force:    false,
	})
	require.NoError(t, err)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.True(t, IsManagedHook(string(content)))

	cfg, err := LoadConfig(scope)
	require.NoError(t, err)
	assert.True(t, cfg.Hooks.PostCommit.Enabled)
	assert.Equal(t, "extract", cfg.Hooks.PostCommit.Strategy)
}

func TestInstallHookUseCase_ExistingHook_NoForce(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)
	require.NoError(t, SaveConfig(scope, DefaultConfig()))

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/sh\necho existing"), 0755))

	uc := NewInstallHookUseCase(resolver)
	err := uc.Execute(context.Background(), InstallHookInput{
		Force: false,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInstallHookUseCase_ExistingHook_Force(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)
	require.NoError(t, SaveConfig(scope, DefaultConfig()))

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/sh\necho existing"), 0755))

	uc := NewInstallHookUseCase(resolver)
	err := uc.Execute(context.Background(), InstallHookInput{
		Force: true,
	})
	require.NoError(t, err)

	bakContent, err := os.ReadFile(hookPath + ".bak")
	require.NoError(t, err)
	assert.Contains(t, string(bakContent), "echo existing")

	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.True(t, IsManagedHook(string(content)))
}

// --- UninstallHookUseCase tests ---

func TestUninstallHookUseCase(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte(HookScript("post-commit")), 0755))

	cfg := DefaultConfig()
	cfg.Hooks.PostCommit = PostCommitHookConfig{Enabled: true, Strategy: "extract"}
	require.NoError(t, SaveConfig(scope, cfg))

	uc := NewUninstallHookUseCase(resolver)

	err := uc.Execute(context.Background(), UninstallHookInput{})
	require.NoError(t, err)

	_, err = os.Stat(hookPath)
	assert.True(t, os.IsNotExist(err))

	loaded, err := LoadConfig(scope)
	require.NoError(t, err)
	assert.False(t, loaded.Hooks.PostCommit.Enabled)
}

func TestUninstallHookUseCase_RestoresBackup(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte(HookScript("post-commit")), 0755))
	require.NoError(t, os.WriteFile(hookPath+".bak", []byte("#!/bin/sh\necho original"), 0755))

	require.NoError(t, SaveConfig(scope, DefaultConfig()))

	uc := NewUninstallHookUseCase(resolver)
	err := uc.Execute(context.Background(), UninstallHookInput{})
	require.NoError(t, err)

	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "echo original")

	_, err = os.Stat(hookPath + ".bak")
	assert.True(t, os.IsNotExist(err))
}

func TestUninstallHookUseCase_KeepConfig(t *testing.T) {
	dir, scope, resolver := setupHookTestDir(t)

	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte(HookScript("post-commit")), 0755))

	cfg := DefaultConfig()
	cfg.Hooks.PostCommit = PostCommitHookConfig{Enabled: true, Strategy: "all"}
	require.NoError(t, SaveConfig(scope, cfg))

	uc := NewUninstallHookUseCase(resolver)
	err := uc.Execute(context.Background(), UninstallHookInput{KeepConfig: true})
	require.NoError(t, err)

	loaded, err := LoadConfig(scope)
	require.NoError(t, err)
	assert.True(t, loaded.Hooks.PostCommit.Enabled)
}

// --- Extract Strategy tests ---

func TestStrategyExtract(t *testing.T) {
	diff := `--- a/main.go
+++ b/main.go
+func NewHandler() {}
+type UserService struct {}
--- /dev/null
+++ b/config.yaml
+key: value
--- a/old.go
+++ /dev/null
-func Deprecated() {}
`
	ctx := CommitContext{
		Hash:    "abc1234",
		Message: "feat: add handler and service",
		Author:  "dev",
		Diff:    diff,
	}

	result, err := StrategyExtract(ctx)
	require.NoError(t, err)
	assert.Contains(t, result, "NewHandler")
	assert.Contains(t, result, "UserService")
	assert.Contains(t, result, "config.yaml")
	assert.Contains(t, result, "old.go")
}

func TestStrategyExtract_EmptyDiff(t *testing.T) {
	ctx := CommitContext{Hash: "abc1234", Diff: ""}
	result, err := StrategyExtract(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

// --- Summarize Strategy tests ---

type mockProvider struct {
	completeFn func(ctx context.Context, prompt string) (string, error)
}

func (m *mockProvider) Complete(ctx context.Context, prompt string) (string, error) {
	return m.completeFn(ctx, prompt)
}

func (m *mockProvider) GenerateObject(_ context.Context, _ string, _ any) error {
	return nil
}

func (m *mockProvider) Stream(_ context.Context, _ string) (<-chan string, error) {
	return nil, nil
}

func TestStrategySummarize(t *testing.T) {
	called := false
	mp := &mockProvider{
		completeFn: func(_ context.Context, prompt string) (string, error) {
			called = true
			assert.Contains(t, prompt, "abc1234")
			assert.Contains(t, prompt, "feat: add handler")
			assert.Contains(t, prompt, "+func NewHandler")
			return "Added a new HTTP handler function", nil
		},
	}

	cc := CommitContext{
		Hash:    "abc1234",
		Message: "feat: add handler",
		Diff:    "+func NewHandler() {}",
	}

	result, err := StrategySummarize(context.Background(), cc, mp)
	require.NoError(t, err)
	assert.Equal(t, "Added a new HTTP handler function", result)
	assert.True(t, called)
}

func TestStrategySummarize_NilProvider(t *testing.T) {
	cc := CommitContext{Hash: "abc1234", Diff: "something"}
	_, err := StrategySummarize(context.Background(), cc, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider")
}

// --- Script Strategy tests ---

func TestStrategyScript(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "hook.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho $MEM_COMMIT_HASH\n"), 0755))

	cc := CommitContext{
		Hash:    "abc1234",
		Message: "test commit",
		Author:  "dev",
		Diff:    "+new line",
	}

	err := StrategyScript(context.Background(), cc, scriptPath)
	require.NoError(t, err)
}

func TestStrategyScript_NoScript(t *testing.T) {
	cc := CommitContext{Hash: "abc1234"}
	err := StrategyScript(context.Background(), cc, "")
	assert.Error(t, err)
}

func TestStrategyScript_MissingScript(t *testing.T) {
	cc := CommitContext{Hash: "abc1234"}
	err := StrategyScript(context.Background(), cc, "/nonexistent/hook.sh")
	assert.Error(t, err)
}

// --- RunHookUseCase tests ---

func TestRunHookUseCase_Extract(t *testing.T) {
	_, scope, resolver := setupHookTestDir(t)

	cfg := DefaultConfig()
	cfg.Hooks.PostCommit = PostCommitHookConfig{
		Enabled:   true,
		Strategy:  "extract",
		KeyPrefix: "hooks/commits",
	}
	require.NoError(t, SaveConfig(scope, cfg))

	var storedKey, storedContent string
	storeFn := func(_ context.Context, key, content string) error {
		storedKey = key
		storedContent = content
		return nil
	}

	uc := NewRunHookUseCase(resolver, nil, storeFn, nil)
	err := uc.Execute(context.Background(), RunHookInput{
		HookType: "post-commit",
		CommitContext: CommitContext{
			Hash:    "abc1234def",
			Message: "feat: add handler",
			Diff:    "+func NewHandler() {}",
		},
	})
	require.NoError(t, err)
	assert.Contains(t, storedKey, "hooks/commits/abc1234")
	assert.Contains(t, storedContent, "NewHandler")
}

func TestRunHookUseCase_Disabled(t *testing.T) {
	_, scope, resolver := setupHookTestDir(t)

	cfg := DefaultConfig()
	cfg.Hooks.PostCommit = PostCommitHookConfig{Enabled: false}
	require.NoError(t, SaveConfig(scope, cfg))

	uc := NewRunHookUseCase(resolver, nil, nil, nil)
	err := uc.Execute(context.Background(), RunHookInput{
		HookType:      "post-commit",
		CommitContext: CommitContext{Hash: "abc1234", Diff: "something"},
	})
	require.NoError(t, err)
}
