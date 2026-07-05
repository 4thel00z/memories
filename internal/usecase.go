package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Use case input/output DTOs

type SetMemoryInput struct {
	Key     string
	Content string
	Scope   string
	// CommitMessage, when set, commits the write in the same locked operation
	// so a concurrent process cannot interleave between write and commit.
	CommitMessage string
}

type GetMemoryInput struct {
	Key   string
	Scope string
}

type GetMemoryOutput struct {
	Key       string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DeleteMemoryInput struct {
	Key   string
	Scope string
	// CommitMessage, when set, commits the removal in the same locked
	// operation. See SetMemoryInput.CommitMessage.
	CommitMessage string
}

type ListMemoriesInput struct {
	Prefix string
	Scope  string
}

type ListMemoriesOutput struct {
	Memories []GetMemoryOutput
}

type CommitInput struct {
	Message string
	Scope   string
}

type CommitOutput struct {
	Hash      string
	Message   string
	Timestamp time.Time
}

type LogInput struct {
	Limit int
	Scope string
}

type LogOutput struct {
	Commits []CommitOutput
}

type DiffInput struct {
	Ref   string
	Scope string
}

type DiffOutput struct {
	Diff string
}

type RevertInput struct {
	Ref   string
	Scope string
}

type SearchInput struct {
	Query string
	Limit int
	Scope string
}

type SearchOutput struct {
	Results []SearchResultOutput
}

type SearchResultOutput struct {
	Key   string
	Score float32
}

type RebuildIndexInput struct {
	Scope    string
	NumTrees int
	// OnProgress, if set, is called once after the memory list is known
	// (done=0, total=N) and after each memory is embedded (done=i+1, total=N).
	OnProgress func(done, total int)
}

type SummarizeInput struct {
	Prefix string
	Scope  string
}

type SummarizeOutput struct {
	Title     string
	Overview  string
	KeyPoints []string
	Tags      []string
}

type AutoTagInput struct {
	Key   string
	Scope string
}

type AutoTagOutput struct {
	Tags       []string
	Category   string
	Confidence float32
}

type BranchInput struct {
	Name  string
	Scope string
}

type BranchOutput struct {
	Name      string
	Head      string
	CreatedAt time.Time
}

type BranchListOutput struct {
	Branches []BranchOutput
}

type ProviderInput struct {
	Name   string
	Scope  string
	Config ProviderConfig
}

type AddMemoryInput struct {
	Key     string
	Content string
	Scope   string
	Message string
}

// UseCases is the holder struct that aggregates all use cases.
type UseCases struct {
	SetMemory      *SetMemoryUseCase
	GetMemory      *GetMemoryUseCase
	DeleteMemory   *DeleteMemoryUseCase
	ListMemories   *ListMemoriesUseCase
	AddMemory      *AddMemoryUseCase
	Commit         *CommitUseCase
	Log            *LogUseCase
	Diff           *DiffUseCase
	Revert         *RevertUseCase
	KeywordSearch  *KeywordSearchUseCase
	SemanticSearch *SemanticSearchUseCase
	RebuildIndex   *RebuildIndexUseCase
	IndexStatus    *IndexStatusUseCase
	Summarize      *SummarizeUseCase
	AutoTag        *AutoTagUseCase
	BranchCurrent  *BranchCurrentUseCase
	BranchList     *BranchListUseCase
	BranchCreate   *BranchCreateUseCase
	BranchSwitch   *BranchSwitchUseCase
	BranchDelete   *BranchDeleteUseCase
	ProviderList   *ProviderListUseCase
	ProviderAdd    *ProviderAddUseCase
	ProviderRemove *ProviderRemoveUseCase
	ProviderSetDef *ProviderSetDefaultUseCase
	ProviderTest   *ProviderTestUseCase
	InstallHook    *InstallHookUseCase
	UninstallHook  *UninstallHookUseCase
	RunHook        *RunHookUseCase
}

// --- SetMemoryUseCase ---

type SetMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	indexFor func(Scope) (VectorIndex, error)
	embedder Embedder
	ignore   func(Scope) (*IgnoreMatcher, error)
}

func NewSetMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	indexFor func(Scope) (VectorIndex, error),
	embedder Embedder,
	ignore func(Scope) (*IgnoreMatcher, error),
) *SetMemoryUseCase {
	return &SetMemoryUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
		ignore:   ignore,
	}
}

func (uc *SetMemoryUseCase) Execute(ctx context.Context, input SetMemoryInput) error {
	key, err := NewKey(input.Key)
	if err != nil {
		return err
	}

	scope := uc.resolver.Resolve(input.Scope)

	if uc.ignore != nil {
		matcher, err := uc.ignore(scope)
		if err != nil {
			return fmt.Errorf("check .memignore: %w", err)
		}
		if matcher.MatchKey(key) {
			return fmt.Errorf("key %q is blocked by .memignore", input.Key)
		}
	}

	repo, err := uc.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	mem := &Memory{
		Key:       key,
		Content:   []byte(input.Content),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	message := input.CommitMessage
	if message == "" {
		message = fmt.Sprintf("set: %s", input.Key)
	}

	commit, err := commitWrite(ctx, repo, mem, message)
	if err != nil {
		return fmt.Errorf("save memory: %w", err)
	}
	if commit == nil {
		// Unchanged content: nothing to embed or re-index either.
		return nil
	}

	updateIndex(ctx, uc.indexFor, uc.embedder, scope, key, input.Content)
	return nil
}

// updateIndex embeds content and persists it into the scope's vector index.
// Failures are logged rather than returned: the memory write has already
// succeeded, and search staleness is recoverable via `mem index rebuild`.
func updateIndex(ctx context.Context, indexFor func(Scope) (VectorIndex, error), embedder Embedder, scope Scope, key Key, content string) {
	if embedder == nil || indexFor == nil {
		return
	}

	index, err := indexFor(scope)
	if err != nil {
		slog.Warn("skipping index update: failed to get index", "error", err)
		return
	}
	defer closeIndex(index)

	vec, err := embedder.Embed(ctx, content)
	if err != nil {
		slog.Warn("skipping index update: embedding failed", "error", err)
		return
	}

	persistIndexUpdate(ctx, index, key, NewEmbedding(vec, "local"))
}

// closeIndex releases an index's resources (e.g. mmap handles) once a use case
// is done with it; failures are logged since there is nothing to recover.
func closeIndex(index VectorIndex) {
	if err := index.Close(); err != nil {
		slog.Warn("index close failed", "error", err)
	}
}

// commitWrite persists a memory and commits it under one lock. It returns the
// commit, or nil when the write changed nothing.
func commitWrite(ctx context.Context, repo MemoryRepository, mem *Memory, message string) (*Commit, error) {
	atomic, ok := repo.(AtomicRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support atomic save+commit")
	}
	return atomic.SaveAndCommit(ctx, mem, message)
}

// persistIndexUpdate adds an embedding to the index and then rebuilds and saves
// it, so incremental writes are durable. Without the Build+Save, an Add only
// mutates an in-memory index that is discarded when the use case returns,
// silently desyncing the vector index from the store. Failures are logged
// rather than fatal: the memory itself is already saved.
func persistIndexUpdate(ctx context.Context, index VectorIndex, key Key, emb Embedding) {
	if err := index.Add(ctx, key, emb); err != nil {
		slog.Warn("index add failed", "key", key.String(), "error", err)
		return
	}
	if err := index.Build(ctx, DefaultIndexTrees); err != nil {
		slog.Warn("index build failed", "error", err)
		return
	}
	if err := index.Save(ctx); err != nil {
		slog.Warn("index save failed", "error", err)
	}
}

// persistIndexRemoval removes a key from the index and rebuilds and saves it so
// the deletion is durable.
func persistIndexRemoval(ctx context.Context, index VectorIndex, key Key) {
	if err := index.Remove(ctx, key); err != nil {
		slog.Warn("index remove failed", "key", key.String(), "error", err)
		return
	}
	if err := index.Build(ctx, DefaultIndexTrees); err != nil {
		slog.Warn("index build failed", "error", err)
		return
	}
	if err := index.Save(ctx); err != nil {
		slog.Warn("index save failed", "error", err)
	}
}

// --- GetMemoryUseCase ---

type GetMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
}

func NewGetMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
) *GetMemoryUseCase {
	return &GetMemoryUseCase{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (uc *GetMemoryUseCase) Execute(ctx context.Context, input GetMemoryInput) (*GetMemoryOutput, error) {
	key, err := NewKey(input.Key)
	if err != nil {
		return nil, err
	}

	scopes := uc.resolver.Cascade()
	if input.Scope != "" {
		scopes = []Scope{uc.resolver.Resolve(input.Scope)}
	}

	for _, scope := range scopes {
		repo, err := uc.repoFor(scope)
		if err != nil {
			continue
		}

		mem, err := repo.Get(ctx, key)
		if err != nil {
			continue
		}

		return &GetMemoryOutput{
			Key:       mem.Key.String(),
			Content:   string(mem.Content),
			CreatedAt: mem.CreatedAt,
			UpdatedAt: mem.UpdatedAt,
		}, nil
	}

	return nil, ErrNotFound
}

// --- DeleteMemoryUseCase ---

type DeleteMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	indexFor func(Scope) (VectorIndex, error)
}

func NewDeleteMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	indexFor func(Scope) (VectorIndex, error),
) *DeleteMemoryUseCase {
	return &DeleteMemoryUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
	}
}

func (uc *DeleteMemoryUseCase) Execute(ctx context.Context, input DeleteMemoryInput) error {
	key, err := NewKey(input.Key)
	if err != nil {
		return err
	}

	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	message := input.CommitMessage
	if message == "" {
		message = fmt.Sprintf("del: %s", input.Key)
	}

	atomic, ok := repo.(AtomicRepository)
	if !ok {
		return fmt.Errorf("repository does not support atomic delete+commit")
	}
	if _, err := atomic.DeleteAndCommit(ctx, key, message); err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	if uc.indexFor != nil {
		if index, err := uc.indexFor(scope); err == nil {
			persistIndexRemoval(ctx, index, key)
			closeIndex(index)
		}
	}

	return nil
}

// --- ListMemoriesUseCase ---

type ListMemoriesUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
}

func NewListMemoriesUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
) *ListMemoriesUseCase {
	return &ListMemoriesUseCase{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (uc *ListMemoriesUseCase) Execute(ctx context.Context, input ListMemoriesInput) (*ListMemoriesOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	memories, err := repo.List(ctx, input.Prefix)
	if err != nil {
		return nil, err
	}

	output := &ListMemoriesOutput{
		Memories: make([]GetMemoryOutput, len(memories)),
	}

	for i, mem := range memories {
		output.Memories[i] = GetMemoryOutput{
			Key:       mem.Key.String(),
			Content:   string(mem.Content),
			CreatedAt: mem.CreatedAt,
			UpdatedAt: mem.UpdatedAt,
		}
	}

	return output, nil
}

// --- AddMemoryUseCase ---

type AddMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	indexFor func(Scope) (VectorIndex, error)
	embedder Embedder
	ignore   func(Scope) (*IgnoreMatcher, error)
}

func NewAddMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	indexFor func(Scope) (VectorIndex, error),
	embedder Embedder,
	ignore func(Scope) (*IgnoreMatcher, error),
) *AddMemoryUseCase {
	return &AddMemoryUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
		ignore:   ignore,
	}
}

func (uc *AddMemoryUseCase) Execute(ctx context.Context, input AddMemoryInput) (*CommitOutput, error) {
	key, err := NewKey(input.Key)
	if err != nil {
		return nil, err
	}

	scope := uc.resolver.Resolve(input.Scope)

	if uc.ignore != nil {
		matcher, err := uc.ignore(scope)
		if err != nil {
			return nil, fmt.Errorf("check .memignore: %w", err)
		}
		if matcher.MatchKey(key) {
			return nil, fmt.Errorf("key %q is blocked by .memignore", input.Key)
		}
	}

	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	existing, _ := repo.Get(ctx, key)
	var newContent []byte
	if existing != nil {
		// Copy rather than append onto existing.Content's backing array, so we
		// never mutate a slice the repository may share or cache.
		newContent = make([]byte, 0, len(existing.Content)+1+len(input.Content))
		newContent = append(newContent, existing.Content...)
		newContent = append(newContent, '\n')
		newContent = append(newContent, input.Content...)
	} else {
		newContent = []byte(input.Content)
	}

	mem := &Memory{
		Key:       key,
		Content:   newContent,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	message := input.Message
	if message == "" {
		message = fmt.Sprintf("add: append to %s", input.Key)
	}

	commit, err := commitWrite(ctx, repo, mem, message)
	if err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}
	if commit == nil {
		return &CommitOutput{Message: message}, nil
	}

	updateIndex(ctx, uc.indexFor, uc.embedder, scope, key, string(newContent))

	return &CommitOutput{
		Hash:      commit.Hash,
		Message:   commit.Message,
		Timestamp: commit.Timestamp,
	}, nil
}

// --- CommitUseCase ---

type CommitUseCase struct {
	resolver *ScopeResolver
	histFor  func(Scope) (HistoryRepository, error)
}

func NewCommitUseCase(
	resolver *ScopeResolver,
	histFor func(Scope) (HistoryRepository, error),
) *CommitUseCase {
	return &CommitUseCase{
		resolver: resolver,
		histFor:  histFor,
	}
}

func (uc *CommitUseCase) Execute(ctx context.Context, input CommitInput) (*CommitOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	hist, err := uc.histFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	// Prefer CommitAll so hand edits to store files — which nothing ever
	// stages — are committed too, not just changes staged by mem itself.
	var commit *Commit
	if all, ok := hist.(AllCommitter); ok {
		commit, err = all.CommitAll(ctx, input.Message)
	} else {
		commit, err = hist.Commit(ctx, input.Message)
	}
	if err != nil {
		return nil, err
	}
	if commit == nil {
		return nil, ErrNothingToCommit
	}

	return &CommitOutput{
		Hash:      commit.Hash,
		Message:   commit.Message,
		Timestamp: commit.Timestamp,
	}, nil
}

// --- LogUseCase ---

type LogUseCase struct {
	resolver *ScopeResolver
	histFor  func(Scope) (HistoryRepository, error)
}

func NewLogUseCase(
	resolver *ScopeResolver,
	histFor func(Scope) (HistoryRepository, error),
) *LogUseCase {
	return &LogUseCase{
		resolver: resolver,
		histFor:  histFor,
	}
}

func (uc *LogUseCase) Execute(ctx context.Context, input LogInput) (*LogOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	hist, err := uc.histFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	commits, err := hist.Log(ctx, input.Limit)
	if err != nil {
		return nil, err
	}

	output := &LogOutput{
		Commits: make([]CommitOutput, len(commits)),
	}

	for i, c := range commits {
		output.Commits[i] = CommitOutput{
			Hash:      c.Hash,
			Message:   c.Message,
			Timestamp: c.Timestamp,
		}
	}

	return output, nil
}

// --- DiffUseCase ---

type DiffUseCase struct {
	resolver *ScopeResolver
	histFor  func(Scope) (HistoryRepository, error)
}

func NewDiffUseCase(
	resolver *ScopeResolver,
	histFor func(Scope) (HistoryRepository, error),
) *DiffUseCase {
	return &DiffUseCase{
		resolver: resolver,
		histFor:  histFor,
	}
}

func (uc *DiffUseCase) Execute(ctx context.Context, input DiffInput) (*DiffOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	hist, err := uc.histFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	diff, err := hist.Diff(ctx, input.Ref)
	if err != nil {
		return nil, err
	}

	return &DiffOutput{Diff: diff}, nil
}

// Clean reports whether the scope has pending memory changes, without the
// cost of rendering a full diff when the repository offers a cheap check.
func (uc *DiffUseCase) Clean(ctx context.Context, scopeHint string) (bool, error) {
	scope := uc.resolver.Resolve(scopeHint)
	hist, err := uc.histFor(scope)
	if err != nil {
		return false, fmt.Errorf("get repository: %w", err)
	}

	if c, ok := hist.(interface {
		Clean(ctx context.Context) (bool, error)
	}); ok {
		return c.Clean(ctx)
	}

	diff, err := hist.Diff(ctx, "")
	if err != nil {
		return false, err
	}
	return diff == "", nil
}

// --- RevertUseCase ---

type RevertUseCase struct {
	resolver *ScopeResolver
	histFor  func(Scope) (HistoryRepository, error)
}

func NewRevertUseCase(
	resolver *ScopeResolver,
	histFor func(Scope) (HistoryRepository, error),
) *RevertUseCase {
	return &RevertUseCase{
		resolver: resolver,
		histFor:  histFor,
	}
}

func (uc *RevertUseCase) Execute(ctx context.Context, input RevertInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	hist, err := uc.histFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return hist.Revert(ctx, input.Ref)
}

// --- KeywordSearchUseCase ---

type KeywordSearchUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
}

func NewKeywordSearchUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
) *KeywordSearchUseCase {
	return &KeywordSearchUseCase{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (uc *KeywordSearchUseCase) Execute(ctx context.Context, input SearchInput) (*SearchOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	all, err := repo.List(ctx, "")
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(input.Query)
	var results []SearchResultOutput

	for _, mem := range all {
		if strings.Contains(strings.ToLower(string(mem.Content)), queryLower) ||
			strings.Contains(strings.ToLower(mem.Key.String()), queryLower) {
			results = append(results, SearchResultOutput{
				Key:   mem.Key.String(),
				Score: 1.0,
			})
		}
		if input.Limit > 0 && len(results) >= input.Limit {
			break
		}
	}

	return &SearchOutput{Results: results}, nil
}

// --- SemanticSearchUseCase ---

type SemanticSearchUseCase struct {
	resolver *ScopeResolver
	indexFor func(Scope) (VectorIndex, error)
	embedder Embedder
}

func NewSemanticSearchUseCase(
	resolver *ScopeResolver,
	indexFor func(Scope) (VectorIndex, error),
	embedder Embedder,
) *SemanticSearchUseCase {
	return &SemanticSearchUseCase{
		resolver: resolver,
		indexFor: indexFor,
		embedder: embedder,
	}
}

func (uc *SemanticSearchUseCase) Execute(ctx context.Context, input SearchInput) (*SearchOutput, error) {
	if uc.embedder == nil {
		return nil, fmt.Errorf("embedder not available")
	}

	scope := uc.resolver.Resolve(input.Scope)
	index, err := uc.indexFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get index: %w", err)
	}
	defer closeIndex(index)

	vec, err := uc.embedder.Embed(ctx, input.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	emb := NewEmbedding(vec, "local")
	results, err := index.Search(ctx, emb, input.Limit)
	if err != nil {
		return nil, err
	}

	output := &SearchOutput{
		Results: make([]SearchResultOutput, len(results)),
	}

	for i, r := range results {
		output.Results[i] = SearchResultOutput{
			Key:   r.Key.String(),
			Score: r.Score,
		}
	}

	return output, nil
}

// --- RebuildIndexUseCase ---

type RebuildIndexUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	indexFor func(Scope) (VectorIndex, error)
	embedder Embedder
}

func NewRebuildIndexUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	indexFor func(Scope) (VectorIndex, error),
	embedder Embedder,
) *RebuildIndexUseCase {
	return &RebuildIndexUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
	}
}

func (uc *RebuildIndexUseCase) Execute(ctx context.Context, input RebuildIndexInput) error {
	if uc.embedder == nil {
		return fmt.Errorf("embedder not available")
	}

	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	index, err := uc.indexFor(scope)
	if err != nil {
		return fmt.Errorf("get index: %w", err)
	}
	defer closeIndex(index)

	memories, err := repo.List(ctx, "")
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}

	total := len(memories)
	if input.OnProgress != nil {
		input.OnProgress(0, total)
	}

	var added, failed int
	for i, mem := range memories {
		vec, err := uc.embedder.Embed(ctx, string(mem.Content))
		if err != nil {
			slog.Warn("rebuild: embed failed", "key", mem.Key.String(), "error", err)
			failed++
		} else {
			emb := NewEmbedding(vec, "local")
			if err := index.Add(ctx, mem.Key, emb); err != nil {
				slog.Warn("rebuild: index add failed", "key", mem.Key.String(), "error", err)
				failed++
			} else {
				added++
			}
		}
		if input.OnProgress != nil {
			input.OnProgress(i+1, total)
		}
	}

	// Don't report success for a rebuild that indexed nothing while there were
	// memories to index — that silently leaves search broken.
	if added == 0 && len(memories) > 0 {
		return fmt.Errorf("rebuild indexed 0 of %d memories (%d failed)", len(memories), failed)
	}

	if err := index.Build(ctx, input.NumTrees); err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	return index.Save(ctx)
}

// --- IndexStatusUseCase ---

type IndexStatusInput struct {
	Scope string
}

// IndexStats describes the on-disk vector index. It is computed from the index
// files directly, so it never triggers an embedding-model load.
type IndexStats struct {
	Exists    bool   `json:"exists"`
	Path      string `json:"path"`
	Vectors   int    `json:"vectors"`
	Dimension int    `json:"dimension"`
	SizeBytes int64  `json:"size_bytes"`
}

type IndexStatusUseCase struct {
	resolver *ScopeResolver
}

func NewIndexStatusUseCase(resolver *ScopeResolver) *IndexStatusUseCase {
	return &IndexStatusUseCase{resolver: resolver}
}

func (uc *IndexStatusUseCase) Execute(ctx context.Context, input IndexStatusInput) (*IndexStats, error) {
	scope := uc.resolver.Resolve(input.Scope)
	base := scope.VectorPath()
	stats := &IndexStats{Path: base}

	info, err := os.Stat(filepath.Join(base, IndexFilename))
	if os.IsNotExist(err) {
		return stats, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat index: %w", err)
	}
	stats.Exists = true
	stats.SizeBytes = info.Size()

	data, err := os.ReadFile(filepath.Join(base, MappingFilename))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read mapping: %w", err)
	}
	if err == nil {
		var m indexMapping
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse mapping: %w", err)
		}
		stats.Vectors = len(m.KeyToID)
	}

	if cfg, err := LoadConfig(scope); err == nil {
		stats.Dimension = cfg.Embeddings.Dimension
	}

	return stats, nil
}

// --- SummarizeUseCase ---

type SummarizeUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	provider Provider
}

func NewSummarizeUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	provider Provider,
) *SummarizeUseCase {
	return &SummarizeUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		provider: provider,
	}
}

func (uc *SummarizeUseCase) Execute(ctx context.Context, input SummarizeInput) (*SummarizeOutput, error) {
	if uc.provider == nil {
		return nil, fmt.Errorf("provider not available")
	}

	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	memories, err := repo.List(ctx, input.Prefix)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	if len(memories) == 0 {
		return &SummarizeOutput{Title: "Empty", Overview: "No memories found"}, nil
	}

	var sb strings.Builder
	sb.WriteString("Summarize the following memories:\n\n")
	for _, mem := range memories {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", mem.Key, mem.Content)
	}

	var summary Summary
	if err := uc.provider.GenerateObject(ctx, sb.String(), &summary); err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}

	return &SummarizeOutput{
		Title:     summary.Title,
		Overview:  summary.Overview,
		KeyPoints: summary.KeyPoints,
		Tags:      summary.Tags,
	}, nil
}

// --- AutoTagUseCase ---

type AutoTagUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (MemoryRepository, error)
	provider Provider
}

func NewAutoTagUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (MemoryRepository, error),
	provider Provider,
) *AutoTagUseCase {
	return &AutoTagUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		provider: provider,
	}
}

func (uc *AutoTagUseCase) Execute(ctx context.Context, input AutoTagInput) (*AutoTagOutput, error) {
	if uc.provider == nil {
		return nil, fmt.Errorf("provider not available")
	}

	key, err := NewKey(input.Key)
	if err != nil {
		return nil, err
	}

	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	mem, err := repo.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get memory: %w", err)
	}

	prompt := fmt.Sprintf("Generate tags for this content:\n\n%s", string(mem.Content))

	var tags AutoTag
	if err := uc.provider.GenerateObject(ctx, prompt, &tags); err != nil {
		return nil, fmt.Errorf("generate tags: %w", err)
	}

	return &AutoTagOutput{
		Tags:       tags.Tags,
		Category:   tags.Category,
		Confidence: tags.Confidence,
	}, nil
}

// --- BranchCurrentUseCase ---

type BranchCurrentUseCase struct {
	resolver  *ScopeResolver
	branchFor func(Scope) (BranchRepository, error)
}

func NewBranchCurrentUseCase(
	resolver *ScopeResolver,
	branchFor func(Scope) (BranchRepository, error),
) *BranchCurrentUseCase {
	return &BranchCurrentUseCase{
		resolver:  resolver,
		branchFor: branchFor,
	}
}

func (uc *BranchCurrentUseCase) Execute(ctx context.Context, input BranchInput) (*BranchOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.branchFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	branch, err := repo.Current(ctx)
	if err != nil {
		return nil, err
	}

	return &BranchOutput{
		Name:      branch.Name,
		Head:      branch.Head,
		CreatedAt: branch.CreatedAt,
	}, nil
}

// --- BranchListUseCase ---

type BranchListUseCase struct {
	resolver  *ScopeResolver
	branchFor func(Scope) (BranchRepository, error)
}

func NewBranchListUseCase(
	resolver *ScopeResolver,
	branchFor func(Scope) (BranchRepository, error),
) *BranchListUseCase {
	return &BranchListUseCase{
		resolver:  resolver,
		branchFor: branchFor,
	}
}

func (uc *BranchListUseCase) Execute(ctx context.Context, input BranchInput) (*BranchListOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.branchFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	branches, err := repo.ListBranches(ctx)
	if err != nil {
		return nil, err
	}

	output := &BranchListOutput{
		Branches: make([]BranchOutput, len(branches)),
	}
	for i, b := range branches {
		output.Branches[i] = BranchOutput{
			Name:      b.Name,
			Head:      b.Head,
			CreatedAt: b.CreatedAt,
		}
	}

	return output, nil
}

// --- BranchCreateUseCase ---

type BranchCreateUseCase struct {
	resolver  *ScopeResolver
	branchFor func(Scope) (BranchRepository, error)
}

func NewBranchCreateUseCase(
	resolver *ScopeResolver,
	branchFor func(Scope) (BranchRepository, error),
) *BranchCreateUseCase {
	return &BranchCreateUseCase{
		resolver:  resolver,
		branchFor: branchFor,
	}
}

func (uc *BranchCreateUseCase) Execute(ctx context.Context, input BranchInput) (*BranchOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.branchFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	branch, err := repo.Create(ctx, input.Name)
	if err != nil {
		return nil, err
	}

	return &BranchOutput{
		Name:      branch.Name,
		Head:      branch.Head,
		CreatedAt: branch.CreatedAt,
	}, nil
}

// --- BranchSwitchUseCase ---

type BranchSwitchUseCase struct {
	resolver  *ScopeResolver
	branchFor func(Scope) (BranchRepository, error)
}

func NewBranchSwitchUseCase(
	resolver *ScopeResolver,
	branchFor func(Scope) (BranchRepository, error),
) *BranchSwitchUseCase {
	return &BranchSwitchUseCase{
		resolver:  resolver,
		branchFor: branchFor,
	}
}

func (uc *BranchSwitchUseCase) Execute(ctx context.Context, input BranchInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.branchFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return repo.Switch(ctx, input.Name)
}

// --- BranchDeleteUseCase ---

type BranchDeleteUseCase struct {
	resolver  *ScopeResolver
	branchFor func(Scope) (BranchRepository, error)
}

func NewBranchDeleteUseCase(
	resolver *ScopeResolver,
	branchFor func(Scope) (BranchRepository, error),
) *BranchDeleteUseCase {
	return &BranchDeleteUseCase{
		resolver:  resolver,
		branchFor: branchFor,
	}
}

func (uc *BranchDeleteUseCase) Execute(ctx context.Context, input BranchInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.branchFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return repo.DeleteBranch(ctx, input.Name)
}

// --- ProviderListUseCase ---

type ProviderListUseCase struct {
	resolver *ScopeResolver
}

// ProviderListItem is a single configured provider with its connection
// details and whether it is the current default.
type ProviderListItem struct {
	Name      string
	Config    ProviderConfig
	IsDefault bool
}

func NewProviderListUseCase(resolver *ScopeResolver) *ProviderListUseCase {
	return &ProviderListUseCase{resolver: resolver}
}

func (uc *ProviderListUseCase) Execute(input ProviderInput) ([]ProviderListItem, error) {
	scope := uc.resolver.Resolve(input.Scope)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return nil, err
	}

	items := make([]ProviderListItem, 0, len(cfg.Providers))
	for name, providerCfg := range cfg.Providers {
		items = append(items, ProviderListItem{
			Name:      name,
			Config:    providerCfg,
			IsDefault: name == cfg.DefaultProvider,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// --- ProviderAddUseCase ---

type ProviderAddUseCase struct {
	resolver *ScopeResolver
}

func NewProviderAddUseCase(resolver *ScopeResolver) *ProviderAddUseCase {
	return &ProviderAddUseCase{resolver: resolver}
}

func (uc *ProviderAddUseCase) Execute(input ProviderInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	cfg.Providers[input.Name] = input.Config
	return SaveConfig(scope, cfg)
}

// --- ProviderRemoveUseCase ---

type ProviderRemoveUseCase struct {
	resolver *ScopeResolver
}

func NewProviderRemoveUseCase(resolver *ScopeResolver) *ProviderRemoveUseCase {
	return &ProviderRemoveUseCase{resolver: resolver}
}

func (uc *ProviderRemoveUseCase) Execute(input ProviderInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	delete(cfg.Providers, input.Name)
	return SaveConfig(scope, cfg)
}

// --- ProviderSetDefaultUseCase ---

type ProviderSetDefaultUseCase struct {
	resolver *ScopeResolver
}

func NewProviderSetDefaultUseCase(resolver *ScopeResolver) *ProviderSetDefaultUseCase {
	return &ProviderSetDefaultUseCase{resolver: resolver}
}

func (uc *ProviderSetDefaultUseCase) Execute(input ProviderInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	if _, exists := cfg.Providers[input.Name]; !exists {
		return fmt.Errorf("provider %q not found", input.Name)
	}

	cfg.DefaultProvider = input.Name
	return SaveConfig(scope, cfg)
}

// --- ProviderTestUseCase ---

type ProviderTestUseCase struct {
	resolver *ScopeResolver
}

func NewProviderTestUseCase(resolver *ScopeResolver) *ProviderTestUseCase {
	return &ProviderTestUseCase{resolver: resolver}
}

func (uc *ProviderTestUseCase) Execute(ctx context.Context, input ProviderInput) error {
	scope := uc.resolver.Resolve(input.Scope)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	providerCfg, exists := cfg.Providers[input.Name]
	if !exists {
		return fmt.Errorf("provider %q not found", input.Name)
	}

	provider, err := NewFantasyProvider(ctx, FantasyConfig{
		Provider: input.Name,
		APIKey:   providerCfg.APIKey,
		BaseURL:  providerCfg.BaseURL,
		Model:    providerCfg.Model,
	})
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	_, err = provider.Complete(ctx, "Say hello")
	return err
}
