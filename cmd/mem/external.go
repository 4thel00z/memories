package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/4thel00z/memories/internal"
)

const externalPrefix = "mem-"

func findExternal(name string) (string, error) {
	binary := externalPrefix + name
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("unknown command %q: %s not found in PATH", name, binary)
	}
	return path, nil
}

func listExternalCommands() []string {
	var commands []string
	seen := make(map[string]bool)

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		commands = appendExternalsFromDir(dir, seen, commands)
	}
	return commands
}

func appendExternalsFromDir(dir string, seen map[string]bool, commands []string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return commands
	}

	for _, entry := range entries {
		name := extractExternalName(dir, entry)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		commands = append(commands, name)
	}
	return commands
}

func extractExternalName(dir string, entry os.DirEntry) string {
	if entry.IsDir() {
		return ""
	}

	name := entry.Name()
	if !strings.HasPrefix(name, externalPrefix) {
		return ""
	}

	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	if info.Mode()&0111 == 0 {
		return ""
	}

	return strings.TrimPrefix(name, externalPrefix)
}

func executeExternal(ctx context.Context, name string, args []string, version string) error {
	binaryPath, err := findExternal(name)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = buildExternalEnv(ctx, version)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildExternalEnv(ctx context.Context, version string) []string {
	resolver := internal.NewScopeResolver()
	scope := resolver.Resolve("")

	// Read HEAD directly instead of opening the repository: this runs on
	// every plugin dispatch. An uninitialized store leaves MEM_BRANCH empty.
	branch := internal.HeadBranch(scope)

	vars := resolver.EnvVars(scope, branch, version)
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	env := os.Environ()
	for _, k := range keys {
		env = append(env, k+"="+vars[k])
	}
	return env
}
