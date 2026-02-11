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
	basePath string
}

func NewIgnoreMatcher(basePath string) (*IgnoreMatcher, error) {
	m := &IgnoreMatcher{
		basePath: basePath,
	}

	ignorePath := filepath.Join(basePath, IgnoreFilename)
	patterns, err := parseIgnoreFile(ignorePath, basePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	m.patterns = patterns
	return m, nil
}

func (m *IgnoreMatcher) Match(path string) bool {
	relPath, err := filepath.Rel(m.basePath, path)
	if err != nil {
		return false
	}

	pathParts := strings.Split(relPath, string(filepath.Separator))

	for _, p := range m.patterns {
		if p.Match(pathParts, false) == gitignore.Exclude {
			return true
		}
	}
	return false
}

func (m *IgnoreMatcher) MatchDir(path string) bool {
	relPath, err := filepath.Rel(m.basePath, path)
	if err != nil {
		return false
	}

	pathParts := strings.Split(relPath, string(filepath.Separator))

	for _, p := range m.patterns {
		if p.Match(pathParts, true) == gitignore.Exclude {
			return true
		}
	}
	return false
}

func parseIgnoreFile(path, basePath string) ([]gitignore.Pattern, error) {
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
