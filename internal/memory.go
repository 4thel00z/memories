package internal

import (
	"context"
	"errors"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound        = errors.New("memory not found")
	ErrAlreadyExists   = errors.New("memory already exists")
	ErrInvalidKey      = errors.New("invalid key")
	ErrNoIndex         = errors.New("no vector index available")
	ErrBranchExists    = errors.New("branch already exists")
	ErrNothingToCommit = errors.New("nothing to commit")
)

var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

type Key string

func NewKey(s string) (Key, error) {
	if s == "" {
		return "", ErrInvalidKey
	}
	if !keyPattern.MatchString(s) {
		return "", ErrInvalidKey
	}
	// Reject path-traversal: the key must already be a clean relative slash path.
	// path.Clean collapses "..", ".", "//" and trailing slashes, so any key that
	// changes under Clean (e.g. "a/../../../etc/passwd") is rejected before it can
	// escape the .mem directory when joined.
	if path.Clean(s) != s {
		return "", ErrInvalidKey
	}
	// Every segment must start alphanumeric, and the ".tmp" suffix is reserved
	// for the store's atomic writes — otherwise such keys would be shadowed by
	// the metadata filters in status/diff/commit/watch and silently lost.
	for _, seg := range strings.Split(s, "/") {
		if seg[0] == '.' {
			return "", ErrInvalidKey
		}
	}
	if strings.HasSuffix(s, ".tmp") {
		return "", ErrInvalidKey
	}
	return Key(s), nil
}

func (k Key) String() string {
	return string(k)
}

type Metadata struct {
	Tags     []string
	MimeType string
}

type Memory struct {
	Key       Key
	Content   []byte
	Metadata  Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewMemory(key Key, content []byte) *Memory {
	now := time.Now().UTC()
	return &Memory{
		Key:       key,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

type MemoryRepository interface {
	Get(ctx context.Context, key Key) (*Memory, error)
	Save(ctx context.Context, mem *Memory) error
	Delete(ctx context.Context, key Key) error
	List(ctx context.Context, prefix string) ([]*Memory, error)
	Exists(ctx context.Context, key Key) (bool, error)
}

// AtomicRepository persists a change and commits it while holding the
// repository lock once, so a concurrent process cannot interleave between the
// write and the commit. A change that leaves the tree identical yields a nil
// *Commit and nil error.
type AtomicRepository interface {
	SaveAndCommit(ctx context.Context, mem *Memory, message string) (*Commit, error)
	DeleteAndCommit(ctx context.Context, key Key, message string) (*Commit, error)
}

// AllCommitter stages every pending memory change — including edits made
// outside of mem — and commits them under one lock. A clean tree yields a nil
// *Commit and nil error.
type AllCommitter interface {
	CommitAll(ctx context.Context, message string) (*Commit, error)
}
