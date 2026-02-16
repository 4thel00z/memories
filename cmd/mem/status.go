package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewStatusCmd(currentUC *internal.BranchCurrentUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		Long:  `Show the current branch and any uncommitted changes.`,
		RunE:  makeStatusRunner(currentUC),
	}

	return cmd
}

func makeStatusRunner(currentUC *internal.BranchCurrentUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")

		out, err := currentUC.Execute(cmd.Context(), internal.BranchInput{
			Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("get current branch: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "On branch %s\n", out.Name)
		return nil
	}
}
