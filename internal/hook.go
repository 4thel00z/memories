package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const HookMarker = "# mem: managed post-commit hook"

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
