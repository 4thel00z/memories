package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	cmd.Env = buildExternalEnv(version)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func buildExternalEnv(version string) []string {
	memBin, _ := os.Executable()
	cwd, _ := os.Getwd()

	env := os.Environ()
	env = append(env,
		"MEM_VERSION="+version,
		"MEM_BIN="+memBin,
		"MEM_ROOT="+cwd,
	)
	return env
}
