package main

import (
	"context"
	"fmt"
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

	app := newApp()
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
	resolver     *internal.ScopeResolver
	memorySvc    *internal.MemoryService
	historySvc   *internal.HistoryService
	branchSvc    *internal.BranchService
	searchSvc    *internal.SearchService
	summarizeSvc *internal.SummarizeService
	providerSvc  *internal.ProviderService
}

func newApp() *app {
	resolver := internal.NewScopeResolver()

	repoFor := func(scope internal.Scope) (*internal.GitRepository, error) {
		return internal.NewGitRepository(scope)
	}
	indexFor := func(scope internal.Scope) (*internal.AnnoyIndex, error) {
		return nil, internal.ErrNoIndex
	}

	return &app{
		resolver:     resolver,
		memorySvc:    internal.NewMemoryService(resolver, repoFor, indexFor, nil),
		historySvc:   internal.NewHistoryService(resolver, repoFor),
		branchSvc:    internal.NewBranchService(resolver, repoFor),
		searchSvc:    internal.NewSearchService(resolver, repoFor, indexFor, nil),
		summarizeSvc: internal.NewSummarizeService(resolver, repoFor, nil),
		providerSvc:  internal.NewProviderService(resolver),
	}
}
