package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewSetCmd(svc func() *internal.MemoryService, hist func() *internal.HistoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Create or update a memory",
		Long:  `Create or update a memory with the given key. Reads from stdin if value is not provided.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  makeSetRunner(svc, hist),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeSetRunner(svc func() *internal.MemoryService, hist func() *internal.HistoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]

		content, err := resolveContent(args)
		if err != nil {
			return err
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		if err := svc().Set(cmd.Context(), key, content, scopeHint); err != nil {
			return fmt.Errorf("set memory: %w", err)
		}

		if err := autoCommit(cmd.Context(), hist(), message, "set", key, scopeHint); err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set %s\n", key)
		return nil
	}
}

func resolveContent(args []string) (string, error) {
	if len(args) >= 2 {
		return args[1], nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}

func autoCommit(ctx context.Context, hist *internal.HistoryService, message, action, key, scopeHint string) error {
	if hist == nil {
		return nil
	}

	if message == "" {
		message = fmt.Sprintf("%s: %s", action, key)
	}

	_, err := hist.Commit(ctx, message, scopeHint)
	return err
}
