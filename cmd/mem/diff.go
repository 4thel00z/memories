package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewDiffCmd(hist func() *internal.HistoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [ref]",
		Short: "Show changes",
		Long:  `Show uncommitted changes or diff against a specific ref.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeDiffRunner(hist),
	}

	return cmd
}

func makeDiffRunner(hist func() *internal.HistoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ref := ""
		if len(args) > 0 {
			ref = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")

		diff, err := hist().Diff(cmd.Context(), ref, scopeHint)
		if err != nil {
			return fmt.Errorf("get diff: %w", err)
		}

		if diff == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
			return nil
		}

		fmt.Fprint(cmd.OutOrStdout(), diff)
		return nil
	}
}
