package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/4thel00z/memories/internal"
	"github.com/charmbracelet/fang"
)

// version is set via ldflags at build time
var version = "dev"

func main() {
	ctx := context.Background()

	if tryExternalCommand(ctx) {
		return
	}

	debug := hasDebugFlag()
	app := newApp(debug)
	rootCmd := NewRootCmd(version, app)
	if err := fang.Execute(ctx, rootCmd); err != nil {
		os.Exit(1)
	}
}

func tryExternalCommand(ctx context.Context) bool {
	if len(os.Args) < 2 {
		return false
	}

	cmd := os.Args[1]
	if cmd == "" || cmd[0] == '-' {
		return false
	}

	if _, err := findExternal(cmd); err != nil {
		return false
	}

	if err := executeExternal(ctx, cmd, os.Args[2:], version); err != nil {
		fmt.Fprintf(os.Stderr, "mem %s: %v\n", cmd, err)
		os.Exit(1)
	}

	return true
}

type app struct {
	resolver *internal.ScopeResolver
	uc       *internal.UseCases
}

func hasDebugFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			return true
		}
	}
	return false
}

func newApp(debug bool) *app {
	resolver := internal.NewScopeResolver()

	repoFor := func(scope internal.Scope) (internal.MemoryRepository, error) {
		return internal.NewGitRepository(scope)
	}
	histFor := func(scope internal.Scope) (internal.HistoryRepository, error) {
		return internal.NewGitRepository(scope)
	}
	branchFor := func(scope internal.Scope) (internal.BranchRepository, error) {
		return internal.NewGitRepository(scope)
	}

	// Lazy embedder + index initialization. The wrapper defers the expensive
	// model download/load until an embedding method is actually called, so plain
	// commands like `mem get`/`list`/`status` never pay that cost.
	embedder := newLazyEmbedder(func() (internal.Embedder, error) {
		cacheDir, err := internal.DefaultCacheDir()
		if err != nil {
			return nil, fmt.Errorf("get cache dir: %w", err)
		}

		// Load config from resolved scope for model URL and token
		modelURL, modelFilename, token := embeddingsFromConfig(resolver)

		dl := internal.NewDownloader(cacheDir, token)
		modelPath, err := dl.EnsureModel(context.Background(), modelURL, modelFilename, nil)
		if err != nil {
			return nil, fmt.Errorf("download embedding model: %w", err)
		}

		var embedOpts []internal.EmbedderOption
		if debug {
			embedOpts = append(embedOpts, internal.WithDebug())
		}
		return internal.NewLocalEmbedder(modelPath, 0, embedOpts...)
	})

	indexFor := func(scope internal.Scope) (internal.VectorIndex, error) {
		// Resolving the embedder triggers the model load; if it fails there is no
		// usable index.
		if _, err := embedder.resolve(); err != nil {
			return nil, internal.ErrNoIndex
		}
		idx, err := internal.NewAnnoyIndex(scope.VectorPath(), embedder.Dimension())
		if err != nil {
			return nil, err
		}
		if err := idx.Load(context.Background()); err != nil {
			slog.Warn("failed to load index", "error", err)
		}
		return idx, nil
	}

	setMemoryUC := internal.NewSetMemoryUseCase(resolver, repoFor, indexFor, embedder, nil)
	rebuildIndexUC := internal.NewRebuildIndexUseCase(resolver, repoFor, indexFor, embedder)

	// SetMemory keeps the vector index in sync incrementally, so the hook needs
	// no separate reindex pass.
	hookStoreFn := func(ctx context.Context, key, content string) error {
		return setMemoryUC.Execute(ctx, internal.SetMemoryInput{Key: key, Content: content})
	}

	uc := &internal.UseCases{
		SetMemory:      setMemoryUC,
		GetMemory:      internal.NewGetMemoryUseCase(resolver, repoFor),
		DeleteMemory:   internal.NewDeleteMemoryUseCase(resolver, repoFor, indexFor),
		ListMemories:   internal.NewListMemoriesUseCase(resolver, repoFor),
		AddMemory:      internal.NewAddMemoryUseCase(resolver, repoFor, histFor, indexFor, embedder, nil),
		EditMemory:     internal.NewEditMemoryUseCase(resolver, repoFor, histFor, indexFor, embedder, nil),
		Commit:         internal.NewCommitUseCase(resolver, histFor),
		Log:            internal.NewLogUseCase(resolver, histFor),
		Diff:           internal.NewDiffUseCase(resolver, histFor),
		Revert:         internal.NewRevertUseCase(resolver, histFor),
		KeywordSearch:  internal.NewKeywordSearchUseCase(resolver, repoFor),
		SemanticSearch: internal.NewSemanticSearchUseCase(resolver, indexFor, embedder),
		RebuildIndex:   rebuildIndexUC,
		Summarize:      internal.NewSummarizeUseCase(resolver, repoFor, nil),
		AutoTag:        internal.NewAutoTagUseCase(resolver, repoFor, nil),
		BranchCurrent:  internal.NewBranchCurrentUseCase(resolver, branchFor),
		BranchList:     internal.NewBranchListUseCase(resolver, branchFor),
		BranchCreate:   internal.NewBranchCreateUseCase(resolver, branchFor),
		BranchSwitch:   internal.NewBranchSwitchUseCase(resolver, branchFor),
		BranchDelete:   internal.NewBranchDeleteUseCase(resolver, branchFor),
		ProviderList:   internal.NewProviderListUseCase(resolver),
		ProviderAdd:    internal.NewProviderAddUseCase(resolver),
		ProviderRemove: internal.NewProviderRemoveUseCase(resolver),
		ProviderSetDef: internal.NewProviderSetDefaultUseCase(resolver),
		ProviderTest:   internal.NewProviderTestUseCase(resolver),
		InstallHook:    internal.NewInstallHookUseCase(resolver),
		UninstallHook:  internal.NewUninstallHookUseCase(resolver),
		RunHook:        internal.NewRunHookUseCase(resolver, nil, hookStoreFn),
	}

	return &app{
		resolver: resolver,
		uc:       uc,
	}
}

func embeddingsFromConfig(resolver *internal.ScopeResolver) (modelURL, modelFilename, token string) {
	modelURL = internal.DefaultModelURL
	modelFilename = internal.DefaultModelFilename

	scope := resolver.Resolve("")
	cfg, err := internal.LoadConfig(scope)
	if err != nil {
		return
	}

	if cfg.Embeddings.ModelURL != "" {
		modelURL = cfg.Embeddings.ModelURL
	}
	if cfg.Embeddings.Model != "" {
		modelFilename = cfg.Embeddings.Model
	}
	token = cfg.Embeddings.Token
	return
}
