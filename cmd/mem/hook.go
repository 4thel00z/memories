package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewHookCmd(uc *internal.RunHookUseCase) *cobra.Command {
	hookCmd := &cobra.Command{
		Use:    "hook",
		Short:  "Git hook management (internal)",
		Hidden: true,
	}

	runCmd := &cobra.Command{
		Use:   "run [hook-type]",
		Short: "Execute a hook handler",
		Args:  cobra.ExactArgs(1),
		RunE:  makeHookRunRunner(uc),
	}

	hookCmd.AddCommand(runCmd)
	return hookCmd
}

func makeHookRunRunner(uc *internal.RunHookUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		hookType := args[0]
		if hookType != "post-commit" {
			return fmt.Errorf("unsupported hook type: %s", hookType)
		}

		cc, err := gatherCommitContext()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "mem hook: failed to gather context: %v\n", err)
			return nil
		}

		if err := uc.Execute(cmd.Context(), internal.RunHookInput{
			HookType:      hookType,
			CommitContext: *cc,
		}); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "mem hook: %v\n", err)
		}

		return nil
	}
}

func gatherCommitContext() (*internal.CommitContext, error) {
	hash, err := gitOutput("rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get commit hash: %w", err)
	}

	message, err := gitOutput("log", "-1", "--format=%s")
	if err != nil {
		return nil, fmt.Errorf("get commit message: %w", err)
	}

	author, err := gitOutput("log", "-1", "--format=%an")
	if err != nil {
		return nil, fmt.Errorf("get commit author: %w", err)
	}

	diff, err := gitOutput("diff", "HEAD~1..HEAD")
	if err != nil {
		diff = ""
	}

	return &internal.CommitContext{
		Hash:    strings.TrimSpace(hash),
		Message: strings.TrimSpace(message),
		Author:  strings.TrimSpace(author),
		Diff:    diff,
	}, nil
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
