package internal

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const IgnoreFilename = ".memignore"

type IgnoreMatcher struct {
	patterns []gitignore.Pattern
}

func NewIgnoreMatcher(scope Scope) (*IgnoreMatcher, error) {
	m := &IgnoreMatcher{}

	ignorePath := filepath.Join(scope.Path, IgnoreFilename)
	patterns, err := parseIgnoreFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	m.patterns = patterns
	return m, nil
}

// MatchKey returns true if the key should be ignored (blocked from add/edit)
func (m *IgnoreMatcher) MatchKey(key Key) bool {
	keyStr := key.String()
	parts := strings.Split(keyStr, "/")

	for _, p := range m.patterns {
		if p.Match(parts, false) == gitignore.Exclude {
			return true
		}
	}
	return false
}

func parseIgnoreFile(path string) ([]gitignore.Pattern, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []gitignore.Pattern
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern := gitignore.ParsePattern(line, nil)
		patterns = append(patterns, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}
