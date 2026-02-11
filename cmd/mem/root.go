package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCmd(version string) *cobra.Command {
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

	return rootCmd
}

func addPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("scope", "", "Target scope (global|project)")
	cmd.PersistentFlags().String("branch", "", "Target branch")
	cmd.PersistentFlags().Bool("json", false, "Output in JSON format")
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
