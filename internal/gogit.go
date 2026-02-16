package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const (
	DefaultBranch = "main"
	DefaultAuthor = "mem"
	DefaultEmail  = "mem@local"
)

type GitRepository struct {
	repo     *git.Repository
	worktree *git.Worktree
	rootPath string
	memPath  string
}

func NewGitRepository(scope Scope) (*GitRepository, error) {
	memPath := scope.MemPath
	rootPath := scope.Path

	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository not initialized: %s", memPath)
	}

	fs := osfs.New(memPath)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	wt := osfs.New(rootPath)

	repo, err := git.Open(storage, wt)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	return &GitRepository{
		repo:     repo,
		worktree: worktree,
		rootPath: rootPath,
		memPath:  memPath,
	}, nil
}

func InitRepository(scope Scope) error {
	memPath := scope.MemPath
	rootPath := scope.Path

	if err := os.MkdirAll(memPath, 0755); err != nil {
		return fmt.Errorf("create .mem directory: %w", err)
	}

	fs := osfs.New(memPath)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	wt := osfs.New(rootPath)

	repo, err := git.Init(storage, wt)
	if err != nil {
		return fmt.Errorf("init repository: %w", err)
	}

	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}
	cfg.Init.DefaultBranch = DefaultBranch
	if err := repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("set config: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	readmePath := filepath.Join(rootPath, ".mem-init")
	if err := os.WriteFile(readmePath, []byte("mem repository initialized\n"), 0644); err != nil {
		return fmt.Errorf("write init file: %w", err)
	}

	if _, err := worktree.Add(".mem-init"); err != nil {
		return fmt.Errorf("stage init file: %w", err)
	}

	_, err = worktree.Commit("init: initialize mem repository", &git.CommitOptions{
		Author: &object.Signature{
			Name:  DefaultAuthor,
			Email: DefaultEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}

	return nil
}

// MemoryRepository implementation

func (r *GitRepository) Get(ctx context.Context, key Key) (*Memory, error) {
	path := r.keyToPath(key)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return &Memory{
		Key:       key,
		Content:   content,
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
	}, nil
}

func (r *GitRepository) Save(ctx context.Context, mem *Memory) error {
	path := r.keyToPath(mem.Key)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, mem.Content, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	relPath, err := filepath.Rel(r.rootPath, path)
	if err != nil {
		return fmt.Errorf("get relative path: %w", err)
	}

	if _, err := r.worktree.Add(relPath); err != nil {
		return fmt.Errorf("stage file: %w", err)
	}

	return nil
}

func (r *GitRepository) Delete(ctx context.Context, key Key) error {
	path := r.keyToPath(key)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}

	relPath, err := filepath.Rel(r.rootPath, path)
	if err != nil {
		return fmt.Errorf("get relative path: %w", err)
	}

	if _, err := r.worktree.Remove(relPath); err != nil {
		return fmt.Errorf("remove file: %w", err)
	}

	return nil
}

func (r *GitRepository) List(ctx context.Context, prefix string) ([]*Memory, error) {
	var memories []*Memory

	err := filepath.Walk(r.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".mem" || info.Name() == ".mem-init" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == ".mem-init" {
			return nil
		}

		relPath, err := filepath.Rel(r.rootPath, path)
		if err != nil {
			return err
		}

		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		key, err := NewKey(relPath)
		if err != nil {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		memories = append(memories, &Memory{
			Key:       key,
			Content:   content,
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return memories, nil
}

func (r *GitRepository) Exists(ctx context.Context, key Key) (bool, error) {
	path := r.keyToPath(key)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// BranchRepository implementation

func (r *GitRepository) Current(ctx context.Context) (*Branch, error) {
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	return &Branch{
		Name: head.Name().Short(),
		Head: head.Hash().String(),
	}, nil
}

func (r *GitRepository) ListBranches(ctx context.Context) ([]*Branch, error) {
	refs, err := r.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var branches []*Branch
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, &Branch{
			Name: ref.Name().Short(),
			Head: ref.Hash().String(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(branches, func(i, j int) bool {
		return branches[i].Name < branches[j].Name
	})

	return branches, nil
}

func (r *GitRepository) Create(ctx context.Context, name string) (*Branch, error) {
	head, err := r.repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	refName := plumbing.NewBranchReferenceName(name)
	ref := plumbing.NewHashReference(refName, head.Hash())

	if err := r.repo.Storer.SetReference(ref); err != nil {
		return nil, fmt.Errorf("create branch: %w", err)
	}

	return &Branch{
		Name:      name,
		Head:      head.Hash().String(),
		CreatedAt: time.Now(),
	}, nil
}

func (r *GitRepository) Switch(ctx context.Context, name string) error {
	branchRef := plumbing.NewBranchReferenceName(name)

	if err := r.worktree.Checkout(&git.CheckoutOptions{
		Branch: branchRef,
	}); err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	return nil
}

func (r *GitRepository) DeleteBranch(ctx context.Context, name string) error {
	current, err := r.Current(ctx)
	if err != nil {
		return err
	}
	if current.Name == name {
		return fmt.Errorf("cannot delete current branch")
	}

	refName := plumbing.NewBranchReferenceName(name)
	if err := r.repo.Storer.RemoveReference(refName); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	return nil
}

// HistoryRepository implementation

func (r *GitRepository) Commit(ctx context.Context, message string) (*Commit, error) {
	hash, err := r.worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  DefaultAuthor,
			Email: DefaultEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	return r.toCommit(commit), nil
}

func (r *GitRepository) Log(ctx context.Context, limit int) ([]*Commit, error) {
	iter, err := r.repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}
	defer iter.Close()

	var commits []*Commit
	count := 0

	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && count >= limit {
			return io.EOF
		}
		commits = append(commits, r.toCommit(c))
		count++
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
	}

	return commits, nil
}

func (r *GitRepository) Diff(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return r.diffWorktreeVsHead()
	}
	return r.diffHeadVsRef(ref)
}

func (r *GitRepository) diffWorktreeVsHead() (string, error) {
	status, err := r.worktree.Status()
	if err != nil {
		return "", fmt.Errorf("get status: %w", err)
	}

	if status.IsClean() {
		return "", nil
	}

	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD: %w", err)
	}

	headCommit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("get HEAD commit: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("get HEAD tree: %w", err)
	}

	// Build a pseudo-diff from worktree status against HEAD tree
	var buf strings.Builder
	for path, s := range status {
		switch {
		case s.Staging == git.Added:
			content, readErr := os.ReadFile(filepath.Join(r.rootPath, path))
			if readErr != nil {
				continue
			}
			fmt.Fprintf(&buf, "--- /dev/null\n+++ b/%s\n", path)
			for _, line := range strings.Split(string(content), "\n") {
				fmt.Fprintf(&buf, "+%s\n", line)
			}
		case s.Staging == git.Modified:
			f, headErr := headTree.File(path)
			if headErr != nil {
				continue
			}
			oldContent, headErr := f.Contents()
			if headErr != nil {
				continue
			}
			newContent, readErr := os.ReadFile(filepath.Join(r.rootPath, path))
			if readErr != nil {
				continue
			}
			fmt.Fprintf(&buf, "--- a/%s\n+++ b/%s\n", path, path)
			for _, line := range strings.Split(oldContent, "\n") {
				fmt.Fprintf(&buf, "-%s\n", line)
			}
			for _, line := range strings.Split(string(newContent), "\n") {
				fmt.Fprintf(&buf, "+%s\n", line)
			}
		case s.Staging == git.Deleted:
			f, headErr := headTree.File(path)
			if headErr != nil {
				continue
			}
			oldContent, headErr := f.Contents()
			if headErr != nil {
				continue
			}
			fmt.Fprintf(&buf, "--- a/%s\n+++ /dev/null\n", path)
			for _, line := range strings.Split(oldContent, "\n") {
				fmt.Fprintf(&buf, "-%s\n", line)
			}
		}
	}

	return buf.String(), nil
}

func (r *GitRepository) diffHeadVsRef(ref string) (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("get HEAD: %w", err)
	}

	headCommit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("get HEAD commit: %w", err)
	}

	resolved, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return "", fmt.Errorf("resolve ref: %w", err)
	}

	targetCommit, err := r.repo.CommitObject(*resolved)
	if err != nil {
		return "", fmt.Errorf("get target commit: %w", err)
	}

	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("get HEAD tree: %w", err)
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("get target tree: %w", err)
	}

	changes, err := targetTree.Diff(headTree)
	if err != nil {
		return "", fmt.Errorf("diff trees: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("get patch: %w", err)
	}

	return patch.String(), nil
}

func (r *GitRepository) Show(ctx context.Context, ref string) (*Commit, error) {
	resolved, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, fmt.Errorf("resolve ref: %w", err)
	}

	commit, err := r.repo.CommitObject(*resolved)
	if err != nil {
		return nil, fmt.Errorf("get commit: %w", err)
	}

	return r.toCommit(commit), nil
}

func (r *GitRepository) Revert(ctx context.Context, ref string) error {
	resolved, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return fmt.Errorf("resolve ref: %w", err)
	}

	if err := r.worktree.Reset(&git.ResetOptions{
		Commit: *resolved,
		Mode:   git.HardReset,
	}); err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	return nil
}

// helpers

func (r *GitRepository) keyToPath(key Key) string {
	return filepath.Join(r.rootPath, key.String())
}

func (r *GitRepository) toCommit(c *object.Commit) *Commit {
	var parents []string
	for _, p := range c.ParentHashes {
		parents = append(parents, p.String())
	}

	return &Commit{
		Hash:      c.Hash.String(),
		Message:   strings.TrimSpace(c.Message),
		Author:    c.Author.Name,
		Timestamp: c.Author.When,
		Parents:   parents,
	}
}
