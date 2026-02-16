package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewCommitCmd(commitUC *internal.CommitUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit staged changes",
		Long:  `Commit all staged changes to the memory store. Opens $EDITOR if no message provided.`,
		RunE:  makeCommitRunner(commitUC),
	}

	cmd.Flags().StringP("message", "m", "", "Commit message")
	return cmd
}

func makeCommitRunner(commitUC *internal.CommitUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		scopeHint, _ := cmd.Flags().GetString("scope")

		if message == "" {
			var err error
			message, err = getMessageFromEditor()
			if err != nil {
				return fmt.Errorf("get message: %w", err)
			}
		}

		if message == "" {
			return fmt.Errorf("commit message required")
		}

		out, err := commitUC.Execute(cmd.Context(), internal.CommitInput{
			Message: message, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("commit: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", out.Hash[:7], out.Message)
		return nil
	}
}

func getMessageFromEditor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "mem-commit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("\n# Enter commit message above. Lines starting with # are ignored.\n"); err != nil {
		return "", err
	}
	tmpFile.Close()

	c := exec.Command(editor, tmpFile.Name())
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return "", err
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			lines = append(lines, line)
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}
