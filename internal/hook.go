package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const HookMarker = "# mem: managed post-commit hook"

// managedBackupSuffix names backups mem itself created, so uninstall never
// restores an unrelated post-commit.bak left by another tool.
const managedBackupSuffix = ".mem.bak"

var validStrategies = map[string]bool{
	"summarize": true,
	"extract":   true,
	"script":    true,
	"all":       true,
}

// HookScript returns the shell shim content for a given hook type.
func HookScript(hookType string) string {
	return fmt.Sprintf("#!/bin/sh\n%s\nexec mem hook run %s \"$@\"\n", HookMarker, hookType)
}

// IsManagedHook checks if the given script content was written by mem.
func IsManagedHook(content string) bool {
	return strings.Contains(content, HookMarker)
}

// FindGitDir walks up from dir looking for a .git directory.
func FindGitDir(dir string) (string, error) {
	for {
		gitDir := filepath.Join(dir, ".git")
		info, err := os.Stat(gitDir)
		if err == nil && info.IsDir() {
			return gitDir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository (no .git found)")
		}
		dir = parent
	}
}

// CommitContext holds metadata about a git commit for hook processing.
type CommitContext struct {
	Hash    string
	Message string
	Author  string
	Diff    string
}

// --- InstallHookUseCase ---

type InstallHookInput struct {
	Scope    string
	Strategy string
	Script   string
	Force    bool
}

type InstallHookUseCase struct {
	resolver *ScopeResolver
}

func NewInstallHookUseCase(resolver *ScopeResolver) *InstallHookUseCase {
	return &InstallHookUseCase{resolver: resolver}
}

func (uc *InstallHookUseCase) Execute(_ context.Context, input InstallHookInput) error {
	scope := uc.resolver.Resolve(input.Scope)

	gitDir, err := FindGitDir(scope.Path)
	if err != nil {
		return err
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "post-commit")

	if existing, err := os.ReadFile(hookPath); err == nil {
		if !IsManagedHook(string(existing)) {
			if !input.Force {
				return fmt.Errorf("hook already exists at %s (use --force to overwrite)", hookPath)
			}
			// Preserve the original file's permission bits in a mem-specific backup
			// name so uninstall can restore exactly what mem replaced (and never
			// mistakes an unrelated post-commit.bak for its own backup).
			mode := os.FileMode(0755)
			if info, statErr := os.Stat(hookPath); statErr == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(hookPath+managedBackupSuffix, existing, mode); err != nil {
				return fmt.Errorf("backup existing hook: %w", err)
			}
		}
	}

	if err := os.WriteFile(hookPath, []byte(HookScript("post-commit")), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	cfg, err := LoadConfig(scope)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	strategy := input.Strategy
	if strategy == "" {
		strategy = "extract"
	}
	if !validStrategies[strategy] {
		return fmt.Errorf("invalid strategy %q (want one of: summarize, extract, script, all)", strategy)
	}
	if input.Script != "" && strategy != "script" && strategy != "all" {
		return fmt.Errorf("--script is only used with strategy=script or strategy=all")
	}

	cfg.Hooks.PostCommit = PostCommitHookConfig{
		Enabled:   true,
		Scope:     input.Scope,
		Strategy:  strategy,
		Script:    input.Script,
		KeyPrefix: "hooks/commits",
	}

	if err := SaveConfig(scope, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

// --- UninstallHookUseCase ---

type UninstallHookInput struct {
	Scope      string
	KeepConfig bool
}

type UninstallHookUseCase struct {
	resolver *ScopeResolver
}

func NewUninstallHookUseCase(resolver *ScopeResolver) *UninstallHookUseCase {
	return &UninstallHookUseCase{resolver: resolver}
}

func (uc *UninstallHookUseCase) Execute(_ context.Context, input UninstallHookInput) error {
	scope := uc.resolver.Resolve(input.Scope)

	gitDir, err := FindGitDir(scope.Path)
	if err != nil {
		return err
	}

	hookPath := filepath.Join(gitDir, "hooks", "post-commit")

	if content, err := os.ReadFile(hookPath); err == nil {
		if !IsManagedHook(string(content)) {
			return fmt.Errorf("hook at %s is not managed by mem", hookPath)
		}

		bakPath := hookPath + managedBackupSuffix
		if bakContent, err := os.ReadFile(bakPath); err == nil {
			// Restore with the backup's preserved permission bits, not a hardcoded mode.
			mode := os.FileMode(0755)
			if info, statErr := os.Stat(bakPath); statErr == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(hookPath, bakContent, mode); err != nil {
				return fmt.Errorf("restore backup: %w", err)
			}
			if err := os.Remove(bakPath); err != nil {
				return fmt.Errorf("remove backup: %w", err)
			}
		} else if err := os.Remove(hookPath); err != nil {
			return fmt.Errorf("remove hook: %w", err)
		}
	}

	if !input.KeepConfig {
		cfg, err := LoadConfig(scope)
		if err != nil {
			return nil
		}
		cfg.Hooks.PostCommit = PostCommitHookConfig{}
		_ = SaveConfig(scope, cfg)
	}

	return nil
}

// --- Extract Strategy ---

var (
	diffFileAddedRe   = regexp.MustCompile(`(?m)^\+\+\+ b/(.+)$`)
	diffFileDeletedRe = regexp.MustCompile(`(?m)^--- a/(.+)$`)
	diffFuncAddedRe   = regexp.MustCompile(`(?m)^\+.*func\s+(\w+)`)
	diffTypeAddedRe   = regexp.MustCompile(`(?m)^\+.*type\s+(\w+)`)
	diffFuncRemovedRe = regexp.MustCompile(`(?m)^-.*func\s+(\w+)`)
	diffTypeRemovedRe = regexp.MustCompile(`(?m)^-.*type\s+(\w+)`)
	configFileRe      = regexp.MustCompile(`\.(yaml|yml|json|toml)$`)
)

// StrategyExtract parses a diff for structural changes without LLM.
func StrategyExtract(ctx CommitContext) (string, error) {
	if ctx.Diff == "" {
		return "", nil
	}

	var parts []string

	addedFiles := diffFileAddedRe.FindAllStringSubmatch(ctx.Diff, -1)
	deletedPrefixes := diffFileDeletedRe.FindAllStringSubmatch(ctx.Diff, -1)

	deletedSet := make(map[string]bool)
	for _, m := range deletedPrefixes {
		if m[1] != "/dev/null" {
			deletedSet[m[1]] = true
		}
	}

	var newFiles, removedFiles, configFiles []string
	for _, m := range addedFiles {
		name := m[1]
		if name == "/dev/null" {
			continue
		}
		if !deletedSet[name] {
			newFiles = append(newFiles, name)
		}
		if configFileRe.MatchString(name) {
			configFiles = append(configFiles, name)
		}
	}

	for _, m := range deletedPrefixes {
		name := m[1]
		if name == "/dev/null" {
			continue
		}
		found := false
		for _, a := range addedFiles {
			if a[1] == name {
				found = true
				break
			}
		}
		if !found {
			removedFiles = append(removedFiles, name)
		}
	}

	if len(newFiles) > 0 {
		parts = append(parts, fmt.Sprintf("added files: %s", strings.Join(newFiles, ", ")))
	}
	if len(removedFiles) > 0 {
		parts = append(parts, fmt.Sprintf("removed files: %s", strings.Join(removedFiles, ", ")))
	}
	if len(configFiles) > 0 {
		parts = append(parts, fmt.Sprintf("config changes: %s", strings.Join(configFiles, ", ")))
	}

	funcsAdded := uniqueMatches(diffFuncAddedRe.FindAllStringSubmatch(ctx.Diff, -1))
	typesAdded := uniqueMatches(diffTypeAddedRe.FindAllStringSubmatch(ctx.Diff, -1))
	funcsRemoved := uniqueMatches(diffFuncRemovedRe.FindAllStringSubmatch(ctx.Diff, -1))
	typesRemoved := uniqueMatches(diffTypeRemovedRe.FindAllStringSubmatch(ctx.Diff, -1))

	if len(funcsAdded) > 0 {
		parts = append(parts, fmt.Sprintf("new funcs: %s", strings.Join(funcsAdded, ", ")))
	}
	if len(typesAdded) > 0 {
		parts = append(parts, fmt.Sprintf("new types: %s", strings.Join(typesAdded, ", ")))
	}
	if len(funcsRemoved) > 0 {
		parts = append(parts, fmt.Sprintf("removed funcs: %s", strings.Join(funcsRemoved, ", ")))
	}
	if len(typesRemoved) > 0 {
		parts = append(parts, fmt.Sprintf("removed types: %s", strings.Join(typesRemoved, ", ")))
	}

	if len(parts) == 0 {
		return "", nil
	}

	return fmt.Sprintf("[%s] %s — %s", ShortHash(ctx.Hash), ctx.Message, strings.Join(parts, "; ")), nil
}

func uniqueMatches(matches [][]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

// --- Summarize Strategy ---

// StrategySummarize sends the diff to an LLM provider for summarization.
func StrategySummarize(ctx context.Context, cc CommitContext, provider Provider) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("no provider configured: skipping summarize")
	}

	prompt := fmt.Sprintf(`Summarize the following git commit in 1-3 sentences.
Focus on what changed and why.

Commit: %s
Message: %s

Diff:
%s`, cc.Hash, cc.Message, cc.Diff)

	return provider.Complete(ctx, prompt)
}

// --- Script Strategy ---

// StrategyScript runs a user-defined script with commit context.
func StrategyScript(ctx context.Context, cc CommitContext, scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("no script configured")
	}

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Stdin = strings.NewReader(cc.Diff)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"MEM_COMMIT_HASH="+cc.Hash,
		"MEM_COMMIT_MSG="+cc.Message,
		"MEM_COMMIT_AUTHOR="+cc.Author,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script %s: %w", scriptPath, err)
	}

	return nil
}

// --- RunHookUseCase ---

// StoreFunc is a function that stores a memory key/value pair. The store path
// keeps the vector index in sync incrementally, so the hook does not perform a
// separate full reindex (which would otherwise block every commit).
//
// scope is the scope (e.g. "global" or "project") the hook config was resolved
// from, so the stored memory lands in the same store the hook was configured in
// rather than wherever Resolve("") would independently pick.
type StoreFunc func(ctx context.Context, scope, key, content string) error

type RunHookInput struct {
	HookType      string
	CommitContext CommitContext
}

type RunHookUseCase struct {
	resolver *ScopeResolver
	provider Provider
	storeFn  StoreFunc
}

func NewRunHookUseCase(
	resolver *ScopeResolver,
	provider Provider,
	storeFn StoreFunc,
) *RunHookUseCase {
	return &RunHookUseCase{
		resolver: resolver,
		provider: provider,
		storeFn:  storeFn,
	}
}

func (uc *RunHookUseCase) Execute(_ context.Context, input RunHookInput) error {
	// The hook may have been installed against a different scope than Resolve("")
	// would pick (e.g. installed --scope global while a project .mem also exists).
	// Search the cascade for the first scope whose post-commit hook is enabled,
	// rather than silently no-opping on the wrong config.
	var hc PostCommitHookConfig
	var storeScope string
	found := false
	for _, scope := range uc.resolver.Cascade() {
		cfg, err := LoadConfig(scope)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem hook: load config (%s): %v\n", scope.MemPath, err)
			continue
		}
		if cfg.Hooks.PostCommit.Enabled {
			hc = cfg.Hooks.PostCommit
			// Store into the same scope the config came from, so config-read and
			// memory-write can't diverge to different stores.
			storeScope = string(scope.Type)
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	if input.CommitContext.Diff == "" {
		return nil
	}

	cc := input.CommitContext
	prefix := hc.KeyPrefix
	if prefix == "" {
		prefix = "hooks/commits"
	}

	baseKey := fmt.Sprintf("%s/%s", prefix, ShortHash(cc.Hash))

	strategy := hc.Strategy
	if strategy == "" {
		strategy = "extract"
	}

	quiet := hc.Quiet
	warn := func(msg string, args ...any) {
		if !quiet {
			fmt.Fprintf(os.Stderr, "mem hook: "+msg+"\n", args...)
		}
	}

	ctx := context.Background()

	switch strategy {
	case "extract":
		uc.runExtract(ctx, storeScope, cc, baseKey, warn)
	case "summarize":
		uc.runSummarize(ctx, storeScope, cc, baseKey, warn)
	case "script":
		uc.runScript(ctx, cc, hc.Script, warn)
	case "all":
		uc.runExtract(ctx, storeScope, cc, baseKey, warn)
		uc.runSummarize(ctx, storeScope, cc, baseKey+"/summary", warn)
		if hc.Script != "" {
			uc.runScript(ctx, cc, hc.Script, warn)
		}
	}

	return nil
}

func (uc *RunHookUseCase) runExtract(ctx context.Context, scope string, cc CommitContext, key string, warn func(string, ...any)) {
	result, err := StrategyExtract(cc)
	if err != nil {
		warn("extract: %v", err)
		return
	}
	if result == "" {
		return
	}
	if uc.storeFn != nil {
		if err := uc.storeFn(ctx, scope, key, result); err != nil {
			warn("extract store: %v", err)
		}
	}
}

func (uc *RunHookUseCase) runSummarize(ctx context.Context, scope string, cc CommitContext, key string, warn func(string, ...any)) {
	result, err := StrategySummarize(ctx, cc, uc.provider)
	if err != nil {
		warn("summarize: %v", err)
		return
	}
	if uc.storeFn != nil {
		if err := uc.storeFn(ctx, scope, key, result); err != nil {
			warn("summarize store: %v", err)
		}
	}
}

func (uc *RunHookUseCase) runScript(ctx context.Context, cc CommitContext, script string, warn func(string, ...any)) {
	if err := StrategyScript(ctx, cc, script); err != nil {
		warn("script: %v", err)
	}
}
