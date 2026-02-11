package internal

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MemoryService handles memory CRUD operations
type MemoryService struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
}

func NewMemoryService(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
	embedder Embedder,
) *MemoryService {
	return &MemoryService{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
	}
}

func (s *MemoryService) Set(ctx context.Context, keyStr, content, scopeHint string) error {
	key, err := NewKey(keyStr)
	if err != nil {
		return err
	}

	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	mem := &Memory{
		Key:       key,
		Content:   []byte(content),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repo.Save(ctx, mem); err != nil {
		return fmt.Errorf("save memory: %w", err)
	}

	if s.embedder == nil {
		return nil
	}

	index, err := s.indexFor(scope)
	if err != nil {
		return nil // ignore index errors, embedding is optional
	}

	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil // ignore embedding errors
	}

	emb := NewEmbedding(vec, "local")
	_ = index.Add(ctx, key, emb)

	return nil
}

func (s *MemoryService) Get(ctx context.Context, keyStr, scopeHint string) (*Memory, error) {
	key, err := NewKey(keyStr)
	if err != nil {
		return nil, err
	}

	if scopeHint != "" {
		scope := s.resolver.Resolve(scopeHint)
		repo, err := s.repoFor(scope)
		if err != nil {
			return nil, fmt.Errorf("get repository: %w", err)
		}
		return repo.Get(ctx, key)
	}

	for _, scope := range s.resolver.Cascade() {
		repo, err := s.repoFor(scope)
		if err != nil {
			continue
		}
		mem, err := repo.Get(ctx, key)
		if err == nil {
			return mem, nil
		}
	}

	return nil, ErrNotFound
}

func (s *MemoryService) Delete(ctx context.Context, keyStr, scopeHint string) error {
	key, err := NewKey(keyStr)
	if err != nil {
		return err
	}

	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	if err := repo.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}

	index, err := s.indexFor(scope)
	if err == nil {
		_ = index.Remove(ctx, key)
	}

	return nil
}

func (s *MemoryService) List(ctx context.Context, prefix, scopeHint string) ([]*Memory, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return repo.List(ctx, prefix)
}

// HistoryService handles git history operations
type HistoryService struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewHistoryService(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
) *HistoryService {
	return &HistoryService{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (s *HistoryService) Commit(ctx context.Context, message, scopeHint string) (*Commit, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return repo.Commit(ctx, message)
}

func (s *HistoryService) Log(ctx context.Context, limit int, scopeHint string) ([]*Commit, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return repo.Log(ctx, limit)
}

func (s *HistoryService) Diff(ctx context.Context, ref, scopeHint string) (string, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return "", fmt.Errorf("get repository: %w", err)
	}

	return repo.Diff(ctx, ref)
}

func (s *HistoryService) Revert(ctx context.Context, ref, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return repo.Revert(ctx, ref)
}

// BranchService handles branch operations
type BranchService struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
}

func NewBranchService(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
) *BranchService {
	return &BranchService{
		resolver: resolver,
		repoFor:  repoFor,
	}
}

func (s *BranchService) Current(ctx context.Context, scopeHint string) (*Branch, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return repo.Current(ctx)
}

func (s *BranchService) List(ctx context.Context, scopeHint string) ([]*Branch, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	return repo.ListBranches(ctx)
}

func (s *BranchService) Create(ctx context.Context, name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	_, err = repo.Create(ctx, name)
	return err
}

func (s *BranchService) Switch(ctx context.Context, name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return repo.Switch(ctx, name)
}

func (s *BranchService) Delete(ctx context.Context, name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	return repo.DeleteBranch(ctx, name)
}

// SearchService handles keyword and semantic search
type SearchService struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	indexFor func(Scope) (*AnnoyIndex, error)
	embedder Embedder
}

func NewSearchService(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	indexFor func(Scope) (*AnnoyIndex, error),
	embedder Embedder,
) *SearchService {
	return &SearchService{
		resolver: resolver,
		repoFor:  repoFor,
		indexFor: indexFor,
		embedder: embedder,
	}
}

func (s *SearchService) Keyword(ctx context.Context, query, scopeHint string) ([]*Memory, error) {
	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	all, err := repo.List(ctx, "")
	if err != nil {
		return nil, err
	}

	var matches []*Memory
	queryLower := strings.ToLower(query)

	for _, mem := range all {
		if strings.Contains(strings.ToLower(string(mem.Content)), queryLower) {
			matches = append(matches, mem)
		}
		if strings.Contains(strings.ToLower(mem.Key.String()), queryLower) {
			matches = append(matches, mem)
		}
	}

	return matches, nil
}

func (s *SearchService) Semantic(ctx context.Context, query string, k int, scopeHint string) ([]SearchResult, error) {
	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not available")
	}

	scope := s.resolver.Resolve(scopeHint)
	index, err := s.indexFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get index: %w", err)
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	emb := NewEmbedding(vec, "local")
	return index.Search(ctx, emb, k)
}

func (s *SearchService) RebuildIndex(ctx context.Context, scopeHint string, numTrees int) error {
	if s.embedder == nil {
		return fmt.Errorf("embedder not available")
	}

	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	index, err := s.indexFor(scope)
	if err != nil {
		return fmt.Errorf("get index: %w", err)
	}

	memories, err := repo.List(ctx, "")
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}

	for _, mem := range memories {
		vec, err := s.embedder.Embed(ctx, string(mem.Content))
		if err != nil {
			continue
		}
		emb := NewEmbedding(vec, "local")
		_ = index.Add(ctx, mem.Key, emb)
	}

	if err := index.Build(ctx, numTrees); err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	return index.Save(ctx)
}

// SummarizeService handles AI-powered summarization
type SummarizeService struct {
	resolver *ScopeResolver
	repoFor  func(Scope) (*GitRepository, error)
	provider Provider
}

func NewSummarizeService(
	resolver *ScopeResolver,
	repoFor func(Scope) (*GitRepository, error),
	provider Provider,
) *SummarizeService {
	return &SummarizeService{
		resolver: resolver,
		repoFor:  repoFor,
		provider: provider,
	}
}

func (s *SummarizeService) Summarize(ctx context.Context, prefix, scopeHint string) (*Summary, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("provider not available")
	}

	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	memories, err := repo.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}

	if len(memories) == 0 {
		return &Summary{Title: "Empty", Overview: "No memories found"}, nil
	}

	var sb strings.Builder
	sb.WriteString("Summarize the following memories:\n\n")
	for _, mem := range memories {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", mem.Key, string(mem.Content)))
	}

	var summary Summary
	if err := s.provider.GenerateObject(ctx, sb.String(), &summary); err != nil {
		return nil, fmt.Errorf("generate summary: %w", err)
	}

	return &summary, nil
}

func (s *SummarizeService) AutoTag(ctx context.Context, keyStr, scopeHint string) (*AutoTag, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("provider not available")
	}

	key, err := NewKey(keyStr)
	if err != nil {
		return nil, err
	}

	scope := s.resolver.Resolve(scopeHint)
	repo, err := s.repoFor(scope)
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}

	mem, err := repo.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}

	prompt := fmt.Sprintf("Generate tags for this content:\n\n%s", string(mem.Content))

	var tags AutoTag
	if err := s.provider.GenerateObject(ctx, prompt, &tags); err != nil {
		return nil, fmt.Errorf("generate tags: %w", err)
	}

	return &tags, nil
}

// ProviderService manages LLM provider configuration
type ProviderService struct {
	resolver *ScopeResolver
}

func NewProviderService(resolver *ScopeResolver) *ProviderService {
	return &ProviderService{resolver: resolver}
}

func (s *ProviderService) List(scopeHint string) ([]string, error) {
	scope := s.resolver.Resolve(scopeHint)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		names = append(names, name)
	}
	return names, nil
}

func (s *ProviderService) Add(name string, providerCfg ProviderConfig, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	cfg.Providers[name] = providerCfg
	return SaveConfig(scope, cfg)
}

func (s *ProviderService) Remove(name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	delete(cfg.Providers, name)
	return SaveConfig(scope, cfg)
}

func (s *ProviderService) SetDefault(name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	if _, exists := cfg.Providers[name]; !exists {
		return fmt.Errorf("provider %q not found", name)
	}

	cfg.DefaultProvider = name
	return SaveConfig(scope, cfg)
}

func (s *ProviderService) Test(ctx context.Context, name, scopeHint string) error {
	scope := s.resolver.Resolve(scopeHint)
	cfg, err := LoadConfig(scope)
	if err != nil {
		return err
	}

	providerCfg, exists := cfg.Providers[name]
	if !exists {
		return fmt.Errorf("provider %q not found", name)
	}

	provider, err := NewFantasyProvider(ctx, FantasyConfig{
		Provider: name,
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
