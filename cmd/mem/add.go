package main

import (
	"fmt"
	"io"
	"os"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewAddCmd(addUC *internal.AddMemoryUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <key> [content]",
		Short: "Append content to a memory",
		Long:  `Append content to an existing memory or create a new one. Reads from stdin if content is not provided.`,
		Args:  cobra.RangeArgs(1, 2),
		RunE:  makeAddRunner(addUC),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeAddRunner(addUC *internal.AddMemoryUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]

		content, err := resolveAddContent(args)
		if err != nil {
			return err
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		_, err = addUC.Execute(cmd.Context(), internal.AddMemoryInput{
			Key: key, Content: content, Scope: scopeHint, Message: message,
		})
		if err != nil {
			return fmt.Errorf("add to memory: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Appended to %s\n", key)
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
