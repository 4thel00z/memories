package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScopeVectorPath(t *testing.T) {
	scope := Scope{MemPath: "/home/user/.mem"}
	expected := "/home/user/.mem/vectors"
	if scope.VectorPath() != expected {
		t.Errorf("expected %q, got %q", expected, scope.VectorPath())
	}
}

func TestScopeConfigPath(t *testing.T) {
	scope := Scope{MemPath: "/home/user/.mem"}
	expected := "/home/user/.mem/config.yaml"
	if scope.ConfigPath() != expected {
		t.Errorf("expected %q, got %q", expected, scope.ConfigPath())
	}
}

func TestScopeResolverGlobal(t *testing.T) {
	resolver := NewScopeResolver()
	scope := resolver.Global()

	if scope.Type != ScopeGlobal {
		t.Errorf("expected ScopeGlobal, got %q", scope.Type)
	}

	home, _ := os.UserHomeDir()
	expectedMemPath := filepath.Join(home, ".mem")
	if scope.MemPath != expectedMemPath {
		t.Errorf("expected MemPath %q, got %q", expectedMemPath, scope.MemPath)
	}
}

func TestScopeResolverProjectNotFound(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(tmp)

	resolver := NewScopeResolver()
	_, found := resolver.Project()
	if found {
		t.Error("expected Project() to return false when no .mem exists")
	}
}

func TestScopeResolverProjectFound(t *testing.T) {
	tmp := t.TempDir()
	memDir := filepath.Join(tmp, ".mem")
	if err := os.Mkdir(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(tmp)

	resolver := NewScopeResolver()
	scope, found := resolver.Project()
	if !found {
		t.Fatal("expected Project() to return true")
	}

	if scope.Type != ScopeProject {
		t.Errorf("expected ScopeProject, got %q", scope.Type)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedMemDir, _ := filepath.EvalSymlinks(memDir)
	actualMemDir, _ := filepath.EvalSymlinks(scope.MemPath)
	if actualMemDir != expectedMemDir {
		t.Errorf("expected MemPath %q, got %q", expectedMemDir, actualMemDir)
	}
}

func TestScopeResolverProjectInParent(t *testing.T) {
	tmp := t.TempDir()
	memDir := filepath.Join(tmp, ".mem")
	if err := os.Mkdir(memDir, 0755); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmp, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(subDir)

	resolver := NewScopeResolver()
	scope, found := resolver.Project()
	if !found {
		t.Fatal("expected Project() to find .mem in parent")
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	expectedPath, _ := filepath.EvalSymlinks(tmp)
	actualPath, _ := filepath.EvalSymlinks(scope.Path)
	if actualPath != expectedPath {
		t.Errorf("expected Path %q, got %q", expectedPath, actualPath)
	}
}

func TestScopeResolverResolveExplicitGlobal(t *testing.T) {
	resolver := NewScopeResolver()
	scope := resolver.Resolve("global")
	if scope.Type != ScopeGlobal {
		t.Errorf("expected ScopeGlobal, got %q", scope.Type)
	}
}

func TestScopeResolverResolveFallbackToGlobal(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(tmp)

	resolver := NewScopeResolver()
	scope := resolver.Resolve("")
	if scope.Type != ScopeGlobal {
		t.Errorf("expected fallback to ScopeGlobal, got %q", scope.Type)
	}
}

func TestScopeResolverCascade(t *testing.T) {
	tmp := t.TempDir()
	memDir := filepath.Join(tmp, ".mem")
	if err := os.Mkdir(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	_ = os.Chdir(tmp)

	resolver := NewScopeResolver()
	scopes := resolver.Cascade()

	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
	if scopes[0].Type != ScopeProject {
		t.Errorf("expected first scope to be ScopeProject, got %q", scopes[0].Type)
	}
	if scopes[1].Type != ScopeGlobal {
		t.Errorf("expected second scope to be ScopeGlobal, got %q", scopes[1].Type)
	}
}

func TestScopeResolverEnvVars(t *testing.T) {
	resolver := NewScopeResolver()
	scope := Scope{
		Type:    ScopeProject,
		Path:    "/project",
		MemPath: "/project/.mem",
	}

	env := resolver.EnvVars(scope, "main", "1.0.0")

	if env["MEM_SCOPE"] != "project" {
		t.Errorf("expected MEM_SCOPE=project, got %q", env["MEM_SCOPE"])
	}
	if env["MEM_SCOPE_PATH"] != "/project/.mem" {
		t.Errorf("expected MEM_SCOPE_PATH=/project/.mem, got %q", env["MEM_SCOPE_PATH"])
	}
	if env["MEM_ROOT"] != "/project" {
		t.Errorf("expected MEM_ROOT=/project, got %q", env["MEM_ROOT"])
	}
	if env["MEM_BRANCH"] != "main" {
		t.Errorf("expected MEM_BRANCH=main, got %q", env["MEM_BRANCH"])
	}
	if env["MEM_VERSION"] != "1.0.0" {
		t.Errorf("expected MEM_VERSION=1.0.0, got %q", env["MEM_VERSION"])
	}
	if env["MEM_CONFIG"] != "/project/.mem/config.yaml" {
		t.Errorf("expected MEM_CONFIG=/project/.mem/config.yaml, got %q", env["MEM_CONFIG"])
	}
}
