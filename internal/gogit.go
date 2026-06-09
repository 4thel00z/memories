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
	"github.com/sergi/go-diff/diffmatchpatch"
)

const (
	DefaultBranch = "main"
	DefaultAuthor = "mem"
	DefaultEmail  = "mem@local"
)

var _ MemoryRepository = (*GitRepository)(nil)
var _ BranchRepository = (*GitRepository)(nil)
var _ HistoryRepository = (*GitRepository)(nil)

type GitRepository struct {
	repo     *git.Repository
	worktree *git.Worktree
	memPath  string
}

func NewGitRepository(scope Scope) (*GitRepository, error) {
	memPath := scope.MemPath

	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository not initialized: %s", memPath)
	}

	dotgit := filepath.Join(memPath, ".git")
	fs := osfs.New(dotgit)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	wt := osfs.New(memPath)

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
		memPath:  memPath,
	}, nil
}

func InitRepository(scope Scope) error {
	memPath := scope.MemPath

	if err := os.MkdirAll(memPath, 0755); err != nil {
		return fmt.Errorf("create .mem directory: %w", err)
	}

	dotgit := filepath.Join(memPath, ".git")
	fs := osfs.New(dotgit)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	wt := osfs.New(memPath)

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

	readmePath := filepath.Join(memPath, ".mem-init")
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
	path, err := r.keyToPath(key)
	if err != nil {
		return nil, err
	}

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
		CreatedAt: r.getFirstCommitTime(key, info.ModTime()),
		UpdatedAt: info.ModTime(),
	}, nil
}

func (r *GitRepository) Save(ctx context.Context, mem *Memory) error {
	path, err := r.keyToPath(mem.Key)
	if err != nil {
		return err
	}

	lock, err := r.lock()
	if err != nil {
		return err
	}
	defer lock.release()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write atomically: write to a temp file in the same directory, then rename
	// over the target so a crash or concurrent reader never sees a partial file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, mem.Content, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write file: %w", err)
	}

	relPath, err := filepath.Rel(r.memPath, path)
	if err != nil {
		return fmt.Errorf("get relative path: %w", err)
	}

	if _, err := r.worktree.Add(relPath); err != nil {
		return fmt.Errorf("stage file: %w", err)
	}

	return nil
}

func (r *GitRepository) Delete(ctx context.Context, key Key) error {
	path, err := r.keyToPath(key)
	if err != nil {
		return err
	}

	lock, err := r.lock()
	if err != nil {
		return err
	}
	defer lock.release()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrNotFound
	}

	relPath, err := filepath.Rel(r.memPath, path)
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

	// Compute first-commit times for every tracked path in a single history
	// walk, instead of one full git-log traversal per file.
	createdTimes := r.firstCommitTimes()

	err := filepath.Walk(r.memPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vectors" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == ".mem-init" || info.Name() == "config.yaml" {
			return nil
		}

		relPath, err := filepath.Rel(r.memPath, path)
		if err != nil {
			return err
		}

		// Match prefix on path-component boundaries so "ctx" matches "ctx/foo"
		// but not "ctxfoo" or "ctx-other".
		if prefix != "" && relPath != prefix && !strings.HasPrefix(relPath, prefix+"/") {
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

		created := info.ModTime()
		if t, ok := createdTimes[relPath]; ok {
			created = t
		}

		memories = append(memories, &Memory{
			Key:       key,
			Content:   content,
			CreatedAt: created,
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
	path, err := r.keyToPath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
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
	lock, err := r.lock()
	if err != nil {
		return nil, err
	}
	defer lock.release()

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

	var buf strings.Builder
	dmp := diffmatchpatch.New()

	for path, s := range status {
		if s.Staging == git.Unmodified && s.Worktree == git.Unmodified {
			continue
		}

		// Compare HEAD content against the current on-disk content directly. This
		// covers both staged and unstaged changes (added/modified/deleted) without
		// depending on which side of the index the change landed on.
		var oldContent string
		if f, headErr := headTree.File(path); headErr == nil {
			if c, err := f.Contents(); err == nil {
				oldContent = c
			}
		}

		var newContent string
		hasNew := false
		if data, readErr := os.ReadFile(filepath.Join(r.memPath, path)); readErr == nil {
			newContent = string(data)
			hasNew = true
		}

		if oldContent == newContent {
			continue
		}

		switch {
		case oldContent == "" && hasNew:
			fmt.Fprintf(&buf, "--- /dev/null\n+++ b/%s\n", path)
			writeUnifiedHunks(&buf, "", newContent, dmp)
		case !hasNew && oldContent != "":
			fmt.Fprintf(&buf, "--- a/%s\n+++ /dev/null\n", path)
			writeUnifiedHunks(&buf, oldContent, "", dmp)
		default:
			fmt.Fprintf(&buf, "--- a/%s\n+++ b/%s\n", path, path)
			writeUnifiedHunks(&buf, oldContent, newContent, dmp)
		}
	}

	return buf.String(), nil
}

func writeUnifiedHunks(buf *strings.Builder, oldText, newText string, dmp *diffmatchpatch.DiffMatchPatch) {
	// Use line-level diffing for proper unified diff output
	a, b, lineArray := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	diffs = dmp.DiffCleanupSemantic(diffs)

	oldLine := 1
	newLine := 1
	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")
		// Remove trailing empty string from split
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		count := len(lines)

		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			for _, line := range lines {
				fmt.Fprintf(buf, " %s\n", line)
			}
			oldLine += count
			newLine += count
		case diffmatchpatch.DiffDelete:
			for _, line := range lines {
				fmt.Fprintf(buf, "-%s\n", line)
			}
			oldLine += count
		case diffmatchpatch.DiffInsert:
			for _, line := range lines {
				fmt.Fprintf(buf, "+%s\n", line)
			}
			newLine += count
		}
	}
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

	// Reset is a hard reset that discards uncommitted work. Refuse when the
	// worktree is dirty so an unsaved/uncommitted memory edit is never silently
	// destroyed; the caller must commit or stash first.
	status, err := r.worktree.Status()
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if !status.IsClean() {
		return fmt.Errorf("worktree has uncommitted changes; commit or discard them before revert")
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

// lock acquires the per-repository write lock. Mutating operations (Save,
// Delete, Commit) hold it so concurrent `mem` processes cannot corrupt the
// go-git index, which lacks git's native index.lock.
func (r *GitRepository) lock() (*fileLock, error) {
	return acquireLock(filepath.Join(r.memPath, ".git"), 10*time.Second)
}

// firstCommitTimes returns the earliest authoring time each tracked path was
// seen, computed in a single pass over the commit history. This replaces an
// O(N·history) per-file log walk in List with one O(history) traversal.
func (r *GitRepository) firstCommitTimes() map[string]time.Time {
	times := make(map[string]time.Time)

	iter, err := r.repo.Log(&git.LogOptions{})
	if err != nil {
		return times
	}
	defer iter.Close()

	_ = iter.ForEach(func(c *object.Commit) error {
		stats, err := c.Stats()
		if err != nil {
			return nil
		}
		for _, s := range stats {
			if t, ok := times[s.Name]; !ok || c.Author.When.Before(t) {
				times[s.Name] = c.Author.When
			}
		}
		return nil
	})

	return times
}

func (r *GitRepository) getFirstCommitTime(key Key, fallback time.Time) time.Time {
	relPath := key.String()

	iter, err := r.repo.Log(&git.LogOptions{
		FileName: &relPath,
	})
	if err != nil {
		return fallback
	}
	defer iter.Close()

	var earliest time.Time
	err = iter.ForEach(func(c *object.Commit) error {
		if earliest.IsZero() || c.Author.When.Before(earliest) {
			earliest = c.Author.When
		}
		return nil
	})
	if err != nil || earliest.IsZero() {
		return fallback
	}

	return earliest
}

// keyToPath resolves a key to its on-disk path and verifies the result stays
// within the .mem directory. NewKey already rejects traversal segments; this is
// a defense-in-depth containment check so a crafted key can never escape the
// store even if it reaches this layer by another path.
func (r *GitRepository) keyToPath(key Key) (string, error) {
	path := filepath.Join(r.memPath, key.String())
	rel, err := filepath.Rel(r.memPath, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: escapes store root", ErrInvalidKey)
	}
	return path, nil
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
