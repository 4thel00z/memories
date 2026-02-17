package main

import (
	"fmt"

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
	cmd.PersistentFlags().Bool("debug", false, "Enable verbose output (e.g. model loading logs)")
}

func addSubcommands(root *cobra.Command, a *app) {
	uc := a.uc
	root.AddCommand(
		NewInitCmd(),
		NewSetCmd(uc.SetMemory, uc.Commit),
		NewGetCmd(uc.GetMemory),
		NewDelCmd(uc.DeleteMemory, uc.Commit),
		NewListCmd(uc.ListMemories),
		NewAddCmd(uc.AddMemory),
		NewCommitCmd(uc.Commit),
		NewStatusCmd(uc.BranchCurrent),
		NewLogCmd(uc.Log),
		NewDiffCmd(uc.Diff),
		NewBranchCmd(uc.BranchCurrent, uc.BranchList, uc.BranchCreate, uc.BranchSwitch, uc.BranchDelete),
		NewSearchCmd(uc.KeywordSearch, uc.SemanticSearch),
		NewProviderCmd(uc.ProviderList, uc.ProviderAdd, uc.ProviderRemove, uc.ProviderSetDef, uc.ProviderTest),
		NewIndexCmd(uc.RebuildIndex),
		NewSummarizeCmd(uc.Summarize),
		NewEditCmd(uc.GetMemory, uc.SetMemory, uc.Commit),
		NewWatchCmd(uc.Commit),
		NewSkillCmd(),
		NewInstallCmd(uc.InstallHook),
		NewUninstallCmd(uc.UninstallHook),
		NewHookCmd(uc.RunHook),
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
