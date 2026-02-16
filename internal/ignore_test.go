package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatcherEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	scope := Scope{Path: tmpDir}

	m, err := NewIgnoreMatcher(scope)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	key, _ := NewKey("anything/goes")
	if m.MatchKey(key) {
		t.Error("empty ignore should not match anything")
	}
}

func TestIgnoreMatcherExactPattern(t *testing.T) {
	tmpDir := t.TempDir()
	ignoreFile := filepath.Join(tmpDir, IgnoreFilename)
	if err := os.WriteFile(ignoreFile, []byte("secret\n"), 0644); err != nil {
		t.Fatalf("write ignore file: %v", err)
	}

	scope := Scope{Path: tmpDir}
	m, err := NewIgnoreMatcher(scope)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	key, _ := NewKey("secret")
	if !m.MatchKey(key) {
		t.Error("expected 'secret' to be ignored")
	}

	key2, _ := NewKey("public")
	if m.MatchKey(key2) {
		t.Error("expected 'public' to not be ignored")
	}
}

func TestIgnoreMatcherGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()
	ignoreFile := filepath.Join(tmpDir, IgnoreFilename)
	if err := os.WriteFile(ignoreFile, []byte("*.tmp\n"), 0644); err != nil {
		t.Fatalf("write ignore file: %v", err)
	}

	scope := Scope{Path: tmpDir}
	m, err := NewIgnoreMatcher(scope)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	key, _ := NewKey("data.tmp")
	if !m.MatchKey(key) {
		t.Error("expected '*.tmp' pattern to match 'data.tmp'")
	}

	key2, _ := NewKey("data.txt")
	if m.MatchKey(key2) {
		t.Error("expected '*.tmp' pattern to not match 'data.txt'")
	}
}

func TestIgnoreMatcherComments(t *testing.T) {
	tmpDir := t.TempDir()
	ignoreFile := filepath.Join(tmpDir, IgnoreFilename)
	content := "# this is a comment\nsecret\n# another comment\n"
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("write ignore file: %v", err)
	}

	scope := Scope{Path: tmpDir}
	m, err := NewIgnoreMatcher(scope)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	key, _ := NewKey("secret")
	if !m.MatchKey(key) {
		t.Error("expected 'secret' to be ignored despite comments")
	}

	// A comment line should not be treated as a pattern
	key2, _ := NewKey("# this is a comment")
	if m.MatchKey(key2) {
		t.Error("expected comment not to be a pattern")
	}
}
