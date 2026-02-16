package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewDelCmd(delUC *internal.DeleteMemoryUseCase, commitUC *internal.CommitUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "del <key>",
		Aliases: []string{"delete", "rm"},
		Short:   "Delete a memory",
		Long:    `Delete a memory by key.`,
		Args:    cobra.ExactArgs(1),
		RunE:    makeDelRunner(delUC, commitUC),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeDelRunner(delUC *internal.DeleteMemoryUseCase, commitUC *internal.CommitUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]
		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		if err := delUC.Execute(cmd.Context(), internal.DeleteMemoryInput{
			Key: key, Scope: scopeHint,
		}); err != nil {
			return fmt.Errorf("delete memory: %w", err)
		}

		if err := autoCommit(cmd.Context(), commitUC, message, "del", key, scopeHint); err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", key)
		return nil
	}
}
