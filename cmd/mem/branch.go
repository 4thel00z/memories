package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewBranchCmd(
	currentUC *internal.BranchCurrentUseCase,
	listUC *internal.BranchListUseCase,
	createUC *internal.BranchCreateUseCase,
	switchUC *internal.BranchSwitchUseCase,
	deleteUC *internal.BranchDeleteUseCase,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch [name]",
		Short: "List, create, or delete branches",
		Long:  `List branches, create and switch to a new branch, or delete an existing branch.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  makeBranchRunner(currentUC, listUC, createUC, switchUC, deleteUC),
	}

	cmd.Flags().BoolP("delete", "d", false, "Delete branch")
	return cmd
}

func makeBranchRunner(
	currentUC *internal.BranchCurrentUseCase,
	listUC *internal.BranchListUseCase,
	createUC *internal.BranchCreateUseCase,
	switchUC *internal.BranchSwitchUseCase,
	deleteUC *internal.BranchDeleteUseCase,
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		scopeHint, _ := cmd.Flags().GetString("scope")
		del, _ := cmd.Flags().GetBool("delete")

		if len(args) == 0 {
			return listBranches(cmd, currentUC, listUC, scopeHint)
		}

		name := args[0]
		if del {
			return deleteBranch(cmd, deleteUC, name, scopeHint)
		}

		return createAndSwitchBranch(cmd, createUC, switchUC, name, scopeHint)
	}
}

func listBranches(cmd *cobra.Command, currentUC *internal.BranchCurrentUseCase, listUC *internal.BranchListUseCase, scopeHint string) error {
	current, err := currentUC.Execute(cmd.Context(), internal.BranchInput{Scope: scopeHint})
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	out, err := listUC.Execute(cmd.Context(), internal.BranchInput{Scope: scopeHint})
	if err != nil {
		return fmt.Errorf("list branches: %w", err)
	}

	for _, b := range out.Branches {
		prefix := "  "
		if b.Name == current.Name {
			prefix = "* "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", prefix, b.Name)
	}
	return nil
}

func deleteBranch(cmd *cobra.Command, deleteUC *internal.BranchDeleteUseCase, name, scopeHint string) error {
	if err := deleteUC.Execute(cmd.Context(), internal.BranchInput{Name: name, Scope: scopeHint}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Deleted branch %s\n", name)
	return nil
}

func createAndSwitchBranch(cmd *cobra.Command, createUC *internal.BranchCreateUseCase, switchUC *internal.BranchSwitchUseCase, name, scopeHint string) error {
	if _, err := createUC.Execute(cmd.Context(), internal.BranchInput{Name: name, Scope: scopeHint}); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}
	if err := switchUC.Execute(cmd.Context(), internal.BranchInput{Name: name, Scope: scopeHint}); err != nil {
		return fmt.Errorf("switch branch: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Switched to new branch %s\n", name)
	return nil
}
