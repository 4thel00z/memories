package internal

import (
	"context"
	"time"
)

type Branch struct {
	Name      string
	Head      string // commit hash
	CreatedAt time.Time
}

type Commit struct {
	Hash      string
	Message   string
	Author    string
	Timestamp time.Time
	Parents   []string
}

type BranchRepository interface {
	Current(ctx context.Context) (*Branch, error)
	List(ctx context.Context) ([]*Branch, error)
	Create(ctx context.Context, name string) (*Branch, error)
	Switch(ctx context.Context, name string) error
	Delete(ctx context.Context, name string) error
}

type HistoryRepository interface {
	Commit(ctx context.Context, message string) (*Commit, error)
	Log(ctx context.Context, limit int) ([]*Commit, error)
	Diff(ctx context.Context, ref string) (string, error)
	Show(ctx context.Context, ref string) (*Commit, error)
	Revert(ctx context.Context, ref string) error
}
