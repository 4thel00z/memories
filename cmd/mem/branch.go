package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewBranchCmd(svc func() *internal.BranchService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch [name]",
		Short: "List, create, or delete branches",
		Long:  `List branches, create and switch to a new branch, or delete an existing branch.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeBranchRunner(svc),
	}

	cmd.Flags().BoolP("delete", "d", false, "Delete branch")
	return cmd
}

func makeBranchRunner(svc func() *internal.BranchService) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")
		del, _ := cmd.Flags().GetBool("delete")

		if len(args) == 0 {
			return listBranches(cmd, svc(), scopeHint)
		}

		name := args[0]
		if del {
			return deleteBranch(cmd, svc(), name, scopeHint)
		}

		return createAndSwitchBranch(cmd, svc(), name, scopeHint)
	}
}

func listBranches(cmd *cobra.Command, svc *internal.BranchService, scopeHint string) error {
	current, err := svc.Current(cmd.Context(), scopeHint)
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	branches, err := svc.List(cmd.Context(), scopeHint)
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}

	for _, b := range branches {
		prefix := "  "
		if b.Name == current.Name {
			prefix = "* "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", prefix, b.Name)
	}
	return nil
}

func deleteBranch(cmd *cobra.Command, svc *internal.BranchService, name, scopeHint string) error {
	if err := svc.Delete(cmd.Context(), name, scopeHint); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch %s\n", name)
	return nil
}

func createAndSwitchBranch(cmd *cobra.Command, svc *internal.BranchService, name, scopeHint string) error {
	if err := svc.Create(cmd.Context(), name, scopeHint); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}
	if err := svc.Switch(cmd.Context(), name, scopeHint); err != nil {
		return fmt.Errorf("switch branch: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Switched to new branch %s\n", name)
	return nil
}
