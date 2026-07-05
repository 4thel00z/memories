package main

import (
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewIndexCmd(rebuildUC *internal.RebuildIndexUseCase, statusUC *internal.IndexStatusUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Manage the vector search index",
		Long:  `Rebuild or inspect the semantic search index.`,
	}

	cmd.AddCommand(
		newIndexRebuildCmd(rebuildUC),
		newIndexStatusCmd(statusUC),
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

			in := internal.RebuildIndexInput{Scope: scopeHint, NumTrees: trees}
			if err := runRebuild(cmd.Context(), rebuildUC, in, cmd.OutOrStdout()); err != nil {
				return fmt.Errorf("rebuild index: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().Int("trees", 10, "Number of trees for the index")
	return cmd
}

func newIndexStatusCmd(statusUC *internal.IndexStatusUseCase) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			scopeHint, _ := cmd.Flags().GetString("scope")
			jsonOut, _ := cmd.Flags().GetBool("json")

			stats, err := statusUC.Execute(cmd.Context(), internal.IndexStatusInput{Scope: scopeHint})
			if err != nil {
				return fmt.Errorf("index status: %w", err)
			}

			out := cmd.OutOrStdout()
			if jsonOut {
				return printJSON(out, stats)
			}

			if !stats.Exists {
				fmt.Fprintln(out, "No index built. Run 'mem index rebuild' to build one.")
				return nil
			}

			fmt.Fprintf(out, "Path:      %s\n", stats.Path)
			fmt.Fprintf(out, "Vectors:   %d\n", stats.Vectors)
			fmt.Fprintf(out, "Dimension: %d\n", stats.Dimension)
			fmt.Fprintf(out, "Size:      %d bytes\n", stats.SizeBytes)
			return nil
		},
	}
}
