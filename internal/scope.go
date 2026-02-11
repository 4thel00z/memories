package internal

import (
	"os"
	"path/filepath"
)

type ScopeType string

const (
	ScopeGlobal  ScopeType = "global"
	ScopeProject ScopeType = "project"
)

type Scope struct {
	Type    ScopeType
	Path    string // working directory root
	MemPath string // .mem directory path
}

func (s Scope) VectorPath() string {
	return filepath.Join(s.MemPath, "vectors")
}

func (s Scope) ConfigPath() string {
	return filepath.Join(s.MemPath, "config.yaml")
}

type ScopeResolver struct {
	homeDir string
}

func NewScopeResolver() *ScopeResolver {
	home, _ := os.UserHomeDir()
	return &ScopeResolver{homeDir: home}
}

func (r *ScopeResolver) Global() Scope {
	memPath := filepath.Join(r.homeDir, ".mem")
	return Scope{
		Type:    ScopeGlobal,
		Path:    r.homeDir,
		MemPath: memPath,
	}
}

func (r *ScopeResolver) Project() (Scope, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return Scope{}, false
	}
	return r.findProjectScope(cwd)
}

func (r *ScopeResolver) findProjectScope(dir string) (Scope, bool) {
	for {
		memPath := filepath.Join(dir, ".mem")
		info, err := os.Stat(memPath)
		if err == nil && info.IsDir() {
			return Scope{Type: ScopeProject, Path: dir, MemPath: memPath}, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Scope{}, false
		}
		dir = parent
	}
}

func (r *ScopeResolver) Resolve(explicit string) Scope {
	if explicit == "global" {
		return r.Global()
	}
	if scope, ok := r.Project(); ok {
		return scope
	}
	return r.Global()
}

func (r *ScopeResolver) Cascade() []Scope {
	scopes := []Scope{}
	if scope, ok := r.Project(); ok {
		scopes = append(scopes, scope)
	}
	scopes = append(scopes, r.Global())
	return scopes
}

func (r *ScopeResolver) EnvVars(scope Scope, branch, version string) map[string]string {
	memBin, _ := os.Executable()
	return map[string]string{
		"MEM_SCOPE":      string(scope.Type),
		"MEM_SCOPE_PATH": scope.MemPath,
		"MEM_ROOT":       scope.Path,
		"MEM_BRANCH":     branch,
		"MEM_CONFIG":     scope.ConfigPath(),
		"MEM_VERSION":    version,
		"MEM_BIN":        memBin,
	}
}
