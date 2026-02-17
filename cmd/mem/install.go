package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewInstallCmd(uc *internal.InstallHookUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install git hooks for automatic memory updates",
		Long:  `Install a post-commit hook that inspects diffs and updates the memory store.`,
		RunE:  makeInstallRunner(uc),
	}

	cmd.Flags().String("strategy", "extract", "Hook strategy (summarize|extract|script|all)")
	cmd.Flags().String("script", "", "Path to custom hook script (used with strategy=script or all)")
	cmd.Flags().Bool("force", false, "Overwrite existing hook (backs up original)")

	return cmd
}

func makeInstallRunner(uc *internal.InstallHookUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		strategy, _ := cmd.Flags().GetString("strategy")
		script, _ := cmd.Flags().GetString("script")
		force, _ := cmd.Flags().GetBool("force")

		err := uc.Execute(cmd.Context(), internal.InstallHookInput{
			Scope:    scope,
			Strategy: strategy,
			Script:   script,
			Force:    force,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Installed post-commit hook")
		return nil
	}
}
