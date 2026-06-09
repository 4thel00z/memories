package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// fileLock is a best-effort cross-process advisory lock backed by an
// exclusively-created lock file. It serializes mutating git operations (Save,
// Delete, Commit) against the same .mem repository so concurrent `mem`
// invocations cannot race the go-git index, which has no index.lock discipline.
type fileLock struct {
	path string
}

// acquire blocks until the lock is obtained or the timeout elapses. A lock file
// older than staleAfter is considered abandoned (e.g. a crashed process) and is
// reclaimed.
func acquireLock(dir string, timeout time.Duration) (*fileLock, error) {
	path := filepath.Join(dir, "mem.lock")
	const staleAfter = 30 * time.Second
	deadline := time.Now().Add(timeout)

	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			_ = f.Close()
			return &fileLock{path: path}, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire lock: %w", err)
		}

		// Lock held: reclaim it if it is stale, otherwise wait.
		if info, statErr := os.Stat(path); statErr == nil {
			if time.Since(info.ModTime()) > staleAfter {
				_ = os.Remove(path)
				continue
			}
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for repository lock at %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (l *fileLock) release() {
	if l != nil {
		_ = os.Remove(l.path)
	}
}
