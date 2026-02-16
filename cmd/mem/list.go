package main

import (
	"encoding/json"
	"fmt"

	"github.com/4thel00z/memories/internal"
	"github.com/spf13/cobra"
)

func NewListCmd(listUC *internal.ListMemoriesUseCase) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [prefix]",
		Aliases: []string{"ls"},
		Short:   "List memories",
		Long:    `List all memories, optionally filtered by prefix.`,
		Args:    cobra.MaximumNArgs(1),
		RunE:    makeListRunner(listUC),
	}

	return cmd
}

func makeListRunner(listUC *internal.ListMemoriesUseCase) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		prefix := ""
		if len(args) > 0 {
			prefix = args[0]
		}

		scopeHint, _ := cmd.Flags().GetString("scope")
		asJSON, _ := cmd.Flags().GetBool("json")

		out, err := listUC.Execute(cmd.Context(), internal.ListMemoriesInput{
			Prefix: prefix, Scope: scopeHint,
		})
		if err != nil {
			return fmt.Errorf("list memories: %w", err)
		}

		if asJSON {
			return outputListJSON(cmd, out)
		}

		for _, mem := range out.Memories {
			fmt.Fprintln(cmd.OutOrStdout(), mem.Key)
		}
		return nil
	}
}

func outputListJSON(cmd *cobra.Command, out *internal.ListMemoriesOutput) error {
	data := make([]map[string]any, 0, len(out.Memories))
	for _, mem := range out.Memories {
		data = append(data, map[string]any{
			"key":        mem.Key,
			"created_at": mem.CreatedAt,
			"updated_at": mem.UpdatedAt,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
