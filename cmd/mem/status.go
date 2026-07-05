package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewStatusCmd(currentUC *internal.BranchCurrentUseCase, diffUC *internal.DiffUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show working tree status",
		Long:  `Show the current branch and whether there are uncommitted changes.`,
		RunE:  makeStatusRunner(currentUC, diffUC),
	}

	return cmd
}

func makeStatusRunner(currentUC *internal.BranchCurrentUseCase, diffUC *internal.DiffUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		out, err := currentUC.Execute(cmd.Context(), internal.BranchInput{
			Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("get current branch: %w", err)
		}

		clean, err := diffUC.Clean(cmd.Context(), scopeHint)
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}
		dirty := !clean

		if asJSON {
			return printJSON(cmd.OutOrStdout(), map[string]any{
				"branch": out.Name,
				"dirty":  dirty,
			})
		}

		fmt.Fprintf(cmd.OutOrStdout(), "On branch %s\n", out.Name)
		if dirty {
			fmt.Fprintln(cmd.OutOrStdout(), "Uncommitted changes present (see 'mem diff')")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Working tree clean")
		}
		return nil
	}
}
