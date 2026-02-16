package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewRootCmd(version string, a *app) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "mem",
		Short:         "Git-powered memory management for CLI",
		Long:          `A file-based memory abstraction with git history and semantic search.`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	addPersistentFlags(rootCmd)
	setHelpWithExternals(rootCmd)

	if a != nil {
		addSubcommands(rootCmd, a)
	}

	return rootCmd
}

func addPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("scope", "", "Target scope (global|project)")
	cmd.PersistentFlags().String("branch", "", "Target branch")
	cmd.PersistentFlags().Bool("json", false, "Output in JSON format")
}

func addSubcommands(root *cobra.Command, a *app) {
	mem := func() *internal.MemoryService { return a.memorySvc }
	hist := func() *internal.HistoryService { return a.historySvc }
	branch := func() *internal.BranchService { return a.branchSvc }
	search := func() *internal.SearchService { return a.searchSvc }
	summarize := func() *internal.SummarizeService { return a.summarizeSvc }
	provider := func() *internal.ProviderService { return a.providerSvc }

	root.AddCommand(
		NewInitCmd(),
		NewSetCmd(mem, hist),
		NewGetCmd(mem),
		NewDelCmd(mem, hist),
		NewListCmd(mem),
		NewAddCmd(mem, hist),
		NewCommitCmd(hist),
		NewStatusCmd(branch),
		NewLogCmd(hist),
		NewDiffCmd(hist),
		NewBranchCmd(branch),
		NewSearchCmd(search),
		NewProviderCmd(provider),
		NewIndexCmd(search),
		NewSummarizeCmd(summarize),
		NewEditCmd(mem, hist),
		NewWatchCmd(hist),
	)
}

func setHelpWithExternals(cmd *cobra.Command) {
	defaultHelp := cmd.HelpFunc()

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		defaultHelp(c, args)
		printExternalCommands(c)
	})
}

func printExternalCommands(cmd *cobra.Command) {
	externals := listExternalCommands()
	if len(externals) == 0 {
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nExternal commands (mem-*):")
	for _, name := range externals {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", name)
	}
}
