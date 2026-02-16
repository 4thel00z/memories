package internal

import (
	"context"
	"errors"
	"regexp"
	"time"
)

var (
	ErrNotFound      = errors.New("memory not found")
	ErrAlreadyExists = errors.New("memory already exists")
	ErrInvalidKey    = errors.New("invalid key")
	ErrNoIndex       = errors.New("no vector index available")
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
