package internal

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Use case input/output DTOs

type SetMemoryInput struct {
	Key     string
	Content string
	Scope   string
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

// Use cases

type SetMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
	ignore   func(Scope) (*IgnoreMatcher, error)
}

func NewSetMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
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
		if err == nil && matcher.MatchKey(key) {
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

	if err := repo.Save(ctx, mem); err != nil {
		return fmt.Errorf("save memory: %w", err)
	}

	if uc.embedder == nil || uc.indexFor == nil {
		return nil
	}

	index, err := uc.indexFor(scope)
	if err != nil {
		return nil
	}

	vec, err := uc.embedder.Embed(ctx, input.Content)
	if err != nil {
		return nil
	}

	emb := NewEmbedding(vec, "local")
	_ = index.Add(ctx, key, emb)

	return nil
}

type GetMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewGetMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
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

type DeleteMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
}

func NewDeleteMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
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

	if err := repo.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	if uc.indexFor != nil {
		if index, err := uc.indexFor(scope); err == nil {
			_ = index.Remove(ctx, key)
		}
	}

	return nil
}

type ListMemoriesUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewListMemoriesUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
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

type AddMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
	ignore   func(Scope) (*IgnoreMatcher, error)
}

func NewAddMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
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

type AddMemoryInput struct {
	Key     string
	Content string
	Scope   string
	Message string
}

func (uc *AddMemoryUseCase) Execute(ctx context.Context, input AddMemoryInput) (*CommitOutput, error) {
	key, err := NewKey(input.Key)
	if err != nil {
		return nil, err
	}

	scope := uc.resolver.Resolve(input.Scope)

	if uc.ignore != nil {
		matcher, err := uc.ignore(scope)
		if err == nil && matcher.MatchKey(key) {
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
		newContent = append(existing.Content, []byte("\n"+input.Content)...)
	} else {
		newContent = []byte(input.Content)
	}

	mem := &Memory{
		Key:       key,
		Content:   newContent,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.Save(ctx, mem); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	message := input.Message
	if message == "" {
		message = fmt.Sprintf("add: append to %s", input.Key)
	}

	commit, err := repo.Commit(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if uc.embedder != nil && uc.indexFor != nil {
		if index, err := uc.indexFor(scope); err == nil {
			if vec, err := uc.embedder.Embed(ctx, string(newContent)); err == nil {
				emb := NewEmbedding(vec, "local")
				_ = index.Add(ctx, key, emb)
			}
		}
	}

	return &CommitOutput{
		Hash:      commit.Hash,
		Message:   commit.Message,
		Timestamp: commit.Timestamp,
	}, nil
}

type EditMemoryUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
	ignore   func(Scope) (*IgnoreMatcher, error)
}

func NewEditMemoryUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
	embedder Embedder,
	ignore func(Scope) (*IgnoreMatcher, error),
) *EditMemoryUseCase {
	return &EditMemoryUseCase{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
		ignore:   ignore,
	}
}

type EditMemoryInput struct {
	Key     string
	Content string
	Scope   string
	Message string
}

func (uc *EditMemoryUseCase) Execute(ctx context.Context, input EditMemoryInput) (*CommitOutput, error) {
	key, err := NewKey(input.Key)
	if err != nil {
		return nil, err
	}

	scope := uc.resolver.Resolve(input.Scope)

	if uc.ignore != nil {
		matcher, err := uc.ignore(scope)
		if err == nil && matcher.MatchKey(key) {
			return nil, fmt.Errorf("key %q is blocked by .memignore", input.Key)
		}
	}

	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	mem := &Memory{
		Key:       key,
		Content:   []byte(input.Content),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.Save(ctx, mem); err != nil {
		return nil, fmt.Errorf("save memory: %w", err)
	}

	message := input.Message
	if message == "" {
		message = fmt.Sprintf("edit: update %s", input.Key)
	}

	commit, err := repo.Commit(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if uc.embedder != nil && uc.indexFor != nil {
		if index, err := uc.indexFor(scope); err == nil {
			if vec, err := uc.embedder.Embed(ctx, input.Content); err == nil {
				emb := NewEmbedding(vec, "local")
				_ = index.Add(ctx, key, emb)
			}
		}
	}

	return &CommitOutput{
		Hash:      commit.Hash,
		Message:   commit.Message,
		Timestamp: commit.Timestamp,
	}, nil
}

type CommitUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewCommitUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
) *CommitUseCase {
	return &CommitUseCase{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (uc *CommitUseCase) Execute(ctx context.Context, input CommitInput) (*CommitOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	commit, err := repo.Commit(ctx, input.Message)
	if err != nil {
		return nil, err
	}

	return &CommitOutput{
		Hash:      commit.Hash,
		Message:   commit.Message,
		Timestamp: commit.Timestamp,
	}, nil
}

type LogUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewLogUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
) *LogUseCase {
	return &LogUseCase{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (uc *LogUseCase) Execute(ctx context.Context, input LogInput) (*LogOutput, error) {
	scope := uc.resolver.Resolve(input.Scope)
	repo, err := uc.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	commits, err := repo.Log(ctx, input.Limit)
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

type KeywordSearchUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewKeywordSearchUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
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

type SemanticSearchUseCase struct {
	resolver *ScopeResolver
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
}

func NewSemanticSearchUseCase(
	resolver *ScopeResolver,
	indexFor func(Scope) (*AnnoyIndex, error),
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

type SummarizeUseCase struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	provider Provider
}

func NewSummarizeUseCase(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
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
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", mem.Key, string(mem.Content)))
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
