package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewDiffCmd(diffUC *internal.DiffUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [ref]",
		Short: "Show changes",
		Long:  `Show uncommitted changes or diff against a specific ref.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeDiffRunner(diffUC),
	}

	return cmd
}

func makeDiffRunner(diffUC *internal.DiffUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ref := ""
		if len(args) > 0 {
			ref = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")

		out, err := diffUC.Execute(cmd.Context(), internal.DiffInput{
			Ref: ref, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("get diff: %w", err)
		}

		if out.Diff == "" {
			fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
			return nil
		}

		fmt.Fprint(cmd.OutOrStdout(), out.Diff)
		return nil
	}
}
