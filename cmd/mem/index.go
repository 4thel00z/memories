package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewIndexCmd(rebuildUC *internal.RebuildIndexUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage the vector search index",
		Long:  `Rebuild or inspect the semantic search index.`,
	}

	cmd.AddCommand(
		newIndexRebuildCmd(rebuildUC),
		newIndexStatusCmd(),
	)

	return cmd
}

func newIndexRebuildCmd(rebuildUC *internal.RebuildIndexUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the search index",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			trees, _ := cmd.Flags().GetInt("trees")

			if err := rebuildUC.Execute(cmd.Context(), internal.RebuildIndexInput{
				Scope: scopeHint, NumTrees: trees,
			}); err != nil {
				return fmt.Errorf("rebuild index: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Index rebuilt successfully.")
			return nil
		},
	}

	cmd.Flags().Int("trees", 10, "Number of trees for the index")
	return cmd
}

func newIndexStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Index status: use 'mem index rebuild' to build.")
			return nil
		},
	}
}
