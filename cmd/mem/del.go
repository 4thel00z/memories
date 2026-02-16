package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewDelCmd(svc func() *internal.MemoryService, hist func() *internal.HistoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "del <key>",
		Aliases: []string{"delete", "rm"},
		Short:   "Delete a memory",
		Long:    `Delete a memory by key.`,
		Args:    cobra.ExactArgs(1),
		RunE:    makeDelRunner(svc, hist),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeDelRunner(svc func() *internal.MemoryService, hist func() *internal.HistoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]
		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		if err := svc().Delete(cmd.Context(), key, scopeHint); err != nil {
			return fmt.Errorf("delete memory: %w", err)
		}

		if err := autoCommit(cmd.Context(), hist(), message, "del", key, scopeHint); err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", key)
		return nil
	}
}
