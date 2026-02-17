package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewUninstallCmd(uc *internal.UninstallHookUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove mem git hooks",
		Long:  `Remove the post-commit hook installed by mem. Restores any backed-up original hook.`,
		RunE:  makeUninstallRunner(uc),
	}

	cmd.Flags().Bool("keep-config", false, "Keep hook configuration in .mem/config.yaml")

	return cmd
}

func makeUninstallRunner(uc *internal.UninstallHookUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		scope, _ := cmd.Flags().GetString("scope")
		keepConfig, _ := cmd.Flags().GetBool("keep-config")

		err := uc.Execute(cmd.Context(), internal.UninstallHookInput{
			Scope:      scope,
			KeepConfig: keepConfig,
		})
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Uninstalled post-commit hook")
		return nil
	}
}
