package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewStatusCmd(branch func() *internal.BranchService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		Long:  `Show the current branch and any uncommitted changes.`,
		RunE:  makeStatusRunner(branch),
	}

	return cmd
}

func makeStatusRunner(branch func() *internal.BranchService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")

		current, err := branch().Current(cmd.Context(), scopeHint)
		if err != nil {
			return fmt.Errorf("get current branch: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "On branch %s\n", current.Name)
		return nil
	}
}
