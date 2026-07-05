package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindExternal(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "mem-test")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok"), 0755); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("PATH")
	t.Setenv("PATH", tmp+":"+orig)

	path, err := findExternal("test")
	if err != nil {
		t.Fatalf("expected to find mem-test, got error: %v", err)
	}
	if path != script {
		t.Errorf("expected %s, got %s", script, path)
	}
}

func TestFindExternalNotFound(t *testing.T) {
	_, err := findExternal("nonexistent-command-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestListExternalCommands(t *testing.T) {
	tmp := t.TempDir()

	scripts := []string{"mem-foo", "mem-bar", "mem-baz"}
	for _, s := range scripts {
		path := filepath.Join(tmp, s)
		if err := os.WriteFile(path, []byte("#!/bin/sh"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Add non-mem script (should be ignored)
	other := filepath.Join(tmp, "other-script")
	if err := os.WriteFile(other, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	orig := os.Getenv("PATH")
	t.Setenv("PATH", tmp+":"+orig)

	cmds := listExternalCommands()

	found := make(map[string]bool)
	for _, c := range cmds {
		found[c] = true
	}

	for _, expected := range []string{"foo", "bar", "baz"} {
		if !found[expected] {
			t.Errorf("expected to find %q in external commands", expected)
		}
	}

	if found["other-script"] {
		t.Error("non-mem script should not be listed")
	}
}

func TestExtractExternalName(t *testing.T) {
	tmp := t.TempDir()

	script := filepath.Join(tmp, "mem-hello")
	if err := os.WriteFile(script, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.Name() == "mem-hello" {
			name := extractExternalName(tmp, e)
			if name != "hello" {
				t.Errorf("expected 'hello', got %q", name)
			}
			return
		}
	}
	t.Fatal("mem-hello not found in dir entries")
}

func TestExtractExternalNameNotExecutable(t *testing.T) {
	tmp := t.TempDir()

	script := filepath.Join(tmp, "mem-noexec")
	if err := os.WriteFile(script, []byte("#!/bin/sh"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.Name() == "mem-noexec" {
			name := extractExternalName(tmp, e)
			if name != "" {
				t.Errorf("expected empty string for non-executable, got %q", name)
			}
			return
		}
	}
	t.Fatal("mem-noexec not found in dir entries")
}

func TestBuildExternalEnv(t *testing.T) {
	env := buildExternalEnv(context.Background(), "1.0.0")

	got := make(map[string]string)
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok && strings.HasPrefix(k, "MEM_") {
			got[k] = v
		}
	}

	// MEM_BRANCH is always present, but may be empty outside an initialized store.
	for _, k := range []string{"MEM_SCOPE", "MEM_SCOPE_PATH", "MEM_ROOT", "MEM_BRANCH", "MEM_CONFIG", "MEM_VERSION", "MEM_BIN"} {
		if _, ok := got[k]; !ok {
			t.Errorf("%s not found in env", k)
		}
	}

	if got["MEM_VERSION"] != "1.0.0" {
		t.Errorf("expected MEM_VERSION=1.0.0, got %q", got["MEM_VERSION"])
	}
}
