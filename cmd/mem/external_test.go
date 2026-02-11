package main

import (
	"os"
	"path/filepath"
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
	env := buildExternalEnv("1.0.0")

	hasVersion := false
	hasBin := false
	hasRoot := false

	for _, e := range env {
		switch {
		case len(e) > 12 && e[:12] == "MEM_VERSION=":
			hasVersion = true
			if e[12:] != "1.0.0" {
				t.Errorf("expected MEM_VERSION=1.0.0, got %s", e)
			}
		case len(e) > 8 && e[:8] == "MEM_BIN=":
			hasBin = true
		case len(e) > 9 && e[:9] == "MEM_ROOT=":
			hasRoot = true
		}
	}

	if !hasVersion {
		t.Error("MEM_VERSION not found in env")
	}
	if !hasBin {
		t.Error("MEM_BIN not found in env")
	}
	if !hasRoot {
		t.Error("MEM_ROOT not found in env")
	}
}
