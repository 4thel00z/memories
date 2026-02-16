package main

import (
	"fmt"
	"io"
	"os"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewAddCmd(svc func() *internal.MemoryService, hist func() *internal.HistoryService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <key> [content]",
		Short: "Append content to a memory",
		Long:  `Append content to an existing memory or create a new one. Reads from stdin if content is not provided.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  makeAddRunner(svc, hist),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeAddRunner(svc func() *internal.MemoryService, hist func() *internal.HistoryService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]

		content, err := resolveAddContent(args)
		if err != nil {
			return err
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		existing, err := svc().Get(cmd.Context(), key, scopeHint)
		if err != nil && err != internal.ErrNotFound {
			return fmt.Errorf("get existing memory: %w", err)
		}

		var newContent string
		if existing != nil {
			newContent = string(existing.Content) + "\n" + content
		} else {
			newContent = content
		}

		if err := svc().Set(cmd.Context(), key, newContent, scopeHint); err != nil {
			return fmt.Errorf("add to memory: %w", err)
		}

		if err := autoCommit(cmd.Context(), hist(), message, "add", key, scopeHint); err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		if existing != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Appended to %s\n", key)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", key)
		}
		return nil
	}
}

func resolveAddContent(args []string) (string, error) {
	if len(args) >= 2 {
		return args[1], nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}
