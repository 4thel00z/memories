package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewEditCmd(getUC *internal.GetMemoryUseCase, setUC *internal.SetMemoryUseCase, commitUC *internal.CommitUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <key>",
		Short: "Edit a memory in $EDITOR",
		Long:  `Open a memory in your editor. Creates the memory if it doesn't exist. Auto-commits on save.`,
		Args:  cobra.ExactArgs(1),
		RunE:  makeEditRunner(getUC, setUC, commitUC),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeEditRunner(getUC *internal.GetMemoryUseCase, setUC *internal.SetMemoryUseCase, commitUC *internal.CommitUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		key := args[0]
		scopeHint, _ := cmd.Flags().GetString("scope")
		message, _ := cmd.Flags().GetString("message")

		existing, err := getUC.Execute(cmd.Context(), internal.GetMemoryInput{
			Key: key, Scope: scopeHint,
		})
		if err != nil && err != internal.ErrNotFound {
			return fmt.Errorf("get memory: %w", err)
		}

		tmpFile, err := os.CreateTemp("", "mem-edit-*.txt")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if existing != nil {
			if _, err := tmpFile.WriteString(existing.Content); err != nil {
				return fmt.Errorf("write temp file: %w", err)
			}
		}
		tmpFile.Close()

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}

		c := exec.Command(editor, tmpFile.Name())
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("editor: %w", err)
		}

		content, err := os.ReadFile(tmpFile.Name())
		if err != nil {
			return fmt.Errorf("read edited file: %w", err)
		}

		if existing != nil && string(content) == existing.Content {
			fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
			return nil
		}

		if err := setUC.Execute(cmd.Context(), internal.SetMemoryInput{
			Key: key, Content: string(content), Scope: scopeHint,
		}); err != nil {
			return fmt.Errorf("save memory: %w", err)
		}

		if err := autoCommit(cmd.Context(), commitUC, message, "edit", key, scopeHint); err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		if existing != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", key)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", key)
		}
		return nil
	}
}
